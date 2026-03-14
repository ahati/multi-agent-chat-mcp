// Package messaging provides message types and routing for agent communication
package messaging

import (
	"encoding/json"
	"time"

	"multi-agent-mcp/pkg/common"
	"multi-agent-mcp/pkg/identity"
)

// MessageType categorizes messages for routing and handling
type MessageType string

const (
	// TypeDirect is for 1:1 messages
	TypeDirect MessageType = "direct"

	// TypeBroadcast sends to all agents
	TypeBroadcast MessageType = "broadcast"

	// TypeTaskCoordination for task delegation and coordination
	TypeTaskCoordination MessageType = "task_coordination"

	// TypeTodoUpdate for task board updates
	TypeTodoUpdate MessageType = "todo_update"

	// TypeIdentityUpdate for identity change notifications
	TypeIdentityUpdate MessageType = "identity_update"

	// TypeAgentJoined for new agent notifications
	TypeAgentJoined MessageType = "agent_joined"

	// TypeAgentLeft for agent departure notifications
	TypeAgentLeft MessageType = "agent_left"

	// TypeSystem for system messages
	TypeSystem MessageType = "system"

	// TypePing for heartbeat requests
	TypePing MessageType = "ping"

	// TypePong for heartbeat responses
	TypePong MessageType = "pong"
)

// Priority levels for messages
type Priority int

const (
	// PriorityLow for non-urgent messages
	PriorityLow Priority = 1

	// PriorityNormal for standard messages
	PriorityNormal Priority = 5

	// PriorityHigh for important messages
	PriorityHigh Priority = 10

	// PriorityUrgent for critical messages
	PriorityUrgent Priority = 20
)

// Message is the core message structure for all agent communication
type Message struct {
	ID        string          `json:"id"`         // Unique message ID
	Type      MessageType     `json:"type"`       // Message category
	From      string          `json:"from"`       // Sender AgentID
	To        *string         `json:"to"`         // Recipient AgentID (nil = broadcast)
	TaskID    string          `json:"task_id"`    // Associated task (optional)
	Priority  Priority        `json:"priority"`   // Message priority
	Payload   json.RawMessage `json:"payload"`    // Typed payload
	Timestamp time.Time       `json:"timestamp"`  // When sent
	Metadata  map[string]any  `json:"metadata"`   // Extra context
}

// NewMessage creates a new message with defaults
func NewMessage(from string, msgType MessageType) *Message {
	return &Message{
		ID:        common.GenerateMessageID(),
		Type:      msgType,
		From:      from,
		Priority:  PriorityNormal,
		Timestamp: common.Now(),
		Metadata:  make(map[string]any),
	}
}

// SetPayload serializes and sets the payload
func (m *Message) SetPayload(v any) error {
	return m.setPayloadFromBytes(common.MustMarshalJSON(v))
}

// setPayloadFromBytes sets payload from pre-serialized data
func (m *Message) setPayloadFromBytes(data []byte) error {
	m.Payload = data
	return nil
}

// GetPayload deserializes the payload into the provided value
func (m *Message) GetPayload(v any) error {
	return common.UnmarshalJSON(m.Payload, v)
}

// IsBroadcast returns true if this is a broadcast message
func (m *Message) IsBroadcast() bool {
	return m.To == nil || *m.To == ""
}

// SetRecipient sets the recipient for direct messages
func (m *Message) SetRecipient(to string) {
	m.To = &to
}

// SetBroadcast clears the recipient for broadcast messages
func (m *Message) SetBroadcast() {
	m.To = nil
}

// SetPriority sets the message priority
func (m *Message) SetPriority(p Priority) {
	m.Priority = p
}

// SetTaskID associates the message with a task
func (m *Message) SetTaskID(taskID string) {
	m.TaskID = taskID
}

// DirectPayload for 1:1 messages
type DirectPayload struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Data    any    `json:"data,omitempty"`
}

// TaskCoordinationPayload for task delegation and coordination
type TaskCoordinationPayload struct {
	Action      string         `json:"action"`      // REQUEST_HELP, DELEGATE, BLOCKED, COMPLETE, ACCEPT, REJECT, UPDATE
	TaskID      string         `json:"task_id"`
	TaskName    string         `json:"task_name"`
	Description string         `json:"description"`
	Role        string         `json:"role"`        // Suggested role for this task
	Priority    Priority       `json:"priority"`
	Data        map[string]any `json:"data"`        // Task-specific data
	Reason      string         `json:"reason"`      // For BLOCKED, REJECT
}

// IsValidAction checks if the action is valid
func (p *TaskCoordinationPayload) IsValidAction() bool {
	validActions := []string{"REQUEST_HELP", "DELEGATE", "BLOCKED", "COMPLETE", "ACCEPT", "REJECT", "UPDATE"}
	for _, a := range validActions {
		if p.Action == a {
			return true
		}
	}
	return false
}

// TodoUpdatePayload for sharing todo list updates
type TodoUpdatePayload struct {
	AgentID   string     `json:"agent_id"`
	Todos     []TodoItem `json:"todos"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// TodoItem represents a single task in an agent's todo list
type TodoItem struct {
	ID          string         `json:"id"`
	TaskID      string         `json:"task_id"`      // Links to a task
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Status      TodoStatus     `json:"status"`
	Priority    Priority       `json:"priority"`
	CreatedAt   time.Time      `json:"created_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Metadata    map[string]any `json:"metadata"`
}

// NewTodoItem creates a new todo item with defaults
func NewTodoItem(title string) TodoItem {
	return TodoItem{
		ID:        common.GenerateTodoID(),
		Title:     title,
		Status:    TodoStatusTodo,
		Priority:  PriorityNormal,
		CreatedAt: common.Now(),
		Metadata:  make(map[string]any),
	}
}

// MarkInProgress marks the todo as in progress
func (t *TodoItem) MarkInProgress() {
	t.Status = TodoStatusInProgress
	now := common.Now()
	t.StartedAt = &now
}

// MarkDone marks the todo as completed
func (t *TodoItem) MarkDone() {
	t.Status = TodoStatusDone
	now := common.Now()
	t.CompletedAt = &now
}

// MarkBlocked marks the todo as blocked with a reason
func (t *TodoItem) MarkBlocked(reason string) {
	t.Status = TodoStatusBlocked
	t.Metadata["block_reason"] = reason
}

// IsDone returns true if the todo is done or cancelled
func (t *TodoItem) IsDone() bool {
	return t.Status == TodoStatusDone || t.Status == TodoStatusCancelled
}

// TodoStatus represents the status of a todo item
type TodoStatus string

const (
	// TodoStatusTodo is the initial status
	TodoStatusTodo TodoStatus = "todo"

	// TodoStatusInProgress when actively working
	TodoStatusInProgress TodoStatus = "in_progress"

	// TodoStatusBlocked when waiting on something
	TodoStatusBlocked TodoStatus = "blocked"

	// TodoStatusDone when completed
	TodoStatusDone TodoStatus = "done"

	// TodoStatusCancelled when no longer needed
	TodoStatusCancelled TodoStatus = "cancelled"
)

// IsValid checks if the status is valid
func (s TodoStatus) IsValid() bool {
	switch s {
	case TodoStatusTodo, TodoStatusInProgress, TodoStatusBlocked, TodoStatusDone, TodoStatusCancelled:
		return true
	}
	return false
}

// IdentityUpdatePayload for identity change notifications
type IdentityUpdatePayload struct {
	AgentID   string                `json:"agent_id"`
	OldName   string                `json:"old_name"`
	NewName   string                `json:"new_name"`
	Context   *identity.TaskContext `json:"context,omitempty"`
	UpdatedAt time.Time             `json:"updated_at"`
}

// AgentLifecyclePayload for join/leave notifications
type AgentLifecyclePayload struct {
	AgentInfo identity.AgentInfo `json:"agent_info"`
	Timestamp time.Time          `json:"timestamp"`
	Reason    string             `json:"reason,omitempty"`
}

// SystemPayload for system messages
type SystemPayload struct {
	Level   string `json:"level"`   // info, warn, error
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// RequestHelpPayload for requesting assistance
type RequestHelpPayload struct {
	TaskID           string              `json:"task_id"`
	Description      string              `json:"description"`
	NeededCapability identity.Capability `json:"needed_capability"`
	Urgency          Priority            `json:"urgency"`
}

// HandlerFunc is a function that handles messages
type HandlerFunc func(msg *Message) error
