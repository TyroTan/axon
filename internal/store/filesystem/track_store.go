// Package filesystem — track store.
// Reads the .experiments/ directory structure and exposes tracks as domain objects.
// Track identity is derived from directory names matching "track_N" or "track_N_M".
package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/tyrohunt/axon/internal/domain"
)

var trackDirRe = regexp.MustCompile(`^track_\d+(_\d+)*$`)

// ErrSplitRequired is returned by LoadInheritedContext when total tokens exceed
// the soft limit. The caller must generate a split plan before proceeding.
var ErrSplitRequired = fmt.Errorf("context exceeds soft token limit: split plan required")

// ErrHardLimitExceeded is returned when a single context load would exceed the
// hard limit (e.g. a single file > 300k tokens). Generation is blocked entirely.
var ErrHardLimitExceeded = fmt.Errorf("context exceeds hard token limit: reduce corpus size")

// TrackStore reads and writes track directories under experimentsDir.
type TrackStore struct {
	experimentsDir string
}

func NewTrackStore(experimentsDir string) *TrackStore {
	return &TrackStore{experimentsDir: experimentsDir}
}

// ExperimentsDir exposes the root directory for use by command handlers.
func (s *TrackStore) ExperimentsDir() string { return s.experimentsDir }

// ListTracks returns all tracks sorted by ID, with Children populated.
func (s *TrackStore) ListTracks(_ context.Context) ([]domain.Track, error) {
	entries, err := os.ReadDir(s.experimentsDir)
	if err != nil {
		return nil, fmt.Errorf("track store: readdir: %w", err)
	}

	byID := map[string]*domain.Track{}
	var ids []string

	for _, e := range entries {
		if !e.IsDir() || !trackDirRe.MatchString(e.Name()) {
			continue
		}
		t, err := s.readTrack(e.Name())
		if err != nil {
			return nil, err
		}
		tc := t
		byID[t.ID] = &tc
		ids = append(ids, t.ID)
	}
	sort.Strings(ids)

	// Attach children to parents.
	var roots []domain.Track
	for _, id := range ids {
		t := byID[id]
		parent := parentID(id)
		if parent == "" {
			roots = append(roots, *t)
			continue
		}
		if p, ok := byID[parent]; ok {
			p.Children = append(p.Children, *t)
		} else {
			// Orphaned child (parent track deleted) — surface as root.
			roots = append(roots, *t)
		}
	}
	return roots, nil
}

// GetTrack returns a single track by ID. Returns store.ErrNotFound if missing.
func (s *TrackStore) GetTrack(_ context.Context, id string) (domain.Track, error) {
	dir := filepath.Join(s.experimentsDir, id)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return domain.Track{}, &trackNotFound{id: id}
	}
	return s.readTrack(id)
}

// GetConceptMap reads concept_map.json for a track.
func (s *TrackStore) GetConceptMap(_ context.Context, trackID string) (domain.ConceptMap, error) {
	path := filepath.Join(s.experimentsDir, trackID, "concept_map.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return domain.ConceptMap{}, fmt.Errorf("track store: read concept_map %s: %w", trackID, err)
	}
	var cm domain.ConceptMap
	if err := json.Unmarshal(b, &cm); err != nil {
		return domain.ConceptMap{}, fmt.Errorf("track store: parse concept_map %s: %w", trackID, err)
	}
	return cm, nil
}

// WriteConceptMap persists an updated concept_map.json.
func (s *TrackStore) WriteConceptMap(_ context.Context, trackID string, cm domain.ConceptMap) error {
	path := filepath.Join(s.experimentsDir, trackID, "concept_map.json")
	return writeJSON(path, cm)
}

// CreateTrack creates the track_N directory with a seed README and concept_map.
func (s *TrackStore) CreateTrack(_ context.Context, t domain.Track, cm domain.ConceptMap) error {
	dir := filepath.Join(s.experimentsDir, t.ID)
	if err := os.MkdirAll(filepath.Join(dir, "sessions"), 0o755); err != nil {
		return fmt.Errorf("track store: mkdir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "context"), 0o755); err != nil {
		return fmt.Errorf("track store: mkdir context: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0o755); err != nil {
		return fmt.Errorf("track store: mkdir prompts: %w", err)
	}

	// Write concept_map.json.
	if err := writeJSON(filepath.Join(dir, "concept_map.json"), cm); err != nil {
		return err
	}

	// Write minimal README.
	readme := fmt.Sprintf("# Track: %s\n\nBranches: %s\n\nCreated: %s\n",
		t.ID, strings.Join(t.Branches, " · "), t.CreatedAt.Format("2006-01-02"))
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644)
}

// ─── internal ─────────────────────────────────────────────────────────────────

func (s *TrackStore) readTrack(id string) (domain.Track, error) {
	dir := filepath.Join(s.experimentsDir, id)
	info, err := os.Stat(dir)
	if err != nil {
		return domain.Track{}, fmt.Errorf("track store: stat %s: %w", id, err)
	}

	t := domain.Track{
		ID:        id,
		ParentID:  parentID(id),
		CreatedAt: info.ModTime(),
	}

	// Try to read branches from concept_map.json.
	cmPath := filepath.Join(dir, "concept_map.json")
	if b, err := os.ReadFile(cmPath); err == nil {
		var cm struct {
			MajorBranches []string `json:"major_branches"`
		}
		if json.Unmarshal(b, &cm) == nil {
			t.Branches = cm.MajorBranches
		}
	}
	return t, nil
}

// parentID derives the parent track ID from a child ID.
// "track_1_2" → "track_1", "track_1" → ""
func parentID(id string) string {
	parts := strings.Split(id, "_")
	if len(parts) <= 2 {
		return "" // "track_1" has no parent
	}
	return strings.Join(parts[:len(parts)-1], "_")
}

type trackNotFound struct{ id string }

func (e *trackNotFound) Error() string {
	return fmt.Sprintf("track store: track %q not found", e.id)
}

// NextTrackID returns the next available child track ID for a given parent.
// NextTrackID("track_1")   → "track_1_2" (or _3, _4 if _2 exists)
// NextTrackID("track_1_2") → "track_1_2_2"
// NextTrackID("")           → "track_1" (or _2, ... for root tracks)
func (s *TrackStore) NextTrackID(parentID string) (string, error) {
	entries, err := os.ReadDir(s.experimentsDir)
	if err != nil {
		return "", fmt.Errorf("track store: readdir: %w", err)
	}
	existing := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			existing[e.Name()] = true
		}
	}
	for i := 2; i <= 999; i++ {
		var candidate string
		if parentID == "" {
			candidate = fmt.Sprintf("track_%d", i-1) // root: track_1, track_2, ...
		} else {
			candidate = fmt.Sprintf("%s_%d", parentID, i)
		}
		if !existing[candidate] {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("track store: could not allocate track ID under %q", parentID)
}

// WriteContextFile writes a file into the track's context/ directory.
func (s *TrackStore) WriteContextFile(_ context.Context, trackID, filename, content string) error {
	dir := filepath.Join(s.experimentsDir, trackID, "context")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("track store: mkdir context: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644)
}

// ReadContextFiles returns all files in the track's context/ directory as a map of filename → content.
func (s *TrackStore) ReadContextFiles(_ context.Context, trackID string) (map[string]string, error) {
	dir := filepath.Join(s.experimentsDir, trackID, "context")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("track store: read context %s: %w", trackID, err)
	}
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		out[e.Name()] = string(b)
	}
	return out, nil
}

// ContextBudget is the result of LoadInheritedContext.
type ContextBudget struct {
	// Files is the merged context: filename → content, nearest track wins on collision.
	Files map[string]string
	// TokensUsed is the naive token count of all loaded content.
	TokensUsed int
	// Truncated is true if the limit was hit before all ancestors were loaded.
	Truncated bool
	// TruncatedAt is the track ID where loading was halted (empty if no truncation).
	TruncatedAt string
}

// LoadInheritedContext walks from trackID up to the root, merging context/
// files. Nearest track wins on filename collision (child overrides parent).
// Stops cleanly when the accumulated naive token count would exceed limitTokens.
// Pass limitTokens ≤ 0 to load everything with no limit.
func (s *TrackStore) LoadInheritedContext(ctx context.Context, trackID string, limitTokens int) (ContextBudget, error) {
	// Build the ancestor chain: [trackID, parent, grandparent, ...]
	chain := []string{}
	id := trackID
	for id != "" {
		chain = append(chain, id)
		id = parentID(id)
	}

	merged := map[string]string{}
	used := 0

	for _, tid := range chain {
		files, err := s.ReadContextFiles(ctx, tid)
		if err != nil {
			return ContextBudget{}, fmt.Errorf("load inherited context: %w", err)
		}

		for name, content := range files {
			// Child files already in merged take priority — skip parent's version.
			if _, exists := merged[name]; exists {
				continue
			}
			tokens := (len(content) + 3) / 4 // CountTokensNaive inline
			if limitTokens > 0 && used+tokens > limitTokens {
				return ContextBudget{
					Files:       merged,
					TokensUsed:  used,
					Truncated:   true,
					TruncatedAt: tid,
				}, nil
			}
			merged[name] = content
			used += tokens
		}
	}

	return ContextBudget{Files: merged, TokensUsed: used}, nil
}

// WriteSessionMetadata writes 00_metadata.json for a session.
func (s *TrackStore) WriteSessionMetadata(ctx context.Context, trackID string, sessionNum int, meta domain.SessionMetadata) error {
	b, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("track store: marshal session metadata: %w", err)
	}
	return s.WriteSessionFile(ctx, trackID, sessionNum, "00_metadata.json", b)
}

// ReadSessionMetadata reads 00_metadata.json for a session.
// Returns zero-value SessionMetadata (ShardID="") if the file does not exist.
func (s *TrackStore) ReadSessionMetadata(ctx context.Context, trackID string, sessionNum int) (domain.SessionMetadata, error) {
	b, err := s.ReadSessionFile(ctx, trackID, sessionNum, "00_metadata.json")
	if err != nil || b == nil {
		return domain.SessionMetadata{}, err
	}
	var meta domain.SessionMetadata
	if err := json.Unmarshal(b, &meta); err != nil {
		return domain.SessionMetadata{}, fmt.Errorf("track store: parse session metadata: %w", err)
	}
	return meta, nil
}

// WriteSessionFile writes a file into a session directory.
func (s *TrackStore) WriteSessionFile(_ context.Context, trackID string, sessionNum int, filename string, content []byte) error {
	dir := SessionDir(s.experimentsDir, trackID, sessionNum)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("track store: mkdir session: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, filename), content, 0o644)
}

// ReadSessionFile reads a file from a session directory.
// Returns (nil, nil) when the file does not exist yet — callers treat this as "not generated".
func (s *TrackStore) ReadSessionFile(_ context.Context, trackID string, sessionNum int, filename string) ([]byte, error) {
	path := filepath.Join(SessionDir(s.experimentsDir, trackID, sessionNum), filename)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("track store: read session file %s/%d/%s: %w", trackID, sessionNum, filename, err)
	}
	return b, nil
}

// ─── session directory helpers ────────────────────────────────────────────────

func SessionDir(experimentsDir, trackID string, sessionNumber int) string {
	return filepath.Join(experimentsDir, trackID, "sessions", fmt.Sprintf("session_%03d", sessionNumber))
}

func NextSessionNumber(experimentsDir, trackID string) (int, error) {
	sessDir := filepath.Join(experimentsDir, trackID, "sessions")
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}
	max := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(e.Name(), "session_%d", &n); err == nil && n > max {
			max = n
		}
	}
	return max + 1, nil
}

// ListSessions returns Session stubs for a track (presence-checked, no file content).
func (s *TrackStore) ListSessions(_ context.Context, trackID string) ([]domain.Session, error) {
	sessDir := filepath.Join(s.experimentsDir, trackID, "sessions")
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("track store: list sessions %s: %w", trackID, err)
	}
	var sessions []domain.Session
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(e.Name(), "session_%d", &n); err != nil {
			continue
		}
		dir := filepath.Join(sessDir, e.Name())
		sessions = append(sessions, domain.Session{
			TrackID:        trackID,
			Number:         n,
			CreatedAt:      fileModTime(filepath.Join(dir, "00_profile_snapshot.json")),
			HasProfile:     fileExists(filepath.Join(dir, "00_profile_snapshot.json")),
			HasQuestions:   fileExists(filepath.Join(dir, "01_questions.json")),
			HasResponses:   fileExists(filepath.Join(dir, "02_responses.json")),
			HasEvaluations: fileExists(filepath.Join(dir, "03_evaluations.json")),
			HasSynthesis:   fileExists(filepath.Join(dir, "04_synthesis.json")),
		})
	}
	return sessions, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
