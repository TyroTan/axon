package queries

import (
	"context"
	"strings"

	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// GetTrackContextQuery returns the context/ file contents for a track.
type GetTrackContextQuery struct {
	TrackID string
}

// GetTrackContextResult separates a track's own (editable) files from files
// inherited from ancestor tracks (read-only, shown for visibility).
type GetTrackContextResult struct {
	TrackID        string            `json:"track_id"`
	Files          map[string]string `json:"files"`           // own — editable
	InheritedFiles map[string]string `json:"inherited_files"` // from ancestors — read-only
	InheritedFrom  map[string]string `json:"inherited_from"`  // filename → source track ID
}

type GetTrackContextHandler struct {
	store *filesystem.TrackStore
}

func NewGetTrackContextHandler(store *filesystem.TrackStore) *GetTrackContextHandler {
	return &GetTrackContextHandler{store: store}
}

func (h *GetTrackContextHandler) Handle(ctx context.Context, q GetTrackContextQuery) (GetTrackContextResult, error) {
	ownFiles, err := h.store.ReadContextFiles(ctx, q.TrackID)
	if err != nil {
		return GetTrackContextResult{}, err
	}

	inheritedFiles := map[string]string{}
	inheritedFrom := map[string]string{}

	seen := make(map[string]bool, len(ownFiles))
	for name := range ownFiles {
		seen[name] = true
	}

	ancestorID := parentTrackID(q.TrackID)
	for ancestorID != "" {
		ancestorFiles, _ := h.store.ReadContextFiles(ctx, ancestorID)
		for name, content := range ancestorFiles {
			if !seen[name] {
				inheritedFiles[name] = content
				inheritedFrom[name] = ancestorID
				seen[name] = true
			}
		}
		ancestorID = parentTrackID(ancestorID)
	}

	return GetTrackContextResult{
		TrackID:        q.TrackID,
		Files:          ownFiles,
		InheritedFiles: inheritedFiles,
		InheritedFrom:  inheritedFrom,
	}, nil
}

func parentTrackID(id string) string {
	parts := strings.Split(id, "_")
	if len(parts) <= 2 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], "_")
}
