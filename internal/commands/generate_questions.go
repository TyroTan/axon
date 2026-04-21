package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tyrohunt/axon/internal/config"
	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/llm"
	"github.com/tyrohunt/axon/internal/metrics"
	"github.com/tyrohunt/axon/internal/rag"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

const questionModel = "claude-opus-4-6"
const questionMaxTokens = 4096

// defaultQuestionTarget is used when AXON_QUESTION_COUNT is not set.
const defaultQuestionTarget = 8

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
	softTokenLimit    int
	hardTokenLimit    int
	questionTarget    int // AXON_QUESTION_COUNT — target questions per session (default 8)
	rec               *metrics.Recorder
}

func NewGenerateQuestionsHandler(store *filesystem.TrackStore, client llm.Client, contextTokenLimit, softTokenLimit, hardTokenLimit, questionTarget int, rec *metrics.Recorder) *GenerateQuestionsHandler {
	if questionTarget <= 0 {
		questionTarget = defaultQuestionTarget
	}
	return &GenerateQuestionsHandler{
		store:             store,
		client:            client,
		contextTokenLimit: contextTokenLimit,
		softTokenLimit:    softTokenLimit,
		hardTokenLimit:    hardTokenLimit,
		questionTarget:    questionTarget,
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

	// ── State snapshot ───────────────────────────────────────────────────────
	configLevel := config.ActiveLevel()
	learnerSignal := h.store.CurrentLearnerSignal(ctx, cmd.TrackID)
	effectiveScore := config.EffectiveScore(configLevel.Score, learnerSignal)
	activeLevel := config.NearestLevel(effectiveScore)
	snapshot := &domain.StateSnapshot{
		AxonConfigScore: configLevel.Score,
		LearnerSignal:   learnerSignal,
		EffectiveScore:  effectiveScore,
		LevelName:       activeLevel.Name,
	}
	if sessMeta, err2 := h.store.ReadSessionMetadata(ctx, cmd.TrackID, cmd.SessionNumber); err2 == nil {
		sessMeta.StateSnapshot = snapshot
		_ = h.store.WriteSessionMetadata(ctx, cmd.TrackID, cmd.SessionNumber, sessMeta)
	}
	_ = h.store.AppendPathEntry(ctx, domain.PathEntry{
		TrackID:         cmd.TrackID,
		SessionNum:      cmd.SessionNumber,
		Event:           "generated",
		VisitedAt:       time.Now(),
		AxonConfigScore: snapshot.AxonConfigScore,
		LearnerSignal:   snapshot.LearnerSignal,
		EffectiveScore:  snapshot.EffectiveScore,
	})

	// ── Partition context: job posts vs regular ──────────────────────────────
	jobFiles := map[string]string{}
	regularFiles := map[string]string{}
	for name, content := range contextFiles {
		if strings.HasSuffix(name, ".job.md") {
			jobFiles[name] = content
		} else {
			regularFiles[name] = content
		}
	}

	generationID := uuid.New().String()
	var allQuestions []domain.Question

	// ── Call 1: concept map questions ────────────────────────────────────────
	// Over-generate by 25% (min +2) so the deduplicator has room to work.
	cmTarget := h.questionTarget + overgenerate(h.questionTarget)
	out <- llm.Chunk{Text: fmt.Sprintf("[concept map: requesting %d questions]\n", cmTarget)}
	cmQuestions, err := h.streamCall(ctx, cmd,
		BuildSystemPrompt(activeLevel),
		BuildUserPrompt(cmd.TrackID, generationID, cm, regularFiles, activeLevel, cmTarget),
		generationID, out,
	)
	if err != nil {
		return err
	}
	allQuestions = append(allQuestions, cmQuestions...)

	// ── Call 2+: one call per job post file ──────────────────────────────────
	// Each call is entirely job-framed; no ratio rules needed.
	if len(jobFiles) > 0 {
		jobTarget := jobQuestionTarget(h.questionTarget)
		for name, content := range jobFiles {
			out <- llm.Chunk{Text: fmt.Sprintf("[job post %s: requesting %d questions]\n", name, jobTarget)}
			jpQuestions, err := h.streamCall(ctx, cmd,
				BuildJobPostSystemPrompt(activeLevel),
				BuildJobPostUserPrompt(cmd.TrackID, generationID, cm, name, content, activeLevel, jobTarget),
				generationID, out,
			)
			if err != nil {
				return err
			}
			allQuestions = append(allQuestions, jpQuestions...)
		}
	}

	// ── Deduplicate + shuffle + trim ─────────────────────────────────────────
	questions := deduplicateQuestions(allQuestions, h.questionTarget)
	questions = shuffleQuestions(questions, generationID)
	renumberQuestions(questions)

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

// streamCall executes a single LLM call, streams chunks to out, and returns parsed questions.
func (h *GenerateQuestionsHandler) streamCall(ctx context.Context, cmd GenerateQuestionsCommand, system, user, generationID string, out chan<- llm.Chunk) ([]domain.Question, error) {
	chunks := h.client.Stream(ctx, questionModel, system, []llm.Message{
		{Role: "user", Content: user},
	}, questionMaxTokens)

	var sb strings.Builder
	for chunk := range chunks {
		if chunk.Error != nil {
			return nil, chunk.Error
		}
		if chunk.Text != "" {
			sb.WriteString(chunk.Text)
			out <- llm.Chunk{Text: chunk.Text}
		}
		if chunk.Done {
			break
		}
	}

	questions, err := parseQuestionsJSON(sb.String(), generationID)
	if err != nil {
		return nil, fmt.Errorf("generate questions: parse JSON: %w", err)
	}
	return questions, nil
}

// ─── prompt builders ──────────────────────────────────────────────────────────

func BuildSystemPrompt(level config.LevelConfig) string {
	var levelRules strings.Builder

	if level.BloomDelta != 0 {
		fmt.Fprintf(&levelRules, "- LEVEL OVERRIDE: target bloom_level = bloom_current %+d for each concept (clamp to 1–6, never exceed bloom_target).\n", level.BloomDelta)
	} else {
		levelRules.WriteString("- Target bloom_level = bloom_current + 1 for each concept (don't exceed 6).\n")
	}

	if level.DifficultyFloor > 0 {
		fmt.Fprintf(&levelRules, "- LEVEL OVERRIDE: all questions must have difficulty_estimate >= %.2f. Do not generate easier questions even if the concept is at a low bloom level.\n", level.DifficultyFloor)
	}

	if level.CrossBranchWeight != nil {
		w := *level.CrossBranchWeight
		if w == 0 {
			levelRules.WriteString("- LEVEL OVERRIDE: do not generate cross-branch questions (is_cross_branch must be false for all).\n")
		} else {
			fmt.Fprintf(&levelRules, "- LEVEL OVERRIDE: at least %.0f%% of questions must be cross-branch (is_cross_branch=true).\n", w*100)
		}
	}

	switch level.FormatBias {
	case "mcq_only":
		levelRules.WriteString("- LEVEL OVERRIDE: use mcq or scenario_mcq format only — no free_text or design questions.\n")
	case "free_text_heavy":
		levelRules.WriteString("- LEVEL OVERRIDE: at least half of questions must be free_text or scenario_mcq format.\n")
	case "design_heavy":
		levelRules.WriteString("- LEVEL OVERRIDE: at least a third of questions must be design format.\n")
	}

	if level.TimeMultiplier != 1.0 {
		fmt.Fprintf(&levelRules, "- LEVEL OVERRIDE: scale expected_time_seconds by %.1f× relative to what you would normally estimate.\n", level.TimeMultiplier)
	}

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
` + levelRules.String() + `- Bottleneck concepts must appear in at least 2 questions
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

// BuildJobPostSystemPrompt generates the system prompt for a dedicated job-post question call.
// All questions in this call are job-framed — no ratio rules needed.
func BuildJobPostSystemPrompt(level config.LevelConfig) string {
	var levelRules strings.Builder

	if level.DifficultyFloor > 0 {
		fmt.Fprintf(&levelRules, "- LEVEL OVERRIDE: all questions must have difficulty_estimate >= %.2f.\n", level.DifficultyFloor)
	}
	if level.TimeMultiplier != 1.0 {
		fmt.Fprintf(&levelRules, "- LEVEL OVERRIDE: scale expected_time_seconds by %.1f×.\n", level.TimeMultiplier)
	}

	return `You are an adaptive quiz question generator for the Axon learning system.
You are generating job-context questions: every question must be framed as a real-world task the learner would face in the described role.
Generate quiz questions as a strict JSON array. Output ONLY the JSON array — no prose, no markdown code fences, no extra text.

Each question object must match this schema exactly:
{
  "id": "q_1",
  "generation_id": "<copy from input>",
  "concept_indexes": [0],
  "bloom_level": 3,
  "bloom_label": "Apply",
  "question": "<scenario question>",
  "format": "scenario_mcq",
  "options": {"A": "...", "B": "...", "C": "...", "D": "..."},
  "correct": "B",
  "correct_explanation": "...",
  "distractor_explanations": {"A": "...", "C": "...", "D": "..."},
  "is_cross_branch": false,
  "difficulty_estimate": 0.5,
  "spaced_repetition_concept_id": null,
  "expected_time_seconds": 90,
  "requires_explanation": false
}

Rules:
- bloom_label: Remember(1) Understand(2) Apply(3) Analyze(4) Evaluate(5) Create(6)
- Default format is scenario_mcq; use free_text for open-ended design/trade-off tasks
- For mcq/scenario_mcq: include all four options A-D, exactly one correct key
- For free_text: omit options/correct/distractor_explanations, set requires_explanation=true
- concept_indexes must reference the provided concept map — do not invent new concepts
- Each question must target a different concept_index — do not repeat the same concept
- Spread questions across different branches of the concept map where possible
- Do not generate questions about the job posting itself (salary, company name, etc.)
- The job posting provides real-world framing only; the concept map determines what is tested
` + levelRules.String()
}

func BuildUserPrompt(trackID, generationID string, cm domain.ConceptMap, contextFiles map[string]string, level config.LevelConfig, targetCount int) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Track: %s\nGeneration ID: %s\nDifficulty Level: %s\n\n", trackID, generationID, level.Name)

	sb.WriteString("Concept Map:\n")
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
		filenames := make([]string, 0, len(contextFiles))
		for name := range contextFiles {
			if !strings.HasPrefix(name, "_") {
				filenames = append(filenames, name)
			}
		}
		sort.Strings(filenames)
		for _, name := range filenames {
			fmt.Fprintf(&sb, "\n--- %s ---\n%s\n", name, contextFiles[name])
		}
	}

	fmt.Fprintf(&sb, "\nGenerate %d questions. Prioritize concepts where bloom_current < bloom_target. Favour quality over count — produce fewer but sharper questions if needed.\n", targetCount)
	return sb.String()
}

// BuildJobPostUserPrompt builds the user prompt for a dedicated job-post question call.
func BuildJobPostUserPrompt(trackID, generationID string, cm domain.ConceptMap, jobFileName, jobContent string, level config.LevelConfig, targetCount int) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Track: %s\nGeneration ID: %s\nDifficulty Level: %s\n\n", trackID, generationID, level.Name)

	sb.WriteString("Concept Map (use for concept_indexes only):\n")
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
			fmt.Fprintf(&sb, "  [%d] %s\n", c.Index, c.Name)
		}
	}

	fmt.Fprintf(&sb, "\nJob Posting (%s):\n%s\n", jobFileName, jobContent)
	fmt.Fprintf(&sb, "\nGenerate %d job-framed questions. Each must target a different concept — spread across branches. Favour quality over count.\n", targetCount)
	return sb.String()
}

// ─── deduplication + shuffle ──────────────────────────────────────────────────

// deduplicateQuestions removes duplicate questions from the pool and returns
// at most n maximally distinct questions. Two-stage:
//  1. Exact match on (sorted concept_indexes, bloom_level) — guaranteed structural duplicates.
//  2. MMR text similarity via rag.DistinctTopN — removes scenario-phrasing duplicates.
func deduplicateQuestions(questions []domain.Question, n int) []domain.Question {
	if len(questions) == 0 {
		return nil
	}

	// Stage 1: exact (concept_indexes, bloom_level) dedup — keep first occurrence.
	type conceptKey string
	seen := map[conceptKey]bool{}
	stage1 := make([]domain.Question, 0, len(questions))
	for _, q := range questions {
		idxCopy := make([]int, len(q.ConceptIndexes))
		copy(idxCopy, q.ConceptIndexes)
		sort.Ints(idxCopy)
		key := conceptKey(fmt.Sprintf("%v|%d", idxCopy, q.BloomLevel))
		if !seen[key] {
			seen[key] = true
			stage1 = append(stage1, q)
		}
	}

	if len(stage1) <= n {
		return stage1
	}

	// Stage 2: MMR text similarity — pick n most mutually distinct questions.
	chunks := make([]rag.Chunk, len(stage1))
	for i, q := range stage1 {
		chunks[i] = rag.Chunk{
			Content: q.Question,
			Heading: fmt.Sprintf("%d", i),
		}
	}
	selected := rag.DistinctTopN(chunks, n)

	result := make([]domain.Question, 0, len(selected))
	for _, c := range selected {
		var idx int
		fmt.Sscanf(c.Heading, "%d", &idx)
		if idx >= 0 && idx < len(stage1) {
			result = append(result, stage1[idx])
		}
	}
	return result
}

// shuffleQuestions returns a deterministically shuffled copy of questions.
// Seed is derived from generationID via FNV-64a so the same session always
// produces the same final order regardless of call ordering.
func shuffleQuestions(questions []domain.Question, generationID string) []domain.Question {
	if len(questions) == 0 {
		return questions
	}
	h := fnv.New64a()
	h.Write([]byte(generationID))
	r := rand.New(rand.NewSource(int64(h.Sum64()))) //nolint:gosec
	out := make([]domain.Question, len(questions))
	copy(out, questions)
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// renumberQuestions resets question IDs to q_1, q_2, ... after consolidation.
func renumberQuestions(questions []domain.Question) {
	for i := range questions {
		questions[i].ID = fmt.Sprintf("q_%d", i+1)
	}
}

// ─── sizing helpers ───────────────────────────────────────────────────────────

// overgenerate returns the extra questions to request beyond target (25%, min 2).
func overgenerate(target int) int {
	extra := target / 4
	if extra < 2 {
		extra = 2
	}
	return extra
}

// jobQuestionTarget returns how many questions to request per job post file.
// Scales with session target: small sessions get 3, larger get up to 8.
func jobQuestionTarget(sessionTarget int) int {
	switch {
	case sessionTarget >= 16:
		return 8
	case sessionTarget >= 12:
		return 5
	default:
		return 3
	}
}

// ─── JSON parsing ─────────────────────────────────────────────────────────────

// parseQuestionsJSON extracts a JSON array from the LLM output.
func parseQuestionsJSON(raw, generationID string) ([]domain.Question, error) {
	s := strings.TrimSpace(raw)
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
	for i := range questions {
		if questions[i].GenerationID == "" {
			questions[i].GenerationID = generationID
		}
	}
	return questions, nil
}
