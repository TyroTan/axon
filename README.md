# Axon

**Adaptive knowledge assessment for ML practitioners — local-first, no platform required.**

Axon builds a behavioral skill profile from your own technical documents, then drives calibrated quiz sessions using Bloom's Taxonomy and a concept prerequisite graph. It tracks not just whether you got the answer right, but whether you knew *why* — and whether your confidence was warranted.

---

## What · Why · How

**What:** A local Go server + browser UI that turns your `.md` files into an adaptive quiz engine. Sessions are file-persisted, concepts are tracked individually, and every run is reproducible.

**Why:** Standard self-assessment is unreliable — you cannot accurately gauge your own blind spots by introspection. Flashcard systems (Anki, Quizlet) track recall, not understanding. Axon separates surface-level pattern matching from genuine mechanism knowledge via explanation scoring and Brier score calibration.

**How:** You define a *track* — a set of 3–5 major knowledge branches (e.g. "RAG Architecture · LLM Systems · ML Fundamentals"). Axon generates a concept map with a prerequisite bottleneck graph. Each quiz session asks questions calibrated to your current Bloom's level per concept, advancing through the graph as mastery is demonstrated. All data lives in plain JSON files you own.

---

## Getting Started

**Prerequisites:** Go 1.21+, `claude` CLI authenticated (`claude --version`), a browser. No API key required.

```bash
cd .experiments
go run .
# → API on http://localhost:3456
# → UI on http://localhost:5173 (start Vite separately: cd ui && npm run dev)
```

### Environment variables

| Variable | Default | Description |
|---|---|---|
| `AXON_DIR` | binary directory | Absolute path to `.experiments/` folder |
| `PORT` | `3456` | Go API server port |
| `AXON_DIST_DIR` | `$AXON_DIR/web/dist` | Built React app (production only) |
| `ANTHROPIC_API_KEY` | *(unset)* | Set to use Anthropic HTTP API; omit to use `claude` CLI auth |
| `AXON_CONTEXT_LIMIT` | `50000` | Max tokens passed to LLM per generation call |
| `AXON_SOFT_LIMIT` | `250000` | Corpus size that triggers split plan generation |
| `AXON_HARD_LIMIT` | `300000` | Corpus size that blocks generation entirely |

---

## Usage — Core Workflows

### 1. Start a session on a small corpus (≤ 250k tokens)

The fast path — no splitting required.

1. Open the track page → **+ Start Session**
2. Click **Generate Questions** — streams 8 questions from the LLM
3. Answer each question: pick MCQ option or write free-text, rate confidence 1–5, add an explanation
4. Click **Submit** → **Evaluate** — LLM scores every answer with Brier calibration and error taxonomy
5. Click **Synthesise** → review concept bloom updates → **Apply** to commit changes to `concept_map.json`

Each step writes a file: `01_questions.json` → `02_responses.json` → `03_evaluations.json` → `04_synthesis.json`.

---

### 2. Add context documents to a track

Context files live in `track_N/context/`. They are inherited by child tracks (`track_N_M` inherits from `track_N`).

1. Track page → **Edit Context**
2. Click any file to edit, or type a new filename and **Create**
3. Save — the file is immediately available for the next session's question generation

Child tracks override parent files on filename collision. Token counting is naive (`(bytes+3)/4`) but deterministic.

---

### 3. Large corpus — split plan workflow (> 250k tokens)

When total inherited context exceeds `AXON_SOFT_LIMIT`:

1. **Generate Questions** is blocked — a yellow banner appears on the track page
2. Open **Edit Context** — a `_split_plan.md` file has been written with greedy bin-packed shards
3. Edit each shard's `status: pending` → `status: approved` for the shards you want to study
4. Save and return to the track page
5. **+ Start Session** → shard picker modal appears — choose which chapter to study
6. The session is tagged with the shard ID; only that shard's files are sent to the LLM
7. After completing sessions for all approved shards, the **Meta-Synthesis** panel unlocks
8. Click **Generate meta-synthesis** — aggregates evaluations across shards → unified bloom update
9. Click **Apply to concept map** to commit

---

### 4. Duplicate a track

Creates a child track (`track_1` → `track_1_2`) that inherits the parent's context files and concept map.

Track page → **Duplicate** → navigates to the new child track immediately.

Useful for: experimenting with a different context slice, resetting bloom levels for a retake, testing a subset of concepts.

---

### 5. Merge tracks into a composite track

Combines the compiled (fully cascaded) contexts of two or more tracks into a new track. Source tracks are never modified.

1. Open any track page → **Merge…**
2. Check the additional tracks to merge in (current track is pre-selected)
3. Optionally set the parent ID of the new track (leave blank for root)
4. Click **Merge N tracks** → navigates to the new composite track

The new track gets:
- Union of all inherited context files (first listed source wins on filename collision)
- Re-indexed union of all concept maps (intra-source prerequisite/unlock links remapped; cross-source links cleared)
- `track_meta.json` recording `is_composite: true` and the source track IDs

Composite tracks are standard tree nodes — they support sessions, synthesis, and further merging.

---

### 6. Per-question follow-up conversations

After evaluating a session, each question card has a **Context preview before sending** panel and an **Open tutoring conversation** button.

1. Click **Context preview before sending** to inspect — without firing an LLM call — the exact context that will be sent: call mode (fresh vs resumed), token breakdown, and the RAG chunks selected from your track's context files
2. Click **Open tutoring conversation** to start a streaming conversation seeded with the question, your answer, and the LLM's evaluation
3. Messages are persisted in `sessions/session_NNN/threads/{question_id}.json`; conversations survive page reloads
4. Each assistant message shows a proof badge: call mode, tokens sent, RAG chunks used, and Claude session ID prefix

Context reuse (`--resume`) is automatic when the Claude session is still live; Axon falls back to a fresh call if the session has expired.

---

### 7. Observe what the system is doing

```bash
curl http://localhost:3456/api/metrics | jq
```

Returns:
```json
{
  "uptime_seconds": 142,
  "counts": {
    "context_load": 7,
    "questions_generated": 3,
    "evaluation_run": 2,
    "synthesis_generated": 1,
    "soft_limit_hit": 0,
    "hard_limit_hit": 0
  },
  "recent": [
    { "ts": "...", "event": "context_load", "track_id": "track_1", "session_num": 4, "tokens": 21600, "extra": { "truncated": false } }
  ]
}
```

Events are also appended to `metrics.jsonl` — survives restarts and is `jq`-queryable.

```bash
# Which sessions hit the soft limit?
jq 'select(.event == "soft_limit_hit")' metrics.jsonl

# Average tokens per context load
jq -s '[.[].tokens // 0] | add / length' metrics.jsonl
```

---

## Track System

| Track ID | Meaning |
|---|---|
| `track_1` | Root track. Context files sourced from `context/`. |
| `track_1_2` | Second iteration of the same topic cluster. Inherits `track_1/context/`. |
| `track_2` | New topic combination entirely. Self-contained. |
| `track_N` (composite) | Merged from multiple source tracks. Has `track_meta.json` with `is_composite: true`. Behaves identically to any other track for sessions and synthesis. |

Each track directory:

```
track_N/
  concept_map.json          32–40 concepts, Bloom targets, bottleneck graph
  track_meta.json           (optional) is_composite, source_ids — only on merged tracks
  context/                  .md files fed to the LLM — editable via UI
    _sources.md             (optional) source registry / merge lineage note
    _split_plan.md          (auto-generated) shard assignments when corpus > soft limit
  sessions/
    session_001/
      00_metadata.json      shard_id (empty for unsplit sessions)
      01_questions.json
      02_responses.json
      03_evaluations.json
      04_synthesis.json
      threads/
        {question_id}.json  follow-up conversation history + per-message meta
  meta_synthesis.json       (auto-generated) unified bloom update across all shards
```

---

## Architecture

```
axon/
  main.go                   entry point — reads env vars, calls server.New()
  server/server.go          composition root — wires all deps, registers routes
  internal/
    domain/types.go         Track, TrackMeta, ConceptMap, Session, Question, Response,
                            Evaluation, Synthesis, MetaSynthesis, SessionMetadata,
                            Thread, ThreadMessage, ThreadMessageMeta, SteerIntent
    store/filesystem/       JSON-file store: TrackStore (incl. ReadTrackMeta/WriteTrackMeta,
                            ReadSessionFile/WriteSessionFile with threads/ subdir support)
    cqrs/bus.go             CommandBus + QueryBus — type-safe generic dispatch
    commands/               generate_questions, evaluate_responses, generate_synthesis,
                            apply_synthesis, meta_synthesis, apply_meta_synthesis,
                            create_session, duplicate_track, merge_tracks, update_context,
                            submit_responses, compact_file, thread_turn
    queries/                list_tracks, get_track, get_track_context, get_session_*,
                            get_synthesis, get_meta_synthesis, get_context_tokens, get_thread
    llm/                    Client interface — Stream + StreamResume;
                            claudecli (default, --resume session reuse) or anthropic HTTP
    rag/                    naive.go — ChunkFiles (heading-based), TopK (keyword overlap)
    metrics/                file-persisted JSONL event recorder
  ui/src/                   React + Vite + Tailwind v4 + shadcn/ui
    api/client.ts           typed fetch helpers (incl. SSE streaming methods)
    api/types.ts            TypeScript mirrors of Go domain types
    pages/                  TrackPage, SessionPage, ContextEditorPage, HomePage,
                            MonitoringPage (token budget, metrics, context tokens, API explorer)
```

**LLM auth:** defaults to the `claude` CLI binary (uses your existing Claude Code session — no API key). Set `ANTHROPIC_API_KEY` to switch to the Anthropic HTTP API.

**Monitoring:** the `/monitoring` page in the UI provides live token budget info (from `GET /api/config`), event metrics, per-file context token breakdown, and a Swagger-style API explorer with inline JSON output (long strings are collapsible). All endpoints are also accessible via `curl /api/*`.

**Store interface:** `Collection[T]` in `store/interface.go` uses MongoDB-style operators (`$set`, `$inc`, `$push`, `$unset`) and dot-notation paths. Swapping to MongoDB requires only a new store implementation — zero handler changes.

---

## Key Concepts

**Bloom's Taxonomy levels** — every question is tagged L1 (Remember) through L6 (Create). `bloom_current` advances +1 after ≥75% accuracy across ≥2 questions; drops −1 below 40% with an identified misconception.

**Bottleneck concepts** — prerequisites for 3+ other concepts. Prioritized when `bloom_current < 3` because unlocking them advances the entire dependency graph.

**Brier score** — measures calibration: `(confidence_normalised − correctness)²`. Confidently wrong is a higher-risk gap than uncertain and wrong.

**Inherited context** — child tracks inherit parent context files; child wins on filename collision. Token budget is enforced naively before each generation call.

**Split plan** — when corpus exceeds `AXON_SOFT_LIMIT`, a `_split_plan.md` is generated with greedy bin-packed shards. Human-approved via the context editor; shard-tagged sessions load only their assigned files.

**Composite track** — a merged track whose `context/` is the union of the fully cascaded contexts of N source tracks, compiled at merge time. Source tracks are untouched. Concept maps are re-indexed unions. Tracked via `track_meta.json` (`is_composite`, `source_ids`).

**Thread context reuse** — follow-up conversations use `--resume <session_id>` when the Claude session is still live, sending only the new user message. On expiry, Axon rebuilds the full seed context + prior message history and starts a fresh session. Per-message metadata records exactly what was sent.

**Naive RAG** — context retrieval for threads splits markdown files at `#` heading boundaries into chunks, scores by stopword-filtered keyword overlap against the question text, and fills an 8k-token budget greedily. No embeddings, no index — deterministic and inspectable via the thread preview endpoint.

---

## Documentation

| File | Contents |
|---|---|
| [ROADMAP.md](ROADMAP.md) | Phased delivery plan — done, queued, deferred |
| [CHANGELOG.md](CHANGELOG.md) | Version history |
| [how_to.md](how_to.md) | Manual session guide (LLM prompt workflow) |
| [concept_taxonomy.md](concept_taxonomy.md) | Concept map schema, bottleneck detection, Bloom update algorithm |
| [plan.md](plan.md) | Original system design — D1–D6, Brier score, error taxonomy, spaced repetition |

---

## License

MIT
