# Concept Taxonomy — Framework Design

> Framework design reference for the Axon concept map schema.
> Framework design document: how concept maps are structured, how bottleneck detection works,
> and how Bloom's levels apply at the concept level.

---

## Why concepts, not just topics

Standard quiz systems have topics. This system has **concepts** — a more specific unit:

| Standard topic | Concept (this system) |
|---|---|
| "Machine Learning" | "gradient descent mechanics" |
| "RAG" | "chunking strategy tradeoffs" |
| "LLM" | "latent space activation via vocabulary choice" |

A concept has three properties a topic lacks:
1. **A testable boundary** — you can write a question whose answer distinguishes someone who has it from someone who doesn't
2. **A Bloom's level** — the learner is currently at L1–L6 on this specific concept
3. **Prerequisites** — some concepts require others (bottleneck structure)

---

## Concept map structure

Every track has a `concept_map.json`. Structure:

```json
{
  "track": "track_1",
  "major_branches": ["ML Fundamentals", "ML Math Theory", "RAG Architecture", "LLM Systems"],
  "generated_at": "YYYY-MM-DD",
  "concepts": [
    {
      "index": 0,
      "name": "gradient descent mechanics",
      "branch": "ML Fundamentals",
      "description": "How parameters update via loss gradient; learning rate, momentum, convergence",
      "bloom_current": 1,
      "bloom_target": 4,
      "is_bottleneck": true,
      "prerequisite_indexes": [],
      "unlocks_indexes": [1, 2, 7],
      "spaced_repetition": {
        "next_review": null,
        "interval_days": 0,
        "consecutive_correct": 0
      }
    }
  ]
}
```

### Field definitions

| Field | Type | Meaning |
|---|---|---|
| `index` | int | Stable identifier within this track. Never changes after creation. |
| `name` | string | Short, specific label. Should be a noun phrase, not a question. |
| `branch` | string | Which major branch this belongs to. Concepts can belong to one branch only; cross-branch knowledge is represented by questions that span multiple `concept_indexes`. |
| `description` | string | One sentence: what specifically does mastery of this concept mean? |
| `bloom_current` | 1–6 | The learner's demonstrated Bloom's level on this concept. Starts at 1 (unknown = treat as Remember). Updated by session synthesizer. |
| `bloom_target` | 1–6 | The level you want to reach for this concept. Set at track creation. |
| `is_bottleneck` | bool | True if this concept is a prerequisite for 3+ other concepts. Bottleneck concepts should be prioritized when `bloom_current` < 3. |
| `prerequisite_indexes` | int[] | Concepts that should be at `bloom_current >= 2` before this one is introduced. |
| `unlocks_indexes` | int[] | Concepts that become meaningfully learnable once this one reaches `bloom_current >= 3`. |
| `spaced_repetition` | object | Managed by session synthesizer. Do not edit manually. |
| `exploration_unlocked` | bool | **v2 exploration state.** When true, concept is included in question selection even if prerequisites are unmet. Never affects bloom scoring — evidence state only. Set manually or auto-set when `aspiration_count` reaches threshold (default 2). |
| `aspiration_count` | int | **v2 exploration state.** Increments each time the learner engages above their evidence floor on this concept (curiosity cluster signal, reach question attempt). Threshold → `exploration_unlocked = true`. |

---

## Bottleneck detection

A concept is marked `is_bottleneck = true` if it appears in `prerequisite_indexes` of
3 or more other concepts. The concept map generator (prompt 00) detects this automatically.

Bottleneck concepts dominate question selection when `bloom_current < 3` because unlocking
them advances the entire dependency graph. The question generator gives bottleneck concepts
2× weight in session selection when `bloom_current < bloom_target - 1`.

Example bottleneck graph for ML:

```
gradient descent ─────→ backpropagation ──→ neural network training
        │                                           │
        └──────→ learning rate scheduling           └──→ regularization
        │
        └──────→ optimization landscape analysis
```

`gradient descent` is bottleneck (appears in 3 prerequisite lists). Get it to L3 first.

---

## Cross-branch questions

A question with `concept_indexes: [3, 17]` spans two branches. These are the most valuable
question type for senior practitioners because they test **integration knowledge** — the
ability to reason about how concepts from different domains interact.

Examples:
- `[ML Math Theory: linear algebra] × [RAG: embedding space]` → "Why does L2 normalization matter before cosine similarity?"
- `[ML Fundamentals: overfitting] × [LLM Systems: fine-tuning vs RAG]` → "When does the overfitting risk of fine-tuning make RAG the safer choice?"
- `[Distributed Systems: atomicity] × [LLM Architecture: state machine]` → "Why must step_hash updates be atomic in a multi-worker poller?"

Cross-branch questions appear at Bloom's L4+ only — they require analysis or evaluation.

---

## Bloom's level meaning per concept

When the concept map shows `bloom_current: 3` for a concept, it means:

| Level | What the learner can do with THIS concept |
|---|---|
| 1 | Can recall the name and a one-line definition |
| 2 | Can explain it in their own words without the definition |
| 3 | Can apply it in a standard scenario when explicitly prompted |
| 4 | Can analyze tradeoffs, compare it to alternatives, identify when it fails |
| 5 | Can evaluate whether a design using it is correct and justify the judgment |
| 6 | Can design a novel solution using it as a component under new constraints |

The goal is not to get every concept to L6. Most operational concepts need L4. L5–L6 is for
the concepts most central to your domain (e.g., for this project: `atomic step_hash claims`,
`RAG context budget`, `agent system prompt design`).

---

## Track composition rules

A track must have:
- **Minimum 3 major branches** — prevents single-domain tunnel vision
- **7–15 concepts per branch** — below 7 is too coarse; above 15 the track becomes unwieldy
- **At least 1 bottleneck concept per branch** — gives the question generator a clear priority anchor
- **At least 2 cross-branch question candidates** — concepts from different branches that interact

A track should NOT have:
- Concepts that cannot be tested with a question (too vague: "understand AI")
- Concepts with no clear L3 expression (cannot be applied — likely not a concept, just a fact)
- More than 2 branches from the same parent domain (reduces diversity of reasoning required)

---

## How `bloom_current` updates

After each session, the synthesizer (prompt 04) updates `bloom_current` per concept:

```
questions_at_level_L = [q for q in session if q.concept_index == c and q.bloom_level == L]
correct_at_L = count(q for q in questions_at_level_L if q.correctness >= 0.7)

if correct_at_L >= 2 and (correct_at_L / len(questions_at_level_L)) >= 0.75:
    bloom_current = max(bloom_current, L)   # advance if consistent performance
elif correct_at_L == 0 and len(questions_at_level_L) >= 2:
    bloom_current = max(1, min(bloom_current, L - 1))  # regress if consistent failure
# else: no change (insufficient data)
```

This means `bloom_current` advances conservatively (need 75% accuracy at level L across
at least 2 questions) and regresses only on consistent failure (2+ questions at L all wrong).

---

## Is this framework novel?

**Compared to Anki/SuperMemo:** Those systems are flashcard-level. No Bloom's targeting,
no bottleneck graphs, no cross-domain integration questions, no attribution correction.

**Compared to Khan Academy / Duolingo:** Platform-locked, curated content only, no user-defined
branches, no attribution-aware profile generation.

**Compared to IRT (Item Response Theory):** IRT models item difficulty and learner ability
as a single latent trait. This system models a 6-dimensional skill profile with
concept-level granularity and explicit prerequisite graphs. Much richer, less mathematically
rigorous — appropriate tradeoff for a practitioner development tool.

**The novel combination:**
1. Attribution-corrected profile from collaborative documents (human + LLM)
2. User-defined major branches → LLM-generated concept map (not curated by a publisher)
3. Track isolation with forward context propagation (portable, auditable, no platform dependency)
4. Cross-branch questions at L4+ (tests integration, not just recall)
5. Concept bottleneck prioritization (prerequisite graph shapes question selection)

No known system combines all five. The portability (plain files + any LLM) is particularly
unusual — most adaptive systems require a platform to compute the adaptive logic.
