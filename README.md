# Axon

**Adaptive knowledge assessment for ML practitioners — local-first, no platform required.**

Axon builds a behavioral skill profile from your own technical documents, then drives calibrated quiz sessions using Bloom's Taxonomy and a concept prerequisite graph. It tracks not just whether you got the answer right, but whether you knew *why* — and whether your confidence was warranted.

---

## What · Why · How

**What:** A local Go server + browser UI that turns your `.md` files into an adaptive quiz engine. Sessions are file-persisted, concepts are tracked individually, and every run is reproducible.

**Why:** Standard self-assessment is unreliable — you cannot accurately gauge your own blind spots by introspection. Flashcard systems (Anki, Quizlet) track recall, not understanding. Axon separates surface-level pattern matching from genuine mechanism knowledge via explanation scoring and Brier score calibration. It also solves a specific problem: when your documents are co-authored with an LLM, naive corpus analysis overcredits your knowledge. Axon's attribution-corrected profiling accounts for this.

**How:** You define a *track* — a set of 3–5 major knowledge branches (e.g. "RAG Architecture · LLM Systems · ML Fundamentals"). Axon generates a concept map with a prerequisite bottleneck graph. Each quiz session asks questions calibrated to your current Bloom's level per concept, advancing through the graph as mastery is demonstrated. All data lives in plain JSON files you own.

---

## Usage — Top 3 Use Cases

### 1. Build a skill profile from your own technical documents

You have design docs, architecture notes, or post-mortems. Axon reads them, strips out LLM-generated content using attribution heuristics, and produces a per-concept starting profile.

```bash
# Start the server pointing at your .experiments/ directory
AXON_DIR=/path/to/.experiments PORT=3456 go run .

# Open the browser
open http://localhost:3456

# Select your track → "Upload context" → paste your .md files → Generate Profile
```

Output: a `00_profile_snapshot.json` with `bloom_current` estimated per concept, with confidence levels and attribution warnings.

---

### 2. Run an adaptive quiz session

Select a track, start a session. Axon picks 12 questions weighted toward your bottleneck concepts (prerequisites for 3+ other concepts) and targets your zone of proximal development (one Bloom's level above demonstrated).

Each question captures:
- **Answer** (MCQ or free-text)
- **Confidence** (1–5 self-report)
- **Explanation** (your reasoning in plain language)
- **Time** (automatically tracked)

After each answer you get: correctness, explanation score (mechanism accuracy, terminology precision, edge case awareness, generalization quality), Brier score contribution, and error taxonomy (misconception / knowledge gap / careless error / ceiling).

---

### 3. Steer and redo generation

Before regenerating a question set, nudge the difficulty:

```
[Make next questions...]
○ slightly harder     ○ significantly harder
○ slightly easier     ○ significantly easier  
○ focus on [concept]  ○ fewer [branch] questions
Note: ___
```

Every generation run is persisted with a unique `generation_id`. You can compare `Run 1 | Run 2 | Run 3` side by side. Steer history is saved alongside each run.

---

## Getting Started

**Prerequisites:** Go 1.21+, a terminal, a browser.

```bash
git clone git@github.com:TyroTan/axon.git
cd axon
go run . 
# → http://localhost:3456
```

By default the server reads the directory it runs from. Point it at your own experiments folder:

```bash
AXON_DIR=/your/path go run .
```

To use a different port:

```bash
PORT=8080 go run .
```

---

## Track System

A *track* covers 3–5 major knowledge branches. Tracks are versioned by iteration:

| Track ID | Meaning |
|---|---|
| `track_1` | First track. May reference external `.md` files. |
| `track_1_2` | Second iteration of the same topic cluster. Inherits from `track_1`. |
| `track_2` | New topic combination entirely. Self-contained. |

Each track has:
- `concept_map.json` — 32–40 concepts with Bloom's targets, bottleneck flags, and prerequisite graph
- `prompts/` — all 5 LLM prompts as self-contained files (paste into Claude or any capable LLM)
- `context/` — snapshots of the source documents used to generate the starting profile
- `sessions/` — one folder per session with profile snapshot, questions, responses, evaluations, synthesis

---

## Architecture

```
axon/
  main.go                          entry point (AXON_DIR, PORT env vars)
  server/server.go                 composition root — wires all dependencies
  internal/
    domain/types.go                Track, ConceptMap, Concept, Session, Question,
                                   Response, Evaluation, Synthesis, SteerIntent
    store/interface.go             Collection[T] — backend-agnostic, MongoDB-style
                                   FindOne / Find / FindOneAndUpdate / UpdateOne /
                                   InsertOne / DeleteOne
    store/filesystem/              JSON-file implementation (default)
    cqrs/bus.go                    CommandBus + QueryBus — type-safe generic dispatch
    commands/                      one file per command (create track, generate questions, ...)
    queries/                       one file per query (list tracks, get session, ...)
```

The store interface uses MongoDB-style operators (`$set`, `$inc`, `$push`, `$unset`) and dot-notation filter paths. Swapping to real MongoDB requires only a new store implementation — no handler changes.

---

## Documentation

| File | Contents |
|---|---|
| [ROADMAP.md](ROADMAP.md) | Phased delivery plan — what's done, what's next, what's deferred |
| [CHANGELOG.md](CHANGELOG.md) | Version history |
| [how_to.md](how_to.md) | Step-by-step guide for running sessions manually (LLM prompt workflow) |
| [concept_taxonomy.md](concept_taxonomy.md) | How concept maps work — bottleneck detection, Bloom's level update algorithm |
| [plan.md](plan.md) | Original system design — D1–D6 baseline, Brier score, error taxonomy, spaced repetition |
| [track_1/README.md](track_1/README.md) | Track 1 parameters — 4 branches, 32 concepts, cross-branch pairs |

---

## Key Concepts

**Bloom's Taxonomy levels** — every question is tagged L1 (Remember) through L6 (Create). The system advances your `bloom_current` per concept only after 75% accuracy across at least 2 questions at that level.

**Bottleneck concepts** — concepts that are prerequisites for 3+ other concepts. These are prioritized when `bloom_current < 3` because unlocking them advances the entire dependency graph.

**Brier score** — measures calibration: `(confidence/5 - correctness)²`. A practitioner who is confidently wrong is a higher-risk gap than one who is wrong and knows it.

**Attribution correction** — when your documents are co-authored with an LLM, the profiler weights down signal that comes from LLM-generated prose and weights up signal from questions you asked, corrections you made, and decisions you justified before being told the answer.

---

## License

MIT
