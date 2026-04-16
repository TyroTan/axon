# Axon — Project Roadmap

> Adaptive knowledge assessment system. Local-first, file-persisted, browser UI.
> Go Fiber · React + shadcn/ui · CQRS · backend-agnostic store · claude CLI auth.

Last updated: 2026-04-16

---

## Status Legend

| Symbol | Meaning |
|---|---|
| ✅ | Done & committed |
| 🔄 | In progress |
| 📋 | Next up (queued, not started) |
| 🔮 | Deferred (future phase) |
| ❌ | Cancelled / won't do |

---

## Completed

### Phase 0 — Foundation ✅
Content scaffolding and system design.
- ✅ Learning framework design (`how_to.md`, `concept_taxonomy.md`, `plan.md`)
- ✅ Track 1 concept map (32 concepts, 4 branches, bottleneck graph)
- ✅ All 5 prompts for track_1 (`00` through `04`)
- ✅ Context sources registry (`context/_sources.md`)
- ✅ Git init as standalone axon project

### Phase 1 — Core Backend ✅
- ✅ Domain types: `Track`, `ConceptMap`, `Session`, `Question`, `Response`, `Evaluation`, `Synthesis`, `SteerIntent`
- ✅ Store interface: `Collection[T]` — MongoDB-style `$set/$inc/$push/$unset`, dot-notation nested access
- ✅ Filesystem store — JSON files, RWMutex, track tree, concept map, sessions
- ✅ CQRS: `CommandBus` + `QueryBus` with type-safe generic dispatch
- ✅ `ListTracksQuery`, `GetTrackQuery`
- ✅ `GET /api/tracks`, `GET /api/tracks/:id`

### Phase 2 — Track UI ✅
- ✅ Vite + React + TypeScript + Tailwind v4 + shadcn/ui (replaced HTMX)
- ✅ Sidebar: track tree, collapsible parent/child, branch initials
- ✅ Breadcrumb: Home → track_N → Context / Session N
- ✅ Track page: concept map heatmap (bloom colour-coded), session list, step badges (P/Q/R/E/S)
- ✅ Home page, 404 page, SPA fallback (`/*` → index.html)

### Phase 3 — Track Management ✅
- ✅ `DuplicateTrackCommand` → creates child track (bloom_current preserved, `_sources.md` seeded)
- ✅ `UpdateContextCommand` + `GET/PUT /tracks/:id/context/:filename`
- ✅ `GetTrackContextQuery`
- ✅ Context editor page: file list, textarea, save, create new file inline
- ✅ Duplicate button → navigates to new track immediately

### Phase 4 — Question Generation ✅
- ✅ `llm.Client` interface: `Stream(ctx, model, system, messages, maxTokens) <-chan Chunk`
- ✅ `llm/anthropic`: raw HTTP+SSE, no SDK
- ✅ `llm/claudecli`: shells out to `claude --print --output-format stream-json` — **no API key needed**
- ✅ Default: claudecli. Override: `ANTHROPIC_API_KEY` env var switches to HTTP client
- ✅ `CreateSessionCommand` → allocates `session_NNN/` directory
- ✅ `GenerateQuestionsCommand` → adaptive prompt (concept map + context), streams, writes `01_questions.json`
- ✅ `GetSessionQuestionsQuery` → returns `[]` (not error) when file absent
- ✅ `POST /sessions/:num/questions/generate` (SSE), `GET /sessions/:num/questions`
- ✅ Session page: generate button, SSE stream display, question cards (MCQ + free-text)

### Phase 5 — Quiz Flow ✅
- ✅ `SubmitResponsesCommand` → writes `02_responses.json`
- ✅ `EvaluateResponsesCommand` → streams claude-sonnet-4-6, computes Brier + calibration flag + time signal, writes `03_evaluations.json`
- ✅ `GetSessionResponsesQuery`, `GetSessionEvaluationsQuery` → return `[]` when absent
- ✅ Session page: MCQ option buttons, free-text textarea, confidence 1–5 picker, explanation field
- ✅ Sticky submit bar (N/N answered), read-only after submission
- ✅ Evaluation cards: correctness bar, bloom-demonstrated badge, calibration/error taxonomy, feedback, correct answer reveal
- ✅ Eval card shows submitted MCQ selection (green/red highlight vs correct), your explanation text, explanation score % badge

### Phase 6 — Synthesis ✅
- ✅ `GenerateSynthesisCommand` → per-concept correctness aggregation, bloom advancement rules, spaced repetition schedule, writes `04_synthesis.json`
- ✅ `ApplySynthesisCommand` → patches `concept_map.json` bloom_current + spaced_repetition; idempotent
- ✅ `GetSynthesisQuery` → returns null when absent
- ✅ Synthesis panel: learner summary, concept update table (before→delta→after), next review date
- ✅ Apply button, re-synthesise button, "Applied" badge

### Context & Token Budget ✅
- ✅ `LoadInheritedContext` — walks track_N_M → track_N → root, child files win on collision
- ✅ `ContextBudget` struct: Files, TokensUsed, Truncated, TruncatedAt
- ✅ `llm.CountTokensNaive` — pure fn, `(len+3)/4`, deterministic
- ✅ `AXON_CONTEXT_LIMIT` env var (default 80k)
- ✅ Truncation notice streamed to UI when limit hit
- ✅ Corpus snapshots copied into `track_1/context/` (`browser_cli_parity`, `agent_state_machine`)

---

## Completed (continued)

### Context Splitting + HITL Compaction ✅

**Goal:** Handle corpora > 250k tokens losslessly. Never silently drop meaning.

Three-zone model:
```
0 ──────── 250k ─────────── 300k ──────────► ∞
│               │                │
│  LOAD AS-IS   │  SPLIT PLAN    │  HARD ABORT
│  (fast path)  │  (HITL)        │  (single file > 300k)
```

Tasks (in order):

- ✅ **T1 — Soft/hard limit config** — `SoftTokenLimit` (250k) + `HardTokenLimit` (300k) in Config; `AXON_SOFT_LIMIT` / `AXON_HARD_LIMIT` env vars; sentinel errors `ErrSplitRequired` / `ErrHardLimitExceeded`
- ✅ **T2 — Split plan generator** — `LoadFullContext` measures total corpus; greedy first-fit-decreasing bin-packing into shards; writes `_split_plan.md` with YAML frontmatter; halts with `ErrSplitRequired`
- ✅ **T3 — Split plan schema** — `SplitShard`/`SplitPlan` types; `ReadSplitPlan` hand-rolled YAML parser; `LoadContextForShard`; `GET /api/tracks/:id/split-plan`
- ✅ **T4 — Shard-aware session creation** — `SessionMetadata{ShardID}`; `00_metadata.json`; shard picker modal on TrackPage; `POST /sessions` accepts `{ shard_id }`; question generation loads shard files only
- ✅ **T5 — Meta-synthesis** — `MetaSynthesisCommand` aggregates evaluations across all shard sessions; readiness check; `meta_synthesis.json` at track level; `ApplyMetaSynthesisCommand`; TrackPage panel

- ✅ **T6 — File-level compaction (optional, for pure noise reduction)**
  `CompactFileCommand(trackID, filename, targetTokens, conceptMap)`
  - Triggered manually from UI (not automatically on load)
  - Writes `filename.compact.md` with hash frontmatter for cache invalidation
  - Used for genuinely verbose files (changelogs, verbose prose)
  - Never replaces splitting as the primary strategy

- 📋 **T7 — Corpora pool**
  `/.experiments/corpora/` shared pool — track-agnostic corpus files
  `_corpus_refs.json` per track — references pool entries
  `LoadInheritedContext` resolves refs from pool before walking ancestor chain
  UI: import file to pool, attach to track, materialise as snapshot

---

## Active / Queued (UX + Observability)

### U1 — localStorage auto-save 📋
Debounced (2s) save of in-progress answers to `localStorage` keyed by `trackId/sessionNum`.
- Hydrates on mount if no server-saved responses exist (i.e. `02_responses.json` absent)
- Clears on successful submit
- Covers: MCQ selection, free-text answer, explanation, confidence
- No backend changes — pure UI

### U2 — Prompt preview endpoint 📋
`GET /api/tracks/:id/sessions/:num/prompt-preview`
- Builds the exact system + user prompt that would be sent to the LLM (same code path as `GenerateQuestionsHandler`, without actually calling it)
- Returns: `{ system_prompt, user_prompt, token_estimate, context_files_included[], soft_limit, hard_limit }`
- UI: expandable "Prompt" panel on session page before/after generation
- Purpose: let you tune context leanness without blind trial-and-error
- Note on 2A/2B: all LLM calls are currently stateless (each `claude --print` is a fresh invocation). Incremental/reused conversation context (2B) would require message-history threading — deferred as U2B.

### U3 — Per-question follow-up threads 📋
After evaluation, collapsible discussion thread per question for eval demystification.
- Storage: `sessions/session_NNN/threads/{question_id}.json` — array of `{ role, content, ts }`
- First message auto-seeded from evaluation (feedback + subscores + misconception) so LLM has full context
- UI: "Ask a follow-up" input below each eval card; streams response inline
- Backend: `POST /tracks/:id/sessions/:num/threads/:question_id` (SSE stream), `GET` to load history
- New commands: `AppendThreadMessageCommand`, `GetThreadQuery`
- Scope: single-question context window (eval + Q&A history for that question only — manageable size)

---

## Remaining Core Phases

### Phase 7 — Steer & Redo 🔮
- 🔮 `SteerIntent` domain type: direction enum + concept focus + branch focus + free-text note
- 🔮 `SteerResultCommand` → injects steer note into question generation prompt, new `generation_id`
- 🔮 Session page: steer dialog (harder/easier/focus concept/fewer branch) + free-text
- 🔮 Multiple generation tabs: `Run 1 | Run 2 | Run 3`

### Phase 8 — Polish 🔮
- 🔮 Markdown rendering in question text, feedback, learner summary (react-markdown)
- 🔮 Session progress bar (Q4 of 12)
- 🔮 Keyboard shortcuts: `1–5` = confidence, `s` = submit, `→` = next question
- 🔮 Concept filter: click concept in sidebar → filter session questions to that concept
- 🔮 Track page: concept map updates live after synthesis applied (re-fetch)

---

## Deferred

- 🔮 **MongoDB store impl** — zero handler changes required (store interface is the seam)
- 🔮 **Multi-user** — path-based isolation trivial; auth layer deferred
- 🔮 **Export** — session history as PDF or structured JSON
- 🔮 **Other LLM providers** — OpenAI, Ollama, Bedrock impls of `llm.Client`
- 🔮 **U2B — Incremental conversation reuse** — pass prior messages as history to the LLM instead of a fresh call each time; enables stateful 10-at-a-time question generation without re-loading context. Requires message-history threading in `llm.Client`.
- 🔮 **True RAG (semantic retrieval)** — what Axon currently does is NOT RAG: it loads all context files into the prompt (flat injection). The 250k soft limit exists because there is no selective retrieval. True RAG = embed context chunks → at query time, retrieve only the top-K relevant chunks by similarity → inject those. This would push the effective context limit far beyond 250k without sharding. Current path: file loading → sharding → compaction. RAG path (deferred): chunking → embedding index → similarity search → inject top-K.
- 🔮 **Graph RAG** — explicit relationship edges between chunks; needed for multi-hop reasoning on large corpora
- 🔮 **RAPTOR-style hierarchical index** — chunk → summarize → cluster → summarize; for corpora > 1M tokens

---

## Architecture Principles (Stable)

| Principle | Decision |
|---|---|
| Auth | CLI auth via `claude` binary — no API key management |
| Persistence | Filesystem JSON — swap to MongoDB via store interface, zero handler changes |
| LLM | `llm.Client` interface — anthropic HTTP or claudecli, caller-agnostic |
| Context | Inherited parent→child, token-budgeted, child wins on filename collision |
| Splitting | Lossless (HITL sharding) preferred over lossy (compaction) |
| CQRS | All reads via QueryBus, all writes via CommandBus — composition root owns wiring |
| Frontend | React SPA on :5173 (dev) / served from web/dist (prod), /api proxy to :3456 |
