# Axon — Claude Code Project Index

> This file is auto-loaded by Claude Code at session start.
> Use the `query_axon_docs` MCP tool to fetch relevant sections before answering.
> Token budget: 5k–20k depending on query depth.

---

## How to use this project's docs

Before answering questions about rules, conventions, plans, or roadmap items,
call `query_axon_docs` with a relevant topic. Do not guess — look it up.

```
query_axon_docs("track naming convention")
query_axon_docs("fork vs clone difference")
query_axon_docs("sprint 1 items")
query_axon_docs("composite learner state")
query_axon_docs("gitignore rules sessions")
```

---

## Document Index

| File | What it governs |
|---|---|
| `how_to.md` | Step-by-step usage: track creation, session workflow, fork/clone, naming conventions |
| `plan.md` | v1 architecture: dimensions D1–D6, Bloom's distribution, spaced repetition schedule |
| `plan_v2.md` | v2 design: dual state model, composite learner states, concept registry, research citations |
| `roadmap_v2.md` | v2 PM backlog: epics, MoSCoW priority, sprint plan, dependency graph, DoD |
| `concept_taxonomy.md` | Concept map schema, bottleneck detection, bloom update rules |
| `experiments_log.md` | Track registry, session log entries across all tracks |
| `DESIGN.md` | System architecture decisions |
| `ROADMAP.md` | High-level roadmap (v1) |
| `CHANGELOG.md` | Release history |

---

## Key guardrails (always apply)

- **Track data is VCS-tracked.** Sessions, contexts, conversations are committed. Never gitignore them.
- **Only `metrics.jsonl` is gitignored.** Nothing else at track level.
- **Fork ≠ Clone.** Fork = child track (track_1_2), inherits parent. Clone = new root (track_2), no parent. Route: `POST /tracks/:id/fork` vs `POST /tracks`.
- **Active session data is immutable.** If a session has evaluations or synthesis, do not mutate its files.
- **Evidence state and exploration state never mix** in scoring logic (v2 dual state model).
- **Files starting with `_` are skipped** by the question generator (e.g. `_sources.md`).

---

## Architecture quick reference

```
axon/
  cmd/axon-mcp/     ← this MCP server (query_axon_docs tool)
  internal/
    rag/            ← naive word-overlap RAG (ChunkFiles + TopK)
    llm/claudecli/  ← Go wrapper around `claude` CLI binary
    commands/       ← all business logic (generate, evaluate, thread, etc.)
    store/          ← filesystem-backed track/session store
  server/           ← Fiber HTTP API
  ui/               ← React frontend
  track_*/          ← VCS-tracked learning tracks
  prompts/          ← generic prompt templates (01–04)
```

---

## Active tracks

| Track | Topic | Status |
|---|---|---|
| `track_1` | ML Fundamentals · RAG · LLM Systems | v1 baseline |
| `track_2` | Upwork RAG interview prep | active session |
| `track_3` | (see experiments_log.md) | active |

---

## MCP server notes

- Binary: `axon-mcp` (built from `cmd/axon-mcp/main.go`)
- Searches: all `.md` files at project root only (not subdirectories)
- Scorer: word-overlap (same as `internal/rag/naive.go`, MinScore=0.30)
- Rebuild after changes: `go build -o axon-mcp ./cmd/axon-mcp/`
- This MCP config is project-scoped (`.mcp.json`) — does not affect other projects
