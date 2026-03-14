// Package messaging provides message routing and queue management
package messaging

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"multi-agent-mcp/pkg/identity"
	"multi-agent-mcp/pkg/interfaces"
)

// Router handles message routing between agents
// Follows SRP: only responsible for routing messages
type Router struct {
	registry   RegistryQuery            // DIP: depends on interface
	conns      map[string]interfaces.Connection // agentID -> connection
	handlers   map[MessageType][]HandlerFunc
	offlineQ   map[string][]*Message    // agentID -> queued messages
	mu         sync.RWMutex
	maxOffline int                      // Max messages to queue per offline agent
}

// RegistryQuery provides read-only access to agent registry (ISP)
type RegistryQuery interface {
	Get(ctx context.Context, agentID string) (identity.AgentInfo, error)
	List(ctx context.Context) ([]identity.AgentInfo, error)
}

// AgentConnectionInfo contains minimal agent information needed for routing
type AgentConnectionInfo struct {
	AgentID string
	Online  bool
}

// NewRouter creates a new message router
func NewRouter(registry RegistryQuery) *Router {
	return &Router{
		registry:   registry,
		conns:      make(map[string]interfaces.Connection),
		handlers:   make(map[MessageType][]HandlerFunc),
		offlineQ:   make(map[string][]*Message),
		maxOffline: 1000,
	}
}

// RegisterConnection registers an agent's connection for direct routing
func (r *Router) RegisterConnection(agentID string, conn interfaces.Connection) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.conns[agentID] = conn

	// Deliver any queued messages
	r.deliverQueued(agentID, conn)
}

// UnregisterConnection removes an agent's connection
func (r *Router) UnregisterConnection(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.conns, agentID)
}

// RegisterHandler adds a handler for a message type
func (r *Router) RegisterHandler(msgType MessageType, handler HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[msgType] = append(r.handlers[msgType], handler)
}

// Route routes a message to its destination
// Small function: delegates to specific routing methods
func (r *Router) Route(ctx context.Context, msg *Message) error {
	if msg == nil {
		return fmt.Errorf("cannot route nil message")
	}

	if err := r.validate(msg); err != nil {
		return err
	}

	if msg.IsBroadcast() {
		return r.routeBroadcast(ctx, msg)
	}

	return r.routeDirect(ctx, msg)
}

// validate checks if a message is valid
func (r *Router) validate(msg *Message) error {
	if msg.From == "" {
		return fmt.Errorf("message missing sender")
	}
	if msg.Type == "" {
		return fmt.Errorf("message missing type")
	}
	return nil
}

// routeDirect routes a message to a specific agent
func (r *Router) routeDirect(ctx context.Context, msg *Message) error {
	if msg.To == nil {
		return fmt.Errorf("direct message missing recipient")
	}

	targetID := *msg.To

	// Get connection
	r.mu.RLock()
	conn, exists := r.conns[targetID]
	r.mu.RUnlock()

	if !exists {
		return r.queueOffline(targetID, msg)
	}

	return r.send(conn, msg)
}

// routeBroadcast routes a message to all connected agents
func (r *Router) routeBroadcast(ctx context.Context, msg *Message) error {
	r.mu.RLock()
	conns := make([]interfaces.Connection, 0, len(r.conns))
	for _, conn := range r.conns {
		conns = append(conns, conn)
	}
	r.mu.RUnlock()

	var lastErr error
	for _, conn := range conns {
		if err := r.send(conn, msg); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// send writes a message to a connection
func (r *Router) send(conn interfaces.Connection, msg *Message) error {
	data, err := msg.Payload.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	return conn.Write(data)
}

// queueOffline stores a message for later delivery
func (r *Router) queueOffline(agentID string, msg *Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	queue := r.offlineQ[agentID]
	if len(queue) >= r.maxOffline {
		return fmt.Errorf("offline queue full for agent %s", agentID)
	}

	r.offlineQ[agentID] = append(queue, msg)
	return nil
}

// deliverQueued sends queued messages to a reconnected agent
func (r *Router) deliverQueued(agentID string, conn interfaces.Connection) {
	queue := r.offlineQ[agentID]
	if len(queue) == 0 {
		return
	}

	// Sort by priority (highest first) and timestamp
	sort.Slice(queue, func(i, j int) bool {
		if queue[i].Priority != queue[j].Priority {
			return queue[i].Priority > queue[j].Priority
		}
		return queue[i].Timestamp.Before(queue[j].Timestamp)
	})

	// Send messages
	for _, msg := range queue {
		if err := r.send(conn, msg); err != nil {
			// Log but continue
			continue
		}
	}

	// Clear queue
	delete(r.offlineQ, agentID)
}

// HandleMessage processes an incoming message through registered handlers
func (r *Router) HandleMessage(msg *Message) error {
	r.mu.RLock()
	handlers := r.handlers[msg.Type]
	r.mu.RUnlock()

	for _, h := range handlers {
		if err := h(msg); err != nil {
			return err
		}
	}

	return nil
}

// MessageStore provides durable message storage (optional)
type MessageStore interface {
	Store(ctx context.Context, msg *Message) error
	GetPending(ctx context.Context, agentID string, since time.Time) ([]*Message, error)
	MarkDelivered(ctx context.Context, msgID string) error
}
