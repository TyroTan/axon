# Axon v2 — Product Roadmap (PM View)

> **Format:** MoSCoW priority · T-shirt size · Sprint assignment · Dependency graph
> **Sprint cadence:** 2 weeks
> **v1 reference:** `plan.md`, `how_to.md`
> **v2 design reference:** `plan_v2.md`

---

## Epic Map

| Epic | ID | Description | Sprints |
|---|---|---|---|
| **Cognitive Fingerprint** | E1 | Conversation analyzer — fourth evidence source | 1–2 |
| **Dual State Architecture** | E2 | Evidence vs exploration state split | 3 |
| **Learner State Detection** | E3 | Composite state detection + bidirectional matrix | 4–5 |
| **Mode Config + Nudge** | E4 | Pre-session nudge, dial system, persona config | 6 |
| **Faith-Based Unlocking** | E5 | Reach questions, provisional unlock, aspiration gap | 7 |
| **Regression Intelligence** | E6 | Delta classifier, frustration turn, situation log | 8 |
| **Generative Formats** | E7 | Teach-back, teach-forward, contradiction, unknown-edge | 9–10 |
| **Self-Reflection Loop** | E8 | Post-session reflection, concept self-discovery, mode evolution | 11 |
| **Inquiry Quality** | E9 | Measure how well learner asks questions, not just answers them | 12 |
| **Application Evidence Sessions** | E10 | New session type: real-work debrief, AI interrogator, transfer function proxy | 13–14 |
| **System-Initiated Elaboration** | E11 | Evaluator emits elaboration triggers; consolidated elaboration page closes E9 loop | 15 |
| **Concept Map Tending** | F4 | Dormancy scoring prevents concepts from being silently skipped across many sessions | 16 |
| **Scalable Multi-Call Generation** | F6 | Multi-call pipeline: concept map + per-job-post calls, question-level MMR dedup, AXON_QUESTION_COUNT | 8d |

---

## Dependency Graph

```
E1 (Conversation Analyzer)
  └── E3 (Composite State Detection)
        └── E4 (Mode Config + Nudge)
              ├── E5 partial (dial config)
              └── E7 partial (format selection)
        └── E6 (Regression Intelligence)

E2 (Dual State Architecture)
  └── E4 (Mode Config + Nudge)
  └── E5 (Faith-Based Unlocking)

E5 requires E2 + E4
E6 requires E3
E7 requires E1 + E4
E8 requires E1 + E2 (reads all accumulated session data)
E9 requires E1 (extends distill-threads; feeds E8 reflection input)
E10 requires E2 + E9 (needs dual state schema + inquiry quality signal)
E11 requires E9 full (elaboration_triggers feed inquiry_precision; evaluator must already emit full E9 output)

F2 (Live Concept Map Inheritance) — pre-condition for E10 trigger logic to be meaningful
F3 (Pre-Merge Idempotent Distill) — consistency fix, no dependencies
```

**Critical path:** `F2 → F3 → E2 → E9 → E10 → E8`
**Self-evolution path:** `E1 → E9 → E8 → E11` (E11 closes the system-initiated inquiry loop)
**Transfer proxy path:** `F2 → E2 → E9 → E10` (first real-world application signal)

---

## Must Have

### E1 — Cognitive Fingerprint (Conversation Analyzer)

**Why first:** blocks E3, E5, E7. All downstream intelligence depends on this signal.

---

**M1.1 — Extend distill_threads output to full v2 schema** ✅ Done — Sprint 1
> As a system, I want tutoring thread distillation to output all v2 cognitive fingerprint
> sections so that question generation has the full learner reasoning signal available.

- Pre-existing: `distill_threads.go` → `context/session_insights.snapshot.md`
- Pre-existing sections: Reasoning Patterns, Misconception Fingerprint, Distractor Affinities,
  Concepts Needing Reinforcement, Calibration Notes
- Added in v2: Curiosity Clusters, Mental Models That Clicked, Mental Models That Failed
- Size: **S** (pre-existing implementation reduced scope from M)
- Priority: **Must**

AC:
- [x] `distill_threads.go:buildDistillSystem()` includes all v2 sections
- [x] Output file: `context/session_insights.snapshot.md` (existing path)
- [x] UI: Distill Threads button on TrackPage (pre-existing)

---

**M1.2 — Question generator ingests full v2 fingerprint** ✅ Done — Sprint 1
> As a system, I want question generation to use all v2 fingerprint sections to adapt
> framing, distractors, and concept weighting.

- Pre-existing: `generate_questions.go` already read Reasoning Patterns, Misconception
  Fingerprint, Distractor Affinities
- Added in v2: Curiosity Clusters → concept weight nudge; Mental Models That Clicked →
  use those framings; Mental Models That Failed → avoid those framings
- Size: **XS**
- Priority: **Must**

AC:
- [x] System prompt instruction updated in `generate_questions.go`
- [x] All v2 sections referenced in the generator instruction

---

**M1.3 — Distill workflow documented in how_to.md** ✅ Done — Sprint 1
> As a learner, I want a clear step for when to run Distill Threads so the signal is
> incorporated before generating questions for the next session or fork.

- Size: **XS**
- Priority: **Must**

AC:
- [x] Step added to `how_to.md` after Step 7 (session synthesis)

---

### E2 — Dual State Architecture ✅ Done

**M2.1 — Split concept_map.json into evidence + exploration fields** ✅ Done

AC:
- [x] Schema change backward-compatible with v1 concept maps (new fields optional, default false/0)
- [x] Question generator reads `exploration_unlocked` for candidate pool selection
- [x] Profile scoring reads `bloom_current` only — never `exploration_unlocked`
- [x] `concept_taxonomy.md` updated with new fields

---

**M2.2 — Provisional unlock logic** ✅ Done

AC:
- [x] aspiration_count increments when learner engages above evidence floor (question bloom_level > concept bloom_current at apply time)
- [x] Provisional unlock threshold: `aspirationThreshold = 2` constant in `apply_synthesis.go`
- [x] `exploration_unlocked` auto-set in `ApplySynthesisHandler` when threshold reached
- [x] Scoring weight communicated to LLM via `difficulty_estimate = 0.5×` rule in system prompt

---

### E3 — Learner State Detection

**M3.1 — Composite state inference engine**
> As a system, I want to infer which of the 8 composite learner states the learner is
> currently in from observable signals, so that the pre-session nudge and question selection
> are state-appropriate.

- Input signals: session frequency, confidence rating pattern (last 3 sessions),
  score trajectory, explanation word count trend, thread open rate
- Output: primary composite state + confidence level
- Size: **L**
- Priority: **Must**
- Sprint: **4**
- Blocked by: E1 complete

AC:
- [ ] 8 states defined with detection heuristics in code (not just docs)
- [ ] State written to `04_synthesis.json` as `composite_state_detected`
- [ ] State confidence score included (0.0–1.0)
- [ ] Fallback: if insufficient signal → state = "unknown" → nudge is skipped, not guessed

---

**M3.2 — Bidirectional matrix: learner trust proxy**
> As a system, I want to infer the learner's perceived trust and relevance state from
> behavioral signals so that we know whether any intervention will land before applying it.

- Proxy signals: pre-session nudge override rate, thread engagement rate,
  explanation length vs. question difficulty (disengagement = short explanations on hard questions)
- Output: `perceived_trust_proxy` field in synthesis
- Size: **M**
- Priority: **Must**
- Sprint: **5**
- Blocked by: M3.1

AC:
- [ ] Trust proxy written to synthesis alongside composite_state
- [ ] If trust_proxy < 0.4 → nudge tone shifts to lower-pressure framing regardless of state
- [ ] If trust_proxy < 0.2 → session opens with consolidation turn only

---

### E4 — Mode Config + Pre-Session Nudge

**M4.1 — Pre-session nudge generation**
> As a learner, I want axon to surface a one-sentence behavioral suggestion before each
> session based on my composite state, so that I can confirm or override the direction
> before questions begin.

- Output: `nudge_suggestion` in synthesis, surfaced before session start in UI
- Format: state name + suggested direction + override option
- Size: **M**
- Priority: **Must**
- Sprint: **6**
- Blocked by: M3.1, M3.2

AC:
- [ ] Nudge generated per composite state (8 state-specific templates)
- [ ] Override recorded as `nudge_override: bool` in session metadata
- [ ] Consistent override pattern (3+ sessions) flags aspiration gap or trust issue
- [ ] Nudge skipped gracefully when state = "unknown"

---

**M4.2 — Multi-axis dial config**
> As a learner, I want to manually adjust pacing, framing, pressure, and abstraction
> dials before a session, so that I can shift the mode beyond what axon suggests when
> I have context axon can't infer.

- 4 dials, each -2 to +2 relative to axon's suggested position
- Dial positions inherited by forked tracks (overridable)
- Size: **M**
- Priority: **Must**
- Sprint: **6**
- Blocked by: M4.1

AC:
- [ ] Dial positions stored in session metadata
- [ ] Fork inherits parent dial positions
- [ ] Dial override written to synthesis for downstream session planning
- [ ] UI: pre-session config screen with dial controls + nudge text + confirm/override

---

## Should Have

### E5 — Faith-Based Unlocking

**S1.1 — Reach question format**
> As a system, I want to inject one question per session 2 Bloom's levels above the
> learner's demonstrated level in a concept they've shown interest in, scored as stretch
> signal not gap signal.

- New field on evaluation: `signal_type: "stretch" | "gap" | "aspiration" | "regression"`
- Size: **S**
- Priority: **Should**
- Sprint: **7**
- Blocked by: M1.2, M2.1

AC:
- [ ] Reach question format added to question generator
- [ ] Reach questions excluded from gap-flagging logic
- [ ] Reach question failure does not depress bloom_current
- [ ] Reach question success provisionally increments aspiration_count

---

**S1.2 — Aspiration gap tracking**
> As a system, I want to track the delta between concepts a learner gravitates toward
> and what their evidence state supports, so that top-down learning orientation is
> detected and handled differently from knowledge gaps.

- New profile field: `aspiration_gap` per concept (float, 0.0–1.0)
- Size: **S**
- Priority: **Should**
- Sprint: **7**
- Blocked by: M2.2, S1.1

AC:
- [ ] aspiration_gap computed from (exploration engagement - evidence level) per concept
- [ ] High aspiration_gap in high-frequency learner → triggers faith-based unlock
- [ ] High aspiration_gap in low-frequency learner → triggers consolidation suggestion

---

### E6 — Regression Intelligence

**S2.1 — Delta-from-last-correct classifier**
> As a system, I want to classify concept regressions (correct in session N, wrong in N+2)
> into: shallow original learning, interference from new concepts, or decay from disuse,
> so that the intervention matches the actual cause.

- Size: **M**
- Priority: **Should**
- Sprint: **8**
- Blocked by: M3.1

AC:
- [ ] Regression detection in synthesis (cross-session concept comparison)
- [ ] Classification heuristics: timing gap → decay; new adjacent concept learned → interference; always weak explanation → shallow
- [ ] Classification written to synthesis as `regression_type`
- [ ] Different question prescription per classification type

---

**S2.2 — Frustration-indexed consolidation turn**
> As a system, I want to detect when error rate spikes AND confidence drops in the same
> session and respond by opening the next session with 3 consolidation questions on
> mastered concepts before re-attacking the gap.

- Size: **S**
- Priority: **Should**
- Sprint: **8**
- Blocked by: M3.1

AC:
- [ ] Frustration signal: error rate > 60% AND avg confidence delta < -1.0 in same session
- [ ] Frustration flag written to synthesis
- [ ] Next session question generator reads flag and opens with consolidation sequence

---

**S2.3 — Situation log field**
> As a learner, I want an optional one-line field per session to record a real context
> where a concept was relevant this week, so that the system can build a practitioner
> exposure graph over time.

- Size: **XS**
- Priority: **Should**
- Sprint: **8**

AC:
- [ ] Optional `situation_log` field in session response format
- [ ] Situation log entries accumulated in synthesis under `practitioner_exposure[]`
- [ ] Concept indexes tagged per entry (manual or inferred by LLM)

---

## Could Have

### E7 — Generative Question Formats

**C1.1 — Teach-back format**
> As a system, I want a teach_back question format that asks the learner to explain a
> concept from scratch, evaluated on completeness, analogy quality, and edge-case awareness.

- Size: **M**
- Priority: **Could**
- Sprint: **9**
- Blocked by: M1.2, M4.1

---

**C1.2 — Teach-forward variant**
> As a system, I want a teach_forward format that specifies a target audience with known
> prior knowledge X but not Y, so that multiple framings of the same concept are measurable.

- Size: **S**
- Priority: **Could**
- Sprint: **9**
- Blocked by: C1.1

---

**C1.3 — Contradiction tolerance question**
> As a system, I want contradiction questions that require holding two valid but contextually
> opposing truths simultaneously, testing judgment rather than recall.

- Size: **S**
- Priority: **Could**
- Sprint: **10**
- Blocked by: S2.3

---

**C1.4 — Unknown-edge probe**
> As a system, I want one unknown-edge probe per track that asks the learner to describe
> a scenario where they would not know what to do, so that the self-reported gap boundary
> can be compared to the system's behavioral evidence boundary.

- Size: **S**
- Priority: **Could**
- Sprint: **10**
- Blocked by: M4.1

---

---

## Could Have (continued)

### E8 — Self-Reflection Loop

**C2.1 — ReflectCommand: post-session track-level reflection**
> As a system, I want to trigger a reflection pass after every synthesis is applied,
> so that accumulated session data is meta-analyzed and the system can surface candidate
> concepts, mode recommendations, and emerging criteria it wasn't originally designed to track.

- Trigger: end of `ApplySynthesisCommand.Handle()` (or manual `POST /tracks/:id/reflect`)
- Reads: all distill-threads snapshots + all syntheses + current concept map
- Writes: `_reflection.json` at track root (track-level artifact, not session-scoped)
- Size: **M**
- Priority: **Could**
- Sprint: **11**
- Blocked by: E1 complete, E2 complete

Output schema:
```json
{
  "candidate_concepts": [{ "name", "branch", "description", "justification", "sessions_referenced" }],
  "recommended_mode": { "direction", "rationale" },
  "emerging_criteria": [{ "pattern", "occurrence_count", "observable_proxy" }],
  "inquiry_patterns": { "best_questions", "missed_pivots" }
}
```

AC:
- [ ] `ReflectCommand` and handler in `internal/commands/`
- [ ] Reads all `session_insights.snapshot.md` files for the track
- [ ] Writes `_reflection.json` to track root (prefixed `_` so question generator skips it)
- [ ] `POST /tracks/:id/reflect` endpoint wired in server
- [ ] UI: "Reflect" button on TrackPage, output readable in context editor

---

**C2.2 — Concept auto-promotion from reflection**
> As a system, I want candidate concepts that recur across 3+ consecutive reflections
> to be auto-promoted to the concept map (with user confirmation), so that the ontology
> evolves with the learner's actual engagement rather than being fixed at track creation.

- Threshold: configurable (default: 3 consecutive reflections)
- Approval flow: `POST /tracks/:id/reflect/apply` presents candidates, user confirms
- Auto-approve path: concepts meeting threshold written directly via `WriteConceptMap`
- Size: **S**
- Priority: **Could**
- Sprint: **11**
- Blocked by: C2.1

AC:
- [ ] Recurrence tracking across `_reflection.json` history
- [ ] Candidate concept added to concept map with new index, branch, bloom_current=1
- [ ] `plan_v2.md` updated: concept map is now a living ontology, not fixed at creation

---

**C2.3 — Reflection feeds forward into next session**
> As a system, I want the question generator to read `_reflection.json` at session start
> and apply the recommended mode and emerging criteria to that session's question distribution,
> so that self-reflection has a measurable downstream effect.

- Same pattern as reading `BOTTLENECK` / `EXPLORATION_UNLOCKED` flags — one additional read
- Mode recommendation maps to `SteerIntent` direction enum (existing)
- Size: **S**
- Priority: **Could**
- Sprint: **11**
- Blocked by: C2.1, M4.1

AC:
- [ ] Question generator reads `_reflection.json` if present
- [ ] `recommended_mode` applied as soft weight (not overriding user dial config)
- [ ] Reflection-sourced mode logged in session metadata for auditability

---

### E9 — Inquiry Quality Measurement

**C3.1 — Inquiry Patterns section in distill-threads**
> As a system, I want distill-threads to extract the quality of questions the learner
> asked in tutoring threads, not just the quality of their answers, so that inquiry
> precision becomes a tracked cognitive signal.

- New section in `session_insights.snapshot.md`: **Inquiry Patterns**
- Dimensions: Bloom level of learner's questions, constraint inclusion, specificity,
  error localization accuracy, thread arc (narrowing vs scattering)
- Size: **S**
- Priority: **Could**
- Sprint: **12**
- Blocked by: M1.1

AC:
- [x] `buildDistillSystem()` includes Inquiry Patterns section
- [x] Section outputs: high-precision questions verbatim, low-precision questions with
  suggested sharper version, missed pivots, thread arc classification
- [x] Question generator ingests Inquiry Patterns (mechanism-forcing rule added to system prompt)

---

**C3.2 — Counterfactual question scaffold in thread feedback**
> As a learner, I want thread feedback to include "had you asked X about Y, you would
> have arrived at the actual insight" when my question missed the mechanism, so that
> inquiry quality is something I'm explicitly trained on, not just rewarded for.

- New field on thread evaluator output: `counterfactual_question` (nullable string)
- Only populated when learner's question targeted symptom rather than mechanism
- Structural parallel to `distractor_explanations` on MCQ — same pattern, applied to questions
- Size: **S**
- Priority: **Could**
- Sprint: **12**
- Blocked by: C3.1

AC:
- [ ] Thread evaluator LLM detects symptom-vs-mechanism targeting in learner questions
- [ ] `counterfactual_question` field surfaced in thread response (UI: shown below answer)
- [ ] High counterfactual rate per concept → concept flagged in Inquiry Patterns snapshot

---

**C3.3 — Inquiry quality as tracked concept-level signal**
> As a system, I want `inquiry_precision` tracked per concept alongside `bloom_current`,
> so that a learner who answers well but questions poorly is distinguishable from one
> who does both.

- New concept field: `inquiry_precision` (float 0.0–1.0, session-averaged)
- Computed from Inquiry Patterns section across sessions
- Feeds into composite state detection (a high-answer/low-inquiry learner is a distinct state)
- Size: **M**
- Priority: **Could**
- Sprint: **12**
- Blocked by: C3.1, C3.2

AC:
- [x] `inquiry_precision` added to Concept struct (omitempty, default 0)
- [ ] ApplySynthesis reads Inquiry Patterns section and updates per concept
- [ ] Composite state detector reads `inquiry_precision` as additional input signal
- [x] `concept_taxonomy.md` updated with field definition

---

---

## Infrastructure

### F1 — Smart Fork (freeze-and-branch) ✅ Done

**F1.1 — Auto-distill threads on fork**
> As a system, I want fork to automatically run Distill Threads before branching
> if no session_insights.snapshot.md exists, so the child inherits the full session
> wisdom without requiring a manual step.

- Idempotent: skipped if snapshot already exists (re-fork is instant)
- Non-fatal: if no threads exist, fork proceeds without snapshot
- Size: **S** · Priority: **Must** · Done

AC:
- [x] `DistillThreadsHandler.RunSilent` — non-streaming, idempotent
- [x] `DuplicateTrackHandler` calls `RunSilent` before branching

---

**F1.2 — Auto-snapshot conversation indexes on fork**
> As a system, I want fork to aggregate all ConversationIndex objects referencing
> the source track into conversations.snapshot.md so the child inherits conversation
> context without an LLM call.

- Idempotent: skipped if snapshot already exists
- No LLM call — pure aggregation of existing ConversationIndex files
- Conversations without an index are listed as stubs (run Index to enrich)
- Size: **S** · Priority: **Must** · Done

AC:
- [x] `snapshotConversations` in `DuplicateTrackHandler` — filters by track_id, merges indexes
- [x] Writes `context/conversations.snapshot.md` with summary, topics, key decisions, open questions per conversation

---

---

### F2 — Live Concept Map Inheritance ✅ Done

> As a learner working in a forked track, I want my sessions to start from my parent track's
> current bloom_current levels as a floor, so that progress made in the parent after I forked
> is not lost to me.

**Why:** concept_map.json is currently a frozen snapshot at fork time. Parent progress after
fork never reaches the child. This mirrors the context inheritance gap that was closed by
`GetTrackContext` walking the ancestor chain — the same pattern should apply to bloom stats.

**Mechanism:** at question-generation time, walk the ancestor chain (same as `parentTrackID`)
and for each concept_index, take `max(own.bloom_current, ancestor.bloom_current)` as the
effective floor. Child's own synthesis updates can only raise, never lower. No write-back to
parent — purely read-time aggregation.

- Size: **M** · Priority: **Must** · Status: **Done**

Decisions made:
- Merge happens in `store.GetEffectiveConceptMap()` called from question generator — not in prompt
- Floor applies to: `bloom_current` (max), `exploration_unlocked` (OR), `inquiry_precision` (max); SR schedule stays own-only
- UI visibility: not yet — deferred

AC:
- [x] Child sessions use `max(own, ancestor)` bloom floor for question targeting
- [x] Ancestor chain walked at generation time, not cached
- [x] No mutation of ancestor concept_map.json — read-only
- [x] Works transitively: track_2_2_1 inherits from track_2_2 inherits from track_2

---

### F3 — Pre-Merge Idempotent Distill ✅ Done

> As a system, I want merge to automatically run Distill Threads on each source track
> before merging their concept maps, so the merged track inherits distilled session wisdom
> from all sources rather than raw state.

**Why:** Smart Fork (F1) added idempotent pre-step distilling before branch. Merge currently
has no equivalent — it copies concept maps but discards session thread knowledge from source
tracks. Inconsistency that grows worse as more tracks accumulate sessions.

- Size: **S** · Priority: **Should** · Status: **Done**

AC:
- [ ] `MergeTracksHandler` calls `DistillThreadsHandler.RunSilent` on each source track before merging
- [ ] Idempotent: skipped per source track if `session_insights.snapshot.md` already exists
- [ ] Non-fatal: if distill fails for a source, merge proceeds without it (warning logged)

---

### E10 — Application Evidence Sessions

> As a learner, I want a session type where I narrate real work I am doing — a project,
> a task, a problem solved at work — and an AI interrogator extracts application-level
> evidence from my account, so that axon can measure transfer, not just recall.

**Why this is the transfer function proxy:** current sessions measure mastery-in-context
(MCQ, scenario, free-text within axon's framing). Application Evidence Sessions measure
whether the learner can apply concepts in their actual work — the signal that matters most
for real-world success and the one most absent from axon today.

**The LLM-assistance dimension:** in the modern engineering context, learners use AI
assistance heavily (often 80–100% of execution). This does not invalidate the signal —
it shifts what is being measured. The interrogator branches on `llm_assistance_mode`:

| Mode | LLM assistance | Interrogator probes | Bloom dimension |
|---|---|---|---|
| `autonomous` | ≤20% | Reasoning process, problem decomposition, solution construction | L3 Apply, L4 Analyze |
| `directed` | ≥80% | How they framed prompts, what they rejected, how they caught errors | L5 Evaluate, L6 Create |
| `mixed` | 20–80% | Both dimensions, weighted by self-report | L3–L6 |

A learner at 90% AI assistance who can articulate *why* they rejected the AI's first approach
is demonstrating higher bloom than a learner at 0% who can only describe what they built.
The ratio is a mode selector, not a penalty modifier.

**Session lifecycle:**
```
1. Task Definition
   Learner: "I am working on X" (freeform)
   LLM co-structures into: task_title, concept_indexes_relevant,
   llm_assistance_mode, expected_bloom_ceiling, success_criteria

2. Standup Loop (N rounds until satisfied)
   AI: probing question targeting a concept or decision point
   Learner: narrative response
   AI: scores evidence_so_far, decides to probe deeper or close
   → repeat until evidence_sufficient OR max_rounds reached

3. Evidence Extraction
   LLM reads full transcript → outputs per concept_index:
   bloom_level_demonstrated, evidence_quote, orchestration_quality_score (directed mode),
   gaps_identified, synthesis_notes

4. Synthesis Integration
   Same pipeline as quiz sessions: updates bloom_current
   Rule: AES can raise bloom to L6 but cannot lower it
   Attribution tag: bloom evidence tagged as 'application' source
```

- Size: **L** · Priority: **Must** · Status: **Spec only — schema decisions needed first**

Open decisions (pre-conditions before building):
- [ ] Session granularity: one AES = one project lifetime (multi-standup) vs. one AES = one exchange
- [ ] Task origin: always learner-defined vs. axon suggests tasks from concept gaps
- [ ] Termination signal: LLM decides "satisfied" vs. fixed round count
- [ ] Bloom write ceiling: AES can raise to L6 vs. capped at L4
- [ ] Trigger threshold: bloom_current ≥ 3 across N concepts in branch (deterministic) vs. post-synthesis LLM flag

AC (once open decisions resolved):
- [ ] `session.type` field: `'quiz' | 'application'` — backward compatible
- [ ] `application_task.json` written at task definition step
- [ ] `application_standup.jsonl` per-round exchanges persisted
- [ ] `application_evidence.json` extracted evidence with bloom attribution
- [ ] `prompts/06_application_evaluator.md` — branches on llm_assistance_mode
- [ ] Trigger suggestion visible in TrackPage when bloom threshold met
- [ ] Synthesis integration updates concept_map bloom_current with 'application' tag
- [ ] UI: new session creation flow for application type

---



---

### E11 — System-Initiated Elaboration

> As a learner, after submitting answers and receiving evaluation, I want axon to surface
> targeted follow-up prompts for answers that contain vocabulary gaps, broken analogies,
> or compound-concept confusions — so that the system forces deeper articulation rather
> than waiting for me to ask.

**Why this closes the E9 loop:** E9 measures inquiry quality from learner-initiated
threads. E11 is the system-initiated counterpart — axon identifies exactly where the
learner's explanation was thin and asks the follow-up question they should have asked
themselves. Together, E9 + E11 cover both sides of inquiry quality.

**Why post-evaluation, not real-time inline:**
Real-time triggering (keyword lookup against a live textarea) creates a mutation problem:
if the learner edits their answer, the trigger signal may no longer apply. The evaluator
already has the final submitted answer and is already LLM-powered — elaboration triggers
are a natural extension of the evaluation output, not a second call.

**Mechanism:**
1. Evaluator prompt extended: for each response, emit an optional `elaboration_triggers`
   array alongside the existing evaluation fields. Each trigger has `signal`, `term`,
   and `prompt`.
2. After evaluation: if any triggers exist, UI offers redirect to consolidated
   **Elaboration page** (before synthesis step).
3. Learner answers elaboration prompts as free-text. These are persisted as
   `03b_elaborations.json` and fed into E9's `inquiry_precision` scoring at synthesis.
4. Trigger aggressiveness controlled by `AXON_ELABORATION_RATE` (0.0–1.0 float):
   - `0.0` = never surface (default — opt-in)
   - `0.5` = bottleneck concepts only + broken analogy signals
   - `1.0` = all vocabulary + compound-concept + analogy signals

**Three trigger signal types:**
| Signal | What it detects | Example follow-up |
|---|---|---|
| `vocabulary` | Important term used imprecisely or in wrong context | "You used 'gradient' — describe what it represents geometrically, not just operationally" |
| `analogy_gap` | Learner used an analogy that transfers surface features but not mechanism | "Your analogy works for X but breaks at Y — where exactly does it stop holding?" |
| `compound_concept` | Answer conflates two concepts from the concept map that are related but distinct | "You described these two things as the same — what's the distinction between them?" |

**Concept map integration:**
- Trigger threshold lowered for `is_bottleneck=true` concepts
- `analogical_structural_mapping` concept (index 34, Pedagogy branch) — when `bloom_current < 3`,
  `analogy_gap` triggers are more aggressive
- `inquiry_precision` on the concept feeds back: low-precision concepts trigger more readily

**New domain additions:**
```go
type ElaborationTrigger struct {
    Signal  string `json:"signal"`  // "vocabulary" | "analogy_gap" | "compound_concept"
    Term    string `json:"term"`    // the word/phrase that fired the signal
    Prompt  string `json:"prompt"`  // the follow-up question to surface
    ConceptIndex int `json:"concept_index"` // concept the trigger is attributed to
}

// Added to Evaluation:
ElaborationTriggers []ElaborationTrigger `json:"elaboration_triggers,omitempty"`

// New session file: 03b_elaborations.json
type ElaborationResponse struct {
    QuestionID   string `json:"question_id"`
    TriggerIndex int    `json:"trigger_index"`
    Response     string `json:"response"`
}
```

**New env var:**
```
AXON_ELABORATION_RATE=0.0   # default — never trigger (opt-in)
AXON_ELABORATION_RATE=0.5   # bottleneck concepts + broken analogies
AXON_ELABORATION_RATE=1.0   # all signal types
```

- Size: **M** · Priority: **Should** · Status: **Spec only**

Open decisions:
- [ ] Does failing to complete elaboration block synthesis, or is it optional?
- [ ] Should elaboration responses affect bloom_current directly, or only inquiry_precision?
- [ ] Maximum triggers per session (prevent elaboration fatigue — cap at 3?)
- [ ] Does AXON_ELABORATION_RATE=0 fully suppress UI redirect, or only suppress aggressive triggers?

AC:
- [ ] `elaboration_triggers` field in `Evaluation` schema (backward compatible — omitempty)
- [ ] `AXON_ELABORATION_RATE` env var read at evaluate time; 0.0 = no triggers emitted
- [ ] Evaluator prompt injects elaboration signal instructions when rate > 0
- [ ] `03b_elaborations.json` persisted per session when elaborations submitted
- [ ] UI: post-evaluation redirect to ElaborationPage when triggers exist
- [ ] ElaborationPage: one card per trigger, free-text response, submit all at once
- [ ] ElaborationPage: back-link returns to main evaluation view
- [ ] Synthesis reads `03b_elaborations.json` and factors into `inquiry_precision` update
- [ ] `GET /api/tracks/:id/sessions/:num/elaborations` — returns triggers + responses

---

---

### F4 — Concept Map Tending

> As a learner with a large concept map (30+ concepts), I want the system to ensure
> that dormant concepts — those never quizzed or untouched across many sessions —
> eventually surface in my sessions, without sacrificing question quality or forcing
> irrelevant questions.

**Why this matters:** with 38 concepts and 8 questions per session, each session
samples ~21% of the concept space. The generator naturally prioritizes bloom-gap
and bottleneck concepts. A concept at bloom=1, bloom_target=5, is_bottleneck=false,
with no bloom gap yet established — is invisible to the generator forever. This is
a silent coverage failure.

**Design constraint:** quality is the top priority. Dormant concepts must be biased
toward, not forced. The mechanism influences selection probability, not selection
outcome.

**Mechanism — pure function, no LLM:**
```
dormancy_score(concept) =
  sessions_since_last_quizzed × (bloom_target - bloom_current) / max(bloom_current, 1)
```
- `sessions_since_last_quizzed`: derived from `times_quizzed` + current session count.
  If `times_quizzed == 0`, treat as `sessions_since_last_quizzed = total_sessions`.
- High score = never quizzed AND large bloom gap = highest tending priority.
- Bottleneck concepts already get generator priority; tending targets non-bottleneck
  dormant concepts specifically.

**Generator integration:**
The user prompt receives a new `## Dormant Concepts` section listing up to 3
concepts with the highest dormancy score:
```
## Dormant Concepts (tending bias — quality permitting)
[7] loss function landscape (score: 12.0) — not quizzed in 3 sessions, bloom gap 3
[22] attention mechanism (score: 9.0) — never quizzed, bloom gap 4
```
The instruction: *"Include at least one dormant concept if it can be tested at
appropriate quality. Skip if no natural question exists — do not force."*

**New concept field:**
```go
TimesQuizzed int `json:"times_quizzed,omitempty"`
```
Updated by `ApplySynthesis` from `01_questions.json` — count how many questions
referenced each concept_index in the session.

**No new LLM call. No new session file. Pure counter + formula.**

- Size: **S** · Priority: **Should** · Status: **Spec only**

Open decisions:
- [ ] Dormancy window: bias kicks in after N sessions (suggest N=2) or always?
- [ ] Cap on dormant concepts injected per session (suggest 3)?
- [ ] Should tending apply to `AXON_LEVEL_OVERRIDE=recall` (probably not — recall focuses reinforcement, not discovery)?

AC:
- [ ] `times_quizzed` field on `Concept` — zero-value safe, omitempty
- [ ] `ApplySynthesis` increments `times_quizzed` for each concept_index appearing in `01_questions.json`
- [ ] `BuildUserPrompt` computes top-N dormant concepts via pure formula, injects as `## Dormant Concepts` section
- [ ] Dormant bias section omitted entirely when `AXON_LEVEL_OVERRIDE=recall`
- [ ] `GET /api/tracks/:id/difficulty-preview` extended: includes `dormant_concepts` list in response

---

| Item | Reason |
|---|---|
| Transfer function direct measurement | Requires real-world outcome data axon cannot collect — E10 is the closest proxy |
| Application sandbox | Separate infrastructure, out of scope |
| Full interleaving engine | Needs S2.1 + multiple sessions of clean data first |
| Biometric arousal signal | No input source available |
| Automated audience framing personalization | Requires richer conversation corpus than available |
| Prerequisite graph auto-correction | Implicit signal via aspiration_count exists (E2); explicit validity scoring deferred — needs multi-track baseline |

---

### F6 — Scalable Multi-Call Generation with Question-Level Deduplication ✅ Done

> As a learner, I want sessions to contain high-quality, non-redundant questions even when
> the concept map is large or job-post context files are present — without extra LLM roundtrips
> or single-prompt constraint overload.

**Mechanism:**

1. **Concept map call** — requests `target + 25% overage` questions (min +2). A single
   call for targets ≤ 16; multi-call with concept exclusion lists is future work.
2. **Per-job-post call** — one dedicated call per `*.job.md` file. The call receives the
   job content as primary framing and the concept map for `concept_indexes` only. Target
   scales with session size: 3 / 5 / 8 questions for target < 12 / 12–15 / 16+.
3. **Two-stage deduplication** (`deduplicateQuestions`):
   - Stage 1: exact match on `(sorted concept_indexes, bloom_level)` — structurally
     identical questions are dropped, keeping first occurrence.
   - Stage 2: MMR text similarity via `rag.DistinctTopN` — picks `n` maximally
     distinct questions from the remaining pool by minimising pairwise word-overlap.
4. **Deterministic shuffle** — FNV-64a hash of `generationID` seeds `math/rand.Shuffle`
   so concept-map and job-post questions are interleaved consistently.

**`AXON_QUESTION_COUNT`** — new env var controlling session target (default 8). Overproduction
and trimming are handled transparently; operator sets the desired output count.

**Why this is better than ratio-rule injection:**
- Each LLM call has a single, narrow responsibility — no competing constraint clauses.
- Structural deduplication is guaranteed by construction (Go code), not LLM compliance.
- Scales to 20+ questions by adding more concept-map calls with exclusion lists (future).

**ACs:**
- [x] `deduplicateQuestions` — two-stage, deterministic
- [x] `shuffleQuestions` — FNV-seeded, deterministic
- [x] `BuildJobPostSystemPrompt` / `BuildJobPostUserPrompt` — focused job-framed call
- [x] `AXON_QUESTION_COUNT` wired through `main.go → server.Config → handler`
- [x] `dev.sh` documents new env var
- [x] Seed injection from previous `BuildUserPrompt` removed (now superseded)
- [x] `rag.DistinctTopN` preserved as the deduplication primitive (stage 2)

---

## Sprint Plan

| Sprint | Focus | Items | Exit Criteria |
|---|---|---|---|
| 1 | Conversation Analyzer | M1.1, M1.2 | Prompt 05 working, question generator ingests output | ✅ Done |
| 2 | Fingerprint workflow | M1.3 | Workflow documented, sample output committed | ✅ Done |
| 3 | State Detection + Nudge | E3, E4 | composite_state in synthesis, nudge banner in UI | ✅ Done |
| 4 | Smart Fork | F1.1, F1.2 | Auto-distill + conversation snapshot on fork | ✅ Done |
| 5 | Live Concept Map Inheritance | F2 | Child sessions use ancestor bloom floor + exploration + inquiry cascade | ✅ Done |
| 6 | Pre-Merge Distill | F3 | RunSilent on source tracks before merge | ✅ Done |
| 7 | Dual State Architecture | M2.1, M2.2 | concept_map schema updated, exploration_unlocked live | ✅ Done |
| 8 | Inquiry Quality schema | E9 schema | inquiry_precision field defined, extraction prompt drafted | ✅ Done |
| 8b | Difficulty level system | — | Named levels (recall→extreme) on numeric spine, AXON_LEVEL_OVERRIDE, /config/levels, /difficulty-preview | ✅ Done |
| 8c | Path-aware contextual scoring | — | StateSnapshot on sessions, learner_path.jsonl, delta_multiplier in apply_synthesis, consecutive fail counter | ✅ Done |
| 8d | Scalable multi-call generation + dedup | F6 | Multi-call pipeline, job-post isolated call, MMR question dedup, AXON_QUESTION_COUNT | ✅ Done |
| **9** | **Application Evidence — schema** | **E10 schema** | **session.type field, application_task.json shape, open decisions resolved** | **← next** |
| 10 | Faith-based unlocking | S1.1, S1.2 | Reach questions in rotation, aspiration gap tracked | |
| 11 | Regression intelligence | S2.1, S2.2, S2.3 | Regression classified, frustration turn applied | |
| 12 | Inquiry Quality full | C3.1, C3.2, C3.3 | Inquiry Patterns in snapshot, inquiry_precision tracked | |
| 13 | Application Evidence — build | E10 full | Interrogator loop live, evidence extraction, bloom write-back | |
| 14 | Self-Reflection Loop | C2.1, C2.2, C2.3 | ReflectCommand live, feeds next session with E9+E10 signals | |
| 15 | System-Initiated Elaboration | E11 | elaboration_triggers in evaluation, ElaborationPage, 03b_elaborations.json, AXON_ELABORATION_RATE | |
| 16 | Concept Map Tending | F4 | times_quizzed on concepts, dormancy_score pure function, dormant bias in generator | |

---

## Definition of Done (all items)

- [ ] Feature works end-to-end in at least one real session (not just unit-tested)
- [ ] Schema changes backward-compatible with v1 tracks
- [ ] New prompts stored in `prompts/` with track-level copies for active tracks
- [ ] Synthesis output updated to include new signal fields
- [ ] `plan_v2.md` architectural section updated if behavior changed from spec
- [ ] No new external dependencies introduced without explicit decision
