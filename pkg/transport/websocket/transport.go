// Package websocket provides WebSocket transport implementation
package websocket

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"multi-agent-mcp/pkg/common"
	"multi-agent-mcp/pkg/interfaces"

	"github.com/gorilla/websocket"
)

// Transport implements interfaces.Transport for WebSocket
type Transport struct {
	upgrader websocket.Upgrader
	dialer   websocket.Dialer
}

// NewTransport creates a new WebSocket transport
func NewTransport() *Transport {
	return &Transport{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in development
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		dialer: websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		},
	}
}

// Listen starts listening for WebSocket connections
func (t *Transport) Listen(addr string) (interfaces.Listener, error) {
	listener := &wsListener{
		transport: t,
		addr:      addr,
		connChan:  make(chan interfaces.Connection, 10),
		stopChan:  make(chan struct{}),
	}

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/agents/", listener.handleWebSocket)

	listener.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start listening
	go listener.start()

	return listener, nil
}

// Dial connects to a WebSocket server
func (t *Transport) Dial(addr string) (interfaces.Connection, error) {
	wsURL := fmt.Sprintf("ws://%s/agents/", addr)

	ws, _, err := t.dialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("dial websocket: %w", err)
	}

	return newConnection(ws), nil
}

// wsListener implements interfaces.Listener
type wsListener struct {
	transport *Transport
	addr      string
	server    *http.Server
	connChan  chan interfaces.Connection
	stopChan  chan struct{}
	mu        sync.Mutex
	stopped   bool
}

// start begins listening for connections
func (l *wsListener) start() {
	go func() {
		if err := l.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error
		}
	}()
}

// handleWebSocket handles WebSocket upgrade requests
func (l *wsListener) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	l.mu.Lock()
	if l.stopped {
		l.mu.Unlock()
		http.Error(w, "server stopped", http.StatusServiceUnavailable)
		return
	}
	l.mu.Unlock()

	// Upgrade to WebSocket
	ws, err := l.transport.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// Create connection wrapper
	conn := newConnection(ws)

	// Send to accept channel
	select {
	case l.connChan <- conn:
	case <-l.stopChan:
		conn.Close()
	}
}

// Accept waits for and returns the next connection
func (l *wsListener) Accept() (interfaces.Connection, error) {
	select {
	case conn := <-l.connChan:
		return conn, nil
	case <-l.stopChan:
		return nil, common.ErrConnection
	}
}

// Close stops the listener
func (l *wsListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.stopped {
		return nil
	}

	l.stopped = true
	close(l.stopChan)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return l.server.Shutdown(ctx)
}

// Ensure wsListener implements interface
var _ interfaces.Listener = (*wsListener)(nil)

// Connection implements interfaces.Connection for WebSocket
type Connection struct {
	ws       *websocket.Conn
	readChan chan []byte
	writeChan chan []byte
	stopChan  chan struct{}
	mu        sync.RWMutex
	closed    bool
}

// newConnection creates a new WebSocket connection wrapper
func newConnection(ws *websocket.Conn) *Connection {
	c := &Connection{
		ws:        ws,
		readChan:  make(chan []byte, 100),
		writeChan: make(chan []byte, 100),
		stopChan:  make(chan struct{}),
	}

	// Start read/write goroutines
	go c.readLoop()
	go c.writeLoop()

	return c
}

// Read reads a message from the connection
func (c *Connection) Read() ([]byte, error) {
	select {
	case data := <-c.readChan:
		return data, nil
	case <-c.stopChan:
		return nil, common.ErrConnection
	}
}

// Write writes a message to the connection
func (c *Connection) Write(data []byte) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return common.ErrConnection
	}
	c.mu.RUnlock()

	select {
	case c.writeChan <- data:
		return nil
	case <-c.stopChan:
		return common.ErrConnection
	}
}

// Close closes the connection
func (c *Connection) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	close(c.stopChan)
	return c.ws.Close()
}

// RemoteAddr returns the remote address
func (c *Connection) RemoteAddr() string {
	return c.ws.RemoteAddr().String()
}

// readLoop continuously reads from WebSocket
func (c *Connection) readLoop() {
	defer close(c.readChan)

	for {
		select {
		case <-c.stopChan:
			return
		default:
		}

		// Set read deadline
		c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))

		messageType, data, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Log error
			}
			return
		}

		if messageType == websocket.BinaryMessage || messageType == websocket.TextMessage {
			select {
			case c.readChan <- data:
			case <-c.stopChan:
				return
			}
		}
	}
}

// writeLoop continuously writes to WebSocket
func (c *Connection) writeLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case data := <-c.writeChan:
			c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.ws.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

		case <-ticker.C:
			// Send ping
			c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.stopChan:
			return
		}
	}
}

// Ensure Connection implements interface
var _ interfaces.Connection = (*Connection)(nil)
