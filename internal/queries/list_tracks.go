package queries

import (
	"context"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// ListTracksQuery returns the full track tree.
type ListTracksQuery struct{}

// ListTracksResult is the query result.
type ListTracksResult struct {
	Tracks []domain.Track `json:"tracks"`
}

// ListTracksHandler satisfies cqrs.QueryHandler[ListTracksQuery, ListTracksResult].
type ListTracksHandler struct {
	store *filesystem.TrackStore
}

func NewListTracksHandler(store *filesystem.TrackStore) *ListTracksHandler {
	return &ListTracksHandler{store: store}
}

func (h *ListTracksHandler) Handle(ctx context.Context, _ ListTracksQuery) (ListTracksResult, error) {
	tracks, err := h.store.ListTracks(ctx)
	if err != nil {
		return ListTracksResult{}, err
	}
	return ListTracksResult{Tracks: tracks}, nil
}
