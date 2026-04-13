// Package view contains view-model types and conversion helpers.
// Keeps template data shapes separate from domain types.
package view

import (
	"github.com/tyrohunt/axon/internal/domain"
)

// Breadcrumb is one step in the page breadcrumb trail.
type Breadcrumb struct {
	Label string
	URL   string // empty = current (non-linkable) crumb
}

// SidebarTrack is the view-model for one node in the sidebar tree.
type SidebarTrack struct {
	ID       string
	Branches []string
	IsActive bool
	Children []SidebarTrack
}

// ToSidebarTracks converts domain tracks to sidebar view-models, marking activeID.
func ToSidebarTracks(tracks []domain.Track, activeID string) []SidebarTrack {
	out := make([]SidebarTrack, 0, len(tracks))
	for _, t := range tracks {
		out = append(out, toSidebarTrack(t, activeID))
	}
	return out
}

func toSidebarTrack(t domain.Track, activeID string) SidebarTrack {
	children := make([]SidebarTrack, 0, len(t.Children))
	for _, c := range t.Children {
		children = append(children, toSidebarTrack(c, activeID))
	}
	return SidebarTrack{
		ID:       t.ID,
		Branches: t.Branches,
		IsActive: t.ID == activeID,
		Children: children,
	}
}

// ConceptsByBranch groups concepts by their branch, preserving branch order.
func ConceptsByBranch(cm domain.ConceptMap) map[string][]domain.Concept {
	// Use a map; order preserved by MajorBranches field in the template loop.
	out := make(map[string][]domain.Concept, len(cm.MajorBranches))
	for _, c := range cm.Concepts {
		out[c.Branch] = append(out[c.Branch], c)
	}
	return out
}

// BloomPct returns the fill percentage of bloom_current relative to bloom_target (capped at 100).
func BloomPct(current, target int) int {
	if target == 0 {
		return 0
	}
	pct := (current * 100) / target
	if pct > 100 {
		return 100
	}
	return pct
}
