// ABOUTME: Tests for read-only agentsview SQLite reader.
// ABOUTME: Uses a fixture DB created in TestMain with known sessions, messages, and tool calls.
package reader

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDBPath string
var legacyDBPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "reader-test-*")
	if err != nil {
		panic(err)
	}

	testDBPath = filepath.Join(dir, "sessions.db")
	createTestDB(testDBPath, true)

	legacyDBPath = filepath.Join(dir, "legacy.db")
	createTestDB(legacyDBPath, false)

	code := m.Run()

	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// createTestDB builds a fixture SQLite DB. When includeNewColumns is true,
// the schema includes the newer agentsview columns (display_name,
// total_output_tokens, peak_context_tokens on sessions; model, token_usage,
// context_tokens, output_tokens on messages).
func createTestDB(path string, includeNewColumns bool) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		panic(err)
	}
	defer func() { _ = db.Close() }()

	sessionsCols := ""
	if includeNewColumns {
		sessionsCols = `,
			display_name TEXT,
			total_output_tokens INTEGER,
			peak_context_tokens INTEGER`
	}

	messagesCols := ""
	if includeNewColumns {
		messagesCols = `,
			model TEXT,
			token_usage TEXT,
			context_tokens INTEGER,
			output_tokens INTEGER`
	}

	// Create schema matching agentsview
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id          TEXT PRIMARY KEY,
			project     TEXT NOT NULL,
			machine     TEXT NOT NULL DEFAULT 'local',
			agent       TEXT NOT NULL DEFAULT 'claude',
			first_message TEXT,
			started_at  TEXT,
			ended_at    TEXT,
			message_count INTEGER NOT NULL DEFAULT 0,
			user_message_count INTEGER NOT NULL DEFAULT 0,
			file_path   TEXT,
			file_size   INTEGER,
			file_mtime  INTEGER,
			file_hash   TEXT,
			parent_session_id TEXT,
			relationship_type TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))` + sessionsCols + `
		);

		CREATE TABLE IF NOT EXISTS messages (
			id             INTEGER PRIMARY KEY,
			session_id     TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			ordinal        INTEGER NOT NULL,
			role           TEXT NOT NULL,
			content        TEXT NOT NULL,
			timestamp      TEXT,
			has_thinking   INTEGER NOT NULL DEFAULT 0,
			has_tool_use   INTEGER NOT NULL DEFAULT 0,
			content_length INTEGER NOT NULL DEFAULT 0` + messagesCols + `,
			UNIQUE(session_id, ordinal)
		);

		CREATE TABLE IF NOT EXISTS tool_calls (
			id         INTEGER PRIMARY KEY,
			message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			tool_name  TEXT NOT NULL,
			category   TEXT NOT NULL,
			tool_use_id TEXT,
			input_json  TEXT,
			skill_name  TEXT,
			result_content_length INTEGER,
			result_content TEXT,
			subagent_session_id TEXT
		);
	`)
	if err != nil {
		panic(err)
	}

	if includeNewColumns {
		_, err = db.Exec(`
			INSERT INTO sessions (id, project, machine, agent, first_message, started_at, ended_at,
				message_count, user_message_count, file_hash, parent_session_id, relationship_type, created_at,
				display_name, total_output_tokens, peak_context_tokens)
			VALUES
				('sess-1', 'proj-a', 'laptop', 'claude', 'Hello world', '2026-01-01T10:00:00Z', '2026-01-01T11:00:00Z',
				 5, 2, 'hash1', '', '', '2026-01-01T10:00:00.000Z',
				 'My Chat Session', 15000, 80000),
				('sess-2', 'proj-a', 'laptop', 'cursor', 'Fix bug', '2026-01-02T10:00:00Z', NULL,
				 3, 1, 'hash2', 'sess-1', 'continuation', '2026-01-02T10:00:00.000Z',
				 NULL, NULL, NULL),
				('sess-3', 'proj-b', 'desktop', 'claude', 'Refactor auth', '2026-01-03T10:00:00Z', '2026-01-03T12:00:00Z',
				 10, 5, 'hash3', '', '', '2026-01-03T10:00:00.000Z',
				 'Auth Refactor', 42000, 120000);
		`)
	} else {
		_, err = db.Exec(`
			INSERT INTO sessions (id, project, machine, agent, first_message, started_at, ended_at,
				message_count, user_message_count, file_hash, parent_session_id, relationship_type, created_at)
			VALUES
				('sess-1', 'proj-a', 'laptop', 'claude', 'Hello world', '2026-01-01T10:00:00Z', '2026-01-01T11:00:00Z',
				 5, 2, 'hash1', '', '', '2026-01-01T10:00:00.000Z'),
				('sess-2', 'proj-a', 'laptop', 'cursor', 'Fix bug', '2026-01-02T10:00:00Z', NULL,
				 3, 1, 'hash2', 'sess-1', 'continuation', '2026-01-02T10:00:00.000Z'),
				('sess-3', 'proj-b', 'desktop', 'claude', 'Refactor auth', '2026-01-03T10:00:00Z', '2026-01-03T12:00:00Z',
				 10, 5, 'hash3', '', '', '2026-01-03T10:00:00.000Z');
		`)
	}
	if err != nil {
		panic(err)
	}

	if includeNewColumns {
		_, err = db.Exec(`
			INSERT INTO messages (id, session_id, ordinal, role, content, timestamp, has_thinking, has_tool_use, content_length,
				model, token_usage, context_tokens, output_tokens)
			VALUES
				(1, 'sess-1', 0, 'user', 'Hello world', '2026-01-01T10:00:00Z', 0, 0, 11,
				 NULL, NULL, 0, 0),
				(2, 'sess-1', 1, 'assistant', 'Hi! How can I help?', '2026-01-01T10:00:01Z', 0, 0, 19,
				 'claude-opus-4-6', '{"input":500,"output":150}', 500, 150),
				(3, 'sess-1', 2, 'user', 'Write a test', '2026-01-01T10:01:00Z', 0, 0, 12,
				 NULL, NULL, 0, 0),
				(4, 'sess-2', 0, 'user', 'Fix bug in auth', '2026-01-02T10:00:00Z', 0, 0, 15,
				 NULL, NULL, 0, 0),
				(5, 'sess-2', 1, 'assistant', 'Looking at it...', '2026-01-02T10:00:01Z', 1, 1, 16,
				 'claude-sonnet-4-20250514', '{"input":2000,"output":800}', 2000, 800),
				(6, 'sess-3', 0, 'user', 'Refactor auth module', '2026-01-03T10:00:00Z', 0, 0, 20,
				 NULL, NULL, 0, 0);
		`)
	} else {
		_, err = db.Exec(`
			INSERT INTO messages (id, session_id, ordinal, role, content, timestamp, has_thinking, has_tool_use, content_length)
			VALUES
				(1, 'sess-1', 0, 'user', 'Hello world', '2026-01-01T10:00:00Z', 0, 0, 11),
				(2, 'sess-1', 1, 'assistant', 'Hi! How can I help?', '2026-01-01T10:00:01Z', 0, 0, 19),
				(3, 'sess-1', 2, 'user', 'Write a test', '2026-01-01T10:01:00Z', 0, 0, 12),
				(4, 'sess-2', 0, 'user', 'Fix bug in auth', '2026-01-02T10:00:00Z', 0, 0, 15),
				(5, 'sess-2', 1, 'assistant', 'Looking at it...', '2026-01-02T10:00:01Z', 1, 1, 16),
				(6, 'sess-3', 0, 'user', 'Refactor auth module', '2026-01-03T10:00:00Z', 0, 0, 20);
		`)
	}
	if err != nil {
		panic(err)
	}

	// Insert test tool calls
	_, err = db.Exec(`
		INSERT INTO tool_calls (id, message_id, session_id, tool_name, category, tool_use_id, input_json, skill_name, result_content_length, result_content, subagent_session_id)
		VALUES
			(1, 5, 'sess-2', 'Read', 'file', 'tc-1', '{"path":"auth.go"}', NULL, 500, NULL, NULL),
			(2, 5, 'sess-2', 'Edit', 'file', 'tc-2', '{"path":"auth.go","old":"bug","new":"fix"}', NULL, 100, NULL, NULL),
			(3, 6, 'sess-3', 'Bash', 'shell', 'tc-3', '{"command":"go test ./..."}', 'testing', 1200, 'PASS\nok  proj/auth 0.5s', 'sub-1');
	`)
	if err != nil {
		panic(err)
	}

	if includeNewColumns {
		// Add deleted_at column and mark sess-2 as deleted
		_, err = db.Exec(`ALTER TABLE sessions ADD COLUMN deleted_at TEXT`)
		if err != nil {
			panic(err)
		}
		_, err = db.Exec(`UPDATE sessions SET deleted_at = '2026-01-05T10:00:00Z' WHERE id = 'sess-2'`)
		if err != nil {
			panic(err)
		}

		// Create starred_sessions table and seed data
		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS starred_sessions (
				session_id TEXT PRIMARY KEY,
				created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
			)`)
		if err != nil {
			panic(err)
		}
		_, err = db.Exec(`
			INSERT INTO starred_sessions (session_id, created_at)
			VALUES
				('sess-1', '2026-01-04T10:00:00.000Z'),
				('sess-3', '2026-01-04T11:00:00.000Z')`)
		if err != nil {
			panic(err)
		}

		// Create pinned_messages table and seed data
		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS pinned_messages (
				id          INTEGER PRIMARY KEY,
				session_id  TEXT NOT NULL,
				message_id  INTEGER NOT NULL,
				ordinal     INTEGER NOT NULL,
				note        TEXT,
				created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
				UNIQUE(session_id, message_id)
			)`)
		if err != nil {
			panic(err)
		}
		_, err = db.Exec(`
			INSERT INTO pinned_messages (id, session_id, message_id, ordinal, note, created_at)
			VALUES
				(1, 'sess-1', 2, 1, 'Great response', '2026-01-04T12:00:00.000Z'),
				(2, 'sess-3', 6, 0, NULL, '2026-01-04T13:00:00.000Z')`)
		if err != nil {
			panic(err)
		}
	}
}

func TestNewReaderOpensReadOnly(t *testing.T) {
	r, err := NewReader(testDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	// Attempting to write should fail since connection is read-only
	_, err = r.db.Exec("INSERT INTO sessions (id, project) VALUES ('x', 'y')")
	assert.Error(t, err, "should not allow writes on read-only connection")
}

func TestReadSessionsSinceReturnsAll(t *testing.T) {
	r, err := NewReader(testDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	sessions, err := r.ReadSessionsSince("")
	require.NoError(t, err)
	assert.Len(t, sessions, 3)

	// Should be ordered by created_at ASC
	assert.Equal(t, "sess-1", sessions[0].ID)
	assert.Equal(t, "sess-2", sessions[1].ID)
	assert.Equal(t, "sess-3", sessions[2].ID)
}

func TestReadSessionsSinceIncremental(t *testing.T) {
	r, err := NewReader(testDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	sessions, err := r.ReadSessionsSince("2026-01-01T10:00:00.000Z")
	require.NoError(t, err)
	assert.Len(t, sessions, 2, "should return sessions after the threshold")
	assert.Equal(t, "sess-2", sessions[0].ID)
	assert.Equal(t, "sess-3", sessions[1].ID)
}

func TestReadSessionsSinceFieldValues(t *testing.T) {
	r, err := NewReader(testDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	sessions, err := r.ReadSessionsSince("")
	require.NoError(t, err)
	require.Len(t, sessions, 3)

	s := sessions[0]
	assert.Equal(t, "sess-1", s.ID)
	assert.Equal(t, "proj-a", s.Project)
	assert.Equal(t, "laptop", s.Machine)
	assert.Equal(t, "claude", s.Agent)
	assert.Equal(t, "Hello world", s.FirstMessage)
	assert.Equal(t, "2026-01-01T10:00:00Z", s.StartedAt)
	assert.Equal(t, "2026-01-01T11:00:00Z", s.EndedAt)
	assert.Equal(t, 5, s.MessageCount)
	assert.Equal(t, 2, s.UserMessageCount)
	assert.Equal(t, "hash1", s.FileHash)
	assert.Equal(t, "", s.ParentSessionID)
	assert.Equal(t, "", s.RelationshipType)
	assert.Equal(t, "2026-01-01T10:00:00.000Z", s.CreatedAt)

	// New fields
	assert.Equal(t, "My Chat Session", s.DisplayName)
	assert.Equal(t, 15000, s.TotalOutputTokens)
	assert.Equal(t, 80000, s.PeakContextTokens)

	// Check session with parent (and NULL new fields)
	s2 := sessions[1]
	assert.Equal(t, "sess-1", s2.ParentSessionID)
	assert.Equal(t, "continuation", s2.RelationshipType)
	assert.Equal(t, "", s2.DisplayName)
	assert.Equal(t, 0, s2.TotalOutputTokens)
	assert.Equal(t, 0, s2.PeakContextTokens)
}

func TestReadMessagesForSession(t *testing.T) {
	r, err := NewReader(testDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	msgs, err := r.ReadMessagesForSession("sess-1")
	require.NoError(t, err)
	assert.Len(t, msgs, 3)

	// Ordered by ordinal ASC
	assert.Equal(t, 0, msgs[0].Ordinal)
	assert.Equal(t, 1, msgs[1].Ordinal)
	assert.Equal(t, 2, msgs[2].Ordinal)

	assert.Equal(t, "sess-1", msgs[0].SessionID)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "Hello world", msgs[0].Content)
	assert.Equal(t, "2026-01-01T10:00:00Z", msgs[0].Timestamp)
	assert.Equal(t, false, msgs[0].HasThinking)
	assert.Equal(t, false, msgs[0].HasToolUse)
	assert.Equal(t, 11, msgs[0].ContentLength)

	// New fields — user message has no model/token data
	assert.Equal(t, "", msgs[0].Model)
	assert.Equal(t, "", msgs[0].TokenUsage)
	assert.Equal(t, 0, msgs[0].ContextTokens)
	assert.Equal(t, 0, msgs[0].OutputTokens)

	// Assistant message has model and token data
	assert.Equal(t, "claude-opus-4-6", msgs[1].Model)
	assert.Equal(t, `{"input":500,"output":150}`, msgs[1].TokenUsage)
	assert.Equal(t, 500, msgs[1].ContextTokens)
	assert.Equal(t, 150, msgs[1].OutputTokens)
}

func TestReadMessagesForSessionWithThinking(t *testing.T) {
	r, err := NewReader(testDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	msgs, err := r.ReadMessagesForSession("sess-2")
	require.NoError(t, err)
	require.Len(t, msgs, 2)

	assert.True(t, msgs[1].HasThinking)
	assert.True(t, msgs[1].HasToolUse)
}

func TestReadMessagesForSessionEmpty(t *testing.T) {
	r, err := NewReader(testDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	msgs, err := r.ReadMessagesForSession("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestReadToolCallsForSession(t *testing.T) {
	r, err := NewReader(testDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	tcs, err := r.ReadToolCallsForSession("sess-2")
	require.NoError(t, err)
	assert.Len(t, tcs, 2)

	// MessageOrdinal is the message's ordinal (not the messages table PK).
	// message_id=5 → messages row with ordinal=1
	assert.Equal(t, 1, tcs[0].MessageOrdinal)
	assert.Equal(t, "sess-2", tcs[0].SessionID)
	assert.Equal(t, "Read", tcs[0].ToolName)
	assert.Equal(t, "file", tcs[0].Category)
	assert.Equal(t, "tc-1", tcs[0].ToolUseID)
	assert.Equal(t, `{"path":"auth.go"}`, tcs[0].InputJSON)
	assert.Equal(t, "", tcs[0].SkillName)
	assert.Equal(t, 500, tcs[0].ResultContentLength)
	assert.Equal(t, "", tcs[0].ResultContent)
	assert.Equal(t, "", tcs[0].SubagentSessionID)
}

func TestReadToolCallsForSessionWithSubagent(t *testing.T) {
	r, err := NewReader(testDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	tcs, err := r.ReadToolCallsForSession("sess-3")
	require.NoError(t, err)
	require.Len(t, tcs, 1)

	assert.Equal(t, "Bash", tcs[0].ToolName)
	assert.Equal(t, "testing", tcs[0].SkillName)
	assert.Equal(t, 1200, tcs[0].ResultContentLength)
	assert.Equal(t, `PASS\nok  proj/auth 0.5s`, tcs[0].ResultContent)
	assert.Equal(t, "sub-1", tcs[0].SubagentSessionID)
}

func TestReadToolCallsForSessionEmpty(t *testing.T) {
	r, err := NewReader(testDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	tcs, err := r.ReadToolCallsForSession("sess-1")
	require.NoError(t, err)
	assert.Empty(t, tcs)
}

func TestReaderClose(t *testing.T) {
	r, err := NewReader(testDBPath)
	require.NoError(t, err)

	err = r.Close()
	assert.NoError(t, err)

	// After closing, queries should fail
	_, err = r.ReadSessionsSince("")
	assert.Error(t, err)
}

func TestNewReaderFailsOnMissingDB(t *testing.T) {
	_, err := NewReader("/nonexistent/path/sessions.db")
	assert.Error(t, err)
}

func TestLegacyDBSessionsReturnDefaults(t *testing.T) {
	r, err := NewReader(legacyDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	sessions, err := r.ReadSessionsSince("")
	require.NoError(t, err)
	require.Len(t, sessions, 3)

	// All existing fields should still work
	assert.Equal(t, "sess-1", sessions[0].ID)
	assert.Equal(t, "proj-a", sessions[0].Project)
	assert.Equal(t, "Hello world", sessions[0].FirstMessage)

	// New fields should be zero values
	assert.Equal(t, "", sessions[0].DisplayName)
	assert.Equal(t, 0, sessions[0].TotalOutputTokens)
	assert.Equal(t, 0, sessions[0].PeakContextTokens)
}

func TestLegacyDBMessagesReturnDefaults(t *testing.T) {
	r, err := NewReader(legacyDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	msgs, err := r.ReadMessagesForSession("sess-1")
	require.NoError(t, err)
	require.Len(t, msgs, 3)

	// All existing fields should still work
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "Hello world", msgs[0].Content)
	assert.Equal(t, 11, msgs[0].ContentLength)

	// New fields should be zero values
	assert.Equal(t, "", msgs[0].Model)
	assert.Equal(t, "", msgs[0].TokenUsage)
	assert.Equal(t, 0, msgs[0].ContextTokens)
	assert.Equal(t, 0, msgs[0].OutputTokens)

	// Same for assistant messages
	assert.Equal(t, "", msgs[1].Model)
	assert.Equal(t, "", msgs[1].TokenUsage)
	assert.Equal(t, 0, msgs[1].ContextTokens)
	assert.Equal(t, 0, msgs[1].OutputTokens)
}

func TestLegacyDBToolCallsStillWork(t *testing.T) {
	r, err := NewReader(legacyDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	tcs, err := r.ReadToolCallsForSession("sess-2")
	require.NoError(t, err)
	assert.Len(t, tcs, 2)
	assert.Equal(t, "Read", tcs[0].ToolName)
}

func TestReadStarredSessionIDs(t *testing.T) {
	r, err := NewReader(testDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	ids, err := r.ReadStarredSessionIDs()
	require.NoError(t, err)
	assert.Len(t, ids, 2)
	assert.Contains(t, ids, "sess-1")
	assert.Contains(t, ids, "sess-3")
}

func TestReadPinnedMessages(t *testing.T) {
	r, err := NewReader(testDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	pins, err := r.ReadPinnedMessages()
	require.NoError(t, err)
	assert.Len(t, pins, 2)

	assert.Equal(t, "sess-1", pins[0].SessionID)
	assert.Equal(t, 1, pins[0].MessageOrdinal)
	assert.Equal(t, "Great response", pins[0].Note)
	assert.Equal(t, "2026-01-04T12:00:00.000Z", pins[0].CreatedAt)

	assert.Equal(t, "sess-3", pins[1].SessionID)
	assert.Equal(t, 0, pins[1].MessageOrdinal)
	assert.Equal(t, "", pins[1].Note)
	assert.Equal(t, "2026-01-04T13:00:00.000Z", pins[1].CreatedAt)
}

func TestReadDeletedSessionIDs(t *testing.T) {
	r, err := NewReader(testDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	ids, err := r.ReadDeletedSessionIDs()
	require.NoError(t, err)
	assert.Len(t, ids, 1)
	assert.Equal(t, "sess-2", ids[0])
}

func TestLegacyDBStarredReturnsEmpty(t *testing.T) {
	r, err := NewReader(legacyDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	ids, err := r.ReadStarredSessionIDs()
	require.NoError(t, err)
	assert.Empty(t, ids)
	assert.NotNil(t, ids)
}

func TestLegacyDBPinnedReturnsEmpty(t *testing.T) {
	r, err := NewReader(legacyDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	pins, err := r.ReadPinnedMessages()
	require.NoError(t, err)
	assert.Empty(t, pins)
	assert.NotNil(t, pins)
}

func TestLegacyDBDeletedReturnsEmpty(t *testing.T) {
	r, err := NewReader(legacyDBPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	ids, err := r.ReadDeletedSessionIDs()
	require.NoError(t, err)
	assert.Empty(t, ids)
	assert.NotNil(t, ids)
}
