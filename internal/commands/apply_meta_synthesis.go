package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// ApplyMetaSynthesisCommand patches concept_map.json using meta_synthesis.json.
type ApplyMetaSynthesisCommand struct {
	TrackID string
}

type ApplyMetaSynthesisHandler struct {
	store *filesystem.TrackStore
}

func NewApplyMetaSynthesisHandler(store *filesystem.TrackStore) *ApplyMetaSynthesisHandler {
	return &ApplyMetaSynthesisHandler{store: store}
}

func (h *ApplyMetaSynthesisHandler) Handle(ctx context.Context, cmd ApplyMetaSynthesisCommand) error {
	b, err := h.store.ReadTrackFile(ctx, cmd.TrackID, "meta_synthesis.json")
	if err != nil || b == nil {
		return fmt.Errorf("apply meta-synthesis: no meta_synthesis.json found for %s", cmd.TrackID)
	}
	var ms domain.MetaSynthesis
	if err := json.Unmarshal(b, &ms); err != nil {
		return fmt.Errorf("apply meta-synthesis: parse: %w", err)
	}
	if ms.Applied {
		return nil // idempotent
	}

	cm, err := h.store.GetConceptMap(ctx, cmd.TrackID)
	if err != nil {
		return fmt.Errorf("apply meta-synthesis: concept map: %w", err)
	}

	conceptByIdx := make(map[int]*domain.Concept, len(cm.Concepts))
	for i := range cm.Concepts {
		conceptByIdx[cm.Concepts[i].Index] = &cm.Concepts[i]
	}
	for _, u := range ms.ConceptMapUpdates {
		c, ok := conceptByIdx[u.ConceptIndex]
		if !ok {
			continue
		}
		c.BloomCurrent = u.BloomCurrentAfter
		c.SpacedRepetition = u.SpacedRepetition
	}

	if err := h.store.WriteConceptMap(ctx, cmd.TrackID, cm); err != nil {
		return fmt.Errorf("apply meta-synthesis: write concept map: %w", err)
	}

	ms.Applied = true
	out, err := json.MarshalIndent(ms, "", "  ")
	if err != nil {
		return fmt.Errorf("apply meta-synthesis: marshal: %w", err)
	}
	return h.store.WriteTrackFile(ctx, cmd.TrackID, "meta_synthesis.json", out)
}
