# Changelog

All notable changes to Axon are documented here.
Format: [semantic version] — date — description.

---

## [Unreleased]

---

## [0.27.0] — 2026-04-20 — v2 E1: Full cognitive fingerprint + MCP RAG server

### Added
- **Curiosity Clusters section** in `distill_threads.go:buildDistillSystem()` — captures
  concepts the learner returned to with follow-up questions beyond evaluation requirements.
  Format: concept name, evidence quote, mastery signal (solid/uncertain).
- **Mental Models That Clicked** section — framings or analogies that visibly unlocked
  understanding mid-thread. Written for future question generator re-use.
- **Mental Models That Failed** section — framings that required re-explanation or produced
  more confusion. Question generator instructed to avoid these.
- **`cmd/axon-mcp/`** — MCP stdio server (`axon-mcp` binary). Exposes `query_axon_docs`
  tool that RAG-searches all root-level `.md` files within a 5k–20k token budget.
  Uses existing `internal/rag` package (no new dependencies).
- **`CLAUDE.md`** — project-scoped session-start index for Claude Code: doc map,
  guardrails, architecture quick-ref, MCP usage instructions.
- **`.mcp.json`** — project-scoped MCP server config (does not affect other projects).
- **`plan_v2.md`** — full v2 conception: mission, dual state model, composite learner
  states (8 primary + bidirectional matrix), concept registry with research citations,
  architectural change table, critical path.
- **`roadmap_v2.md`** — 10-sprint agile PM roadmap: 7 epics (E1–E7), MoSCoW +
  t-shirt sizing, dependency graph, sprint plan, Definition of Done.
- **Step 7b** in `how_to.md` — Distill Threads workflow: when to run, what it produces,
  how it feeds into next-session question generation and fork preparation.

### Updated
- **`generate_questions.go:BuildSystemPrompt`** — extended conversation analysis instruction
  to cover all v2 sections: Curiosity Clusters → concept weight nudge; Mental Models That
  Clicked → mirror those framings; Mental Models That Failed → avoid those framings.
- **`.gitignore`** — binary ignores scoped to root-level paths (`/axon`, `/axon-mcp`)
  so `cmd/axon-mcp/` source directory is correctly tracked.
- **`how_to.md`** — updated `.experiments/` → `axon/`, restored `track_1_2` naming example.
- **`experiments_log.md`** — added track_2 to track registry.
- **`plan.md`** — updated scope header, folder tree, added §9 conversation analyzer.

### Fixed
- **Fork ≠ Clone** — `POST /tracks/:id/fork` route (was `/duplicate`); `forkTrack` client
  method (was `duplicateTrack`); `ForkTrackResult` type. Fork = child track with parent
  inheritance; Clone = new root track with physical context copy.
- **Thread one-shot seed** — removed "Open tutoring conversation" fire button. First
  `sendMessage()` now combines seed context + user's first question in a single LLM call.
  User message recorded in thread whenever non-empty (previously skipped on seed turn).

### Testable now
- Distill Threads button → output includes Curiosity Clusters, Mental Models sections
- Generate Questions after distill → framing adapts to mental model history
- `query_axon_docs` MCP tool (after `go build -o axon-mcp ./cmd/axon-mcp/`)
- Fork button → route is `/tracks/:id/fork`, child track inherits parent
- Tutoring thread → type first question directly, no fire button required

---

## [0.26.0] — 2026-04-19 — Import Job Description → interview prep context file

### Added
- **`AnalyzeJobCommand`** (`internal/commands/analyze_job.go`) — accepts raw pasted job description + optional role label; calls LLM to extract structured interview prep signal; writes `{role}.job.md` to `context/`. Output sections: Role Signal, Must-Have Skills, Likely Interview Probes, Interview Scenario Seeds, Self-Assessment Anchors, Question Format Guidance.
- **`POST /api/tracks/:id/context/import-job`** — SSE endpoint. Body: `{ role_label, job_text }`. Done event carries written filename.
- **Import Job Description panel** on ContextEditorPage — collapsible; role label input + JD textarea + Import & Analyze button; streams live output; confirms filename on completion and reloads file list.
- `api.importJob()` in `client.ts` — SSE consumer returning the written filename.
- `slugify()` helper derives filename from role label or first line of JD; falls back to `job_{date}`.

### Design
- `.job.md` files are first-class context files: injected into question generation, inherited via Fork cascade, physically copied on Clone.
- Question generation prompt already (0.21.0) recognises `## Likely Interview Probes` and `## Interview Scenario Seeds` sections and adapts framing accordingly.
- Idempotent: re-running with the same role label overwrites the existing file.

---

## [0.25.0] — 2026-04-19 — Distill Threads → context snapshot

### Added
- **`DistillThreadsCommand`** (`internal/commands/distill_threads.go`) — scans all sessions of a track for tutoring threads that have at least one learner follow-up turn; builds a structured prompt from the full conversation history; streams an LLM-generated learning signal document with five sections: Reasoning Patterns, Misconception Fingerprint, Distractor Affinities, Concepts Needing Reinforcement, Calibration Notes. Output written to `context/session_insights.snapshot.md`.
- **`POST /api/tracks/:id/distill-threads`** — SSE endpoint following the same chunk/done/error envelope as `/compact`.
- **Distill Threads button** on TrackPage — visible only when the track has at least one session; streams live output in an expandable panel below the header; button label changes to `Distilled ✓` on completion.
- `api.distillThreads()` in `client.ts` — SSE consumer matching the other streaming helpers.

### Design
- `session_insights.snapshot.md` is a first-class context file: injected into future question generation prompts, inherited via Fork cascade, physically copied on Clone. No new plumbing required.
- Threads with only the seed assistant turn (no learner reply) are skipped — no real signal.
- File is overwritten on each run; idempotent.

---

## [0.24.0] — 2026-04-19 — Clone operation

### Added
- **Clone mode** in `CreateTrackCommand` — `SourceTrackID` field triggers `clone()` path: reads source concept map (bloom_current preserved), physically copies all non-`_sources.md` context files, writes clone-origin `_sources.md`, copies `prompts/` directory. New track gets a root-level ID (track_N) with no parent.
- **`POST /api/tracks`** now accepts `{ source_id }` body field to trigger clone mode.
- **NewTrackPage** rewritten with blank / clone mode toggle; clone mode shows a radio list of all tracks (flattened tree via `flattenTracks()`).
- `api.cloneTrack(sourceId)` added to `client.ts`.

---

## [0.23.0] — 2026-04-18 — Root track creation from UI

### Added
- **`CreateTrackCommand` + `CreateTrackHandler`** — blank mode: takes branch names, writes empty concept map + `_sources.md` stub; `POST /api/tracks` body `{ branches: string[] }`.
- **`NewTrackPage`** at `/tracks/new` — branch name inputs (dynamic add/remove, Enter to add next), Create button, navigates to new track on success.
- **`/tracks/new` route** wired in `App.tsx` — was previously 404.
- `CreateTrackResult { new_track_id }` in `types.ts`; `postJSONResult<T>()` helper in `client.ts` (returns parsed JSON body, not void).
- **`api.createTrack(branches)`** in `client.ts`.

### Fixed
- **ListTracks tree assembly** — children (`track_1_2`, `track_1_3`) were not appearing in the API response. Root cause: sorted iteration snapshot root track before children were attached. Fixed with reverse-sort bottom-up processing.
- **Sidebar live re-fetch** — sidebar only fetched track list on mount; new tracks created during a session were invisible until reload. Fixed by adding `location.pathname` to the `useEffect` dependency array.
- **Child-active collapsible** — parent track in the sidebar was collapsing when navigating to a child. Fixed with `childActive` check in `TrackTree`.

---

## [0.22.0] — 2026-04-18 — Copy prompts/ on Fork, onboarding callout, DESIGN.md

### Added
- **Copy `prompts/` on Fork** — `DuplicateTrackHandler` now copies all files from the source track's `prompts/` directory into the new child track. Child tracks have the full system prompt templates for reference and manual bootstrapping without needing to locate the parent.
- **Onboarding callout** on TrackPage — shown only on fresh tracks with no sessions; card with numbered steps (Edit Context → Start Session) + note about `prompts/` directory.
- **`DESIGN.md`** — LLM-readable ground truth for the entire project: core concepts, all user stories (US-01 through US-15) with status, track naming convention (ASCII tree diagram), track lifecycle, data model file layout, LLM integration table, full API surface table.

### Fixed
- Documentation disconnect: `DESIGN.md` now correctly distinguishes Fork (child track_N_M, cascade inheritance) from Clone (root copy, physical snapshot).

---

## [0.21.1] — 2026-04-18 — Rename Duplicate → Fork

### Changed
- **Duplicate → Fork** throughout the UI — button label, loading state, and tooltip on TrackPage all renamed. Fork more precisely names the operation (creates a child track inheriting concept map, not a same-level copy).

---

## [0.21.0] — 2026-04-18 — Cognitive fingerprint instruction, interview prep context

### Added
- **Cognitive fingerprint instruction** in `BuildSystemPrompt()` (`generate_questions.go`) — when `context/` includes a file with sections `## Reasoning Patterns`, `## Misconception Fingerprint`, `## Distractor Affinities`, the question generator now uses it to adapt question framing and distractor selection (not concept selection — those are still governed by bloom state + spaced repetition).
- **`track_1_2/context/upwork_rag_jobs.snapshot.md`** — interview prep corpus for two Upwork RAG roles: RAGFlow deployment expert and Yuktha Health AI RAG audit. Includes skill matrices, likely interview probes, L4/L5 scenario seeds, cross-job concept matrix, self-assessment anchors, question format guidance.

---

## [0.20.0] — 2026-04-18 — Submit UX, RAG quality threshold, conversation markdown

### Added
- **Submit success confirmation bar** — sticky bottom bar no longer disappears silently after answers are submitted; transforms into a green "Responses submitted" banner with an "Evaluate with AI ↑" button that smooth-scrolls to the evaluate section at the top of the page. `submitDone` state drives visibility; clears on `generateQuestions` so the normal submit bar returns for a fresh attempt.
- **`evaluateSectionRef`** anchor on the phase header div — target for the scroll-to-evaluate button.

### Changed
- **RAG MinScore threshold** — `internal/rag/naive.go` now drops chunks scoring below `MinScore = 0.30` before filling the token budget. Previously all chunks were ranked and budget-filled regardless of score; low-relevance chunks (0.13 etc.) were injected as noise. Chunks with zero keyword overlap are now excluded. `MinScore` is an exported constant — set to `0` to disable.
- **Conversation markdown rendering** — `ConversationPage.tsx` now renders assistant messages with `react-markdown` (already in `package.json`, not yet wired). Headings, bold/italic, lists, inline code, fenced code blocks, blockquotes all rendered natively. Streaming buffer also uses `ReactMarkdown` so formatting appears live during generation. User messages remain `whitespace-pre-wrap` plain text.

---

## [0.19.0] — 2026-04-17 — U1: localStorage auto-save for in-progress quiz answers

### Added
- `lsKey / lsLoad / lsSave / lsClear` helpers in `SessionPage.tsx` — key format `axon:session:{trackId}:{sessionNum}`
- Debounced auto-save (1.5 s) of `answers` state to `localStorage` while the session is unanswered; stops once `savedResponses` is non-empty (i.e. already submitted to server)
- Hydration priority on load: server responses (submitted) > `localStorage` draft > blank defaults — so a page reload mid-quiz restores MCQ selections, free-text answers, explanations, and confidence ratings
- `lsClear` called on successful submit (draft no longer needed) and on `generateQuestions` success (clears stale draft from any prior question generation for the same session slot)

### Changed
- No backend changes — pure UI, zero new API calls

---

## [0.18.0] — 2026-04-17 — Composite track merge

### Added
- `domain.TrackMeta` — `{ is_composite, source_ids }` persisted as `track_meta.json` (optional sidecar; absent = plain lineage track)
- `Track` domain type gains `IsComposite bool`, `SourceIDs []string`; `readTrack` populates them when `track_meta.json` exists
- `ReadTrackMeta` / `WriteTrackMeta` on `TrackStore`
- `MergeTracksCommand` + `MergeTracksHandler` — validates sources, calls `LoadInheritedContext(limit=0)` for each (gets fully compiled cascaded snapshot), unions file maps (first source wins on collision), re-indexes concept maps (intra-source prerequisite/unlock links remapped by offset; cross-source links cleared), creates track dir, writes context files + `_sources.md` + `track_meta.json`; returns `new_track_id`, `file_count`, `total_concepts`
- `POST /api/tracks/merge` — body `{ source_ids: string[], parent_id: string }`
- TrackPage: **Merge…** button opens inline panel — checkboxes for all other tracks (current track locked-in), parent ID field, merge button navigates to new track on success
- Composite tracks display amber "composite" badge + clickable source track links in header
- MonitoringPage API explorer: merge endpoint entry in Data group

### Design
- Tree stays a tree — composite node is a standard child track; `track_meta.json` is the only new artifact
- Source tracks are never modified — merge is a read + copy operation on already-compiled snapshots

---

## [0.17.1] — 2026-04-17 — Fix synthesis date parsing

### Fixed
- `parseSynthesis` and `parseMetaSynthesis` now accept `next_review` as a bare date string (`"2026-04-23"`) in addition to full RFC3339. The LLM consistently emits date-only; `time.Time` JSON unmarshalling rejected it. Fix: intermediate `llmSpacedRepetition` + `llmConceptMapUpdate` types absorb the string, `toDomain()` tries RFC3339 first then `"2006-01-02"`.

---

## [0.17.0] — 2026-04-16 — Thread seed dry-run preview

### Added
- `ThreadTurnHandler.Preview(ctx, trackID, sessionNum, questionID)` — builds full seed context (RAG chunks, system + user prompts, call_mode, token breakdown) without calling the LLM; returns `ThreadPreviewResult`
- `ThreadPreviewChunk` — `{ file, heading, tokens, score, preview }` (120-char content preview)
- `ThreadPreviewResult` — `{ question_id, call_mode, claude_session_id, accumulated_tokens, system_prompt, user_prompt, system_tokens, user_tokens, tokens_to_send, tokens_saved_by_resume, rag_chunks }`
- `GET /api/tracks/:id/sessions/:num/threads/:qid/preview` route
- `types.ts`: `ThreadPreviewChunk`, `ThreadPreviewResult`; `client.ts`: `getThreadPreview()`
- SessionPage EvalCard: collapsible **Context preview before sending** panel — lazy-fetches on first expand, shows call_mode badge (amber = fresh, green = resumed), accumulated tokens, token grid (system / user seed / would-send / saved), RAG chunks `<details>` (file, heading, tokens, score, 120-char preview); **Open tutoring conversation** button placed below the panel
- MonitoringPage API explorer: thread preview endpoint entry

---

## [0.16.0] — 2026-04-15 — Interactive API Explorer + config fix

### Added
- MonitoringPage: Swagger-style **API Explorer** — endpoint rows with extracted `:param` text inputs, resolved URL preview, Execute button, status + elapsed, formatted JSON output
- Endpoints split into two groups: **Monitoring** (config, metrics, context-tokens, prompt-preview, meta-synthesis readiness, thread, thread preview) and **Data** (tracks, sessions, questions, responses, evaluations, synthesis, meta-synthesis)
- `JsonNode` recursive renderer — type-colored JSON tree; strings >80 chars or containing `\n` rendered as collapsible `<details>` with `pre-wrap` so long system/user prompts are readable

### Fixed
- Default `AXON_CONTEXT_LIMIT` changed from 80,000 → 50,000 tokens in `main.go`
- `GET /api/config` exposes live `context_token_limit`, `soft_token_limit`, `hard_token_limit`; MonitoringPage token budget panel and context token progress bars now use real server values instead of hardcoded constants

---

## [0.15.0] — 2026-04-15 — Per-question follow-up threads (U3)

### Added
- `domain.ThreadMessageMeta` — `{ call_mode, input_tokens_sent, context_chunks_used, claude_session_id }`
- `domain.ThreadMessage` — `{ role, content, ts time.Time, meta *ThreadMessageMeta }`
- `domain.Thread` — `{ question_id, claude_session_id, accumulated_input_tokens, messages }`
- `internal/rag/naive.go` — deterministic retrieval: `ChunkFiles` splits non-`_`-prefixed markdown files at `#` heading boundaries (then by double-newline paragraph if chunk > `maxChunkTokens`); `TopK` scores by stopword-filtered keyword overlap, fills token budget greedily
- `llm.Client` extended with `StreamResume(ctx, sessionID, system, messages, maxTokens)`
- `claudecli` client rewritten: `stream()` internal method used by both `Stream` and `StreamResume`; when `sessionID != ""` passes only the last user message as prompt + `--resume sessionID`; parses `session_id` + `usage.input_tokens` from `result` SSE event; emits both on Done chunk
- `anthropic` client: `StreamResume` stub delegates to `Stream` (HTTP API has no session resumption)
- `ThreadTurnHandler` — load thread → load q/response/eval context → retrieve RAG chunks if fresh → build seed/fresh-resume prompt → `StreamResume` → persist thread with per-message meta; `ragTokenBudget = 8000`, `ragMaxChunks = 10`, `ragChunkSize = 1000`
- `GetThreadQuery` / `GetThreadResult` — reads `sessions/session_NNN/threads/{question_id}.json`
- `WriteSessionFile` fix: uses `os.MkdirAll(filepath.Dir(dest))` so `threads/` subdirectory is created
- `BuildSystemPrompt()` / `BuildUserPrompt()` exported from `generate_questions.go` for reuse
- Routes: `GET /api/tracks/:id/sessions/:num/threads/:qid`, `POST .../threads/:qid` (SSE — Done event carries `session_id` + `input_tokens`)
- SessionPage EvalCard: per-question thread UI — messages list, per-message proof badges (call_mode, tokens sent, RAG chunk count, session ID prefix), streaming input, "Open tutoring conversation" button
- Token budget button in session header (lazy-fetches `getPromptPreview`)

### Changed
- `SessionMetadata` gains `ClaudeSessionID string`, `AccumulatedInputTokens int`

---

## [0.14.0] — 2026-04-15 — Monitoring page + prompt preview + token budget panel

### Added
- `GET /api/tracks/:id/context-tokens` — per-file token counts for full inherited context, attributed to source track; sorted by size; `GetContextTokensQuery` / `GetContextTokensResult`
- `GET /api/tracks/:id/sessions/:num/prompt-preview` — exact system + user prompt that would be sent, plus `call_mode` (fresh/resumed), `claude_session_id`, `accumulated_input_tokens`, `tokens_to_send`, `tokens_saved_by_resume`, `context_files_included`, `system_prompt_tokens`, `user_prompt_tokens`, `context_tokens`, `total_tokens`, `soft_limit`, `hard_limit`, `context_limit`
- MonitoringPage (`/monitoring`): **Token Budget** panel (live limits from `/api/config`), **Metrics** panel (uptime, per-event counts, recent event log), **Context Tokens** panel (track selector, dual progress bars vs context + soft limit, per-file table with inline share bars)
- Sidebar entry for Monitoring with `Activity` icon
- SessionPage: collapsible **Token Budget** button in session header — shows `call_mode`, `accumulated_input_tokens`, `tokens_to_send`, `tokens_saved_by_resume`, system/user/context token breakdown; lazy-fetches on first expand

---

## [0.13.0] — 2026-04-15 — T6: file-level compaction + README catch-up

### Added
- `CompactFileCommand` + `CompactFileHandler` — LLM distils a verbose context file to a target token budget (default: half); writes `filename.compact.md` with YAML frontmatter (`source_file`, `source_hash` SHA-256 prefix, `source_tokens`, `compacted_tokens`, `compacted_by`)
- `POST /api/tracks/:id/context/:filename/compact` (SSE) — optional `{ target_tokens }` body; streams LLM output
- `metrics` event `compaction_run` with source/compacted token counts and reduction %
- Context editor: **Compact** button in editor toolbar — hidden for `_`-prefixed system files and `.compact.` files; streams output inline; reloads file list after completion so the new `.compact.md` appears immediately

### Changed
- `README.md` fully rewritten to reflect current system state: env vars table, 5 usage workflows (fast path, context editor, split plan, duplicate track, metrics), accurate architecture listing, updated key concepts

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
