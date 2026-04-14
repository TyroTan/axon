package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// CreateSessionCommand allocates the next session directory for a track.
type CreateSessionCommand struct {
	TrackID string
}

type CreateSessionResult struct {
	SessionNumber int `json:"session_number"`
}

type CreateSessionHandler struct {
	store *filesystem.TrackStore
}

func NewCreateSessionHandler(store *filesystem.TrackStore) *CreateSessionHandler {
	return &CreateSessionHandler{store: store}
}

func (h *CreateSessionHandler) Handle(_ context.Context, cmd CreateSessionCommand) (CreateSessionResult, error) {
	n, err := filesystem.NextSessionNumber(h.store.ExperimentsDir(), cmd.TrackID)
	if err != nil {
		return CreateSessionResult{}, fmt.Errorf("create session: %w", err)
	}
	dir := filesystem.SessionDir(h.store.ExperimentsDir(), cmd.TrackID, n)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return CreateSessionResult{}, fmt.Errorf("create session: mkdir: %w", err)
	}
	return CreateSessionResult{SessionNumber: n}, nil
}
