package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FileTokens is a single context file with its naive token count.
type FileTokens struct {
	Name   string
	Tokens int
}

// LoadFullContext loads every inherited context file with no token limit.
// Returns the merged file map, per-file token counts, and the total.
// Used before generation to check soft/hard limits.
func (s *TrackStore) LoadFullContext(ctx context.Context, trackID string) (map[string]string, []FileTokens, int, error) {
	chain := []string{}
	id := trackID
	for id != "" {
		chain = append(chain, id)
		id = parentID(id)
	}

	merged := map[string]string{}
	for _, tid := range chain {
		files, err := s.ReadContextFiles(ctx, tid)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("load full context: %w", err)
		}
		for name, content := range files {
			if _, exists := merged[name]; exists {
				continue // child wins on collision
			}
			merged[name] = content
		}
	}

	total := 0
	ft := make([]FileTokens, 0, len(merged))
	for name, content := range merged {
		t := (len(content) + 3) / 4
		ft = append(ft, FileTokens{Name: name, Tokens: t})
		total += t
	}
	// Sort by name for deterministic output.
	sort.Slice(ft, func(i, j int) bool { return ft[i].Name < ft[j].Name })

	return merged, ft, total, nil
}

// WriteSplitPlan generates a greedy bin-packed _split_plan.md in the track's
// context/ directory. Each shard fits within shardSize tokens.
// The file is human-editable via the existing context editor UI.
func (s *TrackStore) WriteSplitPlan(ctx context.Context, trackID string, files []FileTokens, totalTokens, shardSize int) error {
	shards := binPack(files, shardSize)

	var sb strings.Builder

	// ── YAML frontmatter (machine-readable) ──────────────────────────────────
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "status: pending\n")
	fmt.Fprintf(&sb, "total_tokens: %d\n", totalTokens)
	fmt.Fprintf(&sb, "soft_limit: %d\n", shardSize)
	fmt.Fprintf(&sb, "generated_at: %s\n", time.Now().UTC().Format(time.RFC3339))
	sb.WriteString("shards:\n")
	for _, sh := range shards {
		fmt.Fprintf(&sb, "  - id: %s\n", sh.ID)
		fmt.Fprintf(&sb, "    status: pending\n")
		fmt.Fprintf(&sb, "    token_count: %d\n", sh.TokenCount)
		sb.WriteString("    files:\n")
		for _, f := range sh.Files {
			fmt.Fprintf(&sb, "      - %s\n", f)
		}
	}
	sb.WriteString("---\n\n")

	// ── Human-readable body ───────────────────────────────────────────────────
	fmt.Fprintf(&sb, "# Split Plan — %s\n\n", trackID)
	fmt.Fprintf(&sb, "Total context: **%s tokens** — exceeds soft limit of %s.\n\n",
		commaInt(totalTokens), commaInt(shardSize))
	sb.WriteString("Edit each shard's `status: pending` → `status: approved` once you are\n")
	sb.WriteString("satisfied with the assignment. When all shards are approved, session\n")
	sb.WriteString("creation will show a shard picker.\n\n")
	sb.WriteString("You may move files between shards — just keep each shard's `token_count`\n")
	sb.WriteString("accurate (or leave it stale; the UI recomputes on load).\n\n")

	for _, sh := range shards {
		fmt.Fprintf(&sb, "## %s (%s tokens)\n\n", sh.ID, commaInt(sh.TokenCount))
		for _, f := range sh.Files {
			fmt.Fprintf(&sb, "- `%s`\n", f)
		}
		sb.WriteString("\n")
	}

	dir := filepath.Join(s.experimentsDir, trackID, "context")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("write split plan: mkdir: %w", err)
	}
	path := filepath.Join(dir, "_split_plan.md")
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// ─── shard types ─────────────────────────────────────────────────────────────

type shard struct {
	ID         string
	Files      []string
	TokenCount int
}

// binPack assigns files to shards using first-fit decreasing.
// Files larger than shardSize are placed alone in an oversized shard.
func binPack(files []FileTokens, shardSize int) []shard {
	// Sort descending by token count for better packing.
	sorted := make([]FileTokens, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Tokens > sorted[j].Tokens })

	var shards []shard
	for _, f := range sorted {
		placed := false
		for i := range shards {
			if shards[i].TokenCount+f.Tokens <= shardSize {
				shards[i].Files = append(shards[i].Files, f.Name)
				shards[i].TokenCount += f.Tokens
				placed = true
				break
			}
		}
		if !placed {
			shards = append(shards, shard{
				ID:         fmt.Sprintf("shard_%d", len(shards)+1),
				Files:      []string{f.Name},
				TokenCount: f.Tokens,
			})
		}
	}

	// Sort files within each shard alphabetically for readability.
	for i := range shards {
		sort.Strings(shards[i].Files)
	}
	return shards
}

// commaInt formats an integer with comma thousands separators.
func commaInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	rem := len(s) % 3
	if rem > 0 {
		b.WriteString(s[:rem])
	}
	for i := rem; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
