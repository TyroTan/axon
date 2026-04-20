package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/tyrohunt/axon/internal/config"
	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// aspirationThreshold is the number of above-floor engagements required to
// auto-set exploration_unlocked on a concept.
const aspirationThreshold = 2

// deltaMultiplier converts a generation-vs-answer effective score delta into a
// bloom gain multiplier. Positive delta = session was generated harder than the
// learner's current state → extra reward. Negative = easier than current state → reduced.
func deltaMultiplier(delta int) float64 {
	switch {
	case delta >= 3:
		return 1.5
	case delta == 2:
		return 1.3
	case delta == 1:
		return 1.15
	case delta == 0:
		return 1.0
	case delta == -1:
		return 0.9
	default:
		return 0.8
	}
}

// ApplySynthesisCommand reads 04_synthesis.json and patches concept_map.json.
// Updates two state planes in a single write:
//   - Evidence state: bloom_current + spaced_repetition (from synthesis)
//   - Exploration state: aspiration_count increments + exploration_unlocked
//     auto-set when threshold reached (from session questions + evaluations)
//
// Idempotent: if synthesis.applied is already true, it's a no-op.
type ApplySynthesisCommand struct {
	TrackID       string
	SessionNumber int
}

type ApplySynthesisHandler struct {
	store *filesystem.TrackStore
}

func NewApplySynthesisHandler(store *filesystem.TrackStore) *ApplySynthesisHandler {
	return &ApplySynthesisHandler{store: store}
}

func (h *ApplySynthesisHandler) Handle(ctx context.Context, cmd ApplySynthesisCommand) error {
	// Load synthesis.
	b, err := h.store.ReadSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "04_synthesis.json")
	if err != nil {
		return fmt.Errorf("apply synthesis: load: %w", err)
	}
	if b == nil {
		return fmt.Errorf("apply synthesis: no synthesis found — run synthesis first")
	}
	var synthesis domain.Synthesis
	if err := json.Unmarshal(b, &synthesis); err != nil {
		return fmt.Errorf("apply synthesis: parse: %w", err)
	}

	// Idempotency guard.
	if synthesis.Applied {
		return nil
	}

	// Load concept map.
	cm, err := h.store.GetConceptMap(ctx, cmd.TrackID)
	if err != nil {
		return fmt.Errorf("apply synthesis: concept map: %w", err)
	}

	// Build a slot index for O(1) concept lookup.
	slotByIdx := make(map[int]int, len(cm.Concepts))
	for i, c := range cm.Concepts {
		slotByIdx[c.Index] = i
	}

	// ── Delta multiplier — path-aware reward scaling ─────────────────────────
	// Compare generation-time effective score vs current answer-time score.
	// If the learner answered a session that was generated harder than their
	// current state, they deserve extra bloom credit. Conversely, if they
	// answered something easier than their current state, bloom gain is reduced.
	meta, _ := h.store.ReadSessionMetadata(ctx, cmd.TrackID, cmd.SessionNumber)
	currentLearnerSignal := h.store.CurrentLearnerSignal(ctx, cmd.TrackID)
	currentConfigScore := config.ActiveLevel().Score
	currentEffective := config.EffectiveScore(currentConfigScore, currentLearnerSignal)

	var generationEffective int
	if meta.StateSnapshot != nil {
		generationEffective = meta.StateSnapshot.EffectiveScore
	} else {
		generationEffective = currentEffective // no snapshot = no delta
	}

	delta := generationEffective - currentEffective
	multiplier := deltaMultiplier(delta)
	synthesis.DeltaMultiplier = multiplier
	synthesis.LearnerSignal = currentLearnerSignal

	// Append evaluated path entry.
	_ = h.store.AppendPathEntry(ctx, domain.PathEntry{
		TrackID:         cmd.TrackID,
		SessionNum:      cmd.SessionNumber,
		Event:           "evaluated",
		VisitedAt:       time.Now(),
		AxonConfigScore: currentConfigScore,
		LearnerSignal:   currentLearnerSignal,
		EffectiveScore:  currentEffective,
	})

	// ── Evidence state: bloom_current + spaced_repetition ────────────────────
	for _, u := range synthesis.ConceptMapUpdates {
		slot, ok := slotByIdx[u.ConceptIndex]
		if !ok {
			continue
		}
		// Apply delta multiplier: scale the bloom gain, not the absolute value.
		gain := u.BloomCurrentAfter - u.BloomCurrentBefore
		if gain > 0 {
			scaledGain := int(math.Round(float64(gain) * multiplier))
			if scaledGain < 1 {
				scaledGain = 1 // always get at least 1 if synthesis says improvement
			}
			u.BloomCurrentAfter = u.BloomCurrentBefore + scaledGain
			if u.BloomCurrentAfter > cm.Concepts[slot].BloomTarget {
				u.BloomCurrentAfter = cm.Concepts[slot].BloomTarget
			}
		}
		cm.Concepts[slot].BloomCurrent = u.BloomCurrentAfter
		cm.Concepts[slot].SpacedRepetition = u.SpacedRepetition
	}

	// ── Exploration state: aspiration_count + auto-unlock ────────────────────
	// Compare answered questions against the post-synthesis bloom_current floor.
	// A question asked above a concept's demonstrated level = aspirational stretch.
	// Above-floor attempts accumulate; at threshold the concept is provisionally
	// unlocked for future sessions (exploration state only, never used in scoring).
	qb, _ := h.store.ReadSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "01_questions.json")
	eb, _ := h.store.ReadSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "03_evaluations.json")
	if qb != nil && eb != nil {
		var questions []domain.Question
		var evaluations []domain.Evaluation
		if json.Unmarshal(qb, &questions) == nil && json.Unmarshal(eb, &evaluations) == nil {
			questionByID := make(map[string]domain.Question, len(questions))
			for _, q := range questions {
				questionByID[q.ID] = q
			}
			for _, e := range evaluations {
				q, ok := questionByID[e.QuestionID]
				if !ok {
					continue
				}
				for _, ci := range q.ConceptIndexes {
					slot, exists := slotByIdx[ci]
					if !exists {
						continue
					}
					if q.BloomLevel > cm.Concepts[slot].BloomCurrent {
						cm.Concepts[slot].AspirationCount++
					}
				}
			}
			for i := range cm.Concepts {
				if !cm.Concepts[i].ExplorationUnlocked &&
					cm.Concepts[i].AspirationCount >= aspirationThreshold {
					cm.Concepts[i].ExplorationUnlocked = true
				}
			}
		}
	}

	// Single write — both evidence and exploration state together.
	if err := h.store.WriteConceptMap(ctx, cmd.TrackID, cm); err != nil {
		return fmt.Errorf("apply synthesis: write concept map: %w", err)
	}

	// Mark synthesis as applied and re-persist.
	synthesis.Applied = true
	sb, err := json.MarshalIndent(synthesis, "", "  ")
	if err != nil {
		return fmt.Errorf("apply synthesis: marshal: %w", err)
	}
	return h.store.WriteSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "04_synthesis.json", sb)
}
