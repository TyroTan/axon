// Package metrics provides file-persisted event logging with in-memory aggregation.
// Events are appended as JSONL to experiments_dir/metrics.jsonl.
// Each event carries a timestamp, track/session context, and event-specific fields.
// GET /api/metrics returns aggregated counts + recent events.
package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event is a single observability record appended to metrics.jsonl.
type Event struct {
	Timestamp  time.Time      `json:"ts"`
	Event      string         `json:"event"`
	TrackID    string         `json:"track_id,omitempty"`
	SessionNum int            `json:"session_num,omitempty"`
	Tokens     int64          `json:"tokens,omitempty"`
	Extra      map[string]any `json:"extra,omitempty"`
}

// Recorder writes events to disk and maintains in-memory counters.
type Recorder struct {
	mu        sync.Mutex
	path      string    // absolute path to metrics.jsonl
	startedAt time.Time
	counts    map[string]int64 // event name → count (rebuilt from file on start)
}

// New returns a Recorder that appends to experimentsDir/metrics.jsonl.
// Existing events are replayed into counts so the endpoint is accurate after restart.
func New(experimentsDir string) (*Recorder, error) {
	path := filepath.Join(experimentsDir, "metrics.jsonl")
	r := &Recorder{
		path:      path,
		startedAt: time.Now(),
		counts:    map[string]int64{},
	}
	if err := r.replayFromDisk(); err != nil {
		return nil, fmt.Errorf("metrics: replay: %w", err)
	}
	return r, nil
}

// Record appends an event to disk and increments the in-memory counter.
func (r *Recorder) Record(ev Event) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counts[ev.Event]++
	// Best-effort disk write — never block a caller on I/O failure.
	if b, err := json.Marshal(ev); err == nil {
		f, err := os.OpenFile(r.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			f.Write(append(b, '\n'))
			f.Close()
		}
	}
}

// Snapshot returns aggregated counts and the most recent events (up to limit).
func (r *Recorder) Snapshot(recentLimit int) map[string]any {
	r.mu.Lock()
	counts := make(map[string]int64, len(r.counts))
	for k, v := range r.counts {
		counts[k] = v
	}
	r.mu.Unlock()

	recent := r.readRecentEvents(recentLimit)
	return map[string]any{
		"uptime_seconds": int64(time.Since(r.startedAt).Seconds()),
		"counts":         counts,
		"recent":         recent,
	}
}

// replayFromDisk reads existing metrics.jsonl and populates in-memory counts.
func (r *Recorder) replayFromDisk() error {
	b, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return nil // first run, no file yet
	}
	if err != nil {
		return err
	}
	lines := splitLines(b)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var ev Event
		if json.Unmarshal(line, &ev) == nil {
			r.counts[ev.Event]++
		}
	}
	return nil
}

// readRecentEvents returns the last N events from disk without holding the lock.
func (r *Recorder) readRecentEvents(n int) []Event {
	b, err := os.ReadFile(r.path)
	if err != nil {
		return nil
	}
	lines := splitLines(b)
	// Take the last n lines.
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	out := make([]Event, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var ev Event
		if json.Unmarshal(line, &ev) == nil {
			out = append(out, ev)
		}
	}
	return out
}

// splitLines splits a byte slice on newlines without allocating a string.
func splitLines(b []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, c := range b {
		if c == '\n' {
			lines = append(lines, b[start:i])
			start = i + 1
		}
	}
	if start < len(b) {
		lines = append(lines, b[start:])
	}
	return lines
}
