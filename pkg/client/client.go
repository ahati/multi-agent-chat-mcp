// Package client provides the agent client SDK
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"multi-agent-mcp/pkg/common"
	"multi-agent-mcp/pkg/identity"
	"multi-agent-mcp/pkg/interfaces"
	"multi-agent-mcp/pkg/messaging"
)

// AgentClient is the main client for agents connecting to MCP server
// Follows DIP: depends on Connection interface, not concrete transport
type AgentClient struct {
	identity   *Identity
	conn       interfaces.Connection
	msgChan    chan *messaging.Message
	stopChan   chan struct{}
	wg         sync.WaitGroup
	onMessage  MessageHandler
	onConnect  func()
	onDisconnect func()
	mu         sync.RWMutex
}

// MessageHandler processes incoming messages
type MessageHandler func(msg *messaging.Message)

// Config contains client configuration
type Config struct {
	AgentID      string
	DisplayName  string
	Capabilities []identity.Capability
	ServerAddr   string
}

// NewAgentClient creates a new agent client
func NewAgentClient(identity *Identity) *AgentClient {
	return &AgentClient{
		identity:  identity,
		msgChan:   make(chan *messaging.Message, 100),
		stopChan:  make(chan struct{}),
	}
}

// Connect connects to the MCP server
func (c *AgentClient) Connect(ctx context.Context, transport interfaces.Transport, addr string) error {
	// Dial server
	conn, err := transport.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial server: %w", err)
	}

	c.conn = conn

	// Send registration
	reg := struct {
		AgentID      string                `json:"agent_id"`
		DisplayName  string                `json:"display_name"`
		Capabilities []identity.Capability `json:"capabilities"`
	}{
		AgentID:      c.identity.GetAgentID(),
		DisplayName:  c.identity.GetDisplayName(),
		Capabilities: c.identity.GetCapabilities(),
	}

	if err := c.send(reg); err != nil {
		conn.Close()
		return fmt.Errorf("send registration: %w", err)
	}

	// Wait for confirmation
	resp, err := c.read()
	if err != nil {
		conn.Close()
		return fmt.Errorf("read registration response: %w", err)
	}

	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		conn.Close()
		return fmt.Errorf("parse registration response: %w", err)
	}

	if !result.Success {
		conn.Close()
		return fmt.Errorf("registration failed: %s", result.Error)
	}

	// Start goroutines
	c.wg.Add(2)
	go c.readLoop()
	go c.heartbeatLoop()

	// Call connect handler
	c.mu.RLock()
	onConnect := c.onConnect
	c.mu.RUnlock()
	if onConnect != nil {
		onConnect()
	}

	return nil
}

// Disconnect disconnects from the server
func (c *AgentClient) Disconnect() error {
	close(c.stopChan)

	if c.conn != nil {
		c.conn.Close()
	}

	c.wg.Wait()

	return nil
}

// RenameIdentity renames the agent's identity
func (c *AgentClient) RenameIdentity(name string, context *identity.TaskContext) error {
	// Update local identity
	c.identity.Rename(name, context)

	// Send to server
	req := struct {
		Method string `json:"method"`
		Params struct {
			NewName string `json:"new_name"`
		} `json:"params"`
	}{
		Method: "rename",
		Params: struct {
			NewName string `json:"new_name"`
		}{NewName: name},
	}

	return c.send(req)
}

// SendMessage sends a 1:1 message to another agent
func (c *AgentClient) SendMessage(to string, payload any) error {
	req := struct {
		Method string `json:"method"`
		Params struct {
			To      string `json:"to"`
			Payload any    `json:"payload"`
		} `json:"params"`
	}{
		Method: "send_message",
		Params: struct {
			To      string `json:"to"`
			Payload any    `json:"payload"`
		}{To: to, Payload: payload},
	}

	return c.send(req)
}

// Broadcast sends a message to all agents
func (c *AgentClient) Broadcast(payload any) error {
	req := struct {
		Method string `json:"method"`
		Params struct {
			Payload any `json:"payload"`
		} `json:"params"`
	}{
		Method: "broadcast",
		Params: struct {
			Payload any `json:"payload"`
		}{Payload: payload},
	}

	return c.send(req)
}

// QueryAgents queries agents matching a filter
func (c *AgentClient) QueryAgents(filter identity.AgentFilter) ([]identity.AgentInfo, error) {
	req := struct {
		Method string               `json:"method"`
		Params identity.AgentFilter `json:"params"`
	}{
		Method: "query_agents",
		Params: filter,
	}

	if err := c.send(req); err != nil {
		return nil, err
	}

	// Wait for response
	resp, err := c.read()
	if err != nil {
		return nil, err
	}

	var result struct {
		Success bool                `json:"success"`
		Data    []identity.AgentInfo `json:"data"`
		Error   string              `json:"error,omitempty"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("query failed: %s", result.Error)
	}

	return result.Data, nil
}

// PublishTodo publishes the agent's todo list
func (c *AgentClient) PublishTodo(todos []messaging.TodoItem) error {
	req := struct {
		Method string                 `json:"method"`
		Params map[string]interface{} `json:"params"`
	}{
		Method: "publish_todo",
		Params: map[string]interface{}{
			"todos": todos,
		},
	}

	return c.send(req)
}

// GetTodos retrieves another agent's todo list
func (c *AgentClient) GetTodos(agentID string) ([]messaging.TodoItem, error) {
	req := struct {
		Method string `json:"method"`
		Params struct {
			AgentID string `json:"agent_id"`
		} `json:"params"`
	}{
		Method: "get_todos",
		Params: struct {
			AgentID string `json:"agent_id"`
		}{AgentID: agentID},
	}

	if err := c.send(req); err != nil {
		return nil, err
	}

	// Wait for response
	resp, err := c.read()
	if err != nil {
		return nil, err
	}

	var result struct {
		Success bool                 `json:"success"`
		Data    []messaging.TodoItem `json:"data"`
		Error   string               `json:"error,omitempty"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("get todos failed: %s", result.Error)
	}

	return result.Data, nil
}

// SetMessageHandler sets the handler for incoming messages
func (c *AgentClient) SetMessageHandler(handler MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onMessage = handler
}

// SetConnectHandler sets the handler for connection events
func (c *AgentClient) SetConnectHandler(handler func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onConnect = handler
}

// SetDisconnectHandler sets the handler for disconnection events
func (c *AgentClient) SetDisconnectHandler(handler func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onDisconnect = handler
}

// GetIdentity returns the client's identity
func (c *AgentClient) GetIdentity() *Identity {
	return c.identity
}

// send sends a request to the server
func (c *AgentClient) send(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return common.ErrConnection
	}

	return conn.Write(data)
}

// read reads a response from the server
func (c *AgentClient) read() ([]byte, error) {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return nil, common.ErrConnection
	}

	return conn.Read()
}

// readLoop continuously reads messages from the server
func (c *AgentClient) readLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopChan:
			return
		default:
		}

		data, err := c.read()
		if err != nil {
			// Call disconnect handler
			c.mu.RLock()
			onDisconnect := c.onDisconnect
			c.mu.RUnlock()
			if onDisconnect != nil {
				onDisconnect()
			}
			return
		}

		// Parse as message
		var msg messaging.Message
		if err := json.Unmarshal(data, &msg); err == nil {
			// It's a message, pass to handler
			c.mu.RLock()
			handler := c.onMessage
			c.mu.RUnlock()
			if handler != nil {
				handler(&msg)
			}
		}
		// If not a message, it's a response (handled by caller)
	}
}

// heartbeatLoop sends periodic heartbeats
func (c *AgentClient) heartbeatLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			req := struct {
				Method string `json:"method"`
			}{
				Method: "heartbeat",
			}
			c.send(req)
		}
	}
}

// Identity wraps agent identity with local state management
type Identity struct {
	info identity.Identity
	mu   sync.RWMutex
}

// NewIdentity creates a new client identity
func NewIdentity(agentID, displayName string, capabilities []identity.Capability) *Identity {
	return &Identity{
		info: *identity.NewIdentity(agentID, capabilities),
	}
}

// GetAgentID returns the agent ID
func (i *Identity) GetAgentID() string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.info.AgentID
}

// GetDisplayName returns the display name
func (i *Identity) GetDisplayName() string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.info.DisplayName
}

// GetCapabilities returns the capabilities
func (i *Identity) GetCapabilities() []identity.Capability {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.info.Capabilities
}

// GetCurrentTask returns the current task
func (i *Identity) GetCurrentTask() string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.info.CurrentTask
}

// Rename renames the identity
func (i *Identity) Rename(name string, context *identity.TaskContext) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.info.Rename(name, context)
}

// ToIdentity returns a copy of the underlying identity
func (i *Identity) ToIdentity() identity.Identity {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.info
}
