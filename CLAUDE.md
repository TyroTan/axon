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
query_axon_docs("platform auth magic link JWT")
query_axon_docs("BM25 compaction RAG upgrade")
query_axon_docs("shell lazy loading product federation")
```

---

## Document Index

| File | Layer | What it governs |
|---|---|---|
| `PLATFORM_DESIGN.md` | **Workspace layer** | Multi-product structure, shell + auth, BM25 RAG upgrade, monorepo decision framework — read this first for platform questions |
| `DESIGN.md` | **How it's built + what it is** | Implemented architecture, invariants, data model, API surface, user stories, **learning theory grounding** (Bloom's extensions, calibration, dual state, prerequisite graph) |
| `plan_v2.md` | **Why and what** | Mission, product constraints ("axon is NOT"), theoretical frameworks, v2 design, known unknowns, framework vision (§12) |
| `roadmap_v2.md` | **What and when** | Epics, MoSCoW priority, sprint plan, ACs, dependency graph, open decisions; Epic F5 = platform layer |
| `plan.md` | v1 theory | v1 dimensions D1–D6, Bloom's distribution, spaced repetition schedule |
| `how_to.md` | Usage | Step-by-step: track creation, session workflow, fork/clone, naming conventions |
| `concept_taxonomy.md` | Schema | Concept map schema, bottleneck detection, bloom update rules |
| `experiments_log.md` | Log | Track registry, session log entries across all tracks |
| `CHANGELOG.md` | History | Release history |

**Where constraints live:**
- Product constraints (axon will never be X) → `plan_v2.md` §2
- Architectural invariants (code must always do X) → `DESIGN.md` §1
- Operational guardrails (Claude Code must never do X) → this file, below

**For identity and capability questions** ("what is axon", "does axon use X framework", "is this claim about axon accurate"):
→ consult `DESIGN.md` §2 (Core Concepts, including Bloom's extensions) and §8 (Theoretical Contributors coverage map).
Do not infer capability claims from the codebase alone — the learning theory grounding is documented in DESIGN.md, not derivable from Go files.

---

## Documentation sync (run after every code change)

DESIGN.md sections that describe implementation details carry `<!-- sources: file1, file2 -->` anchors. When you change a file, check whether any anchor references it and update that section if the content is now wrong.

**Mechanical checklist — before every commit:**

1. **Which files changed?** List them.
2. **CHANGELOG.md** — add an entry if the change is user-visible, architectural, or fixes a bug.
3. **DESIGN.md** — run: `grep -n "sources:.*<changed-filename>" DESIGN.md`. For each match, verify the described behavior still matches the code and update if not.
4. **If you wrote a new DESIGN.md section with implementation details** (function names, field lists, routes, step-by-step behavior): add `<!-- sources: path/to/file.go -->` immediately after the heading before committing.

**What does NOT need a doc sync check:**
- Changes to `track_*/` data files (sessions, context, concept maps) — these are learner data, not architecture
- Pure test files
- Comment-only changes

**Why this exists:** DESIGN.md has two kinds of content — stable philosophy (never goes stale) and volatile implementation facts (go stale immediately on refactor). Anchors mark the volatile sections so the staleness check is mechanical, not dependent on reasoning.

---

## Key guardrails (always apply)

- **Track data is VCS-tracked.** Sessions, contexts, conversations are committed. Never gitignore them.
- **Only `metrics.jsonl` is gitignored.** Nothing else at track level.
- **Fork ≠ Clone.** Fork = child track (track_1_2), inherits parent. Clone = new root (track_2), no parent. Route: `POST /tracks/:id/fork` vs `POST /tracks`.
- **Active session data is immutable.** If a session has evaluations or synthesis, do not mutate its files.
- **Evidence state and exploration state never mix** in scoring logic (v2 dual state model).
- **Files starting with `_` are skipped** by the question generator (e.g. `_sources.md`).
- **Concept map inheritance is downstream-only, snapshot-based.** Fork/duplicate = point-in-time snapshot. The parent is effectively frozen for that lineage once LLM activity (question generation, evaluation, synthesis) has occurred on the child. If the parent needs to evolve, fork again — do not modify the parent in place. Do not propose upstream write-back, sibling cross-reading, or continuous sync. See `DESIGN.md` §2 "Concept Map Inheritance, Snapshots, and Learning Path Simulation".
- **Each fork is a learning path divergence, each merge is convergence.** The track tree simulates parallel learning trajectories from snapshot starting points. Do not propose patterns that couple paths laterally or require shared mutable state across branches.
- **Do not introduce foreign patterns.** Proposals that require bidirectional state, lateral coupling, or automatic propagation violate the core design. Raise the idea explicitly rather than implementing it.

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

## Platform — Active Sprint (update each sprint)

> **Source of truth:** `roadmap_v2.md` (axon epics) · `PLATFORM_DESIGN.md` (platform specs)
> This section is a quick-load signal so Claude Code knows where work is focused
> without needing an MCP query first.

**Current focus: Durable Pipeline — central queue goroutine first (Sprints 17–18 = F7)**

| Item | Epic | Status | Key file |
|---|---|---|---|
| CentralQueue goroutine + channel | F7.1 | **Next** | `internal/queue/central.go` |
| PipelineStore interface + JSONL impl | F7.2 | Blocked by F7.1 | `internal/queue/store_jsonl.go` |
| Worker: retries, backoff, stall detection | F7.3 | Blocked by F7.2 | `internal/queue/worker.go` |
| Git-commit-as-atomic-boundary | F7.4 | Blocked by F7.3 | `internal/queue/git_ops.go` |
| Git worktree registry + multi-tenancy | F7.5 | Blocked by F7.4 | `internal/queue/worktree.go` |
| CoreDeps extraction | F5.1 | After F7 | `server/server.go` → `core/deps.go` |
| BM25 scorer + inverted index | F8.1 | After F5 | `core/rag/bm25/indexer.go`, `scorer.go` |

**Deferred (do not implement without explicit instruction):**
- F5 (multi-product router) — after F7 complete (Sprints 19–21)
- F8 (BM25 RAG) — after F5 complete (Sprints 22–23)
- Cross encoder re-ranker (F8.6) — add after BM25 measured in production
- PMI synonym discovery — needs real corpus to tune
- Full platform router F5.2–F5.3 — build when second product has distinct logic
- Durable pipeline F7 — Sprint 20+

---

## Active tracks

| Track | Topic | Status |
|---|---|---|
| `track_1` | ML Fundamentals · ML Math Theory · RAG Architecture · LLM Systems · Pedagogy & Communication | clean slate root — no sessions, seed concept map |
| `track_2` | MLOps: Data Engineering · Model Lifecycle · Deployment & Serving · Monitoring & Reliability · MLOps Infrastructure | cold-start root — 20 scenario concept map, no sessions |

---

## MCP server notes

- Binary: `axon-mcp` (built from `cmd/axon-mcp/main.go`)
- Searches: all `.md` files at project root only (not subdirectories)
- Scorer: word-overlap (same as `internal/rag/naive.go`, MinScore=0.30)
- Rebuild after changes: `go build -o axon-mcp ./cmd/axon-mcp/`
- This MCP config is project-scoped (`.mcp.json`) — does not affect other projects
