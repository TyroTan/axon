---
title: "Goal: Browser Chat as Full CLI Equivalent"
type: project-plan
status: in-progress
created: 2026-04-10
---

# Goal: Browser Chat as Full CLI / VSCode Equivalent

The ultimate goal is that a developer can replace their VSCode CLI session with the
browser-based proxy to `runClaude` — with **no loss of capability**. This means:
- Conversation context survives across turns (same as `--resume` in CLI)
- RAG retrieval is iterative and tool-driven (same as calling `SpecsAISystem_search` mid-task)
- File operations are observed, logged, and auditable (via MCP proxy tools)
- Every ply is traceable to a conversation, workspace, and machine
- Startup doesn't silently fail due to misconfiguration

---

## Feasibility Verdict

**Yes — feasible. The core pipeline (`runClaude` + `--resume` + `buildResumePrompt` +
poller crash-safety) is already built and proven.** The gaps are not architectural —
they are missing wiring, missing fields, and deferred pipeline work.
13 items total. Estimated total hardness: ~330/100. No single item is a blocker;
B-P7 and B-P12 are the highest-effort items. B-P3 is the highest-risk (MCP wiring
requires CLI flag verification).

---

## Blocking Dependency Order

```
B-P2 (machine.yml + startup validation) ← DONE
  └── enables: clean go run . for all developers

B-P6 (runClaude subprocess → proxy_bus routing)           ← SPRINT 1, item 1
  └── enables: automated runClaude traffic flows through proxy at all
      └── B-P1 (proxy_logs conversation linkage)          ← SPRINT 1, item 2
              └── enables: per-ply conversation + machine linkage fields in proxy_logs
                  └── B-P1 + B-P6 together = full ply observability

B-P8 (sessions.messages — D-002)                          ← SPRINT 1, item 3
  └── enables: browser conversation history / replay (independent of B-P6/B-P1)

B-P4a (fix source_file to relative path)                  ← SPRINT 1, item 4
  └── enables: corpora portable across machines (prerequisite for B-P9 + B-P4b)

B-P13 (compaction: buildResumePrompt + token budget)      ← SPRINT 1, item 5
  └── enables: long conversation correctness without context overflow

B-P7 (execution ply gate — D-001)                         ← SPRINT 2
  └── enables: 1-turn file editing (true CLI parity) + stale-processing recovery
      └── depends on: B-P3 for confirm_plan MCP tool

B-P9 (multi-instance mode + machine affinity)              ← SPRINT 2
  └── depends on: B-P4a (relative paths) for full correctness
  └── enables: correct fallback rebuilds in multi-instance/multi-mode deployments

B-P3 (MCP tools wired into claude -p subprocess)          ← SPRINT 2
  └── enables: iterative RAG + proxy tool observability mid-task
      └── B-P10 (long-running MCP tools — timeout + poller)  ← SPRINT 2 (after B-P3)
      └── B-P11 (SpecsAISystem_ingest readiness wait)         ← SPRINT 2 (after B-P3)
      └── B-P12 (AST-first routing: function body + reindex)  ← SPRINT 2 (after B-P3)
          └── enables: 90% token savings on code-modification tasks
      └── B-P4b (ingest studio corpora into MongoDB)          ← SPRINT 2 (after B-P3)
              └── enables: SpecsAISystem_search returns studio-specific context

B-P5 (context management boundary test)                   ← any time (empirical)
  └── independent — run against a live long session; no implementation
```

**Sequential constraints:** B-P6 before B-P1 (traffic must flow through proxy before fields matter).
B-P3 before B-P10/B-P11/B-P12/B-P4b. B-P4a before B-P9 for full correctness.
**Parallel-safe within sprint:** B-P8, B-P4a, B-P13 are fully independent of each other and of B-P6/B-P1.

---

## Items

---

### B-P1 — proxy_logs missing conversation linkage fields
**Status:** Done (2026-04-11)  
**Hardness:** 15/100  
**Blocks:** Ply traceability, browser UUID correlation, audit joins

**Problem (code-grounded):**  
`ProxyLogEntry` in `features/proxy_bus/handler.go:26-35` stores only:
`session_id`, `request_path`, `request_body`, `response_body`, `status_code`,
`duration_ms`, `created_at`. No `workspace_id`, no `conversation_id`, no `machine_id`.

`session_id` is extracted from `metadata.session_id` in the request body (line 119).
If the browser doesn't send this field in metadata, the entry has no session link at all.

The A2A side already has `ConversationID + ParentSessionID + TurnNumber`
(`features/a2a/session_document.go:27-29`). proxy_logs are orphaned from this chain.

**What "browser UUID" means here:**  
The browser generates a stable UUID per tab/session (or per conversation). This UUID
should flow through every Anthropic API call as `metadata.user_id` (Anthropic's standard
field) or in a custom header. The proxy intercepts all calls — it can extract this UUID
from any request header or body field.

**Spec:**
- Add fields to `ProxyLogEntry`:
  - `WorkspaceID string` — from config at proxy startup (already known)
  - `ConversationID string` — extracted from request header `X-Conversation-ID`
    or `metadata.conversation_id` in body
  - `MachineID string` — from `cfg.DistProcessorID` (already stable per machine)
  - `UserID string` — from `metadata.user_id` in request body (Anthropic standard)
- Browser (or CLI env) must send `X-Conversation-ID` header on every API call
- For CLI path: set `ANTHROPIC_CONVERSATION_ID` env var; proxy reads it from header
- For browser path: browser generates UUID at conversation start, sends as header

**Acceptance test:**
- POST two turns to `/v1/messages` with same `X-Conversation-ID`
- GET `/proxy_logs` — both entries have matching `conversation_id`
- Join proxy_logs entry with `a2a_sessions` on `conversation_id` → works

**Changelog:** —

---

### B-P2 — machine.yml + startup validation alignment
**Status:** Done (2026-04-10)  
**Hardness:** 20/100  
**Blocks:** Clean `go run .` without env var hunt; machine-local config portability

**Problem:**  
`config_validator.Validate()` hard-errors on `DB_CONNECTION`, `DB_NAME`,
`DIST_PROCESSOR_ID`, and `CLAUDE_CLI_PATH` (`validator.go:34-36, 69-76`).

`CLAUDE_CLI_PATH` is the only path-dependent hard error — it fails when nvm is
not on PATH (documented: CLI resolves via `exec.LookPath` which misses nvm in
non-interactive shells). All other hard errors are service config, not path config.

**machine.yml scope (confirmed minimal):**
```yaml
# .specsai/machine.yml — gitignored, machine-local
machine_id: tyrohunt-macbook
claude_cli_path: /Users/tyrohunt/.nvm/versions/node/v24.11.1/bin/node  # resolves CLAUDE_CLI_PATH hard error
workspace_root: /Users/tyrohunt/Moonshot Work/repos/specs-ai-dist-processor  # for ActiveModePath check
sub_repos:
  specs_ai_studio: /Users/tyrohunt/Moonshot Work/repos/specs-ai-dist-processor/specs-ai-studio
mcp:
  specsaisystem: http://localhost:3002
  ast_intel: http://localhost:3003
```

machine.yml does NOT replace `.env`. It only covers path-dependent facts.
All service config (`DB_CONNECTION`, `RUN_MODE`, `HAS_QDRANT`, ports, etc.) stays in `.env`.

**Spec:**
- Parse `.specsai/machine.yml` in `config/config.go` before env var resolution
- If `claude_cli_path` is present in machine.yml AND env var `CLAUDE_CLI_PATH` is not set,
  use machine.yml value — env var still wins if set (12-factor precedence)
- `workspace_root` in machine.yml overrides auto-detection from `workspace.yml` location
- Add `file_exists` check for `claude_cli_path` value at startup (not just non-empty)
- Add `.specsai/machine.yml` to `.gitignore`
- Add `.specsai/machine.yml.example` (committed) with placeholder values

**Acceptance test:**
- Remove `CLAUDE_CLI_PATH` from `.env`
- Add correct path to `.specsai/machine.yml`
- `go run .` passes startup validation with no errors
- Change `claude_cli_path` to a non-existent path → startup fails with descriptive error
- Restore correct path → passes again

**Changelog:** —

---

### B-P3 — MCP tools not wired into `claude -p` subprocess
**Status:** Not started  
**Hardness:** 35/100  
**Blocks:** Iterative RAG during execution; SpecsAISystem_search usable mid-task

**Problem:**  
`runClaude()` in `handler.go:248-269` spawns `claude -p` with:
`--output-format stream-json --verbose --system-prompt <gate> [--dangerously-skip-permissions] [--resume <id>]`

No `--mcp-config` flag is passed. The subprocess has no MCP server configured.
This means inside a `claude -p` execution, Claude has NO access to:
- `SpecsAISystem_search` (iterative RAG)
- `SpecsAISystem_context` (workspace-scoped context pull)
- `SpecsAISystem_ingest` (ingest new findings)
- `SpecsAISystem_read/write/bash` (proxy tool calls for observability)

The CLI/VSCode experience has all these tools because the developer's
`.claude/settings.json` or `--mcp-config` is loaded by the interactive session.
The `claude -p` subprocess inherits none of this.

**Spec:**
- Generate a per-invocation MCP config JSON at subprocess start:
  ```json
  {
    "mcpServers": {
      "SpecsAISystem": {
        "type": "sse",
        "url": "http://localhost:3002/sse"
      }
    }
  }
  ```
- Write to a temp file (or pass via `--mcp-config` flag if supported)
- Add `--mcp-config <path>` to `runClaude` args
- Verify: check Claude Code CLI `--mcp-config` flag exists (may need version check)
- System prompt must instruct Claude to call `SpecsAISystem_search` before
  making assumptions about codebase structure (replaces current one-shot enrichPrompt)
- `enrichPrompt` becomes a seed/bootstrap injection only — Claude supplements via MCP

**Risk:** `--mcp-config` flag availability depends on Claude Code CLI version.
If not available, alternative is generating a `.claude/` directory in a temp workspace
and pointing `claude -p` at it via `--cwd`.

**Acceptance test:**
- Submit a query that requires codebase lookup ("where is runClaude defined?")
- Check `a2a_sessions` or `proxy_logs` for `SpecsAISystem_search` tool call in `tools_used`
- Claude's response cites a line number from the index (not a hallucinated one)
- Confirm MCP server logs show the search request

**Changelog:** —

---

### B-P4 — Studio corpora not ingested into MongoDB (I-02)
**Status:** Deferred (dependency: B-P3 must be done first — no point ingesting if search isn't wired)  
**Hardness:** 25/100  
**Blocks:** `SpecsAISystem_search` returning studio-specific context (line maps, guardrails, agent defs)

**Problem:**  
`active_mode: core` means only corpora tagged `core` are ingested at startup.
The studio line maps (`MANIFEST.md`, `src__app__pages__Specifications.md`, etc.),
agent definitions, and guardrail `.md` files in `.workspaces/studio/corpora/` are
filesystem-only. `SpecsAISystem_search` returns nothing for studio-specific queries.

`auto_ingest.go` has a secondary bug: `source_file` stores absolute path (line 80),
making corpora non-portable across machines. Must fix before ingesting studio corpora.

**Spec (two sub-tasks):**

**B-P4a — Fix `source_file` to relative path (B-01):**
- In `features/corpora/command/auto_ingest.go:80`, replace:
  `"source_file": filePath`
  with:
  `"source_file": strings.TrimPrefix(filePath, cfg.WorkspaceRoot+"/")`
- Requires `cfg.WorkspaceRoot` passed into the ingest function
- Re-ingest all existing corpora to backfill relative paths

**B-P4b — Ingest studio corpora:**
- Add `workspace_id: specs-ai-studio-init` + `tags: ["studio"]` to corpora frontmatter
- Configure `active_mode: studio` (or add studio to core tags) to trigger auto-ingest
- Verify MANIFEST.md and line_maps/*.md are ingested and searchable
- Test: `SpecsAISystem_search("Specifications.tsx browse view line range")` returns
  correct line range from the index

**Acceptance test:**
- `GET /corpora?workspace_id=specs-ai-studio-init` returns studio line map entries
- `source_file` values are relative (no absolute paths)
- `SpecsAISystem_search("FrontendBuilderView props")` returns result from studio corpus
- Result survives machine transfer (clone on different machine, same relative paths)

**Changelog:** —

---

### B-P6 — `runClaude` subprocess bypasses proxy_bus (observability gap)
**Status:** Done (2026-04-11)  
**Hardness:** 10/100  
**Blocks:** Full ply auditability; proxy_logs being a complete record of all token spend

**Problem:**  
`StartProxyServer` (`features/proxy_bus/handler.go:147`) intercepts API calls from
clients that point `ANTHROPIC_BASE_URL` at `localhost:ProxyPort`. But `runClaude()`
spawns `claude -p` as a subprocess — that subprocess calls `api.anthropic.com` directly
unless `ANTHROPIC_BASE_URL` is in its inherited environment.

The server process's env is not guaranteed to have this var set. Even if it is set for
the developer's shell, it won't be set in production/CI. So all automated `runClaude`
token spend is invisible to proxy_logs — the very audit trail the system is built around.

The goal states "file operations are observed, logged, and auditable." Without this fix,
only browser-direct API calls are logged. Server-side claude invocations are not.

**Spec:**
- In `runClaude()` (`handler.go:248`), explicitly set `ANTHROPIC_BASE_URL` in the
  subprocess environment when `cfg.ProxyEnabled == true`:
  ```go
  cmd.Env = append(os.Environ(),
    "ANTHROPIC_BASE_URL=http://localhost:"+cfg.ProxyPort,
  )
  ```
- Add `ANTHROPIC_API_KEY` forwarding to subprocess env (already in parent env via `.env`,
  but explicit forwarding ensures it survives env isolation in containers)
- Add a startup log line: `[query_dispatch] proxy routing enabled — claude subprocess will use proxy at :<ProxyPort>`
- Verify: after enabling, `GET /proxy_logs` shows entries with `request_path: /v1/messages`
  from automated runClaude invocations (not just browser calls)

**Acceptance test:**
- Submit a query via the browser → observe `runClaude` log: "proxy routing enabled"
- `GET /proxy_logs` — entry exists for the `/v1/messages` call made by the subprocess
- `session_id` in that proxy_log entry matches the query's `session_id`
- Disable `PROXY_ENABLED` → subprocess calls Anthropic directly again (no proxy_log entry)

**Changelog:** —

---

### B-P7 — Execution ply forced gate breaks CLI parity (D-001 core gap)
**Status:** Not started (tracked in DEFERRED.md D-001)  
**Hardness:** 40/100  
**Blocks:** Full CLI equivalence for file-editing tasks without forced QuestionCard round-trip

**Problem:**  
This is the most direct capability difference between browser and CLI paths:

- **CLI:** `--dangerously-skip-permissions` from turn 1. Claude can read, edit, write,
  run bash in the same turn it plans. No gate.
- **Browser path:** First invocation runs without `--dangerously-skip-permissions`
  (planning ply). Claude can only plan and call `AskUserQuestion`. Must call
  `AskUserQuestion` before any file-editing operation (enforced via system prompt, not
  structurally). User clicks QuestionCard. Second invocation (execution ply) gets the flag.

This means **every file-editing task requires at minimum 2 turns on the browser path**.
CLI does it in 1. That's not a UX difference — it's a structural capability gap.

Additionally: `TaskStalThresholdSec` is configured in `config.go` and checked in
`config_validator.go` but is wired to nothing in the query pipeline. Queries stuck in
`StepQueryProcessing` beyond the threshold are never recovered — they rot. This makes
long tasks (multi-minute Claude invocations) silently fail with no retry.

**Spec (from DEFERRED.md D-001):**
- `confirm_plan` MCP tool: `confirm_plan(description, files[], actions[])` — stores
  plan as typed artifact on query document, parks the query (same as `AskUserQuestion`
  but structured). Browser renders a `PlanCard` (not a generic `QuestionCard`).
- `ExecutionMode string` on `QueryDocument`: set to `"post_approval"` when the resume
  query comes from a PlanCard answer
- `--dangerously-skip-permissions` added only when `ExecutionMode == "post_approval"`
- `TaskStalThresholdSec` wired: queries in `StepQueryProcessing` beyond threshold reset
  to `StepQueryPending` for re-claim (same pattern as corpora pipeline's retry logic)
- `StepQueryAwaitingPlanApproval` step hash registered in `QueryMachine()`

**Acceptance test:**
- Submit a task that edits a file → Claude calls `confirm_plan` → `PlanCard` shown in browser
- User approves → execution ply runs with flag → file is edited → `SetCompleted`
- Kill server mid-execution → restart → stale query detected → re-claimed → completes

**Changelog:** —

---

### B-P8 — `sessions.messages` always empty (D-002: conversation replay gap)
**Status:** Done (2026-04-11)  
**Hardness:** 15/100  
**Blocks:** Browser showing conversation history; "what did we discuss last turn?" context

**Problem:**  
`UpsertSession` in `features/session_store/command/command.go` creates sessions with
`messages: []` and they stay empty. The query pipeline writes to the `queries` collection
only — nobody calls `AppendMessage`.

For the browser to function as a conversation interface (not just fire-and-forget), it
needs to surface message history: the user's prompt, Claude's response, any questions
asked. Without this, there is no conversation thread visible in the browser UI. The
CLI implicitly has this (you see your terminal history). The browser has nothing.

**Spec:**
- In `query_dispatch` handler, after `SetCompleted`: call `AppendMessage` twice:
  - `{role: "user", content: doc.EnrichedPrompt}` (the full enriched prompt sent to Claude)
  - `{role: "assistant", content: result}` (Claude's output)
- If `WaitingForInput` (AskUserQuestion): append the question as a third message:
  `{role: "assistant", content: doc.WaitingQuestion.Question, type: "question"}`
- `GET /sessions/:id` response verified to include populated `messages[]`
- Sidebar component updated to show `last_message` preview from `messages[-1].content[:80]`

**Acceptance test:**
- Submit a query → `GET /sessions/<id>` returns `messages` array with user + assistant entries
- Multi-turn conversation → each turn appended to `messages[]` in order
- AskUserQuestion mid-conversation → question appears as `type: "question"` message
- Sidebar shows last message preview without extra query to `queries` collection

**Changelog:** —

---

### B-P9 — Multi-instance stale-session fallback claims wrong processor (mode + machine affinity)
**Status:** Not started  
**Hardness:** 30/100  
**Blocks:** Correctness of context rebuild in any multi-instance or multi-mode deployment

**Problem (code-grounded):**  
When a session goes stale, `SetPendingFallback` (`command.go:177-184`) transitions the
query to `StepQueryPendingFallback` but records no constraint on which processor may
reclaim it. The poller filter (`poller.go:35-46`) selects by `workspace_id` only — no
`active_mode`, no processor affinity. Any instance in the same workspace claims it.

Three failure modes:

**Failure A — Wrong mode claims the fallback:**  
Instance A (`active_mode=core`) processes the original query. `enrichPrompt` injected
mode=core corpora chunks into `enriched_prompt`. Session goes stale. Instance B
(`active_mode=studio`) claims the fallback. `buildResumePrompt` uses the stored
`enriched_prompt` (mode=core context) but any new enrichment on the fallback ply
would use mode=studio tags — inconsistent context in the same conversation turn.

**Failure B — Wrong machine claims the fallback:**  
`claude -p` subprocess reads files from the claiming instance's local `WorkspaceRoot`.
If Instance B has a different `WorkspaceRoot` (different machine, different repo clone
location), Claude reads from a different filesystem than the conversation started on.
With `source_file` stored as absolute paths (B-P4a), the corpora chunks in the stored
`enriched_prompt` reference Instance A's absolute paths — those paths don't exist on
Instance B's filesystem if Claude tries to follow them.

**Failure C — Mode not recorded on the document:**  
`QueryDocument` has no `active_mode` field (`command.go:34-60`). Mode is config-local
at the claiming instance. There's no way to query "which mode was this conversation
started in?" from MongoDB — you'd have to inspect the stored `enriched_prompt` heuristically.

**What a correct design looks like:**

1. **Record `active_mode` and `machine_id` on `QueryDocument` at creation** — not derived
   from the claiming instance's config, but from the config of the instance that accepted
   the original request.

2. **`SetPendingFallback` sets `sticky_processor_id`** — the processor that detected the
   stale session pins the fallback to itself. Only that instance (by `dist_processor_id`)
   may reclaim `StepQueryPendingFallback` items it created.

3. **Poller filter adds `sticky_processor_id` gate:**
   ```go
   filter := bson.M{
     "step_hash": bson.M{"$in": []string{StepQueryPending, StepQueryPendingFallback}},
     "workspace_id": cfg.WorkspaceID,
     "$or": []bson.M{
       // Normal pending items: no sticky constraint (any processor in workspace can claim)
       {"step_hash": StepQueryPending, "sticky_processor_id": bson.M{"$exists": false}},
       // Fallback items: only the original processor can reclaim
       {"step_hash": StepQueryPendingFallback, "sticky_processor_id": cfg.DistProcessorID},
     },
     ...backoff gate...,
   }
   ```

4. **Sticky affinity timeout (escape hatch):** if the original processor is down, the
   sticky fallback rots. Add `sticky_expires_at` (default: `now + 5 * PollIntervalMs`).
   After expiry, any processor in the workspace + mode can claim it (degraded recovery).

5. **Mode-scoped poller filter for all items:**  
   Add `active_mode` to both `QueryDocument` and poller filter. This ensures any instance
   running `mode=studio` never processes a conversation that started in `mode=core`, even
   for normal (non-fallback) items. Prevents mode drift across a long conversation.

**Schema changes:**
```go
// In QueryDocument:
ActiveMode         string     `bson:"active_mode,omitempty" json:"active_mode,omitempty"`       // mode at query creation time
MachineID          string     `bson:"machine_id,omitempty" json:"machine_id,omitempty"`          // machine.yml machine_id
StickyProcessorID  string     `bson:"sticky_processor_id,omitempty" json:"sticky_processor_id,omitempty"` // only this processor may claim (fallback items)
StickyExpiresAt    *time.Time `bson:"sticky_expires_at,omitempty" json:"sticky_expires_at,omitempty"`    // after this, any mode-matching processor may claim

// In CreateQueryDTO:
ActiveMode string `json:"-"` // set by controller from config.ActiveMode
MachineID  string `json:"-"` // set by controller from config (machine.yml machine_id if available)
```

**Acceptance test:**
- Run two instances: same `workspace_id`, different `active_mode` (core vs studio)
- Submit query on mode=core instance → session goes stale → `SetPendingFallback` fires
- mode=studio instance's poller tick → does NOT claim the fallback (mode mismatch)
- mode=core instance's next tick → claims it (mode + sticky_processor_id match) → `buildResumePrompt` runs
- Kill mode=core instance → wait for `sticky_expires_at` → mode=core instance on a different
  machine claims it (mode matches, sticky expired) → degraded but correct

**Dependency:** B-P4a (fix `source_file` to relative path) should land before this, so
the `enriched_prompt` corpora references are portable. Otherwise sticky-processor affinity
is the only thing preventing wrong-machine context bleed.

**Changelog:** —

---

### B-P10 — Long-running MCP tool calls have no timeout, no crash recovery, no observability
**Status:** Not started  
**Hardness:** 40/100  
**Blocks:** B-P3 being safe to enable; any multi-minute automated task using `SpecsAISystem_bash`

**Problem (code-grounded):**  
`toolProxyBash` (`proxy_tools.go:118-170`) calls `cmd.Run()` with no timeout. A
`go test ./...` or `go build` call blocks the MCP server goroutine indefinitely. No
MongoDB record is written — if the server crashes mid-execution, the result is lost.

More critically: when `claude -p` is wired via `--mcp-config` (B-P3), the subprocess
is blocked waiting for the SSE tool response. There is no visibility into whether the
tool is still running or hung. If the MCP server crashes:
- SSE channel closes
- `claude -p` subprocess gets a broken connection and fails with an error
- `runClaude()` returns an error
- Query document transitions to `SetFailed` or retry
- The bash command may or may not have actually executed — no record either way

This is different from the stale-session problem (B-P7): that's Claude's session TTL.
This is the MCP tool layer crashing under a running Claude invocation. The query pipeline
has no way to distinguish "Claude failed" from "MCP server crashed mid-tool-call."

**Two tiers of fix:**

**Tier 1 — Timeout + in-flight logging (15/100):**  
Add per-tool timeouts to `cmd.Run()` via `exec.CommandContext`. Log tool call start/end
to MongoDB (extend `mcpInvocation` to persist to a `mcp_tool_calls` collection, not just
the in-memory ring buffer). This makes tool calls observable and prevents indefinite hangs.

```go
// In toolProxyBash:
ctx, cancel := context.WithTimeout(ctx, 5*time.Minute) // configurable cap
defer cancel()
cmd := exec.CommandContext(ctx, "sh", "-c", args.Command)
```

**Tier 2 — Poller paradigm for non-deterministic/long-running tools (40/100):**  
Model long-running tool calls as state machine jobs:

```
Claude calls SpecsAISystem_bash("go test ./...") 
  → MCP tool creates mcp_job document: {job_id, command, step_hash: "pending", created_at}
  → Returns immediately: {"job_id": "...", "status": "queued", "message": "use SpecsAISystem_await(job_id) to get result"}

mcp_job_poller (new goroutine):
  → Claims pending jobs → executes → writes result → step_hash: "completed" | "failed"
  → Crash-safe: poller re-claims stale jobs (same pattern as query_dispatch)

Claude calls SpecsAISystem_await("job_id", timeout_ms=30000)
  → Polls mcp_jobs collection until completed | timeout
  → Returns result or: {"status": "timeout", "elapsed_ms": N, "job_id": "..."}
```

**Which tools need which tier:**

| Tool | Tier | Reason |
|---|---|---|
| `SpecsAISystem_bash` (read-only: ls, grep, git status) | Tier 1 only | Fast, deterministic |
| `SpecsAISystem_bash` (build/test: go build, go test) | Tier 2 | Minutes, non-deterministic |
| `SpecsAISystem_write` | Tier 1 only | Fast, synchronous |
| `SpecsAISystem_read` | None needed | Already fast |
| `SpecsAISystem_ingest` | See B-P11 | Special case |
| `SpecsAISystem_search/context` | None needed | Fast MongoDB reads |

**Classification heuristic for bash:** check first word of command against a "slow commands"
set (`go`, `npm`, `pnpm`, `make`, `cargo`) → Tier 2. All others → Tier 1 with timeout.

**Acceptance test:**
- Call `SpecsAISystem_bash("go test ./...")` → returns `{job_id}` immediately
- `GET /mcp_jobs/:id` → shows `step_hash: processing`
- After completion → `step_hash: completed`, `result: "..."`
- Kill MCP server mid-execution → restart → job re-claimed → completes
- `SpecsAISystem_await(job_id)` returns result to Claude
- Call `SpecsAISystem_bash("ls -la")` → returns result immediately (no job created)

**Dependency:** B-P3 (MCP wired into subprocess) must land before this matters in
production. But Tier 1 (timeout + persistent logging) can and should be implemented
independently now — it protects the existing CLI/VSCode path too.

**Changelog:** —

---

### B-P11 — `SpecsAISystem_ingest` returns before content is searchable
**Status:** Not started  
**Hardness:** 20/100  
**Blocks:** Claude-driven ingest-then-search workflows being correct; agentic feedback loops

**Problem (code-grounded):**  
`toolIngest` (`server.go:307-323`) calls `command.Create` which does a single `InsertOne`
into the corpora collection at `step_1_corpora_raw_pending`. It returns immediately:
`"ingested: id=X step_hash=raw_pending"`.

The corpora pipeline (separate poller) hasn't run yet. The embedding chain is:
```
raw_pending → naive_embed_queued → naive_embed_done → graph_build_queued → graph_build_done
```
The document isn't searchable until at minimum `naive_embed_done`. This takes at least
one full poller cycle (`POLL_INTERVAL_MS`, default 5000ms) plus embedding time.

**Failure scenario:** Claude is doing an agentic task:
1. Discovers new architectural fact during execution
2. Calls `SpecsAISystem_ingest("New finding: X pattern is used here", content)`
3. Gets `"ingested: id=abc step_hash=raw_pending"`
4. Continues task, later calls `SpecsAISystem_search("X pattern")` to verify
5. Gets zero results — corpus is still in the embedding pipeline
6. Claude concludes the ingest failed or the fact doesn't exist in the knowledge base
7. Either retries (duplicate ingest) or proceeds on incorrect assumption

This breaks the "ingest → learn → use" feedback loop that is the core value of having
`SpecsAISystem_ingest` available mid-task.

**Spec:**

**Option A — Return readiness estimate (low-effort, imprecise):**  
`toolIngest` returns:
```json
{
  "id": "abc",
  "step_hash": "raw_pending",
  "estimated_ready_ms": 8000,
  "message": "ingested — searchable in ~8s. Call SpecsAISystem_search after that delay."
}
```
Claude waits the estimated time before searching. Fragile — estimate is a guess.

**Option B — `SpecsAISystem_ingest_status(id)` polling tool (recommended):**  
New tool that returns current `step_hash` for an ingested document. Claude polls until
`naive_embed_done` or `graph_build_done`. System prompt instructs: "after ingesting,
always call `SpecsAISystem_ingest_status(id)` until status is 'naive_embed_done' before
searching for the ingested content."

```go
// New tool: SpecsAISystem_ingest_status
type ingestStatusArgs struct { ID string `json:"id"` }
// Returns: {"id": "abc", "step_hash": "naive_embed_done", "ready": true}
```

**Option C — Block-wait with timeout (simplest for Claude UX):**  
`toolIngest` internally polls the corpora document for up to `timeout_ms` (default 10s).
Returns only when `step_hash` reaches `naive_embed_done` or timeout. Claude gets a single
synchronous "ready" confirmation. No new tool needed, no polling logic in Claude's reasoning.

```go
// Inside toolIngest, after InsertOne:
deadline := time.Now().Add(10 * time.Second)
for time.Now().Before(deadline) {
    doc, _ := corporaQuery.GetByID(ctx, s.db, result.ID)
    if doc.StepHash >= "step_2_corpora_naive_embed_done" { break }
    time.Sleep(500 * time.Millisecond)
}
```

**Recommendation: Option C for now, Option B as fallback if block-wait blocks too long.**
Option C keeps Claude's tool use model simple. The 10s cap is safe — if embedding takes
longer, Claude gets `"ingested (embedding in progress — search may not find this yet)"`.

**Acceptance test:**
- Call `SpecsAISystem_ingest` with new content
- Tool returns only after content is at `naive_embed_done` (or explicit timeout message)
- Immediately call `SpecsAISystem_search` for the ingested content → result found
- Ingest during a live `claude -p` execution → search in same execution → found

**Dependency:** None — can implement independently of B-P3. But becomes critical once
B-P3 wires MCP into the subprocess and Claude starts using ingest mid-task.

**Changelog:** —

---

### B-P12 — AST-first code routing: missing function body tool + re-index trigger + routing layer
**Status:** Not started  
**Hardness:** 45/100 (function body: 20, re-index MCP tool: 10, routing system prompt: 15)  
**Blocks:** 90% token savings on code-modification tasks; preventing heuristic file reads

**What exists (already good):**  
`AST_symbol_origin(symbol)` → file + line for any named symbol — zero file read needed  
`AST_import_graph(pkg)` → full dep tree without reading any source  
`AST_impact_analysis(pkg)` → "what breaks if I change this?" without reading importers  
`AST_enumerate_signatures(path)` → all exported signatures for a prefix  
`AST_dependents_tree(pkg)` → downstream dependency tree  
`AddService()` + `Load()` exist in cache.go for incremental + full reload

**What's missing (three gaps):**

**Gap 1 — No `AST_function_body` tool (hardness: 20/100):**  
`AST_symbol_origin("runClaude")` returns `{file: handler.go, line: 248}`. But to read
the function's source, Claude must still call `SpecsAISystem_read("handler.go", limit=120)`.
That's 120 lines of a 400-line file — most of which is irrelevant. Worse: Claude often
reads with generous margins because it doesn't know where the function ends.

The AST cache already has the parsed `*ast.FuncDecl` + `*token.FileSet` for every
function. The body range `[decl.Body.Lbrace, decl.Body.Rbrace]` is deterministic.
We can extract the exact source slice without `os.ReadFile` — the FileSet gives us
absolute positions, and the original file content can be re-read at exactly those lines.

```go
// New tool: AST_function_body
type functionBodyArgs struct {
    Symbol string `json:"symbol"` // e.g. "runClaude"
    Pkg    string `json:"pkg"`    // optional disambiguator, e.g. "query_dispatch"
}
// Returns:
// {
//   "symbol": "runClaude",
//   "kind": "method",
//   "file": "features_dequeue_poller/query_dispatch/handler.go",
//   "line_start": 248,
//   "line_end": 374,
//   "doc_comment": "runClaude runs the claude CLI...",
//   "signature": "func (h *Handler) runClaude(ctx, prompt, claudeResumeID string, isResume bool) ...",
//   "body": "{\n\tconst planningGatePrompt = ...\n\t...\n}"
// }
```

Token savings: a 130-line function body ≈ 400 tokens. A full-file read of handler.go
(400 lines) ≈ 1200 tokens. **3× savings per symbol lookup, ~90% savings vs reading
3-4 files heuristically to "find" the function first.**

**Gap 2 — No `AST_reindex` MCP tool (hardness: 10/100):**  
After Claude writes/edits a file (via `SpecsAISystem_write`), the AST cache is stale.
The next `AST_symbol_origin` call on the modified file returns the pre-edit location.
`AddService()` and `Load()` exist in cache.go but no MCP tool exposes them.

```go
// New tool: AST_reindex
type reindexArgs struct {
    Path string `json:"path"` // workspace-relative file or package path; empty = full reload
}
// Returns: {"indexed_files": N, "packages": M, "duration_ms": K}
```

Call pattern: `SpecsAISystem_write(path, content)` → `AST_reindex(path)` → cache current.
Full reload (`path=""`) takes <500ms for the entire workspace (measured at startup).
Single-file re-index takes <50ms.

**Gap 3 — No routing layer / classifier for code instructions (hardness: 15/100):**  
Without explicit instruction, Claude defaults to `SpecsAISystem_read` (or even the
built-in `Read`) to understand code before modifying it. The system prompt injected
by `runClaude` (`planningGatePrompt`) only addresses *write approval*, not *read strategy*.

A routing system prompt section needs to be injected alongside the MCP config (B-P3)
that instructs the decision tree:

```
## Code task protocol (follow before any file read or edit)

1. LOCATE: AST_symbol_origin(symbol) — find file + line. Never grep for it.
2. READ: AST_function_body(symbol) — get exact source. Never read the whole file.
3. IMPACT: AST_impact_analysis(pkg) — know what changes break before touching anything.
4. EDIT: SpecsAISystem_write(path, content) — then immediately AST_reindex(path).
5. VERIFY: AST_symbol_origin(symbol) again — confirm new location is correct.

Use SpecsAISystem_read only when:
  - You need file-level context (imports, struct fields, constants) not in signatures
  - AST_function_body returns "symbol not found" (unexported or generated code)
  - You are reading non-Go files (SQL, YAML, markdown)

Never read a whole file to find a symbol. Never grep for function names.
```

**Mode/workspace scoping (15/100, partially overlaps B-P9):**  
The AST cache is global — all services indexed together. `AST_enumerate_signatures("")`
returns symbols from all services. In studio mode, this includes core service symbols
that are irrelevant and consume token budget. Add `service_filter` parameter to
`AST_enumerate_signatures` and `AST_function_body` so mode-scoped queries only return
symbols from services listed in `cfg.WorkspaceManifest.ActiveModeConfig().Services`.

**Acceptance test:**
- Instruction: "update the runClaude timeout from 5 minutes to 10 minutes"
- Observe tool calls: `AST_symbol_origin("runClaude")` → `AST_function_body("runClaude")`
  → `SpecsAISystem_write(handler.go, ...)` → `AST_reindex(handler.go)`
- No full-file reads at any point
- Token count for the code read: ≤500 (function body only, not full file)
- Compare against baseline (current): 1200+ tokens for same task via file read
- `AST_function_body` result includes correct `line_start/line_end` matching source

**Dependency:** B-P3 must land first (MCP wired into subprocess) before the routing
protocol matters. But `AST_function_body` and `AST_reindex` tools can be implemented
on the AST server independently — they're useful for the interactive CLI path today.

**Changelog:** —

---

### B-P13 — Compaction: buildResumePrompt structured summary + Claude CLI compaction gap
**Status:** Done (Part A — 2026-04-11). Part B (CompactOnResume token tracking) deferred to Sprint 2.  
**Hardness:** 25/100  
**Blocks:** Long conversation correctness; context bloat after 5+ turns; B-P5 empirical test

**Part A — `buildResumePrompt` grows unboundedly (hardness: 15/100):**

Current `buildResumePrompt` (`handler.go:424-450`) concatenates verbatim:
```
enriched_prompt (full RAG context, ~2000-4000 tokens)
+ PriorAssistantText (Claude's analysis before AskUserQuestion, ~500-2000 tokens)
+ question text (~50 tokens)
+ user's answer (~50 tokens)
```

For a 3-turn conversation with stale sessions on turns 2 and 3, the prompt passed
to `claude -p` on turn 3 is:
```
turn1.enriched_prompt + turn1.PriorAssistantText + Q1 + A1
+ turn2.enriched_prompt + turn2.PriorAssistantText + Q2 + A2
```
That's 10,000-15,000 tokens just for context reconstruction — before Claude writes a
single word of response. Against `MaxContextTokens=4000` (the default), this overflows.

**Fix: structured snapshot compression in `buildResumePrompt`:**

Instead of verbatim concatenation, extract structured fields from prior turns:
```go
func (h *Handler) buildResumePrompt(parent *querycmd.QueryDocument, answer string) string {
    // Compressed snapshot: task summary + key decisions, not full verbatim context
    snapshot := extractSnapshot(parent)
    // snapshot: "Task: [first 200 chars of original prompt]\n
    //            Files considered: [from PriorAssistantText, extracted]\n
    //            Plan proposed: [first 300 chars of PriorAssistantText]\n
    //            Question asked: [question]\n
    //            Answer: [answer]"

    var sb strings.Builder
    sb.WriteString("## Context snapshot (prior turn)\n\n")
    sb.WriteString(snapshot)
    sb.WriteString("\n\n---\n\n## Current instruction\n\n")
    sb.WriteString(answer)
    return sb.String()
}
```

The snapshot replaces the full `enriched_prompt` (2000-4000 tokens → ~200 tokens) and
`PriorAssistantText` (500-2000 tokens → ~300 tokens). Total: ~500 tokens vs 5000+.
The trade-off: re-enrichment is needed for the new turn (RAG runs fresh on `answer`).
This is already the right behavior — prior RAG context may be stale for the new question.

**Snapshot extraction heuristic (deterministic, no LLM):**
- Original task: `parent.Prompt[:min(200, len)]`
- Plan summary: first sentence of `PriorAssistantText` (up to first `\n\n` or 300 chars)
- Files mentioned: regex `[a-z_/]+\.go` matches in `PriorAssistantText` (deduplicated)
- Question + answer: verbatim (already short)

**Part B — Claude CLI `/compact` not available in `-p` mode (hardness: 10/100):**

The `/compact` slash command is interactive-mode only. In `claude -p` mode there is:
- `--continue`: resumes the most recent session in CWD (no compaction)
- `--resume <id>`: resume by ID (no compaction)  
- `--fork-session`: new session ID from a resumed session (no compaction)
- `--no-session-persistence`: don't save session (removes resumeability)

There is **no CLI equivalent of `/compact`** for automated use.

**Implication for the automated pipeline:**  
Claude's native compaction (`/compact`) runs inside the interactive session and
summarizes all prior turns before they're evicted. In `-p` mode, context management
is entirely Claude's internal chunking — we cannot trigger a summary externally.

**What we can do:**  
Track estimated token usage per session across turns. Store `cumulative_prompt_tokens`
on `QueryDocument`. When a new turn arrives and `cumulative_prompt_tokens > 0.8 * MaxContextTokens`:

1. Set a `CompactOnResume` flag on the query document
2. In `runClaude()`, when `CompactOnResume=true`: use `--no-session-persistence` + inject
   the structured snapshot from Part A instead of `--resume`. This is a manual "compact"
   — we throw away the raw session and rebuild from structured snapshots only.
3. The query pipeline already has `buildResumePrompt` for the stale-session case — this
   is the same mechanism, now also triggered by token budget exhaustion.

**Token tracking (needed for this):**  
Parse the `cost_usd` and token counts from Claude's `result` NDJSON line
(`handler.go:336-374`). The stream-json format includes `usage.input_tokens +
usage.output_tokens` in the result block. Store `total_input_tokens` on
`QueryDocument` (similar to `TotalCostUSD`). Accumulate across turns via
`parent.TotalInputTokens + this_turn_input_tokens`.

**Acceptance test:**
- Submit a 3-turn conversation with stale sessions on turns 2+3
- `GET /queries?session_id=X` → `buildResumePrompt` content is ≤600 tokens (not 5000+)
- Submit a 10-turn conversation → session 11 detects `cumulative_prompt_tokens > threshold`
  → `CompactOnResume=true` → turn 11 uses snapshot injection, no `--resume` flag
- Claude on turn 11 receives correctly structured context and continues coherently

**Changelog:** —

---

### B-P5 — Claude context management sufficiency (validation)
**Status:** Not started — needs empirical test, not code change  
**Hardness:** 10/100 (test only, no implementation)  
**Blocks:** Nothing — but informs whether token-budget engineering is needed

**Problem / Question:**  
Is Claude Code's native context management (chunking + compaction) sufficient for
long browser-based conversations, as long as `claude_session_id` is not stale?

**What we know from code:**
- `--resume <claude_session_id>` gives Claude full conversation context (Claude-managed)
- Stale session → `UseFallbackContext = true` → `buildResumePrompt()` called (handler.go:56-83)
- `buildResumePrompt` injects: enriched_prompt + question + answer — enough for single-turn recovery
- Multi-turn recovery (3+ turns after stale) loses intermediate context not captured in enriched prompt

**What we don't know:**
- How long before Claude's sessions go stale (undocumented TTL)
- Whether compaction summary quality is sufficient for complex multi-file tasks
- Whether `buildResumePrompt` alone is enough for 10+ turn conversations

**Spec (test plan):**
1. Run a 5-turn conversation via the browser (not CLI) on a real codebase task
2. After turn 5, artificially expire the session (or wait for natural expiry)
3. Submit turn 6 — observe: does `buildResumePrompt` reconstruct enough context?
4. Run a 10-turn conversation — does compaction lose critical decisions made in turns 1-3?
5. Document findings in this file under Changelog

**Acceptance test:**
- 5-turn task completes correctly with no context loss
- After stale session, turn N+1 uses `buildResumePrompt` and Claude continues coherently
- If compaction is insufficient: escalate to summarization strategy (inject verdict summaries
  from prior turns into each new enriched prompt — already designed in
  `epic_resumeable_conversation_architecture.md §ISSUE-S1`)

**Changelog:** —

---

## Sprint Status

### Sprint 1 — Observability + correctness groundwork — **COMPLETE** ✓
| Item | Status | Hardness | Blocks |
|---|---|---|---|
| B-P2 machine.yml + startup validation | **Done ✓** | 20/100 | Clean startup |
| B-P6 runClaude subprocess → proxy_bus | **Done ✓** | 10/100 | Full ply audit |
| B-P1 proxy_logs conversation linkage | **Done ✓** | 15/100 | Traceability fields |
| B-P8 sessions.messages empty (D-002) | **Done ✓** | 15/100 | Conversation replay |
| B-P4a Fix source_file to relative path | **Done ✓** | 10/100 | Portability |
| B-P13 buildResumePrompt compaction (Part A) | **Done ✓** | 15/100 | Long conversation correctness |

**Sprint 1 result:** All automated `runClaude` calls visible in proxy_logs with workspace/conversation/machine/user linkage. Session messages populated after SetCompleted. source_file paths relative. buildResumePrompt compresses to ≤1600 chars (head+tail) when PriorAssistantText >4000 chars.

### Sprint 2 — A2A state machine correctness + CLI parity (total hardness: ~275/100)
| Item | Status | Hardness | Blocks |
|---|---|---|---|
| B-P14a RULE-4: replace hardcoded step strings in orchestrator_persisted.go | Done (2026-04-12) | 5/100 | Registry typo safety |
| B-P14b A2A parallel agents: cancel siblings on park | Done (2026-04-12) | 15/100 | Token waste on parked rounds |
| B-P14c A2A retry with backoff (mirror query pipeline) | Done (2026-04-12) | 20/100 | Transient failure recovery |
| B-P14d buildA2AResumePrompt compaction (mirror B-P13) | Done (2026-04-12) | 10/100 | A2A context bloat |
| B-P7 Execution ply gate (D-001) | Not started | 40/100 | True 1-turn CLI parity + stale recovery |
| B-P9 Multi-instance mode+machine affinity | Not started | 30/100 | Correct fallback in multi-instance |
| B-P3 MCP tools in claude -p subprocess | Not started | 35/100 | Iterative RAG |
| B-P10 Long-running MCP tools — timeout + poller | Not started | 40/100 | Safe bash/build tool calls |
| B-P11 SpecsAISystem_ingest readiness wait | Not started | 20/100 | Ingest→search feedback loop |
| B-P12 AST-first routing: function body + reindex | Not started | 45/100 | 90% token savings on code tasks |
| B-P4b Ingest studio corpora | Not started | 25/100 | Studio search |
| B-P5 Context management test | Not started | 10/100 | Informs token budget (empirical) |

**Sprint 2 acceptance:** A2A sessions survive transient failures with backoff. Sibling agents cancelled on park. Hardcoded step strings replaced with constants. CLI parity: browser path = 1 turn for file edits. Claude subprocess uses MCP mid-task. AST-first for all code reads. Correct multi-instance fallback.

### Sprint 3 — Autonomous ticket execution (total hardness: ~185/100)
| Item | Status | Hardness | Blocks |
|---|---|---|---|
| B-P15 A2A task graph executor (DAG of agent tasks) | Not started | 80/100 | Autonomous 3–5 story-point ticket |
| Orphan janitor goroutine (ISSUE-2 from agent_state_machine.md) | Not started | 50/100 | Crash recovery for all pipelines |
| UC-6 Structured telemetry for state transitions | Not started | 35/100 | Observability + circuit breaker |
| SM MCP tool (SM_query_state, SM_pipeline_health) | Not started | 20/100 | Agent-accessible pipeline introspection |

**Sprint 3 acceptance:** A2A can autonomously execute a 3–5 story-point JIRA ticket as a dependency-ordered task graph. Processor crashes are auto-recovered. Pipeline health visible to agents via MCP tools.

---

---

### B-P14 — A2A state machine correctness (4 sub-items from agent_state_machine.md audit)
**Status:** Not started  
**Source:** `agent_state_machine.md` RULE-4 violation + ISSUE-3 + ISSUE-4 findings (2026-04-11)

#### B-P14a — RULE-4: hardcoded step strings in orchestrator_persisted.go
**Hardness:** 5/100

`orchestrator_persisted.go` lines 132, 208, 292 set `doc.StepHash` using string literals
instead of `constants.StepA2ACompleted` and `constants.StepA2AWaitingForQuestion`.
The values match today, but a constant rename won't be caught by the compiler.

**Fix:** Replace 3 lines with their constants. 10-minute change.

#### B-P14b — Parallel agents not cancelled on park (ISSUE-4)
**Hardness:** 15/100

In `runAgentRoundPersisted` (runner.go), when Agent A detects `AskUserQuestion` and sets
`parkedQ`, the function still calls `wg.Wait()` — all other agents run to completion.
Their outputs are discarded; the round re-runs after the user answers.
Wasted tokens: `(N-1) × average_agent_cost` per park event.

**Fix:** Pass a `context.WithCancel` derived context into each agent goroutine.
When `parkedQ` is set, immediately call `cancelFn()`. Each agent's `RunClaudeAgent`
uses `exec.CommandContext(runCtx, ...)` so the subprocess is killed within milliseconds.

#### B-P14c — A2A retry path dead (ISSUE-3)
**Hardness:** 20/100

`registry.go:208` declares `StepA2AProcessing → StepA2APending` as a valid transition
(retry with backoff). But `orchestrator_persisted.go` never uses it — on any error,
`SetA2AFailed` goes to terminal immediately. Transient Claude CLI timeouts, network
blips, and rate-limit errors all kill A2A sessions permanently.

**Fix:** Mirror the query pipeline pattern:
- Add `StepA2APendingRetry` constant (or reuse `StepA2APending` with `not_retry_before`)
- On transient error: `SetA2APendingRetry(ctx, db, docID, backoff(attemptCount))`
- On permanent error (GATHER ABORT, auth failure, max retries): `SetA2AFailed`
- Classify error type via substring matching (same as `isStaleSessionError`)

#### B-P14d — buildA2AResumePrompt compaction (mirror of B-P13)
**Hardness:** 10/100

`runner.go:buildA2AResumePrompt` (line 212) includes `PriorAssistantText` verbatim —
no compaction, same 5000–15000 token problem that B-P13 fixed for the query pipeline.

**Fix:** Apply the same `compactPriorText` function from `handler.go` (or extract it
to a shared package). Threshold: 4000 chars → head+tail snapshot.

**Acceptance tests:**
- B-P14a: rename `StepA2ACompleted` constant → build fails if any literal remains
- B-P14b: A2A session with 3 agents; Agent 1 parks → Agents 2+3 subprocess killed within 1s
- B-P14c: force transient error (kill Anthropic connectivity briefly) → session retries with backoff; force 3 failures → session reaches `StepA2AFailed` (terminal)
- B-P14d: A2A session where PriorAssistantText >4000 chars → resume prompt ≤1600 chars

---

### B-P15 — A2A task graph executor
**Status:** Not started (Sprint 3)  
**Hardness:** 80/100  
**Blocks:** Autonomous execution of 3–5 story-point tickets with dependencies between agent tasks

**Problem:**  
Current A2A data structure is a flat array: `RoundOutputs []A2ARoundOutput`. The
orchestration flow is imperative, hardcoded: `GATHER → Round1 (all parallel) → SYNC →
Round2 (all parallel) → VERDICT → AUDIT`. This cannot model:

```
decompose_ticket
  ├── write_migration         (parallel, no deps)
  ├── write_frontend          (parallel, no deps)
  └── write_tests             (parallel, depends on migration schema)
       └── run_integration    (series, after all three complete)
```

**Spec:** Replace `RoundOutputs []A2ARoundOutput` with `TaskNodes []A2ATaskNode`:
```go
type A2ATaskNode struct {
    NodeID      string     // unique within session
    AgentID     string
    Prompt      string     // task-specific prompt
    DependsOn   []string   // NodeIDs that must be Completed first
    Status      string     // pending | processing | completed | failed
    Output      string
    CostUSD     float64
    StartedAt   *time.Time
    CompletedAt *time.Time
}
```

Graph executor loop:
1. Topological sort at session creation — fail-fast if cyclic (Kahn's algorithm)
2. Each tick: collect nodes where `Status=pending` AND all `DependsOn` are `completed`
3. Run ready nodes in parallel (same goroutine pattern as current rounds)
4. On completion: persist output, mark `completed`, re-evaluate ready set
5. Terminal: all nodes `completed` → VERDICT. Any node `failed` beyond retry → escalate

**Dependency:** Needs B-P14c (retry) and UC-6 (telemetry) to be resilient.

---

## Open Questions

- **Q1:** ~~Does `claude -p` support `--mcp-config` flag?~~ **RESOLVED: Yes** — `--mcp-config <configs...>` confirmed in `claude --help`. Also `--strict-mcp-config` available to suppress other MCP configs.
- **Q2:** What is Claude Code session TTL? (determines urgency of stale-session edge cases — affects B-P13 threshold tuning)
- **Q3:** Should `conversation_id` in proxy_logs match A2A `ConversationID`, or is this a
  separate concept for direct `claude -p` (non-A2A) calls? (affects B-P1 schema)
- **Q4:** Should the browser generate the `conversation_id` UUID, or should the server
  generate it on first turn and return it to the browser? (affects B-P1 flow)

---

## Changelog

### 2026-04-10
- File created
- **B-P2 implemented:**
  - Created `config/machine.go` — `MachineManifest` struct + `LoadMachineManifest()`
  - Wired into `config.Load()`: machine.yml fills gap between auto-detection and env vars
  - Priority order: `CLAUDE_CLI_PATH` env var > `machine.yml claude_cli_path` > `exec.LookPath`
  - Created `.specsai/machine.yml` (gitignored, machine-local, `claude_cli_path: /Users/tyrohunt/.local/bin/claude`)
  - Created `.specsai/machine.yml.example` (committed reference)
  - Added `.specsai/machine.yml` to `.gitignore`
  - Confirmed claude binary is at `/Users/tyrohunt/.local/bin/claude` (not nvm path as previously assumed)
- B-P1 through B-P5 identified from code analysis of:
  - `features/proxy_bus/handler.go` (ProxyLogEntry fields)
  - `features/a2a/session_document.go` (ConversationID already implemented)
  - `features_dequeue_poller/query_dispatch/handler.go` (runClaude, buildResumePrompt)
  - `shared/config_validator/validator.go` (startup hard errors)
- machine.yml scope confirmed minimal: only CLAUDE_CLI_PATH and WorkspaceRoot
- Feasibility verdict: YES — no architectural blockers, only missing wiring
- **B-P12 and B-P13 added** after reading `ast_intelligence/cache.go`, `ast_intelligence/tools.go`, `query/context.go`, `handler.go:buildResumePrompt`, and `claude --help`:
  - B-P12: AST cache has symbol location + import graph but no function body extraction — still requires file reads. Three gaps: `AST_function_body` tool (uses existing parsed AST, no file I/O), `AST_reindex` MCP tool (exposes `AddService`/`Load`), routing system prompt injected at B-P3 time. Mode-scoped filtering also needed.
  - B-P13: `buildResumePrompt` does verbatim concatenation — 5000+ tokens for 3-turn reconstructed context. `/compact` is interactive-only, no `-p` equivalent. Fix: structured snapshot compression (deterministic, ~500 tokens), plus `CompactOnResume` flag for token-budget-triggered compaction using snapshot injection instead of `--resume`.
- **B-P10 and B-P11 added** after reading `mcp/server.go` and `mcp/proxy_tools.go`:
  - B-P10: `toolProxyBash` has no timeout on `cmd.Run()`, no MongoDB record of the call,
    no crash recovery. Long-running commands (go test, go build) block the SSE goroutine
    indefinitely. Spec: Tier 1 (timeout + persistent logging), Tier 2 (mcp_job poller +
    `SpecsAISystem_await` tool) for commands classified as slow by first-word heuristic.
  - B-P11: `toolIngest` returns at `raw_pending` — corpus not yet searchable. Claude calling
    `SpecsAISystem_search` immediately after ingest gets zero results and draws wrong conclusions.
    Spec: Option C (block-wait inside tool, 10s cap) recommended for simplicity.
- **B-P9 added** after review of poller.go + command.go: multi-instance stale-session
  fallback has no mode/machine affinity — any instance in the workspace can claim a
  fallback item regardless of mode or originating machine. Three failure modes documented:
  wrong mode context, wrong WorkspaceRoot, mode not recorded on document. Spec covers:
  `active_mode` + `machine_id` on `QueryDocument`, `sticky_processor_id` on `SetPendingFallback`,
  mode-scoped poller filter, `sticky_expires_at` escape hatch for down-instance recovery.
- **B-P6, B-P7, B-P8 added** after review of DEFERRED.md D-001/D-002 and proxy_bus code:
  - B-P6: runClaude subprocess bypasses proxy_bus — subprocess needs ANTHROPIC_BASE_URL in env
  - B-P7: execution ply forced gate = structural CLI parity gap (D-001); also covers stale-processing recovery
  - B-P8: sessions.messages always empty = no conversation replay in browser (D-002)
- **B-P2 partial gap fixed:** `config_validator` now checks `os.Stat(ClaudeCLIPath)` — stale machine.yml paths fail at startup, not silently at runtime

### 2026-04-12
- **Sprint 2 partial (B-P14a, B-P14b, B-P14d) complete:**
  - B-P14a: replaced 3 hardcoded step string literals in `orchestrator_persisted.go` with `constants.StepA2ACompleted` / `constants.StepA2AWaitingForQuestion` — RULE-4 fully resolved
  - B-P14b: `runAgentRoundPersisted` now creates a per-round `context.WithCancel`; when any agent parks, `cancelRound()` is called immediately, killing sibling subprocesses — ISSUE-4 fixed
  - B-P14d: `compactA2APriorText` added to `runner.go`; `buildA2AResumePrompt` applies it (threshold 4000 chars → head+tail 800-char snapshots) — A2A context bloat fixed, mirrors B-P13
  - B-P14c: `SetA2APendingRetry` added to `session_document.go` (Processing→Pending + `not_retry_before` backoff 30s/2m/10m, `attempt_count++`, max 3 retries); `isTransientA2AError` classifier added; `OrchestratePersisted` now routes `runPersistedPhases` errors to retry or terminal fail — ISSUE-3 fully resolved

### 2026-04-11
- **Sprint 1 complete:** B-P6, B-P1, B-P8, B-P4a, B-P13 (Part A) all implemented and committed
  - B-P6: `runClaude` subprocess now sets `ANTHROPIC_BASE_URL` when `PROXY_ENABLED=true`
  - B-P1: `ProxyLogEntry` gains `workspace_id`, `conversation_id`, `machine_id`, `user_id`; `Config.MachineID` promoted from `MachineManifest`
  - B-P8: `AppendMessage` called after `SetCompleted` (user prompt + assistant result); `Handler.SessionCmd` wired in poller
  - B-P4a: `AutoIngestCorpora` + `AutoIngestDomainKnowledge` now accept `workspaceRoot`; `source_file` stored as relative path via `strings.TrimPrefix`
  - B-P13 Part A: `buildResumePrompt` applies `compactPriorText` when `PriorAssistantText` >4000 chars — head+tail snapshot (~500 tokens vs 5000+)
- **B-P14 (a–d) identified** from deep audit of A2A code vs agent_state_machine.md:
  - B-P14a: RULE-4 violation — 3 hardcoded step string literals in `orchestrator_persisted.go` (lines 132, 208, 292)
  - B-P14b: parallel agent cancel bug — `wg.Wait()` runs all agents even after one parks; fix: context cancellation on park
  - B-P14c: A2A retry path dead — registry declares `Processing → Pending` retry transition but no code path uses it; transient failures are terminal
  - B-P14d: `buildA2AResumePrompt` has no compaction — mirror of B-P13 needed for A2A path
- **B-P15 added:** A2A task graph executor — prerequisite for autonomous 3–5 story-point ticket execution; flat round model cannot represent dependency-ordered parallel+series tasks
- **agent_state_machine.md supercharged** as A2A implementation decision anchor: full step inventory, ISSUE-3/4/5 documented, §5 implementation patterns for Architect/Senior Dev, open work table aligned with sprint plan
