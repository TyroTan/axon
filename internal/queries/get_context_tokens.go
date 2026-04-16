package queries

import (
	"context"
	"fmt"
	"sort"

	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// ContextFileTokens is one file's token breakdown.
type ContextFileTokens struct {
	Filename   string `json:"filename"`
	Tokens     int    `json:"tokens"`
	SourceTrackID string `json:"source_track_id"` // which track in the ancestor chain it came from
}

// GetContextTokensQuery returns per-file token counts for a track's full
// inherited context — the same corpus that would be loaded for generation.
type GetContextTokensQuery struct {
	TrackID string
}

type GetContextTokensResult struct {
	TrackID     string              `json:"track_id"`
	Files       []ContextFileTokens `json:"files"`
	TotalTokens int                 `json:"total_tokens"`
	FileCount   int                 `json:"file_count"`
}

type GetContextTokensHandler struct {
	store *filesystem.TrackStore
}

func NewGetContextTokensHandler(store *filesystem.TrackStore) *GetContextTokensHandler {
	return &GetContextTokensHandler{store: store}
}

func (h *GetContextTokensHandler) Handle(ctx context.Context, q GetContextTokensQuery) (GetContextTokensResult, error) {
	// Walk the ancestor chain, same order as LoadInheritedContext, to attribute
	// each file to the track it came from.
	chain := ancestorChain(q.TrackID)

	seen := map[string]bool{}
	var files []ContextFileTokens
	total := 0

	for _, tid := range chain {
		contextFiles, err := h.store.ReadContextFiles(ctx, tid)
		if err != nil {
			return GetContextTokensResult{}, fmt.Errorf("get context tokens: %w", err)
		}
		// Sort filenames for determinism within each track.
		names := make([]string, 0, len(contextFiles))
		for name := range contextFiles {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if seen[name] {
				continue // child track's version takes priority — already counted
			}
			seen[name] = true
			content := contextFiles[name]
			tokens := (len(content) + 3) / 4
			files = append(files, ContextFileTokens{
				Filename:      name,
				Tokens:        tokens,
				SourceTrackID: tid,
			})
			total += tokens
		}
	}

	// Sort output by token count descending so biggest files are obvious.
	sort.Slice(files, func(i, j int) bool { return files[i].Tokens > files[j].Tokens })

	return GetContextTokensResult{
		TrackID:     q.TrackID,
		Files:       files,
		TotalTokens: total,
		FileCount:   len(files),
	}, nil
}

// ancestorChain returns [trackID, parentID, grandparentID, ...] in child-first order.
func ancestorChain(trackID string) []string {
	var chain []string
	id := trackID
	for id != "" {
		chain = append(chain, id)
		id = parentIDOf(id)
	}
	return chain
}

// parentIDOf derives the parent track ID from a track ID.
// "track_1_2" → "track_1", "track_1" → ""
func parentIDOf(trackID string) string {
	last := -1
	for i := len(trackID) - 1; i >= 0; i-- {
		if trackID[i] == '_' {
			last = i
			break
		}
	}
	if last <= 0 {
		return ""
	}
	// Make sure we're not stripping before the initial "track_" prefix.
	prefix := trackID[:last]
	if len(prefix) < 6 { // shorter than "track_"
		return ""
	}
	return prefix
}
