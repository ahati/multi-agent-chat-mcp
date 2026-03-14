// Package memory provides in-memory storage implementations
package memory

import (
	"context"
	"sync"

	"multi-agent-mcp/pkg/common"
	"multi-agent-mcp/pkg/interfaces"
)

// Store implements Storer interface with in-memory storage
// Follows DIP: implements Storer interface
type Store[K comparable, V any] struct {
	data map[K]V
	mu   sync.RWMutex
}

// NewStore creates a new in-memory store
func NewStore[K comparable, V any]() *Store[K, V] {
	return &Store[K, V]{
		data: make(map[K]V),
	}
}

// Get retrieves a value by key
func (s *Store[K, V]) Get(ctx context.Context, key K) (V, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.data[key]
	if !exists {
		var zero V
		return zero, common.ErrNotFound
	}

	return val, nil
}

// Set stores a value by key
func (s *Store[K, V]) Set(ctx context.Context, key K, value V) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = value
	return nil
}

// Delete removes a value by key
func (s *Store[K, V]) Delete(ctx context.Context, key K) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}

// List returns all values
func (s *Store[K, V]) List(ctx context.Context) ([]V, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]V, 0, len(s.data))
	for _, v := range s.data {
		result = append(result, v)
	}

	return result, nil
}

// Filter returns values matching the predicate
// Implements FilterableStorer
func (s *Store[K, V]) Filter(ctx context.Context, predicate func(V) bool) ([]V, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]V, 0)
	for _, v := range s.data {
		if predicate(v) {
			result = append(result, v)
		}
	}

	return result, nil
}

// Ensure Store implements the interfaces
var _ interfaces.Storer[string, any] = (*Store[string, any])(nil)
var _ interfaces.FilterableStorer[string, any] = (*Store[string, any])(nil)
