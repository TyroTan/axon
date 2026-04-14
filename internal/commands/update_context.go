package commands

import (
	"context"
	"fmt"

	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// UpdateContextCommand saves a single context file for a track.
type UpdateContextCommand struct {
	TrackID  string
	Filename string // e.g. "my_docs.md"
	Content  string
}

type UpdateContextHandler struct {
	store *filesystem.TrackStore
}

func NewUpdateContextHandler(store *filesystem.TrackStore) *UpdateContextHandler {
	return &UpdateContextHandler{store: store}
}

func (h *UpdateContextHandler) Handle(ctx context.Context, cmd UpdateContextCommand) error {
	if cmd.Filename == "" {
		return fmt.Errorf("update context: filename required")
	}
	return h.store.WriteContextFile(ctx, cmd.TrackID, cmd.Filename, cmd.Content)
}
