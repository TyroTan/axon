// Package filesystem implements the store.Collection interface using JSON files
// on the local filesystem. Each collection is a directory; each document is a
// JSON file named by its identity key.
//
// Filter matching: all filter fields must match the decoded document's exported
// fields (case-insensitive JSON key comparison). Nested keys use dot notation
// in the filter: {"spaced_repetition.interval_days": 3}.
//
// Update operators supported: $set, $inc, $push, $unset.
// Applied by re-encoding the document to a map, mutating, then decoding back.
package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tyrohunt/axon/internal/store"
)

// Collection[T] is a filesystem-backed implementation of store.Collection[T].
// dir is the directory where documents are stored as JSON files.
// keyFn extracts the filename key from a document (without .json extension).
type Collection[T any] struct {
	dir   string
	keyFn func(T) string
	mu    sync.RWMutex
}

// NewCollection creates a new filesystem collection rooted at dir.
// keyFn must return a stable, filesystem-safe string for each document.
// The directory is created if it does not exist.
func NewCollection[T any](dir string, keyFn func(T) string) (*Collection[T], error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("filesystem collection: mkdir %s: %w", dir, err)
	}
	return &Collection[T]{dir: dir, keyFn: keyFn}, nil
}

func (c *Collection[T]) InsertOne(_ context.Context, doc T) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.keyFn(doc)
	path := filepath.Join(c.dir, key+".json")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("filesystem collection: document %q already exists", key)
	}
	return writeJSON(path, doc)
}

func (c *Collection[T]) FindOne(_ context.Context, filter store.Filter) (T, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	docs, err := c.scan(filter)
	if err != nil {
		var zero T
		return zero, err
	}
	if len(docs) == 0 {
		var zero T
		return zero, &store.ErrNotFound{Collection: c.dir, Filter: filter}
	}
	return docs[0], nil
}

func (c *Collection[T]) Find(_ context.Context, filter store.Filter) ([]T, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.scan(filter)
}

func (c *Collection[T]) UpdateOne(_ context.Context, filter store.Filter, update store.Update) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path, doc, err := c.findPath(filter)
	if err != nil {
		return err
	}
	mutated, err := applyUpdate(doc, update)
	if err != nil {
		return err
	}
	return writeJSON(path, mutated)
}

func (c *Collection[T]) FindOneAndUpdate(_ context.Context, filter store.Filter, update store.Update) (T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	path, doc, err := c.findPath(filter)
	if err != nil {
		var zero T
		return zero, err
	}
	mutated, err := applyUpdate(doc, update)
	if err != nil {
		var zero T
		return zero, err
	}
	if err := writeJSON(path, mutated); err != nil {
		var zero T
		return zero, err
	}
	// Decode mutated map back to T for return.
	var result T
	b, _ := json.Marshal(mutated)
	if err := json.Unmarshal(b, &result); err != nil {
		var zero T
		return zero, fmt.Errorf("filesystem collection: decode after update: %w", err)
	}
	return result, nil
}

func (c *Collection[T]) DeleteOne(_ context.Context, filter store.Filter) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path, _, err := c.findPath(filter)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// ─── internal helpers ─────────────────────────────────────────────────────────

func (c *Collection[T]) scan(filter store.Filter) ([]T, error) {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return nil, fmt.Errorf("filesystem collection: readdir %s: %w", c.dir, err)
	}
	var results []T
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var doc T
		b, err := os.ReadFile(filepath.Join(c.dir, e.Name()))
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &doc); err != nil {
			return nil, err
		}
		if matchesFilter(doc, filter) {
			results = append(results, doc)
		}
	}
	return results, nil
}

func (c *Collection[T]) findPath(filter store.Filter) (string, map[string]any, error) {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return "", nil, fmt.Errorf("filesystem collection: readdir %s: %w", c.dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(c.dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return "", nil, err
		}
		var raw map[string]any
		if err := json.Unmarshal(b, &raw); err != nil {
			return "", nil, err
		}
		if matchesFilterMap(raw, filter) {
			return path, raw, nil
		}
	}
	return "", nil, &store.ErrNotFound{Collection: c.dir, Filter: filter}
}

// matchesFilter encodes doc to a map then delegates to matchesFilterMap.
func matchesFilter[T any](doc T, filter store.Filter) bool {
	if len(filter) == 0 {
		return true
	}
	b, _ := json.Marshal(doc)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return matchesFilterMap(m, filter)
}

// matchesFilterMap checks that all filter fields match the document map.
// Supports dot-notation for nested fields: "spaced_repetition.interval_days".
func matchesFilterMap(doc map[string]any, filter store.Filter) bool {
	for k, v := range filter {
		got := nestedGet(doc, strings.Split(k, "."))
		if !jsonEqual(got, v) {
			return false
		}
	}
	return true
}

func nestedGet(m map[string]any, parts []string) any {
	if len(parts) == 0 {
		return nil
	}
	val, ok := m[parts[0]]
	if !ok {
		return nil
	}
	if len(parts) == 1 {
		return val
	}
	sub, ok := val.(map[string]any)
	if !ok {
		return nil
	}
	return nestedGet(sub, parts[1:])
}

// jsonEqual compares two values by their JSON representation.
func jsonEqual(a, b any) bool {
	ba, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ba) == string(bb)
}

// applyUpdate applies MongoDB-style update operators to a document map.
func applyUpdate(doc map[string]any, update store.Update) (map[string]any, error) {
	for op, rawFields := range update {
		fields, ok := rawFields.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("applyUpdate: operator %q value must be map[string]any", op)
		}
		switch op {
		case "$set":
			for k, v := range fields {
				nestedSet(doc, strings.Split(k, "."), v)
			}
		case "$inc":
			for k, v := range fields {
				parts := strings.Split(k, ".")
				cur := nestedGet(doc, parts)
				curF := toFloat64(cur)
				incF := toFloat64(v)
				nestedSet(doc, parts, curF+incF)
			}
		case "$push":
			for k, v := range fields {
				parts := strings.Split(k, ".")
				cur := nestedGet(doc, parts)
				arr, _ := cur.([]any)
				nestedSet(doc, parts, append(arr, v))
			}
		case "$unset":
			for k := range fields {
				nestedUnset(doc, strings.Split(k, "."))
			}
		default:
			return nil, fmt.Errorf("applyUpdate: unsupported operator %q", op)
		}
	}
	return doc, nil
}

func nestedSet(m map[string]any, parts []string, v any) {
	if len(parts) == 1 {
		m[parts[0]] = v
		return
	}
	sub, ok := m[parts[0]].(map[string]any)
	if !ok {
		sub = map[string]any{}
		m[parts[0]] = sub
	}
	nestedSet(sub, parts[1:], v)
}

func nestedUnset(m map[string]any, parts []string) {
	if len(parts) == 1 {
		delete(m, parts[0])
		return
	}
	sub, ok := m[parts[0]].(map[string]any)
	if !ok {
		return
	}
	nestedUnset(sub, parts[1:])
}

func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
