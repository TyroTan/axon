package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// CreateSessionCommand allocates the next session directory for a track.
// ShardID is optional — set when a split plan exists and the user picked a shard.
type CreateSessionCommand struct {
	TrackID string
	ShardID string // empty → full context; non-empty → shard-filtered context
}

type CreateSessionResult struct {
	SessionNumber int    `json:"session_number"`
	ShardID       string `json:"shard_id,omitempty"`
}

type CreateSessionHandler struct {
	store *filesystem.TrackStore
}

func NewCreateSessionHandler(store *filesystem.TrackStore) *CreateSessionHandler {
	return &CreateSessionHandler{store: store}
}

func (h *CreateSessionHandler) Handle(ctx context.Context, cmd CreateSessionCommand) (CreateSessionResult, error) {
	n, err := filesystem.NextSessionNumber(h.store.ExperimentsDir(), cmd.TrackID)
	if err != nil {
		return CreateSessionResult{}, fmt.Errorf("create session: %w", err)
	}
	dir := filesystem.SessionDir(h.store.ExperimentsDir(), cmd.TrackID, n)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return CreateSessionResult{}, fmt.Errorf("create session: mkdir: %w", err)
	}
	// Always write metadata (ShardID may be empty for full-context sessions).
	if err := h.store.WriteSessionMetadata(ctx, cmd.TrackID, n, domain.SessionMetadata{
		ShardID: cmd.ShardID,
	}); err != nil {
		return CreateSessionResult{}, fmt.Errorf("create session: write metadata: %w", err)
	}
	return CreateSessionResult{SessionNumber: n, ShardID: cmd.ShardID}, nil
}
