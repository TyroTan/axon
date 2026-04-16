package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/llm"
	"github.com/tyrohunt/axon/internal/metrics"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

const synthModel = "claude-sonnet-4-6"
const synthMaxTokens = 2048

// GenerateSynthesisCommand reads evaluations + concept map, streams an LLM
// synthesis, and writes 04_synthesis.json.
type GenerateSynthesisCommand struct {
	TrackID       string
	SessionNumber int
}

type GenerateSynthesisHandler struct {
	store  *filesystem.TrackStore
	client llm.Client
	rec    *metrics.Recorder
}

func NewGenerateSynthesisHandler(store *filesystem.TrackStore, client llm.Client, rec *metrics.Recorder) *GenerateSynthesisHandler {
	return &GenerateSynthesisHandler{store: store, client: client, rec: rec}
}

func (h *GenerateSynthesisHandler) Stream(ctx context.Context, cmd GenerateSynthesisCommand) <-chan llm.Chunk {
	out := make(chan llm.Chunk, 128)
	go func() {
		defer close(out)
		if err := h.run(ctx, cmd, out); err != nil {
			out <- llm.Chunk{Error: err}
		}
	}()
	return out
}

func (h *GenerateSynthesisHandler) run(ctx context.Context, cmd GenerateSynthesisCommand, out chan<- llm.Chunk) error {
	// Load concept map.
	cm, err := h.store.GetConceptMap(ctx, cmd.TrackID)
	if err != nil {
		return fmt.Errorf("generate synthesis: concept map: %w", err)
	}

	// Load evaluations.
	eb, err := h.store.ReadSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "03_evaluations.json")
	if err != nil {
		return fmt.Errorf("generate synthesis: load evaluations: %w", err)
	}
	if eb == nil {
		return fmt.Errorf("generate synthesis: no evaluations found — run evaluation first")
	}
	var evaluations []domain.Evaluation
	if err := json.Unmarshal(eb, &evaluations); err != nil {
		return fmt.Errorf("generate synthesis: parse evaluations: %w", err)
	}

	// Load questions for context.
	qb, err := h.store.ReadSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "01_questions.json")
	if err != nil {
		return fmt.Errorf("generate synthesis: load questions: %w", err)
	}
	var questions []domain.Question
	if qb != nil {
		if err := json.Unmarshal(qb, &questions); err != nil {
			return fmt.Errorf("generate synthesis: parse questions: %w", err)
		}
	}

	generationID := uuid.New().String()
	system := buildSynthSystemPrompt()
	user := buildSynthUserPrompt(cmd.TrackID, cmd.SessionNumber, generationID, cm, questions, evaluations)

	chunks := h.client.Stream(ctx, synthModel, system, []llm.Message{
		{Role: "user", Content: user},
	}, synthMaxTokens)

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

	synthesis, err := parseSynthesis(sb.String(), cmd.SessionNumber, generationID, cm)
	if err != nil {
		return fmt.Errorf("generate synthesis: parse: %w", err)
	}
	b, err := json.MarshalIndent(synthesis, "", "  ")
	if err != nil {
		return fmt.Errorf("generate synthesis: marshal: %w", err)
	}
	if err := h.store.WriteSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "04_synthesis.json", b); err != nil {
		return fmt.Errorf("generate synthesis: write: %w", err)
	}
	h.rec.Record(metrics.Event{
		Event:      "synthesis_generated",
		TrackID:    cmd.TrackID,
		SessionNum: cmd.SessionNumber,
	})

	out <- llm.Chunk{Done: true}
	return nil
}

// ─── prompt builders ──────────────────────────────────────────────────────────

func buildSynthSystemPrompt() string {
	return `You are a learning analytics engine for the Axon adaptive system.
Given a session's evaluations and the current concept map, produce a synthesis.
Output ONLY JSON — no prose, no markdown fences.

Schema:
{
  "concept_map_updates": [
    {
      "concept_index": 0,
      "bloom_current_before": 2,
      "bloom_current_after": 3,
      "spaced_repetition": {
        "next_review": "2026-04-21",
        "interval_days": 7,
        "consecutive_correct": 1
      }
    }
  ],
  "learner_summary": "2-4 sentence plain-English summary of session performance and next steps."
}

Rules for bloom_current_after:
- Advance by 1 if correctness ≥ 0.75 across ≥ 2 questions for this concept
- Drop by 1 (min 1) if correctness < 0.4 and misconception identified
- Otherwise keep the same
- Never exceed bloom_target or 6
- Only include concepts that were actually tested (appeared in concept_indexes)

Rules for spaced_repetition:
- next_review: today + interval_days (ISO date string, YYYY-MM-DD)
- interval_days: 3 if bloom dropped, 7 if unchanged, 14 if advanced
- consecutive_correct: increment from current value if correct, reset to 0 if not`
}

func buildSynthUserPrompt(
	trackID string, sessionNum int, generationID string,
	cm domain.ConceptMap, questions []domain.Question, evaluations []domain.Evaluation,
) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Track: %s | Session: %d | Generation: %s\n", trackID, sessionNum, generationID)
	fmt.Fprintf(&sb, "Today's date: %s\n\n", time.Now().Format("2006-01-02"))

	// Concept map summary — only tested concepts.
	testedIndexes := map[int]bool{}
	for _, e := range evaluations {
		for _, idx := range e.ConceptIndexes {
			testedIndexes[idx] = true
		}
	}

	sb.WriteString("Concept Map (tested concepts only):\n")
	conceptByIdx := make(map[int]domain.Concept, len(cm.Concepts))
	for _, c := range cm.Concepts {
		conceptByIdx[c.Index] = c
	}
	for idx := range testedIndexes {
		c, ok := conceptByIdx[idx]
		if !ok {
			continue
		}
		fmt.Fprintf(&sb, "  [%d] %s (bloom: %d→%d, consecutive_correct: %d)\n",
			c.Index, c.Name, c.BloomCurrent, c.BloomTarget,
			c.SpacedRepetition.ConsecutiveCorrect)
	}

	// Per-concept evaluation summary.
	sb.WriteString("\nPer-concept results:\n")
	type conceptStats struct {
		totalCorrectness float64
		count            int
		misconception    bool
	}
	stats := map[int]*conceptStats{}
	for _, e := range evaluations {
		for _, idx := range e.ConceptIndexes {
			if _, ok := stats[idx]; !ok {
				stats[idx] = &conceptStats{}
			}
			s := stats[idx]
			s.totalCorrectness += e.Correctness
			s.count++
			if e.MisconceptionIdentified != nil {
				s.misconception = true
			}
		}
	}
	for idx, s := range stats {
		avg := s.totalCorrectness / float64(s.count)
		fmt.Fprintf(&sb, "  [%d] avg_correctness=%.2f n=%d misconception=%v\n",
			idx, avg, s.count, s.misconception)
	}

	// Full evaluations.
	sb.WriteString("\nFull evaluations:\n")
	for _, e := range evaluations {
		fmt.Fprintf(&sb, "  %s: correctness=%.2f bloom_demonstrated=%d calibration=%v error=%v\n",
			e.QuestionID, e.Correctness, e.BloomLevelDemonstrated,
			e.CalibrationFlag, e.ErrorTaxonomy)
		if e.FeedbackForLearner != "" {
			fmt.Fprintf(&sb, "    feedback: %s\n", e.FeedbackForLearner)
		}
	}

	return sb.String()
}

// ─── parser ───────────────────────────────────────────────────────────────────

// llmSpacedRepetition mirrors domain.SpacedRepetition but accepts date-only
// strings ("2026-04-23") emitted by the LLM in addition to full RFC3339.
type llmSpacedRepetition struct {
	NextReview         string `json:"next_review"`
	IntervalDays       int    `json:"interval_days"`
	ConsecutiveCorrect int    `json:"consecutive_correct"`
}

func (sr llmSpacedRepetition) toDomain() domain.SpacedRepetition {
	out := domain.SpacedRepetition{
		IntervalDays:       sr.IntervalDays,
		ConsecutiveCorrect: sr.ConsecutiveCorrect,
	}
	if sr.NextReview != "" {
		// Try RFC3339 first, then date-only.
		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			if t, err := time.Parse(layout, sr.NextReview); err == nil {
				out.NextReview = &t
				break
			}
		}
	}
	return out
}

type llmConceptMapUpdate struct {
	ConceptIndex       int                 `json:"concept_index"`
	BloomCurrentBefore int                 `json:"bloom_current_before"`
	BloomCurrentAfter  int                 `json:"bloom_current_after"`
	SpacedRepetition   llmSpacedRepetition `json:"spaced_repetition"`
}

type llmSynthesis struct {
	ConceptMapUpdates []llmConceptMapUpdate `json:"concept_map_updates"`
	LearnerSummary    string                `json:"learner_summary"`
}

func parseSynthesis(raw string, sessionNum int, generationID string, cm domain.ConceptMap) (domain.Synthesis, error) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "```") {
		if first := strings.Index(s, "\n"); first >= 0 {
			s = s[first+1:]
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}

	var ls llmSynthesis
	if err := json.Unmarshal([]byte(s), &ls); err != nil {
		return domain.Synthesis{}, fmt.Errorf("unmarshal: %w (raw prefix: %.200s)", err, s)
	}

	// Enrich each update with bloom_current_before from the live concept map,
	// then convert to domain type (resolving the date-only next_review string).
	conceptByIdx := make(map[int]domain.Concept, len(cm.Concepts))
	for _, c := range cm.Concepts {
		conceptByIdx[c.Index] = c
	}
	domainUpdates := make([]domain.ConceptMapUpdate, len(ls.ConceptMapUpdates))
	for i, u := range ls.ConceptMapUpdates {
		if c, ok := conceptByIdx[u.ConceptIndex]; ok {
			u.BloomCurrentBefore = c.BloomCurrent
			// Clamp after to target.
			if u.BloomCurrentAfter > c.BloomTarget {
				u.BloomCurrentAfter = c.BloomTarget
			}
			if u.BloomCurrentAfter < 1 {
				u.BloomCurrentAfter = 1
			}
		}
		domainUpdates[i] = domain.ConceptMapUpdate{
			ConceptIndex:       u.ConceptIndex,
			BloomCurrentBefore: u.BloomCurrentBefore,
			BloomCurrentAfter:  u.BloomCurrentAfter,
			SpacedRepetition:   u.SpacedRepetition.toDomain(),
		}
	}

	return domain.Synthesis{
		SessionNumber:     sessionNum,
		SessionDate:       time.Now().Format("2006-01-02"),
		GenerationID:      generationID,
		ConceptMapUpdates: domainUpdates,
		LearnerSummary:    ls.LearnerSummary,
		Applied:           false,
	}, nil
}
