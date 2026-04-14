package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// ApplySynthesisCommand reads 04_synthesis.json and patches concept_map.json
// bloom_current + spaced_repetition for each updated concept.
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

	// Apply updates.
	updateByIdx := make(map[int]domain.ConceptMapUpdate, len(synthesis.ConceptMapUpdates))
	for _, u := range synthesis.ConceptMapUpdates {
		updateByIdx[u.ConceptIndex] = u
	}
	for i := range cm.Concepts {
		u, ok := updateByIdx[cm.Concepts[i].Index]
		if !ok {
			continue
		}
		cm.Concepts[i].BloomCurrent = u.BloomCurrentAfter
		cm.Concepts[i].SpacedRepetition = u.SpacedRepetition
	}

	// Persist updated concept map.
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
