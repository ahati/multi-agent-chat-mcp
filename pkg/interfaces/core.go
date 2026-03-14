// Package interfaces defines core abstractions following DIP and ISP principles
package interfaces

import (
	"context"
	"time"
)

// Connection abstracts a network connection
// Implemented by: WebSocketConn, TCPConn, MockConn (LSP)
type Connection interface {
	// Read reads a message from the connection
	Read() ([]byte, error)

	// Write writes a message to the connection
	Write([]byte) error

	// Close closes the connection
	Close() error

	// RemoteAddr returns the remote address
	RemoteAddr() string
}

// Transport abstracts network transport
// Implemented by: WebSocketTransport, MockTransport (OCP, LSP)
type Transport interface {
	// Listen starts listening on the given address
	Listen(addr string) (Listener, error)

	// Dial connects to the given address
	Dial(addr string) (Connection, error)
}

// Listener abstracts connection acceptance
type Listener interface {
	// Accept accepts a new connection
	Accept() (Connection, error)

	// Close stops listening
	Close() error
}

// Storer abstracts key-value storage
// Implemented by: MemoryStorage, MockStorage (OCP, LSP)
type Storer[K comparable, V any] interface {
	// Get retrieves a value by key
	Get(ctx context.Context, key K) (V, error)

	// Set stores a value by key
	Set(ctx context.Context, key K, value V) error

	// Delete removes a value by key
	Delete(ctx context.Context, key K) error

	// List returns all values
	List(ctx context.Context) ([]V, error)
}

// FilterableStorer extends Storer with filtering capability
type FilterableStorer[K comparable, V any] interface {
	Storer[K, V]

	// Filter returns values matching the predicate
	Filter(ctx context.Context, predicate func(V) bool) ([]V, error)
}

// Publisher abstracts event publishing
// Used for decoupling services (DIP, ISP)
type Publisher interface {
	// Publish sends an event to subscribers
	Publish(ctx context.Context, event any) error
}

// Subscriber abstracts event subscription
type Subscriber interface {
	// Subscribe registers a handler for events
	Subscribe(handler EventHandler) error

	// Unsubscribe removes a handler
	Unsubscribe(handler EventHandler) error
}

// EventHandler processes events
type EventHandler func(ctx context.Context, event any) error

// PubSub combines Publisher and Subscriber
type PubSub interface {
	Publisher
	Subscriber
}

// Querier provides read-only access (ISP)
type Querier[K comparable, V any] interface {
	Get(ctx context.Context, key K) (V, error)
	List(ctx context.Context) ([]V, error)
}

// Writer provides write-only access (ISP)
type Writer[K comparable, V any] interface {
	Set(ctx context.Context, key K, value V) error
	Delete(ctx context.Context, key K) error
}

// HeartbeatMonitor tracks agent heartbeats
type HeartbeatMonitor interface {
	// RecordHeartbeat records a heartbeat from an agent
	RecordHeartbeat(ctx context.Context, agentID string, timestamp time.Time) error

	// GetLastHeartbeat returns the last heartbeat time for an agent
	GetLastHeartbeat(ctx context.Context, agentID string) (time.Time, error)

	// GetStaleAgents returns agents that haven't heartbeated since the given time
	GetStaleAgents(ctx context.Context, since time.Time) ([]string, error)
}

// MessageSender provides message sending capability (ISP)
type MessageSender interface {
	// Send sends a message to a specific agent
	Send(ctx context.Context, toAgentID string, msg []byte) error

	// Broadcast sends a message to all agents
	Broadcast(ctx context.Context, msg []byte) error
}

// MessageReceiver provides message receiving capability (ISP)
type MessageReceiver interface {
	// Receive returns a channel of incoming messages
	Receive(ctx context.Context) (<-chan []byte, error)
}

// Logger abstracts logging (DIP)
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}
