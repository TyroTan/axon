package queries

import (
	"context"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// GetTrackQuery fetches one track with its concept map and sessions.
type GetTrackQuery struct {
	TrackID string
}

type GetTrackResult struct {
	Track              domain.Track      `json:"track"`
	ConceptMap         domain.ConceptMap `json:"concept_map"`
	Sessions           []domain.Session  `json:"sessions"`
	HasDistillSnapshot bool              `json:"has_distill_snapshot"` // session_insights.snapshot.md exists in own context
}

type GetTrackHandler struct {
	store *filesystem.TrackStore
}

func NewGetTrackHandler(store *filesystem.TrackStore) *GetTrackHandler {
	return &GetTrackHandler{store: store}
}

func (h *GetTrackHandler) Handle(ctx context.Context, q GetTrackQuery) (GetTrackResult, error) {
	track, err := h.store.GetTrack(ctx, q.TrackID)
	if err != nil {
		return GetTrackResult{}, err
	}
	cm, err := h.store.GetConceptMap(ctx, q.TrackID)
	if err != nil {
		return GetTrackResult{}, err
	}
	sessions, err := h.store.ListSessions(ctx, q.TrackID)
	if err != nil {
		return GetTrackResult{}, err
	}
	ownFiles, _ := h.store.ReadContextFiles(ctx, q.TrackID)
	_, hasSnapshot := ownFiles["session_insights.snapshot.md"]
	return GetTrackResult{Track: track, ConceptMap: cm, Sessions: sessions, HasDistillSnapshot: hasSnapshot}, nil
}
