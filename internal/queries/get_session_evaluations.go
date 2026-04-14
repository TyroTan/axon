package queries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

type GetSessionEvaluationsQuery struct {
	TrackID       string
	SessionNumber int
}

type GetSessionEvaluationsResult struct {
	TrackID       string              `json:"track_id"`
	SessionNumber int                 `json:"session_number"`
	Evaluations   []domain.Evaluation `json:"evaluations"`
}

type GetSessionEvaluationsHandler struct {
	store *filesystem.TrackStore
}

func NewGetSessionEvaluationsHandler(store *filesystem.TrackStore) *GetSessionEvaluationsHandler {
	return &GetSessionEvaluationsHandler{store: store}
}

func (h *GetSessionEvaluationsHandler) Handle(ctx context.Context, q GetSessionEvaluationsQuery) (GetSessionEvaluationsResult, error) {
	b, err := h.store.ReadSessionFile(ctx, q.TrackID, q.SessionNumber, "03_evaluations.json")
	if err != nil {
		return GetSessionEvaluationsResult{}, fmt.Errorf("get session evaluations: %w", err)
	}
	result := GetSessionEvaluationsResult{TrackID: q.TrackID, SessionNumber: q.SessionNumber, Evaluations: []domain.Evaluation{}}
	if b != nil {
		if err := json.Unmarshal(b, &result.Evaluations); err != nil {
			return GetSessionEvaluationsResult{}, fmt.Errorf("get session evaluations: parse: %w", err)
		}
	}
	return result, nil
}
