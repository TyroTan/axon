package queries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

type GetSynthesisQuery struct {
	TrackID       string
	SessionNumber int
}

type GetSynthesisResult struct {
	TrackID       string          `json:"track_id"`
	SessionNumber int             `json:"session_number"`
	Synthesis     *domain.Synthesis `json:"synthesis"` // nil if not yet generated
}

type GetSynthesisHandler struct {
	store *filesystem.TrackStore
}

func NewGetSynthesisHandler(store *filesystem.TrackStore) *GetSynthesisHandler {
	return &GetSynthesisHandler{store: store}
}

func (h *GetSynthesisHandler) Handle(ctx context.Context, q GetSynthesisQuery) (GetSynthesisResult, error) {
	b, err := h.store.ReadSessionFile(ctx, q.TrackID, q.SessionNumber, "04_synthesis.json")
	if err != nil {
		return GetSynthesisResult{}, fmt.Errorf("get synthesis: %w", err)
	}
	result := GetSynthesisResult{TrackID: q.TrackID, SessionNumber: q.SessionNumber}
	if b != nil {
		var synthesis domain.Synthesis
		if err := json.Unmarshal(b, &synthesis); err != nil {
			return GetSynthesisResult{}, fmt.Errorf("get synthesis: parse: %w", err)
		}
		result.Synthesis = &synthesis
	}
	return result, nil
}
