// ABOUTME: Read-only access to agentsview SQLite data for the sync pipeline.
// ABOUTME: Queries sessions, messages, and tool calls incrementally with explicit column lists.
package reader

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

// Session represents an agentsview session record.
type Session struct {
	ID                string
	Project           string
	Machine           string
	Agent             string
	FirstMessage      string
	StartedAt         string
	EndedAt           string
	MessageCount      int
	UserMessageCount  int
	FileHash          string
	ParentSessionID   string
	RelationshipType  string
	CreatedAt         string
	DisplayName       string
	TotalOutputTokens int
	PeakContextTokens int
}

// Message represents an agentsview message record.
type Message struct {
	SessionID     string
	Ordinal       int
	Role          string
	Content       string
	Timestamp     string
	HasThinking   bool
	HasToolUse    bool
	ContentLength int
	Model         string
	TokenUsage    string
	ContextTokens int
	OutputTokens  int
}

// ToolCall represents an agentsview tool call record.
type ToolCall struct {
	MessageOrdinal      int
	SessionID           string
	ToolName            string
	Category            string
	ToolUseID           string
	InputJSON           string
	SkillName           string
	ResultContentLength int
	ResultContent       string
	SubagentSessionID   string
}

// PinnedMessage represents a pinned message from the agentsview database.
type PinnedMessage struct {
	SessionID      string
	MessageOrdinal int
	Note           string
	CreatedAt      string
}

// Reader provides read-only access to an agentsview SQLite database.
type Reader struct {
	db                    *sql.DB
	hasSessionTokenFields bool
	hasMessageTokenFields bool
	hasStarredTable       bool
	hasPinnedTable        bool
	hasDeletedAt          bool
}

// NewReader opens the agentsview SQLite database in read-only mode.
func NewReader(dbPath string) (*Reader, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("database not found: %w", err)
	}

	dsn := "file:" + dbPath + "?mode=ro&_busy_timeout=5000"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Verify the connection works
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	r := &Reader{db: db}
	r.hasSessionTokenFields = tableHasColumn(db, "sessions", "total_output_tokens")
	r.hasMessageTokenFields = tableHasColumn(db, "messages", "model")
	r.hasStarredTable = tableExists(db, "starred_sessions")
	r.hasPinnedTable = tableExists(db, "pinned_messages")
	r.hasDeletedAt = tableHasColumn(db, "sessions", "deleted_at")

	return r, nil
}

// tableHasColumn checks whether a SQLite table contains the named column.
func tableHasColumn(db *sql.DB, table, column string) bool {
	rows, err := db.Query(fmt.Sprintf("SELECT name FROM pragma_table_info('%s')", table))
	if err != nil {
		return false
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

// tableExists checks whether a SQLite database contains the named table.
func tableExists(db *sql.DB, table string) bool {
	var name string
	err := db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
	).Scan(&name)
	return err == nil
}

// ReadSessionsSince returns sessions with created_at > createdAfter, ordered by created_at ASC.
// If createdAfter is empty, all sessions are returned.
func (r *Reader) ReadSessionsSince(createdAfter string) ([]Session, error) {
	var rows *sql.Rows
	var err error

	query := `SELECT id, project, machine, agent,
		COALESCE(first_message, ''), COALESCE(started_at, ''), COALESCE(ended_at, ''),
		message_count, user_message_count, COALESCE(file_hash, ''),
		COALESCE(parent_session_id, ''), relationship_type, created_at`

	if r.hasSessionTokenFields {
		query += `,
		COALESCE(display_name, ''), COALESCE(total_output_tokens, 0), COALESCE(peak_context_tokens, 0)`
	}

	query += ` FROM sessions`

	if createdAfter == "" {
		query += " ORDER BY created_at ASC"
		rows, err = r.db.Query(query)
	} else {
		query += " WHERE created_at > ? ORDER BY created_at ASC"
		rows, err = r.db.Query(query, createdAfter)
	}

	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sessions []Session
	for rows.Next() {
		var s Session
		scanArgs := []any{
			&s.ID, &s.Project, &s.Machine, &s.Agent,
			&s.FirstMessage, &s.StartedAt, &s.EndedAt,
			&s.MessageCount, &s.UserMessageCount, &s.FileHash,
			&s.ParentSessionID, &s.RelationshipType, &s.CreatedAt,
		}
		if r.hasSessionTokenFields {
			scanArgs = append(scanArgs, &s.DisplayName, &s.TotalOutputTokens, &s.PeakContextTokens)
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, s)
	}

	return sessions, rows.Err()
}

// ReadMessagesForSession returns messages for a session ordered by ordinal ASC.
func (r *Reader) ReadMessagesForSession(sessionID string) ([]Message, error) {
	query := `SELECT session_id, ordinal, role, content,
		COALESCE(timestamp, ''), has_thinking, has_tool_use, content_length`

	if r.hasMessageTokenFields {
		query += `,
		COALESCE(model, ''), COALESCE(token_usage, ''), COALESCE(context_tokens, 0), COALESCE(output_tokens, 0)`
	}

	query += `
		FROM messages
		WHERE session_id = ?
		ORDER BY ordinal ASC`

	rows, err := r.db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var messages []Message
	for rows.Next() {
		var m Message
		var hasThinking, hasToolUse int
		scanArgs := []any{
			&m.SessionID, &m.Ordinal, &m.Role, &m.Content,
			&m.Timestamp, &hasThinking, &hasToolUse, &m.ContentLength,
		}
		if r.hasMessageTokenFields {
			scanArgs = append(scanArgs, &m.Model, &m.TokenUsage, &m.ContextTokens, &m.OutputTokens)
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		m.HasThinking = hasThinking != 0
		m.HasToolUse = hasToolUse != 0
		messages = append(messages, m)
	}

	return messages, rows.Err()
}

// ReadToolCallsForSession returns tool calls for a session.
// The returned MessageID is the message ordinal (not the messages table PK).
func (r *Reader) ReadToolCallsForSession(sessionID string) ([]ToolCall, error) {
	rows, err := r.db.Query(`SELECT m.ordinal, tc.session_id, tc.tool_name, tc.category,
		COALESCE(tc.tool_use_id, ''), COALESCE(tc.input_json, ''),
		COALESCE(tc.skill_name, ''), COALESCE(tc.result_content_length, 0),
		COALESCE(tc.result_content, ''),
		COALESCE(tc.subagent_session_id, '')
		FROM tool_calls tc
		JOIN messages m ON tc.message_id = m.id
		WHERE tc.session_id = ?`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query tool calls: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var toolCalls []ToolCall
	for rows.Next() {
		var tc ToolCall
		if err := rows.Scan(
			&tc.MessageOrdinal, &tc.SessionID, &tc.ToolName, &tc.Category,
			&tc.ToolUseID, &tc.InputJSON, &tc.SkillName,
			&tc.ResultContentLength, &tc.ResultContent, &tc.SubagentSessionID,
		); err != nil {
			return nil, fmt.Errorf("scan tool call: %w", err)
		}
		toolCalls = append(toolCalls, tc)
	}

	return toolCalls, rows.Err()
}

// ReadStarredSessionIDs returns session IDs from the starred_sessions table.
// Returns an empty slice if the table doesn't exist.
func (r *Reader) ReadStarredSessionIDs() ([]string, error) {
	if !r.hasStarredTable {
		return []string{}, nil
	}

	rows, err := r.db.Query(`SELECT session_id FROM starred_sessions`)
	if err != nil {
		return nil, fmt.Errorf("query starred sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan starred session: %w", err)
		}
		ids = append(ids, id)
	}

	if ids == nil {
		return []string{}, rows.Err()
	}
	return ids, rows.Err()
}

// ReadPinnedMessages returns all pinned messages from the pinned_messages table.
// Returns an empty slice if the table doesn't exist.
func (r *Reader) ReadPinnedMessages() ([]PinnedMessage, error) {
	if !r.hasPinnedTable {
		return []PinnedMessage{}, nil
	}

	rows, err := r.db.Query(`SELECT session_id, ordinal, COALESCE(note, ''), created_at
		FROM pinned_messages`)
	if err != nil {
		return nil, fmt.Errorf("query pinned messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var pins []PinnedMessage
	for rows.Next() {
		var p PinnedMessage
		if err := rows.Scan(&p.SessionID, &p.MessageOrdinal, &p.Note, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pinned message: %w", err)
		}
		pins = append(pins, p)
	}

	if pins == nil {
		return []PinnedMessage{}, rows.Err()
	}
	return pins, rows.Err()
}

// ReadDeletedSessionIDs returns session IDs where deleted_at IS NOT NULL.
// Returns an empty slice if the deleted_at column doesn't exist.
func (r *Reader) ReadDeletedSessionIDs() ([]string, error) {
	if !r.hasDeletedAt {
		return []string{}, nil
	}

	rows, err := r.db.Query(`SELECT id FROM sessions WHERE deleted_at IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("query deleted sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan deleted session: %w", err)
		}
		ids = append(ids, id)
	}

	if ids == nil {
		return []string{}, rows.Err()
	}
	return ids, rows.Err()
}

// Close closes the database connection.
func (r *Reader) Close() error {
	return r.db.Close()
}
