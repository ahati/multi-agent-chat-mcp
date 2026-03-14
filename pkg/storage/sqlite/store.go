// Package sqlite provides SQLite storage implementations
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"multi-agent-mcp/pkg/common"
	"multi-agent-mcp/pkg/identity"
	"multi-agent-mcp/pkg/messaging"

	_ "github.com/mattn/go-sqlite3"
)

// Store provides SQLite storage
type Store struct {
	db *sql.DB
}

// NewStore creates a new SQLite store
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	store := &Store{db: db}
	if err := store.initTables(); err != nil {
		return nil, err
	}

	return store, nil
}

// Close closes the database
func (s *Store) Close() error {
	return s.db.Close()
}

// initTables creates required tables
func (s *Store) initTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS agents (
		agent_id TEXT PRIMARY KEY,
		display_name TEXT NOT NULL,
		capabilities TEXT,
		status TEXT,
		current_task TEXT,
		last_seen_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		message_type TEXT NOT NULL,
		from_agent TEXT NOT NULL,
		to_agent TEXT,
		task_id TEXT,
		priority INTEGER,
		payload BLOB,
		timestamp DATETIME,
		delivered BOOLEAN DEFAULT FALSE,
		delivered_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS offline_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id TEXT NOT NULL,
		message_id TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (message_id) REFERENCES messages(id)
	);

	CREATE TABLE IF NOT EXISTS subscriptions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id TEXT NOT NULL,
		message_type TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(agent_id, message_type)
	);

	CREATE TABLE IF NOT EXISTS agent_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event_type TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		data BLOB,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_messages_to ON messages(to_agent);
	CREATE INDEX IF NOT EXISTS idx_messages_delivered ON messages(delivered);
	CREATE INDEX IF NOT EXISTS idx_offline_agent ON offline_messages(agent_id);
	CREATE INDEX IF NOT EXISTS idx_events_agent ON agent_events(agent_id);
	`

	_, err := s.db.Exec(schema)
	return err
}

// SaveAgent saves agent info
func (s *Store) SaveAgent(ctx context.Context, info identity.AgentInfo) error {
	caps := common.MustMarshalJSON(info.Capabilities)

	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO agents
		(agent_id, display_name, capabilities, status, current_task, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, info.AgentID, info.DisplayName, caps, info.Status, info.CurrentTask, info.LastSeenAt)

	return err
}

// GetAgent retrieves agent info
func (s *Store) GetAgent(ctx context.Context, agentID string) (identity.AgentInfo, error) {
	var info identity.AgentInfo
	var capsJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT agent_id, display_name, capabilities, status, current_task, last_seen_at
		FROM agents WHERE agent_id = ?
	`, agentID).Scan(&info.AgentID, &info.DisplayName, &capsJSON, &info.Status, &info.CurrentTask, &info.LastSeenAt)

	if err == sql.ErrNoRows {
		return info, common.ErrNotFound
	}
	if err != nil {
		return info, err
	}

	common.UnmarshalJSON(capsJSON, &info.Capabilities)
	return info, nil
}

// ListAgents lists all agents
func (s *Store) ListAgents(ctx context.Context) ([]identity.AgentInfo, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT agent_id, display_name, capabilities, status, current_task, last_seen_at
		FROM agents
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []identity.AgentInfo
	for rows.Next() {
		var info identity.AgentInfo
		var capsJSON []byte

		err := rows.Scan(&info.AgentID, &info.DisplayName, &capsJSON, &info.Status, &info.CurrentTask, &info.LastSeenAt)
		if err != nil {
			continue
		}

		common.UnmarshalJSON(capsJSON, &info.Capabilities)
		agents = append(agents, info)
	}

	return agents, nil
}

// DeleteAgent removes an agent
func (s *Store) DeleteAgent(ctx context.Context, agentID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM agents WHERE agent_id = ?`, agentID)
	return err
}

// SaveMessage stores a message
func (s *Store) SaveMessage(ctx context.Context, msg *messaging.Message) error {
	payload := common.MustMarshalJSON(msg.Payload)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO messages (id, message_type, from_agent, to_agent, task_id, priority, payload, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.ID, msg.Type, msg.From, msg.To, msg.TaskID, msg.Priority, payload, msg.Timestamp)

	return err
}

// QueueOfflineMessage adds message to offline queue for agent
func (s *Store) QueueOfflineMessage(ctx context.Context, agentID string, msgID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO offline_messages (agent_id, message_id)
		VALUES (?, ?)
	`, agentID, msgID)
	return err
}

// GetOfflineMessages retrieves queued messages for agent
func (s *Store) GetOfflineMessages(ctx context.Context, agentID string) ([]*messaging.Message, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.message_type, m.from_agent, m.to_agent, m.task_id, m.priority, m.payload, m.timestamp
		FROM messages m
		JOIN offline_messages om ON m.id = om.message_id
		WHERE om.agent_id = ?
		ORDER BY m.timestamp ASC
	`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*messaging.Message
	for rows.Next() {
		msg := &messaging.Message{}
		var payload []byte

		err := rows.Scan(&msg.ID, &msg.Type, &msg.From, &msg.To, &msg.TaskID, &msg.Priority, &payload, &msg.Timestamp)
		if err != nil {
			continue
		}

		msg.Payload = payload
		messages = append(messages, msg)
	}

	return messages, nil
}

// ClearOfflineMessages removes queued messages for agent
func (s *Store) ClearOfflineMessages(ctx context.Context, agentID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM offline_messages WHERE agent_id = ?`, agentID)
	return err
}

// SaveSubscription saves agent subscription preference
func (s *Store) SaveSubscription(ctx context.Context, agentID string, msgType string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO subscriptions (agent_id, message_type)
		VALUES (?, ?)
	`, agentID, msgType)
	return err
}

// GetSubscriptions retrieves agent's subscriptions
func (s *Store) GetSubscriptions(ctx context.Context, agentID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT message_type FROM subscriptions WHERE agent_id = ?
	`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			types = append(types, t)
		}
	}

	return types, nil
}

// DeleteSubscription removes a subscription
func (s *Store) DeleteSubscription(ctx context.Context, agentID string, msgType string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM subscriptions WHERE agent_id = ? AND message_type = ?
	`, agentID, msgType)
	return err
}

// LogEvent logs a server event
func (s *Store) LogEvent(ctx context.Context, eventType, agentID string, data []byte) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_events (event_type, agent_id, data)
		VALUES (?, ?, ?)
	`, eventType, agentID, data)
	return err
}

// GetRecentEvents retrieves recent events for agent
func (s *Store) GetRecentEvents(ctx context.Context, agentID string, since time.Time) ([]AgentEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT event_type, agent_id, data, created_at
		FROM agent_events
		WHERE agent_id = ? AND created_at > ?
		ORDER BY created_at DESC
	`, agentID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []AgentEvent
	for rows.Next() {
		var ev AgentEvent
		err := rows.Scan(&ev.EventType, &ev.AgentID, &ev.Data, &ev.CreatedAt)
		if err != nil {
			continue
		}
		events = append(events, ev)
	}

	return events, nil
}

// AgentEvent represents a logged event
type AgentEvent struct {
	EventType string
	AgentID   string
	Data      []byte
	CreatedAt time.Time
}
