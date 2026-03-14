// Package memory provides in-memory pub/sub implementation
package memory

import (
	"context"
	"sync"

	"multi-agent-mcp/pkg/interfaces"
)

// PubSub implements interfaces.PubSub in-memory
type PubSub struct {
	handlers []interfaces.EventHandler
	mu       sync.RWMutex
}

// NewPubSub creates a new in-memory pub/sub
func NewPubSub() *PubSub {
	return &PubSub{
		handlers: make([]interfaces.EventHandler, 0),
	}
}

// Publish sends an event to all subscribers
func (p *PubSub) Publish(ctx context.Context, event any) error {
	p.mu.RLock()
	handlers := make([]interfaces.EventHandler, len(p.handlers))
	copy(handlers, p.handlers)
	p.mu.RUnlock()

	for _, h := range handlers {
		if err := h(ctx, event); err != nil {
			// Log but continue
			continue
		}
	}

	return nil
}

// Subscribe registers an event handler
func (p *PubSub) Subscribe(handler interfaces.EventHandler) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.handlers = append(p.handlers, handler)
	return nil
}

// Unsubscribe removes an event handler
func (p *PubSub) Unsubscribe(handler interfaces.EventHandler) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, h := range p.handlers {
		// Compare function pointers
		if &h == &handler {
			p.handlers = append(p.handlers[:i], p.handlers[i+1:]...)
			return nil
		}
	}

	return nil
}

// Ensure PubSub implements interface
var _ interfaces.PubSub = (*PubSub)(nil)
