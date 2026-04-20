package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// MergeTracksCommand creates a new composite track whose context is the union
// of the compiled (inherited) contexts of the listed source tracks.
// The tree structure is preserved: the new track is placed under ParentID.
// Source tracks are read-only — they are not modified.
type MergeTracksCommand struct {
	SourceIDs []string // track IDs to merge, in priority order (first wins on filename collision)
	ParentID  string   // parent of the new composite track; "" = root
}

// MergeTracksResult carries the new track ID.
type MergeTracksResult struct {
	NewTrackID string `json:"new_track_id"`
	FileCount  int    `json:"file_count"`
	TotalConcepts int `json:"total_concepts"`
}

type MergeTracksHandler struct {
	store    *filesystem.TrackStore
	distiller *DistillThreadsHandler
}

func NewMergeTracksHandler(store *filesystem.TrackStore, distiller *DistillThreadsHandler) *MergeTracksHandler {
	return &MergeTracksHandler{store: store, distiller: distiller}
}

func (h *MergeTracksHandler) Handle(ctx context.Context, cmd MergeTracksCommand) (MergeTracksResult, error) {
	if len(cmd.SourceIDs) < 2 {
		return MergeTracksResult{}, fmt.Errorf("merge tracks: at least 2 source tracks required")
	}

	// Validate all sources exist.
	for _, id := range cmd.SourceIDs {
		if _, err := h.store.GetTrack(ctx, id); err != nil {
			return MergeTracksResult{}, fmt.Errorf("merge tracks: source %q: %w", id, err)
		}
	}

	// Pre-distill each source track (F3 — idempotent, non-fatal per source).
	for _, id := range cmd.SourceIDs {
		if _, err := h.distiller.RunSilent(ctx, DistillThreadsCommand{TrackID: id}); err != nil {
			// Non-fatal: if distill fails (no threads, LLM error) merge proceeds without snapshot.
			_ = err
		}
	}

	// Allocate new track ID.
	newID, err := h.store.NextTrackID(cmd.ParentID)
	if err != nil {
		return MergeTracksResult{}, fmt.Errorf("merge tracks: alloc ID: %w", err)
	}

	// ── 1. Union context files (first source wins on collision) ──────────────
	mergedFiles := map[string]string{}
	for _, srcID := range cmd.SourceIDs {
		budget, err := h.store.LoadInheritedContext(ctx, srcID, 0) // 0 = no limit
		if err != nil {
			return MergeTracksResult{}, fmt.Errorf("merge tracks: load context %s: %w", srcID, err)
		}
		for name, content := range budget.Files {
			if _, exists := mergedFiles[name]; !exists {
				mergedFiles[name] = content
			}
		}
	}

	// ── 2. Union concept maps — re-index, preserve intra-source links ────────
	mergedCM, err := h.mergeConceptMaps(ctx, newID, cmd.SourceIDs)
	if err != nil {
		return MergeTracksResult{}, fmt.Errorf("merge tracks: merge concept maps: %w", err)
	}

	// ── 3. Create track directory structure ───────────────────────────────────
	now := time.Now()
	newTrack := domain.Track{
		ID:          newID,
		ParentID:    cmd.ParentID,
		Branches:    mergedCM.MajorBranches,
		CreatedAt:   now,
		IsComposite: true,
		SourceIDs:   cmd.SourceIDs,
	}
	if err := h.store.CreateTrack(ctx, newTrack, mergedCM); err != nil {
		return MergeTracksResult{}, fmt.Errorf("merge tracks: create track: %w", err)
	}

	// ── 4. Write merged context files ─────────────────────────────────────────
	for name, content := range mergedFiles {
		if err := h.store.WriteContextFile(ctx, newID, name, content); err != nil {
			return MergeTracksResult{}, fmt.Errorf("merge tracks: write %s: %w", name, err)
		}
	}

	// ── 5. Write _sources.md lineage note ────────────────────────────────────
	if err := h.store.WriteContextFile(ctx, newID, "_sources.md", buildMergeSourcesMd(newID, cmd.SourceIDs, now)); err != nil {
		return MergeTracksResult{}, fmt.Errorf("merge tracks: write _sources.md: %w", err)
	}

	// ── 6. Write track_meta.json ──────────────────────────────────────────────
	meta := domain.TrackMeta{
		IsComposite: true,
		SourceIDs:   cmd.SourceIDs,
	}
	if err := h.store.WriteTrackMeta(ctx, newID, meta); err != nil {
		return MergeTracksResult{}, fmt.Errorf("merge tracks: write track_meta: %w", err)
	}

	return MergeTracksResult{
		NewTrackID:    newID,
		FileCount:     len(mergedFiles),
		TotalConcepts: len(mergedCM.Concepts),
	}, nil
}

// mergeConceptMaps unions all source concept maps.
// Concepts are re-indexed 0…N in source order.
// Prerequisite/unlock links within the same source block are remapped;
// cross-source links are cleared (they reference different concept spaces).
func (h *MergeTracksHandler) mergeConceptMaps(ctx context.Context, newID string, sourceIDs []string) (domain.ConceptMap, error) {
	branchSet := map[string]bool{}
	var allBranches []string
	var allConcepts []domain.Concept

	for _, srcID := range sourceIDs {
		cm, err := h.store.GetConceptMap(ctx, srcID)
		if err != nil {
			return domain.ConceptMap{}, fmt.Errorf("concept map %s: %w", srcID, err)
		}

		// Collect branches.
		for _, b := range cm.MajorBranches {
			if !branchSet[b] {
				branchSet[b] = true
				allBranches = append(allBranches, b)
			}
		}

		// Re-index this source's concepts starting at the current offset.
		offset := len(allConcepts)
		for _, c := range cm.Concepts {
			nc := c
			nc.Index = offset + c.Index

			// Remap intra-source prerequisite / unlock indexes.
			nc.PrerequisiteIndexes = remapIndexes(c.PrerequisiteIndexes, offset, len(cm.Concepts))
			nc.UnlocksIndexes = remapIndexes(c.UnlocksIndexes, offset, len(cm.Concepts))

			allConcepts = append(allConcepts, nc)
		}
	}

	return domain.ConceptMap{
		Track:         newID,
		MajorBranches: allBranches,
		GeneratedAt:   time.Now().Format("2006-01-02"),
		Note: fmt.Sprintf("Composite track — merged from: %s", strings.Join(sourceIDs, ", ")),
		Concepts:      allConcepts,
	}, nil
}

// remapIndexes shifts a list of concept indexes by offset.
// Indexes that were out of range in the original source (0…srcLen-1) are dropped.
func remapIndexes(idxs []int, offset, srcLen int) []int {
	if len(idxs) == 0 {
		return nil
	}
	out := make([]int, 0, len(idxs))
	for _, i := range idxs {
		if i >= 0 && i < srcLen {
			out = append(out, offset+i)
		}
	}
	return out
}

func buildMergeSourcesMd(newID string, sourceIDs []string, at time.Time) string {
	lines := []string{
		fmt.Sprintf("# Context Sources — %s", newID),
		"",
		fmt.Sprintf("> **Composite track** created on %s", at.Format("2006-01-02")),
		">",
		"> Source tracks (first listed wins on filename collision):",
	}
	for _, id := range sourceIDs {
		lines = append(lines, fmt.Sprintf("> - %s", id))
	}
	lines = append(lines,
		"",
		"## Provenance",
		"",
		"The context files in this track are the union of the fully inherited (cascaded)",
		"contexts of each source track at the time of merge. They are physical copies —",
		"edits here do not affect the source tracks and vice versa.",
		"",
		"The concept map is a re-indexed union of all source concept maps.",
		"Intra-source prerequisite/unlock links are preserved (with remapped indexes).",
		"Cross-source links are not synthesized automatically.",
		"",
		"## Source tracks",
		"",
		"| Track | Purpose |",
		"|---|---|",
	)
	for _, id := range sourceIDs {
		lines = append(lines, fmt.Sprintf("| %s | _(add description)_ |", id))
	}
	return strings.Join(lines, "\n") + "\n"
}
