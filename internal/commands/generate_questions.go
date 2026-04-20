package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/llm"
	"github.com/tyrohunt/axon/internal/metrics"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

const questionModel = "claude-opus-4-6"
const questionMaxTokens = 4096

// GenerateQuestionsCommand starts an LLM generation stream for a session.
type GenerateQuestionsCommand struct {
	TrackID       string
	SessionNumber int
}

// GenerateQuestionsHandler streams questions from the LLM and writes them when done.
type GenerateQuestionsHandler struct {
	store             *filesystem.TrackStore
	client            llm.Client
	contextTokenLimit int
	softTokenLimit    int // triggers split plan when exceeded (T2)
	hardTokenLimit    int // hard abort when exceeded (T2)
	rec               *metrics.Recorder
}

func NewGenerateQuestionsHandler(store *filesystem.TrackStore, client llm.Client, contextTokenLimit, softTokenLimit, hardTokenLimit int, rec *metrics.Recorder) *GenerateQuestionsHandler {
	return &GenerateQuestionsHandler{
		store:             store,
		client:            client,
		contextTokenLimit: contextTokenLimit,
		softTokenLimit:    softTokenLimit,
		hardTokenLimit:    hardTokenLimit,
		rec:               rec,
	}
}

// Stream returns a channel of llm.Chunk. The caller receives incremental text
// as the LLM generates. When the final chunk arrives (Done=true or Error≠nil),
// the questions JSON has already been written to 01_questions.json.
func (h *GenerateQuestionsHandler) Stream(ctx context.Context, cmd GenerateQuestionsCommand) <-chan llm.Chunk {
	out := make(chan llm.Chunk, 128)
	go func() {
		defer close(out)
		if err := h.run(ctx, cmd, out); err != nil {
			out <- llm.Chunk{Error: err}
		}
	}()
	return out
}

func (h *GenerateQuestionsHandler) run(ctx context.Context, cmd GenerateQuestionsCommand, out chan<- llm.Chunk) error {
	// Load concept map with ancestor bloom floor applied (F2 — live concept map inheritance).
	cm, err := h.store.GetEffectiveConceptMap(ctx, cmd.TrackID)
	if err != nil {
		return fmt.Errorf("generate questions: concept map: %w", err)
	}

	// ── Soft/hard limit gate ─────────────────────────────────────────────────
	// Load the full corpus (no limit) to measure total tokens before committing
	// to an LLM call. Hard limit aborts immediately; soft limit writes a split
	// plan and halts — user must approve shards via the context editor (T4).
	_, fullFiles, totalTokens, err := h.store.LoadFullContext(ctx, cmd.TrackID)
	if err != nil {
		return fmt.Errorf("generate questions: full context load: %w", err)
	}
	if h.hardTokenLimit > 0 && totalTokens > h.hardTokenLimit {
		h.rec.Record(metrics.Event{
			Event:      "hard_limit_hit",
			TrackID:    cmd.TrackID,
			SessionNum: cmd.SessionNumber,
			Tokens:     int64(totalTokens),
			Extra:      map[string]any{"hard_limit": h.hardTokenLimit},
		})
		return fmt.Errorf("generate questions: %w (total: %d, limit: %d)",
			filesystem.ErrHardLimitExceeded, totalTokens, h.hardTokenLimit)
	}
	if h.softTokenLimit > 0 && totalTokens > h.softTokenLimit {
		h.rec.Record(metrics.Event{
			Event:      "soft_limit_hit",
			TrackID:    cmd.TrackID,
			SessionNum: cmd.SessionNumber,
			Tokens:     int64(totalTokens),
			Extra:      map[string]any{"soft_limit": h.softTokenLimit},
		})
		if err := h.store.WriteSplitPlan(ctx, cmd.TrackID, fullFiles, totalTokens, h.softTokenLimit); err != nil {
			return fmt.Errorf("generate questions: write split plan: %w", err)
		}
		return fmt.Errorf("generate questions: %w (total: %d, limit: %d) — review _split_plan.md in context editor",
			filesystem.ErrSplitRequired, totalTokens, h.softTokenLimit)
	}

	// ── Load context for LLM ─────────────────────────────────────────────────
	// If the session was created with a shard, load only that shard's files.
	// Otherwise use the normal budget-gated inherited context.
	var contextFiles map[string]string
	meta, _ := h.store.ReadSessionMetadata(ctx, cmd.TrackID, cmd.SessionNumber)
	if meta.ShardID != "" {
		shardFiles, shardTokens, err := h.store.LoadContextForShard(ctx, cmd.TrackID, meta.ShardID)
		if err != nil {
			return fmt.Errorf("generate questions: shard context: %w", err)
		}
		h.rec.Record(metrics.Event{
			Event:      "context_load",
			TrackID:    cmd.TrackID,
			SessionNum: cmd.SessionNumber,
			Tokens:     int64(shardTokens),
			Extra:      map[string]any{"shard_id": meta.ShardID},
		})
		contextFiles = shardFiles
	} else {
		budget, err := h.store.LoadInheritedContext(ctx, cmd.TrackID, h.contextTokenLimit)
		if err != nil {
			return fmt.Errorf("generate questions: context files: %w", err)
		}
		h.rec.Record(metrics.Event{
			Event:      "context_load",
			TrackID:    cmd.TrackID,
			SessionNum: cmd.SessionNumber,
			Tokens:     int64(budget.TokensUsed),
			Extra:      map[string]any{"truncated": budget.Truncated, "truncated_at": budget.TruncatedAt},
		})
		contextFiles = budget.Files
		if budget.Truncated {
			out <- llm.Chunk{Text: fmt.Sprintf(
				"[context truncated at track %s — %d tokens used of %d limit]\n",
				budget.TruncatedAt, budget.TokensUsed, h.contextTokenLimit,
			)}
		}
	}

	generationID := uuid.New().String()
	system := BuildSystemPrompt()
	user := BuildUserPrompt(cmd.TrackID, generationID, cm, contextFiles)

	chunks := h.client.Stream(ctx, questionModel, system, []llm.Message{
		{Role: "user", Content: user},
	}, questionMaxTokens)

	// Accumulate streamed text, forward to caller.
	var sb strings.Builder
	for chunk := range chunks {
		if chunk.Error != nil {
			return chunk.Error
		}
		if chunk.Text != "" {
			sb.WriteString(chunk.Text)
			out <- llm.Chunk{Text: chunk.Text}
		}
		if chunk.Done {
			break
		}
	}

	// Parse the accumulated JSON and persist.
	questions, err := parseQuestionsJSON(sb.String(), generationID)
	if err != nil {
		return fmt.Errorf("generate questions: parse JSON: %w", err)
	}
	b, err := json.MarshalIndent(questions, "", "  ")
	if err != nil {
		return fmt.Errorf("generate questions: marshal: %w", err)
	}
	if err := h.store.WriteSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "01_questions.json", b); err != nil {
		return fmt.Errorf("generate questions: write: %w", err)
	}
	h.rec.Record(metrics.Event{
		Event:      "questions_generated",
		TrackID:    cmd.TrackID,
		SessionNum: cmd.SessionNumber,
		Extra:      map[string]any{"count": len(questions), "generation_id": generationID},
	})

	out <- llm.Chunk{Done: true}
	return nil
}

// ─── prompt builders ──────────────────────────────────────────────────────────

func BuildSystemPrompt() string {
	return `You are an adaptive quiz question generator for the Axon learning system.
Generate quiz questions as a strict JSON array. Output ONLY the JSON array — no prose, no markdown code fences, no extra text.

Each question object must match this schema exactly:
{
  "id": "q_1",
  "generation_id": "<copy from input>",
  "concept_indexes": [0],
  "bloom_level": 2,
  "bloom_label": "Understand",
  "question": "<question text>",
  "format": "mcq",
  "options": {"A": "...", "B": "...", "C": "...", "D": "..."},
  "correct": "B",
  "correct_explanation": "...",
  "distractor_explanations": {"A": "...", "C": "...", "D": "..."},
  "is_cross_branch": false,
  "difficulty_estimate": 0.4,
  "spaced_repetition_concept_id": null,
  "expected_time_seconds": 60,
  "requires_explanation": false
}

Rules:
- bloom_label: Remember(1) Understand(2) Apply(3) Analyze(4) Evaluate(5) Create(6)
- For free_text or design format: omit options/correct/distractor_explanations, set requires_explanation=true
- For mcq/scenario_mcq: include all four options A-D, exactly one correct key
- Target bloom_level = bloom_current + 1 for each concept (don't exceed 6)
- Bottleneck concepts must appear in at least 2 questions
- EXPLORATION_UNLOCKED concepts: include in the session even if prerequisites are unmet.
  Set difficulty_estimate to 0.5× what it would normally be (provisional scoring weight).
  This is faith-based exposure — the learner is stretching; failure is expected and acceptable.
- Include at least 3 MCQ and 1 free_text
- is_cross_branch=true when concept_indexes span multiple branches
- If a context file is a conversation analysis (contains sections like "Reasoning Patterns",
  "Misconception Fingerprint", "Distractor Affinities", "Curiosity Clusters", "Mental Models
  That Clicked/Failed", or "Inquiry Patterns"), use it to adapt question framing and distractor
  selection — not to change which concepts are tested, but how. Mirror the learner's reasoning
  style; target distractors at their specific misframings; prioritise concepts from Curiosity
  Clusters slightly above what bloom_gap alone would suggest; use framings from Mental Models
  That Clicked and avoid framings from Mental Models That Failed.
- If an "Inquiry Patterns" section is present: for concepts flagged with low-precision questions,
  include at least one question that forces the learner to target mechanism rather than symptom.
  For concepts with a divergent thread arc, prefer scenario questions over recall questions to
  encourage narrowing. Do not reward surface-feature answers — make the correct explanation
  require mechanism-level reasoning.`
}

func BuildUserPrompt(trackID, generationID string, cm domain.ConceptMap, contextFiles map[string]string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Track: %s\nGeneration ID: %s\n\n", trackID, generationID)

	sb.WriteString("Concept Map:\n")
	// Group by branch.
	branchConcepts := map[string][]domain.Concept{}
	for _, c := range cm.Concepts {
		branchConcepts[c.Branch] = append(branchConcepts[c.Branch], c)
	}
	for _, branch := range cm.MajorBranches {
		concepts := branchConcepts[branch]
		if len(concepts) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "\nBranch: %s\n", branch)
		for _, c := range concepts {
			flags := ""
			if c.IsBottleneck {
				flags += ", BOTTLENECK"
			}
			if c.ExplorationUnlocked {
				flags += ", EXPLORATION_UNLOCKED"
			}
			prereqs := ""
			if len(c.PrerequisiteIndexes) > 0 {
				prereqs = fmt.Sprintf(", prereqs: %v", c.PrerequisiteIndexes)
			}
			fmt.Fprintf(&sb, "  [%d] %s (bloom: %d→%d%s%s)\n",
				c.Index, c.Name, c.BloomCurrent, c.BloomTarget, flags, prereqs)
			if c.Description != "" {
				fmt.Fprintf(&sb, "      %s\n", c.Description)
			}
		}
	}

	if len(contextFiles) > 0 {
		sb.WriteString("\nContext Documents:\n")
		// Sort filenames for determinism.
		filenames := make([]string, 0, len(contextFiles))
		for name := range contextFiles {
			if !strings.HasPrefix(name, "_") { // skip _sources.md etc.
				filenames = append(filenames, name)
			}
		}
		for _, name := range filenames {
			fmt.Fprintf(&sb, "\n--- %s ---\n%s\n", name, contextFiles[name])
		}
	}

	sb.WriteString("\nGenerate 8 questions. Prioritize concepts where bloom_current < bloom_target.\n")
	return sb.String()
}

// parseQuestionsJSON extracts a JSON array from the LLM output.
// Strips markdown fences if present, patches generation_id if missing.
func parseQuestionsJSON(raw, generationID string) ([]domain.Question, error) {
	s := strings.TrimSpace(raw)
	// Strip markdown code fence if LLM added one despite instructions.
	if strings.HasPrefix(s, "```") {
		first := strings.Index(s, "\n")
		if first >= 0 {
			s = s[first+1:]
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}

	var questions []domain.Question
	if err := json.Unmarshal([]byte(s), &questions); err != nil {
		return nil, fmt.Errorf("unmarshal: %w (raw prefix: %.200s)", err, s)
	}
	// Patch any missing generation IDs.
	for i := range questions {
		if questions[i].GenerationID == "" {
			questions[i].GenerationID = generationID
		}
	}
	return questions, nil
}
