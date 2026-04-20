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
```

**Critical path:** `E1 → E3 → E4 → E2 → E5`

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

## Won't Have — This Iteration

| Item | Reason |
|---|---|
| Transfer function measurement | Requires real-world outcome data axon cannot collect |
| Application sandbox | Separate infrastructure, out of scope |
| Full interleaving engine | Needs S2.1 + multiple sessions of clean data first |
| Biometric arousal signal | No input source available |
| Automated audience framing personalization | Requires richer conversation corpus than available |

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

---

## Definition of Done (all items)

- [ ] Feature works end-to-end in at least one real session (not just unit-tested)
- [ ] Schema changes backward-compatible with v1 tracks
- [ ] New prompts stored in `prompts/` with track-level copies for active tracks
- [ ] Synthesis output updated to include new signal fields
- [ ] `plan_v2.md` architectural section updated if behavior changed from spec
- [ ] No new external dependencies introduced without explicit decision
