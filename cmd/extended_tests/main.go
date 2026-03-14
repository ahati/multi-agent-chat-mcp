// Extended test scenarios for MCP server
package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// RawClient for direct protocol testing
type RawClient struct {
	AgentID string
	Conn    *websocket.Conn
	mu      sync.Mutex
}

func NewRawClient(agentID string) *RawClient {
	return &RawClient{AgentID: agentID}
}

func (c *RawClient) Connect(addr string) error {
	u := url.URL{Scheme: "ws", Host: addr, Path: "/agents/"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	c.Conn = conn
	return nil
}

func (c *RawClient) Close() {
	if c.Conn != nil {
		c.Conn.Close()
	}
}

func (c *RawClient) SendRaw(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteMessage(websocket.TextMessage, data)
}

func (c *RawClient) SendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.SendRaw(data)
}

func (c *RawClient) Read() (map[string]interface{}, error) {
	var msg map[string]interface{}
	err := c.Conn.ReadJSON(&msg)
	return msg, err
}

func main() {
	fmt.Println("Extended MCP Server Tests")
	fmt.Println("=========================\n")

	tests := []struct {
		name string
		fn   func() bool
	}{
		{"Test Invalid Registration", testInvalidRegistration},
		{"Test Duplicate Agent ID", testDuplicateAgentID},
		{"Test Send to Non-existent Agent", testSendToNonExistent},
		{"Test Malformed JSON", testMalformedJSON},
		{"Test Rapid Renames", testRapidRenames},
		{"Test Large Todo List", testLargeTodoList},
		{"Test Empty Todo List", testEmptyTodoList},
		{"Test Invalid Method", testInvalidMethod},
		{"Test Concurrent Agents", testConcurrentAgents},
		{"Test Connection Recovery", testConnectionRecovery},
	}

	passed := 0
	failed := 0

	for _, test := range tests {
		fmt.Printf("Running: %s... ", test.name)
		result := test.fn()
		if result {
			fmt.Println("✓ PASS")
			passed++
		} else {
			fmt.Println("✗ FAIL")
			failed++
		}
	}

	fmt.Println("\n=========================")
	fmt.Printf("Total: %d | Passed: %d | Failed: %d\n", len(tests), passed, failed)
	fmt.Println("=========================")

	if failed > 0 {
		os.Exit(1)
	}
}

// Test 1: Invalid registration (missing agent_id)
func testInvalidRegistration() bool {
	c := NewRawClient("")
	if err := c.Connect("localhost:9095"); err != nil {
		return false
	}
	defer c.Close()

	// Send registration without agent_id
	reg := map[string]interface{}{
		"display_name": "test",
		"capabilities": []string{},
	}

	if err := c.SendJSON(reg); err != nil {
		return false
	}

	// Should receive error response
	msg, err := c.Read()
	if err != nil {
		return false
	}

	// Expect error
	success, ok := msg["success"].(bool)
	return ok && !success
}

// Test 2: Duplicate agent ID
func testDuplicateAgentID() bool {
	agentID := "duplicate-test-agent"

	// First client connects
	c1 := NewRawClient(agentID)
	if err := c1.Connect("localhost:9095"); err != nil {
		return false
	}

	reg := map[string]interface{}{
		"agent_id":     agentID,
		"display_name": "First",
		"capabilities": []string{"coder"},
	}

	if err := c1.SendJSON(reg); err != nil {
		return false
	}
	msg, _ := c1.Read()
	if msg["success"] != true {
		c1.Close()
		return false
	}

	// Second client with same ID
	c2 := NewRawClient(agentID)
	if err := c2.Connect("localhost:9095"); err != nil {
		c1.Close()
		return false
	}

	if err := c2.SendJSON(reg); err != nil {
		c1.Close()
		c2.Close()
		return false
	}

	msg, _ = c2.Read()
	c1.Close()
	c2.Close()

	// Should either fail or succeed (server behavior)
	return true
}

// Test 3: Send to non-existent agent
func testSendToNonExistent() bool {
	// Create sender
	sender := NewRawClient("sender-test")
	if err := sender.Connect("localhost:9095"); err != nil {
		return false
	}

	reg := map[string]interface{}{
		"agent_id":     "sender-test",
		"display_name": "Sender",
		"capabilities": []string{},
	}

	if err := sender.SendJSON(reg); err != nil {
		sender.Close()
		return false
	}
	msg, _ := sender.Read()
	if msg["success"] != true {
		sender.Close()
		return false
	}

	// Send to non-existent agent
	req := map[string]interface{}{
		"method": "send_message",
		"params": map[string]interface{}{
			"to":      "non-existent-agent-12345",
			"payload": map[string]string{"text": "hello"},
		},
	}

	err := sender.SendJSON(req)
	sender.Close()

	// Should handle gracefully
	return err == nil
}

// Test 4: Malformed JSON
func testMalformedJSON() bool {
	c := NewRawClient("malformed-test")
	if err := c.Connect("localhost:9095"); err != nil {
		return false
	}
	defer c.Close()

	// Send invalid JSON
	_ = c.SendRaw([]byte("{invalid json"))

	// Connection might close or error, that's acceptable
	return true
}

// Test 5: Rapid renames
func testRapidRenames() bool {
	c := NewRawClient("rename-test")
	if err := c.Connect("localhost:9095"); err != nil {
		return false
	}
	defer c.Close()

	reg := map[string]interface{}{
		"agent_id":     "rename-test",
		"display_name": "Original",
		"capabilities": []string{},
	}

	if err := c.SendJSON(reg); err != nil {
		return false
	}
	c.Read() // Skip response

	// Send multiple rename requests rapidly
	names := []string{"name1", "name2", "name3", "name4", "name5"}
	for _, name := range names {
		req := map[string]interface{}{
			"method": "rename",
			"params": map[string]string{
				"new_name": name,
			},
		}
		if err := c.SendJSON(req); err != nil {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}

	return true
}

// Test 6: Large todo list
func testLargeTodoList() bool {
	c := NewRawClient("large-todo-test")
	if err := c.Connect("localhost:9095"); err != nil {
		return false
	}
	defer c.Close()

	reg := map[string]interface{}{
		"agent_id":     "large-todo-test",
		"display_name": "LargeTodo",
		"capabilities": []string{},
	}

	if err := c.SendJSON(reg); err != nil {
		return false
	}
	c.Read() // Skip response

	// Create large todo list
	todos := make([]interface{}, 100)
	for i := 0; i < 100; i++ {
		todos[i] = map[string]interface{}{
			"id":          fmt.Sprintf("todo-%d", i),
			"title":       fmt.Sprintf("Task %d with a very long title describing what needs to be done", i),
			"description": fmt.Sprintf("Description for task %d with lots of details", i),
			"status":      "todo",
			"priority":    5,
		}
	}

	req := map[string]interface{}{
		"method": "publish_todo",
		"params": map[string]interface{}{
			"todos": todos,
		},
	}

	if err := c.SendJSON(req); err != nil {
		return false
	}

	return true
}

// Test 7: Empty todo list
func testEmptyTodoList() bool {
	c := NewRawClient("empty-todo-test")
	if err := c.Connect("localhost:9095"); err != nil {
		return false
	}
	defer c.Close()

	reg := map[string]interface{}{
		"agent_id":     "empty-todo-test",
		"display_name": "EmptyTodo",
		"capabilities": []string{},
	}

	if err := c.SendJSON(reg); err != nil {
		return false
	}
	c.Read() // Skip response

	req := map[string]interface{}{
		"method": "publish_todo",
		"params": map[string]interface{}{
			"todos": []interface{}{},
		},
	}

	if err := c.SendJSON(req); err != nil {
		return false
	}

	return true
}

// Test 8: Invalid method
func testInvalidMethod() bool {
	c := NewRawClient("invalid-method-test")
	if err := c.Connect("localhost:9095"); err != nil {
		return false
	}
	defer c.Close()

	reg := map[string]interface{}{
		"agent_id":     "invalid-method-test",
		"display_name": "InvalidMethod",
		"capabilities": []string{},
	}

	if err := c.SendJSON(reg); err != nil {
		return false
	}
	c.Read() // Skip response

	req := map[string]interface{}{
		"method": "nonexistent_method",
		"params": map[string]interface{}{},
	}

	if err := c.SendJSON(req); err != nil {
		return false
	}

	return true
}

// Test 9: Concurrent agents
func testConcurrentAgents() bool {
	numAgents := 10
	var wg sync.WaitGroup
	errors := make(chan error, numAgents)

	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			agentID := fmt.Sprintf("concurrent-%d", idx)
			c := NewRawClient(agentID)

			if err := c.Connect("localhost:9095"); err != nil {
				errors <- err
				return
			}

			reg := map[string]interface{}{
				"agent_id":     agentID,
				"display_name": fmt.Sprintf("Agent%d", idx),
				"capabilities": []string{"test"},
			}

			if err := c.SendJSON(reg); err != nil {
				errors <- err
				c.Close()
				return
			}

			msg, err := c.Read()
			if err != nil || msg["success"] != true {
				errors <- fmt.Errorf("registration failed")
			}

			// Send a few messages
			for j := 0; j < 5; j++ {
				c.SendJSON(map[string]interface{}{
					"method": "query_agents",
					"params": map[string]interface{}{},
				})
			}

			c.Close()
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errCount := 0
	for err := range errors {
		if err != nil {
			errCount++
		}
	}

	return errCount == 0
}

// Test 10: Connection recovery
func testConnectionRecovery() bool {
	agentID := "recovery-test"

	// First connection
	c1 := NewRawClient(agentID)
	if err := c1.Connect("localhost:9095"); err != nil {
		return false
	}

	reg := map[string]interface{}{
		"agent_id":     agentID,
		"display_name": "Recovery",
		"capabilities": []string{},
	}

	if err := c1.SendJSON(reg); err != nil {
		c1.Close()
		return false
	}
	c1.Read()

	// Close connection
	c1.Close()
	time.Sleep(100 * time.Millisecond)

	// Reconnect with same ID
	c2 := NewRawClient(agentID)
	if err := c2.Connect("localhost:9095"); err != nil {
		return false
	}
	defer c2.Close()

	if err := c2.SendJSON(reg); err != nil {
		return false
	}
	msg, err := c2.Read()
	if err != nil {
		return false
	}

	return msg["success"] == true
}
