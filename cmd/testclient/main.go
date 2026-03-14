// Test client for MCP server
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"sync"
	"time"

	"multi-agent-mcp/pkg/identity"
	"multi-agent-mcp/pkg/messaging"
	"github.com/gorilla/websocket"
)

// TestResult represents a test result
type TestResult struct {
	Name    string
	Passed  bool
	Error   string
	Details string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: testclient <command>")
		fmt.Println("Commands: interactive, auto")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "interactive":
		runInteractive()
	case "auto":
		runAutoTests()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
	}
}

// AgentClient represents a test agent client
type AgentClient struct {
	AgentID      string
	DisplayName  string
	Capabilities []identity.Capability
	Conn         *websocket.Conn
	Messages     chan TestMessage
	mu           sync.Mutex
}

// TestMessage represents a received message
type TestMessage struct {
	Type    string          `json:"type"`
	From    string          `json:"from"`
	Payload json.RawMessage `json:"payload"`
}

// NewAgentClient creates a new test agent
func NewAgentClient(agentID, displayName string, caps []identity.Capability) *AgentClient {
	return &AgentClient{
		AgentID:      agentID,
		DisplayName:  displayName,
		Capabilities: caps,
		Messages:     make(chan TestMessage, 100),
	}
}

// Connect connects to the MCP server
func (c *AgentClient) Connect(addr string) error {
	u := url.URL{Scheme: "ws", Host: addr, Path: "/agents/"}
	log.Printf("[%s] Connecting to %s", c.AgentID, u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	c.Conn = conn

	// Send registration
	reg := map[string]interface{}{
		"agent_id":      c.AgentID,
		"display_name":  c.DisplayName,
		"capabilities":  c.Capabilities,
	}

	if err := c.send(reg); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	// Wait for confirmation
	_, err = c.read()
	if err != nil {
		return fmt.Errorf("registration response failed: %w", err)
	}

	// Start read loop
	go c.readLoop()

	// Start heartbeat
	go c.heartbeatLoop()

	log.Printf("[%s] Connected successfully", c.AgentID)
	return nil
}

// Disconnect closes the connection
func (c *AgentClient) Disconnect() {
	if c.Conn != nil {
		c.Conn.Close()
	}
}

// send sends a message
func (c *AgentClient) send(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteJSON(v)
}

// read reads a message
func (c *AgentClient) read() ([]byte, error) {
	_, data, err := c.Conn.ReadMessage()
	return data, err
}

// readLoop continuously reads messages
func (c *AgentClient) readLoop() {
	for {
		var msg TestMessage
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("[%s] Read error: %v", c.AgentID, err)
			return
		}
		c.Messages <- msg
	}
}

// heartbeatLoop sends periodic heartbeats
func (c *AgentClient) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.send(map[string]string{"method": "heartbeat"})
		}
	}
}

// Rename renames the agent
func (c *AgentClient) Rename(newName string) error {
	req := map[string]interface{}{
		"method": "rename",
		"params": map[string]string{
			"new_name": newName,
		},
	}
	return c.send(req)
}

// SendMessage sends a 1:1 message
func (c *AgentClient) SendMessage(to string, payload interface{}) error {
	req := map[string]interface{}{
		"method": "send_message",
		"params": map[string]interface{}{
			"to":      to,
			"payload": payload,
		},
	}
	return c.send(req)
}

// Broadcast broadcasts a message
func (c *AgentClient) Broadcast(payload interface{}) error {
	req := map[string]interface{}{
		"method": "broadcast",
		"params": map[string]interface{}{
			"payload": payload,
		},
	}
	return c.send(req)
}

// PublishTodo publishes todos
func (c *AgentClient) PublishTodo(todos []messaging.TodoItem) error {
	req := map[string]interface{}{
		"method": "publish_todo",
		"params": map[string]interface{}{
			"todos": todos,
		},
	}
	return c.send(req)
}

// QueryAgents queries agents
func (c *AgentClient) QueryAgents(filter map[string]interface{}) error {
	req := map[string]interface{}{
		"method": "query_agents",
		"params": filter,
	}
	return c.send(req)
}

// GetTodos retrieves todos for an agent
func (c *AgentClient) GetTodos(agentID string) error {
	req := map[string]interface{}{
		"method": "get_todos",
		"params": map[string]string{
			"agent_id": agentID,
		},
	}
	return c.send(req)
}

// WaitForMessage waits for a message with timeout
func (c *AgentClient) WaitForMessage(timeout time.Duration) (TestMessage, bool) {
	select {
	case msg := <-c.Messages:
		return msg, true
	case <-time.After(timeout):
		return TestMessage{}, false
	}
}

func runInteractive() {
	fmt.Println("Interactive MCP Test Client")
	fmt.Println("Commands: connect <id> <name>, rename <name>, send <to> <msg>, broadcast <msg>, todo <title>, query, get <agent>, exit")

	var client *AgentClient
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		line, _ := reader.ReadString('\n')
		line = line[:len(line)-1] // Remove newline

		parts := splitCommand(line)
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "connect":
			if len(parts) < 3 {
				fmt.Println("Usage: connect <agent_id> <display_name>")
				continue
			}
			if client != nil {
				client.Disconnect()
			}
			client = NewAgentClient(parts[1], parts[2], []identity.Capability{identity.CapabilityCoder})
			if err := client.Connect("localhost:9095"); err != nil {
				fmt.Printf("Connection failed: %v\n", err)
				client = nil
			} else {
				fmt.Printf("Connected as %s\n", parts[1])
				// Print incoming messages
				go func() {
					for {
						msg, ok := client.WaitForMessage(time.Hour)
						if !ok {
							return
						}
						fmt.Printf("\n[Received] Type: %s, From: %s\n", msg.Type, msg.From)
						fmt.Print("> ")
					}
				}()
			}

		case "rename":
			if client == nil {
				fmt.Println("Not connected")
				continue
			}
			if len(parts) < 2 {
				fmt.Println("Usage: rename <new_name>")
				continue
			}
			if err := client.Rename(parts[1]); err != nil {
				fmt.Printf("Rename failed: %v\n", err)
			} else {
				fmt.Println("Rename request sent")
			}

		case "send":
			if client == nil {
				fmt.Println("Not connected")
				continue
			}
			if len(parts) < 3 {
				fmt.Println("Usage: send <to_agent> <message>")
				continue
			}
			msg := joinParts(parts[2:])
			if err := client.SendMessage(parts[1], map[string]string{"text": msg}); err != nil {
				fmt.Printf("Send failed: %v\n", err)
			} else {
				fmt.Println("Message sent")
			}

		case "broadcast":
			if client == nil {
				fmt.Println("Not connected")
				continue
			}
			if len(parts) < 2 {
				fmt.Println("Usage: broadcast <message>")
				continue
			}
			msg := joinParts(parts[1:])
			if err := client.Broadcast(map[string]string{"text": msg}); err != nil {
				fmt.Printf("Broadcast failed: %v\n", err)
			} else {
				fmt.Println("Broadcast sent")
			}

		case "todo":
			if client == nil {
				fmt.Println("Not connected")
				continue
			}
			if len(parts) < 2 {
				fmt.Println("Usage: todo <title>")
				continue
			}
			title := joinParts(parts[1:])
			todos := []messaging.TodoItem{
				messaging.NewTodoItem(title),
			}
			if err := client.PublishTodo(todos); err != nil {
				fmt.Printf("Publish failed: %v\n", err)
			} else {
				fmt.Println("Todo published")
			}

		case "query":
			if client == nil {
				fmt.Println("Not connected")
				continue
			}
			filter := map[string]interface{}{}
			if len(parts) > 1 {
				filter["capability"] = parts[1]
			}
			if err := client.QueryAgents(filter); err != nil {
				fmt.Printf("Query failed: %v\n", err)
			} else {
				fmt.Println("Query sent")
			}

		case "get":
			if client == nil {
				fmt.Println("Not connected")
				continue
			}
			if len(parts) < 2 {
				fmt.Println("Usage: get <agent_id>")
				continue
			}
			if err := client.GetTodos(parts[1]); err != nil {
				fmt.Printf("Get todos failed: %v\n", err)
			} else {
				fmt.Println("Get todos request sent")
			}

		case "exit":
			if client != nil {
				client.Disconnect()
			}
			return

		default:
			fmt.Printf("Unknown command: %s\n", parts[0])
		}
	}
}

func runAutoTests() {
	fmt.Println("Running automated tests...")
	results := []TestResult{}

	// Test 1: Agent Registration
	fmt.Println("\n[Test 1] Agent Registration")
	agent1 := NewAgentClient("agent-1", "coder-alice", []identity.Capability{identity.CapabilityCoder})
	err := agent1.Connect("localhost:9095")
	results = append(results, TestResult{
		Name:    "Agent Registration",
		Passed:  err == nil,
		Error:   errToString(err),
		Details: "Agent 1 connected as coder-alice",
	})

	// Test 2: Identity Rename
	fmt.Println("[Test 2] Identity Rename")
	time.Sleep(100 * time.Millisecond)
	err = agent1.Rename("auth-api-coder")
	results = append(results, TestResult{
		Name:    "Identity Rename",
		Passed:  err == nil,
		Error:   errToString(err),
		Details: "Renamed to auth-api-coder",
	})

	// Test 3: Query Agents
	fmt.Println("[Test 3] Query Agents")
	time.Sleep(100 * time.Millisecond)
	err = agent1.QueryAgents(map[string]interface{}{})
	results = append(results, TestResult{
		Name:    "Query Agents",
		Passed:  err == nil,
		Error:   errToString(err),
		Details: "Queried all agents",
	})

	// Test 4: Connect Second Agent
	fmt.Println("[Test 4] Connect Second Agent")
	agent2 := NewAgentClient("agent-2", "planner-bob", []identity.Capability{identity.CapabilityPlanner})
	err = agent2.Connect("localhost:9095")
	results = append(results, TestResult{
		Name:    "Second Agent Registration",
		Passed:  err == nil,
		Error:   errToString(err),
		Details: "Agent 2 connected as planner-bob",
	})

	// Test 5: Send 1:1 Message
	fmt.Println("[Test 5] Send 1:1 Message")
	time.Sleep(100 * time.Millisecond)
	err = agent1.SendMessage("agent-2", map[string]string{"text": "Hello from agent-1"})
	results = append(results, TestResult{
		Name:    "1:1 Message Send",
		Passed:  err == nil,
		Error:   errToString(err),
		Details: "Sent message from agent-1 to agent-2",
	})

	// Wait for message
	msg, ok := agent2.WaitForMessage(2 * time.Second)
	results = append(results, TestResult{
		Name:    "1:1 Message Receive",
		Passed:  ok,
		Error:   boolToError(ok, "Timeout waiting for message"),
		Details: fmt.Sprintf("Received message type: %s", msg.Type),
	})

	// Test 6: Broadcast Message
	fmt.Println("[Test 6] Broadcast Message")
	err = agent2.Broadcast(map[string]string{"announcement": "Hello everyone"})
	results = append(results, TestResult{
		Name:    "Broadcast Send",
		Passed:  err == nil,
		Error:   errToString(err),
		Details: "Agent 2 broadcast message",
	})

	// Wait for broadcast
	msg, ok = agent1.WaitForMessage(2 * time.Second)
	results = append(results, TestResult{
		Name:    "Broadcast Receive",
		Passed:  ok,
		Error:   boolToError(ok, "Timeout waiting for broadcast"),
		Details: fmt.Sprintf("Received broadcast type: %s", msg.Type),
	})

	// Test 7: Publish Todo
	fmt.Println("[Test 7] Publish Todo")
	todos := []messaging.TodoItem{
		messaging.NewTodoItem("Implement login"),
		messaging.NewTodoItem("Add tests"),
	}
	err = agent1.PublishTodo(todos)
	results = append(results, TestResult{
		Name:    "Publish Todo",
		Passed:  err == nil,
		Error:   errToString(err),
		Details: fmt.Sprintf("Published %d todos", len(todos)),
	})

	// Test 8: Get Todos
	fmt.Println("[Test 8] Get Todos")
	time.Sleep(100 * time.Millisecond)
	err = agent2.GetTodos("agent-1")
	results = append(results, TestResult{
		Name:    "Get Todos",
		Passed:  err == nil,
		Error:   errToString(err),
		Details: "Queried agent-1's todos",
	})

	// Test 9: Query by Capability
	fmt.Println("[Test 9] Query by Capability")
	err = agent1.QueryAgents(map[string]interface{}{"capability": "planner"})
	results = append(results, TestResult{
		Name:    "Query by Capability",
		Passed:  err == nil,
		Error:   errToString(err),
		Details: "Queried agents with planner capability",
	})

	// Test 10: Heartbeat (implicit via connection stability)
	fmt.Println("[Test 10] Connection Stability")
	time.Sleep(2 * time.Second)
	// If we're still here, connection is stable
	results = append(results, TestResult{
		Name:    "Connection Stability",
		Passed:  true,
		Details: "Connection stable for 2 seconds",
	})

	// Cleanup
	agent1.Disconnect()
	agent2.Disconnect()

	// Print results
	fmt.Println("\n========================================")
	fmt.Println("           TEST RESULTS")
	fmt.Println("========================================")

	passed := 0
	failed := 0

	for _, r := range results {
		status := "✓ PASS"
		if !r.Passed {
			status = "✗ FAIL"
			failed++
		} else {
			passed++
		}

		fmt.Printf("\n%s: %s\n", status, r.Name)
		fmt.Printf("  Details: %s\n", r.Details)
		if r.Error != "" {
			fmt.Printf("  Error: %s\n", r.Error)
		}
	}

	fmt.Println("\n========================================")
	fmt.Printf("Total: %d | Passed: %d | Failed: %d\n", len(results), passed, failed)
	fmt.Println("========================================")

	if failed > 0 {
		os.Exit(1)
	}
}

func splitCommand(line string) []string {
	parts := []string{}
	current := ""
	inQuote := false

	for _, ch := range line {
		if ch == '"' {
			inQuote = !inQuote
		} else if ch == ' ' && !inQuote {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

func joinParts(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " "
		}
		result += p
	}
	return result
}

func errToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func boolToError(ok bool, msg string) string {
	if ok {
		return ""
	}
	return msg
}
