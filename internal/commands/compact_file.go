package commands

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/llm"
	"github.com/tyrohunt/axon/internal/metrics"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

const compactModel = "claude-sonnet-4-6"
const compactMaxTokens = 4096

// CompactFileCommand asks the LLM to distil a verbose context file down to
// targetTokens while preserving all information relevant to the concept map.
// Output is written as filename.compact.md with a SHA-256 hash frontmatter so
// callers can detect when the source file has changed and re-compact.
//
// This is intentionally a manual, per-file operation — it is NOT triggered
// automatically during context loading. Use it for genuinely verbose files
// (changelogs, transcripts, long prose) where the signal-to-noise ratio is low.
// Splitting (T2) remains the primary strategy for large corpora.
type CompactFileCommand struct {
	TrackID     string
	Filename    string // source file inside track's context/ directory
	TargetTokens int   // desired token budget for the compacted version
}

type CompactFileHandler struct {
	store  *filesystem.TrackStore
	client llm.Client
	rec    *metrics.Recorder
}

func NewCompactFileHandler(store *filesystem.TrackStore, client llm.Client, rec *metrics.Recorder) *CompactFileHandler {
	return &CompactFileHandler{store: store, client: client, rec: rec}
}

// Stream returns a channel of llm.Chunk. When Done=true, filename.compact.md
// has been written to the track's context/ directory.
func (h *CompactFileHandler) Stream(ctx context.Context, cmd CompactFileCommand) <-chan llm.Chunk {
	out := make(chan llm.Chunk, 128)
	go func() {
		defer close(out)
		if err := h.run(ctx, cmd, out); err != nil {
			out <- llm.Chunk{Error: err}
		}
	}()
	return out
}

func (h *CompactFileHandler) run(ctx context.Context, cmd CompactFileCommand, out chan<- llm.Chunk) error {
	// Load the source file.
	files, err := h.store.ReadContextFiles(ctx, cmd.TrackID)
	if err != nil {
		return fmt.Errorf("compact file: read context: %w", err)
	}
	content, ok := files[cmd.Filename]
	if !ok {
		return fmt.Errorf("compact file: file %q not found in %s context", cmd.Filename, cmd.TrackID)
	}

	sourceTokens := (len(content) + 3) / 4
	if cmd.TargetTokens <= 0 {
		cmd.TargetTokens = sourceTokens / 2 // default: halve it
	}

	// Compute source hash for cache invalidation frontmatter.
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))[:16]

	// Load concept map for context-aware compaction.
	cm, err := h.store.GetConceptMap(ctx, cmd.TrackID)
	if err != nil {
		return fmt.Errorf("compact file: concept map: %w", err)
	}

	system := buildCompactSystemPrompt()
	user := buildCompactUserPrompt(cmd.Filename, content, cmd.TargetTokens, cm)

	chunks := h.client.Stream(ctx, compactModel, system, []llm.Message{
		{Role: "user", Content: user},
	}, compactMaxTokens)

	var sb strings.Builder
	for chunk := range chunks {
		if chunk.Error != nil {
			return chunk.Error
		}
		if chunk.Text != "" {
			sb.WriteString(chunk.Text)
			out <- llm.Chunk{Text: chunk.Text}
		}
		if chunk.Done {
			break
		}
	}

	compacted := sb.String()
	compactedTokens := (len(compacted) + 3) / 4

	// Write filename.compact.md with hash frontmatter.
	output := buildCompactOutput(cmd.Filename, hash, sourceTokens, compactedTokens, compacted)
	compactName := compactFilename(cmd.Filename)
	if err := h.store.WriteContextFile(ctx, cmd.TrackID, compactName, output); err != nil {
		return fmt.Errorf("compact file: write: %w", err)
	}

	h.rec.Record(metrics.Event{
		Event:   "compaction_run",
		TrackID: cmd.TrackID,
		Tokens:  int64(sourceTokens),
		Extra: map[string]any{
			"filename":         cmd.Filename,
			"source_tokens":    sourceTokens,
			"compacted_tokens": compactedTokens,
			"reduction_pct":    100 - (compactedTokens*100/max(sourceTokens, 1)),
		},
	})

	out <- llm.Chunk{Done: true}
	return nil
}

// ─── prompt builders ──────────────────────────────────────────────────────────

func buildCompactSystemPrompt() string {
	return `You are a technical document compactor for the Axon learning system.
Your task: distil a verbose document to a target token budget while preserving
all information relevant to the provided concept map.

Rules:
- Keep every definition, mechanism, causal relationship, and example that maps
  to a concept in the concept map.
- Remove: filler prose, excessive repetition, meta-commentary, padding sentences.
- Do NOT summarize in a way that loses precision — prefer cutting whole sections
  over paraphrasing core claims.
- Output ONLY the compacted document text. No frontmatter, no commentary.`
}

func buildCompactUserPrompt(filename, content string, targetTokens int, cm domain.ConceptMap) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "File: %s\n", filename)
	fmt.Fprintf(&sb, "Source tokens (approx): %d\n", (len(content)+3)/4)
	fmt.Fprintf(&sb, "Target tokens: %d\n\n", targetTokens)

	sb.WriteString("Concept map (preserve information relevant to these):\n")
	for _, c := range cm.Concepts {
		fmt.Fprintf(&sb, "  [%d] %s (%s)\n", c.Index, c.Name, c.Branch)
	}

	fmt.Fprintf(&sb, "\n--- SOURCE DOCUMENT: %s ---\n%s\n", filename, content)
	return sb.String()
}

func buildCompactOutput(filename, hash string, sourceTokens, compactedTokens int, compacted string) string {
	return fmt.Sprintf(`---
source_file: %s
source_hash: %s
source_tokens: %d
compacted_tokens: %d
compacted_by: axon/compact
---

%s`, filename, hash, sourceTokens, compactedTokens, compacted)
}

// compactFilename derives the .compact.md output name from the source filename.
// "agent_state_machine.md" → "agent_state_machine.compact.md"
// "notes.txt"              → "notes.compact.txt"
// "README"                 → "README.compact.md"
func compactFilename(name string) string {
	dot := strings.LastIndex(name, ".")
	if dot < 0 {
		return name + ".compact.md"
	}
	return name[:dot] + ".compact" + name[dot:]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
