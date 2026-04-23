# Axon — Design Reference

> **Purpose of this file:** LLM-readable ground truth for the Axon project.
> Use this when answering questions like: "What features exist?", "How does X work?",
> "What is the status of Y?", "How does a user do Z?", "What is the data model?"
>
> Complements: ROADMAP.md (what's planned), CHANGELOG.md (what changed when),
> README.md (quick-start + usage), plan.md (learning system theory).

Last updated: 2026-04-21 (F6 multi-call generator, difficulty spine, path scoring)

---

## 1. What Axon is

Axon is a **local-first adaptive learning system** — a calibrated interrogator that
builds a precise, behavioral model of how one specific person thinks and knows, then
uses that model to navigate them toward real-world application of that knowledge.

**v1:** measurement — turns `.md` documents into Bloom-calibrated quiz sessions backed
by a concept prerequisite graph and spaced repetition.

**v2:** navigation — adds composite learner state detection, pre-session nudge,
dual-state model (evidence vs exploration), cognitive fingerprinting from conversation
threads, and Application Evidence Sessions (real-work debrief as transfer proxy).

Stack: Go Fiber backend · React + shadcn/ui frontend · filesystem JSON store ·
`claude` CLI auth (no API key required by default) · CQRS command/query bus.

### Architectural invariants

These rules are non-negotiable. They must hold across all features, refactors, and
new session types. If a proposed change violates one, the change is wrong — not the rule.

| Invariant | Rule |
|---|---|
| **Evidence and exploration never mix** | `bloom_current` is only updated by behavioral evidence (responses + evaluations). `exploration_unlocked` is only toggled by faith-based/aspiration logic. These two state variables never feed each other. |
| **Active session data is immutable** | Once a session has evaluations or synthesis, its files are read-only. No mutation, no re-evaluation. Start a new session. |
| **Track data is VCS-tracked** | Sessions, contexts, conversations, concept maps are all committed to git. Never gitignore them. Only `metrics.jsonl` is excluded. |
| **Context inheritance is read-time, not copy-time** | Context files are read from the ancestor chain at question-generation time. Forking does not copy context files. Child file wins on collision. |
| **Concept map inheritance is downstream-only** | Concept map state cascades parent → child at read time. No upstream write-back, no sibling cross-reading. A child track's progress never modifies a parent or sibling concept map. This is by design. |
| **Concept map propagation is explicit, not continuous** | New concept definitions only flow downstream on explicit signals: fork, duplicate, merge. There is no automatic sync. If a parent gains new concepts after a fork, descendants do not receive them until a new explicit fork/merge event. |
| **Concept map is the track's memory** | All session files (questions, responses, evaluations) are ephemeral relative to `concept_map.json`. The concept map is the authoritative learner state. |
| **`_`-prefixed files are system files** | Files beginning with `_` are never injected into question generation prompts. They are internal scaffolding (`_sources.md`, `_split_plan.md`, `_exclude`). |
| **Synthesis is applied, not auto-applied** | `Apply Synthesis` is always an explicit user action. Concept map mutations never happen automatically after evaluation. |
| **LLM calls are never in the hot path of reads** | All API GET endpoints return stored data only. LLM calls happen only on explicit generate/evaluate/synthesize/distill actions. |
| **Elaboration is post-evaluation, never real-time** | Elaboration triggers are emitted by the evaluator on the final submitted answer — never against a live textarea. This avoids the mutation problem where trigger signals invalidate as the learner edits. |

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
| 0 | `00_metadata.json` | Session metadata: creation time, shard_id, state_snapshot (difficulty spine) |
| 1 | `01_questions.json` | Generated questions (MCQ, free_text, scenario_mcq, design) |
| 2 | `02_responses.json` | User's submitted answers, confidence ratings, explanations |
| 3 | `03_evaluations.json` | LLM evaluation: correctness, Brier score, bloom_demonstrated, error taxonomy, elaboration_triggers (E11) |
| 3b | `03b_elaborations.json` | Learner responses to system-initiated elaboration prompts (E11, optional) |
| 4 | `04_synthesis.json` | Per-concept bloom advancement rules, spaced repetition schedule, delta_multiplier, learner_signal |

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

> **⚠ Live inheritance warning:** Ancestor context files are read at question-generation
> time, not copied at fork time. Editing a file in track_2/context/ affects track_2_2,
> track_2_2_2, and all other descendants immediately on their next session. To isolate
> a descendant, add a file with the same name to the descendant's own context/ — the
> child file wins on collision and the ancestor version is ignored for that track.
>
> **Excluding a specific inherited file (intentional override pattern):** In the Context
> Editor, click **Exclude** on any inherited file. This creates a same-named file in the
> descendant's own context with the sentinel content `<!-- axon:exclude inherited -->`.
> The file is intentionally empty — its only purpose is to shadow the ancestor file so
> it is not injected into sessions for this track. This is a supported, documented
> pattern and not a workaround.

### Concept Map Inheritance, Snapshots, and Learning Path Simulation

The concept map is the structured, indexed knowledge graph for each track node. It is not just a learner state file — it is the explicit skeleton that every session draws from, and it carries the full lineage of what was known when this node was created.

#### Fork/duplicate = point-in-time snapshot

When a child track is created (fork or duplicate), it receives a full copy of the parent's `concept_map.json` **at that exact moment**. This is a snapshot — not a live link. All concept definitions (indexes, names, branches, prerequisites) and their current state are frozen into the child at creation time.

**Parent freeze constraint:** Once LLM activity has occurred on a child (question generation, evaluation, or synthesis), the parent's concept structure is effectively frozen for that lineage. If the parent's knowledge structure needs to evolve, the correct action is to fork again — creating a new independent node from the updated parent. Modifying a parent's concept map after children have active sessions is not a supported workflow.

#### Live state cascade at generation time (read-only)

`GetEffectiveConceptMap` walks the full ancestor chain on every question generation and applies these rules per concept index:

| Field | Rule | Rationale |
|---|---|---|
| `bloom_current` | max(own, ancestor) | Child never regresses below what was demonstrated upstream |
| `exploration_unlocked` | OR(own, ancestor) | Faith-based unlock anywhere in the chain propagates down |
| `inquiry_precision` | max(own, ancestor) | Inquiry quality floor is inherited |
| `aspiration_count` | own only | Stretch signal is local to each track's sessions |

This is read-only — no ancestor file is ever mutated. The live cascade only applies to *state fields*, not concept *definitions*. New concept indexes added to a parent after forking do not appear in existing children.

#### What does NOT propagate — by design

- Child progress never writes back to parent or sibling. Downstream only.
- Siblings never read each other's concept maps. No lateral inheritance.
- Post-fork parent changes (new concepts, structural edits) are invisible to existing children. Fork again to get them.

#### The learning path simulation model

Each fork is a deliberate **path divergence** — a new learning trajectory starting from a known snapshot of the knowledge graph. Tracks form a tree of learning journeys, not a single linear path. This is the core use case:

- Fork at any node to simulate a new trajectory from that exact knowledge state
- Each path accumulates its own sessions, bloom progression, cognitive fingerprint, and context
- Paths are independent — the same underlying concept map can be explored from multiple angles simultaneously
- Merge is the **convergence event** — it unions concept maps from multiple paths, combining trajectories that started from the same or different ancestry

**The compound value:** each path represents LLM-pre-solved domain traversal from a different angle. A merge across two paths that covered the same concepts differently creates a richer combined map than either path alone — surfacing which concepts were bottlenecks on one trajectory but not another, which contexts unlocked exploration faster, which bloom targets were reached via different question types. This is the growth-hack pattern: compounding LLM-generated learning intelligence across deliberately chosen paths.

#### Fork workflow — implementation detail

`DuplicateTrackHandler.Handle()` runs these steps in order:

1. **Freeze step 1** — `DistillThreadsHandler.RunSilent` → `context/session_insights.snapshot.md`.
   LLM call. Idempotent: skipped if snapshot already exists.
2. **Freeze step 2** — `snapshotConversations` → `context/conversations.snapshot.md`.
   No LLM call — pure aggregation from `ConversationIndex` objects. Idempotent: explicit file
   existence check before writing.
3. **Branch** — `store.NextTrackID` → `store.GetEffectiveConceptMap` (full ancestor cascade,
   not just parent's own state) → `store.CreateTrack` with the cascaded map.
4. **`_track_origin.json`** written at child track root — machine-readable provenance.
   Records event type (`"fork"`), source track ID(s), RFC3339 timestamp, and a snapshot of
   every concept's effective `bloom_current`/`bloom_target` at copy time.
   Underscore prefix keeps it invisible to the question generator. Enables future multi-track
   synergy detection at merge time.
5. **`context/_sources.md`** written — human-readable provenance doc explaining the inheritance
   cascade for anyone inspecting the child track.
6. **`prompts/`** copied from source track.

**Critical invariant (implemented):** Step 3 uses `GetEffectiveConceptMap`, not `GetConceptMap`.
If a grandparent has `bloom_current=4` on a concept but the parent hasn't run Apply Synthesis
yet (so the parent's own file still shows `bloom_current=1`), the child correctly starts at 4 —
not 1. The child's bloom floor is the full ancestor cascade, not just its immediate parent's state.

#### `_track_origin.json` — attribution schema

Written at track root for every fork and merge. Underscore prefix: invisible to question generator.

```json
{
  "event_type": "fork" | "merge",
  "source_ids": ["track_X"],
  "created_at": "2026-04-23T10:00:00Z",
  "concept_floor": [
    { "index": 0, "name": "ML data pipeline anatomy", "bloom_current": 3, "bloom_target": 4 }
  ],
  "per_source_floors": [...]
}
```

**Field rationale:**

| Field | Type | Why this, not something else |
|---|---|---|
| `event_type` | `"fork"` \| `"merge"` | Changes interpretation of `source_ids` and `per_source_floors` |
| `source_ids` | `[]string` (track IDs) | IDs are stable keys; file paths break if axon dir moves |
| `created_at` | RFC3339 string | Absolute — remains interpretable after time passes |
| `concept_floor[].index` | int | Index in **this track's** concept map (post-remap for merges) |
| `concept_floor[].name` | string | Snapshot for human readability without loading concept_map.json |
| `concept_floor[].bloom_current` | int | Effective cascaded floor at copy time (GetEffectiveConceptMap output, not raw file value) |
| `concept_floor[].bloom_target` | int | Aspiration ceiling — captures intent, not just current state |
| `per_source_floors` | array, merge only | Pre-remap per-source bloom state; see below |

**Not captured (and why):**
- Concept definitions (description, prerequisites, unlocks) — already in `concept_map.json`; no duplication
- Branch labels — derivable from `concept_map.json`
- Session IDs / session count — captured in the snapshot `.md` files
- Concept map version hash — not versioned; timestamp + track ID is the key

**Fork vs merge — what differs:**

*Fork* (`event_type: "fork"`, one `source_id`):
- `concept_floor` indexes match the parent's concept map 1:1 (no remapping)
- `per_source_floors` is omitted — all concepts come from the single source
- `bloom_current` is the effective floor from `GetEffectiveConceptMap(parent)` — may be higher than what's in the parent's own `concept_map.json` if an ancestor had unsynced progress

*Merge* (`event_type: "merge"`, two or more `source_ids`):
- `concept_floor` indexes are the re-indexed positions in the merged map (source A concepts: 0..lenA-1, source B: lenA..lenA+lenB-1, etc.)
- `per_source_floors` is populated — one entry per source, with that source's concept floors at their **original pre-remap indexes**. This is the attribution record needed for synergy detection: given concept index N in source A and concept M in source B with the same name/branch, it is possible to compare which path reached a higher bloom level and why.

**Synergy detection use case (not yet built):**
At merge time, `per_source_floors` makes it possible to ask: "did source track A demonstrate concept X at a higher bloom level than source track B, or vice versa?" The merged map takes the effective bloom floor (max of all sources via `GetEffectiveConceptMap`), but the per-source record shows the contribution. This is the foundation for future multi-path learning intelligence: which trajectory was more effective for which concept cluster.

### Bloom's Taxonomy — Standard Use and Structural Extensions

Axon uses the **revised Bloom's Taxonomy (Anderson & Krathwohl, 2001)** as its primary cognitive scaffold, but the accurate claim is that it *structurally extends* Bloom's rather than simply applying it.

**What the standard taxonomy provides:**
Revised Bloom's is 2-dimensional, not 1-dimensional:

| | Factual | Conceptual | Procedural | Metacognitive |
|---|---|---|---|---|
| Remember | | | | |
| Understand | | | | |
| Apply | | | | |
| Analyze | | | | |
| Evaluate | | | | |
| Create | | | | |

Most systems that claim to "use Bloom's" implement only the left column of the Cognitive Process dimension (1–6). Axon explicitly targets the full second axis:
- **Procedural knowledge** — `teach_back` (construct explanation from scratch), `teach_forward` (adapt explanation to audience's known gaps)
- **Metacognitive knowledge** — `unknown_edge` (describe where you don't know what to do), `decision_audit` (reflect on a real past decision), `inquiry_precision` score in E9 (measuring how well the learner asks questions, not just answers them)

**"Higher order" — partially correct, not the full claim:**
Higher-order thinking (HOT) in educational research refers specifically to Bloom's L4–L6 (Analyze, Evaluate, Create). Axon does bias toward these levels via `bloom_target` ceilings and the difficulty spine — so the HOT framing is accurate but understates what's happening. The stronger and more precise claim is structural extension.

**Three structural extensions that Bloom's has no concept of:**

| Extension | What Bloom's says | What Axon adds |
|---|---|---|
| **Prerequisite graph** | No concept of concept-level dependency | `prerequisite_indexes` gates access; mastery of one concept unlocks another |
| **Dual state** | Single competence level per concept | `bloom_current` (evidence state, behaviorally proven) vs `exploration_unlocked` (faith-based, aspiration-driven) — Bloom's encodes neither |
| **Calibration overlay** | Nothing about confidence accuracy | Brier score on every response; metacognitive gap between stated confidence and behavioral evidence |

Spaced repetition scheduling is a fourth layer orthogonal to Bloom's — Bloom's is a taxonomy of cognitive depth, not a retention schedule.

**Research grounding for the extensions:**
- Prerequisite graph: Carroll's mastery learning model (1963) + concept-graph approaches in intelligent tutoring systems
- Dual state: Winne & Hadwin COPES model (1998); productive failure (Kapur, 2010)
- Calibration: Brier (1950); Dunning-Kruger (1999); metacognition as defined by Flavell (1979)
- Procedural/metacognitive knowledge dimension: Anderson & Krathwohl (2001) — the same revision that added these axes to Bloom's original 1956 taxonomy

For conversations and thread tutoring, context files are **chunked** by markdown headings
and ranked by keyword overlap (`rag/naive.go`). Chunks scoring below `MinScore = 0.30`
are discarded before injection. `rag.DistinctTopN` is also used post-generation to
deduplicate questions across multi-call batches (MMR-style greedy selection).

### Difficulty System

Sessions are generated at an **effective difficulty level** computed from two sides:

| Side | Source | Range |
|---|---|---|
| Operator config | `AXON_LEVEL_OVERRIDE` env var | `recall(−3)` → `extreme(+5)` |
| Learner signal | min of TrackState signal + composite_state synthesis signal | −3 → +5 |

**Effective score** = simple average of both sides, rounded half-up, clamped to [−3, +5].
**Named levels** (numeric spine): `recall(−3)`, `easy(−2)`, `default(0)`, `medium(+1)`,
`challenge(+2)`, `intense(+3)`, `extreme(+5)`.

Each level controls: `BloomDelta` (relative to bloom_current), `DifficultyFloor`,
`CrossBranchWeight`, `FormatBias`, `TimeMultiplier`.

**StateSnapshot** is frozen into `00_metadata.json` at question-generation time and never
mutated — it records the axon_config_score, learner_signal, effective_score, and level_name
that were active when the session was created. Used by `apply_synthesis` to compute the
delta multiplier for bloom gain scaling.

**Delta multiplier** — `generationEffective − currentEffective` at synthesis time:

| Delta | Multiplier | Meaning |
|---|---|---|
| ≥ 3 | 1.5× | Session was much harder than current state |
| 2 | 1.3× | |
| 1 | 1.15× | Slight extra reward |
| 0 | 1.0× | No adjustment |
| −1 | 0.9× | Session easier than current state |
| ≤ −2 | 0.8× | |

**Consecutive fail counter** — `track_state.json` tracks `consecutive_fails` and
`consecutive_fails_easy_delta`. When `delta < 0` (easier session) and the learner fails
(avg_correctness < 0.5), SR reset is amplified. Two consecutive nudged failures feed back
into the learner signal, reducing effective score in subsequent sessions automatically.

**Learner path** — every generation event is appended to `learner_path.jsonl` (global,
append-only) with the three-way score record. Used for auditing and future session planning.

---

## 3. User stories — implemented

### US-01: Study a topic I understand using my own documents
1. Track exists with context files loaded
2. Open track page → **+ Start Session**
3. **Generate Questions** → streams N questions (default 8, `AXON_QUESTION_COUNT`), Bloom-calibrated at the effective difficulty level (operator config × learner signal)
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
- Open the parent track (e.g. `track_1`) → click **Fork**
- Child track ID is assigned automatically: `track_1` → `track_1_2` → `track_1_2_2`
- Child inherits parent's `concept_map.json` (with `bloom_current` preserved) and all `prompts/`
- Child's `context/` starts with only `_sources.md`; add new `.md` files via Edit Context
- Context is inherited at question-generation time: child files + parent files (child wins on collision)

**Status: ✅ fully implemented** (Fork button on TrackPage)

### US-04: Specialize a track for a specific job or context
1. Open parent track → **Fork** → navigates to child track
2. **Edit Context** → create a new `.md` file with job/interview-specific content
3. Start a session — child context is merged with inherited parent context

**Status: ✅ fully implemented (Fork + Edit Context)**

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

### US-16: Distill tutoring threads into a reusable context snapshot
Quiz tutoring conversations (per-question `threads/*.json`) accumulate rich learning signal
that would otherwise stay buried in session directories. **Distill Threads** extracts this
signal into a context file that future sessions — and forked/cloned tracks — can use.

1. Track page → **Distill Threads** button (visible when at least one session exists)
2. Backend scans all sessions for threads with at least one learner follow-up turn
3. LLM generates a structured learning signal document with eight sections:
   - **Reasoning Patterns** — how the learner approaches problems
   - **Misconception Fingerprint** — specific wrong beliefs surfaced, whether resolved
   - **Distractor Affinities** — MCQ wrong-answer patterns (informs future distractor generation)
   - **Concepts Needing Reinforcement** — persistent confusion areas
   - **Calibration Notes** — confidence vs correctness patterns
   - **Curiosity Clusters** — concepts learner returned to beyond eval requirements; signals top-down engagement candidates
   - **Mental Models That Clicked** — framings/analogies that visibly unlocked understanding mid-thread
   - **Mental Models That Failed** — framings requiring re-explanation; question generator avoids these
4. Output written to `context/session_insights.snapshot.md`
5. File propagates automatically: inherited via Fork cascade, physically copied on Clone

The question generator already reads `context/*.md` files verbatim and has an existing
instruction (0.21.0) to adapt framing when it finds `## Reasoning Patterns` /
`## Misconception Fingerprint` / `## Distractor Affinities` sections.

**Status: ✅ fully implemented** (`POST /api/tracks/:id/distill-threads`, TrackPage button)

### US-17: Import a job description and get interview-ready context instantly
After cloning or forking a track for interview prep:

1. Track context editor → **Import Job Description** (collapsible panel)
2. Optional role label (e.g. `ragflow_expert`) + paste raw JD text
3. Click **Import & Analyze** — LLM streams structured analysis
4. Output written to `{role}.job.md` in `context/` with six sections:
   - **Role Signal** — what the client actually wants beyond the stated requirements
   - **Must-Have Skills** — with role-specific rationale per skill
   - **Likely Interview Probes** — specific questions a technical screen would ask
   - **Interview Scenario Seeds** — L4/L5 scenario questions grounded in the JD's deliverables
   - **Self-Assessment Anchors** — what to close before interviewing
   - **Question Format Guidance** — instructions for the quiz generator (types, distractors, Bloom floor)
5. File is immediately picked up by question generation; inherited via Fork, copied on Clone

**Status: ✅ fully implemented** (`POST /api/tracks/:id/context/import-job`, ContextEditorPage panel)

---

## 4. User stories — planned / deferred

### US-12: Use a conversation to improve future quiz calibration (Cognitive Fingerprint)
A free-form conversation reveals how the learner *thinks*, not just what they know.
Running `AnalyzeConversationCommand` extracts a cognitive fingerprint — reasoning style,
misconception framings, curiosity clusters — into a `conversation_analysis.snapshot.md`
file written to the target track's `context/`. The question generator already has an
instruction to use this file when present (0.21.0).

Note: **quiz thread distillation** (US-16) is the implemented counterpart for session
threads. US-12 is specifically for free-form `conversations/*.json` records.

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

**Child tracks** (`track_N_M`): specialization of a parent, created via **Fork** button on
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
    track_state.json          consecutive_fails counter; feeds learner signal
    context/
      _sources.md             human-only; not sent to LLM
      _split_plan.md          auto-generated when corpus > soft limit
      *.md / *.snapshot.md    context injected into questions
    sessions/
      session_NNN/
        00_metadata.json      shard_id (if large corpus), state_snapshot (difficulty spine)
        01_questions.json     generated questions (post-dedup, deterministically shuffled)
        02_responses.json     user answers
        03_evaluations.json   LLM evaluations
        04_synthesis.json     bloom update proposals, learner_signal, delta_multiplier
        threads/
          {question_id}.json  per-question tutoring thread
    prompts/                  manual bootstrap prompts (human use only)
    README.md                 track-level description
  conversations/
    {uuid}.json               ConversationMessage[] with per-message meta
    {uuid}.index.json         ConversationIndex (summary, topics, per-ply breakdown)
  learner_path.jsonl          global append-only path log: one entry per generation event
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
`Stream` used in: question generation (multiple calls per session), evaluation, synthesis,
meta-synthesis, compaction.

### Question generation pipeline (F6)

A single session generation runs multiple sequential LLM calls:
1. **Concept map call** — `BuildSystemPrompt` + `BuildUserPrompt`, requests `target + 25%` questions.
2. **Job post calls** — one call per `*.job.md` in context, using `BuildJobPostSystemPrompt` +
   `BuildJobPostUserPrompt`. Each call is fully job-framed; target scales with session size (3/5/8).
3. **Consolidate** — `deduplicateQuestions`: stage 1 exact `(concept_indexes, bloom_level)` match,
   stage 2 `rag.DistinctTopN` MMR text similarity → trim to `AXON_QUESTION_COUNT`.
4. **Shuffle** — FNV-64a hash of `generationID` → deterministic interleaving of all sources.
5. **Renumber** → write `01_questions.json`.

`AXON_QUESTION_COUNT` (default 8) is the final target count. The pipeline over-generates
by design so the deduplicator has headroom to prefer quality over quantity.

---

## 9. API surface (selected)

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/tracks` | List all tracks |
| POST | `/api/tracks` | Create new root track. Body: `{ branches: string[] }`. Returns `new_track_id` |
| GET | `/api/tracks/:id` | Track detail + concept map |
| POST | `/api/tracks/:id/fork` | Fork — create child track (`track_N` → `track_N_2`) |
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
| POST | `/api/tracks/:id/distill-threads` | SSE — distill tutoring threads → `context/session_insights.snapshot.md` |
| POST | `/api/tracks/:id/context/import-job` | SSE — analyze raw JD → `context/{role}.job.md` |
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
| GET | `/api/config/levels` | Full difficulty level matrix with scores, active level name |
| GET | `/api/learner-path` | Full learner_path.jsonl as JSON array |
| GET | `/api/tracks/:id/difficulty-preview` | Deterministic stat check: effective level given current track state |
| GET | `/api/metrics` | Operational metrics |

Full interactive explorer: `/monitoring` → API Explorer.
