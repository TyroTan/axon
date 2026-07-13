# Axon — Claude Code Project Index

> Auto-loaded at session start. The substantive doctrine is **not** here — it's the always-loaded
> **directives** (`.claude/directives_cache.md`, injected by the `axon-clauding` SessionStart hook
> via `.claude/hooks/clauding.sh`; SSOT = the `curated_directives` store). This file is the lean **pointer**: how to retrieve,
> the unconditional triggers, and the few rules not yet in a directive.

---

## Doctrine lives in the directives (lean_v2)

The architecture + product contracts are **curated directives**, tagged by cluster:
- **`axon-internals`** — Shell-First · CCQ · Conduit · CVP · Isolation · Workspace-Config · Workspace-Identity · Near-Decomposability. **Always-loaded** every session via the SessionStart hook (`.claude/directives_cache.md`, ~7.8k tok).
- **`adaptive-learning`** — tracks, forks, concept maps, sessions — and other situational directives: **on-demand** (present in `curated_directives`, *not* auto-injected; reach via search/selection).

Authoritative SSOT = the `curated_directives` collection. Regenerate the always-load cache after editing directives:
`go test -tags qa_with_remote_db -run TestGenerateDirectivesCache ./qa/qa_play1/directives_load/`

---

## Retrieval — the MCP tools (look it up; don't guess)

Each `query_*` call costs ~2k–5k tok; one per response is usually enough.

| Tool | Use for |
|---|---|
| `query_axon_docs` | design, roadmap, conventions, sprint, concept-map/track questions (DESIGN.md, roadmap_v2.md, how_to.md) |
| `query_platform_docs` | platform architecture, F9 pipeline, v2 theory, identity (PLATFORM_DESIGN.md, plan_v2.md, PLY_PIPELINE.md) |
| `query_qorlib_docs` | persistence adapter, `IDatabase[T]`, file/RAM/Mongo backends, qorlib internals |
| `query_surface_docs` | outward surface: portfolio landing, ui_rct playground, magic-link demo, design-token pipeline (SURFACE.md) |
| `deep_recall` | RECONSTRUCT context when single-pass feels lossy — enforced multi-hop (BM25⊕Qdrant RRF over `axon_evolution` → Sonnet judge hops → Opus VERDICT). One call ≈ several LLM calls; use deliberately. |

**Never** call `query_ply_sessions` unless `AXON_PLY_RETRIEVAL=1` (loads hundreds of KB). See TOKEN_BUDGET.md.

### deep_recall pre-flight (orchestrator-side, unconditional)
Recall fidelity = (tier) × (freshness):
- Default to **`mode:"preview"`** — its `[R: …]` header shows the tier (`⚠️ DEGRADED RECALL` = sparse-only) and the Opus cost *before* you spend; then `mode:"synthesize", handle:…`.
- **`strict_hybrid:true`** fails loudly if qdrant/ollama are down (vs silent sparse degrade).
- **`refresh_recall`** ingests recent ply+signals → `axon_evolution` (sparse, idempotent) so recent work is visible to the BM25 leg.
- When the dense tier may be down **or** recent un-ingested sessions are in scope → **`AskUserQuestion` first**: proceed sparse? · refresh first? · auto-chain refresh→recall?

---

## Unconditional triggers (no inference, no skipping)

Distill cycle full reference: `docs/DISTILL_FIRST.md`; the cycle *contract* is the Shell-First directive (D5). Code is authoritative — distill makes drift visible.

| Keyword | Action |
|---|---|
| `distill [topic]` | full cycle: Phase 0 triage (`AskUserQuestion`: sprint-priority vs deferred) → Phase 1 (search equivalents + `capture_signal(open_decision)` with test-verdict hypothesis + `suggest_doc_update`) → "go" → Phase 2 (code+test together) → Phase 3 (go test → atomic commit → suggest_doc_update → `capture_signal(resolved_decision)`). |
| `pre distill` | Phase 1 only — opens the cycle (close with `post distill` + commit). |
| `post distill` | Phase 3 only — go test → atomic commit → suggest_doc_update → resolved_decision. |
| `re distill [topic]` | hypothesis was wrong: `capture_signal(frontier_gap)` + new `open_decision`, reopen. |
| `horizon distill [topic]` | pre-planning: `suggest_doc_update(target=planning doc)` + `frontier_gap` per divergence. |
| `discovery distill` | a *read* revealed a truth: capture_signal + write ASPIRATIONS/DESIGN + suggest_doc_update. |
| `spiral` / `spiral alert` | capture to ASPIRATIONS immediately (no deliberation) → triage (blocker→now · design→DESIGN.md · aspiration→ASPIRATIONS) → one-line verdict → resume. |

Phase-1 mandatory gates: **search for equivalents** (grep + `query_axon_docs` — don't reinvent; `docstore.SingleDocStore[T]` when `IDatabase[T]` existed is the canonical failure) and the **blast-radius grep** for any domain constant (`grep -rn '"<raw_value>"' --include="*.go"` — every hit outside `constants/` is a greppability violation).

**Hook responses (unconditional):**
- `[session-router]` (recurring concept, 4+ turns, no signal) → treat as `horizon distill [concept]`: `suggest_doc_update` + `capture_signal(frontier_gap)`.
- `[intent-check] PROBABLE_INTENT:` → do **not** auto-fire; `AskUserQuestion` to confirm the inferred phase first.

---

## Rules not (yet) in a directive

- **Explicit `bson:` tags on every MongoDB-bound struct field** — match the `json:` value exactly; `bson:",inline"` for embedded structs promoted flat. BSON defaults lowercase the Go field name silently → missing data at runtime, no error/panic. Full failure mode: `query_axon_docs("bson tags ConceptMap")`.
- **Commit atomically** after each completed unit (a package · a feature wired end-to-end · a doc spec · a qa artifact) — don't accumulate uncommitted units.
- **Write code only within `workspace.yml` `clauding_targets`** (the declared write zone for the current focus) — READ it before writing instead of inferring the target from the active product (the failure that put foundry code in `ui_rct_playground` while work happened in the clone). A target under `products/` MUST be a **leaf product** (`products/<group>/<product>`), **never a `*_group`** (`products/<group>`) — a group silently targets every product inside it, defeating isolation; `scripts/probe_clauding_mistarget.sh` now **rejects a `*_group` entry as an invariant violation** (and still tripwires off-target writes from the dirty tree). If scope legitimately grew, add the leaf-product dir to `clauding_targets` (don't silence it by writing wherever).

> Product isolation, per-product stores, `mongo.Connect`/`init.go`-off-limits, no-raw-env-vars, fork≠clone, track-data-VCS, concept-map inheritance, preset-1 FSD, the correlation-id seal, greppability/constants-first — **all now live in the always-loaded directives**, not here.

---

## Active focus

**lean_v2 — knowledge engineering: directives (clustered, tagged) + always-load hook + lean CLAUDE.md.**
SSOT for sprints: `sprints/README.md` · platform spec: `cascading_axon_core/ARCHITECTURE.md`. (Prior: `main_v5` Sprint 5 — backend_proprietary decoupling.)

---

## Retrievable doctrine (load on demand via `query_axon_docs` / `deep_recall`)

Document Index · Three Adaptive Learning Modes · Documentation sync · CQRS module doctrine · User-story invariants · Architecture quick reference · Entry points (three modes) · Active tracks · MCP server notes
