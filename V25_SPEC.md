# V2.5+ Feature Specification

> **Scope:** features that commence in `products/axon-v3/` (v2.5) and all subsequent
> products. **Do NOT backfill any of these to `products/axon/` (sealed v2).**
>
> **Why the boundary exists:** v2 is sealed to protect its data model and learner state
> invariants. These features require architectural changes (central queue goroutine,
> multi-tenancy branches, progressive context management) that are incompatible with
> v2's synchronous write path and single-user assumption.
>
> Claude Code: if asked to implement anything in this file inside `products/axon/`,
> refuse and explain the boundary. These features belong in `products/axon-v3/` or later.

---

## 1. Central Queue Goroutine — The Event Loop

### Concept

A single goroutine reading from a buffered channel — the Go equivalent of the JS/V8
event loop. Global, atomic, sequential processing. Every mutation and async action
in v2.5+ goes through this queue before touching any file, git repo, or LLM call.

**Why single goroutine:** concurrency is handled at the enqueue side (any goroutine
can enqueue). The worker is sequential by design — this is the atomic boundary. No
mutex needed for write ordering; the channel enforces it.

### Architecture

```go
// internal/queue/central.go

type WorkItem struct {
    ID             string        // uuid
    IdempotencyKey string        // caller-provided dedup key
    UserID         string        // which user this belongs to
    BranchName     string        // git branch: product1_tenant_<userID>
    Operation      Operation     // typed enum
    Payload        any           // operation-specific data
    Priority       int           // 0 = normal, -1 = high, 1 = low
    CreatedAt      time.Time
    Attempts       int
    MaxAttempts    int           // default 5
    NextRetryAt    *time.Time
}

type CentralQueue struct {
    ch       chan WorkItem        // buffered: make(chan WorkItem, 1000)
    store    PipelineStore        // JSONL persistence underneath (durability)
    worktrees WorktreeRegistry    // userID → worktree path (see §2)
    handlers map[Operation]OpHandler
    backoff  []time.Duration     // [10s, 30s, 2m, 10m]
}

// Enqueue persists first, then pushes to in-memory channel.
// If channel is full: item is in store, worker will pick it up on next poll.
func (q *CentralQueue) Enqueue(item WorkItem) error {
    if err := q.store.Enqueue(item); err != nil {   // durable first
        return err
    }
    select {
    case q.ch <- item:           // fast path: channel has room
    default:                     // channel full: store is the backup
    }
    return nil
}

// Run is the event loop. ONE goroutine. Never spawn more.
func (q *CentralQueue) Run(ctx context.Context) {
    q.drainStore(ctx)            // on startup: re-enqueue any PENDING from store
    for {
        select {
        case <-ctx.Done():
            return
        case item := <-q.ch:
            q.process(ctx, item) // blocking: next item waits
        case <-time.After(5 * time.Second):
            q.pollStore(ctx)     // periodic: pick up items that missed the channel
        }
    }
}
```

**The "blocking" property:** because there is ONE goroutine processing, item N+1 cannot
start until item N completes. This is the atomic guarantee. It does not block the Go
scheduler — HTTP handlers continue serving reads. Only writes are serialized.

**Idempotence:** `IdempotencyKey` checked on enqueue against the store. Duplicate
keys with status `completed` → return existing result, skip. Duplicate keys with status
`pending/in_progress` → return existing entry ID, caller polls for completion.

### Startup: drain the store

On startup, the worker reads all `PENDING` and stale `IN_PROGRESS` entries from the
JSONL store and re-enqueues them to the channel. This is crash recovery — no item
is lost across restarts.

---

## 2. Multi-Tenancy via Git Worktrees (NOT checkout switching)

### Branch naming convention

```
product1_tenant_<userID>        ← e.g. product1_tenant_u7f3a2b1
product2_tenant_<userID>
ceo_tenant_<userID>
```

The main branch (or a dedicated `queue` branch) holds:
- The queue JSONL file (`_pipeline_queue.jsonl`)
- Shared config, prompts, MCP index
- Schema definitions

Each tenant branch holds:
- That user's `track_*/` data
- That user's concept maps, sessions, conversations
- That user's `rag_index.json` (per-user BM25 index)

### Git worktrees — NOT checkout switching

`git checkout` changes the working tree for the whole repo. Under a multi-tenant
queue, checkout switching between user branches is dangerous (race conditions,
dirty state, index.lock conflicts).

`git worktree add` creates a separate working directory per branch. Each user gets
their own directory. The queue worker operates in the correct worktree without
touching any other user's files.

```bash
# On user creation:
git worktree add /data/worktrees/product1_tenant_u7f3a2b1 product1_tenant_u7f3a2b1

# Directory structure on disk:
/data/
  repo/                          ← main repo (queue branch)
    _pipeline_queue.jsonl
  worktrees/
    product1_tenant_u7f3a2b1/   ← user 1's working tree (their branch)
      track_1/
      track_2/
    product1_tenant_u7f3a2b2/   ← user 2's working tree (their branch)
      track_1/
```

### Queue worker: correct worktree per item

```go
func (q *CentralQueue) process(ctx context.Context, item WorkItem) {
    wt, err := q.worktrees.Get(item.UserID)  // get or create worktree
    // Execute operation in wt.Path — all file reads/writes go here
    // git add + git commit happen inside wt.Path (worktree has its own index)
    // No git checkout. No touching other users' worktrees.
}
```

### 10-user limit (DigitalOcean)

| Resource | Per user | 10 users |
|---|---|---|
| Worktree disk (tracks + data) | ~50MB | ~500MB |
| BM25 index per user | ~5MB | ~50MB |
| Queue entries (JSONL) | shared | ~1MB total |
| RAM (Go, no per-user state) | shared | ~150MB binary |

$12 Droplet (2GB RAM, 50GB disk): comfortable at 10 users. Hard limit enforced via
`AXON_MAX_TENANTS=10` env var — enqueue returns 503 if new user exceeds limit.

---

## 3. Top 10 Queueable Actions

Priority order — implement in this sequence:

| # | Operation | Why queue it | Idempotency key |
|---|---|---|---|
| 1 | `evaluate_responses` | LLM call (10–60s), data loss = evaluation gone | `evaluate:<trackID>:<sessionNum>:<responseHash>` |
| 2 | `generate_synthesis` | LLM call, bloom state mutation — most valuable learner data | `synthesize:<trackID>:<sessionNum>` |
| 3 | `apply_synthesis` | Concept map mutation — must be atomic, never partial | `apply_synthesis:<trackID>:<sessionNum>` |
| 4 | `git_commit` | ALL file writes sealed by this — atomic boundary | `commit:<trackID>:<operation>:<fileHash>` |
| 5 | `generate_questions` | LLM call, retried if interrupted — session unusable without it | `generate_q:<trackID>:<sessionNum>` |
| 6 | `distill_threads` | Long LLM call, idempotent — fork/merge depend on output | `distill:<trackID>` |
| 7 | `compact_file` | LLM call, source-hash gated — safe to retry | `compact:<trackID>:<filename>:<sourceHash>` |
| 8 | `conversation_turn_write` | Thread persistence — message must not be lost on crash | `conv_write:<convID>:<turnIdx>` |
| 9 | `summarize` (new) | Progressive conversation summarization — see §4 | `summarize:<convID>:<messageWindow>` |
| 10 | `git_push` | Remote sync — best-effort, non-blocking, retriable | `push:<branch>:<commitHash>` |

**Not queued:** GET requests, LLM stream itself (stream is stateless — only the
write at the end is queued), UI state, `submit_responses` (fast sync write is fine),
`create_session` (directory allocation — idempotent by nature).

---

## 4. Compaction Triggers — Complete List

### Conceptual model: compaction as context re-construction

Compaction is not merely size reduction. It is **context re-construction** — the act
of making a degraded or oversized context coherent enough to be the seed of the next
LLM call.

This distinction matters because the system relies on `--resume` (Claude's own session
continuity) as the primary context management mechanism. When `--resume` is healthy,
Claude carries its own rolling window — the system sends only the latest message and
gets back a coherent response. **No compaction needed.**

The moment `--resume` fails, the system is on its own:

```
--resume healthy:
  system sends: [latest user message only]
  Claude carries: full prior context internally
  compaction role: none (Claude manages it)

--resume failed (session expired / token limit reached):
  system must send: full rebuilt context
  Claude carries: nothing (fresh session)
  compaction role: CRITICAL — the rebuilt context must fit in one seed turn
```

**Why the token limit causes a hard failure:** Claude's context window (~200k tokens)
is bounded. When `accumulated_input_tokens` crosses the model's limit, Claude auto-
expires the session. The next `StreamResume` call returns a NEW sessionID. The system
detects `newSessionID ≠ storedSessionID` — this is the signal. From this point, the
system must rebuild context from scratch, and if the un-compacted corpus is large,
the rebuilt context immediately exceeds the limit again → infinite expiry loop.

**Compaction breaks the loop:** before the fresh call, compact context files (reduce
per-file token count) + summarize conversation history (replace message chain with
rolling summary). The rebuilt seed now fits within the model's window.

### Three distinct mechanisms

**Compactor** (reduces a document, LLM), **Summarizer**
(condenses conversation history, LLM), **Pruner** (hard window truncation, no LLM).

### Compactor triggers

| # | Trigger | Condition | Action | Queued? |
|---|---|---|---|---|
| C1 | **Manual** | User clicks "Compact" in UI | `compact_file` | No — synchronous SSE |
| C2 | **Soft limit hit** | `LoadInheritedContext > AXON_SOFT_LIMIT` (250k) | BM25-score files → compact lowest-scoring | Yes — queue `compact_file` |
| C3 | **Thread: session expiry** | `thread_turn` resumeID gone, rebuilding full context | Compact context files before fresh seed | Yes — inline before queue `generate_questions` |
| C4 | **Conversation: session expiry** | `conversation_turn` gets new sessionID ≠ old | Compact context before rebuilding history | Yes — queue `compact_file` then retry |
| C11 | **--resume token limit** | `accumulated_input_tokens` crossed model window (~200k) → Claude auto-expired session → new sessionID detected | Compact ALL context files + trigger S3 summarizer before fresh seed call | Yes — highest priority, blocks next generation |
| C12 | **--resume hard failure** | `StreamResume` returns error or sessionID mismatch on consecutive turns | Same as C11: compact + summarize + rebuild seed | Yes — retry with compacted context |
| C5 | **Before fork** | User triggers fork | Compact parent context to reduce child's inherited size | Yes — pre-step in `DuplicateTrackHandler` |
| C6 | **Before merge** | User triggers merge | Compact each source track's context | Yes — pre-step in `MergeTracksHandler` |
| C7 | **Evaluation context** | Question + response + context > `AXON_CONTEXT_LIMIT` | Compact context files before evaluation LLM call | Yes |
| C8 | **Question generation** | `LoadInheritedContext` near limit | Compact before generating | Yes |
| C9 | **MCP token budget** | `query_axon_docs` result > `AXON_MCP_TOKEN_LIMIT` | Summarize top-K chunks at retrieval time (see §5) | No — inline in MCP handler |
| C10 | **Auto-compact schedule** | File not compacted in N days + above threshold | Background queue item at low priority | Yes |

**Environment variables:**

```
AXON_COMPACT_TRIGGER_TOKENS=50000   # per-file token count that triggers auto-compact
AXON_SOFT_LIMIT=250000              # total corpus tokens → compact candidates
AXON_HARD_LIMIT=300000              # blocks generation entirely
AXON_MCP_TOKEN_LIMIT=50000          # MCP query result limit → summarize above this
```

### Summarizer triggers (v2.5+ only — do NOT backfill)

Summarizer condenses conversation history: keeps last N messages verbatim, replaces
older messages with a rolling summary paragraph. LLM call, queued.

| # | Trigger | Condition | Action |
|---|---|---|---|
| S1 | **Message count** | `len(messages) > 20` | Summarize oldest 10 messages → 1 summary message |
| S2 | **Token count** | `accumulated_input_tokens > 50000` | Summarize messages until under 30k tokens |
| S3 | **Session expiry / token limit** | `--resume` fails OR Claude auto-expired session (token limit hit) → new sessionID detected | Summarize full history → then compact context files (C11/C12) → then rebuild fresh seed | Always paired with C11 or C12 |
| S4 | **Before fork** | CEO-Non-Tech fork of conversation | Summarize conversation as context for child |
| S5 | **MCP retrieval** | Retrieved chunks > `AXON_MCP_TOKEN_LIMIT` | Summarize each chunk to 30% size, return more coverage |

### Pruner triggers (v2.5+ only — do NOT backfill)

Pruner removes messages beyond a hard window. No LLM call — pure slice truncation.
Safety valve for when summarizer fails or is disabled.

| # | Trigger | Condition | Action |
|---|---|---|---|
| P1 | **Hard message limit** | `len(messages) > 50` | Drop oldest messages until `len = 30` |
| P2 | **Hard token limit** | `accumulated_input_tokens > 100000` | Drop oldest messages until under 60k |
| P3 | **Compaction failure** | Compact/summarize returns error after 3 retries | Fall back to pruner |

**Order of application:** S (summarize) → if S fails: P (prune) → C (compact context files).
Summarizer preserves signal. Pruner just protects against runaway context growth.

### Session expiry decision flow

```
StreamResume() called
  ↓
newSessionID == storedSessionID?
  ├─ YES → --resume healthy, Claude managing context
  │         send only latest user message
  │         update AccumulatedInputTokens from response
  │         check if approaching limit (> 150k) → pre-emptive S1/S2 queue
  │
  └─ NO  → session expired (manual expiry OR token limit auto-expiry)
            system is on its own — must re-construct context
            │
            ├─ STEP 1: trigger S3 (summarize conversation history)
            ├─ STEP 2: trigger C11 (compact all context files)
            ├─ STEP 3: wait for both queue items to complete
            ├─ STEP 4: rebuild seed prompt with compacted corpus + summary
            └─ STEP 5: Stream() (NOT StreamResume) → get fresh sessionID
                        store new sessionID, reset AccumulatedInputTokens
```

**Pre-emptive threshold:** when `AccumulatedInputTokens > 150k` (75% of model limit),
queue S1/S2 summarization proactively. Goal: never reach forced expiry — summarize
while the session is still alive. Cheaper than re-construction.

### SaaS rate limit (deferred — post v2.5)

Per-user token budget enforcement (usage limits, tier quotas, billing caps) is a
SaaS concern that requires the JWT/multi-tenancy foundation to be in place first.

Deferred tracking:
- `AccumulatedInputTokens` per-user per-day (persisted in user branch)
- Soft cap: return 429 with `Retry-After` when daily budget exceeded
- Hard cap: return 402 Payment Required when monthly quota exceeded
- Usage reporting endpoint: `GET /api/usage` (per-user, admin aggregate)
- Billing integration: deferred — implement after F5 + F7 are stable

**Why deferred:** the token count infrastructure (`AccumulatedInputTokens`) already
exists in `conversation_turn.go`. The missing pieces are: per-user persistence (needs
F7 durable pipeline), and quota configuration (needs JWT claims to carry tier info).

---

## 5. MCP Tool: Compaction Proxy

Current behavior: `query_axon_docs` retrieves top-K chunks up to token budget, returns as-is.

V2.5+ behavior: the MCP handler wraps retrieval with an adaptive token budget:

```
query intent received
  ↓
BM25 retrieves top-30 candidates (over-retrieve)
  ↓
total tokens of candidates > AXON_MCP_TOKEN_LIMIT?
  ├─ NO  → return as-is (fast path, most common)
  └─ YES → summarize each chunk to 30% of its size via LLM
            return summarized chunks (more coverage, same token budget)
            + did_you_mean if spelling correction applied
            + source attribution per chunk
```

This is "adaptive summarization at retrieval time." The MCP caller (Claude Code) gets
more coverage of the corpus within the same token envelope, at the cost of one extra
LLM call when the limit is breached.

**Env var:** `AXON_MCP_COMPRESS_ON_OVERFLOW=true` (default false — opt-in in v2.5+).

---

## 6. CEO-Non-Tech Product Spec

`products/ceo/` — free-form conversation as the PRIMARY interface. Not quiz-session
based. The CEO does not answer structured questions — they narrate, reflect, plan,
and the system interrogates.

### How it differs from CEO-Technical (v2/axon)

| Dimension | CEO-Technical (axon v2) | CEO-Non-Tech (ceo product) |
|---|---|---|
| Primary interface | Quiz sessions (structured Q&A) | Free-form conversation |
| LLM call mode | `Stream` (fresh per generation) | `StreamResume` with progressive summarization |
| RAG callsite | Seed turn + session expiry | **Every turn** (like `conversation_turn.go`) |
| Context management | Token limit hard stop | Summarizer + pruner (progressive) |
| Bloom tracking | `bloom_current` per concept | Aspiration map (goals, decisions, milestones) |
| Session type | quiz / application | conversation / reflection / planning |
| Compaction | Manual + soft limit | Automatic (S1–S5 triggers) |
| Queue | Not applicable (v2) | Central queue goroutine (§1) |
| Multi-tenancy | Single user | Branch per user (§2) |

### RAG for CEO-Non-Tech — is the R solid?

**`conversation_turn.go` pattern is the base.** It already RAGs on every turn:
```go
chunks := rag.ChunkFiles(allFiles, convRagChunkSize)
selected := rag.TopK(chunks, cmd.UserMessage, convRagMaxChunks, convRagTokenBudget)
```
With BM25 (F8), this becomes spelling-tolerant, stemmed, bigram-aware.

**Gaps that v2.5 must close:**

| Gap | Current state | V2.5 fix |
|---|---|---|
| Session expiry → no compaction | Full history resent on expiry, no size reduction | Trigger S3 (summarize) before fresh call |
| No progressive summarization | `accumulated_input_tokens` tracked but not acted on | Trigger S1/S2 at thresholds |
| Evaluator has no RAG | `evaluate_responses` sends full context, no retrieval | Add RAG retrieval of relevant concept chunks into evaluator prompt |
| Synthesizer has no RAG | `generate_synthesis` reads session files but no concept-definition retrieval | Add RAG for concept-level reference material |
| MCP no compression | Returns raw chunks up to budget | C9 / MCP proxy (§5) |

**Solid callsites (keep as-is, upgrade to BM25):**
- `conversation_turn.go` RAG on every turn ✓
- `thread_turn.go` RAG on seed + session expiry ✓ (good for CEO-Technical, not needed for CEO-Non-Tech which has no thread structure)
- MCP `query_axon_docs` ✓ (upgraded by F8)

---

## 7. V2.5 Guardrails for Claude Code

These rules apply when working in `products/axon-v3/` or `products/ceo/`. They
complement (not replace) the root `CLAUDE.md` guardrails.

- **Central queue is mandatory.** Every mutation (file write, LLM result persist,
  git commit) goes through the queue goroutine. No synchronous direct writes for
  operations listed in §3.
- **Git worktrees, never checkout switching.** `git checkout` is forbidden in queue
  worker code. Use `q.worktrees.Get(userID)` to get the correct worktree path.
- **Summarizer before pruner.** Always attempt summarization first. Pruner is a
  safety valve, not the primary mechanism.
- **BM25 only, no naive RAG.** `internal/rag/naive.go` is not imported in v2.5
  products. `CoreDeps.RAG` is always the BM25 retriever.
- **Compaction is queued.** Auto-compact triggers (C2–C10) enqueue a `compact_file`
  work item. They do NOT block the request handler.
- **No backfill.** If a feature from this spec is requested for `products/axon/`,
  raise the boundary explicitly. Do not implement it there.
- **Multi-tenancy scope.** A queue item's `UserID` and `BranchName` must be
  validated before the worktree is accessed. Never write to a worktree without
  a valid, registered `BranchName`.
- **AXON_MAX_TENANTS enforcement.** On new user creation, check tenant count.
  Return 503 if at limit. No silent overflow.
