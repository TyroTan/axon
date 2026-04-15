package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/llm"
	"github.com/tyrohunt/axon/internal/metrics"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

const metaSynthModel = "claude-sonnet-4-6"
const metaSynthMaxTokens = 2048

// MetaSynthesisCommand aggregates evaluations from all shard sessions and
// produces a unified bloom update for the whole track.
type MetaSynthesisCommand struct {
	TrackID string
}

// MetaSynthesisHandler streams the meta-synthesis and writes meta_synthesis.json.
type MetaSynthesisHandler struct {
	store  *filesystem.TrackStore
	client llm.Client
	rec    *metrics.Recorder
}

func NewMetaSynthesisHandler(store *filesystem.TrackStore, client llm.Client, rec *metrics.Recorder) *MetaSynthesisHandler {
	return &MetaSynthesisHandler{store: store, client: client, rec: rec}
}

// ReadinessCheck returns the list of approved shard IDs that still lack an
// evaluated session. Empty slice = all shards ready, safe to run.
func (h *MetaSynthesisHandler) ReadinessCheck(ctx context.Context, trackID string) ([]string, error) {
	plan, err := h.store.ReadSplitPlan(ctx, trackID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("meta-synthesis: no split plan found for %s", trackID)
	}

	sessions, err := h.store.ListSessionsWithMeta(ctx, trackID)
	if err != nil {
		return nil, err
	}

	// For each approved shard, find the most recent session with evaluations.
	evaluated := map[string]bool{}
	for _, s := range sessions {
		if s.ShardID != "" && s.HasEvaluations {
			evaluated[s.ShardID] = true
		}
	}

	var missing []string
	for _, sh := range plan.Shards {
		if sh.Status == "approved" && !evaluated[sh.ID] {
			missing = append(missing, sh.ID)
		}
	}
	return missing, nil
}

func (h *MetaSynthesisHandler) Stream(ctx context.Context, cmd MetaSynthesisCommand) <-chan llm.Chunk {
	out := make(chan llm.Chunk, 128)
	go func() {
		defer close(out)
		if err := h.run(ctx, cmd, out); err != nil {
			out <- llm.Chunk{Error: err}
		}
	}()
	return out
}

func (h *MetaSynthesisHandler) run(ctx context.Context, cmd MetaSynthesisCommand, out chan<- llm.Chunk) error {
	// Gate: all approved shards must have an evaluated session.
	missing, err := h.ReadinessCheck(ctx, cmd.TrackID)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return fmt.Errorf("meta-synthesis: shards not yet evaluated: %s", strings.Join(missing, ", "))
	}

	cm, err := h.store.GetConceptMap(ctx, cmd.TrackID)
	if err != nil {
		return fmt.Errorf("meta-synthesis: concept map: %w", err)
	}

	plan, _ := h.store.ReadSplitPlan(ctx, cmd.TrackID)
	sessions, err := h.store.ListSessionsWithMeta(ctx, cmd.TrackID)
	if err != nil {
		return fmt.Errorf("meta-synthesis: list sessions: %w", err)
	}

	// Collect the most recent evaluated session per shard.
	// "Most recent" = highest session number with evaluations for that shard.
	bestByShardID := map[string]filesystem.SessionWithMeta{}
	for _, s := range sessions {
		if s.ShardID == "" || !s.HasEvaluations {
			continue
		}
		if prev, ok := bestByShardID[s.ShardID]; !ok || s.Number > prev.Number {
			bestByShardID[s.ShardID] = s
		}
	}

	// Aggregate evaluations across all selected sessions.
	var allEvals []domain.Evaluation
	var sessionNums []int
	var shardIDs []string

	for shardID, sess := range bestByShardID {
		eb, err := h.store.ReadSessionFile(ctx, cmd.TrackID, sess.Number, "03_evaluations.json")
		if err != nil || eb == nil {
			return fmt.Errorf("meta-synthesis: read evaluations session %d: %w", sess.Number, err)
		}
		var evals []domain.Evaluation
		if err := json.Unmarshal(eb, &evals); err != nil {
			return fmt.Errorf("meta-synthesis: parse evaluations session %d: %w", sess.Number, err)
		}
		allEvals = append(allEvals, evals...)
		sessionNums = append(sessionNums, sess.Number)
		shardIDs = append(shardIDs, shardID)
	}
	sort.Ints(sessionNums)
	sort.Strings(shardIDs)

	out <- llm.Chunk{Text: fmt.Sprintf("[meta-synthesis] aggregating %d evaluations across %d shards\n",
		len(allEvals), len(shardIDs))}

	generationID := uuid.New().String()
	system := buildMetaSynthSystemPrompt()
	user := buildMetaSynthUserPrompt(cmd.TrackID, generationID, plan, cm, allEvals, sessionNums, shardIDs)

	chunks := h.client.Stream(ctx, metaSynthModel, system, []llm.Message{
		{Role: "user", Content: user},
	}, metaSynthMaxTokens)

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

	synthesis, err := parseMetaSynthesis(sb.String(), generationID, cmd.TrackID, sessionNums, shardIDs, cm)
	if err != nil {
		return fmt.Errorf("meta-synthesis: parse: %w", err)
	}
	b, err := json.MarshalIndent(synthesis, "", "  ")
	if err != nil {
		return fmt.Errorf("meta-synthesis: marshal: %w", err)
	}
	if err := h.store.WriteTrackFile(ctx, cmd.TrackID, "meta_synthesis.json", b); err != nil {
		return fmt.Errorf("meta-synthesis: write: %w", err)
	}
	h.rec.Record(metrics.Event{
		Event:   "meta_synthesis_generated",
		TrackID: cmd.TrackID,
		Extra:   map[string]any{"shards": shardIDs, "sessions": sessionNums},
	})

	out <- llm.Chunk{Done: true}
	return nil
}

// ─── prompt builders ──────────────────────────────────────────────────────────

func buildMetaSynthSystemPrompt() string {
	return `You are a learning analytics engine for the Axon adaptive system.
You are given aggregated evaluations from MULTIPLE shard sessions covering different
subsets of a large corpus. Synthesize a UNIFIED bloom update for the whole track.

Output ONLY JSON — no prose, no markdown fences.

Schema:
{
  "concept_map_updates": [
    {
      "concept_index": 0,
      "bloom_current_before": 2,
      "bloom_current_after": 3,
      "spaced_repetition": {
        "next_review": "2026-04-21",
        "interval_days": 7,
        "consecutive_correct": 1
      }
    }
  ],
  "learner_summary": "3-5 sentence summary covering all shards. Note any concepts where performance varied across shards."
}

Rules for bloom_current_after:
- Advance by 1 if correctness ≥ 0.75 across ≥ 2 questions for this concept (across all shards combined)
- Drop by 1 (min 1) if correctness < 0.4 and misconception identified
- Otherwise keep the same
- Never exceed bloom_target or 6
- Only include concepts that were actually tested (appeared in concept_indexes)`
}

func buildMetaSynthUserPrompt(
	trackID, generationID string,
	plan *filesystem.SplitPlan,
	cm domain.ConceptMap,
	allEvals []domain.Evaluation,
	sessionNums []int, shardIDs []string,
) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Track: %s | Generation: %s\n", trackID, generationID)
	fmt.Fprintf(&sb, "Today's date: %s\n", time.Now().Format("2006-01-02"))
	fmt.Fprintf(&sb, "Sessions aggregated: %v | Shards: %v\n\n", sessionNums, shardIDs)

	// Concept map — all concepts.
	sb.WriteString("Concept Map:\n")
	conceptByIdx := make(map[int]domain.Concept, len(cm.Concepts))
	for _, c := range cm.Concepts {
		conceptByIdx[c.Index] = c
		fmt.Fprintf(&sb, "  [%d] %s (bloom: %d→%d, consecutive_correct: %d)\n",
			c.Index, c.Name, c.BloomCurrent, c.BloomTarget, c.SpacedRepetition.ConsecutiveCorrect)
	}

	// Per-concept aggregate stats.
	type cs struct {
		totalCorrectness float64
		count            int
		misconception    bool
	}
	stats := map[int]*cs{}
	for _, e := range allEvals {
		for _, idx := range e.ConceptIndexes {
			if _, ok := stats[idx]; !ok {
				stats[idx] = &cs{}
			}
			s := stats[idx]
			s.totalCorrectness += e.Correctness
			s.count++
			if e.MisconceptionIdentified != nil {
				s.misconception = true
			}
		}
	}

	sb.WriteString("\nAggregated per-concept results (across all shards):\n")
	// Sort by concept index for determinism.
	idxs := make([]int, 0, len(stats))
	for idx := range stats {
		idxs = append(idxs, idx)
	}
	sort.Ints(idxs)
	for _, idx := range idxs {
		s := stats[idx]
		avg := s.totalCorrectness / float64(s.count)
		fmt.Fprintf(&sb, "  [%d] avg_correctness=%.2f n=%d misconception=%v\n",
			idx, avg, s.count, s.misconception)
	}

	fmt.Fprintf(&sb, "\nTotal evaluations aggregated: %d\n", len(allEvals))
	return sb.String()
}

// ─── parser ───────────────────────────────────────────────────────────────────

func parseMetaSynthesis(
	raw, generationID, trackID string,
	sessionNums []int, shardIDs []string,
	cm domain.ConceptMap,
) (domain.MetaSynthesis, error) {
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

	var ls llmSynthesis // reuse the same shape as per-session synthesis
	if err := json.Unmarshal([]byte(s), &ls); err != nil {
		return domain.MetaSynthesis{}, fmt.Errorf("unmarshal: %w (raw prefix: %.200s)", err, s)
	}

	// Enrich with bloom_current_before + clamp.
	conceptByIdx := make(map[int]domain.Concept, len(cm.Concepts))
	for _, c := range cm.Concepts {
		conceptByIdx[c.Index] = c
	}
	for i := range ls.ConceptMapUpdates {
		u := &ls.ConceptMapUpdates[i]
		if c, ok := conceptByIdx[u.ConceptIndex]; ok {
			u.BloomCurrentBefore = c.BloomCurrent
			if u.BloomCurrentAfter > c.BloomTarget {
				u.BloomCurrentAfter = c.BloomTarget
			}
			if u.BloomCurrentAfter < 1 {
				u.BloomCurrentAfter = 1
			}
		}
	}

	return domain.MetaSynthesis{
		TrackID:            trackID,
		Date:               time.Now().Format("2006-01-02"),
		GenerationID:       generationID,
		SessionsAggregated: sessionNums,
		ShardsAggregated:   shardIDs,
		ConceptMapUpdates:  ls.ConceptMapUpdates,
		LearnerSummary:     ls.LearnerSummary,
		Applied:            false,
	}, nil
}
