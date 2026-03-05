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
	ID               string
	Project          string
	Machine          string
	Agent            string
	FirstMessage     string
	StartedAt        string
	EndedAt          string
	MessageCount     int
	UserMessageCount int
	FileHash         string
	ParentSessionID  string
	RelationshipType string
	CreatedAt        string
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

// Reader provides read-only access to an agentsview SQLite database.
type Reader struct {
	db *sql.DB
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
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Reader{db: db}, nil
}

// ReadSessionsSince returns sessions with created_at > createdAfter, ordered by created_at ASC.
// If createdAfter is empty, all sessions are returned.
func (r *Reader) ReadSessionsSince(createdAfter string) ([]Session, error) {
	var rows *sql.Rows
	var err error

	query := `SELECT id, project, machine, agent,
		COALESCE(first_message, ''), COALESCE(started_at, ''), COALESCE(ended_at, ''),
		message_count, user_message_count, COALESCE(file_hash, ''),
		COALESCE(parent_session_id, ''), relationship_type, created_at
		FROM sessions`

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
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(
			&s.ID, &s.Project, &s.Machine, &s.Agent,
			&s.FirstMessage, &s.StartedAt, &s.EndedAt,
			&s.MessageCount, &s.UserMessageCount, &s.FileHash,
			&s.ParentSessionID, &s.RelationshipType, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, s)
	}

	return sessions, rows.Err()
}

// ReadMessagesForSession returns messages for a session ordered by ordinal ASC.
func (r *Reader) ReadMessagesForSession(sessionID string) ([]Message, error) {
	rows, err := r.db.Query(`SELECT session_id, ordinal, role, content,
		COALESCE(timestamp, ''), has_thinking, has_tool_use, content_length
		FROM messages
		WHERE session_id = ?
		ORDER BY ordinal ASC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		var hasThinking, hasToolUse int
		if err := rows.Scan(
			&m.SessionID, &m.Ordinal, &m.Role, &m.Content,
			&m.Timestamp, &hasThinking, &hasToolUse, &m.ContentLength,
		); err != nil {
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
	defer rows.Close()

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

// Close closes the database connection.
func (r *Reader) Close() error {
	return r.db.Close()
}
