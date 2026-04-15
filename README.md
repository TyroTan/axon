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
| `AXON_CONTEXT_LIMIT` | `80000` | Max tokens passed to LLM per generation call |
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

### 5. Observe what the system is doing

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

Each track directory:

```
track_N/
  concept_map.json          32–40 concepts, Bloom targets, bottleneck graph
  context/                  .md files fed to the LLM — editable via UI
    _sources.md             (optional) source registry
    _split_plan.md          (auto-generated) shard assignments when corpus > soft limit
  sessions/
    session_001/
      00_metadata.json      shard_id (empty for unsplit sessions)
      01_questions.json
      02_responses.json
      03_evaluations.json
      04_synthesis.json
  meta_synthesis.json       (auto-generated) unified bloom update across all shards
```

---

## Architecture

```
axon/
  main.go                   entry point — reads env vars, calls server.New()
  server/server.go          composition root — wires all deps, registers routes
  internal/
    domain/types.go         Track, ConceptMap, Session, Question, Response,
                            Evaluation, Synthesis, MetaSynthesis, SessionMetadata, SteerIntent
    store/filesystem/       JSON-file store: TrackStore, split plan helpers
    cqrs/bus.go             CommandBus + QueryBus — type-safe generic dispatch
    commands/               generate_questions, evaluate_responses, generate_synthesis,
                            apply_synthesis, meta_synthesis, apply_meta_synthesis,
                            create_session, duplicate_track, update_context, submit_responses
    queries/                list_tracks, get_track, get_track_context, get_session_*,
                            get_synthesis, get_meta_synthesis
    llm/                    Client interface — claudecli (default) or anthropic HTTP
    metrics/                file-persisted JSONL event recorder
  ui/src/                   React + Vite + Tailwind v4 + shadcn/ui
    api/client.ts           typed fetch helpers
    api/types.ts            TypeScript mirrors of Go domain types
    pages/                  TrackPage, SessionPage, ContextEditorPage, HomePage
```

**LLM auth:** defaults to the `claude` CLI binary (uses your existing Claude Code session — no API key). Set `ANTHROPIC_API_KEY` to switch to the Anthropic HTTP API.

**Store interface:** `Collection[T]` in `store/interface.go` uses MongoDB-style operators (`$set`, `$inc`, `$push`, `$unset`) and dot-notation paths. Swapping to MongoDB requires only a new store implementation — zero handler changes.

---

## Key Concepts

**Bloom's Taxonomy levels** — every question is tagged L1 (Remember) through L6 (Create). `bloom_current` advances +1 after ≥75% accuracy across ≥2 questions; drops −1 below 40% with an identified misconception.

**Bottleneck concepts** — prerequisites for 3+ other concepts. Prioritized when `bloom_current < 3` because unlocking them advances the entire dependency graph.

**Brier score** — measures calibration: `(confidence_normalised − correctness)²`. Confidently wrong is a higher-risk gap than uncertain and wrong.

**Inherited context** — child tracks inherit parent context files; child wins on filename collision. Token budget is enforced naively before each generation call.

**Split plan** — when corpus exceeds `AXON_SOFT_LIMIT`, a `_split_plan.md` is generated with greedy bin-packed shards. Human-approved via the context editor; shard-tagged sessions load only their assigned files.

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
