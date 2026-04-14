package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// SubmitResponsesCommand overwrites 02_responses.json with the given slice.
// The UI sends all responses at once after the learner finishes answering.
type SubmitResponsesCommand struct {
	TrackID       string
	SessionNumber int
	Responses     []domain.Response
}

type SubmitResponsesHandler struct {
	store *filesystem.TrackStore
}

func NewSubmitResponsesHandler(store *filesystem.TrackStore) *SubmitResponsesHandler {
	return &SubmitResponsesHandler{store: store}
}

func (h *SubmitResponsesHandler) Handle(ctx context.Context, cmd SubmitResponsesCommand) error {
	if len(cmd.Responses) == 0 {
		return fmt.Errorf("submit responses: no responses provided")
	}
	b, err := json.MarshalIndent(cmd.Responses, "", "  ")
	if err != nil {
		return fmt.Errorf("submit responses: marshal: %w", err)
	}
	return h.store.WriteSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "02_responses.json", b)
}
