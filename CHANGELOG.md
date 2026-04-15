# Changelog

All notable changes to Axon are documented here.
Format: [semantic version] — date — description.

---

## [Unreleased]

---

## [0.12.0] — 2026-04-15 — T5: meta-synthesis across all shards

### Added
- `domain.MetaSynthesis` — track-level aggregate: `sessions_aggregated`, `shards_aggregated`, `concept_map_updates`, `learner_summary`, `applied`; written to `track_N/meta_synthesis.json`
- `MetaSynthesisCommand` + `MetaSynthesisHandler` — gates on all approved shards having evaluated sessions; aggregates evaluations across them; first-fit-decreasing per-concept stats; streams LLM synthesis
- `MetaSynthesisHandler.ReadinessCheck` — returns list of approved shards still missing evaluated sessions
- `ApplyMetaSynthesisCommand` + `ApplyMetaSynthesisHandler` — idempotent patch of `concept_map.json` from meta-synthesis; mirrors per-session `ApplySynthesis`
- `GetMetaSynthesisQuery` / `GetMetaSynthesisResult`
- `ListSessionsWithMeta` — enriches session stubs with `ShardID` from `00_metadata.json`
- `ReadTrackFile` / `WriteTrackFile` — track-level file I/O (not inside a session directory)
- Routes: `GET /api/tracks/:id/meta-synthesis`, `GET /api/tracks/:id/meta-synthesis/readiness`, `POST /api/tracks/:id/meta-synthesis/generate` (SSE), `POST /api/tracks/:id/meta-synthesis/apply`
- `metrics` event `meta_synthesis_generated` with shards + sessions in extra
- `TrackPage` meta-synthesis panel: readiness check, generate/re-synthesise button, learner summary, sessions/shards/updates summary, Apply button + Applied badge

---

## [0.11.0] — 2026-04-15 — T3+T4: split plan schema + shard-aware sessions

### Added
- `SplitShard` / `SplitPlan` exported domain types in `split_plan.go`
- `ReadSplitPlan` — parses `_split_plan.md` YAML frontmatter (hand-rolled, no external YAML dep); returns nil when absent
- `LoadContextForShard` — filters full inherited corpus to a single shard's file list
- `domain.SessionMetadata` — `ShardID string`; written to `00_metadata.json` at session creation
- `WriteSessionMetadata` / `ReadSessionMetadata` on `TrackStore`
- `CreateSessionCommand.ShardID` optional field; `CreateSessionResult` echoes it back
- `GET /api/tracks/:id/split-plan` endpoint
- `POST /api/tracks/:id/sessions` now accepts optional `{ "shard_id": "shard_N" }` body
- `GenerateQuestionsHandler` reads session metadata: shard set → `LoadContextForShard`; no shard → budget path unchanged
- `TrackPage`: fetches split plan on load, yellow banner with shard counts and edit hint
- Shard picker modal — intercepts "+ Start Session" when approved shards exist; shows id / token count / file count per shard

---

## [0.10.0] — 2026-04-15 — T2: split plan generator

### Added
- `LoadFullContext` — loads entire inherited corpus without token cap (measurement only)
- `WriteSplitPlan` — greedy first-fit-decreasing bin-packing into shards ≤ soft limit; writes `_split_plan.md` with YAML frontmatter + human-readable body
- Hard limit gate in `GenerateQuestionsHandler.run`: total > 300k → `ErrHardLimitExceeded`, records `hard_limit_hit` metric
- Soft limit gate: total > 250k → `WriteSplitPlan` + `ErrSplitRequired`, records `soft_limit_hit` metric

---

## [0.9.1] — 2026-04-15 — File-persisted metrics + GET /api/metrics

### Added
- `internal/metrics` package: `Recorder` appends events as JSONL to `metrics.jsonl`; replays counts on restart
- `Event` struct: `ts`, `event`, `track_id`, `session_num`, `tokens`, `extra`
- Instrumented events: `context_load`, `questions_generated`, `evaluation_run`, `synthesis_generated`
- `GET /api/metrics` — returns `{ uptime_seconds, counts{}, recent[] }` (last 50 events)
- `Recorder` threaded into `GenerateQuestionsHandler`, `EvaluateResponsesHandler`, `GenerateSynthesisHandler`

---

## [0.9.0] — 2026-04-15 — T1: soft/hard token limit config

### Added
- `Config.SoftTokenLimit` (default 250,000) + `Config.HardTokenLimit` (default 300,000)
- `AXON_SOFT_LIMIT` / `AXON_HARD_LIMIT` env vars read in `main.go`
- `ErrSplitRequired` + `ErrHardLimitExceeded` sentinel errors in `filesystem` package
- `GenerateQuestionsHandler` stores both limits (wired in T2)

---

## [0.8.0] — 2026-04-14 — Inherited context + claudecli auth + token budget

### Added
- `internal/llm/claudecli` — shells out to `claude --print --output-format stream-json --verbose`; 1 MB scanner buffer; parses `assistant` SSE events; no API key required
- `internal/llm/tokens.go` — `CountTokensNaive` `(len+3)/4`; `TruncateToTokenLimit`
- `LoadInheritedContext` on `TrackStore` — walks `track_N_M → track_N → root`, child files win on collision, halts cleanly at `AXON_CONTEXT_LIMIT` (default 80k)
- `ContextBudget` struct: `Files`, `TokensUsed`, `Truncated`, `TruncatedAt`
- `AXON_CONTEXT_LIMIT` env var in `main.go`
- Truncation notice streamed to UI when budget hit
- `track_1/context/browser_cli_parity.snapshot.md` — corpus snapshot (~14.9k tokens)
- `track_1/context/agent_state_machine.snapshot.md` — corpus snapshot (~6k tokens)

### Changed
- `claudecli` is now the default LLM client; Anthropic HTTP only when `ANTHROPIC_API_KEY` is set
- Removed all "API key not set" 503 guards
- `GenerateQuestionsHandler` uses `LoadInheritedContext` instead of bare `ReadContextFiles`

---

## [0.7.0] — 2026-04-14 — Phase 6: synthesis + apply

### Added
- `GenerateSynthesisCommand` → LLM synthesizes bloom advancement rules + spaced repetition schedule, writes `04_synthesis.json`
- `ApplySynthesisCommand` → patches `concept_map.json` bloom_current + spaced_repetition; idempotent (`applied` flag)
- `GetSynthesisQuery` → returns null when file absent
- Bloom rules: advance +1 if avg_correctness ≥ 0.75 across ≥ 2 questions; drop -1 if < 0.4 with misconception
- Spaced repetition intervals: 3d (dropped) / 7d (unchanged) / 14d (advanced)
- Synthesis panel: learner summary, concept update table (before→delta→after), next review date, Apply + re-synthesise buttons, "Applied" badge
- `POST /synthesize` (SSE), `GET /synthesis`, `POST /apply-synthesis` routes

---

## [0.6.0] — 2026-04-14 — Phase 5: quiz flow

### Added
- `SubmitResponsesCommand` → writes `02_responses.json`
- `EvaluateResponsesCommand` → streams LLM evaluation; computes Brier score `(correctness − confidence_normalised)²`; writes `03_evaluations.json`
- Calibration flags: `overconfident` / `underconfident` (±0.3 threshold)
- Time signal: `fast` / `on_time` / `slow` / `very_slow` vs expected seconds
- `GetSessionResponsesQuery`, `GetSessionEvaluationsQuery` → return empty arrays when absent
- Session page quiz flow: MCQ option buttons, free-text textarea, confidence 1–5 picker, explanation field, sticky submit bar (N/N answered), read-only after submission
- Evaluation cards: correctness bar, bloom-demonstrated badge, calibration/error taxonomy, feedback, correct answer reveal
- `POST /responses`, `GET /responses`, `POST /evaluate` (SSE), `GET /evaluations` routes

### Fixed
- `ReadSessionFile` returns `(nil, nil)` on `os.IsNotExist` instead of propagating error; all query handlers check `if b != nil` before unmarshaling

---

## [0.5.0] — 2026-04-14 — Phase 4: question generation

### Added
- `internal/llm/client.go` — `llm.Client` interface: `Stream(ctx, model, system, messages, maxTokens) <-chan Chunk`
- `internal/llm/anthropic` — raw HTTP+SSE client, parses `content_block_delta` events
- `GenerateQuestionsCommand` → adaptive prompt (concept map + context files), streams, writes `01_questions.json`
- `GetSessionQuestionsQuery` → returns empty array when file absent
- `CreateSessionCommand` → allocates `session_NNN/` directory
- Session page: generate button, SSE stream display, question cards (MCQ + free-text)
- `POST /sessions`, `POST /questions/generate` (SSE), `GET /questions` routes
- SSE pattern: all streaming routes use `fasthttp.StreamWriter` + `bufio.Writer.Flush()`; React side uses POST+fetch ReadableStream (EventSource is GET-only)

### Fixed
- Session page generate card gate: `!questions` → `!questions || questions.length === 0`

---

## [0.4.0] — 2026-04-13 — Phase 3: track management

### Added
- `DuplicateTrackCommand` → creates child track; preserves `bloom_current`; seeds `_sources.md`
- `UpdateContextCommand` + `GET /tracks/:id/context` + `PUT /tracks/:id/context/:filename`
- `GetTrackContextQuery`
- Context editor page: file list, textarea, save, inline new file creation
- `POST /tracks/:id/duplicate` → returns `{ new_track_id }`, navigates immediately
- `WriteContextFile` / `ReadContextFiles` on `TrackStore`

---

## [0.3.0] — 2026-04-13 — Phase 2: React UI

### Added
- Vite + React + TypeScript + Tailwind v4 + shadcn/ui (replaced HTMX)
- Sidebar: track tree, collapsible parent/child, branch initials
- Breadcrumb: Home → track_N → Context / Session N
- Track page: concept map heatmap (bloom colour-coded), session list, step badges P/Q/R/E/S
- Home page, 404 page, SPA fallback (`/*` → `index.html`)
- `api/client.ts` — typed fetch helpers: `get`, `post`, `postJSON`, `put`
- `api/types.ts` — TypeScript mirrors of all Go domain types

---

## [0.2.0] — 2026-04-13 — Phase 1: Go server skeleton

### Added
- `go.mod` — module `github.com/tyrohunt/axon`, Fiber v2 dependency
- `main.go` — entry point; `AXON_DIR` env (experiments dir), `PORT` env (default 3456)
- `internal/domain/types.go` — all domain types: `Track`, `ConceptMap`, `Concept`, `Session`, `Question`, `Response`, `Evaluation`, `Synthesis`, `SteerIntent`, `SpacedRepetition`
- `internal/store/interface.go` — backend-agnostic `Collection[T]` interface; MongoDB-style `Filter`/`Update` with `$set`, `$inc`, `$push`, `$unset` operators and dot-notation nested field access; `ErrNotFound` type
- `internal/store/filesystem/collection.go` — JSON-file `Collection[T]` implementation with `sync.RWMutex`
- `internal/store/filesystem/track_store.go` — `TrackStore`: `ListTracks` (parent/child tree), `GetTrack`, `GetConceptMap`, `WriteConceptMap`, `CreateTrack`, `ListSessions`, `NextTrackID`, `NextSessionNumber`
- `internal/cqrs/bus.go` — `CommandBus` and `QueryBus` with type-safe generic `Register`/`RegisterQuery`/`Dispatch`/`Ask` helpers
- `internal/queries/list_tracks.go` — `ListTracksQuery` → `ListTracksResult`
- `internal/queries/get_track.go` — `GetTrackQuery` → `GetTrackResult` (track + concept map + sessions)
- `server/server.go` — composition root: wires stores → buses → Fiber routes
- `ROADMAP.md`, `CHANGELOG.md` — project management docs

---

## [0.1.0] — 2026-04-12 — Foundation scaffold

### Added
- Learning framework design documents:
  - `how_to.md` — step-by-step session guide, folder structure, track naming convention
  - `concept_taxonomy.md` — concept map schema, bottleneck detection algorithm, bloom_current update rules
  - `plan.md` — original D1–D6 baseline design + analytics layer (Brier score, error taxonomy, spaced repetition)
  - `experiments_log.md` — session log (local only)
- Track 1 — ML Fundamentals · ML Math Theory · RAG Architecture · LLM Systems:
  - `track_1/README.md` — track parameters and branch definitions
  - `track_1/concept_map.json` — 32-concept seed map, bloom_current = 1 for all, bottleneck graph
  - `track_1/context/_sources.md` — external source registry (3 files to snapshot before session 1)
  - `track_1/prompts/00_concept_map_generator.md` — generates concept_map.json from branch names
  - `track_1/prompts/01_profile_from_docs.md` — attribution-corrected profile from corpus
  - `track_1/prompts/02_question_generator.md` — concept-graph-aware question generator with `concept_indexes`
  - `track_1/prompts/03_response_evaluator.md` — per-response evaluator with `bloom_level_demonstrated`
  - `track_1/prompts/04_session_synthesizer.md` — bloom_current updater, outputs `concept_map_updates` diff
- Generic prompts in `prompts/` (D1–D6 notation, framework-level, not track-specific)
