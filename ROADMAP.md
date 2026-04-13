# Axon — Roadmap

> Adaptive knowledge assessment system. Local-first, file-persisted, browser UI.
> Go Fiber backend · HTMX frontend · CQRS + DI · backend-agnostic store layer.

---

## Status legend

| Symbol | Meaning |
|---|---|
| ✅ | Done |
| 🔄 | In progress |
| 📋 | Todo (current batch) |
| 🔮 | Deferred (next phase) |
| ❌ | Cancelled / won't do |

---

## Phase 0 — Foundation ✅

Content scaffolding and system design. No server yet.

- ✅ Learning framework design (`how_to.md`, `concept_taxonomy.md`, `plan.md`)
- ✅ Track 1 concept map (32 concepts, 4 branches, bottleneck graph)
- ✅ All 5 prompts for track_1 (`00` through `04`)
- ✅ Context sources registry (`context/_sources.md`)
- ✅ Git init as standalone project

---

## Phase 1 — Core Backend ✅

**Goal:** `go run .` starts a server that can list tracks and read the filesystem.

- ✅ `go.mod` + project skeleton (`main.go`, `server/`, `internal/`)
- ✅ Domain types: `Track`, `ConceptMap`, `Session`, `Question`, `Response`, `Evaluation`, `Synthesis`, `SteerIntent`
- ✅ Store interface: `Collection[T]` with `FindOne`, `Find`, `FindOneAndUpdate`, `UpdateOne`, `InsertOne`, `DeleteOne` — MongoDB-style `$set/$inc/$push/$unset`, dot-notation nested access
- ✅ Filesystem store — `Collection[T]` (JSON files, RWMutex), `TrackStore` (track tree, concept map, sessions)
- ✅ CQRS buses: `CommandBus` + `QueryBus` with type-safe generic dispatch
- ✅ `ListTracksQuery` → track tree (parent/child via `_N` suffix parsing)
- ✅ `GetTrackQuery` → track + concept map + session stubs
- ✅ Composition root in `server/server.go` — all DI here
- ✅ `GET /api/tracks` and `GET /api/tracks/:id` live and returning data

---

## Phase 2 — Track UI 🔮

**Goal:** Browser shows track tree in sidebar, track home with concept map heatmap.

- 🔮 Fiber HTML template setup + HTMX CDN include
- 🔮 Layout: sidebar (track tree) + main content area + breadcrumb
- 🔮 Track home page: concept map heatmap (bloom_current vs bloom_target, color coded)
- 🔮 Session list per track
- 🔮 `[Start Session]` button → creates session folder, navigates to context editor

---

## Phase 3 — Track Management 🔮

**Goal:** Duplicate a track, edit context, name the duplicate.

- 🔮 `DuplicateTrackCommand` → creates sibling (e.g. `track_1_2`) with context snapshot from parent
- 🔮 Context editor: textarea populated from `context/` files, save → writes to filesystem
- 🔮 Track naming dialog (shown after duplicate)
- 🔮 Track tree reflects parent/child via `_N` suffix parsing

---

## Phase 4 — Question Generation 🔮

**Goal:** Generate questions from context, streamed to browser via SSE.

- 🔮 `GenerateQuestionsCommand` → calls LLM (prompt 02), streams response, persists `01_questions.json`
- 🔮 LLM client interface + Claude impl (`claude-sonnet-4-6` for generation)
- 🔮 Mock LLM client for local dev without API key
- 🔮 SSE endpoint: `POST /sessions/:id/generate` streams token-by-token
- 🔮 `generation_id` (UUID) on each run — multiple runs per session are all persisted
- 🔮 Redo: `POST /sessions/:id/redo` creates new generation_id, re-runs generation

---

## Phase 5 — Quiz Flow 🔮

**Goal:** Answer questions, get per-question evaluation, proceed to next.

- 🔮 Quiz page: question text + MCQ options + confidence slider (1–5) + explanation textarea + timer
- 🔮 `SubmitResponseCommand` → persists to `02_responses.md`
- 🔮 `EvaluateResponseCommand` → calls LLM (prompt 03), persists `03_evaluations.json`
- 🔮 Inline evaluation reveal: correct/incorrect + explanation score + Brier contribution
- 🔮 Multi-generation-id tab UI: `Run 1 | Run 2 | Run 3` (redo display)
- 🔮 Markdown rendering: Goldmark server-side, embedded in evaluation feedback

---

## Phase 6 — Synthesis 🔮

**Goal:** End-of-session synthesis, concept map updated.

- 🔮 `GenerateSynthesisCommand` → calls LLM (prompt 04, claude-opus-4-6), persists `04_synthesis.json`
- 🔮 `ApplySynthesisCommand` → writes `concept_map_updates` back to `concept_map.json`
- 🔮 Synthesis view: concept advancement list, Brier score, error taxonomy summary, learner summary
- 🔮 Concept map heatmap updates after apply

---

## Phase 7 — Steer & Redo 🔮

**Goal:** Nudge generation direction before redo.

- 🔮 `SteerIntent` domain type: direction enum + free-text note
- 🔮 `SteerResultCommand` → injects steer note into prompt 02 user message, re-runs
- 🔮 Steer dialog UI: radio buttons (slightly harder / significantly harder / slightly easier / significantly easier / focus on concept / fewer branch questions) + free-text note
- 🔮 Steer history persisted alongside generation run

---

## Phase 8 — Polish 🔮

- 🔮 `.md` file preview anywhere in the UI (Goldmark render on hover/expand)
- 🔮 Keyboard shortcuts: `n` = next question, `1–5` = confidence, `s` = submit
- 🔮 Session progress bar (Q4 of 12)
- 🔮 Concept filter in sidebar (click concept → filter questions to that concept)

---

## Deferred / Future

- 🔮 **MongoDB store impl** — implement `Collection[T]` against real MongoDB. Zero handler changes required (store interface is the seam).
- 🔮 **Multi-user** — right now path-based isolation per user is trivial; auth layer deferred.
- 🔮 **Export** — export session history as PDF or structured JSON for sharing.
- 🔮 **LLM provider abstraction** — OpenAI, Ollama, Bedrock impls of `LLMClient` interface.
