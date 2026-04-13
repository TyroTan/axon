// Package store defines the backend-agnostic persistence interface.
// Implementations: filesystem (default), MongoDB (future).
//
// The Filter and Update types mirror MongoDB semantics intentionally —
// swapping to a real MongoDB implementation requires only a new impl, no handler changes.
package store

import "context"

// Filter expresses a query predicate as field → value pairs.
// All specified fields must match (implicit AND).
// Example: Filter{"track_id": "track_1", "session": 1}
type Filter map[string]any

// Update expresses a mutation using operator keys.
// Supported operators:
//   - "$set":   map[string]any  — set field values
//   - "$inc":   map[string]any  — increment numeric fields
//   - "$push":  map[string]any  — append to array fields
//   - "$unset": map[string]any  — remove fields
//
// Example: Update{"$set": map[string]any{"bloom_current": 3}}
type Update map[string]any

// Collection is a generic, backend-agnostic persistence interface.
// T must be a JSON-serializable struct type.
type Collection[T any] interface {
	// InsertOne persists a new document. Returns error if a document with the
	// same identity already exists (identity is implementation-defined).
	InsertOne(ctx context.Context, doc T) error

	// FindOne returns the first document matching filter.
	// Returns ErrNotFound if no document matches.
	FindOne(ctx context.Context, filter Filter) (T, error)

	// Find returns all documents matching filter. Returns empty slice (not error)
	// if no documents match.
	Find(ctx context.Context, filter Filter) ([]T, error)

	// UpdateOne applies update to the first document matching filter.
	// Returns ErrNotFound if no document matches.
	UpdateOne(ctx context.Context, filter Filter, update Update) error

	// FindOneAndUpdate atomically finds the first matching document, applies update,
	// and returns the document AFTER the update (post-image).
	// Returns ErrNotFound if no document matches.
	FindOneAndUpdate(ctx context.Context, filter Filter, update Update) (T, error)

	// DeleteOne removes the first document matching filter.
	// Returns ErrNotFound if no document matches.
	DeleteOne(ctx context.Context, filter Filter) error
}

// ErrNotFound is returned when a query matches no documents.
type ErrNotFound struct {
	Collection string
	Filter     Filter
}

func (e *ErrNotFound) Error() string {
	return "store: not found in " + e.Collection
}

// IsNotFound returns true if err is an ErrNotFound.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrNotFound)
	return ok
}
