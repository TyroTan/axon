package commands

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/tyrohunt/axon/internal/llm"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

const analyzeJobModel = "claude-sonnet-4-6"
const analyzeJobMaxTokens = 4096

// AnalyzeJobCommand takes a raw job description (pasted by the user) and an
// optional role label, calls the LLM to extract structured interview prep
// signal, and writes the result to context/{role}.job.md.
//
// The output file is a first-class context file — question generation reads it
// and adapts framing, scenario difficulty, and distractor selection to the
// specific role requirements. It propagates via Fork cascade and Clone copy.
type AnalyzeJobCommand struct {
	TrackID     string
	RoleLabel   string // e.g. "ragflow_expert" — used as filename stem; derived from JD if empty
	JobText     string // raw pasted job description
}

type AnalyzeJobHandler struct {
	store  *filesystem.TrackStore
	client llm.Client
}

func NewAnalyzeJobHandler(store *filesystem.TrackStore, client llm.Client) *AnalyzeJobHandler {
	return &AnalyzeJobHandler{store: store, client: client}
}

// Stream returns a channel of llm.Chunk. When Done=true, {role}.job.md has
// been written to the track's context/ directory.
func (h *AnalyzeJobHandler) Stream(ctx context.Context, cmd AnalyzeJobCommand) <-chan llm.Chunk {
	out := make(chan llm.Chunk, 128)
	go func() {
		defer close(out)
		if err := h.run(ctx, cmd, out); err != nil {
			out <- llm.Chunk{Error: err}
		}
	}()
	return out
}

func (h *AnalyzeJobHandler) run(ctx context.Context, cmd AnalyzeJobCommand, out chan<- llm.Chunk) error {
	if strings.TrimSpace(cmd.JobText) == "" {
		return fmt.Errorf("analyze job: job text is required")
	}

	system := buildAnalyzeJobSystem()
	user := buildAnalyzeJobUser(cmd.JobText)

	chunks := h.client.Stream(ctx, analyzeJobModel, system, []llm.Message{
		{Role: "user", Content: user},
	}, analyzeJobMaxTokens)

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

	// Derive filename from role label or first line of JD.
	stem := slugify(cmd.RoleLabel)
	if stem == "" {
		stem = slugify(firstLine(cmd.JobText))
	}
	if stem == "" {
		stem = fmt.Sprintf("job_%s", time.Now().Format("20060102"))
	}
	filename := stem + ".job.md"

	header := fmt.Sprintf("---\ngenerated_by: axon/analyze-job\ntrack: %s\ndate: %s\n---\n\n",
		cmd.TrackID, time.Now().Format("2006-01-02"))
	output := header + sb.String()

	if err := h.store.WriteContextFile(ctx, cmd.TrackID, filename, output); err != nil {
		return fmt.Errorf("analyze job: write context file: %w", err)
	}

	// Signal the filename so the UI can show it without guessing.
	out <- llm.Chunk{Done: true, SessionID: filename} // reuse SessionID field to carry filename
	return nil
}

// ─── prompt builders ──────────────────────────────────────────────────────────

func buildAnalyzeJobSystem() string {
	return `You are an interview prep analyst for the Axon adaptive quiz system.
You will receive a raw job description. Your task: extract structured interview preparation signal that will be injected into the quiz question generator for a track focused on this role.

Output a structured markdown document with these sections:

## Role Signal
One paragraph: what this client/company actually wants (beyond the stated requirements). What signals are they sending about their stack, maturity level, and what a successful candidate looks like? What is the unstated bar?

## Must-Have Skills
Bulleted list of skills/knowledge areas directly required. For each: the skill, why it matters for this role specifically (1 sentence).

## Likely Interview Probes
Table with columns: Skill Area | Likely Question / Scenario. 6–10 rows. These are the specific questions a technical screen would ask — not generic, but derived from this JD's language.

## Interview Scenario Seeds
2–3 L4/L5 scenario questions (Apply/Evaluate level) grounded in the role's actual deliverables. Format each as a paragraph starting with "You are..." or "A client reports...". These feed directly into quiz question generation.

## Self-Assessment Anchors
What a strong candidate already knows vs what they likely need to close before interviewing. Frame as: Strong if... / Gap if... / Close by...

## Question Format Guidance
Instructions for the quiz question generator:
- Which question types to weight (mcq, free_text, interview_scenario)
- What misconceptions to probe as distractors
- Minimum Bloom level for this role
- Any domain-specific framing (e.g. safety constraints, latency SLAs, client handoff)

Rules:
- Be specific — quote or paraphrase the JD language, don't generalize
- Do not invent requirements not present in the JD
- Write for an LLM reader: precise, structured, no filler prose
- This file will be injected verbatim into future question generation prompts`
}

func buildAnalyzeJobUser(jobText string) string {
	return fmt.Sprintf("Job description to analyze:\n\n---\n%s\n---\n\nPlease produce the structured interview prep document.", jobText)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// Replace non-alphanumeric runs with underscore.
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if len(s) > 40 {
		s = s[:40]
	}
	// Must start with a letter.
	if len(s) > 0 && unicode.IsDigit(rune(s[0])) {
		s = "job_" + s
	}
	return s
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\n\r"); i > 0 {
		return s[:i]
	}
	if len(s) > 60 {
		return s[:60]
	}
	return s
}
