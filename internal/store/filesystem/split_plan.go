package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ─── Split plan domain types (exported for query results + JSON encoding) ─────

// SplitShard is one bin in a split plan — a subset of context files that fits
// within the soft token limit.
type SplitShard struct {
	ID         string   `json:"id"`
	Status     string   `json:"status"` // "pending" | "approved"
	TokenCount int      `json:"token_count"`
	Files      []string `json:"files"`
}

// SplitPlan is the parsed content of _split_plan.md frontmatter.
type SplitPlan struct {
	Status      string       `json:"status"` // "pending" | "approved"
	TotalTokens int          `json:"total_tokens"`
	SoftLimit   int          `json:"soft_limit"`
	GeneratedAt time.Time    `json:"generated_at"`
	Shards      []SplitShard `json:"shards"`
}

// HasApprovedShards returns true if at least one shard is approved.
func (p *SplitPlan) HasApprovedShards() bool {
	for _, s := range p.Shards {
		if s.Status == "approved" {
			return true
		}
	}
	return false
}

// ReadSplitPlan reads and parses _split_plan.md from the track's context dir.
// Returns (nil, nil) if the file does not exist yet.
func (s *TrackStore) ReadSplitPlan(_ context.Context, trackID string) (*SplitPlan, error) {
	path := filepath.Join(s.experimentsDir, trackID, "context", "_split_plan.md")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read split plan %s: %w", trackID, err)
	}
	return parseSplitPlanFrontmatter(string(b))
}

// parseSplitPlanFrontmatter parses the YAML-like frontmatter block written by
// WriteSplitPlan. It is a hand-rolled parser tuned to the exact format we emit —
// no external YAML library needed.
func parseSplitPlanFrontmatter(content string) (*SplitPlan, error) {
	lines := strings.Split(content, "\n")

	// Find the two --- delimiters.
	start, end := -1, -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			if start == -1 {
				start = i + 1
			} else {
				end = i
				break
			}
		}
	}
	if start == -1 || end == -1 {
		return nil, fmt.Errorf("parse split plan: missing frontmatter delimiters")
	}

	plan := &SplitPlan{}
	var cur *SplitShard
	inShards := false
	inFiles := false

	for _, raw := range lines[start:end] {
		// Measure indent (spaces only — we always emit spaces).
		trimmed := strings.TrimLeft(raw, " ")
		indent := len(raw) - len(trimmed)

		switch {
		// ── Root-level key ────────────────────────────────────────────────
		case indent == 0:
			inShards = false
			inFiles = false
			kv := strings.SplitN(trimmed, ": ", 2)
			if len(kv) != 2 {
				continue
			}
			v := strings.TrimSpace(kv[1])
			switch kv[0] {
			case "status":
				plan.Status = v
			case "total_tokens":
				plan.TotalTokens, _ = strconv.Atoi(v)
			case "soft_limit":
				plan.SoftLimit, _ = strconv.Atoi(v)
			case "generated_at":
				plan.GeneratedAt, _ = time.Parse(time.RFC3339, v)
			case "shards":
				inShards = true
			}

		// ── Shard list item start: "  - id: shard_N" ─────────────────────
		case indent == 2 && inShards && strings.HasPrefix(trimmed, "- id: "):
			if cur != nil {
				plan.Shards = append(plan.Shards, *cur)
			}
			cur = &SplitShard{ID: strings.TrimPrefix(trimmed, "- id: ")}
			inFiles = false

		// ── Shard field: "    status: ...", "    token_count: ...", "    files:" ──
		case indent == 4 && inShards && cur != nil:
			inFiles = false
			kv := strings.SplitN(trimmed, ": ", 2)
			if len(kv) == 1 && strings.TrimSpace(kv[0]) == "files:" {
				inFiles = true
				continue
			}
			if len(kv) != 2 {
				continue
			}
			v := strings.TrimSpace(kv[1])
			switch kv[0] {
			case "status":
				cur.Status = v
			case "token_count":
				cur.TokenCount, _ = strconv.Atoi(v)
			case "files":
				inFiles = true
			}

		// ── File item: "      - filename.md" ─────────────────────────────
		case indent == 6 && inFiles && cur != nil && strings.HasPrefix(trimmed, "- "):
			cur.Files = append(cur.Files, strings.TrimPrefix(trimmed, "- "))
		}
	}
	if cur != nil {
		plan.Shards = append(plan.Shards, *cur)
	}
	return plan, nil
}

// LoadContextForShard loads only the files assigned to shardID, using the full
// inherited context as the source pool. Returns an error if no split plan exists
// or shardID is not found.
func (s *TrackStore) LoadContextForShard(ctx context.Context, trackID, shardID string) (map[string]string, int, error) {
	plan, err := s.ReadSplitPlan(ctx, trackID)
	if err != nil {
		return nil, 0, fmt.Errorf("load context for shard: %w", err)
	}
	if plan == nil {
		return nil, 0, fmt.Errorf("load context for shard: no split plan found for %s", trackID)
	}
	var sh *SplitShard
	for i := range plan.Shards {
		if plan.Shards[i].ID == shardID {
			sh = &plan.Shards[i]
			break
		}
	}
	if sh == nil {
		return nil, 0, fmt.Errorf("load context for shard: shard %q not found", shardID)
	}

	// Load the full merged corpus, then filter to this shard's files.
	merged, _, _, err := s.LoadFullContext(ctx, trackID)
	if err != nil {
		return nil, 0, err
	}
	allow := make(map[string]bool, len(sh.Files))
	for _, f := range sh.Files {
		allow[f] = true
	}
	result := make(map[string]string, len(sh.Files))
	total := 0
	for name, content := range merged {
		if allow[name] {
			result[name] = content
			total += (len(content) + 3) / 4
		}
	}
	return result, total, nil
}

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
