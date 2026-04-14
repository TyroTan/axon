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

// Stream implements llm.Client. The model parameter is ignored — the CLI uses
// the account's default model. system + messages are concatenated into a single
// prompt passed via --print (stdin not used to avoid shell escaping issues).
func (c *Client) Stream(ctx context.Context, _, system string, messages []llm.Message, _ int) <-chan llm.Chunk {
	ch := make(chan llm.Chunk, 64)
	go func() {
		defer close(ch)
		if err := c.run(ctx, system, messages, ch); err != nil {
			ch <- llm.Chunk{Error: err}
		}
	}()
	return ch
}

// ─── internal ─────────────────────────────────────────────────────────────────

// cliEvent is the subset of stream-json fields we care about.
type cliEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	IsError bool   `json:"is_error"`
	Result  string `json:"result"` // present on type=result
	Message *struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"` // present on type=assistant
}

func (c *Client) run(ctx context.Context, system string, messages []llm.Message, ch chan<- llm.Chunk) error {
	// Build a single prompt string: system block + conversation turns.
	var sb strings.Builder
	if system != "" {
		sb.WriteString(system)
		sb.WriteString("\n\n---\n\n")
	}
	for _, m := range messages {
		if m.Role == "user" {
			sb.WriteString(m.Content)
		}
		// assistant turns are rare at generation time; append if present
		if m.Role == "assistant" {
			sb.WriteString("\n\nAssistant: ")
			sb.WriteString(m.Content)
			sb.WriteString("\n\nHuman: ")
		}
	}
	prompt := sb.String()

	cmd := exec.CommandContext(ctx, c.cliPath,
		"--print", prompt,
		"--output-format", "stream-json",
		"--verbose",
	)

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
			ch <- llm.Chunk{Done: true}
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
