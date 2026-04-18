package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// CreateTrackCommand creates a new root track (e.g. "track_2") from a list of
// branch names. The concept map is seeded with no concepts — the user runs
// prompts/00_concept_map_generator.md manually (or via a conversation) to
// populate it before starting sessions.
type CreateTrackCommand struct {
	// Branches is the list of major knowledge branches, e.g.
	// ["RAG Architecture", "LLM Systems", "ML Fundamentals"]
	Branches []string
}

// CreateTrackResult carries the assigned track ID back to the caller.
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
	if len(cmd.Branches) == 0 {
		return CreateTrackResult{}, fmt.Errorf("create track: at least one branch required")
	}

	// Root tracks use NextTrackID("") → track_1, track_2, track_3, ...
	newID, err := h.store.NextTrackID("")
	if err != nil {
		return CreateTrackResult{}, fmt.Errorf("create track: %w", err)
	}

	cm := domain.ConceptMap{
		Track:         newID,
		MajorBranches: cmd.Branches,
		GeneratedAt:   time.Now().Format("2006-01-02"),
		Note: "Seed map — no concepts yet. Run prompts/00_concept_map_generator.md " +
			"(paste as system prompt in Claude with branch names as user message) to populate.",
		Concepts: []domain.Concept{},
	}

	t := domain.Track{
		ID:        newID,
		ParentID:  "",
		Branches:  cmd.Branches,
		CreatedAt: time.Now(),
	}

	if err := h.store.CreateTrack(ctx, t, cm); err != nil {
		return CreateTrackResult{}, fmt.Errorf("create track: %w", err)
	}

	// Write _sources.md stub.
	sources := fmt.Sprintf(`# Context Sources — %s

> Root track created on %s.
> Branches: %s
>
> Add context .md files here (Edit Context in the UI) then run prompt 01 to
> build a starting profile, or start a session directly once concept_map.json
> is populated via prompt 00.

## Sources

| Snapshot filename | Source | Purpose |
|---|---|---|
| _(add snapshots here)_ | | |
`,
		newID, time.Now().Format("2006-01-02"), strings.Join(cmd.Branches, " · "),
	)
	if err := h.store.WriteContextFile(ctx, newID, "_sources.md", sources); err != nil {
		return CreateTrackResult{}, fmt.Errorf("create track: write _sources.md: %w", err)
	}

	return CreateTrackResult{NewTrackID: newID}, nil
}
