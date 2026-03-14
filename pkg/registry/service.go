// Package registry provides agent registration and discovery
package registry

import (
	"context"
	"sync"
	"time"

	"multi-agent-mcp/pkg/common"
	"multi-agent-mcp/pkg/identity"
	"multi-agent-mcp/pkg/storage/memory"
)

// Service manages agent registration and discovery
// Follows SRP: only responsible for agent presence tracking
type Service struct {
	store    *memory.AgentInfoStore
	index    *capabilityIndex
	heartbeats map[string]time.Time
	mu         sync.RWMutex
	timeout    time.Duration
}

// capabilityIndex provides fast capability-based lookups
type capabilityIndex struct {
	byCap map[identity.Capability][]string // capability -> agentIDs
	mu    sync.RWMutex
}

// NewService creates a new registry service
func NewService(store *memory.AgentInfoStore) *Service {
	return &Service{
		store:      store,
		index:      &capabilityIndex{byCap: make(map[identity.Capability][]string)},
		heartbeats: make(map[string]time.Time),
		timeout:    2 * time.Minute,
	}
}

// Register registers a new agent or updates an existing one (for reconnection)
// Small function: single responsibility
func (s *Service) Register(ctx context.Context, info identity.AgentInfo) error {
	if info.AgentID == "" {
		return common.ErrEmptyName
	}

	// Check if already exists - if so, update instead of error (allows reconnection)
	_, err := s.store.Get(ctx, info.AgentID)
	if err == nil {
		// Agent exists - remove from index first to update capabilities
		oldInfo, _ := s.store.Get(ctx, info.AgentID)
		s.removeFromIndex(oldInfo)
	}

	// Set defaults
	if info.Status == "" {
		info.Status = identity.StatusOnline
	}
	info.LastSeenAt = common.Now()

	// Store agent info (overwrites if exists)
	if err := s.store.Set(ctx, info.AgentID, info); err != nil {
		return err
	}

	// Update index
	s.updateIndex(info)

	// Record heartbeat
	s.recordHeartbeat(info.AgentID)

	return nil
}

// Unregister removes an agent from the registry
func (s *Service) Unregister(ctx context.Context, agentID string) error {
	if agentID == "" {
		return common.ErrEmptyName
	}

	// Get current info to remove from index
	info, err := s.store.Get(ctx, agentID)
	if err != nil {
		return err
	}

	// Remove from store
	if err := s.store.Delete(ctx, agentID); err != nil {
		return err
	}

	// Remove from index
	s.removeFromIndex(info)

	// Remove heartbeat
	s.mu.Lock()
	delete(s.heartbeats, agentID)
	s.mu.Unlock()

	return nil
}

// Get retrieves an agent's information
func (s *Service) Get(ctx context.Context, agentID string) (identity.AgentInfo, error) {
	if agentID == "" {
		return identity.AgentInfo{}, common.ErrEmptyName
	}

	return s.store.Get(ctx, agentID)
}

// Query returns agents matching the filter
func (s *Service) Query(ctx context.Context, filter identity.AgentFilter) ([]identity.AgentInfo, error) {
	agents, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]identity.AgentInfo, 0)
	for _, agent := range agents {
		if filter.Match(agent) {
			result = append(result, agent)
		}
	}

	return result, nil
}

// List returns all registered agents
func (s *Service) List(ctx context.Context) ([]identity.AgentInfo, error) {
	return s.store.List(ctx)
}

// UpdateHeartbeat updates the last seen timestamp for an agent
func (s *Service) UpdateHeartbeat(ctx context.Context, agentID string) error {
	if agentID == "" {
		return common.ErrEmptyName
	}

	// Update heartbeat
	s.recordHeartbeat(agentID)

	// Update last seen in agent info
	info, err := s.store.Get(ctx, agentID)
	if err != nil {
		return err
	}

	info.LastSeenAt = common.Now()
	return s.store.Set(ctx, agentID, info)
}

// UpdateStatus updates an agent's status
func (s *Service) UpdateStatus(ctx context.Context, agentID string, status identity.AgentStatus) error {
	if agentID == "" {
		return common.ErrEmptyName
	}

	info, err := s.store.Get(ctx, agentID)
	if err != nil {
		return err
	}

	info.Status = status
	return s.store.Set(ctx, agentID, info)
}

// GetByCapability returns agents with a specific capability
// Uses index for efficient lookup
func (s *Service) GetByCapability(cap identity.Capability) []string {
	s.index.mu.RLock()
	defer s.index.mu.RUnlock()

	return s.index.byCap[cap]
}

// CheckStaleAgents marks agents as offline if they haven't heartbeated
func (s *Service) CheckStaleAgents(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	cutoff := common.Now().Add(-s.timeout)
	stale := make([]string, 0)

	for agentID, lastBeat := range s.heartbeats {
		if lastBeat.Before(cutoff) {
			stale = append(stale, agentID)
		}
	}
	s.mu.RUnlock()

	// Mark stale agents offline
	for _, agentID := range stale {
		if err := s.UpdateStatus(ctx, agentID, identity.StatusOffline); err != nil {
			return stale, err
		}
	}

	return stale, nil
}

// recordHeartbeat records a heartbeat timestamp
func (s *Service) recordHeartbeat(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.heartbeats[agentID] = common.Now()
}

// updateIndex adds an agent to the capability index
func (s *Service) updateIndex(info identity.AgentInfo) {
	s.index.mu.Lock()
	defer s.index.mu.Unlock()

	for _, cap := range info.Capabilities {
		s.index.byCap[cap] = append(s.index.byCap[cap], info.AgentID)
	}
}

// removeFromIndex removes an agent from the capability index
func (s *Service) removeFromIndex(info identity.AgentInfo) {
	s.index.mu.Lock()
	defer s.index.mu.Unlock()

	for _, cap := range info.Capabilities {
		agents := s.index.byCap[cap]
		for i, id := range agents {
			if id == info.AgentID {
				s.index.byCap[cap] = append(agents[:i], agents[i+1:]...)
				break
			}
		}
	}
}
