// Package view contains view-model types and conversion helpers.
// Keeps template data shapes separate from domain types.
package view

import "github.com/tyrohunt/axon/internal/domain"

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
