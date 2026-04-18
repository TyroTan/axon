package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// DuplicateTrackCommand creates a sibling track (e.g. track_1 → track_1_2).
// The new track inherits the parent's concept_map.json with all bloom_current
// values preserved, and its context/ folder is pre-populated with a note
// pointing back to the source track.
type DuplicateTrackCommand struct {
	SourceTrackID string // e.g. "track_1" or "track_1_2"
}

// DuplicateTrackResult is returned after successful duplication.
type DuplicateTrackResult struct {
	NewTrackID string `json:"new_track_id"`
}

// DuplicateTrackHandler satisfies cqrs.CommandHandler.
// Returns the new track ID via a result field on the command struct —
// commands are fire-and-forget in the bus, so callers read NewTrackID
// from the result stored in the handler after dispatch.
type DuplicateTrackHandler struct {
	store *filesystem.TrackStore
}

func NewDuplicateTrackHandler(store *filesystem.TrackStore) *DuplicateTrackHandler {
	return &DuplicateTrackHandler{store: store}
}

func (h *DuplicateTrackHandler) Handle(ctx context.Context, cmd DuplicateTrackCommand) error {
	// Resolve source track.
	source, err := h.store.GetTrack(ctx, cmd.SourceTrackID)
	if err != nil {
		return fmt.Errorf("duplicate track: source %q not found: %w", cmd.SourceTrackID, err)
	}

	// Determine parent: siblings share the same parent as the source.
	// track_1     → parent ""       → siblings are track_1, track_2, ...
	//                                  new sibling = track_N_2 where track_N = source
	// track_1_2   → parent "track_1" → siblings are track_1_2, track_1_3, ...
	//
	// The convention: duplicating always creates a child of the source's *own* ID.
	// track_1   → track_1_2 (first child)
	// track_1_2 → track_1_2_2 (first child of track_1_2)
	// This makes the tree reflect lineage clearly.
	newID, err := h.store.NextTrackID(cmd.SourceTrackID)
	if err != nil {
		return fmt.Errorf("duplicate track: %w", err)
	}

	// Copy concept map from source (bloom_current state preserved).
	cm, err := h.store.GetConceptMap(ctx, cmd.SourceTrackID)
	if err != nil {
		return fmt.Errorf("duplicate track: read concept map: %w", err)
	}
	cm.Track = newID
	cm.GeneratedAt = time.Now().Format("2006-01-02")
	cm.Note = fmt.Sprintf("Inherited from %s on %s. bloom_current preserved.", cmd.SourceTrackID, cm.GeneratedAt)

	// Build new track domain object.
	newTrack := domain.Track{
		ID:        newID,
		ParentID:  cmd.SourceTrackID,
		Branches:  source.Branches,
		CreatedAt: time.Now(),
	}

	// Create the directory structure + write files.
	if err := h.store.CreateTrack(ctx, newTrack, cm); err != nil {
		return fmt.Errorf("duplicate track: create: %w", err)
	}

	// Write a _sources.md stub in context/ explaining the lineage.
	sourcesContent := fmt.Sprintf(`# Context Sources — %s

> Inherited from: **%s**
> Duplicated on: %s
>
> This track was created by duplicating %s. The concept map bloom_current
> values were carried over. Edit the context files below and run prompt 01
> to regenerate the profile, or start a session directly.

## Sources

| Snapshot filename | Source | Purpose |
|---|---|---|
| _(add snapshots here)_ | | |

## Inherited concept map

The concept map was copied from **%s** with all bloom_current values preserved.
To reset to a blank slate, set all bloom_current to 1 manually in concept_map.json.
`,
		newID, cmd.SourceTrackID, time.Now().Format("2006-01-02"),
		cmd.SourceTrackID, cmd.SourceTrackID,
	)

	if err := h.store.WriteContextFile(ctx, newID, "_sources.md", sourcesContent); err != nil {
		return fmt.Errorf("duplicate track: write _sources.md: %w", err)
	}

	// Copy prompts/ from source track into new track.
	// prompts/ contains the human-readable system prompt templates that mirror
	// the backend logic — they travel with the track so the user can inspect
	// what the backend is doing and adapt prompts for manual bootstrapping.
	srcPrompts := filepath.Join(h.store.ExperimentsDir(), cmd.SourceTrackID, "prompts")
	entries, err := os.ReadDir(srcPrompts)
	if err == nil { // non-fatal — source may have an empty prompts dir
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			content, err := os.ReadFile(filepath.Join(srcPrompts, e.Name()))
			if err != nil {
				continue // skip unreadable file, don't abort the whole duplicate
			}
			_ = h.store.WriteTrackFile(ctx, newID, filepath.Join("prompts", e.Name()), content)
		}
	}

	return nil
}

