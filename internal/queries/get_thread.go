package queries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

type GetThreadQuery struct {
	TrackID       string
	SessionNumber int
	QuestionID    string
}

type GetThreadResult struct {
	TrackID       string        `json:"track_id"`
	SessionNumber int           `json:"session_number"`
	Thread        *domain.Thread `json:"thread"` // nil if no conversation started yet
}

type GetThreadHandler struct {
	store *filesystem.TrackStore
}

func NewGetThreadHandler(store *filesystem.TrackStore) *GetThreadHandler {
	return &GetThreadHandler{store: store}
}

func (h *GetThreadHandler) Handle(ctx context.Context, q GetThreadQuery) (GetThreadResult, error) {
	filename := threadFilename(q.QuestionID)
	b, err := h.store.ReadSessionFile(ctx, q.TrackID, q.SessionNumber, filename)
	if err != nil {
		return GetThreadResult{}, fmt.Errorf("get thread: %w", err)
	}
	if b == nil {
		return GetThreadResult{TrackID: q.TrackID, SessionNumber: q.SessionNumber, Thread: nil}, nil
	}
	var thread domain.Thread
	if err := json.Unmarshal(b, &thread); err != nil {
		return GetThreadResult{}, fmt.Errorf("get thread: unmarshal: %w", err)
	}
	return GetThreadResult{TrackID: q.TrackID, SessionNumber: q.SessionNumber, Thread: &thread}, nil
}

// threadFilename maps a question ID to its thread filename.
// "q_1" → "threads/q_1.json"
func threadFilename(questionID string) string {
	return "threads/" + questionID + ".json"
}
