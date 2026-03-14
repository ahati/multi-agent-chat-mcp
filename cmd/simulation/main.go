// Agent conversation simulation
package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

// Colors for output
const (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Gray    = "\033[90m"
)

// Agent represents a simulated agent
type Agent struct {
	ID       string
	Name     string
	Role     string
	Conn     *websocket.Conn
	Messages chan Message
}

// Message represents a received message
type Message struct {
	Type    string          `json:"type"`
	From    string          `json:"from"`
	To      *string         `json:"to"`
	Payload json.RawMessage `json:"payload"`
}

func main() {
	fmt.Println(Cyan + "╔══════════════════════════════════════════════════════════════╗" + Reset)
	fmt.Println(Cyan + "║          Multi-Agent MCP Conversation Simulation             ║" + Reset)
	fmt.Println(Cyan + "╚══════════════════════════════════════════════════════════════╝" + Reset)
	fmt.Println()

	// Create agents
	alice := &Agent{
		ID:       "agent-alice",
		Name:     "alice-coder",
		Role:     "Backend Developer",
		Messages: make(chan Message, 10),
	}

	bob := &Agent{
		ID:       "agent-bob",
		Name:     "bob-reviewer",
		Role:     "Code Reviewer",
		Messages: make(chan Message, 10),
	}

	// Scenario: Alice and Bob collaborate on implementing an auth API
	runScenario(alice, bob)
}

func runScenario(alice, bob *Agent) {
	// Step 1: Both agents connect
	printScene("SCENE 1: Agents Connect to MCP Server")
	printAction(alice.Name, "Connecting to MCP server...")
	connectAgent(alice, "coder")
	printSuccess(alice.Name, "Connected as " + alice.Role)

	printAction(bob.Name, "Connecting to MCP server...")
	connectAgent(bob, "reviewer")
	printSuccess(bob.Name, "Connected as " + bob.Role)
	fmt.Println()

	// Step 2: Alice assumes a task-specific identity
	printScene("SCENE 2: Alice Assumes Task Identity")
	printAction(alice.Name, "Starting work on 'auth-api' task...")
	printAction(alice.Name, "Renaming identity to reflect current task...")
	renameAgent(alice, "auth-api-backend-dev")
	printSuccess(alice.Name, "Renamed to 'auth-api-backend-dev'")
	printDialog(alice.Name, "I'm now focused on the auth API implementation!")
	fmt.Println()

	// Step 3: Alice publishes her todo list
	printScene("SCENE 3: Alice Publishes Todo List")
	printAction(alice.Name, "Publishing my task list...")
	todos := []map[string]interface{}{
		{"id": "1", "title": "Design JWT token structure", "status": "done", "priority": 10},
		{"id": "2", "title": "Implement login endpoint", "status": "in_progress", "priority": 10},
		{"id": "3", "title": "Add password hashing", "status": "todo", "priority": 8},
		{"id": "4", "title": "Write unit tests", "status": "todo", "priority": 7},
	}
	publishTodos(alice, todos)
	printSuccess(alice.Name, "Published 4 tasks")
	fmt.Println()

	// Step 4: Bob discovers Alice and her tasks
	printScene("SCENE 4: Bob Discovers Alice")
	printAction(bob.Name, "Querying available agents...")
	queryAgents(bob)
	printSuccess(bob.Name, "Found Alice working on auth-api")
	printDialog(bob.Name, "Hi Alice! I see you're working on the auth API. Need any help?")
	fmt.Println()

	// Step 5: Bob assumes his role
	printAction(bob.Name, "Renaming for auth-api review task...")
	renameAgent(bob, "auth-api-reviewer")
	printSuccess(bob.Name, "Renamed to 'auth-api-reviewer'")
	fmt.Println()

	// Step 6: Alice gets stuck and asks for help
	printScene("SCENE 6: Alice Requests Help")
	printDialog(alice.Name, "Actually, I'm stuck on the password hashing. What's the best practice?")
	printAction(alice.Name, "Broadcasting help request...")
	helpRequest := map[string]string{
		"request": "help",
		"topic":   "password hashing",
		"urgency": "medium",
	}
	sendBroadcast(alice, helpRequest)
	fmt.Println()

	// Step 7: Bob responds with advice
	printScene("SCENE 7: Bob Responds")
	printDialog(bob.Name, "I recommend using bcrypt with a cost factor of 12. Here's an example...")
	printAction(bob.Name, "Sending detailed response to Alice...")
	advice := map[string]string{
		"advice":  "Use bcrypt with cost 12",
		"example": "bcrypt.hashpw(password, bcrypt.gensalt(12))",
	}
	sendDirectMessage(bob, alice.ID, advice)
	fmt.Println()

	// Step 8: Alice updates her todo
	printScene("SCENE 8: Alice Updates Progress")
	printDialog(alice.Name, "Thanks Bob! That helps a lot. I'll implement that now.")
	printAction(alice.Name, "Updating task status...")
	updatedTodos := []map[string]interface{}{
		{"id": "1", "title": "Design JWT token structure", "status": "done", "priority": 10},
		{"id": "2", "title": "Implement login endpoint", "status": "done", "priority": 10},
		{"id": "3", "title": "Add password hashing (bcrypt cost 12)", "status": "in_progress", "priority": 8},
		{"id": "4", "title": "Write unit tests", "status": "todo", "priority": 7},
	}
	publishTodos(alice, updatedTodos)
	printSuccess(alice.Name, "Updated tasks - 2 done, 1 in progress")
	fmt.Println()

	// Step 9: Bob prepares for review
	printScene("SCENE 9: Bob Prepares for Review")
	printAction(bob.Name, "Fetching Alice's current todo list...")
	getTodos(bob, alice.ID)
	printSuccess(bob.Name, "Retrieved Alice's tasks")
	printDialog(bob.Name, "I'll review your code once you finish the password hashing and tests!")
	fmt.Println()

	// Step 10: Final updates
	printScene("SCENE 10: Task Completion")
	printDialog(alice.Name, "All done! Password hashing implemented and tests passing.")
	printAction(alice.Name, "Publishing final task list...")
	finalTodos := []map[string]interface{}{
		{"id": "1", "title": "Design JWT token structure", "status": "done", "priority": 10},
		{"id": "2", "title": "Implement login endpoint", "status": "done", "priority": 10},
		{"id": "3", "title": "Add password hashing (bcrypt cost 12)", "status": "done", "priority": 8},
		{"id": "4", "title": "Write unit tests", "status": "done", "priority": 7},
	}
	publishTodos(alice, finalTodos)
	printSuccess(alice.Name, "All 4 tasks completed!")
	fmt.Println()

	printDialog(bob.Name, "Great work Alice! I'll start the code review now.")
	fmt.Println()

	// Cleanup
	printScene("SCENE 11: Agents Disconnect")
	printAction(alice.Name, "Disconnecting...")
	alice.Conn.Close()
	printAction(bob.Name, "Disconnecting...")
	bob.Conn.Close()
	fmt.Println()

	fmt.Println(Green + "╔══════════════════════════════════════════════════════════════╗" + Reset)
	fmt.Println(Green + "║              Simulation Complete Successfully!               ║" + Reset)
	fmt.Println(Green + "╚══════════════════════════════════════════════════════════════╝" + Reset)
}

func connectAgent(agent *Agent, capability string) {
	u := url.URL{Scheme: "ws", Host: "localhost:9095", Path: "/agents/"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		panic(err)
	}
	agent.Conn = conn

	// Send registration
	reg := map[string]interface{}{
		"agent_id":     agent.ID,
		"display_name": agent.Name,
		"capabilities": []string{capability},
	}
	conn.WriteJSON(reg)

	// Wait for confirmation
	var resp map[string]interface{}
	conn.ReadJSON(&resp)

	// Start message handler
	go func() {
		for {
			var msg Message
			err := conn.ReadJSON(&msg)
			if err != nil {
				return
			}
			agent.Messages <- msg
		}
	}()
}

func renameAgent(agent *Agent, newName string) {
	req := map[string]interface{}{
		"method": "rename",
		"params": map[string]string{
			"new_name": newName,
		},
	}
	agent.Conn.WriteJSON(req)
	agent.Name = newName
	time.Sleep(100 * time.Millisecond)
}

func publishTodos(agent *Agent, todos []map[string]interface{}) {
	req := map[string]interface{}{
		"method": "publish_todo",
		"params": map[string]interface{}{
			"todos": todos,
		},
	}
	agent.Conn.WriteJSON(req)
	time.Sleep(100 * time.Millisecond)
}

func queryAgents(agent *Agent) {
	req := map[string]interface{}{
		"method": "query_agents",
		"params": map[string]interface{}{},
	}
	agent.Conn.WriteJSON(req)
	time.Sleep(100 * time.Millisecond)
}

func sendDirectMessage(from *Agent, to string, payload map[string]string) {
	req := map[string]interface{}{
		"method": "send_message",
		"params": map[string]interface{}{
			"to":      to,
			"payload": payload,
		},
	}
	from.Conn.WriteJSON(req)
	time.Sleep(100 * time.Millisecond)
}

func sendBroadcast(from *Agent, payload map[string]string) {
	req := map[string]interface{}{
		"method": "broadcast",
		"params": map[string]interface{}{
			"payload": payload,
		},
	}
	from.Conn.WriteJSON(req)
	time.Sleep(100 * time.Millisecond)
}

func getTodos(agent *Agent, targetID string) {
	req := map[string]interface{}{
		"method": "get_todos",
		"params": map[string]string{
			"agent_id": targetID,
		},
	}
	agent.Conn.WriteJSON(req)
	time.Sleep(100 * time.Millisecond)
}

// Output helpers
func printScene(title string) {
	fmt.Println(Yellow + "▶ " + title + Reset)
}

func printAction(agent, action string) {
	fmt.Printf("  %s[%s]%s %s\n", Blue, agent, Reset, Gray+action+Reset)
}

func printDialog(agent, message string) {
	fmt.Printf("  %s[%s]:%s \"%s\"\n", Green, agent, Reset, message)
}

func printSuccess(agent, message string) {
	fmt.Printf("  %s[%s]%s ✓ %s\n", Green, agent, Reset, message)
}
