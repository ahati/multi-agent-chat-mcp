// Package taskboard provides shared todo list management
package taskboard

import (
	"context"
	"sync"

	"multi-agent-mcp/pkg/common"
	"multi-agent-mcp/pkg/interfaces"
	"multi-agent-mcp/pkg/messaging"
	"multi-agent-mcp/pkg/storage/memory"
)

// Service manages shared todo lists
// Follows SRP: only responsible for todo list CRDT operations
type Service struct {
	store   *memory.TodoStore
	pubsub  interfaces.Publisher
	subs    map[string][]TodoUpdateHandler
	mu      sync.RWMutex
}

// TodoUpdateHandler is called when todos are updated
type TodoUpdateHandler func(update messaging.TodoUpdatePayload)

// NewService creates a new task board service
func NewService(store *memory.TodoStore, pubsub interfaces.Publisher) *Service {
	return &Service{
		store:  store,
		pubsub: pubsub,
		subs:   make(map[string][]TodoUpdateHandler),
	}
}

// Publish publishes an agent's todo list
// Small function: single responsibility
func (s *Service) Publish(ctx context.Context, agentID string, todos []messaging.TodoItem) error {
	if agentID == "" {
		return common.ErrEmptyName
	}

	// Validate todos
	if err := s.validateTodos(todos); err != nil {
		return err
	}

	// Store todos
	if err := s.store.Set(ctx, agentID, todos); err != nil {
		return err
	}

	// Create update payload
	update := messaging.TodoUpdatePayload{
		AgentID:   agentID,
		Todos:     todos,
		UpdatedAt: common.Now(),
	}

	// Notify subscribers
	s.notifySubscribers(update)

	// Publish event (for other services)
	return s.pubsub.Publish(ctx, update)
}

// Get retrieves an agent's todo list
func (s *Service) Get(ctx context.Context, agentID string) ([]messaging.TodoItem, error) {
	if agentID == "" {
		return nil, common.ErrEmptyName
	}

	todos, err := s.store.Get(ctx, agentID)
	if err != nil {
		if err == common.ErrNotFound {
			return []messaging.TodoItem{}, nil
		}
		return nil, err
	}

	return todos, nil
}

// GetAll retrieves all agents' todo lists
func (s *Service) GetAll(ctx context.Context) (map[string][]messaging.TodoItem, error) {
	agentIDs, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]messaging.TodoItem)
	for _, agentID := range agentIDs {
		todos, err := s.store.Get(ctx, agentID)
		if err != nil {
			continue
		}
		result[agentID] = todos
	}

	return result, nil
}

// Subscribe registers a handler for todo updates
func (s *Service) Subscribe(handler TodoUpdateHandler) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	subID := common.GenerateID(common.PrefixTask)
	s.subs[subID] = append(s.subs[subID], handler)

	return subID
}

// Unsubscribe removes a handler
func (s *Service) Unsubscribe(subID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.subs, subID)
}

// notifySubscribers calls all registered handlers
func (s *Service) notifySubscribers(update messaging.TodoUpdatePayload) {
	s.mu.RLock()
	handlers := make([]TodoUpdateHandler, 0)
	for _, hList := range s.subs {
		handlers = append(handlers, hList...)
	}
	s.mu.RUnlock()

	for _, h := range handlers {
		h(update)
	}
}

// validateTodos validates a list of todos
func (s *Service) validateTodos(todos []messaging.TodoItem) error {
	seenIDs := make(map[string]bool)

	for _, todo := range todos {
		if todo.ID == "" {
			return common.ErrEmptyName
		}

		if seenIDs[todo.ID] {
			return common.ErrAlreadyExists
		}
		seenIDs[todo.ID] = true

		if !todo.Status.IsValid() {
			return common.ErrInvalidState
		}
	}

	return nil
}

// CRDT provides conflict-free replicated data type operations
type CRDT struct {
	// Clock for ordering (simplified - in production use vector clocks)
	clock int64
	mu    sync.Mutex
}

// NewCRDT creates a new CRDT instance
func NewCRDT() *CRDT {
	return &CRDT{}
}

// Merge combines two todo lists using last-write-wins
// Small function: single responsibility - just merge logic
func (c *CRDT) Merge(local, remote []messaging.TodoItem) []messaging.TodoItem {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create map of todos by ID
	byID := make(map[string]messaging.TodoItem)

	// Add local todos
	for _, todo := range local {
		byID[todo.ID] = todo
	}

	// Merge remote todos (last write wins based on timestamps)
	for _, remoteTodo := range remote {
		if existing, exists := byID[remoteTodo.ID]; exists {
			// Keep the one with later timestamp
			if remoteTodo.CreatedAt.After(existing.CreatedAt) {
				byID[remoteTodo.ID] = remoteTodo
			}
		} else {
			byID[remoteTodo.ID] = remoteTodo
		}
	}

	// Convert back to slice
	result := make([]messaging.TodoItem, 0, len(byID))
	for _, todo := range byID {
		result = append(result, todo)
	}

	// Sort by creation time
	sortByTime(result)

	return result
}

// sortByTime sorts todos by creation time (ascending)
func sortByTime(todos []messaging.TodoItem) {
	// Simple bubble sort for small lists
	for i := 0; i < len(todos); i++ {
		for j := i + 1; j < len(todos); j++ {
			if todos[j].CreatedAt.Before(todos[i].CreatedAt) {
				todos[i], todos[j] = todos[j], todos[i]
			}
		}
	}
}

// VectorClock provides distributed ordering
// Simplified version - full implementation would track per-agent counters
type VectorClock struct {
	counters map[string]int64 // agentID -> counter
	mu       sync.RWMutex
}

// NewVectorClock creates a new vector clock
func NewVectorClock() *VectorClock {
	return &VectorClock{
		counters: make(map[string]int64),
	}
}

// Increment increments the counter for an agent
func (vc *VectorClock) Increment(agentID string) int64 {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	vc.counters[agentID]++
	return vc.counters[agentID]
}

// Get returns the counter for an agent
func (vc *VectorClock) Get(agentID string) int64 {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	return vc.counters[agentID]
}

// Merge merges another vector clock
func (vc *VectorClock) Merge(other *VectorClock) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	for agentID, counter := range other.counters {
		if counter > vc.counters[agentID] {
			vc.counters[agentID] = counter
		}
	}
}

// Compare compares two vector clocks
// Returns: -1 if vc < other, 0 if concurrent, 1 if vc > other
func (vc *VectorClock) Compare(other *VectorClock) int {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	hasGreater := false
	hasLess := false

	// Check all agents in both clocks
	allAgents := make(map[string]bool)
	for id := range vc.counters {
		allAgents[id] = true
	}
	for id := range other.counters {
		allAgents[id] = true
	}

	for agentID := range allAgents {
		vcVal := vc.counters[agentID]
		otherVal := other.counters[agentID]

		if vcVal > otherVal {
			hasGreater = true
		} else if vcVal < otherVal {
			hasLess = true
		}
	}

	if hasGreater && !hasLess {
		return 1
	}
	if hasLess && !hasGreater {
		return -1
	}
	return 0
}
