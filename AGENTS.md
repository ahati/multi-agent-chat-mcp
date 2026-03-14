# Repository Guidelines

## Project Structure & Module Organization

This is a Go-based multi-agent collaboration system built on the Model Context Protocol (MCP) Go SDK.

```
multi-agent-chat-mcp/
├── cmd/                    # Application entry points
│   ├── agent/              # Agent executable
│   ├── server/             # MCP server executable
│   └── simulation/         # Simulation tools
├── pkg/                    # Core library packages
│   ├── client/             # MCP client implementation
│   ├── identity/           # Identity management
│   ├── mcp/                # MCP protocol handlers
│   ├── messaging/          # Message routing
│   ├── pubsub/             # Publish/subscribe layer
│   ├── registry/           # Agent registry
│   ├── server/             # Server components
│   ├── storage/            # Storage backends (memory, sqlite)
│   ├── taskboard/          # CRDT-based task board
│   └── transport/          # Transport layer (WebSocket, TCP)
├── docs/                   # Documentation
└── plans/                  # Development plans
```

## Build, Test, and Development Commands

```bash
go build ./cmd/...          # Build all executables
go test ./...               # Run all tests
go test ./pkg/...           # Test library packages only
go vet ./...                # Run Go vet for static analysis
go mod tidy                 # Clean up go.mod dependencies
```

The project uses Go 1.23.4 with gorilla/websocket and go-sqlite3 dependencies.

## Coding Style & Naming Conventions

- **Indentation**: Tabs (Go standard)
- **Formatting**: Run `go fmt ./...` before committing
- **Naming**: Use `PascalCase` for exported identifiers, `camelCase` for private
- **Interfaces**: Name with `-er` suffix (e.g., `Transport`, `Storage`, `Handler`)
- **Architecture**: Strictly follow **DRY** (Don't Repeat Yourself) and **SOLID** principles:
  - Each package has a single responsibility
  - Depend on abstractions, not concrete implementations
  - Components are open for extension, closed for modification

## Testing Guidelines

- **Framework**: Go's built-in `testing` package
- **Coverage**: **90%+ code coverage required** for all contributions
- **Mocks**: Use interface-based mocks for transport and storage layers
- **Test files**: Name as `<package>_test.go` in the same directory as source

Run tests with:
```bash
go test -v ./pkg/...        # Verbose output
go test -cover ./...        # With coverage report
```

## Commit & Pull Request Guidelines

- **Commit messages**: Use imperative mood ("Add feature", "Fix bug", not "Added" or "Fixes")
- **Scope**: Keep commits focused on a single concern
- **PR descriptions**: Include purpose, testing approach, and any breaking changes
- **Reviews**: Ensure changes maintain DRY and SOLID principles; verify 90%+ coverage

## Architecture Notes

The system follows a layered architecture:
1. **Network Layer**: MCP server as central hub with WebSocket transports
2. **Services Layer**: Identity, Registry, Router, TaskBoard, and MessageStore services
3. **Agent Layer**: Individual agents connecting via the transport interface

All components depend on abstractions (interfaces), enabling easy substitution of implementations for testing or deployment variations.
