package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/llm"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

const stateModel = "claude-sonnet-4-6"
const stateMaxTokens = 512

// DetectLearnerStateCommand infers the composite learner state from session signals
// and patches the synthesis file with composite_state, perceived_trust_proxy, and
// nudge_suggestion. Safe to re-run — overwrites prior detection.
type DetectLearnerStateCommand struct {
	TrackID       string
	SessionNumber int
}

type DetectLearnerStateHandler struct {
	store  *filesystem.TrackStore
	client llm.Client
}

func NewDetectLearnerStateHandler(store *filesystem.TrackStore, client llm.Client) *DetectLearnerStateHandler {
	return &DetectLearnerStateHandler{store: store, client: client}
}

func (h *DetectLearnerStateHandler) Handle(ctx context.Context, cmd DetectLearnerStateCommand) (*domain.Synthesis, error) {
	sb, err := h.store.ReadSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "04_synthesis.json")
	if err != nil || sb == nil {
		return nil, fmt.Errorf("detect learner state: synthesis not found — run synthesis first")
	}
	var synthesis domain.Synthesis
	if err := json.Unmarshal(sb, &synthesis); err != nil {
		return nil, fmt.Errorf("detect learner state: parse synthesis: %w", err)
	}

	rb, _ := h.store.ReadSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "02_responses.json")
	var responses []domain.Response
	if rb != nil {
		json.Unmarshal(rb, &responses) //nolint:errcheck
	}

	eb, _ := h.store.ReadSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "03_evaluations.json")
	var evaluations []domain.Evaluation
	if eb != nil {
		json.Unmarshal(eb, &evaluations) //nolint:errcheck
	}

	prior := h.store.ListPriorSyntheses(ctx, cmd.TrackID, cmd.SessionNumber, 3)

	system := buildStateSystem()
	user := buildStateUser(cmd.SessionNumber, responses, evaluations, prior)

	chunks := h.client.Stream(ctx, stateModel, system, []llm.Message{
		{Role: "user", Content: user},
	}, stateMaxTokens)

	var raw strings.Builder
	for chunk := range chunks {
		if chunk.Error != nil {
			return nil, fmt.Errorf("detect learner state: llm: %w", chunk.Error)
		}
		raw.WriteString(chunk.Text)
		if chunk.Done {
			break
		}
	}

	detected, err := parseStateOutput(raw.String())
	if err != nil {
		return nil, fmt.Errorf("detect learner state: parse output: %w", err)
	}

	synthesis.CompositeState = detected.CompositeState
	synthesis.StateConfidence = detected.StateConfidence
	synthesis.PerceivedTrustProxy = detected.PerceivedTrustProxy
	synthesis.NudgeSuggestion = detected.NudgeSuggestion

	patched, err := json.MarshalIndent(synthesis, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("detect learner state: marshal: %w", err)
	}
	if err := h.store.WriteSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "04_synthesis.json", patched); err != nil {
		return nil, fmt.Errorf("detect learner state: write: %w", err)
	}
	return &synthesis, nil
}

// ─── prompt builders ──────────────────────────────────────────────────────────

func buildStateSystem() string {
	return `You are the Axon learner state detector. Given session signals, infer the
composite learner state and produce a pre-session nudge for the next session.
Output ONLY JSON — no prose, no markdown fences.

## The 8 composite states

Momentum × Challenge Alignment:

| State        | Momentum | Challenge      | Observable pattern                                         |
|---|---|---|---|
| Flow         | charged  | in-zone        | High engagement, 60–85% accuracy, advancing bloom levels   |
| Stretch      | charged  | over-challenged| High engagement, <60% accuracy, attempting above floor     |
| Cruising     | charged  | under-challenged| High engagement, >85% accuracy, no bloom advancement      |
| Grind        | stable   | in-zone        | Steady effort, flat trajectory, moderate accuracy          |
| Overreach    | stable   | over-challenged| Consistent attempts above demonstrated level, stalling     |
| Plateau      | stable   | under-challenged| Stable accuracy >85%, no new concept advancement          |
| Coasting     | depleted | under-challenged| Low engagement, easy questions, short explanations        |
| AversiveEdge | depleted | over-challenged| Low engagement + high difficulty — high friction state     |

Momentum signals: confidence trend, explanation length trend, sessions with threads opened.
- charged: avg_confidence ≥ 3.5 AND explanation lengths growing
- stable: avg_confidence 2.5–3.5 AND flat explanation lengths
- depleted: avg_confidence < 2.5 OR explanation lengths shrinking OR very short

Challenge signals: accuracy and bloom_level_demonstrated vs bloom_current.
- under-challenged: accuracy > 85%
- in-zone: accuracy 60–85%
- over-challenged: accuracy < 60%

## Trust proxy (0.0–1.0)
Estimate perceived trust from behavioral signals:
- 0.8–1.0: deep thread engagement, long explanations on hard questions, no skip patterns
- 0.5–0.7: moderate engagement, some thread use
- 0.2–0.4: short explanations on hard questions, no threads, possible disengagement
- 0.0–0.2: minimal engagement signals across all dimensions

## Nudge format
One sentence. Actionable. Rejectable. State what axon observed + one concrete suggestion.
Do NOT include scores. End with "You can skip this."
Example: "Last session showed strong framing but stalling under time pressure —
suggested: one question with a strict constraint applied. You can skip this."

## Output schema
{
  "composite_state": "<one of the 8 state names>",
  "state_confidence": <0.0–1.0>,
  "perceived_trust_proxy": <0.0–1.0>,
  "nudge_suggestion": "<one sentence>"
}

If insufficient signal (first session, no prior data), use:
- composite_state: "unknown"
- state_confidence: 0.0
- perceived_trust_proxy: 0.5
- nudge_suggestion: "Not enough sessions to detect a pattern yet — focus on whatever feels most relevant."
`
}

func buildStateUser(sessionNum int, responses []domain.Response, evaluations []domain.Evaluation, prior []domain.Synthesis) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Current session: %d\n\n", sessionNum)

	// Current session signals.
	if len(evaluations) > 0 {
		var totalCorrect, totalConf float64
		bloomCounts := map[int]int{}
		for _, e := range evaluations {
			totalCorrect += e.Correctness
			bloomCounts[e.BloomLevelDemonstrated]++
		}
		for _, r := range responses {
			totalConf += float64(r.Confidence)
		}
		avgAcc := totalCorrect / float64(len(evaluations)) * 100
		avgConf := 0.0
		if len(responses) > 0 {
			avgConf = totalConf / float64(len(responses))
		}
		fmt.Fprintf(&sb, "Current session signals:\n")
		fmt.Fprintf(&sb, "  accuracy: %.0f%%\n", avgAcc)
		fmt.Fprintf(&sb, "  avg_confidence: %.1f/5\n", avgConf)
		fmt.Fprintf(&sb, "  questions: %d\n", len(evaluations))
		sb.WriteString("  bloom_levels_demonstrated:")
		for lvl, cnt := range bloomCounts {
			fmt.Fprintf(&sb, " L%d×%d", lvl, cnt)
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("Current session signals: no evaluations available\n")
	}

	// Prior session summaries for trajectory.
	if len(prior) > 0 {
		sb.WriteString("\nPrior sessions (most recent first):\n")
		for i, syn := range prior {
			fmt.Fprintf(&sb, "  Session %d: %s\n", syn.SessionNumber, syn.LearnerSummary)
			if i >= 2 {
				break
			}
		}
	} else {
		sb.WriteString("\nPrior sessions: none (first session)\n")
	}

	return sb.String()
}

// ─── parser ───────────────────────────────────────────────────────────────────

type stateDetectionOutput struct {
	CompositeState      string  `json:"composite_state"`
	StateConfidence     float64 `json:"state_confidence"`
	PerceivedTrustProxy float64 `json:"perceived_trust_proxy"`
	NudgeSuggestion     string  `json:"nudge_suggestion"`
}

func parseStateOutput(raw string) (stateDetectionOutput, error) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "```") {
		if first := strings.Index(s, "\n"); first >= 0 {
			s = s[first+1:]
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	var out stateDetectionOutput
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return out, fmt.Errorf("unmarshal: %w (raw: %.200s)", err, s)
	}
	return out, nil
}
