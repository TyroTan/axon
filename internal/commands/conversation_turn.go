// Package commands — ConversationTurnHandler.
// Drives a free-form, multi-track conversation using naive RAG over the union
// of all selected tracks' inherited contexts plus optional ad-hoc inline text.
// Uses --resume session continuity exactly like ThreadTurnHandler.
package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/llm"
	"github.com/tyrohunt/axon/internal/rag"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

const (
	convRagTokenBudget = 10_000 // tokens of retrieved context per turn
	convRagMaxChunks   = 15
	convRagChunkSize   = 1_000
)

// ConversationTurnCommand is issued for each user message in a conversation.
type ConversationTurnCommand struct {
	ConversationID string
	UserMessage    string
	AdHocText      string // optional extra context text injected for this turn only
}

// ConversationTurnHandler handles one turn: load → RAG → prompt → stream → persist.
type ConversationTurnHandler struct {
	store             *filesystem.TrackStore
	llm               llm.Client
	contextTokenLimit int
}

func NewConversationTurnHandler(store *filesystem.TrackStore, llm llm.Client, contextTokenLimit int) *ConversationTurnHandler {
	return &ConversationTurnHandler{store: store, llm: llm, contextTokenLimit: contextTokenLimit}
}

// Stream processes one user turn, streams the assistant reply via onChunk,
// and returns the updated conversation.
func (h *ConversationTurnHandler) Stream(
	ctx context.Context,
	cmd ConversationTurnCommand,
	onChunk func(string),
) (domain.Conversation, error) {
	// ── Load conversation ──────────────────────────────────────────────────────
	conv, err := h.store.ReadConversation(ctx, cmd.ConversationID)
	if err != nil {
		return domain.Conversation{}, fmt.Errorf("conversation turn: load: %w", err)
	}
	if conv.ID == "" {
		return domain.Conversation{}, fmt.Errorf("conversation turn: not found: %s", cmd.ConversationID)
	}

	// ── Union context from all track IDs ───────────────────────────────────────
	allFiles := map[string]string{}
	for _, tid := range conv.TrackIDs {
		budget, err := h.store.LoadInheritedContext(ctx, tid, 0)
		if err != nil {
			return domain.Conversation{}, fmt.Errorf("conversation turn: load context %s: %w", tid, err)
		}
		for name, content := range budget.Files {
			if _, exists := allFiles[name]; !exists {
				allFiles[name] = content
			}
		}
	}

	// ── Ad-hoc text (current turn + any persistent adhoc from conversation) ────
	adhoc := conv.AdHocText
	if cmd.AdHocText != "" {
		if adhoc != "" {
			adhoc += "\n\n"
		}
		adhoc += cmd.AdHocText
	}
	if adhoc != "" {
		allFiles["_adhoc.md"] = adhoc
	}

	// ── RAG retrieval ──────────────────────────────────────────────────────────
	chunks := rag.ChunkFiles(allFiles, convRagChunkSize)
	selected := rag.TopK(chunks, cmd.UserMessage, convRagMaxChunks, convRagTokenBudget)

	// ── Build prompts ──────────────────────────────────────────────────────────
	system := h.buildSystem(conv)
	userPrompt := h.buildUserPrompt(cmd.UserMessage, selected, len(conv.Messages) == 0)

	// ── Determine call mode ────────────────────────────────────────────────────
	sessionID := conv.ClaudeSessionID
	callMode := "resumed"
	if sessionID == "" {
		callMode = "fresh"
	}

	// ── Build message history for LLM ─────────────────────────────────────────
	// On fresh: pass full history. On resume: StreamResume sends only the latest message.
	var messages []llm.Message
	for _, m := range conv.Messages {
		messages = append(messages, llm.Message{Role: m.Role, Content: m.Content})
	}
	messages = append(messages, llm.Message{Role: "user", Content: userPrompt})

	// ── Stream ─────────────────────────────────────────────────────────────────
	var replyBuf strings.Builder
	var newSessionID string
	var inputTokens int

	ch := h.llm.StreamResume(ctx, sessionID, "claude-sonnet-4-6", system, messages, 4096)
	for chunk := range ch {
		if chunk.Error != nil {
			return domain.Conversation{}, fmt.Errorf("conversation turn: llm: %w", chunk.Error)
		}
		if chunk.Text != "" {
			replyBuf.WriteString(chunk.Text)
			onChunk(chunk.Text)
		}
		if chunk.Done {
			newSessionID = chunk.SessionID
			inputTokens = chunk.InputTokens
		}
	}

	if callMode == "fresh" && newSessionID != "" {
		callMode = "fresh"
	} else if sessionID != "" && newSessionID != "" && newSessionID != sessionID {
		// session expired, got a new one
		callMode = "fresh"
	}

	// ── Append messages ────────────────────────────────────────────────────────
	now := time.Now()
	conv.Messages = append(conv.Messages, domain.ConversationMessage{
		Role:    "user",
		Content: cmd.UserMessage,
		Ts:      now,
	})
	conv.Messages = append(conv.Messages, domain.ConversationMessage{
		Role:    "assistant",
		Content: replyBuf.String(),
		Ts:      now,
		Meta: &domain.ConversationMessageMeta{
			CallMode:        callMode,
			InputTokensSent: inputTokens,
			ContextChunks:   len(selected),
			ClaudeSessionID: newSessionID,
			TrackIDsLoaded:  conv.TrackIDs,
		},
	})

	// Persist ad-hoc text additions permanently if provided.
	if cmd.AdHocText != "" {
		if conv.AdHocText != "" {
			conv.AdHocText += "\n\n"
		}
		conv.AdHocText += cmd.AdHocText
	}

	if newSessionID != "" {
		conv.ClaudeSessionID = newSessionID
	}
	conv.AccumulatedInputTokens += inputTokens
	conv.UpdatedAt = now

	if err := h.store.WriteConversation(ctx, conv); err != nil {
		return domain.Conversation{}, fmt.Errorf("conversation turn: save: %w", err)
	}

	return conv, nil
}

// AddContext appends additional track IDs to a conversation's context sources.
func (h *ConversationTurnHandler) AddContext(ctx context.Context, id string, trackIDs []string) (domain.Conversation, error) {
	conv, err := h.store.ReadConversation(ctx, id)
	if err != nil || conv.ID == "" {
		return domain.Conversation{}, fmt.Errorf("add context: load conversation: %w", err)
	}
	existing := map[string]bool{}
	for _, tid := range conv.TrackIDs {
		existing[tid] = true
	}
	for _, tid := range trackIDs {
		if !existing[tid] {
			conv.TrackIDs = append(conv.TrackIDs, tid)
		}
	}
	conv.UpdatedAt = time.Now()
	if err := h.store.WriteConversation(ctx, conv); err != nil {
		return domain.Conversation{}, fmt.Errorf("add context: save: %w", err)
	}
	return conv, nil
}

// Create allocates a new conversation document.
func (h *ConversationTurnHandler) Create(ctx context.Context, trackIDs []string, title string) (domain.Conversation, error) {
	if len(trackIDs) == 0 {
		return domain.Conversation{}, fmt.Errorf("create conversation: at least one track ID required")
	}
	// Validate all tracks exist.
	for _, tid := range trackIDs {
		if _, err := h.store.GetTrack(ctx, tid); err != nil {
			return domain.Conversation{}, fmt.Errorf("create conversation: track %q not found: %w", tid, err)
		}
	}
	now := time.Now()
	conv := domain.Conversation{
		ID:        uuid.New().String(),
		Title:     title,
		TrackIDs:  trackIDs,
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  []domain.ConversationMessage{},
	}
	if err := h.store.WriteConversation(ctx, conv); err != nil {
		return domain.Conversation{}, fmt.Errorf("create conversation: save: %w", err)
	}
	return conv, nil
}

// ── prompt builders ────────────────────────────────────────────────────────────

func (h *ConversationTurnHandler) buildSystem(conv domain.Conversation) string {
	var sb strings.Builder
	sb.WriteString("You are a knowledgeable tutor and thinking partner. ")
	sb.WriteString("You have access to the learner's study materials, retrieved as context chunks below. ")
	sb.WriteString("Answer questions accurately, explain mechanisms, challenge assumptions, and help the learner build deep understanding.\n\n")
	sb.WriteString("Context sources for this conversation:\n")
	for _, tid := range conv.TrackIDs {
		sb.WriteString("  - " + tid + "\n")
	}
	if conv.AdHocText != "" {
		sb.WriteString("\nAdditional ad-hoc context is included in the user message when relevant.")
	}
	return sb.String()
}

func (h *ConversationTurnHandler) buildUserPrompt(message string, chunks []rag.Chunk, isFirst bool) string {
	var sb strings.Builder
	if len(chunks) > 0 {
		sb.WriteString("## Retrieved context\n\n")
		for _, c := range chunks {
			sb.WriteString(fmt.Sprintf("### [%s] %s\n\n%s\n\n", c.File, c.Heading, c.Content))
		}
		sb.WriteString("---\n\n")
	}
	if isFirst {
		sb.WriteString("## User\n\n")
	}
	sb.WriteString(message)
	return sb.String()
}
