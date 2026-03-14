// Package claudebridge provides an MCP client for Claude Code integration
package claudebridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client is an MCP client for Claude Code
type Client struct {
	AgentID      string
	DisplayName  string
	Capabilities []string
	ServerAddr   string
	conn         *websocket.Conn
	mu           sync.RWMutex
	messages     chan Message
	ctx          context.Context
	cancel       context.CancelFunc
}

// Message represents an MCP message
type Message struct {
	Type    string          `json:"type"`
	From    string          `json:"from"`
	To      string          `json:"to,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

// NewClient creates a new MCP client for Claude
func NewClient(agentID, displayName string, capabilities []string, serverAddr string) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		AgentID:      agentID,
		DisplayName:  displayName,
		Capabilities: capabilities,
		ServerAddr:   serverAddr,
		messages:     make(chan Message, 100),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Connect connects to the MCP server
func (c *Client) Connect() error {
	u := url.URL{Scheme: "ws", Host: c.ServerAddr, Path: "/agents/"}
	log.Printf("[MCP] Connecting to %s", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	c.conn = conn

	// Send registration
	reg := map[string]interface{}{
		"agent_id":     c.AgentID,
		"display_name": c.DisplayName,
		"capabilities": c.Capabilities,
	}

	if err := c.send(reg); err != nil {
		conn.Close()
		return fmt.Errorf("registration failed: %w", err)
	}

	// Wait for confirmation
	_, err = c.read()
	if err != nil {
		conn.Close()
		return fmt.Errorf("registration response failed: %w", err)
	}

	log.Printf("[MCP] Connected as %s", c.AgentID)

	// Start read loop
	go c.readLoop()

	// Subscribe to relevant message types
	c.Subscribe([]string{"direct", "broadcast", "todo_update", "identity_update"})

	return nil
}

// Disconnect closes the connection
func (c *Client) Disconnect() {
	c.cancel()
	if c.conn != nil {
		c.conn.Close()
	}
}

// SendMessage sends a direct message to another agent
func (c *Client) SendMessage(to string, payload interface{}) error {
	req := map[string]interface{}{
		"method": "send_message",
		"params": map[string]interface{}{
			"to":      to,
			"payload": payload,
		},
	}
	return c.send(req)
}

// Broadcast sends a broadcast message
func (c *Client) Broadcast(payload interface{}) error {
	req := map[string]interface{}{
		"method": "broadcast",
		"params": map[string]interface{}{
			"payload": payload,
		},
	}
	return c.send(req)
}

// Subscribe subscribes to message types
func (c *Client) Subscribe(messageTypes []string) error {
	req := map[string]interface{}{
		"method": "subscribe",
		"params": map[string]interface{}{
			"message_types": messageTypes,
		},
	}
	return c.send(req)
}

// QueryAgents queries agents by capability
func (c *Client) QueryAgents(capability string) ([]AgentInfo, error) {
	filter := map[string]interface{}{}
	if capability != "" {
		filter["capability"] = capability
	}

	req := map[string]interface{}{
		"method": "query_agents",
		"params": filter,
	}

	if err := c.send(req); err != nil {
		return nil, err
	}

	// Note: In a real implementation, you'd wait for the response
	// This is simplified for demonstration
	return nil, nil
}

// PublishTodo publishes a todo list
func (c *Client) PublishTodo(todos []TodoItem) error {
	req := map[string]interface{}{
		"method": "publish_todo",
		"params": map[string]interface{}{
			"todos": todos,
		},
	}
	return c.send(req)
}

// Rename renames the agent
func (c *Client) Rename(newName string) error {
	req := map[string]interface{}{
		"method": "rename",
		"params": map[string]interface{}{
			"new_name": newName,
		},
	}
	return c.send(req)
}

// GetMessages returns the message channel
func (c *Client) GetMessages() <-chan Message {
	return c.messages
}

// Internal methods

func (c *Client) send(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(v)
}

func (c *Client) read() ([]byte, error) {
	_, data, err := c.conn.ReadMessage()
	return data, err
}

func (c *Client) readLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		var msg Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("[MCP] Read error: %v", err)
			return
		}

		c.messages <- msg
	}
}

// AgentInfo represents agent information
type AgentInfo struct {
	AgentID      string   `json:"agent_id"`
	DisplayName  string   `json:"display_name"`
	Capabilities []string `json:"capabilities"`
	Status       string   `json:"status"`
}

// TodoItem represents a todo item
type TodoItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	Priority    int    `json:"priority,omitempty"`
}

// NewTodoItem creates a new todo item
func NewTodoItem(title string) TodoItem {
	return TodoItem{
		ID:     generateID(),
		Title:  title,
		Status: "todo",
	}
}

func generateID() string {
	return fmt.Sprintf("todo-%d", time.Now().UnixNano())
}
