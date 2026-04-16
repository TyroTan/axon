// Package anthropic implements llm.Client against the Anthropic Messages API.
// Uses raw HTTP + SSE — no SDK dependency.
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tyrohunt/axon/internal/llm"
)

const apiURL = "https://api.anthropic.com/v1/messages"
const anthropicVersion = "2023-06-01"

// Client is an Anthropic Messages API client.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// New creates an Anthropic client with the given API key.
func New(apiKey string) *Client {
	return &Client{apiKey: apiKey, httpClient: &http.Client{}}
}

// Stream implements llm.Client. Sends chunks over the returned channel;
// closes the channel when the stream ends (Done=true) or errors.
func (c *Client) Stream(ctx context.Context, model, system string, messages []llm.Message, maxTokens int) <-chan llm.Chunk {
	ch := make(chan llm.Chunk, 64)
	go func() {
		defer close(ch)
		if err := c.stream(ctx, model, system, messages, maxTokens, ch); err != nil {
			ch <- llm.Chunk{Error: err}
		}
	}()
	return ch
}

// StreamResume implements llm.Client. The Anthropic HTTP API does not support
// session-based resumption, so sessionID is ignored and this behaves like Stream.
func (c *Client) StreamResume(ctx context.Context, _ /*sessionID*/, model, system string, messages []llm.Message, maxTokens int) <-chan llm.Chunk {
	return c.Stream(ctx, model, system, messages, maxTokens)
}

// ─── internal ─────────────────────────────────────────────────────────────────

type requestBody struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system"`
	Messages  []apiMessage `json:"messages"`
	Stream    bool         `json:"stream"`
}

type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type sseEvent struct {
	Type  string          `json:"type"`
	Delta *sseDelta       `json:"delta,omitempty"`
	Error *sseError       `json:"error,omitempty"`
}

type sseDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type sseError struct {
	Message string `json:"message"`
}

func (c *Client) stream(
	ctx context.Context,
	model, system string,
	messages []llm.Message,
	maxTokens int,
	ch chan<- llm.Chunk,
) error {
	apiMsgs := make([]apiMessage, len(messages))
	for i, m := range messages {
		apiMsgs[i] = apiMessage{Role: m.Role, Content: m.Content}
	}

	body := requestBody{
		Model:     model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  apiMsgs,
		Stream:    true,
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("anthropic: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("anthropic: new request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("anthropic: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("anthropic: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			ch <- llm.Chunk{Done: true}
			return nil
		}

		var ev sseEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue // skip unparseable lines
		}

		switch ev.Type {
		case "content_block_delta":
			if ev.Delta != nil && ev.Delta.Type == "text_delta" {
				ch <- llm.Chunk{Text: ev.Delta.Text}
			}
		case "message_stop":
			ch <- llm.Chunk{Done: true}
			return nil
		case "error":
			if ev.Error != nil {
				return fmt.Errorf("anthropic: stream error: %s", ev.Error.Message)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("anthropic: scan: %w", err)
	}
	ch <- llm.Chunk{Done: true}
	return nil
}
