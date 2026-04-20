# Axon v2 — Conception & System Design

> **Status:** v2 design. For v1 architecture see `plan.md`. For step-by-step usage see `how_to.md`.
> **Scope:** Extends axon from a measurement system into a navigating system — one that adapts
> not just what it asks, but how it reads the learner and how the learner configures it.

---

## 1. Mission & Vision

**Mission:**
Build a precise, portable, auditable model of how one specific person thinks and knows —
then use it to calibrate every next challenge to the exact edge of their competence.

**Vision:**
Replace self-assessment (unreliable) and generic quizzing (uncalibrated) with a system
that triangulates behavioral evidence across sessions until the learning curve itself
becomes measurable — and eventually, until the path to solutions expertise is navigable
by any type of person, not just structured learners.

---

## 2. What Axon Is Not — Product Constraints

These are decision-forcing constraints. When two implementation paths look equivalent
locally, these break the tie. An LLM or developer reading this should use them to
reject designs that technically work but violate the product identity.

| Constraint | What this rules out |
|---|---|
| **Not a flashcard app** | No "flip and mark known/unknown" flows. All evidence is behavioral, not self-reported. |
| **Not a gamified streak tracker** | No points, badges, streaks, or visible score trends shown to the learner during sessions. Progress is shown in synthesis notes, not as reward mechanics. |
| **Not a self-report system** | Learner-stated confidence is an input signal only — never the output or the score. Brier score calibrates it against behavioral evidence. |
| **Not a content delivery platform** | Axon does not teach. It assesses and calibrates. Content comes from the learner's own `.md` files. |
| **Not a cloud service** | Local-first always. No account, no API key required by default, no remote storage of learner data. |
| **Not a one-size curriculum** | No global concept list, no shared question bank. Every track is specific to one learner's documents and context. |
| **Not a passive tool** | Axon must be able to push back — suggest harder questions, name a state the learner is in, recommend against a path. It is a calibrated interrogator, not a compliant quiz generator. |

---

## 3. What v1 Established

v1 built the measurement foundation:

- Attribution-corrected profiling (separating learner signal from LLM co-authorship)
- Bloom's Taxonomy targeting (6-level cognitive complexity scaffold)
- Concept bottleneck graph (prerequisite-gated unlocking)
- Brier score calibration (confidence accuracy tracking)
- Error taxonomy (misconception vs gap vs careless vs ceiling)
- Spaced repetition scheduling (retention half-life per concept)
- Track isolation with forward propagation (VCS-tracked, auditable, portable)
- Free-form conversation threads per question (first-class, not a side channel)

**v1 ceiling:** it measures competence well but prescribes interventions crudely.
It knows *what* the learner knows. It does not yet know *how they learn*, *what state
they're currently in*, or *how to meet them there*.

---

## 4. What v2 Adds — The Navigation Layer

v2 converts measurement into navigation. Four new capabilities:

### 3.1 Cognitive Fingerprinting (Conversation Analyzer)
The fourth evidence source. Free-form conversation threads reveal reasoning style,
misconception framings, curiosity clusters, and mental models — signal that structured
Q&A cannot surface. This becomes a first-class input to question generation.

**Output:** `context/conversation_analysis.json` per track
**Feeds into:** distractor selection, question framing, reach question targeting

### 3.2 Dual State Model
Split the learner model into two non-mixing state variables:

| State | Definition | Rules |
|---|---|---|
| **Evidence state** | What behavioral data proves | Strict, prerequisite-gated, drives scoring |
| **Exploration state** | What the learner is allowed to engage | Faith-based, curiosity-driven, drives question selection |

Profile scoring always derives from evidence state only. Faith-based exposure operates
on exploration state only. These never merge in the scoring logic.

**Why this matters:** removes the bottleneck gate as a hard block. Top-down learners
can engage above their demonstrated floor without corrupting the profile.

### 3.3 Composite Learner State Detection
Eight named states detectable from observable signals (session frequency, confidence
pattern, score trajectory, explanation length, thread depth):

**Axon's view of the learner:**

```
                      CHALLENGE ALIGNMENT
               under-challenged | in-zone | over-challenged
              ─────────────────────────────────────────────
  charged     │   Cruising      │  Flow   │   Stretch      │
              │                 │         │                │
M stable      │   Plateau       │  Grind  │   Overreach    │
O             │                 │         │                │
M depleted    │   Coasting      │  Grind  │   Aversion     │
E             │                 │  (heavy)│   Edge         │
N             ─────────────────────────────────────────────
T
```

**Learner's view of axon** (the mirror — determines whether any intervention lands):

```
                      PERCEIVED RELEVANCE
               feels irrelevant | feels right | feels abstract
              ──────────────────────────────────────────────────
  trusted     │ Trustworthy     │  Partner   │ Trusted but    │
              │ but off-topic   │            │ over my head   │
P neutral     │ Indifferent     │  Tool      │ Intimidating   │
E             │                 │            │                │
R distrusted  │ Irrelevant      │  Tolerated │ Hostile        │
C             │                 │            │                │
              ──────────────────────────────────────────────────
```

**Critical:** the learner's trust matrix is the prior condition for the axon matrix.
A learner in Hostile or Irrelevant will produce responses that corrupt the profile.
The pre-session nudge is where both matrices get reconciled.

**Most likely mappings:**

| Axon sees | Learner likely feels |
|---|---|
| Flow | Partner |
| Aversion Edge | Intimidating → Hostile if unaddressed |
| Plateau | Tool or Indifferent |
| Cruising | Trustworthy but off-topic |
| Overreach | Trusted but over my head |
| Coasting | Irrelevant |

### 3.4 Mode Config + Pre-Session Nudge
Before each session, axon names its inferred composite state and suggests a behavioral
direction. The learner can confirm or override. The override is itself a signal.

**Format:** one sentence, actionable, rejectable.
> *"Last two sessions: strong on framing, stalling on pressure. Suggested: one forced-attempt
> question outside your comfort area. You can skip this."*

**Multi-axis dials** (adjustable per session, inherited by forks):

| Axis | Left | Right |
|---|---|---|
| Pacing | consolidate known ground | push into new territory |
| Framing | bottom-up (foundations first) | top-down (big picture first) |
| Pressure | low stakes, exploratory | high stakes, performance |
| Abstraction | concrete examples | formal/structural |

Secondary modifiers (shift behavior within any composite state, not new states):
- Identity misalignment → add audience framing before hard questions
- Revision resistance → force one contradiction question per session
- Cold start → warmup sequence regardless of state
- Proximity to stakes → shift toward application framing

---

## 5. Concept Registry

All named concepts introduced in v2, with research grounding:

| # | Concept | Plain English | Research Anchor |
|---|---|---|---|
| 1 | Adaptive Cognitive Scaffolding | Questions at the edge of demonstrated competence | Vygotsky ZPD (1978); Anderson & Krathwohl (2001) |
| 2 | Attribution-Corrected Signal Isolation | Separate learner knowledge from LLM co-authorship | Signal detection theory (Green & Swets, 1966) |
| 3 | Calibrated Confidence Tracking | Does the learner know what they know | Brier (1950); Dunning & Kruger (1999) |
| 4 | Productive Failure / Faith-Based Unlocking | Exposure before prerequisites — failure is the mechanism | Kapur (2010); Kapur & Bielaczyc (2012) |
| 5 | Desirable Difficulties | Harder retrieval = more durable retention | Robert Bjork, UCLA (1994, 2011) |
| 6 | Generative Learning / Teach-Back | Construct explanation from scratch — not justify a choice | Wittrock (1989) |
| 7 | Transfer-Appropriate Processing | Learn in the format closest to real application | Morris, Bransford & Franks (1977); Chi et al. (1981) |
| 8 | Cognitive Load Management | Match difficulty to available working memory | Sweller (1988) |
| 9 | Yerkes-Dodson Arousal Calibration | Peak learning at moderate arousal — too easy and too hard both fail | Yerkes & Dodson (1908) |
| 10 | Metacognitive Gap Detection | Delta between self-assessed and behavioral competence | Flavell (1979) |
| 11 | Interleaved Practice | Mix concepts across sessions — harder but more durable | Rohrer & Taylor (2007) |
| 12 | Intrinsic Motivation / Identity Alignment | Learning accelerates when learner sees themselves becoming this type | Deci & Ryan, SDT (1985) |
| 13 | Dual State Model | Evidence state (scoring) vs exploration state (selection) — never mix | Winne & Hadwin COPES model (1998) |
| 14 | Composite Learner State | Named multi-axis configurations with predictable intervention responses | D'Mello & Graesser (2012) |
| 15 | Aspiration Gap | Delta between concepts learner gravitates toward vs. evidence supports | Novel — implied by SDT + Vygotsky |
| 16 | Situation Library | Practitioner-layer record of real contexts encountered | Klein, naturalistic decision making (1998) |

---

## 6. New Question Formats in v2

| Format | Description | Bloom's Level | Scoring |
|---|---|---|---|
| `reach` | 2 levels above demonstrated — failure expected | L5–L6 | Stretch signal, not gap signal |
| `teach_back` | Explain concept X from scratch, unprompted | L4–L6 | Completeness, analogy quality, edge-case awareness |
| `teach_forward` | Explain to audience with specific prior knowledge X but not Y | L5–L6 | Multiple framings = deeper depth signal |
| `contradiction` | Hold two valid but contextually opposing truths simultaneously | L5 | Judgment, not recall |
| `unknown_edge` | Describe a scenario where you wouldn't know what to do | Meta | Self-map vs system map delta |
| `decision_audit` | You chose X in a real project — what did you learn was wrong | L4–L5 | Application history as evidence |

---

## 7. New Session Signals in v2

| Signal | Source | What It Tells You |
|---|---|---|
| Aspiration gap | Curiosity clusters vs evidence state | Top-down learner vs foundational gap |
| Composite state | Frequency + confidence + score + thread depth | Which intervention regime applies |
| Perceived trust | Pre-session nudge override pattern | Whether any intervention will land |
| Delta-from-last-correct | Regression between sessions | Shallow learning vs interference vs decay |
| Situation log | Optional one-line field per session | Practitioner exposure depth |
| Reach attempt | Engagement on out-of-zone questions | Aspiration evidence even on failure |

---

## 8. Theoretical Contributors — Coverage Map

Axon's ultimate goal is learner improvement and real-world success, not just score improvement.
That goal is served by multiple learning science frameworks. This section names them, estimates
their contribution weight, and tracks axon's current coverage honestly.

| # | Framework | What it measures | Est. weight | Axon coverage |
|---|---|---|---|---|
| 1 | **Spaced Repetition** (Ebbinghaus/Leitner) | Retention over time, forgetting curve | ~20% | ✅ Implemented — `spaced_repetition` in concept_map, synthesis scheduling |
| 2 | **Deliberate Practice** (Ericsson) | Practicing at the edge of ability with tight feedback | ~20% | ⚠ Partial — bottleneck detection + composite states approximate it; no explicit challenge-calibration dial yet |
| 3 | **Bloom's Taxonomy** | Cognitive depth progression L1→L6 | ~15% | ✅ Primary framework — question generation, evaluation, synthesis |
| 4 | **Transfer / Application** | Can the learner use knowledge in novel real-world contexts | ~15% | ⚠ E10 (Application Evidence Sessions) targets this — not yet built |
| 5 | **Metacognition** (Flavell) | Knowing what you know and don't; directing your own learning | ~10% | ⚠ Brier score calibration + E9 Inquiry Quality — incomplete |
| 6 | **Motivation / Self-Determination** (Deci & Ryan) | Autonomy, competence, intrinsic drive | ~10% | ⚠ Trust proxy + pre-session nudge — shallow; no explicit intrinsic motivation model |
| 7 | **Feedback Quality** | Specific, actionable, timely correction per attempt | ~5% | ✅ `feedback_for_learner` in evaluations |
| 8 | **Social / Collaborative** (Vygotsky ZPD) | Learning with and through others; zone of proximal development | ~5% | ❌ Not in axon |

**Current coverage: ~55–60% of what drives learner success in the knowledge/engineering domain.**

### Known gaps — recorded as explicit deferred items

- **Deliberate Practice calibration**: the dial system (E4/E5) partially addresses this but there is no
  explicit "challenge at exactly your edge" mechanism. Composite state detection (E3) is the closest proxy.
- **Transfer**: the single most valuable and most absent signal. E10 (Application Evidence Sessions) is
  the primary path to measuring this. Until E10 is built, axon measures mastery-in-context only.
- **Metacognition depth**: calibration via Brier score is shallow. E9 (Inquiry Quality) adds one more
  dimension. True metacognitive tracking would require learner self-reports and longitudinal gap awareness.
- **Intrinsic motivation**: trust proxy and nudge system are behaviorally inferred approximations.
  No direct motivation model is planned for this iteration.
- **Social/collaborative**: out of scope for axon's current single-learner model. Noted as a ceiling.

### Creative and physical domains — known category mismatch

Bloom's Taxonomy and MCQ-style evaluation are the wrong shape for creative or physical skill domains.
These domains require different frameworks:

| Domain | Right framework | Status |
|---|---|---|
| Physical skill (cycling, swimming, running) | Fitts & Posner motor learning stages + performance metrics (pace, RPE, power, periodization) | Out of scope |
| Creative output (music, art, design) | Divergent thinking (Guilford) + creative self-efficacy (Bandura) — fluency, flexibility, originality | Out of scope |
| Endurance / performance sport | Periodization theory + psychological toughness models | Out of scope |
| All domains | Flow state (Csikszentmihalyi) | ⚠ Partially captured in composite learner states |

**Axon's realistic ceiling** for the knowledge/engineering domain, fully built out: ~75–80% of what
drives learner success. For creative or physical domains, the core model would need a different ontology —
not just gap-filling.

---

## 9. The Unknown — Transfer Function

The single unknown that, if resolved, unlocks compounding gains:

**The transfer function** — the mapping between "demonstrates mastery on this question type"
and "can solve a novel problem requiring this concept in an unfamiliar context under pressure."

We don't know its shape. It may be:
- Linear (concept mastery → application ability scales proportionally)
- Threshold-gated (nothing transfers below a certain level, then it suddenly does)
- Context-dependent (transfers within domain, barely across domains)

If threshold-gated, axon's incremental progression is sub-optimal — you'd want to push
harder through the threshold. If context-dependent, branch isolation may be actively harmful.

**Proxies being built toward it:** situation log (S6), decision audit questions, reach
question engagement pattern. The conversation analyzer (M1) is the most direct current
path to approximating this function.

**E10 (Application Evidence Sessions) is the primary direct proxy.** By having the learner
narrate real application work under AI interrogation, axon gains the first signal that
is not mediated by question format — it is raw evidence of transfer in progress.

---

## 10. Cyclic Dependencies and Their Resolutions

| Cycle | Why It Blocks | v2 Resolution |
|---|---|---|
| Confidence ↔ Competence | Can't calibrate without competence; can't build competence without attempting at the edge | Forced-attempt reach questions scored as stretch, not gap |
| Vocabulary ↔ Conceptual depth | Can't explain without words; can't acquire words without concept | Vocabulary anchor after wrong answer — "the word is X, does that change your explanation?" |
| Application ↔ Understanding | Need understanding to apply; need application to deepen past textbook level | Situation log + decision audit questions as proxies |
| Motivation ↔ Progress | Need visible progress to sustain motivation; need motivation to make progress | Behavioral delta in synthesis ("you did X you couldn't do in session N-2") — not score delta |
| Situation exposure ↔ Pattern recognition | Need library to extract lessons; need foundational recognition to learn from exposure | Situation log builds the library; reach questions seed early exposure |

---

## 11. Architectural Changes Required for v2

| Component | v1 State | v2 Change |
|---|---|---|
| `concept_map.json` | bloom_current, is_bottleneck | + exploration_unlocked (bool), aspiration_count (int) |
| `00_profile_snapshot.json` | per-concept bloom scores | + composite_state, aspiration_gap, perceived_trust_proxy |
| `01_questions.json` | format: mcq/free_text/scenario_mcq | + format: reach/teach_back/teach_forward/contradiction/unknown_edge |
| `03_evaluations.json` | correctness, feedback | + signal_type: gap/stretch/aspiration/regression |
| `04_synthesis.json` | bloom updates, next session plan | + composite_state_detected, nudge_suggestion, dial_positions |
| `context/` | source snapshots | + `conversation_analysis.json` (from prompt 05) |
| `prompts/` | 01–04 | + `05_conversation_analyzer.md`, `06_application_evaluator.md` |
| Session pre-flight | none | pre-session nudge output before question generation |
| Session type | implicit (always quiz) | + `type: 'quiz' \| 'application'` field on session |
| `application_task.json` | — | task definition: title, concept_indexes, llm_assistance_mode, success_criteria |
| `application_standup.jsonl` | — | per-round exchanges in the interrogator loop |
| `application_evidence.json` | — | extracted evidence: bloom_level per concept_index, orchestration quality score |

---

## 12. Critical Path to Compounding Gains

```
F2 (Live Concept Map Inheritance)       ← immediate unblock for multi-track learners
  → feeds E10 trigger logic meaningfully

F3 (Pre-Merge Idempotent Distill)       ← consistency fix before more tracks exist

M1 (Conversation Analyzer) ✅
  → M3 (Composite State Detection) ✅
      → M4 (Pre-Session Nudge) ✅
          → M2 (Dual State Model)
              → E9 (Inquiry Quality)
                  → E10 (Application Evidence Sessions)   ← transfer function proxy
                      → E8 (Self-Reflection Loop)
```

Everything else is additive. This chain is what converts axon from a measurement system
into a navigating system. Bypass it and v2 features are cosmetic.
