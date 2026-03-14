// Package server provides the enhanced MCP server with persistence
package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"multi-agent-mcp/pkg/common"
	"multi-agent-mcp/pkg/identity"
	"multi-agent-mcp/pkg/messaging"
	"multi-agent-mcp/pkg/storage/sqlite"

	"github.com/gorilla/websocket"
)

// EnhancedServer extends Server with SQLite persistence
type EnhancedServer struct {
	store       *sqlite.Store
	connections map[string]*websocket.Conn
	subscribers map[string]map[string]bool // agentID -> messageType -> subscribed
	upgrader    websocket.Upgrader
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewEnhancedServer creates a new enhanced server
func NewEnhancedServer(store *sqlite.Store) *EnhancedServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &EnhancedServer{
		store:       store,
		connections: make(map[string]*websocket.Conn),
		subscribers: make(map[string]map[string]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the WebSocket server
func (s *EnhancedServer) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/agents/", s.handleWebSocket)

	server := &http.Server{Addr: addr, Handler: mux}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	log.Printf("Enhanced MCP Server started on %s", addr)
	return nil
}

// handleWebSocket handles WebSocket upgrades
func (s *EnhancedServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.wg.Add(1)
	go s.handleConnection(conn)
}

// Stop stops the server
func (s *EnhancedServer) Stop() error {
	s.cancel()
	s.mu.Lock()
	for _, conn := range s.connections {
		conn.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
	return s.store.Close()
}

// handleConnection handles agent connections
func (s *EnhancedServer) handleConnection(conn *websocket.Conn) {
	defer s.wg.Done()

	// Read registration
	_, data, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return
	}

	var reg RegistrationMessage
	if err := json.Unmarshal(data, &reg); err != nil || reg.AgentID == "" {
		s.sendError(conn, "invalid registration")
		time.Sleep(100 * time.Millisecond)
		conn.Close()
		return
	}

	// Store connection
	s.mu.Lock()
	s.connections[reg.AgentID] = conn
	s.mu.Unlock()

	// Save agent to DB
	info := identity.AgentInfo{
		AgentID:      reg.AgentID,
		DisplayName:  reg.DisplayName,
		Capabilities: reg.Capabilities,
		Status:       identity.StatusOnline,
		LastSeenAt:   common.Now(),
	}
	s.store.SaveAgent(s.ctx, info)

	// Log event
	s.logEvent("agent_joined", reg.AgentID, nil)

	// Send success
	s.sendSuccess(conn, "registered")

	// Deliver offline messages
	s.deliverOfflineMessages(reg.AgentID, conn)

	// Broadcast to other agents (if subscribed)
	s.notifyAgentJoined(info)

	// Handle messages
	s.handleMessages(reg.AgentID, conn)
}

// handleMessages processes agent messages
func (s *EnhancedServer) handleMessages(agentID string, conn *websocket.Conn) {
	defer func() {
		s.mu.Lock()
		delete(s.connections, agentID)
		s.mu.Unlock()

		// Mark offline
		if info, err := s.store.GetAgent(s.ctx, agentID); err == nil {
			info.Status = identity.StatusOffline
			s.store.SaveAgent(s.ctx, info)
		}

		s.logEvent("agent_left", agentID, nil)
		s.notifyAgentLeft(agentID)
		conn.Close()
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var req Request
		if err := json.Unmarshal(data, &req); err != nil {
			s.sendError(conn, "invalid request")
			continue
		}

		s.handleRequest(agentID, conn, &req)
	}
}

// handleRequest handles specific requests
func (s *EnhancedServer) handleRequest(agentID string, conn *websocket.Conn, req *Request) error {
	switch req.Method {
	case "subscribe":
		return s.handleSubscribe(agentID, conn, req.Params)
	case "unsubscribe":
		return s.handleUnsubscribe(agentID, conn, req.Params)
	case "send_message":
		return s.handleSendMessage(agentID, req.Params)
	case "broadcast":
		return s.handleBroadcast(agentID, req.Params)
	case "rename":
		return s.handleRename(agentID, conn, req.Params)
	case "query_agents":
		return s.handleQueryAgents(conn, req.Params)
	case "publish_todo":
		return s.handlePublishTodo(agentID, req.Params)
	case "get_todos":
		return s.handleGetTodos(conn, req.Params)
	case "get_offline_messages":
		return s.handleGetOfflineMessages(agentID, conn)
	case "heartbeat":
		return s.handleHeartbeat(agentID)
	default:
		return s.sendError(conn, "unknown method")
	}
}

// handleSubscribe subscribes agent to message types
func (s *EnhancedServer) handleSubscribe(agentID string, conn *websocket.Conn, params json.RawMessage) error {
	var req struct {
		MessageTypes []string `json:"message_types"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return s.sendError(conn, "invalid params")
	}

	s.mu.Lock()
	if s.subscribers[agentID] == nil {
		s.subscribers[agentID] = make(map[string]bool)
	}
	for _, mt := range req.MessageTypes {
		s.subscribers[agentID][mt] = true
		s.store.SaveSubscription(s.ctx, agentID, mt)
	}
	s.mu.Unlock()

	return s.sendSuccess(conn, "subscribed")
}

// handleUnsubscribe unsubscribes agent
func (s *EnhancedServer) handleUnsubscribe(agentID string, conn *websocket.Conn, params json.RawMessage) error {
	var req struct {
		MessageType string `json:"message_type"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return s.sendError(conn, "invalid params")
	}

	s.mu.Lock()
	delete(s.subscribers[agentID], req.MessageType)
	s.mu.Unlock()

	s.store.DeleteSubscription(s.ctx, agentID, req.MessageType)
	return s.sendSuccess(conn, "unsubscribed")
}

// handleSendMessage sends message with persistence
func (s *EnhancedServer) handleSendMessage(from string, params json.RawMessage) error {
	var req struct {
		To      string          `json:"to"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return err
	}

	msg := messaging.NewMessage(from, messaging.TypeDirect)
	msg.SetRecipient(req.To)
	msg.SetPayload(req.Payload)

	// Save to database
	s.store.SaveMessage(s.ctx, msg)

	// Check if recipient is online
	s.mu.RLock()
	conn, online := s.connections[req.To]
	s.mu.RUnlock()

	if online {
		// Check subscription
		if s.shouldDeliver(req.To, messaging.TypeDirect) {
			data, _ := json.Marshal(msg)
			conn.WriteMessage(websocket.TextMessage, data)
		}
	} else {
		// Queue for offline delivery
		s.store.QueueOfflineMessage(s.ctx, req.To, msg.ID)
	}

	return nil
}

// handleBroadcast broadcasts with subscription filtering
func (s *EnhancedServer) handleBroadcast(from string, params json.RawMessage) error {
	var req struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return err
	}

	msg := messaging.NewMessage(from, messaging.TypeBroadcast)
	msg.SetPayload(req.Payload)

	// Save to database
	s.store.SaveMessage(s.ctx, msg)

	// Broadcast to subscribed agents only
	s.mu.RLock()
	conns := make(map[string]*websocket.Conn)
	for id, conn := range s.connections {
		if id != from && s.shouldDeliver(id, messaging.TypeBroadcast) {
			conns[id] = conn
		}
	}
	s.mu.RUnlock()

	data, _ := json.Marshal(msg)
	for _, conn := range conns {
		conn.WriteMessage(websocket.TextMessage, data)
	}

	return nil
}

// handleRename handles identity rename
func (s *EnhancedServer) handleRename(agentID string, conn *websocket.Conn, params json.RawMessage) error {
	var req struct {
		NewName string `json:"new_name"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return s.sendError(conn, "invalid params")
	}

	info, err := s.store.GetAgent(s.ctx, agentID)
	if err != nil {
		return s.sendError(conn, "agent not found")
	}

	oldName := info.DisplayName
	info.DisplayName = req.NewName
	s.store.SaveAgent(s.ctx, info)

	// Broadcast identity update to subscribers
	update := messaging.IdentityUpdatePayload{
		AgentID:   agentID,
		OldName:   oldName,
		NewName:   req.NewName,
		UpdatedAt: common.Now(),
	}

	msg := messaging.NewMessage(agentID, messaging.TypeIdentityUpdate)
	msg.SetPayload(update)
	s.store.SaveMessage(s.ctx, msg)

	s.broadcastToSubscribers(messaging.TypeIdentityUpdate, msg)

	return s.sendSuccess(conn, "renamed")
}

// handleQueryAgents queries agents from DB
func (s *EnhancedServer) handleQueryAgents(conn *websocket.Conn, params json.RawMessage) error {
	agents, err := s.store.ListAgents(s.ctx)
	if err != nil {
		return s.sendError(conn, "query failed")
	}

	return s.sendResponse(conn, agents)
}

// handlePublishTodo publishes todos with notifications
func (s *EnhancedServer) handlePublishTodo(agentID string, params json.RawMessage) error {
	var req struct {
		Todos []messaging.TodoItem `json:"todos"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return err
	}

	update := messaging.TodoUpdatePayload{
		AgentID:   agentID,
		Todos:     req.Todos,
		UpdatedAt: common.Now(),
	}

	msg := messaging.NewMessage(agentID, messaging.TypeTodoUpdate)
	msg.SetPayload(update)
	s.store.SaveMessage(s.ctx, msg)

	// Notify subscribers
	s.broadcastToSubscribers(messaging.TypeTodoUpdate, msg)

	return nil
}

// handleGetTodos retrieves todos (placeholder - would fetch from storage)
func (s *EnhancedServer) handleGetTodos(conn *websocket.Conn, params json.RawMessage) error {
	return s.sendResponse(conn, []messaging.TodoItem{})
}

// handleGetOfflineMessages delivers queued messages
func (s *EnhancedServer) handleGetOfflineMessages(agentID string, conn *websocket.Conn) error {
	messages, err := s.store.GetOfflineMessages(s.ctx, agentID)
	if err != nil {
		return s.sendError(conn, "fetch failed")
	}

	// Send messages
	for _, msg := range messages {
		data, _ := json.Marshal(msg)
		conn.WriteMessage(websocket.TextMessage, data)
	}

	// Clear queue
	s.store.ClearOfflineMessages(s.ctx, agentID)

	return s.sendResponse(conn, map[string]int{"delivered": len(messages)})
}

// handleHeartbeat updates last seen
func (s *EnhancedServer) handleHeartbeat(agentID string) error {
	info, err := s.store.GetAgent(s.ctx, agentID)
	if err != nil {
		return err
	}
	info.LastSeenAt = common.Now()
	return s.store.SaveAgent(s.ctx, info)
}

// deliverOfflineMessages sends queued messages on connect
func (s *EnhancedServer) deliverOfflineMessages(agentID string, conn *websocket.Conn) {
	messages, err := s.store.GetOfflineMessages(s.ctx, agentID)
	if err != nil || len(messages) == 0 {
		return
	}

	for _, msg := range messages {
		data, _ := json.Marshal(msg)
		conn.WriteMessage(websocket.TextMessage, data)
	}

	s.store.ClearOfflineMessages(s.ctx, agentID)
}

// shouldDeliver checks if agent is subscribed to message type
func (s *EnhancedServer) shouldDeliver(agentID string, msgType messaging.MessageType) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// If no subscriptions, deliver all (backward compatible)
	if len(s.subscribers[agentID]) == 0 {
		return true
	}

	return s.subscribers[agentID][string(msgType)]
}

// broadcastToSubscribers sends to subscribed agents only
func (s *EnhancedServer) broadcastToSubscribers(msgType messaging.MessageType, msg *messaging.Message) {
	data, _ := json.Marshal(msg)

	s.mu.RLock()
	for agentID, conn := range s.connections {
		if s.shouldDeliver(agentID, msgType) {
			conn.WriteMessage(websocket.TextMessage, data)
		}
	}
	s.mu.RUnlock()
}

// notifyAgentJoined notifies subscribers
func (s *EnhancedServer) notifyAgentJoined(info identity.AgentInfo) {
	msg := messaging.NewMessage(info.AgentID, messaging.TypeAgentJoined)
	msg.SetPayload(info)
	s.store.SaveMessage(s.ctx, msg)
	s.broadcastToSubscribers(messaging.TypeAgentJoined, msg)
}

// notifyAgentLeft notifies subscribers
func (s *EnhancedServer) notifyAgentLeft(agentID string) {
	msg := messaging.NewMessage(agentID, messaging.TypeAgentLeft)
	msg.SetPayload(map[string]string{"agent_id": agentID})
	s.store.SaveMessage(s.ctx, msg)
	s.broadcastToSubscribers(messaging.TypeAgentLeft, msg)
}

// logEvent logs server event
func (s *EnhancedServer) logEvent(eventType, agentID string, data []byte) {
	s.store.LogEvent(s.ctx, eventType, agentID, data)
}

// Helper methods
func (s *EnhancedServer) sendSuccess(conn *websocket.Conn, msg string) error {
	resp := Response{Success: true, Message: msg}
	data, _ := json.Marshal(resp)
	return conn.WriteMessage(websocket.TextMessage, data)
}

func (s *EnhancedServer) sendError(conn *websocket.Conn, err string) error {
	resp := Response{Success: false, Error: err}
	data, _ := json.Marshal(resp)
	return conn.WriteMessage(websocket.TextMessage, data)
}

func (s *EnhancedServer) sendResponse(conn *websocket.Conn, data interface{}) error {
	resp := Response{Success: true, Data: data}
	bytes, _ := json.Marshal(resp)
	return conn.WriteMessage(websocket.TextMessage, bytes)
}

// WebSocketTransport wraps WebSocket transport
type WebSocketTransport struct {
	server *EnhancedServer
}

// NewWebSocketTransport creates transport
func NewWebSocketTransport(server *EnhancedServer) *WebSocketTransport {
	return &WebSocketTransport{server: server}
}
