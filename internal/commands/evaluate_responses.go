package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/llm"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

const evalModel = "claude-sonnet-4-6"
const evalMaxTokens = 4096

// EvaluateResponsesCommand reads 01_questions.json + 02_responses.json,
// calls the LLM evaluator, and writes 03_evaluations.json.
type EvaluateResponsesCommand struct {
	TrackID       string
	SessionNumber int
}

type EvaluateResponsesHandler struct {
	store  *filesystem.TrackStore
	client llm.Client
}

func NewEvaluateResponsesHandler(store *filesystem.TrackStore, client llm.Client) *EvaluateResponsesHandler {
	return &EvaluateResponsesHandler{store: store, client: client}
}

// Stream starts LLM evaluation and returns a channel of incremental chunks.
// When Done=true, 03_evaluations.json has been written.
func (h *EvaluateResponsesHandler) Stream(ctx context.Context, cmd EvaluateResponsesCommand) <-chan llm.Chunk {
	out := make(chan llm.Chunk, 128)
	go func() {
		defer close(out)
		if err := h.run(ctx, cmd, out); err != nil {
			out <- llm.Chunk{Error: err}
		}
	}()
	return out
}

func (h *EvaluateResponsesHandler) run(ctx context.Context, cmd EvaluateResponsesCommand, out chan<- llm.Chunk) error {
	// Load questions.
	qb, err := h.store.ReadSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "01_questions.json")
	if err != nil {
		return fmt.Errorf("evaluate: load questions: %w", err)
	}
	var questions []domain.Question
	if err := json.Unmarshal(qb, &questions); err != nil {
		return fmt.Errorf("evaluate: parse questions: %w", err)
	}

	// Load responses.
	rb, err := h.store.ReadSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "02_responses.json")
	if err != nil {
		return fmt.Errorf("evaluate: load responses: %w", err)
	}
	var responses []domain.Response
	if err := json.Unmarshal(rb, &responses); err != nil {
		return fmt.Errorf("evaluate: parse responses: %w", err)
	}

	// Index responses by question ID.
	respByID := make(map[string]domain.Response, len(responses))
	for _, r := range responses {
		respByID[r.QuestionID] = r
	}

	system := buildEvalSystemPrompt()
	user := buildEvalUserPrompt(questions, respByID)

	chunks := h.client.Stream(ctx, evalModel, system, []llm.Message{
		{Role: "user", Content: user},
	}, evalMaxTokens)

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

	evaluations, err := parseEvaluations(sb.String(), questions, respByID)
	if err != nil {
		return fmt.Errorf("evaluate: parse result: %w", err)
	}
	b, err := json.MarshalIndent(evaluations, "", "  ")
	if err != nil {
		return fmt.Errorf("evaluate: marshal: %w", err)
	}
	if err := h.store.WriteSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "03_evaluations.json", b); err != nil {
		return fmt.Errorf("evaluate: write: %w", err)
	}

	out <- llm.Chunk{Done: true}
	return nil
}

// ─── prompt builders ──────────────────────────────────────────────────────────

func buildEvalSystemPrompt() string {
	return `You are an expert evaluator for the Axon adaptive learning system.
Given question-response pairs, evaluate each response and output ONLY a JSON array. No prose, no markdown fences.

Each object in the array must follow this schema:
{
  "question_id": "q_1",
  "correctness": 0.0,
  "explanation_score": 0.0,
  "explanation_subscores": {
    "mechanism_accuracy": 0,
    "terminology_precision": 0,
    "edge_case_awareness": 0,
    "generalization_quality": 0
  },
  "error_taxonomy": null,
  "misconception_identified": null,
  "bloom_level_demonstrated": 1,
  "evaluator_notes": "...",
  "feedback_for_learner": "..."
}

Scoring rules:
- correctness: for MCQ use 1.0 (correct) or 0.0 (incorrect) based on whether selected matches correct answer. For free_text/design use 0.0–1.0 judgment.
- explanation_score: 0.0 if no explanation given or not required; otherwise 0.0–1.0.
- explanation_subscores: all zeros if no explanation; 0–5 each otherwise.
- error_taxonomy: "recall_gap" (didn't know the fact), "misconception" (wrong mental model), "reasoning_error" (knew facts but wrong logic), or null if correct.
- bloom_level_demonstrated: the actual Bloom level the response evidence shows (1-6).
- feedback_for_learner: 1-3 sentences, constructive and specific.`
}

func buildEvalUserPrompt(questions []domain.Question, respByID map[string]domain.Response) string {
	var sb strings.Builder
	sb.WriteString("Evaluate the following responses:\n")

	for _, q := range questions {
		r, hasResp := respByID[q.ID]
		sb.WriteString("\n---\n")
		fmt.Fprintf(&sb, "Question %s [Bloom L%d %s] [%s] [Concepts: %v]\n",
			q.ID, q.BloomLevel, q.BloomLabel, q.Format, q.ConceptIndexes)
		fmt.Fprintf(&sb, "Q: %s\n", q.Question)

		if q.Options != nil {
			for _, key := range []string{"A", "B", "C", "D"} {
				if text, ok := q.Options[key]; ok {
					fmt.Fprintf(&sb, "  %s: %s\n", key, text)
				}
			}
			if q.Correct != "" {
				fmt.Fprintf(&sb, "Correct answer: %s\n", q.Correct)
			}
		}
		if q.CorrectExplanation != "" {
			fmt.Fprintf(&sb, "Model answer: %s\n", q.CorrectExplanation)
		}

		if !hasResp {
			sb.WriteString("Learner: (no response)\n")
		} else {
			fmt.Fprintf(&sb, "Learner answered: %s\n", r.SelectedAnswer)
			fmt.Fprintf(&sb, "Confidence: %d/5\n", r.Confidence)
			if r.Explanation != "" {
				fmt.Fprintf(&sb, "Explanation: %s\n", r.Explanation)
			}
		}
	}
	return sb.String()
}

// ─── partial LLM output + enrichment ─────────────────────────────────────────

type llmEvaluation struct {
	QuestionID              string                        `json:"question_id"`
	Correctness             float64                       `json:"correctness"`
	ExplanationScore        float64                       `json:"explanation_score"`
	ExplanationSubscores    domain.ExplanationSubscores   `json:"explanation_subscores"`
	ErrorTaxonomy           *string                       `json:"error_taxonomy"`
	MisconceptionIdentified *string                       `json:"misconception_identified"`
	BloomLevelDemonstrated  int                           `json:"bloom_level_demonstrated"`
	EvaluatorNotes          string                        `json:"evaluator_notes"`
	FeedbackForLearner      string                        `json:"feedback_for_learner"`
}

func parseEvaluations(raw string, questions []domain.Question, respByID map[string]domain.Response) ([]domain.Evaluation, error) {
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

	var llmEvals []llmEvaluation
	if err := json.Unmarshal([]byte(s), &llmEvals); err != nil {
		return nil, fmt.Errorf("unmarshal: %w (raw prefix: %.200s)", err, s)
	}

	// Index questions by ID for enrichment.
	qByID := make(map[string]domain.Question, len(questions))
	for _, q := range questions {
		qByID[q.ID] = q
	}

	evaluations := make([]domain.Evaluation, 0, len(llmEvals))
	for _, le := range llmEvals {
		q := qByID[le.QuestionID]
		r := respByID[le.QuestionID]

		// Compute Brier contribution and calibration flag.
		p := float64(r.Confidence-1) / 4.0 // map 1–5 → 0.0–1.0
		brier := math.Pow(le.Correctness-p, 2)
		var calibFlag *string
		if p > le.Correctness+0.3 {
			s := "overconfident"
			calibFlag = &s
		} else if p < le.Correctness-0.3 {
			s := "underconfident"
			calibFlag = &s
		}

		// Compute time signal.
		timeSignal := "unknown"
		if r.TimeSeconds > 0 && q.ExpectedTimeSeconds > 0 {
			ratio := float64(r.TimeSeconds) / float64(q.ExpectedTimeSeconds)
			switch {
			case ratio < 0.5:
				timeSignal = "fast"
			case ratio <= 1.5:
				timeSignal = "on_time"
			case ratio <= 3.0:
				timeSignal = "slow"
			default:
				timeSignal = "very_slow"
			}
		}

		evaluations = append(evaluations, domain.Evaluation{
			QuestionID:              le.QuestionID,
			GenerationID:            q.GenerationID,
			ConceptIndexes:          q.ConceptIndexes,
			Correctness:             le.Correctness,
			ExplanationScore:        le.ExplanationScore,
			ExplanationSubscores:    le.ExplanationSubscores,
			BrierContribution:       brier,
			CalibrationFlag:         calibFlag,
			ErrorTaxonomy:           le.ErrorTaxonomy,
			MisconceptionIdentified: le.MisconceptionIdentified,
			TimeSignal:              timeSignal,
			BloomLevelDemonstrated:  le.BloomLevelDemonstrated,
			EvaluatorNotes:          le.EvaluatorNotes,
			FeedbackForLearner:      le.FeedbackForLearner,
		})
	}
	return evaluations, nil
}
