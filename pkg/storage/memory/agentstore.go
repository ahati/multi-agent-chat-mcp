// Package memory provides in-memory storage implementations
package memory

import (
	"context"
	"sync"

	"multi-agent-mcp/pkg/common"
)

// AgentStore stores agent info
type AgentStore struct {
	data map[string]any
	mu   sync.RWMutex
}

// NewAgentStore creates a new agent store
func NewAgentStore() *AgentStore {
	return &AgentStore{
		data: make(map[string]any),
	}
}

// Get retrieves a value by key
func (s *AgentStore) Get(ctx context.Context, key string) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.data[key]
	if !exists {
		return nil, common.ErrNotFound
	}

	return val, nil
}

// Set stores a value by key
func (s *AgentStore) Set(ctx context.Context, key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = value
	return nil
}

// Delete removes a value by key
func (s *AgentStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}

// List returns all values
func (s *AgentStore) List(ctx context.Context) ([]any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]any, 0, len(s.data))
	for _, v := range s.data {
		result = append(result, v)
	}

	return result, nil
}

// Filter returns values matching the predicate
func (s *AgentStore) Filter(ctx context.Context, predicate func(any) bool) ([]any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]any, 0)
	for _, v := range s.data {
		if predicate(v) {
			result = append(result, v)
		}
	}

	return result, nil
}
