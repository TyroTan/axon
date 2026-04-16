// Package claudecli implements llm.Client by shelling out to the `claude` CLI.
// No API key required — uses the CLI's existing authentication.
package claudecli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/tyrohunt/axon/internal/llm"
)

const defaultCLIPath = "claude"

// Client wraps the claude CLI binary.
type Client struct {
	cliPath string // path to the claude binary, defaults to "claude"
}

// New creates a claudecli Client. Pass "" to use "claude" from PATH.
func New(cliPath string) *Client {
	if cliPath == "" {
		cliPath = defaultCLIPath
	}
	return &Client{cliPath: cliPath}
}

// Stream implements llm.Client. Fresh call, no session reuse.
func (c *Client) Stream(ctx context.Context, _, system string, messages []llm.Message, _ int) <-chan llm.Chunk {
	return c.stream(ctx, "", system, messages)
}

// StreamResume implements llm.Client. Passes --resume sessionID to the CLI
// when sessionID is non-empty. If the session has expired, the CLI starts
// fresh — the Done chunk will carry the new session ID.
func (c *Client) StreamResume(ctx context.Context, sessionID, _, system string, messages []llm.Message, _ int) <-chan llm.Chunk {
	return c.stream(ctx, sessionID, system, messages)
}

// ─── internal ─────────────────────────────────────────────────────────────────

// cliEvent is the subset of stream-json fields we care about.
type cliEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	IsError bool   `json:"is_error"`
	Result  string `json:"result"`    // present on type=result
	SessionID string `json:"session_id"` // conversation session — set on result event
	Usage   *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"` // token usage — set on result event
	Message *struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"` // present on type=assistant
}

func (c *Client) stream(ctx context.Context, sessionID, system string, messages []llm.Message) <-chan llm.Chunk {
	ch := make(chan llm.Chunk, 64)
	go func() {
		defer close(ch)
		if err := c.run(ctx, sessionID, system, messages, ch); err != nil {
			ch <- llm.Chunk{Error: err}
		}
	}()
	return ch
}

func (c *Client) run(ctx context.Context, sessionID, system string, messages []llm.Message, ch chan<- llm.Chunk) error {
	// When resuming, we only send the new user message — the session carries
	// the prior context. When fresh, we concatenate system + all messages.
	var prompt string
	if sessionID != "" && len(messages) > 0 {
		// Only send the latest user message; claude --resume has the rest.
		last := messages[len(messages)-1]
		prompt = last.Content
	} else {
		var sb strings.Builder
		if system != "" {
			sb.WriteString(system)
			sb.WriteString("\n\n---\n\n")
		}
		for _, m := range messages {
			if m.Role == "user" {
				sb.WriteString(m.Content)
			}
			if m.Role == "assistant" {
				sb.WriteString("\n\nAssistant: ")
				sb.WriteString(m.Content)
				sb.WriteString("\n\nHuman: ")
			}
		}
		prompt = sb.String()
	}

	args := []string{
		"--print", prompt,
		"--output-format", "stream-json",
		"--verbose",
	}
	if sessionID != "" {
		args = append(args, "--resume", sessionID)
	}

	cmd := exec.CommandContext(ctx, c.cliPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("claudecli: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("claudecli: start: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1 MB line buffer for large responses

	var finalText string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev cliEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue // skip non-JSON lines (e.g. stderr leaked to stdout)
		}

		switch ev.Type {
		case "assistant":
			// Stream each text block as it arrives.
			if ev.Message != nil {
				for _, block := range ev.Message.Content {
					if block.Type == "text" && block.Text != "" {
						ch <- llm.Chunk{Text: block.Text}
						finalText += block.Text
					}
				}
			}
		case "result":
			if ev.IsError {
				return fmt.Errorf("claudecli: error result: %s", ev.Result)
			}
			// result.result is the full accumulated text — use it if we got
			// no streaming text (e.g. older CLI versions).
			if finalText == "" && ev.Result != "" {
				ch <- llm.Chunk{Text: ev.Result}
			}
			// Emit session ID and token usage on the Done chunk.
			inputTokens := 0
			if ev.Usage != nil {
				inputTokens = ev.Usage.InputTokens
			}
			ch <- llm.Chunk{Done: true, SessionID: ev.SessionID, InputTokens: inputTokens}
			_ = cmd.Wait()
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("claudecli: scan: %w", err)
	}

	_ = cmd.Wait()
	ch <- llm.Chunk{Done: true}
	return nil
}
