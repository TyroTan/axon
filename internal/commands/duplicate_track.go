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

// DuplicateTrackCommand creates a child track (e.g. track_2 → track_2_2).
// Before branching, it crystallizes accumulated session and conversation signal
// into context/ snapshots so the child inherits the richest possible starting state.
// Both snapshot steps are idempotent — they are skipped if the snapshot already exists.
type DuplicateTrackCommand struct {
	SourceTrackID string
}

type DuplicateTrackResult struct {
	NewTrackID string `json:"new_track_id"`
}

type DuplicateTrackHandler struct {
	store     *filesystem.TrackStore
	distiller *DistillThreadsHandler
}

func NewDuplicateTrackHandler(store *filesystem.TrackStore, distiller *DistillThreadsHandler) *DuplicateTrackHandler {
	return &DuplicateTrackHandler{store: store, distiller: distiller}
}

func (h *DuplicateTrackHandler) Handle(ctx context.Context, cmd DuplicateTrackCommand) error {
	source, err := h.store.GetTrack(ctx, cmd.SourceTrackID)
	if err != nil {
		return fmt.Errorf("duplicate track: source %q not found: %w", cmd.SourceTrackID, err)
	}

	// ── Freeze step 1: distill threads → context/session_insights.snapshot.md ──
	// Idempotent: skipped if snapshot already exists.
	// Errors are non-fatal — snapshot absence doesn't block the fork.
	if h.distiller != nil {
		_, _ = h.distiller.RunSilent(ctx, DistillThreadsCommand{TrackID: cmd.SourceTrackID})
	}

	// ── Freeze step 2: snapshot conversation indexes ───────────────────────────
	// Builds context/conversations.snapshot.md from ConversationIndex objects
	// for all conversations referencing this track. No LLM call — pure aggregation.
	// Idempotent: skipped if snapshot already exists.
	if err := h.snapshotConversations(ctx, cmd.SourceTrackID); err != nil {
		// Non-fatal — missing snapshot doesn't block the fork.
		_ = err
	}

	// ── Branch: create child track ────────────────────────────────────────────
	newID, err := h.store.NextTrackID(cmd.SourceTrackID)
	if err != nil {
		return fmt.Errorf("duplicate track: %w", err)
	}

	cm, err := h.store.GetConceptMap(ctx, cmd.SourceTrackID)
	if err != nil {
		return fmt.Errorf("duplicate track: read concept map: %w", err)
	}
	cm.Track = newID
	cm.GeneratedAt = time.Now().Format("2006-01-02")
	cm.Note = fmt.Sprintf("Inherited from %s on %s. bloom_current preserved.", cmd.SourceTrackID, cm.GeneratedAt)

	newTrack := domain.Track{
		ID:        newID,
		ParentID:  cmd.SourceTrackID,
		Branches:  source.Branches,
		CreatedAt: time.Now(),
	}

	if err := h.store.CreateTrack(ctx, newTrack, cm); err != nil {
		return fmt.Errorf("duplicate track: create: %w", err)
	}

	sourcesContent := fmt.Sprintf(`# Context Sources — %s

> Inherited from: **%s**
> Forked on: %s
>
> Context is inherited at question-generation time — all ancestor context/ files
> are loaded automatically (child overrides parent on filename collision).
> To add specialized context for this fork, create new .md files via Edit Context.

## Inherited from %s

- session_insights.snapshot.md (if distilled before fork)
- conversations.snapshot.md (if conversations were indexed before fork)
- All other context/ files via cascade
`,
		newID, cmd.SourceTrackID, time.Now().Format("2006-01-02"), cmd.SourceTrackID,
	)

	if err := h.store.WriteContextFile(ctx, newID, "_sources.md", sourcesContent); err != nil {
		return fmt.Errorf("duplicate track: write _sources.md: %w", err)
	}

	// Copy prompts/ from source track.
	srcPrompts := filepath.Join(h.store.ExperimentsDir(), cmd.SourceTrackID, "prompts")
	entries, err := os.ReadDir(srcPrompts)
	if err == nil {
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

	return nil
}

// snapshotConversations writes context/conversations.snapshot.md aggregating all
// ConversationIndex objects for conversations referencing sourceTrackID.
// Skipped (idempotent) if the snapshot already exists.
// No LLM call — uses existing ConversationIndex files only.
func (h *DuplicateTrackHandler) snapshotConversations(ctx context.Context, trackID string) error {
	existing, _ := h.store.ReadContextFiles(ctx, trackID)
	if _, ok := existing["conversations.snapshot.md"]; ok {
		return nil
	}

	convs, err := h.store.ListConversations(ctx)
	if err != nil {
		return err
	}

	type indexedConv struct {
		conv domain.Conversation
		idx  domain.ConversationIndex
		hasIdx bool
	}

	var relevant []indexedConv
	for _, c := range convs {
		for _, tid := range c.TrackIDs {
			if tid == trackID {
				idx, _ := h.store.ReadConversationIndex(ctx, c.ID)
				relevant = append(relevant, indexedConv{conv: c, idx: idx, hasIdx: idx.ConversationID != ""})
				break
			}
		}
	}

	if len(relevant) == 0 {
		return nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "---\ngenerated_by: axon/fork-snapshot\ntrack: %s\ndate: %s\nconversations: %d\n---\n\n",
		trackID, time.Now().Format("2006-01-02"), len(relevant))
	sb.WriteString("# Conversation History Snapshot\n\n")
	sb.WriteString("> Captured at fork time from conversations referencing this track.\n")
	sb.WriteString("> Injected as context for question generation in descendant tracks.\n\n")

	for _, r := range relevant {
		title := r.conv.Title
		if title == "" {
			title = r.conv.ID
		}
		fmt.Fprintf(&sb, "## %s\n\n", title)

		if r.hasIdx {
			if r.idx.Summary != "" {
				fmt.Fprintf(&sb, "%s\n\n", r.idx.Summary)
			}
			if len(r.idx.Topics) > 0 {
				fmt.Fprintf(&sb, "**Topics:** %s\n\n", strings.Join(r.idx.Topics, " · "))
			}
			if len(r.idx.KeyDecisions) > 0 {
				sb.WriteString("**Key decisions:**\n")
				for _, d := range r.idx.KeyDecisions {
					fmt.Fprintf(&sb, "- %s\n", d)
				}
				sb.WriteString("\n")
			}
			if len(r.idx.OpenQuestions) > 0 {
				sb.WriteString("**Open questions:**\n")
				for _, q := range r.idx.OpenQuestions {
					fmt.Fprintf(&sb, "- %s\n", q)
				}
				sb.WriteString("\n")
			}
		} else {
			fmt.Fprintf(&sb, "_(no index — %d messages. Run Index on this conversation to include detail.)_\n\n",
				len(r.conv.Messages))
		}
	}

	return h.store.WriteContextFile(ctx, trackID, "conversations.snapshot.md", sb.String())
}
