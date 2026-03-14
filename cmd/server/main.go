// main.go is the MCP server entry point
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"multi-agent-mcp/pkg/messaging"
	mempusb "multi-agent-mcp/pkg/pubsub/memory"
	"multi-agent-mcp/pkg/registry"
	"multi-agent-mcp/pkg/server"
	memstore "multi-agent-mcp/pkg/storage/memory"
	"multi-agent-mcp/pkg/taskboard"
	websockettransport "multi-agent-mcp/pkg/transport/websocket"
)

func main() {
	var addr string
	flag.StringVar(&addr, "addr", ":8080", "Server listen address")
	flag.Parse()

	// Create stores
	agentStore := memstore.NewAgentInfoStore()
	todoStore := memstore.NewTodoStore()
	pubsub := mempusb.NewPubSub()

	// Create services
	registrySvc := registry.NewService(agentStore)
	router := messaging.NewRouter(registrySvc)
	taskBoardSvc := taskboard.NewService(todoStore, pubsub)

	// Create transport
	transport := websockettransport.NewTransport()

	// Create server
	srv := server.NewServer(transport, agentStore, todoStore, registrySvc, router, taskBoardSvc)

	// Start server
	if err := srv.Start(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Printf("MCP Server listening on %s", addr)

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	if err := srv.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}
