package queries

import (
	"context"
	"fmt"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// ─── Get Conversation ────────────────────────────────────────────────────────

type GetConversationQuery struct {
	ConversationID string
}

type GetConversationResult struct {
	Conversation domain.Conversation      `json:"conversation"`
	Index        *domain.ConversationIndex `json:"index,omitempty"` // nil if not yet indexed
}

type GetConversationHandler struct {
	store *filesystem.TrackStore
}

func NewGetConversationHandler(store *filesystem.TrackStore) *GetConversationHandler {
	return &GetConversationHandler{store: store}
}

func (h *GetConversationHandler) Handle(ctx context.Context, q GetConversationQuery) (GetConversationResult, error) {
	conv, err := h.store.ReadConversation(ctx, q.ConversationID)
	if err != nil {
		return GetConversationResult{}, err
	}
	if conv.ID == "" {
		return GetConversationResult{}, fmt.Errorf("conversation %q not found", q.ConversationID)
	}
	idx, _ := h.store.ReadConversationIndex(ctx, q.ConversationID)
	var idxPtr *domain.ConversationIndex
	if idx.ConversationID != "" {
		idxPtr = &idx
	}
	return GetConversationResult{Conversation: conv, Index: idxPtr}, nil
}

// ─── List Conversations ───────────────────────────────────────────────────────

type ListConversationsQuery struct{}

type ListConversationsResult struct {
	Conversations []domain.Conversation `json:"conversations"`
}

type ListConversationsHandler struct {
	store *filesystem.TrackStore
}

func NewListConversationsHandler(store *filesystem.TrackStore) *ListConversationsHandler {
	return &ListConversationsHandler{store: store}
}

func (h *ListConversationsHandler) Handle(ctx context.Context, _ ListConversationsQuery) (ListConversationsResult, error) {
	convs, err := h.store.ListConversations(ctx)
	if err != nil {
		return ListConversationsResult{}, err
	}
	if convs == nil {
		convs = []domain.Conversation{}
	}
	return ListConversationsResult{Conversations: convs}, nil
}
