---
agent_id: state_machine
name: "State Machine Agent"
rank: 1
role: "Pipeline Orchestration & State Integrity Guardian"
description: "Manages all step_hash state transitions, retry policies, orphan recovery, claim safety, and pipeline observability. The 'Hand of the King' for workspace-level orchestration — every agent that needs to move a document through the pipeline delegates to this agent. Used during A2A as an anchor for Architect and Senior Developer implementation decisions — on queries, on mutations, and during live coding sessions."
trigger_signals: ["state", "step_hash", "transition", "retry", "orphan", "stuck", "pipeline", "claim", "abandon", "recover", "stale", "backoff", "failure", "processing", "idempotent", "concurrent", "race", "crash", "resume", "park", "graph", "dag", "cycle", "task node", "parallel step", "series step"]
corpora_filter:
  tags: ["core", "system", "architecture"]
role_scoped: false
parent_agent_id: null
is_active: true
---

## Purpose During A2A Sessions

This document is an **implementation decision anchor**, not just an FAQ reference.

When the Architect or Senior Developer agent is asked to design or mutate pipeline
code, they MUST consult this document first. The rules here are non-negotiable.
Violating them is a latent bug, even if tests pass.

**Read this before:**
- Adding a new pipeline (new collection, new step_hash family)
- Adding a new step to an existing pipeline
- Writing any `SetXxx` command function
- Writing any poller filter
- Designing parallelism or serialization of agent work
- Adding retry, backoff, or failure handling
- Deciding whether a task needs MongoDB persistence vs in-memory state

---

## 1. Current Capability Matrix

### What works today

| Capability | Status | Location |
|---|---|---|
| Explicit DAG of valid transitions | DONE | `shared/statemachine/machine.go` + `registry.go` |
| Startup validation (DAG uniqueness, terminal rules) | DONE | `Machine.Validate()` — fatal on violation |
| Atomic step_hash claims (poller → handler) | DONE | `FindOneAndUpdate` with step_hash in filter |
| Query introspection: `CanTransition`, `NextSteps`, `Meta` | DONE | `machine.go` — runtime queryable |
| Retry with exponential backoff | DONE (query pipeline only) | `RetryPolicy` in `shared/poller_options/options.go` |
| Workflow failure tracking + auto-abandon (corpora) | DONE | `SetFailedTracked` → `workflow_failure_count` → `StepCorporaAbandoned` |
| Failure history append (corpora) | DONE | `failure_history[]` with step, error, timestamp, processor_id |
| Backoff gate (query pipeline) | DONE | `not_retry_before` field, poller `$or` filter skips unexpired |
| Idempotent re-claim skip | DONE | `ErrNoDocuments` = silent skip, not error |
| Draft deferral + promotion | DONE | `StepQueryDraftAwaitingSession` → `promoteDrafts` on parent resolve |
| Stale session fallback + context rebuild | DONE | `StepQueryPendingFallback` → `buildResumePrompt` |
| Post-claim step_hash guard on all SetXxx (query + corpora) | DONE (v0.16.0) | All `SetXxx` include `step_hash` in filter |
| workspace_id in all poller filters + document creation | DONE (v0.16.0) | All 5 poller filters + both Create commands |
| A2A session document persistence + phase checkpointing | DONE | `features/a2a/session_document.go`, `orchestrator_persisted.go` |
| A2A parking / resume (AskUserQuestion) | DONE | `A2AParkedQuestion`, `SetA2AWaitingForQuestion`, `ResumeParkedSession` |
| A2A conversation chain (ConversationID, TurnNumber, ParentSessionID) | DONE | `session_document.go:27–29` |
| A2A abort gate (GATHER phase rejects vague/irrelevant tasks) | DONE | `isGatherAbort()` in `runner.go` |
| buildResumePrompt compaction (query pipeline) | DONE (Sprint 1, B-P13) | `handler.go:compactPriorText` — 4000-char threshold |

### What exists but is broken or incomplete

| Capability | Status | Gap | Severity |
|---|---|---|---|
| Orphan detection | CONFIG EXISTS, NOT WIRED | `TaskStalThresholdSec` parsed, `started_at` written, no recovery goroutine — stuck-Processing documents never recovered | HIGH |
| A2A retry backoff | REGISTRY DECLARES IT, CODE NEVER USES IT | `StepA2AProcessing → StepA2APending` is a valid transition in `registry.go:208` but `SetA2AFailed` transitions to terminal `StepA2AFailed` immediately — no `StepA2APendingRetry` equivalent, transient failures permanently kill sessions | HIGH |
| Parallel agent cancel on park | BUG | In `runAgentRoundPersisted`: if Agent A calls AskUserQuestion, `parkedQ` is set but `wg.Wait()` still runs all other agents to completion — tokens wasted on a round that will be re-run after user answers | MEDIUM |
| buildA2AResumePrompt compaction | MISSING | `runner.go:buildA2AResumePrompt` includes `PriorAssistantText` verbatim — no compaction applied, same 5000–15000 token problem that B-P13 fixed for the query pipeline | MEDIUM |
| Hardcoded step strings in orchestrator_persisted.go | RULE-4 VIOLATION | Lines 132, 208, 292 use string literals (`"step_3_a2a_session_completed"`, `"step_2a_a2a_agent_waiting_for_question"`) instead of `constants.StepA2ACompleted`, `constants.StepA2AWaitingForQuestion` — bypasses the registry, typo-unsafe | LOW |

### What does not exist yet

| Capability | Status | Blocks |
|---|---|---|
| Orphan adoption queue with backoff | NOT IMPLEMENTED | Autonomous recovery from processor crashes |
| Per-step retry config (step_hash → retry options map) | NOT IMPLEMENTED | Step-specific resilience tuning |
| Structured telemetry events for state transitions | NOT IMPLEMENTED | Pipeline observability, circuit breaker, self-healing |
| A2A task graph executor (DAG of agent tasks, not flat rounds) | NOT IMPLEMENTED | Autonomous 3–5 story-point ticket execution |
| Circuit breaker at poller level | NOT IMPLEMENTED | Retry storm prevention when dependency is down |
| Poller backpressure / concurrency cap | NOT IMPLEMENTED | Goroutine count unbounded under load |
| SM MCP tool (`SM_query_state`, `SM_pipeline_health`) | NOT IMPLEMENTED | Agent-accessible pipeline introspection without reading Go code |

---

## 2. Full Step-Hash Inventory

### Query pipeline

| Constant | String value | Terminal | Claimed by |
|---|---|---|---|
| `StepQueryPending` | `step_1_query_pending` | No | query_dispatch |
| `StepQueryPendingFallback` | `step_1_query_pending_fallback` | No | query_dispatch |
| `StepQueryDraftAwaitingSession` | `step_1_query_draft_awaiting_session` | No | — promoted by promoteDrafts |
| `StepQueryProcessing` | `step_2_query_processing` | No | query_dispatch |
| `StepQueryWaitingForInput` | `step_2_query_waiting_for_input` | No | — unparked by POST /queries/:id/answer |
| `StepQueryCompleted` | `step_3_query_completed` | Yes | — |
| `StepQueryFailed` | `step_3_query_failed` | Yes | — |

### Corpora pipeline

| Constant | String value | Terminal | Claimed by |
|---|---|---|---|
| `StepCorporaRaw` | `step_1_corpora_raw_pending` | No | naive_embed / qdrant_embed |
| `StepCorporaNaiveEmbedProcessing` | `step_2_corpora_naive_embed_processing` | No | naive_embed |
| `StepCorporaNaiveEmbedDone` | `step_2_corpora_naive_embed_done` | Yes | — |
| `StepCorporaAbandoned` | `step_3_corpora_abandoned` | Yes | — |

### A2A pipeline

| Constant | String value | Terminal | Claimed by |
|---|---|---|---|
| `StepA2APending` | `step_1_a2a_session_pending` | No | a2a_orchestrator |
| `StepA2AProcessing` | `step_2_a2a_session_processing` | No | a2a_orchestrator |
| `StepA2AWaitingForQuestion` | `step_2a_a2a_agent_waiting_for_question` | No | — unparked by POST /a2a/answer |
| `StepA2ACompleted` | `step_3_a2a_session_completed` | Yes | — |
| `StepA2AFailed` | `step_3_a2a_session_failed` | Yes | — |

---

## 3. Architecture Audit — Known Issues

### ISSUE-1: Post-claim transitions missing step_hash guard
**Status: FIXED in v0.16.0.** All `SetXxx` functions now include `step_hash` in their filter.
See RULE-1 below.

### ISSUE-2: Orphan detection declared but not wired (SEVERITY: HIGH)

`TaskStalThresholdSec` is parsed from env and stored in config. `started_at` is written
on every claim. But no goroutine scans for documents where `now - started_at > threshold`.
If a processor crashes after claiming, that document is stuck in Processing forever.

**Fix spec:** A janitor goroutine that runs every `TaskStalThresholdSec` and:
1. Scans `{step_hash: Processing, started_at: {$lt: staleBefore}}` across query + a2a collections
2. Atomically transitions to a new `StepXxxOrphaned` step (adds it to registry)
3. Writes `adoption_count`, `adoption_history`, `not_adopt_before` (backoff)
4. Another poller claim picks up `StepXxxOrphaned` like a normal pending step
5. After N adoption failures → terminal `StepXxxAbandoned`

### ISSUE-3: A2A retry path is dead (SEVERITY: HIGH)

The registry declares `StepA2AProcessing → StepA2APending` as a valid transition
(meaning: retry with backoff). But `orchestrator_persisted.go` never calls it.
On any error during orchestration, `SetA2AFailed` is called immediately, going to
terminal. A transient Claude CLI timeout, a network blip, or a 30-second Anthropic
rate limit kills the entire session permanently.

**Fix spec:** Mirror the query pipeline pattern:
- Add `StepA2APendingRetry` constant
- On transient error: call `SetA2APendingRetry` with `not_retry_before = now + backoff(attempt_count)`
- On permanent error (e.g., GATHER ABORT, auth failure): call `SetA2AFailed`
- Distinguish transient vs permanent by error class (same pattern as `isStaleSessionError`)

### ISSUE-4: Parallel agents not cancelled on park (SEVERITY: MEDIUM)

In `runAgentRoundPersisted` (runner.go): when Agent A returns a `parkedQ`, the goroutine
sets `parkedQ` under the mutex and returns. But `wg.Wait()` blocks until ALL agents finish.
The other agents run to completion, their outputs are discarded, and the round is re-run
after the user answers. Wasted tokens × (N-1) agents per park event.

**Fix spec:** Pass a `context.WithCancel` derived context into each agent goroutine.
When `parkedQ` is set, call `cancelFn()` immediately. Each agent's `RunClaudeAgent` uses
`exec.CommandContext(runCtx, ...)` so the subprocess is killed within milliseconds.

### ISSUE-5: A2A has no task graph — blocks autonomous ticket execution (SEVERITY: HIGH for goal)

Current A2A data structure: `RoundOutputs []A2ARoundOutput` — a flat array of all agent
outputs across all rounds. The orchestration flow is imperative, hardcoded:
`GATHER → Round1 (parallel) → SYNC → Round2 (parallel) → VERDICT → AUDIT`.

This cannot model a 3–5 story-point ticket like:
```
decompose_ticket
  ├── write_migration         (parallel, no deps)
  ├── write_frontend          (parallel, no deps)
  └── write_tests             (parallel, depends on write_migration schema)
       └── run_integration    (series, after all three complete)
```

**Fix spec (B-P15 — Sprint 3):** Replace `RoundOutputs []A2ARoundOutput` with
`TaskNodes []A2ATaskNode` on the session document:
```go
type A2ATaskNode struct {
    NodeID      string    // unique within session
    AgentID     string    // which agent executes this node
    Prompt      string    // task-specific prompt
    DependsOn   []string  // NodeIDs that must be Completed before this runs
    Status      string    // pending | processing | completed | failed
    Output      string    // populated on completion
    CostUSD     float64
    StartedAt   *time.Time
    CompletedAt *time.Time
}
```

**Graph executor loop:**
1. At session start: topological sort on `DependsOn` — fail-fast if cyclic (DAG validation)
2. Each tick: collect all nodes where `Status=pending` AND all `DependsOn` are `completed`
3. Run ready nodes in parallel (same goroutine pattern as current rounds)
4. On node completion: persist output, mark `completed`, re-evaluate ready set
5. Cycle guard: transitive closure check at creation (NodeA cannot depend on NodeB if NodeB
   directly or transitively depends on NodeA)
6. Terminal condition: all nodes `completed` → VERDICT synthesis. Any node `failed` beyond
   retry budget → escalate to Scrum Master for human decision.

---

## 4. Design Rules (Non-Negotiable)

These rules apply to ALL pipeline code — query, corpora, A2A, and any future pipeline.
Violating them = silent data corruption or stuck documents. There are no exceptions.

### RULE-1: Every post-claim transition MUST include step_hash in its MongoDB filter

```go
// CORRECT — transition is rejected if document moved out of expected state:
filter := bson.M{"_id": docID, "step_hash": constants.StepQueryProcessing}

// WRONG — silently overwrites terminal state if two processors race:
filter := bson.M{"_id": docID}
```

**Status: IMPLEMENTED (v0.16.0).** Applies to all `SetXxx` functions in
`features/queries/command/command.go`, `features/a2a/session_document.go`,
and `features/corpora/command/`.

### RULE-2: Documents in claimed steps are immutable by external actors

Any external mutation must use `ProcessingGuardFilter()` which rejects writes if the
document is currently in a claimed step. A processor may already be executing side
effects (file edits via Claude CLI, DB writes, API calls) — concurrent mutation creates
uncompensatable state.

**Status: IMPLEMENTED (v0.16.0).** `shared/statemachine/machine.go`.

### RULE-3: workspace_id MUST be in every poller filter AND every document creation

```go
filter := bson.M{"step_hash": constants.StepQueryPending, "workspace_id": cfg.WorkspaceID}
```

Without this, processors from workspace A claim documents from workspace B in shared-DB
deployments. The poller filter is the only enforcement point.

**Status: IMPLEMENTED (v0.16.0).** All 5 poller filters + both Create commands.

### RULE-4: step_hash values MUST use constants — never string literals in business logic

```go
// CORRECT:
doc.StepHash = constants.StepA2ACompleted

// WRONG (RULE-4 VIOLATION — currently exists in orchestrator_persisted.go:132,292):
doc.StepHash = "step_3_a2a_session_completed"
```

**Status: VIOLATION EXISTS.** `orchestrator_persisted.go` lines 132, 208, 292 use
string literals. The values happen to match the constants today, but a constant rename
won't be caught by the compiler. Fix: replace with `constants.StepA2ACompleted` and
`constants.StepA2AWaitingForQuestion`.

### RULE-5: Every valid transition MUST be pre-mapped in the Machine registry

```go
Transition(constants.StepA2AProcessing, constants.StepA2ACompleted).
Transition(constants.StepA2AProcessing, constants.StepA2AFailed).
// If you add a new transition, add it here first — then implement it.
```

Handlers SHOULD call `CanTransition(from, to)` before every `SetXxx`. The registry
`Validate()` at startup enforces DAG integrity (uniqueness, no orphan terminals, etc.).

### RULE-6: Retry must use not_retry_before — never immediate re-queue

```go
// CORRECT — backoff prevents retry storms:
notBefore := time.Now().Add(backoff(attemptCount))
col.UpdateOne(ctx, filter, bson.M{"$set": bson.M{
    "step_hash":        constants.StepQueryPending,
    "not_retry_before": notBefore,
    "attempt_count":    attemptCount + 1,
}})

// WRONG — immediate re-queue + sustained failure = retry storm:
col.UpdateOne(ctx, filter, bson.M{"$set": bson.M{"step_hash": constants.StepQueryPending}})
```

**Status:** IMPLEMENTED for query pipeline. **NOT IMPLEMENTED for A2A** (see ISSUE-3).

### RULE-7: New step_hash constants MUST follow the naming convention

```
step_{N}_{pipeline}_{descriptive_name}_{state}

step_1_a2a_session_pending       ← queue entry
step_2_a2a_session_processing    ← claimed/running
step_2a_a2a_agent_waiting_...    ← sub-state of processing (use N+letter suffix)
step_3_a2a_session_completed     ← terminal success
step_3_a2a_session_failed        ← terminal failure
```

The name must be self-documenting when read from a raw MongoDB document. Generic names
(`"processing"`, `"step_2"`, `"done"`) are rejected at code review.

---

## 5. Implementation Patterns — Reference for Architect / Senior Dev

### 5a. Adding a new pipeline (new collection + step family)

1. Add constants to `shared/constants/step_hashes.go` (follow RULE-7)
2. Register all steps and all valid transitions in `shared/statemachine/registry.go`
3. Call `machine.Validate()` in `main.go` startup (already done for existing machines)
4. Add `workspace_id` to the document struct and to all poller filters (RULE-3)
5. All `SetXxx` command functions: include both `_id` AND `step_hash` in filter (RULE-1)
6. Use `constants.StepXxx` everywhere — no string literals (RULE-4)
7. Wire exponential backoff from `shared/poller_options/options.go` (RULE-6)

### 5b. Adding a new step to an existing pipeline

1. Add the constant (RULE-7)
2. Register it and all transitions to/from it in the registry (RULE-5)
3. Add a `SetXxxNewStep` command function with step_hash guard (RULE-1)
4. Update the poller filter if this new step needs a separate poller goroutine
5. If this step can be stuck (processor can crash mid-step): add it to the orphan
   detection scan (ISSUE-2 — once orphan recovery is implemented)

### 5c. Designing parallelism within a pipeline step

**Safe:** Spin goroutines inside a single handler invocation (same document, same claim).
The document stays in Processing. Each goroutine's output is accumulated. On completion,
one `SetCompleted` call exits Processing. This is what `runAgentRoundPersisted` does.

**Unsafe:** Having two separate handler goroutines both claim the same document for
"parallel sub-processing." RULE-1 prevents the second write from landing, but the wasted
work is unrecoverable. Design instead: one handler claims, fans out internally, aggregates,
then transitions.

**For cross-document parallelism** (e.g., multiple tickets being processed concurrently):
Each document is a separate claim. The poller's `go handler.Handle(ctx, doc.ID)` pattern
already handles this — N documents = N concurrent goroutines, each with its own claim.

### 5d. When to use MongoDB persistence vs in-memory state

Use MongoDB persistence when:
- The operation takes >5 seconds (crash window is real)
- The result needs to be observable by another goroutine, another process, or the browser
- The operation involves LLM calls (always crash-window risk)
- Resume/replay must be possible (AskUserQuestion, restart recovery)

Use in-memory state when:
- Pure computation with no external calls
- Result is ephemeral to a single request/response cycle
- Failure is acceptable and idempotent (caller can retry)

### 5e. When a task graph is needed vs flat rounds

Use flat rounds (current model) when:
- All agents see the same context and produce independent analyses
- Order within a round doesn't matter
- The goal is consensus / synthesis, not incremental construction

Use a task graph (B-P15, not yet implemented) when:
- Task A must complete before Task B can start (data dependency)
- Some tasks can run in parallel AND others must be serialized
- The output of one agent is the input to another (pipeline, not just synthesis)
- You're building an artifact (code, spec, migration) incrementally across agents

Rule of thumb: if "implement a JIRA ticket" means writing code, a graph is needed.
If it means "analyze and recommend," flat rounds are sufficient.

---

## 6. Open Work — Prioritized

| Item | Hardness | Impact | Priority | Depends on | Sprint |
|---|---|---|---|---|---|
| RULE-4 fix: replace hardcoded strings in orchestrator_persisted.go | 5/100 | Safety | DO NOW | Nothing | Sprint 2 |
| ISSUE-4: cancel sibling agents on park | 15/100 | Token cost | HIGH | Nothing | Sprint 2 |
| ISSUE-3: A2A retry with backoff (mirror query pipeline) | 20/100 | Resilience | HIGH | Nothing | Sprint 2 |
| buildA2AResumePrompt compaction (mirror B-P13) | 10/100 | Token cost | HIGH | Nothing | Sprint 2 |
| ISSUE-2: Orphan janitor goroutine | 50/100 | Resilience | HIGH (prod) | UC-6 telemetry | Sprint 3 |
| UC-6: Structured telemetry for state transitions | 35/100 | Observability | HIGH | Nothing | Sprint 3 |
| UC-5: Per-step retry config | 30/100 | Flexibility | MEDIUM | Nothing | Sprint 3 |
| EDGE-3: Circuit breaker at poller level | 25/100 | Resilience | MEDIUM | UC-6 | Sprint 3 |
| B-P15: A2A task graph executor | 80/100 | Autonomous tickets | HIGHEST | Telemetry, retry | Sprint 3 |
| §7b: SM MCP tool (SM_query_state, SM_pipeline_health) | 20/100 | Agent access | MEDIUM | Nothing | Sprint 3 |
| §7d: Poller backpressure / goroutine cap | 15/100 | Stability | LOW | Nothing | Sprint 3 |

---

## 7. Edge Cases

### EDGE-1: Cascading orphan-adoption failure (poison pill document)

Processor A claims → crashes. Janitor adopts → Processor B claims → crashes.
Without a cap: infinite bounce between Processing and QueuedAdopter.
**Guard:** `adoption_count` capped at 3 → terminal `StepXxxAbandoned`.
**Detection:** `adoption_count >= 2` in telemetry → alert.

### EDGE-2: Split-brain on workspace mode switch

Two processors running: A reads `active_mode: core`, operator switches to `active_mode: repo1`,
B starts with new mode. A filters corpora by `["core"]`, B by `["repo1"]`. Documents are
invisible across the split. Already mitigated by `$or: [{tags: {$size: 0}}, ...]` fallback.
Full fix: `global: true` flag on corpora documents that should be visible in all modes.

### EDGE-3: Retry storm on sustained external failure

50 documents pending. Dependency (Ollama, Claude CLI, Anthropic API) down for 30 minutes.
All 50 claim → fail → retry → fail → terminal. When dependency recovers, all 50 are
permanently failed. **Fix:** Circuit breaker at poller level (EDGE-3 in Open Work above).

### EDGE-4: A2A task graph cycle (once B-P15 exists)

Agent A's task depends on Agent B's output. Agent B's task was accidentally declared as
depending on Agent A. **Guard:** Topological sort (Kahn's algorithm) at session creation
in `CreateA2ASession`. If a cycle is detected, the session is rejected with a structured
error listing the cyclic nodes — it never enters the pipeline. This is the A2A equivalent
of `Machine.Validate()` for the step graph.

---

## 8. API Surface

### Available now (Go, internal to pipeline code)

```go
// State machine registry — read-only introspection
sm := statemachine.A2AMachine()          // or QueryMachine(), CorporaMachine()
sm.Meta("step_2_a2a_session_processing") → StepMeta{Description, Terminal, ClaimedBy, ...}
sm.NextSteps("step_2_a2a_session_processing") → []string of valid next steps
sm.CanTransition(from, to string) → bool
sm.ClaimedSteps() → []string  // all steps where ClaimedBy is non-empty
sm.Validate() → []error       // DAG integrity check (run at startup)

// Session document commands — all enforce step_hash guard (RULE-1)
CreateA2ASession(ctx, db, opts)                            → *A2ASessionDocument
SetA2AProcessing(ctx, db, docID, processorID)
SetA2AWaitingForQuestion(ctx, db, docID, parkedQ, phase, round)
SaveA2APhaseOutput(ctx, db, docID, updates bson.M)
AppendA2ARoundOutputs(ctx, db, docID, outputs, costUSD, runs)
SetA2ACompleted(ctx, db, docID, verdict, auditReport)
SetA2AFailed(ctx, db, docID, errMsg)
FindPendingA2ASessions(ctx, db, workspaceID) → []A2ASessionDocument
```

### Needs implementation (MCP tools for agent-accessible introspection)

```
SM_query_state(step_hash)          → StepMeta: description, valid_next, claimed_by, terminal
SM_pipeline_health(collection)     → step_hash distribution, orphan count, avg_duration_ms
SM_list_stuck(older_than_seconds)  → documents in Processing past the stale threshold
```

---

## 9. Changelog

| Date | Change | Author |
|---|---|---|
| 2026-04-10 | Initial document created | Claude Sonnet 4.6 |
| 2026-04-11 | Full rewrite: verified against actual code (orchestrator_persisted.go, runner.go, registry.go, step_hashes.go); added ISSUE-3 (A2A retry dead), ISSUE-4 (parallel cancel bug), ISSUE-5 (task graph gap); updated capability matrix to reflect v0.16.0 fixes; added step inventory table; supercharged with implementation patterns for Architect/Senior Dev use; added RULE-4 violation finding; updated open work priority table | Claude Sonnet 4.6 |
