# Experiments Log

> Session-level log for the Axon adaptive learning system.
> All track data, sessions, and context snapshots are VCS-tracked in this repo.
> Raw session data lives in `track_N/sessions/`.

---

## Assessment System

**Goal:** Build practitioner-level mastery across targeted concept clusters using
Bloom's Taxonomy-driven adaptive quiz sessions. Each track covers 3–5 major branches
with 32–40 concepts. Track completion advances the concept map's bloom_current scores.

**System design:** See `.experiments/how_to.md` for the step-by-step session guide.
**Concept model:** See `.experiments/concept_taxonomy.md` for how concepts, bottlenecks,
and bloom_current updates work.
**Framework plan:** See `.experiments/plan.md` for the original D1–D6 baseline and
analytics layer design (still valid — concept indexes are the track-specific refinement).

---

## Track Registry

| Track | Branches | Concepts | Status | Started |
|---|---|---|---|---|
| `track_1` | ML Fundamentals · ML Math Theory · RAG Architecture · LLM Systems | 32 | active | 2026-04-12 |
| `track_2` | ML Fundamentals · ML Math Theory · RAG Architecture · LLM Systems | 32 | active (interview prep — Upwork RAG roles) | 2026-04-19 |

---

## Track 1 — Baseline Profile

**Assessed:** 2026-04-12 (pre-quiz, from corpus analysis)
**Method:** Attribution-corrected analysis of `BROWSER_CLI_PARITY.md`, `agent_state_machine.md`,
`debug_usage.md`. See `plan.md` §2 and `concept_taxonomy.md` for attribution rules.

Concept coverage from corpus — high-signal concepts only:

| Concept index | Name | Estimated bloom_current | Confidence |
|---|---|---|---|
| 22 | context window mechanics | 3 | high |
| 27 | agentic loop design | 3 | high |
| 28 | multi-agent orchestration | 2 | medium |
| 26 | tool use and function calling | 2 | medium |
| 14 | chunking strategies | 2 | medium |
| 29 | model capability calibration | 2 | medium |
| 0–13 | ML Fundamentals + Math Theory | 1 | low (no corpus signal) |
| 15–21 | RAG Architecture (non-chunking) | 1 | low (no corpus signal) |

**All other concepts:** bloom_current = 1, confidence = low (no corpus signal)

---

## Session Log

| Date | Track | Session | Questions | Accuracy | Brier | Concepts Advanced | Top insight |
|---|---|---|---|---|---|---|---|
| _(first quiz session will appear here)_ | | | | | | | |

---

## Persistent Misconceptions (updated each session)

_(none yet — populated after first quiz session)_

---

## Milestone targets

| Milestone | Target | Status |
|---|---|---|
| Track 1 concept map seeded | 32 concepts, all bloom_current = 1 | 2026-04-12 ✓ |
| Track 1 baseline profile | corpus analysis complete | 2026-04-12 ✓ |
| Track 1 first session | 12 questions, ≥3 bottleneck concepts tested | pending |
| Track 1 bottleneck unlocked | any bottleneck concept reaches bloom_current = 3 | pending |
| Track 1 complete | all concept bloom_current >= bloom_target - 1 | pending |
| Track 1_2 started | 3+ sessions done, same weak cluster persists | pending |
