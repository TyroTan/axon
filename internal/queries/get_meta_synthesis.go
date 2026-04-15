package queries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

type GetMetaSynthesisQuery struct {
	TrackID string
}

type GetMetaSynthesisResult struct {
	TrackID       string                 `json:"track_id"`
	MetaSynthesis *domain.MetaSynthesis  `json:"meta_synthesis"` // null if not yet generated
}

type GetMetaSynthesisHandler struct {
	store *filesystem.TrackStore
}

func NewGetMetaSynthesisHandler(store *filesystem.TrackStore) *GetMetaSynthesisHandler {
	return &GetMetaSynthesisHandler{store: store}
}

func (h *GetMetaSynthesisHandler) Handle(ctx context.Context, q GetMetaSynthesisQuery) (GetMetaSynthesisResult, error) {
	b, err := h.store.ReadTrackFile(ctx, q.TrackID, "meta_synthesis.json")
	if err != nil {
		return GetMetaSynthesisResult{}, fmt.Errorf("get meta-synthesis: %w", err)
	}
	result := GetMetaSynthesisResult{TrackID: q.TrackID}
	if b != nil {
		var ms domain.MetaSynthesis
		if err := json.Unmarshal(b, &ms); err != nil {
			return GetMetaSynthesisResult{}, fmt.Errorf("get meta-synthesis: parse: %w", err)
		}
		result.MetaSynthesis = &ms
	}
	return result, nil
}
