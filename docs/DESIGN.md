# Multi-Agent MCP System Design Document

## Executive Summary

This document describes a distributed multi-agent collaboration system built on the Model Context Protocol (MCP) Go SDK. The architecture strictly follows **DRY** (Don't Repeat Yourself) and **SOLID** principles to ensure maintainability, testability, and extensibility.

---

## Core Concepts

### Agent Identity

Each agent has a dynamic identity that can change based on task context:

```go
type Identity struct {
    AgentID      string            // Permanent UUID
    DisplayName  string            // Dynamic: "auth-api-coder", "hotfix-agent"
    CurrentTask  string            // Task ID currently working on
    Capabilities []Capability      // What this agent can do
    Status       AgentStatus       // online, busy, away, offline
    Metadata     map[string]any    // Task-specific context
}
```

Agents can rename themselves based on their current task role (e.g., "frontend-reviewer", "database-migration-agent").

### Capability-Based Discovery

Agents declare capabilities (`coder`, `planner`, `reviewer`, `deployer`, `database`, `testing`, `devops`, `ai`) enabling task delegation and help requests.

### Message Routing

The system supports:
- **Direct messages**: 1:1 communication between agents
- **Broadcast messages**: Send to all connected agents
- **Offline queues**: Messages queued for disconnected agents (max 1000 per agent)

---

## SOLID Principles Application

### Single Responsibility Principle (SRP)

| Component | Responsibility |
|-----------|---------------|
| `Identity Manager` | Only manages identity state changes |
| `Message Router` | Only routes messages, no business logic |
| `Registry` | Only tracks agent presence |
| `Transport` | Only handles network I/O |
| `TaskBoard` | Only manages CRDT-based todo lists |

### Open/Closed Principle (OCP)

- New transports added via `interfaces.Transport` implementation
- New message handlers registered without changing router
- New tools added to server without modifying core

### Liskov Substitution Principle (LSP)

- Any `Transport` can replace another (WebSocket, TCP, Mock)
- Any `Storage` backend works with any service (Memory, SQLite, Redis)
- Mock implementations valid for testing

### Interface Segregation Principle (ISP)

Clients depend only on methods they use:
- `Querier[K,V]` for read-only access
- `Writer[K,V]` for write-only access
- `Publisher` / `Subscriber` for events

### Dependency Inversion Principle (DIP)

- Services depend on `interfaces.Storer`, not concrete DB
- Server depends on `interfaces.Transport`, not WebSocket
- Client depends on `interfaces.Connection`, not socket

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         NETWORK LAYER                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐     │
│  │                    MCP SERVER (Central Hub)                         │     │
│  │              Network: ws://mcp-server:8080/agents                   │     │
│  │                                                                     │     │
│  │  ┌─────────────────────────────────────────────────────────────┐   │     │
│  │  │                    CORE INTERFACES                          │   │     │
│  │  │  - Transport (Listen, Dial)                                 │   │     │
│  │  │  - Connection (Read, Write, Close)                          │   │     │
│  │  │  - Storer[K,V] (Get, Set, Delete, List)                     │   │     │
│  │  │  - PubSub (Publish, Subscribe)                              │   │     │
│  │  └─────────────────────────────────────────────────────────────┘   │     │
│  │                                                                     │     │
│  │  ┌─────────────────────────────────────────────────────────────┐   │     │
│  │  │                    SERVICES (SRP)                           │   │     │
│  │  │  - RegistryService: Agent registration & presence           │   │     │
│  │  │  - Router: Message routing & offline queues                 │   │     │
│  │  │  - TaskBoard: CRDT-based todo synchronization               │   │     │
│  │  └─────────────────────────────────────────────────────────────┘   │     │
│  └─────────────────────────────────────────────────────────────────────┘     │
│                            │ WebSocket                                        │
│         ┌──────────────────┼──────────────────┐                               │
│         ▼                  ▼                  ▼                               │
│    ┌─────────┐       ┌─────────┐       ┌─────────┐                           │
│    │ Agent-A │       │ Agent-B │       │ Agent-C │                           │
│    │ (coder) │       │(planner)│       │(reviewer)│                          │
│    └─────────┘       └─────────┘       └─────────┘                           │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Sequence Diagrams

### Agent Registration Flow

```
Agent                          MCP Server                    Registry
  │                               │                              │
  │─── RegistrationMessage ──────▶│                              │
  │    {agent_id, capabilities}   │                              │
  │                               │─── Register(agentInfo) ─────▶│
  │                               │                              │
  │◀─── Success Response ─────────│                              │
  │    "registered"               │                              │
  │                               │                              │
  │─── Heartbeat (periodic) ─────▶│                              │
  │                               │─── UpdateHeartbeat ─────────▶│
  │                               │                              │
```

### Identity Rename Flow

```
Agent                          MCP Server                    Router        Other Agents
  │                               │                              │              │
  │─── rename request ───────────▶│                              │              │
  │    {new_name: "auth-coder"}   │                              │              │
  │                               │─── Update Registry ─────────▶│              │
  │                               │                              │              │
  │                               │─── IdentityUpdate ──────────▶│──────────────▶│
  │                               │    {agent_id, old, new}      │  Broadcast   │
  │◀─── "renamed" ────────────────│                              │              │
  │                               │                              │              │
```

### Direct Message Flow

```
Agent-A                        MCP Server                    Router        Agent-B
  │                               │                              │              │
  │─── send_message ─────────────▶│                              │              │
  │    {to: "B", payload: {...}}  │                              │              │
  │                               │─── Route(message) ──────────▶│              │
  │                               │                              │              │
  │                               │                              │─── Write ───▶│
  │                               │                              │  (WebSocket) │
  │                               │                              │              │
  │                               │◀─── Delivery Confirm ────────│              │
  │◀─── Success ──────────────────│                              │              │
  │                               │                              │              │
```

### Offline Message Queue Flow

```
Agent-A                        MCP Server                    Router        Agent-B (offline)
  │                               │                              │              │
  │─── send_message ─────────────▶│                              │              │
  │    {to: "B", payload: {...}}  │                              │              │
  │                               │─── Route(message) ──────────▶│              │
  │                               │                              │              │
  │                               │                              │─── Queue ───▶│
  │                               │                              │  (max 1000)  │
  │◀─── "queued" ─────────────────│                              │              │
  │                               │                              │              │
  │                               │                              │              │
  │                    ┌──────────┴──────────┐                   │              │
  │                    │ Agent-B reconnects  │                   │              │
  │                    └──────────┬──────────┘                   │              │
  │                               │                              │              │
  │                               │                              │─── Deliver ─▶│
  │                               │                              │  (sorted by  │
  │                               │                              │   priority)  │
  │                               │                              │              │
```

### TaskBoard Publish/Get Flow

```
Agent-A                        MCP Server                    TaskBoard      Agent-B
  │                               │                              │              │
  │─── publish_todo ─────────────▶│                              │              │
  │    {todos: [{id, text, ...}]} │                              │              │
  │                               │─── Publish(agentID, todos) ─▶│              │
  │                               │                              │  (CRDT merge)│
  │◀─── Success ──────────────────│                              │              │
  │                               │                              │              │
  │                               │                              │              │
  │                    Agent-B requests todos                     │              │
  │                               │                              │              │
  │◀──────────────────────────────│◀─── Get(agentID) ────────────│              │
  │    {todos: [...]}             │                              │              │
  │                               │                              │              │
```

---

## Package Organization

```
pkg/
├── interfaces/       # Core abstractions (Transport, Connection, Storer, PubSub)
├── identity/         # Dynamic identity management with capabilities
├── registry/         # Agent presence tracking with heartbeat monitoring
├── messaging/        # Message router with offline queue support
├── transport/        # Network transport implementations
│   └── websocket/    # WebSocket transport (gorilla/websocket)
├── storage/          # Storage backends
│   ├── memory/       # In-memory stores (AgentInfo, Todo, Message)
│   └── sqlite/       # SQLite persistence
├── taskboard/        # CRDT-based todo list synchronization
├── server/           # MCP server composition & request handlers
├── client/           # MCP client implementation
└── common/           # Shared utilities (validation, time helpers)
```

---

## Key Design Decisions

### 1. Interface-First Architecture

All core dependencies are defined as interfaces in `pkg/interfaces/core.go`:

```go
type Transport interface {
    Listen(addr string) (Listener, error)
    Dial(addr string) (Connection, error)
}

type Connection interface {
    Read() ([]byte, error)
    Write([]byte) error
    Close() error
    RemoteAddr() string
}

type Storer[K comparable, V any] interface {
    Get(ctx context.Context, key K) (V, error)
    Set(ctx context.Context, key K, value V) error
    Delete(ctx context.Context, key K) error
    List(ctx context.Context) ([]V, error)
}
```

### 2. Generic Storage Interface

The `Storer[K,V]` generic interface enables type-safe storage across all backends:

```go
// Usage examples
AgentInfoStore  = Storer[string, identity.AgentInfo]
TodoStore       = Storer[string, []TodoItem]
MessageStore    = Storer[string, *Message]
```

### 3. Message Router with Offline Support

The router maintains per-agent connection maps and offline queues:

```go
type Router struct {
    conns      map[string]interfaces.Connection
    offlineQ   map[string][]*Message
    maxOffline int  // 1000 messages max
}
```

### 4. CRDT-Based TaskBoard

Todo lists use CRDT semantics for conflict-free merging across agents.

---

## Testing Strategy

### Coverage Requirement: 90%+

| Test Type | Scope | Tools |
|-----------|-------|-------|
| Unit Tests | Individual functions | `testing`, mocks |
| Integration | Service interaction | In-memory stores |
| E2E | Full agent scenarios | Test servers |

### Mock-Based Testing

Interfaces enable easy mocking:

```go
type MockTransport struct {
    DialFunc   func(string) (Connection, error)
    ListenFunc func(string) (Listener, error)
}
```

### Test File Organization

```
pkg/
├── identity/
│   ├── types.go
│   └── types_test.go
├── messaging/
│   ├── router.go
│   └── router_test.go
└── common/
    ├── utils.go
    └── utils_test.go
```

---

## Deployment

### Single-Host, In-Memory (Current)

```yaml
deployment:
  mode: single
  storage: memory
  transport: websocket
  max_agents: 1000
  max_message_queue: 10000
```

### Future: Distributed Deployment

The interface-based design enables scaling:

```yaml
deployment:
  mode: distributed
  storage: redis      # Replace memory with Redis
  transport: websocket
  registry: consul    # Service discovery
```

---

## API Summary

### Server Tools

| Tool | Input | Output | Description |
|------|-------|--------|-------------|
| `identity_register` | `Identity` | `Success` | Register agent |
| `identity_rename` | `agentID, name` | `Identity` | Rename identity |
| `identity_get` | `agentID` | `Identity` | Get identity |
| `identity_list` | `filter` | `[]AgentInfo` | Query agents |
| `message_send` | `to, message` | `MessageID` | 1:1 message |
| `message_broadcast` | `message` | `MessageID` | Broadcast |
| `message_get_pending` | `since` | `[]Message` | Get missed |
| `taskboard_publish` | `todos` | `Version` | Share todos |
| `taskboard_get` | `agentID` | `[]TodoItem` | Get todos |

---

## Success Metrics

| Metric | Target |
|--------|--------|
| Function length | < 20 lines |
| Interface size | < 5 methods |
| Code duplication | 0% |
| **Test coverage** | **> 90%** |
| Cyclomatic complexity | < 10 per function |
| Dependencies per package | < 5 |
