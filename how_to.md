# How to Use This Learning System

> Entry point for the Axon adaptive learning system.
> All track data, sessions, and context snapshots are VCS-tracked in this repo.

---

## What this system is

A domain-agnostic, adaptive knowledge assessment and growth framework built on:

- **Bloom's Revised Taxonomy** for cognitive level targeting
- **Concept bottleneck graphs** for identifying prerequisite knowledge gaps
- **Attribution-corrected profiling** — separating what you know from what the LLM said
- **Spaced repetition** for durable retention across sessions
- **Track isolation** — every track is a self-contained folder, portable and reproducible

It is deliberately not a platform. It is markdown files + JSON + LLM prompts. No account,
no subscription, no lock-in. You run it by pasting prompts into Claude (or any capable LLM).

---

## Folder structure

```
axon/                      ← this repo (was .experiments/ — now a standalone project)
  how_to.md                ← this file
  concept_taxonomy.md      ← framework design: how concept maps work
  experiments_log.md       ← session-level log across all tracks
  plan.md                  ← system architecture and roadmap

  prompts/                 ← generic/framework prompts (D1-D6 notation, domain-agnostic)
    01_profile_from_docs.md
    02_question_generator.md
    03_response_evaluator.md
    04_session_synthesizer.md

  track_1/                 ← ML Fundamentals + Math Theory + RAG + LLM Systems
    README.md              ← what this track covers, which external sources it snapshots
    concept_map.json       ← concept legend: index → { name, branch, bloom_current, is_bottleneck }
    context/               ← snapshots of external .md files this track depends on
      _sources.md          ← list of what was snapshotted, at which commit/date
    prompts/               ← track-specific copies of all prompts
      00_concept_map_generator.md
      01_profile_from_docs.md
      02_question_generator.md
      03_response_evaluator.md
      04_session_synthesizer.md
    sessions/
      session_001/
        00_profile_snapshot.json
        01_questions.json
        02_responses.md
        03_evaluations.json
        04_synthesis.json

  track_1_2/               ← second iteration of track 1 (example; use fork button)
    context/               ← snapshot of track_1 context + session syntheses
    concept_map.json       ← extended from track_1's map (new concepts may be added)
    ...

  track_2/                 ← new topic combination (example; use clone or blank)
    ...
```

---

## Track naming convention

| Name | Meaning |
|---|---|
| `track_1` | First track. May reference external `.md` files via `context/` snapshots. |
| `track_1_2` | Second iteration of track 1's topics. Inherits context from `track_1`. No outside dependencies. |
| `track_1_3` | Third iteration. Self-contained. Parent: `track_1_2`. |
| `track_2` | Entirely new topic combination. Self-contained from inception. |
| `track_2_2` | Second iteration of track 2. |

The `_n` suffix means "nth session batch on the same topic group." A new base number means
a genuinely new major-branch combination was chosen.

---

## Step-by-step: starting a new track

### Step 1 — Define major branches (you do this)

Choose 3–5 major bodies of knowledge. These can be anything:

```
Track 1 example: ML Fundamentals · ML Math Theory · RAG Architecture · LLM Systems
Track 2 example: Systems Design · Database Internals · Distributed Consensus · Cloud Economics
Track 3 example: Cognitive Psychology · Pedagogy · Curriculum Design · Assessment Theory
```

### Step 2 — Generate the concept map (prompt 00)

Paste `prompts/00_concept_map_generator.md` as system prompt.  
User message: `"Generate a concept map for: [your major branches]"`  
Output: `concept_map.json` — your track's concept legend.

### Step 3 — Snapshot context sources

If this track depends on external documents (e.g., `BROWSER_CLI_PARITY.md`):
- Copy them into `context/`
- Record name, source path, and date in `context/_sources.md`

If this is `track_1_n` (not the first track), copy the prior track's `concept_map.json`
and `sessions/*/04_synthesis.json` into this track's `context/` instead.

### Step 4 — Generate your starting profile (prompt 01)

Paste `prompts/01_profile_from_docs.md` as system prompt.  
User message: paste the contents of all files in `context/`  
Output: `sessions/session_001/00_profile_snapshot.json`

### Step 5 — Generate questions (prompt 02)

Paste `prompts/02_question_generator.md` as system prompt.  
User message: paste `sessions/session_001/00_profile_snapshot.json` + `concept_map.json`  
Output: `sessions/session_001/01_questions.json`

### Step 6 — Answer each question

For each question:
1. Read the question. Start a timer.
2. Record your `selected_answer`, `confidence` (1–5), `explanation` (your own words), `time_seconds`.
3. Paste question + your response to prompt 03 to get evaluation.
4. Save evaluation to `03_evaluations.json` (array, one entry per question).

Format for your response (copy this):
```json
{
  "question_id": "q001",
  "selected_answer": "B",
  "confidence": 3,
  "explanation": "<your explanation in plain language>",
  "time_seconds": 74
}
```

### Step 7 — Session synthesis (prompt 04)

At the end of the session:  
Paste `prompts/04_session_synthesizer.md` as system prompt.  
User message: `00_profile_snapshot.json` + `01_questions.json` + `03_evaluations.json`  
Output: `04_synthesis.json` — updated profile, analytics, next-session plan.

Copy the one-line log entry from `04_synthesis.json` into `experiments_log.md`.

### Step 8 — Next session

Use `04_synthesis.json` as input to Step 5 of the next session (replaces the profile snapshot).

---

## Starting track_1_2 (second iteration)

When a track has completed 3+ sessions and the synthesis consistently shows the same
weak dimensions, it is time to start a fresh iteration:

1. Use the Fork button on track_1 (creates `track_1_2`)
2. Copy `track_1/concept_map.json` to `track_1_2/context/track_1_concept_map.json`
3. Copy `track_1/sessions/*/04_synthesis.json` files to `track_1_2/context/`
4. Generate a new profile from `context/` contents using prompt 01
5. Optionally extend the concept map (add new sub-topics if the prior sessions revealed gaps)
6. Proceed from Step 5

---

## Analytics you can track manually

After 3+ sessions, look for these patterns in your `04_synthesis.json` files:

| Pattern | What it means | Action |
|---|---|---|
| Same dimension stuck below 50 after 2 sessions | Knowledge gap, not recall gap | Need foundational content, not more quizzes |
| Brier score > 0.20 consistently | Overconfident in gaps | Add confidence calibration as explicit practice |
| Error taxonomy shows same misconception 2+ sessions | Persistent wrong mental model | Targeted explanation needed (not more MCQs) |
| Bloom's level not advancing after 3 sessions | Questions not hard enough, or ceiling | Move to L6 (Create) questions or cross-branch |
| One dimension advances fast, another stalls | Uneven depth profile | Deliberately increase question ratio for stalled dimension |

---

## Novelty note

This framework differs from existing adaptive learning systems in three ways that matter:

1. **Attribution-aware profiling** — it accounts for the fact that your `.md` files were
   co-authored with an LLM. Standard systems assume all input signal comes from the learner.

2. **Track isolation with forward propagation** — tracks don't share a live database. Context
   propagates by explicit snapshot, making the system auditable, reproducible, and portable.

3. **Cross-branch questions** — questions that intentionally span two major branches test
   integration knowledge, not just within-domain recall. This is what separates practitioners
   who can apply knowledge in novel situations from those who can only recall it.
