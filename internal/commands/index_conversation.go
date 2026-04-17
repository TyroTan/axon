// Package commands — IndexConversationHandler.
// After a conversation ends (or at any point), this handler sends the full
// conversation transcript to the LLM and produces a structured ConversationIndex:
// summary, topics, key decisions, open questions, per-ply summaries, concept refs.
// The index is written as {id}.index.json and is queryable independently of the
// full conversation transcript.
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/llm"
	"github.com/tyrohunt/axon/internal/metrics"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// IndexConversationCommand triggers structured indexing of a conversation.
type IndexConversationCommand struct {
	ConversationID string
}

// IndexConversationHandler sends the transcript to the LLM and writes the index.
type IndexConversationHandler struct {
	store   *filesystem.TrackStore
	llm     llm.Client
	metrics *metrics.Recorder
}

func NewIndexConversationHandler(store *filesystem.TrackStore, llm llm.Client, rec *metrics.Recorder) *IndexConversationHandler {
	return &IndexConversationHandler{store: store, llm: llm, metrics: rec}
}

// Stream indexes a conversation, streaming LLM output via onChunk, and writes the index.
func (h *IndexConversationHandler) Stream(
	ctx context.Context,
	cmd IndexConversationCommand,
	onChunk func(string),
) (domain.ConversationIndex, error) {
	// ── Load conversation ──────────────────────────────────────────────────────
	conv, err := h.store.ReadConversation(ctx, cmd.ConversationID)
	if err != nil || conv.ID == "" {
		return domain.ConversationIndex{}, fmt.Errorf("index conversation: load: conversation %q not found", cmd.ConversationID)
	}
	if len(conv.Messages) == 0 {
		return domain.ConversationIndex{}, fmt.Errorf("index conversation: conversation has no messages")
	}

	// ── Load concept maps for all track IDs (for concept ref resolution) ───────
	conceptNames := map[int]string{}
	for _, tid := range conv.TrackIDs {
		if cm, err := h.store.GetConceptMap(ctx, tid); err == nil {
			for _, c := range cm.Concepts {
				if _, exists := conceptNames[c.Index]; !exists {
					conceptNames[c.Index] = c.Name
				}
			}
		}
	}

	// ── Build indexing prompt ──────────────────────────────────────────────────
	system := buildIndexSystem()
	userPrompt := buildIndexUserPrompt(conv, conceptNames)

	// ── Stream LLM response ────────────────────────────────────────────────────
	var buf strings.Builder
	ch := h.llm.Stream(ctx, "claude-sonnet-4-6", system, []llm.Message{
		{Role: "user", Content: userPrompt},
	}, 4096)
	for chunk := range ch {
		if chunk.Error != nil {
			return domain.ConversationIndex{}, fmt.Errorf("index conversation: llm: %w", chunk.Error)
		}
		if chunk.Text != "" {
			buf.WriteString(chunk.Text)
			onChunk(chunk.Text)
		}
	}

	// ── Parse structured JSON from LLM ────────────────────────────────────────
	idx, err := parseConversationIndex(buf.String(), conv)
	if err != nil {
		return domain.ConversationIndex{}, fmt.Errorf("index conversation: parse: %w", err)
	}

	// ── Write index ───────────────────────────────────────────────────────────
	if err := h.store.WriteConversationIndex(ctx, idx); err != nil {
		return domain.ConversationIndex{}, fmt.Errorf("index conversation: write: %w", err)
	}

	if h.metrics != nil {
		h.metrics.Record(metrics.Event{
			Event:   "conversation_indexed",
			Extra:   map[string]any{"conversation_id": conv.ID, "turns": len(conv.Messages)},
		})
	}

	return idx, nil
}

// ── prompt builders ────────────────────────────────────────────────────────────

func buildIndexSystem() string {
	return `You are an expert at extracting structured knowledge from technical conversations.
Given a conversation transcript, produce a structured JSON index that captures:
- A concise overall summary
- Topics discussed
- Key decisions or conclusions reached
- Open questions left unresolved
- Concept map indexes referenced (by number, from the available concept list)
- Per-message (ply) summaries with topics and key points

Output ONLY valid JSON matching this exact schema:
{
  "summary": "string — 2-4 sentence overview of the whole conversation",
  "topics": ["topic1", "topic2"],
  "key_decisions": ["decision or conclusion 1", "..."],
  "open_questions": ["unresolved question 1", "..."],
  "concept_indexes_referenced": [0, 3, 7],
  "ply_index": [
    {
      "turn": 0,
      "role": "user",
      "summary": "brief summary of this message",
      "topics": ["topic"],
      "key_points": []
    },
    {
      "turn": 1,
      "role": "assistant",
      "summary": "brief summary",
      "topics": ["topic"],
      "key_points": ["point 1", "point 2"]
    }
  ]
}

concept_indexes_referenced: only include indexes from the provided concept list where the concept was meaningfully discussed. Empty array if none apply.
key_decisions: only concrete conclusions reached — skip if conversation was exploratory.
open_questions: questions raised but not resolved.`
}

func buildIndexUserPrompt(conv domain.Conversation, conceptNames map[int]string) string {
	var sb strings.Builder

	sb.WriteString("## Conversation metadata\n\n")
	sb.WriteString(fmt.Sprintf("Title: %s\n", conv.Title))
	sb.WriteString(fmt.Sprintf("Context tracks: %s\n", strings.Join(conv.TrackIDs, ", ")))
	sb.WriteString(fmt.Sprintf("Messages: %d\n\n", len(conv.Messages)))

	if len(conceptNames) > 0 {
		sb.WriteString("## Available concept map indexes\n\n")
		for idx, name := range conceptNames {
			sb.WriteString(fmt.Sprintf("  [%d] %s\n", idx, name))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Transcript\n\n")
	for i, m := range conv.Messages {
		sb.WriteString(fmt.Sprintf("### Turn %d — %s\n\n%s\n\n", i, m.Role, m.Content))
	}

	sb.WriteString("---\n\nProduce the structured JSON index now.")
	return sb.String()
}

// ── parser ─────────────────────────────────────────────────────────────────────

type llmConversationIndex struct {
	Summary                  string                       `json:"summary"`
	Topics                   []string                     `json:"topics"`
	KeyDecisions             []string                     `json:"key_decisions"`
	OpenQuestions            []string                     `json:"open_questions"`
	ConceptIndexesReferenced []int                        `json:"concept_indexes_referenced"`
	PlyIndex                 []domain.ConversationPlyIndex `json:"ply_index"`
}

func parseConversationIndex(raw string, conv domain.Conversation) (domain.ConversationIndex, error) {
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

	var li llmConversationIndex
	if err := json.Unmarshal([]byte(s), &li); err != nil {
		return domain.ConversationIndex{}, fmt.Errorf("unmarshal: %w (raw prefix: %.200s)", err, s)
	}

	// Enrich ply_index with token estimates from actual messages.
	for i := range li.PlyIndex {
		turn := li.PlyIndex[i].Turn
		if turn >= 0 && turn < len(conv.Messages) {
			li.PlyIndex[i].Tokens = (len(conv.Messages[turn].Content) + 3) / 4
		}
	}

	return domain.ConversationIndex{
		ConversationID:           conv.ID,
		TrackIDs:                 conv.TrackIDs,
		Title:                    conv.Title,
		GenerationID:             uuid.New().String(),
		IndexedAt:                time.Now(),
		TurnCount:                len(conv.Messages),
		Summary:                  li.Summary,
		Topics:                   li.Topics,
		KeyDecisions:             li.KeyDecisions,
		OpenQuestions:            li.OpenQuestions,
		ConceptIndexesReferenced: li.ConceptIndexesReferenced,
		PlyIndex:                 li.PlyIndex,
	}, nil
}
