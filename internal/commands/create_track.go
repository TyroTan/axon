package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// CreateTrackCommand creates a new root track (e.g. "track_2").
//
// Two modes:
//   - Blank: Branches provided, SourceTrackID empty. Concept map seeded with no concepts.
//   - Clone: SourceTrackID provided. Concept map + context files physically copied from
//     source into the new root track. Branches inferred from source if not provided.
type CreateTrackCommand struct {
	// Branches — major knowledge branches. Required for blank mode.
	// Optional for clone mode (inferred from source concept map if empty).
	Branches []string
	// SourceTrackID — if set, clone this track's concept map and context into the new root.
	// The new track gets a root-level ID (track_2, not track_1_2) and no parent.
	SourceTrackID string
}

type CreateTrackResult struct {
	NewTrackID string `json:"new_track_id"`
}

type CreateTrackHandler struct {
	store *filesystem.TrackStore
}

func NewCreateTrackHandler(store *filesystem.TrackStore) *CreateTrackHandler {
	return &CreateTrackHandler{store: store}
}

func (h *CreateTrackHandler) Handle(ctx context.Context, cmd CreateTrackCommand) (CreateTrackResult, error) {
	newID, err := h.store.NextTrackID("")
	if err != nil {
		return CreateTrackResult{}, fmt.Errorf("create track: %w", err)
	}

	// ── Clone mode ────────────────────────────────────────────────────────────
	if cmd.SourceTrackID != "" {
		return h.clone(ctx, cmd, newID)
	}

	// ── Blank mode ────────────────────────────────────────────────────────────
	if len(cmd.Branches) == 0 {
		return CreateTrackResult{}, fmt.Errorf("create track: at least one branch required")
	}
	return h.blank(ctx, cmd.Branches, newID)
}

func (h *CreateTrackHandler) blank(ctx context.Context, branches []string, newID string) (CreateTrackResult, error) {
	cm := domain.ConceptMap{
		Track:         newID,
		MajorBranches: branches,
		GeneratedAt:   time.Now().Format("2006-01-02"),
		Note: "Seed map — no concepts yet. Run prompts/00_concept_map_generator.md " +
			"(paste as system prompt in Claude with branch names as user message) to populate.",
		Concepts: []domain.Concept{},
	}
	t := domain.Track{ID: newID, Branches: branches, CreatedAt: time.Now()}
	if err := h.store.CreateTrack(ctx, t, cm); err != nil {
		return CreateTrackResult{}, fmt.Errorf("create track: %w", err)
	}
	sources := fmt.Sprintf("# Context Sources — %s\n\n> Root track created on %s.\n> Branches: %s\n\n## Sources\n\n| Snapshot filename | Source | Purpose |\n|---|---|---|\n| _(add snapshots here)_ | | |\n",
		newID, time.Now().Format("2006-01-02"), strings.Join(branches, " · "))
	if err := h.store.WriteContextFile(ctx, newID, "_sources.md", sources); err != nil {
		return CreateTrackResult{}, fmt.Errorf("create track: write _sources.md: %w", err)
	}
	return CreateTrackResult{NewTrackID: newID}, nil
}

func (h *CreateTrackHandler) clone(ctx context.Context, cmd CreateTrackCommand, newID string) (CreateTrackResult, error) {
	// Read source concept map.
	cm, err := h.store.GetConceptMap(ctx, cmd.SourceTrackID)
	if err != nil {
		return CreateTrackResult{}, fmt.Errorf("create track (clone): read source concept map: %w", err)
	}
	branches := cmd.Branches
	if len(branches) == 0 {
		branches = cm.MajorBranches
	}
	cm.Track = newID
	cm.GeneratedAt = time.Now().Format("2006-01-02")
	cm.Note = fmt.Sprintf("Cloned from %s on %s. bloom_current preserved.", cmd.SourceTrackID, cm.GeneratedAt)

	t := domain.Track{ID: newID, Branches: branches, CreatedAt: time.Now()}
	if err := h.store.CreateTrack(ctx, t, cm); err != nil {
		return CreateTrackResult{}, fmt.Errorf("create track (clone): create: %w", err)
	}

	// Copy context files from source (own context/ only — not cascaded ancestors).
	contextFiles, err := h.store.ReadContextFiles(ctx, cmd.SourceTrackID)
	if err != nil {
		return CreateTrackResult{}, fmt.Errorf("create track (clone): read context: %w", err)
	}
	for filename, content := range contextFiles {
		if filename == "_sources.md" {
			continue // will write our own below
		}
		if err := h.store.WriteContextFile(ctx, newID, filename, content); err != nil {
			return CreateTrackResult{}, fmt.Errorf("create track (clone): write context %s: %w", filename, err)
		}
	}

	// Write _sources.md documenting the clone origin.
	sources := fmt.Sprintf("# Context Sources — %s\n\n> Cloned from **%s** on %s.\n> Context files copied as snapshot (not inherited via cascade — this is a root track).\n\n## Sources\n\n| Snapshot filename | Source | Purpose |\n|---|---|---|\n| _(review and update)_ | | |\n",
		newID, cmd.SourceTrackID, time.Now().Format("2006-01-02"))
	if err := h.store.WriteContextFile(ctx, newID, "_sources.md", sources); err != nil {
		return CreateTrackResult{}, fmt.Errorf("create track (clone): write _sources.md: %w", err)
	}

	// Copy prompts/ from source.
	srcPrompts := filepath.Join(h.store.ExperimentsDir(), cmd.SourceTrackID, "prompts")
	if entries, err := os.ReadDir(srcPrompts); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			content, err := os.ReadFile(filepath.Join(srcPrompts, e.Name()))
			if err != nil {
				continue
			}
			_ = h.store.WriteTrackFile(ctx, newID, filepath.Join("prompts", e.Name()), content)
		}
	}

	return CreateTrackResult{NewTrackID: newID}, nil
}
