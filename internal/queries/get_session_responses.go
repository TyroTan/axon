package queries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

type GetSessionResponsesQuery struct {
	TrackID       string
	SessionNumber int
}

type GetSessionResponsesResult struct {
	TrackID       string            `json:"track_id"`
	SessionNumber int               `json:"session_number"`
	Responses     []domain.Response `json:"responses"`
}

type GetSessionResponsesHandler struct {
	store *filesystem.TrackStore
}

func NewGetSessionResponsesHandler(store *filesystem.TrackStore) *GetSessionResponsesHandler {
	return &GetSessionResponsesHandler{store: store}
}

func (h *GetSessionResponsesHandler) Handle(ctx context.Context, q GetSessionResponsesQuery) (GetSessionResponsesResult, error) {
	b, err := h.store.ReadSessionFile(ctx, q.TrackID, q.SessionNumber, "02_responses.json")
	if err != nil {
		return GetSessionResponsesResult{}, fmt.Errorf("get session responses: %w", err)
	}
	var responses []domain.Response
	if err := json.Unmarshal(b, &responses); err != nil {
		return GetSessionResponsesResult{}, fmt.Errorf("get session responses: parse: %w", err)
	}
	return GetSessionResponsesResult{
		TrackID:       q.TrackID,
		SessionNumber: q.SessionNumber,
		Responses:     responses,
	}, nil
}
