package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// aspirationThreshold is the number of above-floor engagements required to
// auto-set exploration_unlocked on a concept.
const aspirationThreshold = 2

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

	// ── Evidence state: bloom_current + spaced_repetition ────────────────────
	for _, u := range synthesis.ConceptMapUpdates {
		slot, ok := slotByIdx[u.ConceptIndex]
		if !ok {
			continue
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
