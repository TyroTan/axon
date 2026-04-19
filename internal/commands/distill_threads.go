package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/llm"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

const distillModel = "claude-sonnet-4-6"
const distillMaxTokens = 4096

// minThreadTurns is the minimum number of messages (excluding the seed) a
// thread must have to be included in distillation. A thread with only the
// seed assistant message carries no learner signal.
const minThreadTurns = 2

// DistillThreadsCommand reads all tutoring threads across all sessions of a
// track, extracts the learning signal, and writes the result to
// context/session_insights.snapshot.md.
//
// The output file is a first-class context file: it is injected into future
// question generation prompts (inherited via Fork cascade, physically copied
// on Clone) so the question generator can adapt framing and distractors based
// on observed misconceptions and reasoning patterns.
type DistillThreadsCommand struct {
	TrackID string
}

// DistillThreadsHandler streams the LLM distillation and writes the output
// context file when done.
type DistillThreadsHandler struct {
	store  *filesystem.TrackStore
	client llm.Client
}

func NewDistillThreadsHandler(store *filesystem.TrackStore, client llm.Client) *DistillThreadsHandler {
	return &DistillThreadsHandler{store: store, client: client}
}

// Stream returns a channel of llm.Chunk. When Done=true, the snapshot file
// has been written to the track's context/ directory.
func (h *DistillThreadsHandler) Stream(ctx context.Context, cmd DistillThreadsCommand) <-chan llm.Chunk {
	out := make(chan llm.Chunk, 128)
	go func() {
		defer close(out)
		if err := h.run(ctx, cmd, out); err != nil {
			out <- llm.Chunk{Error: err}
		}
	}()
	return out
}

type threadEntry struct {
	SessionNum int
	Thread     domain.Thread
	Question   string
}

func (h *DistillThreadsHandler) run(ctx context.Context, cmd DistillThreadsCommand, out chan<- llm.Chunk) error {
	// ── 1. Collect all threads with enough signal ─────────────────────────────
	sessions, err := h.store.ListSessions(ctx, cmd.TrackID)
	if err != nil {
		return fmt.Errorf("distill threads: list sessions: %w", err)
	}

	var entries []threadEntry

	for _, sess := range sessions {
		threadsDir := filepath.Join(h.store.ExperimentsDir(), cmd.TrackID, "sessions",
			fmt.Sprintf("session_%03d", sess.Number), "threads")
		files, err := os.ReadDir(threadsDir)
		if err != nil {
			continue // no threads dir for this session — skip
		}

		// Load questions for this session so we can include question text.
		qb, _ := h.store.ReadSessionFile(ctx, cmd.TrackID, sess.Number, "01_questions.json")
		questionText := map[string]string{}
		if qb != nil {
			var questions []domain.Question
			if err := json.Unmarshal(qb, &questions); err == nil {
				for _, q := range questions {
					questionText[q.ID] = q.Question
				}
			}
		}

		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(threadsDir, f.Name()))
			if err != nil {
				continue
			}
			var thread domain.Thread
			if err := json.Unmarshal(b, &thread); err != nil {
				continue
			}
			// Count non-seed turns: skip threads where learner never responded.
			userTurns := 0
			for _, m := range thread.Messages {
				if m.Role == "user" {
					userTurns++
				}
			}
			if userTurns < minThreadTurns-1 {
				continue
			}
			entries = append(entries, threadEntry{
				SessionNum: sess.Number,
				Thread:     thread,
				Question:   questionText[thread.QuestionID],
			})
		}
	}

	if len(entries) == 0 {
		return fmt.Errorf("distill threads: no threads with learner turns found in %s", cmd.TrackID)
	}

	// ── 2. Build prompt ───────────────────────────────────────────────────────
	system := buildDistillSystem()
	user := buildDistillUser(cmd.TrackID, entries)

	// ── 3. Stream LLM call ────────────────────────────────────────────────────
	chunks := h.client.Stream(ctx, distillModel, system, []llm.Message{
		{Role: "user", Content: user},
	}, distillMaxTokens)

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

	// ── 4. Write context file ─────────────────────────────────────────────────
	now := time.Now().Format("2006-01-02")
	header := fmt.Sprintf("---\ngenerated_by: axon/distill-threads\ntrack: %s\ndate: %s\nsessions_scanned: %d\nthreads_included: %d\n---\n\n",
		cmd.TrackID, now, len(sessions), len(entries))
	output := header + sb.String()

	if err := h.store.WriteContextFile(ctx, cmd.TrackID, "session_insights.snapshot.md", output); err != nil {
		return fmt.Errorf("distill threads: write context file: %w", err)
	}

	out <- llm.Chunk{Done: true}
	return nil
}

// ─── prompt builders ──────────────────────────────────────────────────────────

func buildDistillSystem() string {
	return `You are a learning analyst for the Axon adaptive quiz system.
You will receive tutoring conversation threads from a learner's quiz sessions.
Each thread begins with the LLM tutoring seed (question + evaluation summary) followed by the learner's follow-up messages and your responses.

Your task: extract durable learning signal that will help the question generator in future sessions.

Output a structured markdown document with these sections:

## Reasoning Patterns
How the learner tends to approach problems. Are they bottom-up or top-down? Do they over-apply one mental model? Do they ask "why" or "how" questions? 2–5 bullet points, specific and evidence-based.

## Misconception Fingerprint
Specific wrong beliefs that surfaced in the conversations. For each: the misconception, which question triggered it, and whether it appeared to be resolved. Use sub-bullets.

## Distractor Affinities
Which types of wrong answers the learner gravitates toward (e.g., "picks the answer that sounds most technical", "confuses mechanism with outcome", "conflates two similar concepts"). These directly inform MCQ distractor selection.

## Concepts Needing Reinforcement
Concept names or topic areas where the learner needed multiple follow-ups or showed persistent confusion. One line each.

## Calibration Notes
Patterns in how the learner rates their confidence vs actual correctness. Are they overconfident? Underconfident on topics they actually understand?

Rules:
- Be specific — cite the question content, not just "question 3"
- Do not fabricate signal that isn't in the conversations
- Keep each section concise (3–8 bullet points max)
- This file will be injected verbatim into future question generation prompts — write it for an LLM reader, not a human`
}

func buildDistillUser(trackID string, entries []threadEntry) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Track: %s\nThreads to analyze: %d\n\n", trackID, len(entries))

	for i, e := range entries {
		fmt.Fprintf(&sb, "---\n### Thread %d — Session %d, Question ID: %s\n", i+1, e.SessionNum, e.Thread.QuestionID)
		if e.Question != "" {
			fmt.Fprintf(&sb, "Question text: %s\n\n", e.Question)
		}
		for _, m := range e.Thread.Messages {
			role := "Tutor"
			if m.Role == "user" {
				role = "Learner"
			}
			fmt.Fprintf(&sb, "**%s:** %s\n\n", role, m.Content)
		}
	}

	sb.WriteString("\nPlease produce the structured learning signal document as described.")
	return sb.String()
}
