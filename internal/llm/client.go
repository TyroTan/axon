// Package llm defines the backend-agnostic LLM streaming interface.
package llm

import "context"

// Message is a single turn in a conversation.
type Message struct {
	Role    string // "user" or "assistant"
	Content string
}

// Chunk is one piece of a streaming response.
type Chunk struct {
	Text  string // incremental text (may be empty on Done/Error)
	Done  bool   // true on the final chunk (Text may still carry last text)
	Error error  // non-nil means the stream failed
}

// Client streams responses from an LLM.
type Client interface {
	// Stream sends a request and returns a channel of chunks.
	// The channel is closed after the last chunk (Done=true or Error≠nil).
	// model is a provider-specific model ID (e.g. "claude-opus-4-6").
	Stream(ctx context.Context, model, system string, messages []Message, maxTokens int) <-chan Chunk
}
