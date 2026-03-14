// Enhanced MCP server with SQLite persistence
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"multi-agent-mcp/pkg/server"
	"multi-agent-mcp/pkg/storage/sqlite"
)

func main() {
	var (
		addr   string
		dbPath string
	)
	flag.StringVar(&addr, "addr", ":9095", "Server listen address")
	flag.StringVar(&dbPath, "db", "/tmp/mcp.db", "SQLite database path")
	flag.Parse()

	// Create SQLite store
	store, err := sqlite.NewStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}

	// Create enhanced server
	srv := server.NewEnhancedServer(store)

	// Start server
	if err := srv.Start(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Printf("Enhanced MCP Server listening on %s (DB: %s)", addr, dbPath)

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	if err := srv.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}
