# MCP Agent Bridge for Claude Code

This directory contains the Claude Code MCP integration for the multi-agent coordination server.

## Setup

1. Start the MCP server:
```bash
cd /workspaces/multi-agent-chat-mcp
./bin/enhanced-server -addr :9095 -db /tmp/mcp.db
```

2. Claude Code connects via the bridge at `ws://localhost:9095/agents/`

## How It Works

Claude Code acts as an agent in the multi-agent system:
- Agent ID: `claude-code-<workspace>`
- Capabilities: `coder`, `reviewer`, `planner`, `architect`
- Can send/receive messages from other agents
- Can publish todo lists
- Can query other agents by capability

## Configuration

Add to your project's `CLAUDE.md`:

```markdown
# MCP Multi-Agent Configuration

You are connected to a multi-agent MCP server on port 9095.
Your agent ID is "claude-code-<project>".

When working on tasks:
1. Check for other agents: query agents with capabilities matching the task
2. Collaborate: send messages to agents for help
3. Publish todos: share your task list with the team
4. Subscribe: listen for relevant updates from other agents

## Available Commands

- `/agents` - List connected agents
- `/collaborate <agent-id> <message>` - Send message to agent
- `/broadcast <message>` - Broadcast to all agents
- `/todo <title>` - Add todo item
- `/todos` - Show my todo list
```

## Integration with Claude Code

The MCP server runs as a separate process. Claude Code interacts with it through:
- WebSocket connection at `ws://localhost:9095/agents/`
- JSON-RPC style protocol
- Automatic reconnection on disconnect

## Troubleshooting

- **Connection refused**: Ensure the MCP server is running on port 9095
- **Agent ID conflict**: Each Claude instance should use a unique agent ID
- **Offline messages**: Messages are queued when offline and delivered on reconnect
