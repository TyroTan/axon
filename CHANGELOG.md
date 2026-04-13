# Changelog

All notable changes to Axon are documented here.
Format: [semantic version] — date — description.

---

## [Unreleased]

---

## [0.2.0] — 2026-04-13 — Phase 1: Go server skeleton

### Added
- `go.mod` — module `github.com/tyrohunt/axon`, Fiber v2 dependency
- `main.go` — entry point; `AXON_DIR` env (experiments dir), `PORT` env (default 3456)
- `internal/domain/types.go` — all domain types: `Track`, `ConceptMap`, `Concept`, `Session`, `Question`, `Response`, `Evaluation`, `Synthesis`, `SteerIntent`, `SpacedRepetition`
- `internal/store/interface.go` — backend-agnostic `Collection[T]` interface; MongoDB-style `Filter`/`Update` with `$set`, `$inc`, `$push`, `$unset` operators and dot-notation nested field access; `ErrNotFound` type
- `internal/store/filesystem/collection.go` — JSON-file `Collection[T]` implementation with `sync.RWMutex`
- `internal/store/filesystem/track_store.go` — `TrackStore`: `ListTracks` (parent/child tree), `GetTrack`, `GetConceptMap`, `WriteConceptMap`, `CreateTrack`, `ListSessions`, `NextTrackID`, `NextSessionNumber`
- `internal/cqrs/bus.go` — `CommandBus` and `QueryBus` with type-safe generic `Register`/`RegisterQuery`/`Dispatch`/`Ask` helpers
- `internal/queries/list_tracks.go` — `ListTracksQuery` → `ListTracksResult`
- `internal/queries/get_track.go` — `GetTrackQuery` → `GetTrackResult` (track + concept map + sessions)
- `server/server.go` — composition root: wires stores → buses → Fiber routes
- `ROADMAP.md`, `CHANGELOG.md` — project management docs

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
