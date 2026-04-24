# Platform Design — Workspace Layer

> **What this file is:** workspace-level design document for the multi-product platform
> that axon will become the core of. Captures architectural decisions, rationale, and
> open items from the planning conversation (2026-04-24).
>
> **Scope:** everything ABOVE the individual product level — shell, auth, RAG core,
> product registry, lazy loading, workspace conventions.
>
> **Not here:** axon-specific learning theory, track conventions, session rules → `DESIGN.md`
>
> **When you cd into a product folder**, this file is still loaded by Claude Code alongside
> that product's own `CLAUDE.md`. Use `query_axon_docs("platform auth")` to retrieve
> sections without reading the whole file.

---

## 1. Workspace Structure

```
<workspace-root>/
  CLAUDE.md                    ← orchestrator: product registry, context inference rules
  PLATFORM_DESIGN.md           ← this file: platform architecture, BM25, auth, shell
  core/                        ← shared Go: llm, rag, cqrs, store, metrics, CoreDeps
    deps.go                    ← CoreDeps struct — single construction, injected into all products
    registry.go                ← Product interface + filesystem discovery
    frontend.go                ← lazy build + SPA serve per product
    CLAUDE.md                  ← "never add product logic here"
  auth/                        ← JWT issuer + middleware (Keycloak-modeled, swap-ready)
  shell/                       ← thin React shell: magic link landing, ProductPicker, Suspense
    frontend/src/
    frontend/dist/             ← always pre-built — only artifact that must never lazy-load
  main.go                      ← discovers + mounts products, starts Fiber
  products/
    axon/                      ← SEALED v2. Read-only by convention.
    axon-v3/                   ← active axon lineage post-framework enforcement
    ceo/                       ← CEO product — skeleton-copied, diverging ontology
    <new-product>/             ← drop a folder, implement plugin.go, done
```

**Convention:** `cd products/<name>` activates that product's `CLAUDE.md`. The root
`CLAUDE.md` and this file are always in scope. No other setup required.

---

## 2. Product Registry

| Directory | Product | Status | CLAUDE.md | DESIGN.md |
|---|---|---|---|---|
| `products/axon` | Axon v2 — adaptive learning system | **Sealed** — no new features | `products/axon/CLAUDE.md` | `products/axon/DESIGN.md` |
| `products/axon-v3` | Axon v3 (or renamed) — post-framework lineage | **Active** | `products/axon-v3/CLAUDE.md` | `products/axon-v3/DESIGN.md` |
| `products/ceo` | CEO Path — executive learning system | **Active** | `products/ceo/CLAUDE.md` | `products/ceo/DESIGN.md` |
| `core/` | Shared Go infrastructure | **Active** | `core/CLAUDE.md` | — |
| `auth/` | JWT + OAuth2 layer | **Active** | — | §4 of this file |
| `shell/` | Auth shell + ProductPicker | **Active** | — | §3 of this file |

### Sealed product protocol

`products/axon/` is sealed at the commit that enforces multi-product structure.
- No new features, no refactors.
- Track data (`track_*/`) stays here — fully operational, independent growth.
- `DESIGN.md`, `plan_v2.md`, `roadmap_v2.md` are frozen at seal time.
- If a learner is mid-track in axon v2 — no disruption. The v2 instance keeps running.

### "Smartly copied" product creation

Copy the **skeleton**, not the **corpus**:

```bash
cp -r products/axon products/ceo
# Then purge:
rm -rf products/ceo/track_*/
rm products/ceo/plan_v2.md products/ceo/roadmap_v2.md
# Rewrite:
# products/ceo/DESIGN.md  → CEO-specific ontology (goals/decisions/stakeholders)
# products/ceo/CLAUDE.md  → CEO-specific guardrails
```

What carries over: Go command/query/handler patterns, React component structure,
store interface usage, CQRS bus wiring.

What does NOT carry over: domain types specific to learning (Session → Question →
Evaluation), track data, axon theory docs.

---

## 3. Shell — Authenticated Entry Point + Lazy Product Federation

### Pattern

```
magic link
  ↓
GET /launch/:token     → validate token, extract role, set HttpOnly JWT cookie
  ↓
GET /shell             → serve shell/dist/index.html (tiny, always pre-built, ~10kb)
  ↓
Shell renders ProductPicker filtered by Claims.Roles
  ↓
User clicks a product
  ↓
React.lazy(() => import('/assets/products/<name>.js'))
  ↓   first hit: Go build trigger if dist/ absent (sync.Once, mutex-guarded)
  ↓   subsequent: Cache-Control: public, max-age=31536000, immutable
Product mounts inside Suspense boundary
```

### Why the shell is the only always-pre-built artifact

Every product can lazy-load because the shell is the trust boundary. If the shell
failed to load, nothing works. Everything else can afford a first-hit build delay.
In production, all products are pre-built at deploy time — lazy build is a dev
convenience and a zero-downtime "new product just dropped" escape hatch.

### React structure

```
shell/frontend/src/
  Shell.tsx           ← ProductPicker + Suspense wrapper + role filtering
  products.ts         ← ROLE_PRODUCTS map: role → available product IDs
  registry.ts         ← React.lazy imports per product ID
```

```tsx
// Each product is a separate Vite entry — its own JS chunk
const registry: Record<string, React.LazyExoticComponent<...>> = {
  axon:    React.lazy(() => import('/assets/products/axon.js')),
  'axon-v3': React.lazy(() => import('/assets/products/axon-v3.js')),
  ceo:     React.lazy(() => import('/assets/products/ceo.js')),
}
```

### Go static handler — build-on-demand + cache

```go
// core/frontend.go
GET /assets/products/:product
  → check products/<name>/frontend/dist/<name>.js
  → absent: once.Do(func { exec npm run build in products/<name>/frontend })
  → serve: Cache-Control: public, max-age=31536000, immutable
```

`sync.Once` per product ID — concurrent magic link hits never spawn duplicate builds.

### Role-specific hidden endpoint

The `/launch/:token` route is not linked anywhere in normal navigation. It is the
only entry point. No magic link → no shell → no product access. The route itself
is not discoverable by URL guessing (tokens are HMAC-signed, short-lived, single-use).

This is the code-splitting trigger: the token validates → role is known → shell
renders only the products that role can see → user clicks → lazy import fires.

---

## 4. Auth — JWT + OAuth2, Keycloak-Modeled

### Design constraint

The JWT validation middleware must be agnostic to who issued the token. Start with
the in-house lightweight issuer; swap to real Keycloak or Auth0 by changing one env
var (`JWKS_URL`) without touching any product handler.

### Claims — Keycloak-compatible field names

```go
// auth/tokens.go
type Claims struct {
    Sub               string   `json:"sub"`                 // user ID
    PreferredUsername  string   `json:"preferred_username"`  // display name
    Roles             []string `json:"roles"`                // ["ceo", "axon", "admin"]
    jwt.RegisteredClaims
}
```

Using Keycloak's claim names means product code that reads `claims.Roles` works
unchanged whether the token came from the in-house issuer or a real Keycloak realm.

### Magic link flow

```
POST /auth/magic-link { email }
  → HMAC-signed token, 15min TTL, single-use
  → stored in SQLite (survives restarts) or in-memory (simpler, acceptable for low-traffic)
  → email: https://<domain>/launch/<token>

GET /launch/:token
  → validate (exists, not expired, not consumed)
  → mark consumed (idempotent: second hit → 410 Gone)
  → lookup user roles from user store
  → issue RS256 JWT (access: 1h) + refresh token (7d)
  → set both as HttpOnly, Secure, SameSite=Strict cookies
  → redirect to /shell

POST /auth/refresh
  → validate refresh token → issue new access JWT
  → rotate refresh token (single-use refresh)
```

### Swap path to Keycloak/Auth0

```
JWKS_URL=https://keycloak.internal/realms/prod/protocol/openid-connect/certs
```

Middleware reads public keys from JWKS endpoint. When set, in-house issuer is bypassed
entirely. Zero product code changes. This is the "backend agnostic" guarantee.

### DigitalOcean deployment

```
Droplet ($12/mo, 2GB RAM)
  Caddy → TLS termination, proxy to :3456
  systemd unit → single Go binary (all products + shell + auth)
  deploy.sh:
    git pull
    for p in products/*/frontend; do (cd $p && npm run build); done
    go build -o platform . && systemctl restart platform
```

---

## 5. BM25 RAG Upgrade — Replacing naive.go

<!-- sources: internal/rag/naive.go, internal/rag/bm25/ (planned) -->

### Why now / why not dense yet

Current `naive.go` scores by pure word overlap: `hits / len(queryWords)`. No IDF,
no TF saturation, no stemming, no spelling tolerance. For axon's corpus this is
adequate; for the platform corpus (all products' `.md` files) it degrades as
vocabulary grows and query phrasing diverges from doc phrasing.

Dense (embedding) models are deferred because:
- Corpus is tens-to-hundreds of `.md` files — BM25 saturates quality at this scale
- Technical domain benefits from exact term matching (BM25 precision > dense recall here)
- No GPU dependency, no inference overhead, no external model, stays local-first
- Dense as a **re-ranker** (not retriever) is the right future step, not a replacement

### Components

```
core/rag/
  retriever.go         ← Retriever interface (TopK signature unchanged)
  naive.go             ← current impl — keep for fallback and testing
  bm25/
    indexer.go         ← InvertedIndex build, SymSpell map, bigram extraction, PMI
    scorer.go          ← BM25 Okapi (k1=1.5, b=0.75, tunable via env)
    watcher.go         ← fsnotify + 2s debounce + reindex_needed flag
    synonyms.json      ← hand-edited domain synonyms (override layer on top of PMI)
    index.json         ← persisted index (VCS-ignored — regenerated from corpus)
```

### Index structure

```json
{
  "generated_at": "...",
  "corpus_hash": "sha256:...",
  "inverted_index": {
    "concept":      [{ "doc": "DESIGN.md#core-concepts", "tf": 12, "field": "body" }],
    "concept_map":  [{ "doc": "DESIGN.md#core-concepts", "tf":  8, "field": "heading" }]
  },
  "correction_map": {
    "concpet":   ["concept"],
    "evalution": ["evaluation", "evolution"]
  },
  "synonym_map": {
    "quiz":     ["session", "assessment"],
    "bloom":    ["bloom_level", "cognitive_level"]
  },
  "corpus_pmi": {
    "synthesis": ["apply", "bloom", "advancement"],
    "track":     ["session", "concept_map", "fork"]
  }
}
```

**Disambiguation:** when `correction_map` entry has multiple candidates (e.g.
`["evaluation", "evolution"]`), return union of results and surface a
`did_you_mean` field in the MCP tool output. BM25 ranking sorts out relevance —
no forced choice required.

### Layers applied at index time

| Layer | What it does | Go library |
|---|---|---|
| Stemming | "learning" / "learned" / "learns" → "learn" | `kljensen/snowball` |
| Bigrams | "concept map" indexed as one token | built-in |
| SymSpell correction map | edit-distance-1/2 variants pre-built | built-in (~80 lines) |
| PMI synonyms | co-occurrence thesaurus from corpus itself | built-in |
| Field weighting | heading match scores 2.5× body match | BM25 scorer |
| Hand-edited synonyms | `synonyms.json` overrides PMI | JSON file |

### Corpus watcher + re-index

```
fsnotify watches products/*/**.md + core/prompts/**
  ↓ .md file saved
  → debounce 2s
  → recompute corpus_hash
  → if changed: reindex_needed = true

next query
  → if reindex_needed: rebuild index (RWMutex write lock, ~100ms)
  → serve from in-memory index (RWMutex read — readers don't block each other)
```

Idempotent: same corpus → same corpus_hash → rebuild skipped.

### Compaction integration — the key upgrade

Current `compact_file.go` sends the entire source file to the LLM and asks it
to decide what's concept-map-relevant. BM25 flips this responsibility:

```
source file chunks → BM25-score each against all concepts in concept map
  ↓
score ≥ threshold  → keep verbatim (already concept-relevant, no LLM needed)
score < threshold  → send only these to LLM for distillation
  ↓
result: smaller LLM context, more precise compaction, faster response
```

The LLM stops making relevance judgments (which it does poorly for technical content)
and does only distillation (which it does well).

**Shared invalidation signal:** `compact_file.go` already computes `sha256` of
source content for the compact frontmatter. The BM25 index uses `corpus_hash` for
the same purpose. One watcher, one hash computation, both invalidations fire together.

**Auto-compact candidate selection** (new pattern, deferred):
When `AXON_SOFT_LIMIT` is breached, BM25-score all context files against the concept
map. Files with low avg chunk scores are compaction candidates. High-score files
earn their token budget. Replaces the manual "go compact this file" workflow.

### Interface unchanged — drop-in swap

```go
// core/rag/retriever.go
type Retriever interface {
    TopK(query string, k int, tokenBudget int) ([]Chunk, error)
}
// naive.Retriever satisfies this. bm25.Retriever satisfies this.
// CoreDeps.RAG switches in one line. Zero handler changes.
```

### MCP server upgrade

`cmd/axon-mcp/main.go` uses the same `internal/rag` package. Upgrading to BM25
automatically upgrades `query_axon_docs`. The MCP tool then queries across all
`products/*/**.md` files with spelling tolerance, stemming, and synonym expansion —
making it the effective "second brain" for Claude Code across the entire workspace.

This is the self-referential loop: better BM25 → better MCP retrieval →
Claude Code retrieves deeper design signals (aspirations, open decisions, rationale)
more accurately → better assistance on complex multi-direction conversations.

### BM25 interaction boundary with `--resume` (claudecli)

**Compaction + `--resume`:** `compact_file.go` calls `h.client.Stream()` — a fresh
call with no session. BM25 pre-filters chunks injected into that prompt. The `--resume`
mechanism is not involved. These are independent.

**Thread tutoring + `--resume`:** `thread_turn.go` calls `StreamResume()`. The
claudecli wrapper (line 79–82 of `client.go`) sends **only the latest user message**
when resuming — the CLI carries all prior context via `--resume <sessionID>`. This means:

- BM25 chunk injection happens on the **seed turn only** (fresh call, full system prompt + RAG chunks)
- Subsequent resume turns send the new user message only — no re-injection of RAG chunks
- BM25 improves the seed quality, which is where domain context matters most

**Implication for distillation:** `distill_threads.go` also uses `Stream()` (fresh).
BM25 pre-filters any context chunks fed into distillation prompts. No conflict.

**The only risk:** if the seed prompt's RAG chunks were poor (naive.go), all subsequent
`--resume` turns in that thread inherit that weakness — the bad context is baked into the
session. BM25 fixes this at the root: better seed → better whole thread, not just better
first turn.

---

## 6. Monorepo Decision Framework

### When to stay in the monorepo

The monorepo is correct as long as products share the same **domain ontology**.
Axon's ontology: `track → session → question → response → evaluation → synthesis`.
Any product that naturally maps onto this shape belongs here.

Signs you're still safe:
- A new feature in product A doesn't require changing `core/` interfaces
- Products A and B share the same Go domain types without awkward embedding
- Frontend routing structures are parallel, not competing

### When to extract to a separate repo

The signal is **ontological drift**: the product's data model stops mapping cleanly
onto the shared model. Indicators:
- You find yourself adding product-specific fields to `core/domain/types.go`
- A new product feature requires changing `store.Collection[T]` for a shape no other product uses
- The frontend routing structure is structurally different (goal-centric vs session-centric)

**How to extract cleanly (F5.1 makes this cheap):**
`CoreDeps` extraction means `core/` is already a clean boundary. At extraction time:
1. Publish `core/` as a Go module (`github.com/tyrohunt/axon-core`)
2. Update the product's `go.mod` to depend on it
3. Remove the product directory from the workspace
4. The product is now a standalone repo with no monorepo artifacts

### The safe preparation step (build now)

`F5.1 — CoreDeps extraction`: extract `server/server.go` wiring into `core/deps.go`.
This is a pure refactor with zero behavioral change. It cuts the clean interface that
makes future extraction cheap. Build this regardless of whether the full router gets built.

Everything else in Epic F5 (router, registry, frontend serving, signal bridge) is
additive from this foundation — build when the need is real, not before.

---

## 7. Latency Budget

| Operation | Current | With BM25 | Delta |
|---|---|---|---|
| `TopK` query (thread, conversation) | O(n × \|q\|) string scan | O(\|q\|) index lookup | **faster** |
| `query_axon_docs` MCP | Same string scan | Index lookup | **faster** |
| BM25 index build | N/A | ~100ms async on corpus change | invisible to user |
| Compaction LLM call | Full file → LLM | Low-score chunks only → LLM | **fewer tokens, faster** |
| First product load (shell click) | N/A | `npm run build` (dev only) | pre-built in prod: 0 |
| Magic link validation | N/A | HMAC verify + DB lookup | <5ms |
| JWT middleware | N/A | ECDSA verify (cached public key) | <1ms per request |
| Pipeline enqueue | N/A (synchronous write) | Append to JSONL + git commit | ~200ms — client sees 202 immediately |
| Pipeline execute (LLM op) | Synchronous, no retry | Worker async, retriable | 10–60s LLM call unchanged; failure no longer = data loss |

The 2-second tolerance applies to LLM synthesis calls. BM25 reduces the token count
fed to those calls, which reduces their latency. Net: BM25 helps the exact operations
where latency is most sensitive.

---

## 9. Durable Pipeline — Offline-Safe Write Queue

<!-- sources: internal/store/filesystem/track_store.go, internal/commands/ (planned: internal/pipeline/) -->

### The problem

Axon's current write path is synchronous in the request handler: the API call writes the
file, or it doesn't. If a write fails mid-synthesis (network drop, process crash, storage
hiccup), the learner's work is lost. The frontend holds answers in `localStorage` but has
no durable handoff guarantee once it submits. There is no retry, no resume, no audit trail.

On DigitalOcean, this is a real failure mode: LLM streaming calls are long (10–60s),
and anything can interrupt them mid-way. The evaluation and synthesis writes that follow
are the most valuable data in the system — losing them is the worst possible outcome.

### The solution: a temporal.io-shaped lightweight worker

Not temporal.io itself — that's production infrastructure with a separate server, DB,
and SDK. Instead: the same **conceptual shape** (durable workflow, activity functions,
retries, compensation) implemented as a Go goroutine + a flat JSONL file + a mutex.
No new external dependency. No separate process. Same binary.

**Key properties:**
- **Idempotence** — same write submitted twice produces one result, not two
- **Retries with exponential backoff** — 10s → 30s → 2min → 10min → dead letter
- **Resume on restart** — IN_PROGRESS entries older than 5min are re-attempted on startup
- **Atomic boundary** — in file-based mode, the git commit is the transaction; in MongoDB mode, a session write is the transaction
- **Compensation (future)** — if a downstream step fails after upstream succeeded, record a compensation entry rather than leaving state corrupt

### Two-phase persistence

```
Phase 1 — Enqueue (fast, offline-safe, always succeeds)
  Client submits → POST /api/queue/enqueue
    → write PipelineEntry to _pipeline_queue.jsonl (append-only)
    → git add _pipeline_queue.jsonl && git commit (file-based mode)
    → 202 Accepted  ← client is done, data is durable

Phase 2 — Dequeue + Execute (atomic, retried, background)
  Worker polls _pipeline_queue.jsonl
    → picks oldest PENDING entry
    → marks IN_PROGRESS (status update written to queue file)
    → calls OperationHandler (writes domain files, OR MongoDB write)
    → on file-based mode: git add <affected_files> && git commit
    → marks COMPLETED, records commit hash
    → on failure: exponential backoff, increment attempts
    → on max attempts: mark DEAD_LETTER, emit metrics event
```

The queue file is itself git-committed on enqueue. This means: even if the process crashes
between Phase 1 and Phase 2, the queue entry is in git → dequeuer picks it up on restart.
The queue IS the write-ahead log.

### PipelineStore interface (backend-agnostic)

```go
// internal/pipeline/store.go
type PipelineStore interface {
    Enqueue(ctx context.Context, e PipelineEntry) error           // idempotent: same key → noop
    Peek(ctx context.Context) (*PipelineEntry, error)             // oldest PENDING
    MarkInProgress(ctx context.Context, id string) error
    MarkCompleted(ctx context.Context, id string, commitHash string) error
    MarkFailed(ctx context.Context, id string, err error, nextRetry time.Time) error
    Deadletter(ctx context.Context, id string) error
    ResumeStalled(ctx context.Context, stalledAfter time.Duration) ([]PipelineEntry, error)
    Get(ctx context.Context, idempotencyKey string) (*PipelineEntry, error)
}

// Implementations:
// internal/pipeline/store_jsonl.go   ← append-only JSONL, same pattern as metrics.jsonl
// internal/pipeline/store_mongo.go   ← MongoDB pipeline_queue collection
```

Chosen at startup via the same env-var pattern as the persistence layer:
`PIPELINE_STORE=jsonl` (default) or `PIPELINE_STORE=mongodb`.

### PipelineEntry schema

```go
type PipelineEntry struct {
    ID             string     `json:"id"`               // uuid
    IdempotencyKey string     `json:"idempotency_key"`  // "<op>:<track>:<session>:<payload_hash>"
    PipelineID     string     `json:"pipeline_id"`      // e.g. "evaluate:track_1:session_003"
    Operation      string     `json:"operation"`        // "submit_responses" | "evaluate" | "apply_synthesis" ...
    Payload        any        `json:"payload"`
    AffectsFiles   []string   `json:"affects_files"`    // for git add in file-based mode
    Status         string     `json:"status"`           // pending|in_progress|completed|failed|dead_letter
    Attempts       int        `json:"attempts"`
    MaxAttempts    int        `json:"max_attempts"`     // default 5
    LastAttemptAt  *time.Time `json:"last_attempt_at,omitempty"`
    NextRetryAt    *time.Time `json:"next_retry_at,omitempty"`
    CreatedAt      time.Time  `json:"created_at"`
    CompletedAt    *time.Time `json:"completed_at,omitempty"`
    CommitHash     string     `json:"commit_hash,omitempty"` // set on file-based COMPLETED
    Error          string     `json:"error,omitempty"`
}
```

### Worker design

```go
// internal/pipeline/worker.go
type Worker struct {
    store    PipelineStore
    gitDir   string
    gitMu    sync.Mutex           // ONE git operation at a time, ever
    handlers map[string]OpHandler // operation name → handler func
    backoff  []time.Duration      // [10s, 30s, 2min, 10min]
}

type OpHandler func(ctx context.Context, e PipelineEntry) (affectedFiles []string, err error)

// Run polls continuously. Crash-safe: stalled IN_PROGRESS entries are resumed on startup.
func (w *Worker) Run(ctx context.Context) {
    w.resumeStalled(ctx)  // crash recovery on startup
    ticker := time.NewTicker(1 * time.Second)
    for { select { case <-ctx.Done(): return; case <-ticker.C: w.tick(ctx) } }
}
```

### Git operations policy (file-based mode only)

The worker is the **only code path** that runs git commands. All git operations go through
`w.gitMu` — concurrent git calls are impossible by construction.

**Permitted operations:**
```
git add <specific files>        ← never git add -A or git add .
git commit -m "pipeline: ..."   ← message includes operation + queue entry ID
git push origin <SYNC_BRANCH>   ← background, best-effort, non-blocking
```

**Forbidden operations (never, under any circumstances):**
```
git reset --hard      ← destroys uncommitted work
git checkout -- .     ← destroys uncommitted work
git clean -f          ← destroys untracked files
git push --force      ← destroys remote history
git rebase            ← rewrites history
```

**`.git/index.lock` detection:** if present, another git process is running externally
(e.g. the user ran `git commit` manually). Worker waits up to 30s with 1s polling,
then logs a warning and retries the entry later. Never forcibly removes the lock file.

**Commit message format:**
```
pipeline: <operation> <track_id>/<session_num> [<queue_entry_id[:8]>]

Examples:
  pipeline: submit_responses track_1/session_003 [a1b2c3d4]
  pipeline: apply_synthesis track_2/session_001 [e5f6a7b8]
```

Git log becomes a human-readable audit trail of all mutations. `git log --oneline`
shows the full pipeline history.

### Remote git sync (DigitalOcean)

The process on DigitalOcean is tied to a specific remote and branch via env vars:

```
GIT_SYNC_REMOTE=origin               # default
GIT_SYNC_BRANCH=main                 # the branch that gets pushed
GIT_SYNC_INTERVAL=300                # push every 5 minutes (seconds)
GIT_SYNC_ENABLED=true                # false = local-only, no push
```

Push runs in a separate goroutine, independent of the worker. Push failure does NOT
block the worker — commits are durable locally, push is best-effort durability to remote.

```
git push origin main --no-verify
  → success: log sync event
  → failure: log warning, retry on next interval
  → NEVER: force push, reset, or modify branch history
```

### The offline → online flow (end-to-end)

```
1. User answers questions in browser
   → saved to localStorage (existing axon behavior, debounced 1.5s)

2. User clicks Submit
   → POST /api/tracks/:id/sessions/:num/responses
   → handler writes 02_responses.json (direct, fast — no queue needed for reads)
   → handler enqueues "evaluate" operation → 202 Accepted

3. Evaluate operation picked up by worker
   → calls EvaluateHandler (LLM call, potentially 10–60s)
   → on success: writes 03_evaluations.json
   → git add + commit (file-based) or MongoDB write
   → marks COMPLETED

4. If evaluate fails (LLM timeout, crash mid-stream):
   → entry remains IN_PROGRESS → stall detection after 5min → retry
   → exponential backoff: 10s, 30s, 2min, 10min
   → on max retries: dead_letter → metrics event → alert

5. User reconnects or refreshes
   → GET /api/queue/track_1/session_003/status → { pipeline_id, status, attempts }
   → UI shows "evaluation in progress" or "evaluation failed — retrying"
   → localStorage still holds answers as backup (cleared only on COMPLETED)
```

### What does NOT go through the queue

| Operation | Reason |
|---|---|
| GET requests (reads) | Never mutate state |
| LLM streaming SSE itself | Stream is stateless; only the WRITE at the end is queued |
| UI state (localStorage) | Client-side queue; frontend manages it |
| Direct file reads in handlers | Read-path, no mutation |
| Index/metric reads | Read-path |

The queue is for **writes that must not be lost** and **writes that may need retry**.
Simple fast writes (creating a session directory, writing metadata) can stay synchronous.
Long writes (evaluation, synthesis, apply_synthesis) should be queued.

### Cost-benefit of git-commit-as-atomic-boundary

| | Pro | Con |
|---|---|---|
| **Atomicity** | All files in a commit succeed or none do | Serializes concurrent writes (gitMu) |
| **Audit trail** | `git log` = full mutation history, free | Repo grows with every auto-commit; run `git gc` periodically |
| **Durability** | Commits survive process crashes | Push can fail independently of commit |
| **Consistency** | Aligns with existing VCS-tracking invariant | External git operations (user's manual commits) must not race |
| **Offline** | Works with no network — commits accumulate, push when back online | Branch can diverge if remote moves; pull before push on reconnect |

**Verdict: cost is acceptable.** The operations that go through the queue already take
10–60s (LLM calls). A git commit adds ~100–500ms — imperceptible. The alternative
(silent data loss on crash) is categorically worse. The invariant "track data is
VCS-tracked" already exists; this just makes it mechanical rather than manual.

### Startup decision: JSONL vs MongoDB

```go
// main.go
var pipelineStore pipeline.PipelineStore
switch os.Getenv("PIPELINE_STORE") {
case "mongodb":
    pipelineStore = pipeline.NewMongoStore(mongoClient, "axon", "pipeline_queue")
default: // "jsonl" or unset
    pipelineStore = pipeline.NewJSONLStore(filepath.Join(axonDir, "_pipeline_queue.jsonl"))
}
worker := pipeline.NewWorker(pipelineStore, axonDir, handlers)
go worker.Run(ctx)
```

The JSONL store is mandatory — it is the offline guarantee. MongoDB is additive.
Both implement the same `PipelineStore` interface. In MongoDB mode, the JSONL store
can optionally run in parallel as a shadow log (append-only backup of every enqueue).

### New endpoints

| Method | Path | Purpose |
|---|---|---|
| POST | `/api/queue/enqueue` | Enqueue a pipeline entry (idempotent) |
| GET | `/api/queue/:pipeline_id/status` | Poll status of a pipeline |
| GET | `/api/queue/dead-letter` | List dead-letter entries |
| POST | `/api/queue/:id/retry` | Manually re-enqueue a dead-letter entry |
| GET | `/api/queue/pending` | Admin: list all PENDING entries |

---

## 8. Open Decisions

- [ ] **Products/axon path vs legacy root:** axon currently serves routes at `/api/*`.
  Post-restructure, should axon move to `/axon/api/*` or keep the legacy root for
  backward compatibility with existing `curl` workflows?
- [ ] **auth/ store:** SQLite (survives restarts, file-based, local-first) vs in-memory
  (simpler, loses state on restart — acceptable if magic links are short-lived enough)?
- [ ] **BM25 index persistence:** VCS-ignored (regenerated from corpus on startup) vs
  committed (reproducible without rebuild cost)? Prefer VCS-ignored — index is a
  pure derivative of the corpus.
- [ ] **Synonyms.json scope:** one per workspace (shared across all products) vs one
  per product (product-specific vocabulary)? Likely both: workspace-level general
  synonyms + product-level overrides.
- [ ] **CEO product name:** `products/ceo/` is a placeholder. Rename when the product
  identity crystallizes — path change is one `mv` + one `plugin.go` ID() update.
- [ ] **Shell authentication for single-product ship:** when a product is extracted
  and shipped standalone, does the magic link / JWT layer simplify to a static
  password, or is magic link still the right UX for the target audience?
- [ ] **BM25 k1/b parameters:** defaults (k1=1.5, b=0.75) are Elasticsearch standard.
  Tune after real query logs exist — not before.
- [ ] **Pipeline: which operations are queued vs synchronous?** Candidates for queuing:
  evaluate, synthesize, apply_synthesis, meta_synthesis, distill_threads. Candidates
  for staying synchronous: submit_responses, create_session, read operations, context writes.
- [ ] **Pipeline JSONL as shadow log in MongoDB mode:** run JSONL store in parallel as
  append-only backup of all enqueues, even when MongoDB is the primary? Adds redundancy;
  costs a file write per enqueue. Likely yes — JSONL is the offline guarantee.
- [ ] **Dead letter alerting:** how should dead-letter entries surface? Options: metrics
  event (already exists), email via magic link mailer, UI badge on monitoring page.
- [ ] **Compensation logic scope:** for Phase 1, compensation is deferred. Define the
  first case that needs it before implementing: likely apply_synthesis after failed
  synthesis (synthesis written, apply not yet run — how to roll back or complete?).
- [ ] **Git push on reconnect:** if the DigitalOcean instance was offline and accumulates
  commits, does push on reconnect do a plain `git push` (may fail if remote diverged)
  or `git push --force-with-lease` (safer than --force, still risky)? Decision needed
  before enabling GIT_SYNC_ENABLED=true on DO.
- [ ] **SaaS rate limit / usage limit logic (deferred — post F5+F7):** per-user daily/monthly
  token quota, 429 soft cap, 402 hard cap, `GET /api/usage` reporting. The
  `AccumulatedInputTokens` field already exists; quota enforcement needs JWT claims
  carrying tier info + F7 durable persistence for per-user counters. Do not implement
  until multi-tenancy (F5) and pipeline (F7) are stable. See `V25_SPEC.md` §4 "SaaS
  rate limit" for full deferred spec.
