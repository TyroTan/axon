package queries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

type GetSessionQuestionsQuery struct {
	TrackID       string
	SessionNumber int
}

type GetSessionQuestionsResult struct {
	TrackID       string            `json:"track_id"`
	SessionNumber int               `json:"session_number"`
	Questions     []domain.Question `json:"questions"`
}

type GetSessionQuestionsHandler struct {
	store *filesystem.TrackStore
}

func NewGetSessionQuestionsHandler(store *filesystem.TrackStore) *GetSessionQuestionsHandler {
	return &GetSessionQuestionsHandler{store: store}
}

func (h *GetSessionQuestionsHandler) Handle(ctx context.Context, q GetSessionQuestionsQuery) (GetSessionQuestionsResult, error) {
	b, err := h.store.ReadSessionFile(ctx, q.TrackID, q.SessionNumber, "01_questions.json")
	if err != nil {
		return GetSessionQuestionsResult{}, fmt.Errorf("get session questions: %w", err)
	}
	result := GetSessionQuestionsResult{TrackID: q.TrackID, SessionNumber: q.SessionNumber, Questions: []domain.Question{}}
	if b != nil {
		if err := json.Unmarshal(b, &result.Questions); err != nil {
			return GetSessionQuestionsResult{}, fmt.Errorf("get session questions: parse: %w", err)
		}
	}
	return result, nil
}
