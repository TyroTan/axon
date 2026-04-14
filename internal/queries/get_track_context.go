package queries

import (
	"context"

	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// GetTrackContextQuery returns the context/ file contents for a track.
type GetTrackContextQuery struct {
	TrackID string
}

type GetTrackContextResult struct {
	TrackID string            `json:"track_id"`
	Files   map[string]string `json:"files"` // filename → content
}

type GetTrackContextHandler struct {
	store *filesystem.TrackStore
}

func NewGetTrackContextHandler(store *filesystem.TrackStore) *GetTrackContextHandler {
	return &GetTrackContextHandler{store: store}
}

func (h *GetTrackContextHandler) Handle(ctx context.Context, q GetTrackContextQuery) (GetTrackContextResult, error) {
	files, err := h.store.ReadContextFiles(ctx, q.TrackID)
	if err != nil {
		return GetTrackContextResult{}, err
	}
	return GetTrackContextResult{TrackID: q.TrackID, Files: files}, nil
}
