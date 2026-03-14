# Multi-Agent MCP System

A production-ready Multi-Agent Model Context Protocol (MCP) server written in Go. Enables dynamic agent collaboration with real-time messaging, identity management, shared todo lists, and SQLite persistence.

## Features

### Core Features
- **Dynamic Identity Management** - Agents can rename themselves based on current tasks
- **Capability-Based Discovery** - Query agents by capabilities (coder, planner, reviewer, etc.)
- **Real-Time Messaging** - WebSocket-based 1:1 and broadcast messaging
- **Shared Todo Lists** - CRDT-based collaborative todo lists with conflict resolution
- **Heartbeat Monitoring** - Automatic agent health tracking

### Enhanced Features (v2)
- **SQLite Persistence** - Messages, agents, and events stored in SQLite
- **Subscription Filtering** - Agents subscribe to specific message types
- **Offline Message Queueing** - Messages delivered when agents reconnect
- **Push Notifications** - Server events (join/leave/updates) broadcast to subscribers

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     MCP Server (Port 9095)                  │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │   Registry   │  │   Messaging  │  │    Identity      │   │
│  │   Service    │  │    Service   │  │    Service       │   │
│  └──────────────┘  └──────────────┘  └──────────────────┘   │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │   SQLite     │  │ Subscription │  │  Offline Queue   │   │
│  │   Storage    │  │   Manager    │  │                  │   │
│  └──────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                    WebSocket Transport
                              │
         ┌─────────┬──────────┴──────────┬─────────┐
         ▼         ▼                     ▼         ▼
   ┌─────────┐ ┌─────────┐         ┌─────────┐ ┌──────────┐
   │ Agent 1 │ │ Agent 2 │   ...   │ Agent N │ │ Agent N+1│
   └─────────┘ └─────────┘         └─────────┘ └──────────┘
```

## Quick Start

### Prerequisites
- Go 1.23+
- SQLite3

### Installation

```bash
# Clone the repository
git clone <repository>
cd multi-agent-chat-mcp

# Install dependencies
go mod tidy

# Build binaries
go build -o bin/server ./cmd/server/
go build -o bin/enhanced-server ./cmd/enhanced-server/
go build -o bin/testclient ./cmd/testclient/
```

### Running the Server

#### Standard Server (In-Memory)
```bash
./bin/server -addr :9095
```

#### Enhanced Server (SQLite Persistence)
```bash
./bin/enhanced-server -addr :9095 -db /tmp/mcp.db
```

### Testing

#### Automated Tests
```bash
# Start server in one terminal
./bin/enhanced-server -addr :9095 -db /tmp/mcp.db

# Run tests in another terminal
./bin/testclient auto
```

#### Interactive Test Client
```bash
./bin/testclient interactive

# Commands:
# connect <agent_id> <display_name>  - Connect to server
# rename <new_name>                  - Rename agent
# send <to_agent> <message>          - Send 1:1 message
# broadcast <message>                - Broadcast to all
# todo <title>                       - Publish todo
# query [capability]                 - Query agents
# get <agent_id>                     - Get agent's todos
# exit                               - Disconnect
```

## API Protocol

### Connection
Agents connect via WebSocket to `ws://localhost:9095/agents/`

### Registration (First Message)
```json
{
  "agent_id": "unique-agent-id",
  "display_name": "My Agent",
  "capabilities": ["coder", "reviewer"]
}
```

### Methods

#### Subscribe to Message Types
```json
{
  "method": "subscribe",
  "params": {
    "message_types": ["direct", "broadcast", "todo_update"]
  }
}
```

#### Unsubscribe
```json
{
  "method": "unsubscribe",
  "params": {
    "message_type": "broadcast"
  }
}
```

#### Send Direct Message
```json
{
  "method": "send_message",
  "params": {
    "to": "recipient-agent-id",
    "payload": {"text": "Hello!"}
  }
}
```

#### Broadcast Message
```json
{
  "method": "broadcast",
  "params": {
    "payload": {"announcement": "Hello everyone!"}
  }
}
```

#### Rename Agent
```json
{
  "method": "rename",
  "params": {
    "new_name": "new-display-name"
  }
}
```

#### Query Agents
```json
{
  "method": "query_agents",
  "params": {
    "capability": "coder"
  }
}
```

#### Publish Todo List
```json
{
  "method": "publish_todo",
  "params": {
    "todos": [
      {"id": "1", "title": "Task 1", "status": "todo"},
      {"id": "2", "title": "Task 2", "status": "in_progress"}
    ]
  }
}
```

#### Get Offline Messages
```json
{
  "method": "get_offline_messages",
  "params": {}
}
```

#### Heartbeat
```json
{
  "method": "heartbeat"
}
```

### Server Push Events

The server automatically broadcasts these events to subscribed agents:

- `agent_joined` - New agent connected
- `agent_left` - Agent disconnected
- `identity_update` - Agent renamed
- `todo_update` - Agent published todos
- `direct` - Direct message received
- `broadcast` - Broadcast message received

## Project Structure

```
.
├── cmd/
│   ├── server/           # Standard in-memory server
│   ├── enhanced-server/  # SQLite persistence server
│   ├── testclient/       # Interactive test client
│   ├── extended_tests/   # Edge case tests
│   └── agent/            # Example agent implementation
├── pkg/
│   ├── server/           # Server implementations
│   ├── identity/         # Identity management
│   ├── messaging/        # Message types & CRDT
│   ├── registry/         # Agent registry service
│   ├── storage/          # Storage abstractions
│   │   └── sqlite/       # SQLite implementation
│   ├── common/           # Utilities
│   └── interfaces/       # Core abstractions
├── DESIGN.md             # Detailed design document
└── TEST_RESULTS.md       # Test results & analysis
```
## SQLite Schema

```sql
-- Agents table
CREATE TABLE agents (
    agent_id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    capabilities TEXT,
    status TEXT,
    current_task TEXT,
    last_seen_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Messages table
CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    message_type TEXT NOT NULL,
    from_agent TEXT NOT NULL,
    to_agent TEXT,
    task_id TEXT,
    priority INTEGER,
    payload BLOB,
    timestamp DATETIME,
    delivered BOOLEAN DEFAULT FALSE
);

-- Offline message queue
CREATE TABLE offline_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Subscriptions
CREATE TABLE subscriptions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT NOT NULL,
    message_type TEXT NOT NULL,
    UNIQUE(agent_id, message_type)
);

-- Events log
CREATE TABLE agent_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    data BLOB,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## Configuration

### Server Flags
```
-addr string    Server listen address (default ":9095")
-db string      SQLite database path (default "/tmp/mcp.db")
```

### Environment Variables
None required. All configuration via command-line flags.

## Development

### Adding New Message Types
1. Add type constant in `pkg/messaging/types.go`
2. Add handler in `pkg/server/enhanced.go` or `server.go`
3. Update subscription logic if needed

### Adding New Storage Backends
Implement the storage interfaces in `pkg/storage/`:
- `SaveAgent`, `GetAgent`, `ListAgents`
- `SaveMessage`, `QueueOfflineMessage`
- `SaveSubscription`, `DeleteSubscription`

## License

MIT License

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## Troubleshooting

### Port Already in Use
```bash
# Find process using port 9095
lsof -i :9095

# Kill the process
kill <PID>
```

### SQLite Errors
Ensure the database directory is writable:
```bash
chmod 755 /tmp
```

### WebSocket Connection Issues
Check firewall settings and ensure the server is running:
```bash
nc -zv localhost 9095
```
