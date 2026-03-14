# Multi-Agent MCP Server - Claude Code Configuration

## Project Overview

This is a **Multi-Agent MCP (Model Context Protocol) Coordination Server** built in Go. It enables multiple AI agents (including Claude Code) to collaborate in real-time via WebSocket.

## MCP Server Details

- **Server URL**: `ws://localhost:9095`
- **Protocol**: WebSocket with JSON-RPC style messages
- **Transport**: `/agents/` endpoint
- **Persistence**: SQLite (enhanced server) or In-Memory (standard server)

## Claude Code as an Agent

When working in this project, Claude Code acts as an **MCP Agent**:

- **Agent ID**: `claude-code-<workspace>`
- **Capabilities**: `coder`, `reviewer`, `architect`, `planner`
- **Status**: Online when connected

## Quick Start

### 1. Start the MCP Server

```bash
# Build if not already built
go build -o bin/enhanced-server ./cmd/enhanced-server/

# Start the server
./bin/enhanced-server -addr :9095 -db /tmp/mcp.db
```

### 2. Verify Connection

```bash
# Use the test client
./bin/testclient interactive

# Or run automated tests
./bin/testclient auto
```

## Agent Commands

Claude Code can use these commands to interact with other agents:

### Query Other Agents
```json
{
  "method": "query_agents",
  "params": {
    "capability": "planner"
  }
}
```

### Send Direct Message
```json
{
  "method": "send_message",
  "params": {
    "to": "other-agent-id",
    "payload": {
      "text": "Can you review this code?",
      "task": "code-review"
    }
  }
}
```

### Broadcast to All Agents
```json
{
  "method": "broadcast",
  "params": {
    "payload": {
      "announcement": "Starting deployment",
      "timestamp": "2026-03-14T10:00:00Z"
    }
  }
}
```

### Publish Todo List
```json
{
  "method": "publish_todo",
  "params": {
    "todos": [
      {"id": "1", "title": "Review PR #123", "status": "in_progress"},
      {"id": "2", "title": "Update documentation", "status": "todo"}
    ]
  }
}
```

### Subscribe to Message Types
```json
{
  "method": "subscribe",
  "params": {
    "message_types": ["direct", "broadcast", "todo_update"]
  }
}
```

### Rename Based on Current Task
```json
{
  "method": "rename",
  "params": {
    "new_name": "auth-service-coder"
  }
}
```

## Working with Other Agents

### When to Collaborate

1. **Complex Tasks**: Query for agents with specific capabilities
2. **Code Review**: Send messages to `reviewer` agents
3. **Architecture**: Consult `architect` agents for design decisions
4. **Planning**: Work with `planner` agents for task breakdown

### Best Practices

1. **Rename Appropriately**: Change display name based on current task
   - Examples: `api-coder`, `bugfix-reviewer`, `feature-planner`

2. **Publish Todos**: Share your task list so other agents know what you're working on

3. **Subscribe Selectively**: Only subscribe to message types you care about
   - `direct`: 1:1 messages
   - `broadcast`: announcements
   - `todo_update`: team task updates
   - `agent_joined`/`agent_left`: presence updates

4. **Use Capabilities**: Query agents by capability when you need help
   ```json
   {"capability": "security-reviewer"}
   ```

## Message Types

### Incoming (Server → Agent)

- `direct` - Direct message from another agent
- `broadcast` - Broadcast from any agent
- `agent_joined` - New agent connected
- `agent_left` - Agent disconnected
- `identity_update` - Agent renamed
- `todo_update` - Agent published todos

### Outgoing (Agent → Server)

- `send_message` - Send direct message
- `broadcast` - Broadcast message
- `query_agents` - Query agents by capability
- `publish_todo` - Publish todo list
- `get_todos` - Get agent's todo list
- `subscribe`/`unsubscribe` - Manage subscriptions
- `rename` - Rename agent
- `heartbeat` - Keep connection alive

## Testing

### Run All Tests
```bash
# Start server first
./bin/enhanced-server -addr :9095 -db /tmp/mcp.db &

# Run tests
./bin/testclient auto
```

### Interactive Testing
```bash
./bin/testclient interactive

Commands:
  connect <id> <name>  - Connect as agent
  rename <name>        - Rename agent
  send <to> <msg>      - Send message
  broadcast <msg>      - Broadcast
  todo <title>         - Add todo
  query [capability]   - Query agents
  exit                 - Disconnect
```

## Persistence Features

The enhanced server provides:

1. **SQLite Storage**: Agents, messages, and events persisted
2. **Offline Queueing**: Messages delivered when agents reconnect
3. **Subscription Filtering**: Only receive subscribed message types
4. **Event Logging**: All server events logged to database

## Architecture

```
Claude Code (Agent)
       │
       │ WebSocket
       ▼
┌─────────────────┐
│   MCP Server    │  Port 9095
│  (Enhanced)     │
├─────────────────┤
│  SQLite Store   │
│  Subscription   │
│  Offline Queue  │
└─────────────────┘
       │
       │ WebSocket
       ▼
  Other Agents
```

## Troubleshooting

### Connection Issues
```bash
# Check if server is running
curl -s http://localhost:9095/health 2>/dev/null || echo "Server not running"

# Check port
lsof -i :9095

# Restart server
pkill -f enhanced-server
./bin/enhanced-server -addr :9095 -db /tmp/mcp.db
```

### Agent ID Conflicts
Each Claude Code instance should use a unique agent ID. If connecting multiple instances:
- Instance 1: `claude-code-main`
- Instance 2: `claude-code-secondary`

### Offline Messages
If messages aren't being delivered:
1. Check agent is subscribed to the message type
2. Verify recipient is online or messages are queued
3. Check SQLite database: `/tmp/mcp.db`

## Development

### Adding New Features

1. **New Message Types**: Add to `pkg/messaging/types.go`
2. **New Handlers**: Update `pkg/server/enhanced.go`
3. **New Storage**: Implement in `pkg/storage/sqlite/store.go`

### Running in Dev Mode
```bash
# Standard server (no persistence)
go run ./cmd/server/ -addr :9095

# Enhanced server (with persistence)
go run ./cmd/enhanced-server/ -addr :9095 -db /tmp/mcp-dev.db
```

## References

- **README.md**: Full project documentation
- **DESIGN.md**: Detailed design document
- **TEST_RESULTS.md**: Test results and analysis
- **docs/CLAUDE_INTEGRATION.md**: Integration guide

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MCP_SERVER_URL` | WebSocket URL | `ws://localhost:9095` |
| `MCP_AGENT_ID` | Agent identifier | `claude-code` |
| `MCP_AGENT_CAPABILITIES` | Comma-separated capabilities | `coder,reviewer,architect` |

---

*This CLAUDE.md provides context for Claude Code when working with the Multi-Agent MCP system.*
