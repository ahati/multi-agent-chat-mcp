// Package server provides the MCP server implementation
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"multi-agent-mcp/pkg/common"
	"multi-agent-mcp/pkg/identity"
	"multi-agent-mcp/pkg/interfaces"
	"multi-agent-mcp/pkg/messaging"
	"multi-agent-mcp/pkg/registry"
	"multi-agent-mcp/pkg/storage/memory"
	"multi-agent-mcp/pkg/taskboard"
)

// Server is the MCP server that composes all services
// Follows DIP: depends on service interfaces, not implementations
type Server struct {
	transport interfaces.Transport
	listener  interfaces.Listener

	// Services (DIP)
	registry   *registry.Service
	router     *messaging.Router
	taskBoard  *taskboard.Service
	agentStore *memory.AgentInfoStore
	todoStore  *memory.TodoStore

	// Agent connections
	connections map[string]interfaces.Connection
	connMu      sync.RWMutex

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Config contains server configuration
type Config struct {
	Addr string
}

// NewServer creates a new MCP server
// Dependencies are injected for testability (DIP)
func NewServer(
	transport interfaces.Transport,
	agentStore *memory.AgentInfoStore,
	todoStore *memory.TodoStore,
	registry *registry.Service,
	router *messaging.Router,
	taskBoard *taskboard.Service,
) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		transport:   transport,
		agentStore:  agentStore,
		todoStore:   todoStore,
		registry:    registry,
		router:      router,
		taskBoard:   taskBoard,
		connections: make(map[string]interfaces.Connection),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start starts the server
func (s *Server) Start(addr string) error {
	// Start listening
	listener, err := s.transport.Listen(addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.listener = listener

	// Accept connections
	s.wg.Add(1)
	go s.acceptLoop()

	log.Printf("MCP Server started on %s", addr)
	return nil
}

// Stop stops the server
func (s *Server) Stop() error {
	s.cancel()

	// Close all connections
	s.connMu.Lock()
	for _, conn := range s.connections {
		conn.Close()
	}
	s.connections = make(map[string]interfaces.Connection)
	s.connMu.Unlock()

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	// Wait for goroutines
	s.wg.Wait()

	return nil
}

// acceptLoop accepts incoming connections
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if s.ctx.Err() != nil {
				return
			}
			log.Printf("Accept error: %v", err)
			continue
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single agent connection
func (s *Server) handleConnection(conn interfaces.Connection) {
	defer s.wg.Done()

	// Wait for registration message
	data, err := conn.Read()
	if err != nil {
		conn.Close()
		return
	}

	// Parse registration
	var reg RegistrationMessage
	if err := json.Unmarshal(data, &reg); err != nil {
		s.sendError(conn, "invalid registration")
		time.Sleep(100 * time.Millisecond) // Allow client to read error
		conn.Close()
		return
	}

	// Validate
	if reg.AgentID == "" {
		s.sendError(conn, "missing agent ID")
		time.Sleep(100 * time.Millisecond) // Allow client to read error
		conn.Close()
		return
	}

	// Register agent
	info := identity.AgentInfo{
		AgentID:      reg.AgentID,
		DisplayName:  reg.DisplayName,
		Capabilities: reg.Capabilities,
		Status:       identity.StatusOnline,
	}

	if err := s.registry.Register(s.ctx, info); err != nil {
		s.sendError(conn, fmt.Sprintf("registration failed: %v", err))
		conn.Close()
		return
	}

	// Store connection
	s.connMu.Lock()
	s.connections[reg.AgentID] = conn
	s.connMu.Unlock()

	// Register with router
	s.router.RegisterConnection(reg.AgentID, conn)

	// Send success
	s.sendSuccess(conn, "registered")

	log.Printf("Agent registered: %s (%s)", reg.DisplayName, reg.AgentID)

	// Handle messages
	s.handleMessages(reg.AgentID, conn)
}

// handleMessages processes messages from an agent
func (s *Server) handleMessages(agentID string, conn interfaces.Connection) {
	defer func() {
		// Cleanup on disconnect
		s.registry.Unregister(s.ctx, agentID)

		s.connMu.Lock()
		delete(s.connections, agentID)
		s.connMu.Unlock()

		s.router.UnregisterConnection(agentID)

		conn.Close()
		log.Printf("Agent disconnected: %s", agentID)
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		data, err := conn.Read()
		if err != nil {
			return
		}

		// Parse message
		var req Request
		if err := json.Unmarshal(data, &req); err != nil {
			s.sendError(conn, "invalid request")
			continue
		}

		// Handle request
		if err := s.handleRequest(agentID, conn, &req); err != nil {
			s.sendError(conn, err.Error())
		}
	}
}

// handleRequest handles a single request
// Small function: delegates to specific handlers
func (s *Server) handleRequest(agentID string, conn interfaces.Connection, req *Request) error {
	switch req.Method {
	case "rename":
		return s.handleRename(agentID, conn, req.Params)
	case "query_agents":
		return s.handleQueryAgents(conn, req.Params)
	case "send_message":
		return s.handleSendMessage(agentID, req.Params)
	case "broadcast":
		return s.handleBroadcast(agentID, req.Params)
	case "publish_todo":
		return s.handlePublishTodo(agentID, req.Params)
	case "get_todos":
		return s.handleGetTodos(conn, req.Params)
	case "heartbeat":
		return s.handleHeartbeat(agentID)
	default:
		return fmt.Errorf("unknown method: %s", req.Method)
	}
}

// handleRename handles identity rename requests
func (s *Server) handleRename(agentID string, conn interfaces.Connection, params json.RawMessage) error {
	var req RenameRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return err
	}

	// Validate name
	if err := common.ValidateAgentName(req.NewName); err != nil {
		return err
	}

	// Get current info
	info, err := s.registry.Get(s.ctx, agentID)
	if err != nil {
		return err
	}

	// Update name
	info.DisplayName = req.NewName
	if err := s.registry.Register(s.ctx, info); err != nil {
		return err
	}

	// Broadcast identity update
	update := messaging.IdentityUpdatePayload{
		AgentID:   agentID,
		OldName:   info.DisplayName,
		NewName:   req.NewName,
		UpdatedAt: common.Now(),
	}

	msg := messaging.NewMessage(agentID, messaging.TypeIdentityUpdate)
	msg.SetPayload(update)
	s.router.Route(s.ctx, msg)

	return s.sendSuccess(conn, "renamed")
}

// handleQueryAgents handles agent query requests
func (s *Server) handleQueryAgents(conn interfaces.Connection, params json.RawMessage) error {
	var filter identity.AgentFilter
	if err := json.Unmarshal(params, &filter); err != nil {
		filter = identity.AgentFilter{} // Empty filter matches all
	}

	agents, err := s.registry.Query(s.ctx, filter)
	if err != nil {
		return err
	}

	return s.sendResponse(conn, agents)
}

// handleSendMessage handles 1:1 message requests
func (s *Server) handleSendMessage(fromAgentID string, params json.RawMessage) error {
	var req SendMessageRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return err
	}

	msg := messaging.NewMessage(fromAgentID, messaging.TypeDirect)
	msg.SetRecipient(req.To)
	msg.SetPayload(req.Payload)

	return s.router.Route(s.ctx, msg)
}

// handleBroadcast handles broadcast message requests
func (s *Server) handleBroadcast(fromAgentID string, params json.RawMessage) error {
	var req BroadcastRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return err
	}

	msg := messaging.NewMessage(fromAgentID, messaging.TypeBroadcast)
	msg.SetBroadcast()
	msg.SetPayload(req.Payload)

	return s.router.Route(s.ctx, msg)
}

// handlePublishTodo handles todo list publish requests
func (s *Server) handlePublishTodo(agentID string, params json.RawMessage) error {
	var req PublishTodoRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return err
	}

	return s.taskBoard.Publish(s.ctx, agentID, req.Todos)
}

// handleGetTodos handles get todos requests
func (s *Server) handleGetTodos(conn interfaces.Connection, params json.RawMessage) error {
	var req GetTodosRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return err
	}

	todos, err := s.taskBoard.Get(s.ctx, req.AgentID)
	if err != nil {
		return err
	}

	return s.sendResponse(conn, todos)
}

// handleHeartbeat handles heartbeat requests
func (s *Server) handleHeartbeat(agentID string) error {
	return s.registry.UpdateHeartbeat(s.ctx, agentID)
}

// sendSuccess sends a success response
func (s *Server) sendSuccess(conn interfaces.Connection, message string) error {
	resp := Response{Success: true, Message: message}
	data, _ := json.Marshal(resp)
	return conn.Write(data)
}

// sendError sends an error response
func (s *Server) sendError(conn interfaces.Connection, message string) error {
	resp := Response{Success: false, Error: message}
	data, _ := json.Marshal(resp)
	return conn.Write(data)
}

// sendResponse sends a data response
func (s *Server) sendResponse(conn interfaces.Connection, data any) error {
	resp := Response{Success: true, Data: data}
	bytes, _ := json.Marshal(resp)
	return conn.Write(bytes)
}

// Request represents an incoming request
type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// Response represents an outgoing response
type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// RegistrationMessage is the first message from an agent
type RegistrationMessage struct {
	AgentID      string                `json:"agent_id"`
	DisplayName  string                `json:"display_name"`
	Capabilities []identity.Capability `json:"capabilities"`
}

// RenameRequest is a rename request
type RenameRequest struct {
	NewName string `json:"new_name"`
}

// SendMessageRequest is a send message request
type SendMessageRequest struct {
	To      string `json:"to"`
	Payload any    `json:"payload"`
}

// BroadcastRequest is a broadcast request
type BroadcastRequest struct {
	Payload any `json:"payload"`
}

// PublishTodoRequest is a publish todo request
type PublishTodoRequest struct {
	Todos []messaging.TodoItem `json:"todos"`
}

// GetTodosRequest is a get todos request
type GetTodosRequest struct {
	AgentID string `json:"agent_id"`
}
