# Knowledge Assessment & Growth System — Architecture Plan

> **Scope:** Axon is a standalone VCS-tracked project (was `.experiments/` — promoted to its own repo).
> All track data, sessions, context snapshots, and conversations are committed to VCS.

---

## 1. The problem this solves

Standard self-assessment is unreliable. You cannot accurately gauge your own blind spots
by introspection — you don't know what you don't know. The `.md` files in this repo
contain rich signal about the practitioner's knowledge level, but that signal is noisy
because the artifacts are collaborative (human + LLM). A single quiz session doesn't
solve this either — it gives a snapshot, not a trajectory.

This system triangulates from three independent evidence sources:

```
Source 1: .md corpus analysis     → starting profile (attribution-corrected)
Source 2: quiz performance         → behavioral evidence (can't fake this)
Source 3: explanation quality      → depth vs. surface knowledge separator
                     ↓
              Unified profile score (per dimension, with confidence)
                     ↓
              Adaptive next challenge (calibrated to zone of proximal development)
```

---

## 2. Bloom's Taxonomy as the scaffolding layer

Bloom's Revised Taxonomy (Anderson & Krathwohl, 2001) gives a principled ordering of
cognitive complexity. Every question in this system is tagged to a level. The system
automatically adjusts the distribution based on current profile score.

| Level | Verb family | What it tests | Example (LLM domain) |
|---|---|---|---|
| 1. Remember | Recall, name, list | Does it exist in long-term memory? | "What does cosine similarity measure?" |
| 2. Understand | Explain, summarize, classify | Can they restate in own words? | "Why does dot product need normalization?" |
| 3. Apply | Use, execute, implement | Can they use it in a standard situation? | "Given these two vectors, compute similarity" |
| 4. Analyze | Distinguish, compare, break down | Can they see structure and relationships? | "Why would BM25 outperform dense retrieval on this corpus?" |
| 5. Evaluate | Judge, critique, justify | Can they make principled tradeoffs? | "Critique this RAG pipeline design for a daily-update corpus" |
| 6. Create | Design, construct, compose | Can they produce something new? | "Design a retrieval strategy for a multi-lingual codebase" |

### Distribution targets by composite score

| Score range | L1–L2 | L3 | L4–L5 | L6 |
|---|---|---|---|---|
| 0–40 (novice) | 60% | 30% | 10% | 0% |
| 40–65 (developing) | 30% | 35% | 30% | 5% |
| 65–80 (proficient) | 10% | 25% | 45% | 20% |
| 80–100 (expert) | 5% | 10% | 40% | 45% |

The system never stays at one distribution — it samples toward the **zone of proximal
development**: 70% at demonstrated level, 30% one level above.

---

## 3. Assessment dimensions

These map to the six scoring dimensions in the profile prompt. Each dimension is scored
independently because a practitioner can be expert in one and novice in another.

| ID | Dimension | What signals competence |
|---|---|---|
| D1 | LLM application architecture | Orchestration, state machines, retry, atomicity, idempotency |
| D2 | RAG stack depth | Embedding models, chunking strategies, retrieval, re-ranking |
| D3 | Prompt engineering | Systematic reasoning about prompt structure, not just trial-and-error |
| D4 | ML fundamentals | Loss functions, training dynamics, model architecture tradeoffs |
| D5 | Distributed systems | Failure modes, backoff, concurrency, consistency guarantees |
| D6 | Creative synthesis | Novel application of known concepts to unfamiliar constraints |

---

## 4. Analytics layer — the precision machinery

### 4.1 Per-question signals

Every question captures more than right/wrong:

| Signal | How captured | What it tells you |
|---|---|---|
| **Time-to-answer** | Start/stop timestamp | Fast+correct = automated recall; Slow+correct = working through it; Fast+wrong = misconception or guessing |
| **Confidence rating** | 1–5 self-report before reveal | Calibration quality — overconfidence is a distinct problem from ignorance |
| **Explanation** | Free-text after answer | Depth separator — can produce correct surface answer with wrong mental model |
| **Answer path** | Which distractor chosen if wrong | Identifies specific misconception, not just "wrong" |

### 4.2 Calibration score (Brier score)

The Brier score measures probabilistic forecast accuracy:

```
BS = (1/N) × Σ (confidence_fraction - outcome)²
```

Where `confidence_fraction` = confidence/5 (0.2 to 1.0) and `outcome` = 1 (correct) or 0 (wrong).

- **BS = 0.0**: perfect calibration
- **BS = 0.25**: random (no skill)
- **BS > 0.25**: confidently wrong (worse than random)

A practitioner with high domain knowledge but poor calibration (overconfident on gaps) is
a specific learning risk — they won't seek help where they need it. Track separately.

### 4.3 Error taxonomy

Every incorrect answer is categorized:

| Category | Signature | Intervention |
|---|---|---|
| **Misconception** | Consistently picks same wrong distractor | Targeted explanation of the wrong mental model |
| **Knowledge gap** | Random wrong answers, low confidence | Foundational content on that sub-topic |
| **Careless error** | Wrong answer, immediately self-corrects on review | No intervention needed; reduce weight in scoring |
| **Ceiling** | Correct but very slow, low confidence | At the edge of their zone — slightly easier next question |
| **Ambiguous item** | Many practitioners answer differently | Our question is bad — flag for removal |

### 4.4 Improvement metrics (tracked across sessions)

| Metric | Formula | Meaning |
|---|---|---|
| Absolute score delta | `score_n - score_0` | Raw improvement |
| Bloom's level advancement | `avg_level_n - avg_level_0` | Moving up cognitive complexity |
| Error recurrence rate | `recurring_errors / total_errors` | Are old mistakes going away? |
| Calibration trend | `BS_n - BS_0` | Getting better at knowing what you know |
| Retention half-life | Spaced repetition decay rate per topic | Durable vs. shallow learning |
| Explanation depth trend | LLM-rated explanation score over time | Growing from recall to understanding |

### 4.5 Spaced repetition schedule

Topics answered correctly get scheduled for re-test at increasing intervals:
- First correct: +3 days
- Second consecutive correct: +7 days
- Third: +21 days
- Fourth: +60 days (treated as consolidated)

Topics answered incorrectly reset to +1 day.

---

## 5. System components and prompt inventory

**Note:** The system evolved from a flat session structure (below) to a track-based
structure. The D1–D6 dimensions below map to the concept indexes in `track_1/concept_map.json`:
D1 ≈ concepts 22–31 (LLM Systems branch), D2 ≈ concepts 14–21 (RAG Architecture),
D3 ≈ concepts 23–25 (tokenization, system prompt, latent space), D4 ≈ concepts 0–7
(ML Fundamentals), D5 ≈ distributed systems patterns within the agentic loop concepts,
D6 ≈ cross-branch L5–L6 questions. The concept map provides finer granularity.

For the authoritative current structure, see `how_to.md`.

```
axon/                               ← this repo (was .experiments/ — now a standalone project)
  plan.md                           ← this file (baseline design; how_to.md is current)
  how_to.md                         ← CURRENT: step-by-step guide, track naming, folder layout
  concept_taxonomy.md               ← CURRENT: concept map schema, bottleneck detection, bloom updates
  experiments_log.md                ← session-level log across all tracks

  prompts/                          ← generic/framework prompts (D1-D6 notation)
    01_profile_from_docs.md         ← generic profile assessment (any domain)
    02_question_generator.md        ← generic question generator
    03_response_evaluator.md        ← generic evaluator (same as track version)
    04_session_synthesizer.md       ← generic synthesizer

  track_1/                          ← ML Fundamentals + Math Theory + RAG + LLM Systems
    README.md                       ← track parameters and branch definitions
    concept_map.json                ← 32-concept map with bloom_current, bottlenecks
    context/
      _sources.md                   ← which external .md files to snapshot before session 1
      *.snapshot.md                 ← snapshots of external .md files
    prompts/                        ← track-specific prompts (concept_index notation)
      00_concept_map_generator.md   ← generates concept_map.json from major branch names
      01_profile_from_docs.md       ← concept-level profile from corpus
      02_question_generator.md      ← concept-graph-aware question generator
      03_response_evaluator.md      ← evaluator (concept-aware)
      04_session_synthesizer.md     ← updates bloom_current per concept after session
    sessions/
      session_001/
        00_profile_snapshot.json    ← profile at session start (bloom_current per concept)
        01_questions.json           ← questions with concept_indexes
        02_responses.md             ← learner responses
        03_evaluations.json         ← evaluator output per question
        04_synthesis.json           ← bloom_current updates + next session plan

  track_2/                        ← second iteration; Upwork RAG interview-prep focus
    context/
      upwork_rag_jobs.snapshot.md   ← interview corpora for two production RAG roles
      _sources.md                   ← lineage + diff-from-track_1 table
    concept_map.json                ← copied from track_1 with bloom_current preserved
    ...
```

---

## 6. The iteration loop

```
┌─────────────────────────────────────────────────────────┐
│  SESSION N (within a track)                             │
│                                                         │
│  1. Load concept_map.json + prior profile snapshot      │
│  2. Generate 12 questions (concept-graph-aware)         │
│     - Bottleneck concepts with bloom_current < 3 first  │
│     - Spaced repetition due concepts second             │
│     - Largest bloom_gap third                           │
│  3. For each question:                                  │
│     a. Display question (concept_indexes attached)      │
│     b. Capture: time, confidence (1–5), answer, explain │
│     c. Evaluate explanation quality (LLM call)          │
│  4. Session synthesis:                                  │
│     a. Update bloom_current per concept (75% threshold) │
│     b. Compute Brier score                              │
│     c. Classify errors by taxonomy                      │
│     d. Update spaced_repetition per concept             │
│     e. Write session_log entry to experiments_log.md    │
│  5. Apply concept_map_updates to concept_map.json       │
└─────────────────────────────────────────────────────────┘
              ↓ iterate
┌─────────────────────────────────────────────────────────┐
│  SESSION N+1                                            │
│  bloom_current reflects behavioral evidence             │
│  Prerequisite-unlocked concepts now eligible            │
└─────────────────────────────────────────────────────────┘
              ↓ after 3+ sessions with same weak cluster
┌─────────────────────────────────────────────────────────┐
│  track_2                                              │
│  New iteration inheriting track_1 concept map           │
│  All context self-contained — no external dependencies  │
└─────────────────────────────────────────────────────────┘
```

---

## 7. Precision rules and constraints (what not to do)

**Never score from vocabulary alone.** A person who uses "idempotency" correctly in a
sentence they wrote is a weak signal. A person who asks "is the retry path actually
called?" before being told about ISSUE-3 is a strong signal. Behavioral evidence > linguistic evidence.

**Never collapse dimensions.** A composite score hides the most actionable information.
A practitioner who is 90/100 on distributed systems and 30/100 on ML fundamentals has
completely different needs than one at 60/60.

**Never skip the explanation step.** Multiple-choice alone allows pattern-matching without
understanding. The explanation requirement is the primary defense against surface-level
performance masking deep gaps.

**Never remove the calibration check.** A practitioner who is wrong but thinks they are
right is a higher-risk gap than one who is wrong and knows it. Metacognitive calibration
is a first-class skill in the ML industry — practitioners make deployment decisions based
on their confidence in their own understanding.

**Honor the ceiling.** When a practitioner consistently scores above 85% at Bloom's L4–L5,
stop asking those questions. Move to L6 (Create) or cross-dimension questions. Staying
at a mastered level wastes the session and erodes engagement.

---

## 8. Future: the creative/application layer (not yet)

The creative layer is deliberately deferred because it requires:
1. A reliable profile across all dimensions (need 3+ sessions)
2. Novel scenario generation (harder to validate for correctness)
3. Evaluation rubric for open-ended design work (requires multi-turn dialogue)

When activated, the format shifts from Q&A to:
- "Design X given constraint Y" → evaluated on tradeoff awareness, not correctness
- "Critique this architecture" → evaluated on specificity and mechanism-level reasoning
- "What would break if..." → evaluated on failure-mode depth

This maps to the ultimate goal: not just knowing the industry, but having the judgment
to make novel decisions in it.

---

## 9. Roadmap: Conversation analyzer — fourth evidence source

**Status:** Planned (Sprint 2+)
**Blocks:** Richer question calibration; adaptive framing; misconception-targeted distractors

### The gap

The current system triangulates from three evidence sources (§1). All three capture
**what the learner knows** at varying depths. None capture **how the learner thinks** —
their reasoning patterns, the analogies they reach for, the misconceptions they hold
structurally (not just on individual questions).

The `conversations/` directory holds free-form tutoring dialogue between the learner and
the system. This is richer signal than quiz performance: it reveals *cognitive fingerprint*
— reasoning style, curiosity clusters, misconception framings — that structured Q&A cannot
surface. A conversation where the learner consistently frames retrieval as a "database
lookup problem" reveals a mental model gap that no multiple-choice question would catch.

### The fourth evidence source

```
Source 4: free-form conversation   → cognitive fingerprint
          (how you reason, not just what you know and recall)
```

### Implementation: prompt 05 — Conversation Analyzer

**New prompt:** `05_conversation_analyzer.md`

**Input:** `conversations/*.json` — the raw conversation thread(s) for this track,
compiled into a single snapshot. Not the quiz Q&A (that's already in sessions/), but
the free-form tutoring dialogue: the learner's follow-up questions, their pushback,
their analogies, their confusions.

**Output:** `conversation_analysis.json` stored in `context/` (gitignored alongside
other snapshots):

```json
{
  "analyzed_at": "YYYY-MM-DD",
  "source_conversations": ["uuid1.json", "uuid2.json"],
  "reasoning_style": "bottom-up (mechanics before system design) | top-down | example-driven | first-principles",
  "misconception_fingerprint": [
    {
      "concept_index": 17,
      "pattern": "frames chunking as a storage concern, not a retrieval concern",
      "evidence_quote": "...",
      "frequency": "recurring"
    }
  ],
  "curiosity_clusters": [
    {
      "concept_indexes": [18, 19],
      "signal": "asked 3 follow-up questions on hybrid retrieval failure modes — high interest, uncertain mastery"
    }
  ],
  "mental_models_that_clicked": ["analogy between vector similarity and gradient direction worked well"],
  "mental_models_that_failed": ["formal definition of BM25 required 2 re-framings"],
  "distractor_affinities": [
    "tends to conflate embedding model quality with retrieval quality (treats them as one lever)"
  ]
}
```

**How it feeds into question generation:**

The `conversation_analysis.json` is placed in `context/` and included in question
generation (full file, `--- filename ---` separator, same as other context files).
The question generator's system prompt (`generate_questions.go:BuildSystemPrompt`) needs
one added instruction:

> "If context includes a `conversation_analysis.json`, use `reasoning_style` to adapt
> question framing (analogical, scenario-based, or first-principles) and use
> `misconception_fingerprint` to select distractors that target the learner's specific
> misframings — not generic misconceptions for the concept."

**What this adapts (not what concepts are asked, but how):**

- If `reasoning_style = "example-driven"` → wrap scenarios in concrete examples before
  asking the conceptual question
- If `misconception_fingerprint` shows "conflates embedding quality with retrieval quality"
  → distractors in chunking/retrieval MCQs specifically probe this boundary
- If `curiosity_clusters` shows high interest in hybrid retrieval → prioritize that
  concept cluster slightly above what `bloom_gap` alone would suggest

### Conversation snapshotting workflow

When a track's conversation threads have accumulated enough signal (suggested: 2+ sessions
of tutoring dialogue):

1. Identify relevant conversation UUIDs from `conversations/` — those linked to this track
2. Run prompt 05 against those conversation files
3. Output: `context/conversation_analysis.json` (gitignored)
4. On next session: question generator automatically picks it up via `LoadInheritedContext`

### Why this is achievable before §8's other features

- The data already exists in `conversations/` — no new infrastructure needed
- It is additive: a new prompt + a new context file, not a replacement of existing prompts
- The injection point (`generate_questions.go` system prompt) is a one-line addition
- The conversation analysis file follows the exact same `context/` loading pattern as
  existing snapshots — `LoadInheritedContext` already handles it

### Alignment with Axon's core mission

Axon's goal is not just tracking mastery scores — it is building a progressively accurate
model of how a specific learner thinks, so that every subsequent question is as precisely
calibrated as possible. The conversation analyzer closes the loop between free-form
learning (where real reasoning happens) and structured assessment (where it is measured).
It makes the tutoring conversations a first-class input to the adaptive engine, not just
a side channel.
