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
```

**Critical path:** `E1 → E3 → E4 → E2 → E5`
**Self-evolution path:** `E1 → E9 → E8` (parallel to main critical path, mergeable at E4)

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

### E2 — Dual State Architecture

**M2.1 — Split concept_map.json into evidence + exploration fields**
> As a system, I want each concept to carry both an evidence-state score (from behavioral
> data) and an exploration-state flag (from curiosity/aspiration signal) so that question
> selection and profile scoring never use the same variable.

- Touches: `concept_map.json` schema, profile snapshot schema, question generator
- New fields: `exploration_unlocked: bool`, `aspiration_count: int`
- Size: **M**
- Priority: **Must**
- Sprint: **3**

AC:
- [ ] Schema change backward-compatible with v1 concept maps (new fields optional, default false/0)
- [ ] Question generator reads `exploration_unlocked` for candidate pool selection
- [ ] Profile scoring reads `bloom_current` only — never `exploration_unlocked`
- [ ] `concept_taxonomy.md` updated with new fields

---

**M2.2 — Provisional unlock logic**
> As a system, I want concepts with aspiration_count >= 2 to become provisionally
> exploration-unlocked at 0.5 scoring weight, so that top-down learners can engage above
> their demonstrated floor without corrupting their profile.

- Size: **M**
- Priority: **Must**
- Sprint: **3**
- Blocked by: M2.1

AC:
- [ ] aspiration_count increments when curiosity cluster detected OR learner engages with concept outside question flow
- [ ] Provisional unlock threshold configurable (default: 2)
- [ ] Scoring weight for reach questions on provisionally unlocked concepts: 0.5
- [ ] Weight scales toward 1.0 with consecutive non-failure engagement (3 attempts)

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
- [ ] `buildDistillSystem()` includes Inquiry Patterns section
- [ ] Section outputs: high-precision questions verbatim, low-precision questions with
  suggested sharper version, missed pivots (the question one step away from the insight)
- [ ] Question generator ingests Inquiry Patterns (same pattern as Curiosity Clusters)

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
- [ ] `inquiry_precision` added to Concept struct (omitempty, default 0)
- [ ] ApplySynthesis reads Inquiry Patterns section and updates per concept
- [ ] Composite state detector reads `inquiry_precision` as additional input signal
- [ ] `concept_taxonomy.md` updated with field definition

---

## Won't Have — This Iteration

| Item | Reason |
|---|---|
| Transfer function measurement | Requires real-world outcome data axon cannot collect |
| Application sandbox | Separate infrastructure, out of scope |
| Full interleaving engine | Needs S2.1 + multiple sessions of clean data first |
| Biometric arousal signal | No input source available |
| Automated audience framing personalization | Requires richer conversation corpus than available |
| Prerequisite graph auto-correction | Implicit signal via aspiration_count exists (E2); explicit validity scoring deferred — needs multi-track baseline |

---

## Sprint Plan

| Sprint | Focus | Items | Exit Criteria |
|---|---|---|---|
| 1 | Conversation Analyzer | M1.1, M1.2 | Prompt 05 working, question generator ingests output |
| 2 | Fingerprint workflow | M1.3 | Workflow documented, gitignore updated, sample output committed |
| 3 | Dual State Architecture | M2.1, M2.2 | concept_map schema updated, provisional unlock logic tested |
| 4 | State Detection core | M3.1 | 8 states detectable, composite_state in synthesis |
| 5 | Trust proxy | M3.2 | perceived_trust_proxy in synthesis, session behavior modulated by it |
| 6 | Nudge + Dials | M4.1, M4.2 | Pre-session nudge in UI, dial controls working, fork inheritance |
| 7 | Faith-based unlocking | S1.1, S1.2 | Reach questions in rotation, aspiration gap tracked |
| 8 | Regression intelligence | S2.1, S2.2, S2.3 | Regression classified, frustration turn applied, situation log live |
| 9 | Generative formats | C1.1, C1.2 | Teach-back and teach-forward evaluated and scored |
| 10 | Edge formats | C1.3, C1.4 | Contradiction and unknown-edge probe live |
| 11 | Self-Reflection Loop | C2.1, C2.2, C2.3 | ReflectCommand live, concept auto-promotion working, feeds next session |
| 12 | Inquiry Quality | C3.1, C3.2, C3.3 | Inquiry Patterns in snapshot, counterfactual scaffold in threads, inquiry_precision tracked |

---

## Definition of Done (all items)

- [ ] Feature works end-to-end in at least one real session (not just unit-tested)
- [ ] Schema changes backward-compatible with v1 tracks
- [ ] New prompts stored in `prompts/` with track-level copies for active tracks
- [ ] Synthesis output updated to include new signal fields
- [ ] `plan_v2.md` architectural section updated if behavior changed from spec
- [ ] No new external dependencies introduced without explicit decision
