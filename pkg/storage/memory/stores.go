// Package memory provides in-memory storage for agent info
package memory

import (
	"context"
	"sync"
	"time"

	"multi-agent-mcp/pkg/common"
	"multi-agent-mcp/pkg/identity"
	"multi-agent-mcp/pkg/messaging"
)

// AgentInfoStore stores agent information
type AgentInfoStore struct {
	data map[string]identity.AgentInfo
	mu   sync.RWMutex
}

// NewAgentInfoStore creates a new agent info store
func NewAgentInfoStore() *AgentInfoStore {
	return &AgentInfoStore{
		data: make(map[string]identity.AgentInfo),
	}
}

// Get retrieves a value by key
func (s *AgentInfoStore) Get(ctx context.Context, key string) (identity.AgentInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.data[key]
	if !exists {
		return identity.AgentInfo{}, common.ErrNotFound
	}

	return val, nil
}

// Set stores a value by key
func (s *AgentInfoStore) Set(ctx context.Context, key string, value identity.AgentInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = value
	return nil
}

// Delete removes a value by key
func (s *AgentInfoStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}

// List returns all values
func (s *AgentInfoStore) List(ctx context.Context) ([]identity.AgentInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]identity.AgentInfo, 0, len(s.data))
	for _, v := range s.data {
		result = append(result, v)
	}

	return result, nil
}

// TodoStore stores todo lists
type TodoStore struct {
	data map[string][]messaging.TodoItem
	mu   sync.RWMutex
}

// NewTodoStore creates a new todo store
func NewTodoStore() *TodoStore {
	return &TodoStore{
		data: make(map[string][]messaging.TodoItem),
	}
}

// Get retrieves todos by agent ID
func (s *TodoStore) Get(ctx context.Context, key string) ([]messaging.TodoItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.data[key]
	if !exists {
		return []messaging.TodoItem{}, nil
	}

	return val, nil
}

// Set stores todos by agent ID
func (s *TodoStore) Set(ctx context.Context, key string, value []messaging.TodoItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = value
	return nil
}

// List returns all agent IDs
func (s *TodoStore) List(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, 0, len(s.data))
	for k := range s.data {
		result = append(result, k)
	}

	return result, nil
}

// Delete removes todos for an agent
func (s *TodoStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}

// HeartbeatStore tracks agent heartbeats
type HeartbeatStore struct {
	data map[string]time.Time
	mu   sync.RWMutex
}

// NewHeartbeatStore creates a new heartbeat store
func NewHeartbeatStore() *HeartbeatStore {
	return &HeartbeatStore{
		data: make(map[string]time.Time),
	}
}

// Set stores a heartbeat timestamp
func (s *HeartbeatStore) Set(agentID string, timestamp time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[agentID] = timestamp
}

// Get retrieves the last heartbeat
func (s *HeartbeatStore) Get(agentID string) (time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ts, exists := s.data[agentID]
	return ts, exists
}

// Delete removes heartbeat tracking
func (s *HeartbeatStore) Delete(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, agentID)
}

// GetAll returns all tracked heartbeats
func (s *HeartbeatStore) GetAll() map[string]time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]time.Time)
	for k, v := range s.data {
		result[k] = v
	}
	return result
}
