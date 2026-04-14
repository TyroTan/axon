package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/llm"
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
	store  *filesystem.TrackStore
	client llm.Client
}

func NewGenerateQuestionsHandler(store *filesystem.TrackStore, client llm.Client) *GenerateQuestionsHandler {
	return &GenerateQuestionsHandler{store: store, client: client}
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
	// Load concept map.
	cm, err := h.store.GetConceptMap(ctx, cmd.TrackID)
	if err != nil {
		return fmt.Errorf("generate questions: concept map: %w", err)
	}

	// Load context files.
	contextFiles, err := h.store.ReadContextFiles(ctx, cmd.TrackID)
	if err != nil {
		return fmt.Errorf("generate questions: context files: %w", err)
	}

	generationID := uuid.New().String()
	system := buildSystemPrompt()
	user := buildUserPrompt(cmd.TrackID, generationID, cm, contextFiles)

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

	out <- llm.Chunk{Done: true}
	return nil
}

// ─── prompt builders ──────────────────────────────────────────────────────────

func buildSystemPrompt() string {
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
- Include at least 3 MCQ and 1 free_text
- is_cross_branch=true when concept_indexes span multiple branches`
}

func buildUserPrompt(trackID, generationID string, cm domain.ConceptMap, contextFiles map[string]string) string {
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
			bottleneck := ""
			if c.IsBottleneck {
				bottleneck = ", BOTTLENECK"
			}
			prereqs := ""
			if len(c.PrerequisiteIndexes) > 0 {
				prereqs = fmt.Sprintf(", prereqs: %v", c.PrerequisiteIndexes)
			}
			fmt.Fprintf(&sb, "  [%d] %s (bloom: %d→%d%s%s)\n",
				c.Index, c.Name, c.BloomCurrent, c.BloomTarget, bottleneck, prereqs)
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
