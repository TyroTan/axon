# Changelog

All notable changes to Axon are documented here.
Format: [semantic version] — date — description.

---

## [Unreleased]

### Added
- `ROADMAP.md` — phased delivery plan with status tracking
- `CHANGELOG.md` — this file
- Git initialized as standalone project

---

## [0.1.0] — 2026-04-12 — Foundation scaffold

### Added
- Learning framework design documents:
  - `how_to.md` — step-by-step session guide, folder structure, track naming convention
  - `concept_taxonomy.md` — concept map schema, bottleneck detection algorithm, bloom_current update rules
  - `plan.md` — original D1–D6 baseline design + analytics layer (Brier score, error taxonomy, spaced repetition)
  - `experiments_log.md` — session log (local only)
- Track 1 — ML Fundamentals · ML Math Theory · RAG Architecture · LLM Systems:
  - `track_1/README.md` — track parameters and branch definitions
  - `track_1/concept_map.json` — 32-concept seed map, bloom_current = 1 for all, bottleneck graph
  - `track_1/context/_sources.md` — external source registry (3 files to snapshot before session 1)
  - `track_1/prompts/00_concept_map_generator.md` — generates concept_map.json from branch names
  - `track_1/prompts/01_profile_from_docs.md` — attribution-corrected profile from corpus
  - `track_1/prompts/02_question_generator.md` — concept-graph-aware question generator with `concept_indexes`
  - `track_1/prompts/03_response_evaluator.md` — per-response evaluator with `bloom_level_demonstrated`
  - `track_1/prompts/04_session_synthesizer.md` — bloom_current updater, outputs `concept_map_updates` diff
- Generic prompts in `prompts/` (D1–D6 notation, framework-level, not track-specific)
