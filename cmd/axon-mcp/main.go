// axon-mcp: MCP stdio server that RAG-searches axon root-level .md files.
// Exposes one tool: query_axon_docs(topic, token_budget?)
// Claude Code loads this via .mcp.json at project root.
//
// Build: go build -o axon-mcp ./cmd/axon-mcp
// Config: set AXON_ROOT env var or binary auto-detects from its location.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tyrohunt/axon/internal/rag"
)

const (
	defaultTokenBudget = 8000
	maxTokenBudget     = 20000
	chunkSize          = 800
	maxChunks          = 20
)

// ─── MCP protocol types ───────────────────────────────────────────────────────

type rpcRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Result  any              `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type queryArgs struct {
	Topic       string `json:"topic"`
	TokenBudget int    `json:"token_budget"`
}

// ─── main ─────────────────────────────────────────────────────────────────────

func main() {
	root := resolveRoot()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	enc := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}

		// Notifications have no id and expect no response.
		if req.ID == nil {
			continue
		}

		resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}

		switch req.Method {
		case "initialize":
			resp.Result = map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "axon-rag", "version": "1.0.0"},
			}

		case "tools/list":
			resp.Result = map[string]any{
				"tools": []any{toolDef()},
			}

		case "tools/call":
			var p callParams
			if err := json.Unmarshal(req.Params, &p); err != nil {
				resp.Error = &rpcError{Code: -32602, Message: "invalid params"}
				break
			}
			if p.Name != "query_axon_docs" {
				resp.Error = &rpcError{Code: -32601, Message: "unknown tool: " + p.Name}
				break
			}
			var args queryArgs
			json.Unmarshal(p.Arguments, &args) //nolint:errcheck — zero value is safe default
			clamp(&args.TokenBudget, defaultTokenBudget, maxTokenBudget)

			text, err := queryDocs(root, args.Topic, args.TokenBudget)
			if err != nil {
				resp.Error = &rpcError{Code: -32603, Message: err.Error()}
				break
			}
			resp.Result = map[string]any{
				"content": []any{map[string]any{"type": "text", "text": text}},
			}

		default:
			resp.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
		}

		enc.Encode(resp) //nolint:errcheck
	}
}

// ─── tool definition ──────────────────────────────────────────────────────────

func toolDef() map[string]any {
	return map[string]any{
		"name": "query_axon_docs",
		"description": "Search axon project .md files and return relevant sections. " +
			"Use before answering questions about: guardrails, track conventions, roadmap items, " +
			"sprint plans, concept definitions, session rules, or any project policy. " +
			"Files searched: CLAUDE.md, plan.md, plan_v2.md, roadmap_v2.md, how_to.md, " +
			"concept_taxonomy.md, experiments_log.md, DESIGN.md, ROADMAP.md, CHANGELOG.md.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"topic": map[string]any{
					"type":        "string",
					"description": "Topic, question, or keyword to search for in axon docs",
				},
				"token_budget": map[string]any{
					"type":        "number",
					"description": "Max tokens to return (default 8000, max 20000)",
				},
			},
			"required": []string{"topic"},
		},
	}
}

// ─── RAG query ────────────────────────────────────────────────────────────────

func queryDocs(root, topic string, tokenBudget int) (string, error) {
	files, err := loadMDFiles(root)
	if err != nil {
		return "", fmt.Errorf("load md files: %w", err)
	}
	if len(files) == 0 {
		return "No .md files found at AXON_ROOT=" + root, nil
	}

	chunks := rag.ChunkFiles(files, chunkSize)
	results := rag.TopK(chunks, topic, maxChunks, tokenBudget)

	if len(results) == 0 {
		return fmt.Sprintf("No relevant sections found for %q in %d files.", topic, len(files)), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "axon docs — %d sections for %q:\n\n", len(results), topic)
	for _, c := range results {
		fmt.Fprintf(&sb, "--- %s", c.File)
		if c.Heading != "" {
			fmt.Fprintf(&sb, " > %s", c.Heading)
		}
		fmt.Fprintf(&sb, " [score %.2f, ~%d tok] ---\n%s\n\n", c.Score, c.Tokens, c.Content)
	}
	return sb.String(), nil
}

func loadMDFiles(root string) (map[string]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	files := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			continue
		}
		files[e.Name()] = string(b)
	}
	return files, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func resolveRoot() string {
	if r := os.Getenv("AXON_ROOT"); r != "" {
		return r
	}
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	// Walk up from binary location until we find go.mod (axon root).
	dir := filepath.Dir(exe)
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return filepath.Dir(exe)
}

func clamp(v *int, def, max int) {
	if *v <= 0 {
		*v = def
	}
	if *v > max {
		*v = max
	}
}
