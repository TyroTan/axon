# Axon — Design Reference

> **Purpose of this file:** LLM-readable ground truth for the Axon project.
> Use this when answering questions like: "What features exist?", "How does X work?",
> "What is the status of Y?", "How does a user do Z?", "What is the data model?"
>
> Complements: ROADMAP.md (what's planned), CHANGELOG.md (what changed when),
> README.md (quick-start + usage), plan.md (learning system theory).

Last updated: 2026-04-18

---

## 1. What Axon is

Axon is a **local-first adaptive knowledge assessment system**. It turns your own `.md`
documents into calibrated quiz sessions driven by Bloom's Taxonomy and a concept
prerequisite graph. It does not require an account, an API key, or a cloud service.

Stack: Go Fiber backend · React + shadcn/ui frontend · filesystem JSON store ·
`claude` CLI auth (no API key required by default) · CQRS command/query bus.

---

## 2. Core concepts

### Track
The unit of study. A track covers 3–5 major knowledge branches (e.g. "RAG Architecture ·
LLM Systems"). A track has:
- `concept_map.json` — 32–40 concepts, each with bloom_current, bloom_target,
  is_bottleneck, prerequisite_indexes, unlocks_indexes, spaced_repetition schedule
- `context/` — `.md` files injected into the question generator as full text
- `sessions/` — numbered session directories, each containing the five step files
- `track_meta.json` (optional) — only on composite/merged tracks; records is_composite + source_ids

Tracks form a tree by ID convention: `track_1` is a root, `track_1_2` is a child of
`track_1`, `track_1_2_3` is a child of `track_1_2`. IDs are assigned automatically
by `NextTrackID()` at creation time.

### Session
A single quiz cycle inside a track. Five sequential steps, each writing a file:

| Step | File | Description |
|---|---|---|
| 0 | `00_metadata.json` | Session metadata: creation time, shard_id if large corpus |
| 1 | `01_questions.json` | Generated questions (MCQ, free_text, scenario_mcq, interview_scenario) |
| 2 | `02_responses.json` | User's submitted answers, confidence ratings, explanations |
| 3 | `03_evaluations.json` | LLM evaluation: correctness, Brier score, bloom_demonstrated, error taxonomy |
| 4 | `04_synthesis.json` | Per-concept bloom advancement rules, spaced repetition schedule |

A session is not submitted until the user explicitly clicks Submit. Unanswered drafts
are held in `localStorage` (`axon:session:{trackId}:{sessionNum}`), auto-saved with
1.5 s debounce, cleared on submit.

### Concept Map
The learning graph. Contains all concepts for a track with:
- `bloom_current` — where the learner is now (1–6), updated by Apply Synthesis
- `bloom_target` — ceiling for this concept in this track (2–5)
- `is_bottleneck` — true if 3+ other concepts depend on this one
- `prerequisite_indexes` — concepts to master before this one is introduced
- `spaced_repetition` — next_review date, interval_days, consecutive_correct count

The concept map is the **memory of the track**. All other files (sessions, evaluations)
are ephemeral relative to the concept map.

### Context Files
Plain `.md` files in `track_N/context/`. They are:
- Injected **whole** (no RAG) into the question generation prompt
- Inherited parent → child (child file wins on filename collision)
- Excluded from injection if filename starts with `_` (`_sources.md`, `_split_plan.md`)
- Subject to the token budget (`AXON_CONTEXT_LIMIT` default 50k)

For conversations and thread tutoring, context files are **chunked** by markdown headings
and ranked by keyword overlap (`rag/naive.go`). Chunks scoring below `MinScore = 0.30`
are discarded before injection.

---

## 3. User stories — implemented

### US-01: Study a topic I understand using my own documents
1. Track exists with context files loaded
2. Open track page → **+ Start Session**
3. **Generate Questions** → streams 8 questions (MCQ + free_text, Bloom-calibrated)
4. Answer each question: MCQ option, free-text, confidence 1–5, optional explanation
5. **Submit** → sticky bar transforms to green confirmation + "Evaluate with AI ↑"
6. **Evaluate** → LLM scores every response with Brier calibration, bloom_demonstrated,
   error taxonomy, distractor explanation
7. **Synthesise** → LLM aggregates session results → bloom advancement proposals
8. **Apply** → patches `concept_map.json` (idempotent)

**Status: ✅ fully implemented**

### US-02: Pick up where I left off after closing the browser
- `localStorage` draft auto-saves every 1.5 s while answering
- On reload: server responses > localStorage draft > blank defaults
- Clears on submit and on Regenerate

**Status: ✅ fully implemented**

### US-03: Create a new root track (new topic cluster)
- Click **+** in the sidebar → `/tracks/new`
- Enter 2–5 major branch names (e.g. "RAG Architecture · LLM Systems")
- Click **Create** → navigates to new track (e.g. `track_2`)
- Track is created with an empty concept map and `_sources.md` stub
- Next: **Edit Context** to add `.md` files, then run `prompts/00_concept_map_generator.md` manually
  in Claude to populate `concept_map.json`, then start sessions

**Status: ✅ fully implemented** (`POST /api/tracks`, `NewTrackPage`)

### US-04a: Create a child track (specialize or iterate on a parent)
- Open the parent track (e.g. `track_1`) → click **Duplicate**
- Child track ID is assigned automatically: `track_1` → `track_1_2` → `track_1_2_2`
- Child inherits parent's `concept_map.json` (with `bloom_current` preserved) and all `prompts/`
- Child's `context/` starts with only `_sources.md`; add new `.md` files via Edit Context
- Context is inherited at question-generation time: child files + parent files (child wins on collision)

**Status: ✅ fully implemented** (Duplicate button on TrackPage)

### US-04: Specialize a track for a specific job or context
1. Open parent track → **Duplicate** → navigates to child track
2. **Edit Context** → create a new `.md` file with job/interview-specific content
3. Start a session — child context is merged with inherited parent context

**Status: ✅ fully implemented (Duplicate + Edit Context)**

### US-05: Combine knowledge from two tracks
1. Open any track → **Merge…** panel
2. Check additional source tracks, set optional parent ID
3. Click **Merge** → new composite track with union of compiled context + re-indexed concept maps

Composite tracks are standard tree nodes — they support sessions, synthesis, further merging.

**Status: ✅ fully implemented**

### US-06: Handle a very large corpus (> 250k tokens)
1. Generate Questions is blocked — yellow banner on track page
2. `_split_plan.md` written to `context/` with YAML frontmatter shards
3. Edit context: set shard `status: pending → approved`
4. Session creation → shard picker modal → session tagged with shard_id
5. After all shards: **Meta-Synthesis** aggregates bloom updates across shards

**Status: ✅ fully implemented (T1–T6)**

### US-07: Ask a follow-up question after being evaluated
- Each evaluation card has **Open tutoring conversation** (thread per question)
- First click: seeds context with question + answer + evaluation + RAG chunks
- Subsequent messages: `--resume` session reuse (fresh fallback on expiry)
- Per-message proof badge: call_mode, tokens sent, RAG chunks, Claude session ID prefix

**Status: ✅ fully implemented**

### US-08: Preview exactly what the LLM will receive before generating
- Session page header → **Token budget** collapsible panel
- Shows: call_mode (fresh/resumed), token grid (system / context / user), context files included, saved tokens from resume
- Thread tutoring: **Context preview** panel on each eval card (lazy-fetched on first open)

**Status: ✅ fully implemented**

### US-09: Start a free-form conversation across multiple tracks
1. Sidebar → **Conversations** section → **+ New conversation**
2. Select one or more tracks as context sources, give the conversation a title
3. Chat interface: streaming assistant responses with markdown rendering
4. **+ Context** panel: inject an ad-hoc text snippet into the next turn
5. **Index** panel: generate a structured summary (topics, key decisions, open questions,
   per-ply breakdown) via LLM → writes `{id}.index.json`

Conversations are stored in `experiments/conversations/` (not under any track).
RAG is applied per turn: context files from all selected tracks are chunked, scored
(MinScore 0.30), and top-K injected alongside the user message.

**Status: ✅ fully implemented**

### US-10: Compact a large context file
- Edit Context page → **Compact** button on a verbose file
- Writes `filename.compact.md` with hash frontmatter for cache invalidation
- Manual trigger only — not run automatically

**Status: ✅ fully implemented**

### US-11: Monitor token usage and explore the API
- `/monitoring` page: token budget panel, metrics panel, context token breakdown
- Interactive API explorer: two endpoint groups (Monitoring / Data), param inputs,
  Execute button, elapsed time, recursive JSON renderer

**Status: ✅ fully implemented**

---

## 4. User stories — planned / deferred

### US-12: Use a conversation to improve future quiz calibration (Cognitive Fingerprint)
A free-form conversation reveals how the learner *thinks*, not just what they know.
Running `AnalyzeConversationCommand` extracts a cognitive fingerprint — reasoning style,
misconception framings, curiosity clusters — into a `conversation_analysis.snapshot.md`
file written to the target track's `context/`. The question generator already has an
instruction to use this file when present (0.21.0).

**Status: 📋 queued — backend command + UI entry point not yet built**
See ROADMAP.md → "Conversation cognitive fingerprint"

### US-13: Re-run questions with a different steering bias
Phase 7 — Steer & Redo. User can set direction (harder/easier/focus concept) and get a
new generation run without losing the current one. Multiple generation tabs per session.

**Status: 🔮 deferred**

### US-14: Create a shared pool of context files used across tracks
T7 — Corpora pool. `/.experiments/corpora/` shared pool; `_corpus_refs.json` per track
references pool entries; UI to import, attach, materialise as snapshot.

**Status: 📋 queued**

### US-15: Upgrade retrieval from keyword overlap to semantic embeddings
Embedding retrieval behind the existing `Retriever` interface (planned). Current naive
retrieval is binary bag-of-words; TF-IDF and then embedding cosine similarity are the
next two steps. Fine-tuning loop on session-labeled data is a longer-term goal.

**Status: 📋 queued — Retriever interface refactor first**

---

## 5. Track naming convention and tree structure

Track IDs encode the tree structure directly:

```
track_1          ← root (new topic cluster, created via + button)
  track_1_2      ← child of track_1 (Duplicate button inside track_1)
    track_1_2_2  ← child of track_1_2 (Duplicate button inside track_1_2)
  track_1_3      ← second child of track_1

track_2          ← second root track (different topic entirely, + button)
  track_2_2      ← child of track_2
```

**Root tracks** (`track_N`): new topic cluster, no parent, created via `+` sidebar → `/tracks/new`.
Concept map starts empty; user populates it via `prompts/00_concept_map_generator.md`.

**Child tracks** (`track_N_M`): specialization of a parent, created via Duplicate button on
the parent's track page. Inherits `concept_map.json` (with `bloom_current` preserved) and `prompts/`.
Context is inherited at question-generation time — no copying, just cascade via `LoadInheritedContext`.

The `_` separator is significant: `NextTrackID("track_1")` → `track_1_2`, `track_1_3`, …
`NextTrackID("")` → `track_1`, `track_2`, …

---

## 6. Track lifecycle — how a track is born

All root tracks are created via `+` in the sidebar. The very first root track (`track_1`) was bootstrapped manually before the `POST /api/tracks` endpoint existed:

1. Added context `.md` files to `track_1/context/` (copied from the wider repo — `BROWSER_CLI_PARITY.md`,
   `agent_state_machine.md`, `debug_usage.md` — per the instructions in `context/_sources.md`)
2. Ran `prompts/00_concept_map_generator.md` as a system prompt in Claude with the branch names
   as user message → pasted the JSON output into `track_1/concept_map.json`
3. Ran `prompts/01_profile_from_docs.md` with the context files as input → wrote
   `sessions/session_001/00_profile_snapshot.json` (starting bloom profile)

After that one-time bootstrap, all subsequent operation is automated by Axon's backend.

### What `prompts/` actually is

The `prompts/` directory contains **human-readable system prompt templates** that mirror what
Axon's backend does automatically. They are NOT executed by the backend. Their purposes:

| File | What it documents | When you'd use it manually |
|---|---|---|
| `00_concept_map_generator.md` | How to generate a concept map from branch names | Only when creating a brand-new root track from scratch |
| `01_profile_from_docs.md` | How to build a starting bloom profile from context files | Only for a root track's first session before any quiz data exists |
| `02_question_generator.md` | The question generation schema + rules | Reference / debugging — the backend uses `BuildSystemPrompt()` in `generate_questions.go` |
| `03_response_evaluator.md` | The evaluation schema + Brier scoring rules | Reference only |
| `04_session_synthesizer.md` | The synthesis + bloom advancement rules | Reference only |

Child tracks created via Duplicate get a copy of `prompts/` automatically (since 0.22.0).
They do not need to re-run prompts 00 or 01 — the concept map and context are inherited.

---

## 7. Data model — file layout

```
axon/
  track_N/
    concept_map.json          concept graph with bloom state
    track_meta.json           optional; is_composite + source_ids
    context/
      _sources.md             human-only; not sent to LLM
      _split_plan.md          auto-generated when corpus > soft limit
      *.md / *.snapshot.md    context injected into questions
    sessions/
      session_NNN/
        00_metadata.json      shard_id (if large corpus)
        01_questions.json     generated questions
        02_responses.json     user answers
        03_evaluations.json   LLM evaluations
        04_synthesis.json     bloom update proposals
        threads/
          {question_id}.json  per-question tutoring thread
    prompts/                  manual bootstrap prompts (human use only)
    README.md                 track-level description
  conversations/
    {uuid}.json               ConversationMessage[] with per-message meta
    {uuid}.index.json         ConversationIndex (summary, topics, per-ply breakdown)
  metrics.jsonl               append-only operational telemetry
  dev.sh                      build UI + start server in one command
```

---

## 8. LLM integration

All LLM calls go through `llm.Client` interface (`Stream` + `StreamResume`). Two implementations:

| Implementation | When used | Auth |
|---|---|---|
| `claudecli` | Default (no env var) | `claude` CLI binary — no API key needed |
| `anthropic` | When `ANTHROPIC_API_KEY` is set | Direct HTTP SSE — no SDK |

`StreamResume` passes `--resume <sessionID>` (claudecli) or falls back to fresh (anthropic).
`StreamResume` used in: thread tutoring, conversation turns.
`Stream` used in: question generation, evaluation, synthesis, meta-synthesis, compaction.

---

## 9. API surface (selected)

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/tracks` | List all tracks |
| POST | `/api/tracks` | Create new root track. Body: `{ branches: string[] }`. Returns `new_track_id` |
| GET | `/api/tracks/:id` | Track detail + concept map |
| POST | `/api/tracks/:id/duplicate` | Create child track (`track_N` → `track_N_2`) |
| POST | `/api/tracks/merge` | Create composite track from multiple sources |
| GET/PUT | `/api/tracks/:id/context/:filename` | Read/write context file |
| POST | `/api/tracks/:id/context/:filename/compact` | SSE compact |
| GET | `/api/tracks/:id/split-plan` | Read split plan |
| POST | `/api/tracks/:id/sessions` | Allocate session directory |
| POST | `/api/tracks/:id/sessions/:num/questions/generate` | SSE question generation |
| POST | `/api/tracks/:id/sessions/:num/responses` | Submit responses |
| POST | `/api/tracks/:id/sessions/:num/evaluate` | SSE evaluation |
| POST | `/api/tracks/:id/sessions/:num/synthesize` | SSE synthesis |
| POST | `/api/tracks/:id/sessions/:num/apply-synthesis` | Apply synthesis to concept map |
| POST | `/api/tracks/:id/meta-synthesis/generate` | SSE meta-synthesis |
| POST | `/api/tracks/:id/meta-synthesis/apply` | Apply meta-synthesis |
| GET | `/api/tracks/:id/sessions/:num/prompt-preview` | Token budget preview |
| POST | `/api/tracks/:id/sessions/:num/threads/:qid` | SSE thread turn |
| GET | `/api/tracks/:id/sessions/:num/threads/:qid/preview` | Thread context preview |
| GET | `/api/conversations` | List conversations |
| GET | `/api/conversations/:id` | Get conversation + optional index |
| POST | `/api/conversations` | Create conversation |
| POST | `/api/conversations/:id/turn` | SSE conversation turn |
| POST | `/api/conversations/:id/context` | Add ad-hoc context chunk |
| GET | `/api/conversations/:id/index` | Get conversation index |
| POST | `/api/conversations/:id/index` | SSE generate conversation index |
| GET | `/api/config` | Live server config (limits, mode) |
| GET | `/api/metrics` | Operational metrics |

Full interactive explorer: `/monitoring` → API Explorer.
