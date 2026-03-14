// Package identity provides dynamic identity management for agents
package identity

import (
	"encoding/json"
	"fmt"
	"time"
)

// Capability represents what an agent can do
type Capability string

const (
	CapabilityCoder     Capability = "coder"
	CapabilityPlanner   Capability = "planner"
	CapabilityReviewer  Capability = "reviewer"
	CapabilityDeployer  Capability = "deployer"
	CapabilityDatabase  Capability = "database"
	CapabilityTesting   Capability = "testing"
	CapabilityDevOps    Capability = "devops"
	CapabilityAI        Capability = "ai"
)

// Identity represents an agent's current identity in the mesh
// The DisplayName can change dynamically based on the task context
type Identity struct {
	AgentID      string            `json:"agent_id"`       // Permanent UUID
	DisplayName  string            `json:"display_name"`   // Dynamic: "auth-api-coder", "hotfix-agent"
	CurrentTask  string            `json:"current_task"`   // Task ID currently working on
	Capabilities []Capability      `json:"capabilities"`   // What this agent can do
	Status       AgentStatus       `json:"status"`         // online, busy, away, offline
	Metadata     map[string]any    `json:"metadata"`       // Task-specific context
	Version      string            `json:"version"`        // Agent software version
	RenamedAt    time.Time         `json:"renamed_at"`     // When identity last changed
	ConnectedAt  time.Time         `json:"connected_at"`   // When agent connected
	LastSeenAt   time.Time         `json:"last_seen_at"`   // Last heartbeat
}

// AgentStatus represents the current status of an agent
type AgentStatus string

const (
	StatusOnline   AgentStatus = "online"
	StatusBusy     AgentStatus = "busy"
	StatusAway     AgentStatus = "away"
	StatusOffline  AgentStatus = "offline"
)

// TaskContext provides context for identity renaming
type TaskContext struct {
	TaskID      string            `json:"task_id"`
	TaskName    string            `json:"task_name"`
	Role        string            `json:"role"`
	Description string            `json:"description"`
	Priority    int               `json:"priority"`
	StartedAt   time.Time         `json:"started_at"`
}

// NewIdentity creates a new identity with the given agent ID
func NewIdentity(agentID string, capabilities []Capability) *Identity {
	now := time.Now()
	return &Identity{
		AgentID:      agentID,
		DisplayName:  agentID, // Default to agent ID
		Capabilities: capabilities,
		Status:       StatusOnline,
		Metadata:     make(map[string]any),
		Version:      "1.0.0",
		RenamedAt:    now,
		ConnectedAt:  now,
		LastSeenAt:   now,
	}
}

// Rename updates the identity based on task context
func (i *Identity) Rename(name string, context *TaskContext) {
	i.DisplayName = name
	i.RenamedAt = time.Now()
	if context != nil {
		i.CurrentTask = context.TaskID
		i.Metadata["task_context"] = context
	}
}

// UpdateHeartbeat updates the last seen timestamp
func (i *Identity) UpdateHeartbeat() {
	i.LastSeenAt = time.Now()
}

// SetStatus updates the agent status
func (i *Identity) SetStatus(status AgentStatus) {
	i.Status = status
}

// HasCapability checks if the agent has a specific capability
func (i *Identity) HasCapability(cap Capability) bool {
	for _, c := range i.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// String returns a human-readable representation
func (i *Identity) String() string {
	return fmt.Sprintf("%s (%s) [%s] - Task: %s", i.DisplayName, i.AgentID, i.Status, i.CurrentTask)
}

// ToJSON serializes identity to JSON
func (i *Identity) ToJSON() ([]byte, error) {
	return json.Marshal(i)
}

// IdentityFromJSON deserializes identity from JSON
func IdentityFromJSON(data []byte) (*Identity, error) {
	var id Identity
	err := json.Unmarshal(data, &id)
	return &id, err
}

// AgentInfo is a lightweight version for registry listings
type AgentInfo struct {
	AgentID      string       `json:"agent_id"`
	DisplayName  string       `json:"display_name"`
	Status       AgentStatus  `json:"status"`
	Capabilities []Capability `json:"capabilities"`
	CurrentTask  string       `json:"current_task"`
	LastSeenAt   time.Time    `json:"last_seen_at"`
}

// ToAgentInfo converts Identity to AgentInfo
func (i *Identity) ToAgentInfo() AgentInfo {
	return AgentInfo{
		AgentID:      i.AgentID,
		DisplayName:  i.DisplayName,
		Status:       i.Status,
		Capabilities: i.Capabilities,
		CurrentTask:  i.CurrentTask,
		LastSeenAt:   i.LastSeenAt,
	}
}

// AgentFilter is used to query agents from the registry
type AgentFilter struct {
	Capability  Capability `json:"capability,omitempty"`
	Status      AgentStatus `json:"status,omitempty"`
	HasTask     bool        `json:"has_task,omitempty"`     // true = has task, false = no task
	TaskID      string      `json:"task_id,omitempty"`      // Agents working on specific task
	NotBusy     bool        `json:"not_busy,omitempty"`     // Agents with status online (not busy)
}

// Match checks if an agent matches the filter
func (f *AgentFilter) Match(info AgentInfo) bool {
	if f.Capability != "" {
		found := false
		for _, c := range info.Capabilities {
			if c == f.Capability {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if f.Status != "" && info.Status != f.Status {
		return false
	}

	if f.TaskID != "" && info.CurrentTask != f.TaskID {
		return false
	}

	if f.HasTask && info.CurrentTask == "" {
		return false
	}

	if !f.HasTask && info.CurrentTask != "" {
		return false
	}

	if f.NotBusy && info.Status == StatusBusy {
		return false
	}

	return true
}
