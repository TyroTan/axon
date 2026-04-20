package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/llm"
	"github.com/tyrohunt/axon/internal/rag"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

const threadModel = "claude-sonnet-4-6"

// Maximum tokens of RAG context chunks to inject on a fresh thread call.
const ragTokenBudget = 8000

// Maximum number of RAG chunks to inject.
const ragMaxChunks = 10

// Maximum tokens per chunk before it's further split.
const ragChunkSize = 1000

// ThreadTurnCommand sends one user message in a per-question conversation thread.
// On the very first call (no thread exists), Role must be "" and Content must be
// "" — the handler auto-seeds the conversation with the evaluation summary.
// For subsequent turns, Role="user" and Content=<the user's message>.
type ThreadTurnCommand struct {
	TrackID       string
	SessionNumber int
	QuestionID    string
	UserMessage   string // empty string = seed the conversation from evaluation
}

// ThreadTurnHandler streams one LLM turn in a per-question follow-up thread.
type ThreadTurnHandler struct {
	store             *filesystem.TrackStore
	client            llm.Client
	contextTokenLimit int // passed in from config (typically 50k)
}

func NewThreadTurnHandler(store *filesystem.TrackStore, client llm.Client, contextTokenLimit int) *ThreadTurnHandler {
	return &ThreadTurnHandler{store: store, client: client, contextTokenLimit: contextTokenLimit}
}

// ThreadPreviewChunk is one retrieved RAG chunk surfaced in the preview.
type ThreadPreviewChunk struct {
	File    string  `json:"file"`
	Heading string  `json:"heading"`
	Tokens  int     `json:"tokens"`
	Score   float64 `json:"score"`
	Preview string  `json:"preview"` // first 120 chars of content
}

// ThreadPreviewResult is returned by Preview — the full context that would be
// sent to the LLM on the seed call, without actually calling it.
type ThreadPreviewResult struct {
	QuestionID          string               `json:"question_id"`
	CallMode            string               `json:"call_mode"`            // "fresh" | "resumed"
	ClaudeSessionID     string               `json:"claude_session_id"`    // empty if no prior thread
	AccumulatedTokens   int                  `json:"accumulated_tokens"`   // from prior turns
	SystemPrompt        string               `json:"system_prompt"`
	UserPrompt          string               `json:"user_prompt"`
	SystemTokens        int                  `json:"system_tokens"`
	UserTokens          int                  `json:"user_tokens"`
	TokensToSend        int                  `json:"tokens_to_send"`
	TokensSavedByResume int                  `json:"tokens_saved_by_resume"`
	RAGChunks           []ThreadPreviewChunk `json:"rag_chunks"`
}

// Preview builds the seed context for a question thread without calling the LLM.
// This is the "dry run" used by the UI to show what would be sent.
func (h *ThreadTurnHandler) Preview(ctx context.Context, trackID string, sessionNum int, questionID string) (ThreadPreviewResult, error) {
	thread, err := h.loadThread(ctx, trackID, sessionNum, questionID)
	if err != nil {
		return ThreadPreviewResult{}, err
	}

	q, response, evaluation, err := h.loadQuestionContext(ctx, ThreadTurnCommand{
		TrackID: trackID, SessionNumber: sessionNum, QuestionID: questionID,
	})
	if err != nil {
		return ThreadPreviewResult{}, err
	}

	ragChunks, err := h.retrieveRelevantChunks(ctx, trackID, sessionNum, q.Question)
	if err != nil {
		return ThreadPreviewResult{}, err
	}

	system := buildThreadSystem()
	userPrompt := buildSeedPrompt(q, response, evaluation, ragChunks)
	systemTokens := (len(system) + 3) / 4
	userTokens := (len(userPrompt) + 3) / 4

	callMode := "fresh"
	tokensToSend := systemTokens + userTokens
	tokensSaved := 0
	if thread.ClaudeSessionID != "" {
		callMode = "resumed"
		tokensToSend = userTokens // only delta
		tokensSaved = systemTokens
	}

	previewChunks := make([]ThreadPreviewChunk, len(ragChunks))
	for i, c := range ragChunks {
		preview := c.Content
		if len(preview) > 120 {
			preview = preview[:120] + "…"
		}
		previewChunks[i] = ThreadPreviewChunk{
			File:    c.File,
			Heading: c.Heading,
			Tokens:  c.Tokens,
			Score:   c.Score,
			Preview: preview,
		}
	}

	return ThreadPreviewResult{
		QuestionID:          questionID,
		CallMode:            callMode,
		ClaudeSessionID:     thread.ClaudeSessionID,
		AccumulatedTokens:   thread.AccumulatedInputTokens,
		SystemPrompt:        system,
		UserPrompt:          userPrompt,
		SystemTokens:        systemTokens,
		UserTokens:          userTokens,
		TokensToSend:        tokensToSend,
		TokensSavedByResume: tokensSaved,
		RAGChunks:           previewChunks,
	}, nil
}

func (h *ThreadTurnHandler) Stream(ctx context.Context, cmd ThreadTurnCommand) <-chan llm.Chunk {
	out := make(chan llm.Chunk, 128)
	go func() {
		defer close(out)
		if err := h.run(ctx, cmd, out); err != nil {
			out <- llm.Chunk{Error: err}
		}
	}()
	return out
}

func (h *ThreadTurnHandler) run(ctx context.Context, cmd ThreadTurnCommand, out chan<- llm.Chunk) error {
	// ── Load existing thread (or create empty) ───────────────────────────────
	thread, err := h.loadThread(ctx, cmd.TrackID, cmd.SessionNumber, cmd.QuestionID)
	if err != nil {
		return err
	}

	seeding := len(thread.Messages) == 0 // true on first call

	// ── Load question, response, evaluation ─────────────────────────────────
	q, response, evaluation, err := h.loadQuestionContext(ctx, cmd)
	if err != nil {
		return err
	}

	// ── Load RAG context (only on fresh calls or if session expired) ─────────
	resumeID := thread.ClaudeSessionID
	callMode := "resumed"
	var ragChunks []rag.Chunk
	if resumeID == "" || seeding {
		callMode = "fresh"
		ragChunks, err = h.retrieveRelevantChunks(ctx, cmd.TrackID, cmd.SessionNumber, q.Question)
		if err != nil {
			return err // non-fatal in theory, but surface it
		}
	}

	// ── Build system + user prompt ───────────────────────────────────────────
	system := buildThreadSystem()
	var userPrompt string
	if seeding {
		callMode = "seeded"
		userPrompt = buildSeedPrompt(q, response, evaluation, ragChunks, cmd.UserMessage)
	} else {
		if resumeID != "" {
			// Resumed: only send the new user message.
			userPrompt = cmd.UserMessage
		} else {
			// Session expired — rebuild context, then append the new message.
			callMode = "fresh"
			userPrompt = buildFreshResumePrompt(q, response, evaluation, ragChunks, thread, cmd.UserMessage)
		}
	}

	// Record the user message in the thread whenever one is present.
	if cmd.UserMessage != "" {
		thread.Messages = append(thread.Messages, domain.ThreadMessage{
			Role:      "user",
			Content:   cmd.UserMessage,
			Timestamp: time.Now(),
		})
	}

	// ── Call LLM ─────────────────────────────────────────────────────────────
	messages := []llm.Message{{Role: "user", Content: userPrompt}}
	inputTokenEst := (len(system) + len(userPrompt) + 3) / 4

	chunks := h.client.StreamResume(ctx, resumeID, threadModel, system, messages, 2048)

	var sb strings.Builder
	var returnedSessionID string
	var returnedInputTokens int
	for chunk := range chunks {
		if chunk.Error != nil {
			return chunk.Error
		}
		if chunk.Text != "" {
			sb.WriteString(chunk.Text)
			out <- llm.Chunk{Text: chunk.Text}
		}
		if chunk.Done {
			returnedSessionID = chunk.SessionID
			returnedInputTokens = chunk.InputTokens
			if returnedInputTokens == 0 {
				returnedInputTokens = inputTokenEst // fallback to naive estimate
			}
			break
		}
	}

	// ── Persist assistant message + update thread ─────────────────────────────
	thread.Messages = append(thread.Messages, domain.ThreadMessage{
		Role:      "assistant",
		Content:   sb.String(),
		Timestamp: time.Now(),
		Meta: &domain.ThreadMessageMeta{
			CallMode:          callMode,
			InputTokensSent:   returnedInputTokens,
			ContextChunksUsed: len(ragChunks),
			ClaudeSessionID:   returnedSessionID,
		},
	})

	if returnedSessionID != "" {
		thread.ClaudeSessionID = returnedSessionID
	}
	thread.AccumulatedInputTokens += returnedInputTokens

	if err := h.saveThread(ctx, cmd.TrackID, cmd.SessionNumber, thread); err != nil {
		return err
	}

	out <- llm.Chunk{Done: true, SessionID: returnedSessionID, InputTokens: returnedInputTokens}
	return nil
}

// ─── context loaders ─────────────────────────────────────────────────────────

func (h *ThreadTurnHandler) loadThread(ctx context.Context, trackID string, sessionNum int, questionID string) (*domain.Thread, error) {
	filename := filepath.Join("threads", questionID+".json")
	b, err := h.store.ReadSessionFile(ctx, trackID, sessionNum, filename)
	if err != nil {
		return nil, fmt.Errorf("thread: load: %w", err)
	}
	if b == nil {
		return &domain.Thread{QuestionID: questionID}, nil
	}
	var t domain.Thread
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, fmt.Errorf("thread: unmarshal: %w", err)
	}
	return &t, nil
}

func (h *ThreadTurnHandler) saveThread(ctx context.Context, trackID string, sessionNum int, thread *domain.Thread) error {
	b, err := json.MarshalIndent(thread, "", "  ")
	if err != nil {
		return fmt.Errorf("thread: marshal: %w", err)
	}
	filename := filepath.Join("threads", thread.QuestionID+".json")
	return h.store.WriteSessionFile(ctx, trackID, sessionNum, filename, b)
}

func (h *ThreadTurnHandler) loadQuestionContext(ctx context.Context, cmd ThreadTurnCommand) (
	q domain.Question, response *domain.Response, evaluation *domain.Evaluation, err error,
) {
	// Questions
	qb, err := h.store.ReadSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "01_questions.json")
	if err != nil || qb == nil {
		return q, nil, nil, fmt.Errorf("thread: load questions: %w", err)
	}
	var questions []domain.Question
	if err := json.Unmarshal(qb, &questions); err != nil {
		return q, nil, nil, fmt.Errorf("thread: unmarshal questions: %w", err)
	}
	for _, qq := range questions {
		if qq.ID == cmd.QuestionID {
			q = qq
			break
		}
	}

	// Responses (optional)
	rb, _ := h.store.ReadSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "02_responses.json")
	if rb != nil {
		var responses []domain.Response
		if err := json.Unmarshal(rb, &responses); err == nil {
			for i := range responses {
				if responses[i].QuestionID == cmd.QuestionID {
					r := responses[i]
					response = &r
					break
				}
			}
		}
	}

	// Evaluations (optional)
	eb, _ := h.store.ReadSessionFile(ctx, cmd.TrackID, cmd.SessionNumber, "03_evaluations.json")
	if eb != nil {
		var evaluations []domain.Evaluation
		if err := json.Unmarshal(eb, &evaluations); err == nil {
			for i := range evaluations {
				if evaluations[i].QuestionID == cmd.QuestionID {
					e := evaluations[i]
					evaluation = &e
					break
				}
			}
		}
	}

	return q, response, evaluation, nil
}

func (h *ThreadTurnHandler) retrieveRelevantChunks(ctx context.Context, trackID string, sessionNum int, questionText string) ([]rag.Chunk, error) {
	// Check if the session is shard-specific.
	meta, _ := h.store.ReadSessionMetadata(ctx, trackID, sessionNum)
	var contextFiles map[string]string
	if meta.ShardID != "" {
		files, _, err := h.store.LoadContextForShard(ctx, trackID, meta.ShardID)
		if err != nil {
			return nil, fmt.Errorf("thread: rag load shard: %w", err)
		}
		contextFiles = files
	} else {
		budget, err := h.store.LoadInheritedContext(ctx, trackID, h.contextTokenLimit)
		if err != nil {
			return nil, fmt.Errorf("thread: rag load context: %w", err)
		}
		contextFiles = budget.Files
	}

	chunks := rag.ChunkFiles(contextFiles, ragChunkSize)
	return rag.TopK(chunks, questionText, ragMaxChunks, ragTokenBudget), nil
}

// ─── prompt builders ─────────────────────────────────────────────────────────

func buildThreadSystem() string {
	return `You are a learning tutor helping a student understand a quiz question they just answered.
You have access to: the question, the student's answer and explanation, the evaluation feedback, and relevant course material.
Be conversational, precise, and concise. Correct misconceptions directly. Use examples from the course material when relevant.
When seeding the conversation: summarise the evaluation outcome in 2-3 sentences, highlight the key insight the student missed (if any), and invite a follow-up question.
In subsequent turns: answer the student's follow-up question directly. Keep responses focused — 3-6 sentences unless a longer explanation is essential.`
}

func buildSeedPrompt(q domain.Question, response *domain.Response, evaluation *domain.Evaluation, chunks []rag.Chunk, firstMessage ...string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Question: %s\n", q.Question)
	if q.Format == "mcq" || q.Format == "scenario_mcq" {
		if q.Options != nil {
			for k, v := range q.Options {
				fmt.Fprintf(&sb, "  %s) %s\n", k, v)
			}
		}
		if q.Correct != "" {
			fmt.Fprintf(&sb, "Correct answer: %s\n", q.Correct)
		}
	}

	if response != nil {
		fmt.Fprintf(&sb, "\nStudent's answer: %s\n", response.SelectedAnswer)
		if response.Explanation != "" {
			fmt.Fprintf(&sb, "Student's explanation: %s\n", response.Explanation)
		}
	}

	if evaluation != nil {
		fmt.Fprintf(&sb, "\nEvaluation:\n")
		fmt.Fprintf(&sb, "  Correctness: %.0f%%\n", evaluation.Correctness*100)
		if evaluation.FeedbackForLearner != "" {
			fmt.Fprintf(&sb, "  Feedback: %s\n", evaluation.FeedbackForLearner)
		}
		if evaluation.MisconceptionIdentified != nil && *evaluation.MisconceptionIdentified != "" {
			fmt.Fprintf(&sb, "  Misconception: %s\n", *evaluation.MisconceptionIdentified)
		}
	}
	if q.CorrectExplanation != "" {
		fmt.Fprintf(&sb, "  Correct explanation: %s\n", q.CorrectExplanation)
	}

	appendRAGChunks(&sb, chunks)

	if len(firstMessage) > 0 && firstMessage[0] != "" {
		fmt.Fprintf(&sb, "\n\nStudent's first question: %s", firstMessage[0])
	} else {
		sb.WriteString("\nPlease open the tutoring conversation based on the above.")
	}
	return sb.String()
}

func buildFreshResumePrompt(q domain.Question, response *domain.Response, evaluation *domain.Evaluation, chunks []rag.Chunk, thread *domain.Thread, newUserMessage string) string {
	var sb strings.Builder

	// Re-establish full context since session expired.
	sb.WriteString("(Session context re-established after expiry)\n\n")
	sb.WriteString(buildSeedPrompt(q, response, evaluation, chunks))

	// Replay prior conversation turns so the model has history.
	if len(thread.Messages) > 0 {
		sb.WriteString("\n\nPrior conversation:\n")
		for _, m := range thread.Messages {
			role := "Student"
			if m.Role == "assistant" {
				role = "Tutor"
			}
			fmt.Fprintf(&sb, "\n%s: %s", role, m.Content)
		}
	}

	fmt.Fprintf(&sb, "\n\nStudent: %s", newUserMessage)
	return sb.String()
}

func appendRAGChunks(sb *strings.Builder, chunks []rag.Chunk) {
	if len(chunks) == 0 {
		return
	}
	sb.WriteString("\n\nRelevant course material (top-")
	fmt.Fprintf(sb, "%d sections by keyword relevance):\n", len(chunks))
	for _, c := range chunks {
		fmt.Fprintf(sb, "\n--- %s", c.File)
		if c.Heading != "" {
			fmt.Fprintf(sb, " > %s", c.Heading)
		}
		fmt.Fprintf(sb, " [~%d tok] ---\n%s\n", c.Tokens, c.Content)
	}
}
