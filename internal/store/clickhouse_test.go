// ABOUTME: Integration tests for the ClickHouse store implementation.
// ABOUTME: Each test creates a unique temporary ClickHouse database for isolation.

package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clickhouseAddr() string {
	if addr := os.Getenv("CLICKHOUSE_ADDR"); addr != "" {
		return addr
	}
	return "host.docker.internal:9440"
}

func clickhouseUser() string {
	if u := os.Getenv("CLICKHOUSE_USER"); u != "" {
		return u
	}
	return "agentstrove"
}

func clickhousePassword() string {
	return os.Getenv("CLICKHOUSE_PASSWORD")
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func setupTestStore(t *testing.T) *ClickHouseStore {
	t.Helper()
	addr := clickhouseAddr()
	user := clickhouseUser()
	password := clickhousePassword()
	dbName := fmt.Sprintf("test_%s", randomHex(8))

	// Constructor bootstraps the database via "default" connection
	store, err := NewClickHouseStoreWithAuth(addr, dbName, user, password)
	require.NoError(t, err, "create store")
	require.NoError(t, store.EnsureSchema(context.Background()), "ensure schema")

	t.Cleanup(func() {
		_ = store.Close()
		dropConn, err := clickhouse.Open(&clickhouse.Options{
			Addr:     []string{addr},
			Protocol: clickhouse.Native,
			Auth: clickhouse.Auth{
				Username: user,
				Password: password,
			},
		})
		if err == nil {
			_ = dropConn.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbName)
			_ = dropConn.Close()
		}
	})

	return store
}

// testTime returns a UTC time for use in test data.
func testTime(year, month, day, hour int) *time.Time {
	t := time.Date(year, time.Month(month), day, hour, 0, 0, 0, time.UTC)
	return &t
}

func seedTestData(t *testing.T, s *ClickHouseStore) {
	t.Helper()
	ctx := context.Background()
	orgID := ""

	sessions := []Session{
		// Session 1: alice, frontend project, claude-code
		{
			ID:               "session-1",
			UserID:           "alice@test.com",
			UserName:         "Alice",
			ProjectID:        "proj-frontend",
			ProjectName:      "frontend",
			ProjectPath:      "/home/alice/frontend",
			AgentType:        "claude-code",
			FirstMessage:     "Help me with React hooks",
			StartedAt:        testTime(2024, 1, 5, 10),
			EndedAt:          testTime(2024, 1, 5, 11),
			MessageCount:     4,
			UserMessageCount: 2,
			ParentSessionID:  "",
			RelationshipType: "",
			Machine:          "macbook",
			SourceCreatedAt:  "2024-01-05",
		},
		// Session 2: alice, backend project, claude-code
		{
			ID:               "session-2",
			UserID:           "alice@test.com",
			UserName:         "Alice",
			ProjectID:        "proj-backend",
			ProjectName:      "backend",
			ProjectPath:      "/home/alice/backend",
			AgentType:        "claude-code",
			FirstMessage:     "Fix the database connection pool",
			StartedAt:        testTime(2024, 1, 4, 9),
			EndedAt:          testTime(2024, 1, 4, 10),
			MessageCount:     6,
			UserMessageCount: 3,
			ParentSessionID:  "",
			RelationshipType: "",
			Machine:          "macbook",
			SourceCreatedAt:  "2024-01-04",
		},
		// Session 3: bob, frontend project, cursor
		{
			ID:               "session-3",
			UserID:           "bob@test.com",
			UserName:         "Bob",
			ProjectID:        "proj-frontend",
			ProjectName:      "frontend",
			ProjectPath:      "/home/bob/frontend",
			AgentType:        "cursor",
			FirstMessage:     "Add TypeScript support",
			StartedAt:        testTime(2024, 1, 3, 14),
			EndedAt:          testTime(2024, 1, 3, 15),
			MessageCount:     2,
			UserMessageCount: 1,
			ParentSessionID:  "",
			RelationshipType: "",
			Machine:          "linux-box",
			SourceCreatedAt:  "2024-01-03",
		},
		// Session 4: ghost session (user_message_count=0) — should be excluded from browsable queries
		{
			ID:               "session-ghost",
			UserID:           "alice@test.com",
			UserName:         "Alice",
			ProjectID:        "proj-frontend",
			ProjectName:      "frontend",
			ProjectPath:      "/home/alice/frontend",
			AgentType:        "claude-code",
			FirstMessage:     "",
			StartedAt:        testTime(2024, 1, 2, 8),
			MessageCount:     0,
			UserMessageCount: 0,
			ParentSessionID:  "",
			Machine:          "macbook",
			SourceCreatedAt:  "2024-01-02",
		},
		// Session 5: subagent session — should be excluded from browsable queries
		{
			ID:               "session-sub",
			UserID:           "alice@test.com",
			UserName:         "Alice",
			ProjectID:        "proj-backend",
			ProjectName:      "backend",
			ProjectPath:      "/home/alice/backend",
			AgentType:        "claude-code",
			FirstMessage:     "Subagent task",
			StartedAt:        testTime(2024, 1, 1, 7),
			MessageCount:     2,
			UserMessageCount: 1,
			ParentSessionID:  "session-2",
			RelationshipType: "subagent",
			Machine:          "macbook",
			SourceCreatedAt:  "2024-01-01",
		},
	}

	for _, sess := range sessions {
		require.NoError(t, s.WriteSession(ctx, orgID, sess, nil, nil),
			"seed session %s", sess.ID)
	}

	// Messages for session-1
	msgs1 := []Message{
		{SessionID: "session-1", Ordinal: 1, Role: "user", Content: "Help me with React hooks", HasToolUse: false, ContentLength: 24},
		{SessionID: "session-1", Ordinal: 2, Role: "assistant", Content: "I'll help you understand React hooks. The useState hook is very useful for managing state.", HasThinking: false, HasToolUse: true, ContentLength: 85},
		{SessionID: "session-1", Ordinal: 3, Role: "user", Content: "Can you show me an example?", HasToolUse: false, ContentLength: 27},
		{SessionID: "session-1", Ordinal: 4, Role: "assistant", Content: "Here's a simple counter example using useState.", HasToolUse: false, ContentLength: 48},
	}

	// Messages for session-2
	msgs2 := []Message{
		{SessionID: "session-2", Ordinal: 1, Role: "user", Content: "Fix the database connection pool", ContentLength: 32},
		{SessionID: "session-2", Ordinal: 2, Role: "assistant", Content: "I'll investigate the connection pool issue.", ContentLength: 43, HasToolUse: true},
		{SessionID: "session-2", Ordinal: 3, Role: "user", Content: "What did you find?", ContentLength: 18},
		{SessionID: "session-2", Ordinal: 4, Role: "assistant", Content: "Found the issue with max_connections setting.", ContentLength: 45},
		{SessionID: "session-2", Ordinal: 5, Role: "user", Content: "Please fix it", ContentLength: 13},
		{SessionID: "session-2", Ordinal: 6, Role: "assistant", Content: "Fixed by updating the pool configuration.", ContentLength: 40, HasToolUse: true},
	}

	// Tool calls for session-1 (message ordinal 2 has tool use)
	tc1 := []ToolCall{
		{
			SessionID:      "session-1",
			MessageOrdinal: 2,
			ToolUseID:      "tool-1a",
			ToolName:       "Bash",
			Category:       "bash",
			InputJSON:      `{"command": "ls -la"}`,
			ResultContent:  "total 42\ndrwxr-xr-x ...",
		},
	}

	// Tool calls for session-2
	tc2 := []ToolCall{
		{
			SessionID:      "session-2",
			MessageOrdinal: 2,
			ToolUseID:      "tool-2a",
			ToolName:       "Bash",
			Category:       "bash",
			InputJSON:      `{"command": "cat config.yaml"}`,
			ResultContent:  "max_connections: 10\npool_size: 5",
		},
		{
			SessionID:      "session-2",
			MessageOrdinal: 6,
			ToolUseID:      "tool-2b",
			ToolName:       "Write",
			Category:       "file",
			InputJSON:      `{"path": "config.yaml", "content": "max_connections: 100"}`,
			ResultContent:  "File written successfully",
		},
	}

	ts1 := testTime(2024, 1, 5, 10)
	for i := range msgs1 {
		msgs1[i].OrgID = orgID
		msgs1[i].Timestamp = ts1
	}
	ts2 := testTime(2024, 1, 4, 9)
	for i := range msgs2 {
		msgs2[i].OrgID = orgID
		msgs2[i].Timestamp = ts2
	}
	for i := range tc1 {
		tc1[i].OrgID = orgID
	}
	for i := range tc2 {
		tc2[i].OrgID = orgID
	}

	require.NoError(t, s.WriteSession(ctx, orgID, sessions[0], msgs1, tc1), "seed session-1 messages")
	require.NoError(t, s.WriteSession(ctx, orgID, sessions[1], msgs2, tc2), "seed session-2 messages")

	// Git links for session-2
	links := []GitLink{
		{
			SessionID:      "session-2",
			UserID:         "alice@test.com",
			MessageOrdinal: 6,
			CommitSHA:      "abc1234def5678",
			PRURL:          "",
			LinkType:       "commit",
			Confidence:     "high",
		},
		{
			SessionID:      "session-2",
			UserID:         "alice@test.com",
			MessageOrdinal: 6,
			CommitSHA:      "",
			PRURL:          "https://github.com/org/repo/pull/42",
			LinkType:       "pr",
			Confidence:     "medium",
		},
	}
	require.NoError(t, s.WriteGitLinks(ctx, orgID, links), "seed git links")
}

func TestEnsureSchema(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// Verify all tables exist by counting rows in each
	for _, table := range []string{"sessions", "messages", "tool_calls", "git_links"} {
		var rows []struct {
			Count uint64 `ch:"count"`
		}
		err := s.conn.Select(ctx, &rows, fmt.Sprintf("SELECT count() AS count FROM %s", table))
		assert.NoError(t, err, "table %s should exist", table)
	}
}

func TestWriteSession(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	orgID := ""

	sess := Session{
		ID:               "write-test-1",
		UserID:           "test@example.com",
		UserName:         "Test User",
		ProjectID:        "proj-1",
		ProjectName:      "myproject",
		ProjectPath:      "/home/test/myproject",
		AgentType:        "claude-code",
		FirstMessage:     "Hello",
		StartedAt:        testTime(2024, 6, 1, 12),
		MessageCount:     2,
		UserMessageCount: 1,
		ParentSessionID:  "",
		Machine:          "laptop",
		SourceCreatedAt:  "2024-06-01",
	}
	msgs := []Message{
		{OrgID: orgID, SessionID: "write-test-1", Ordinal: 1, Role: "user", Content: "Hello", ContentLength: 5},
		{OrgID: orgID, SessionID: "write-test-1", Ordinal: 2, Role: "assistant", Content: "Hi there!", HasToolUse: true, ContentLength: 9},
	}
	size := 42
	tcs := []ToolCall{
		{OrgID: orgID, SessionID: "write-test-1", MessageOrdinal: 2, ToolUseID: "tc-1", ToolName: "Bash", Category: "bash", InputJSON: `{"command":"echo hello"}`, ResultContent: "hello", ResultContentLength: &size},
	}

	require.NoError(t, s.WriteSession(ctx, orgID, sess, msgs, tcs))

	// Verify counts using FINAL to get consistent reads in tests
	type countRow struct{ Count uint64 }
	var sessionRows []countRow
	err := s.conn.Select(ctx, &sessionRows, "SELECT count() AS Count FROM sessions FINAL WHERE id = 'write-test-1'")
	require.NoError(t, err)
	require.Len(t, sessionRows, 1)
	assert.Equal(t, uint64(1), sessionRows[0].Count)

	var msgRows []countRow
	err = s.conn.Select(ctx, &msgRows, "SELECT count() AS Count FROM messages FINAL WHERE session_id = 'write-test-1'")
	require.NoError(t, err)
	require.Len(t, msgRows, 1)
	assert.Equal(t, uint64(2), msgRows[0].Count)

	var tcRows []countRow
	err = s.conn.Select(ctx, &tcRows, "SELECT count() AS Count FROM tool_calls FINAL WHERE session_id = 'write-test-1'")
	require.NoError(t, err)
	require.Len(t, tcRows, 1)
	assert.Equal(t, uint64(1), tcRows[0].Count)
}

func TestWriteGitLinks(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	orgID := ""

	sess := Session{
		ID: "gl-session", UserID: "u", UserName: "U",
		StartedAt: testTime(2024, 1, 1, 0), MessageCount: 2, UserMessageCount: 1,
	}
	require.NoError(t, s.WriteSession(ctx, orgID, sess, nil, nil))

	links := []GitLink{
		{SessionID: "gl-session", UserID: "u", MessageOrdinal: 1, CommitSHA: "deadbeef", LinkType: "commit", Confidence: "high"},
		{SessionID: "gl-session", UserID: "u", MessageOrdinal: 2, PRURL: "https://github.com/org/repo/pull/1", LinkType: "pr", Confidence: "medium"},
	}
	require.NoError(t, s.WriteGitLinks(ctx, orgID, links))

	type countRow struct{ Count uint64 }
	var rows []countRow
	err := s.conn.Select(ctx, &rows, "SELECT count() AS Count FROM git_links FINAL WHERE session_id = 'gl-session'")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, uint64(2), rows[0].Count)
}

func TestListSessions(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	page, err := s.ListSessions(ctx, orgID, SessionFilter{})
	require.NoError(t, err)

	// 3 browsable sessions (session-1, session-2, session-3); ghost and subagent excluded
	assert.Equal(t, int64(3), page.Total)
	assert.Len(t, page.Sessions, 3)
	assert.Empty(t, page.NextCursor)

	// Ordered by started_at DESC
	assert.Equal(t, "session-1", page.Sessions[0].ID) // 2024-01-05
	assert.Equal(t, "session-2", page.Sessions[1].ID) // 2024-01-04
	assert.Equal(t, "session-3", page.Sessions[2].ID) // 2024-01-03
}

func TestListSessionsFilterUserID(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	page, err := s.ListSessions(ctx, orgID, SessionFilter{UserID: "alice@test.com"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), page.Total)
	for _, sess := range page.Sessions {
		assert.Equal(t, "alice@test.com", sess.UserID)
	}
}

func TestListSessionsFilterProjectID(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	page, err := s.ListSessions(ctx, orgID, SessionFilter{ProjectID: "proj-frontend"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), page.Total)
	for _, sess := range page.Sessions {
		assert.Equal(t, "proj-frontend", sess.ProjectID)
	}
}

func TestListSessionsFilterAgentType(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	page, err := s.ListSessions(ctx, orgID, SessionFilter{AgentType: "cursor"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), page.Total)
	assert.Equal(t, "cursor", page.Sessions[0].AgentType)
}

func TestListSessionsFilterDateRange(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	page, err := s.ListSessions(ctx, orgID, SessionFilter{DateFrom: "2024-01-04", DateTo: "2024-01-04"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), page.Total)
	assert.Equal(t, "session-2", page.Sessions[0].ID)
}

func TestListSessionsCombinedFilters(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	page, err := s.ListSessions(ctx, orgID, SessionFilter{
		UserID:    "alice@test.com",
		ProjectID: "proj-frontend",
		AgentType: "claude-code",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), page.Total)
	assert.Equal(t, "session-1", page.Sessions[0].ID)
}

func TestGetSession(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	sess, err := s.GetSession(ctx, orgID, "session-1")
	require.NoError(t, err)
	assert.Equal(t, "session-1", sess.ID)
	assert.Equal(t, "alice@test.com", sess.UserID)
	assert.Equal(t, "Alice", sess.UserName)
	assert.Equal(t, "frontend", sess.ProjectName)
	assert.Equal(t, "claude-code", sess.AgentType)
	assert.Equal(t, "Help me with React hooks", sess.FirstMessage)
}

func TestGetSessionNotFound(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	orgID := ""

	_, err := s.GetSession(ctx, orgID, "nonexistent-session")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetSessionCommitCount(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	// session-2 has 2 git links seeded
	sess, err := s.GetSession(ctx, orgID, "session-2")
	require.NoError(t, err)
	assert.Equal(t, 2, sess.CommitCount)

	// session-1 has no git links
	sess1, err := s.GetSession(ctx, orgID, "session-1")
	require.NoError(t, err)
	assert.Equal(t, 0, sess1.CommitCount)
}

func TestGetSessionMessages(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	msgs, err := s.GetSessionMessages(ctx, orgID, "session-1")
	require.NoError(t, err)
	assert.Len(t, msgs, 4)

	// Verify ordering
	for i, m := range msgs {
		assert.Equal(t, i+1, m.Ordinal, "ordinal should be in order")
	}
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "assistant", msgs[1].Role)
	assert.True(t, msgs[1].HasToolUse)
}

func TestGetSessionMessagesEmpty(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	orgID := ""

	msgs, err := s.GetSessionMessages(ctx, orgID, "nonexistent")
	require.NoError(t, err)
	assert.NotNil(t, msgs, "should return empty slice, not nil")
	assert.Len(t, msgs, 0)
}

func TestGetSessionToolCalls(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	tcs, err := s.GetSessionToolCalls(ctx, orgID, "session-2")
	require.NoError(t, err)
	assert.Len(t, tcs, 2)

	// Ordered by message_ordinal ASC, tool_use_id ASC
	assert.Equal(t, 2, tcs[0].MessageOrdinal)
	assert.Equal(t, "tool-2a", tcs[0].ToolUseID)
	assert.Equal(t, 6, tcs[1].MessageOrdinal)
	assert.Equal(t, "tool-2b", tcs[1].ToolUseID)
}

func TestGetSessionToolCallsEmpty(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	orgID := ""

	tcs, err := s.GetSessionToolCalls(ctx, orgID, "nonexistent")
	require.NoError(t, err)
	assert.NotNil(t, tcs, "should return empty slice, not nil")
	assert.Len(t, tcs, 0)
}

func TestListUsers(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	users, err := s.ListUsers(ctx, orgID)
	require.NoError(t, err)
	assert.Len(t, users, 2)

	// Ordered by user_name
	assert.Equal(t, "Alice", users[0].Name)
	assert.Equal(t, "alice@test.com", users[0].ID)
	assert.Equal(t, "Bob", users[1].Name)
	assert.Equal(t, "bob@test.com", users[1].ID)
}

func TestListUsersExcludesGhostAndSubagent(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	// Only alice and bob from browsable sessions; ghost/subagent shouldn't add duplicates
	users, err := s.ListUsers(ctx, orgID)
	require.NoError(t, err)
	// Should be exactly 2 distinct users, not 3 (ghost alice would be excluded)
	assert.Len(t, users, 2)
}

func TestListProjects(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	projects, err := s.ListProjects(ctx, orgID)
	require.NoError(t, err)
	assert.Len(t, projects, 2)

	// Ordered by project_name
	assert.Equal(t, "backend", projects[0].Name)
	assert.Equal(t, "frontend", projects[1].Name)
}

func TestListProjectsEmpty(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	orgID := ""

	projects, err := s.ListProjects(ctx, orgID)
	require.NoError(t, err)
	assert.NotNil(t, projects, "should return empty slice, not nil")
	assert.Len(t, projects, 0)
}

func TestListAgents(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	agents, err := s.ListAgents(ctx, orgID)
	require.NoError(t, err)
	assert.Len(t, agents, 2)

	// Ordered alphabetically
	assert.Equal(t, "claude-code", agents[0])
	assert.Equal(t, "cursor", agents[1])
}

func TestListAgentsEmpty(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	orgID := ""

	agents, err := s.ListAgents(ctx, orgID)
	require.NoError(t, err)
	assert.NotNil(t, agents, "should return empty slice, not nil")
	assert.Len(t, agents, 0)
}

func TestLookupGitLinksBySHA(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	// Short prefix match
	results, err := s.LookupGitLinks(ctx, orgID, "abc1234", "")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "session-2", results[0].SessionID)
	assert.Equal(t, "abc1234def5678", results[0].CommitSHA)
	assert.Equal(t, "commit", results[0].LinkType)
	assert.Equal(t, "high", results[0].Confidence)
	assert.Equal(t, "alice@test.com", results[0].UserID)
}

func TestLookupGitLinksByFullSHA(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	results, err := s.LookupGitLinks(ctx, orgID, "abc1234def5678", "")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "abc1234def5678", results[0].CommitSHA)
}

func TestLookupGitLinksByPRURL(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	results, err := s.LookupGitLinks(ctx, orgID, "", "https://github.com/org/repo/pull/42")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "session-2", results[0].SessionID)
	assert.Equal(t, "pr", results[0].LinkType)
}

func TestLookupGitLinksNotFound(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	results, err := s.LookupGitLinks(ctx, orgID, "nonexistentsha", "")
	require.NoError(t, err)
	assert.NotNil(t, results, "should return empty slice, not nil")
	assert.Len(t, results, 0)
}

func TestLookupGitLinksNoParams(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	orgID := ""

	_, err := s.LookupGitLinks(ctx, orgID, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestSearch(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	page, err := s.Search(ctx, orgID, SearchQuery{Query: "React hooks"})
	require.NoError(t, err)
	assert.Greater(t, page.Total, 0)
	assert.NotEmpty(t, page.Results)

	for _, r := range page.Results {
		assert.NotEmpty(t, r.Snippet)
		assert.NotNil(t, r.Highlights)
	}
}

func TestSearchNotFound(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	page, err := s.Search(ctx, orgID, SearchQuery{Query: "xyzzy_nonexistent_term_abc123"})
	require.NoError(t, err)
	assert.Equal(t, 0, page.Total)
	assert.NotNil(t, page.Results, "results should be empty slice, not nil")
	assert.Len(t, page.Results, 0)
}

func TestSearchMatchesToolCallContent(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	// "max_connections" is in a tool call result_content
	page, err := s.Search(ctx, orgID, SearchQuery{Query: "max_connections"})
	require.NoError(t, err)
	assert.Greater(t, page.Total, 0)
}

func TestSearchFilterByUser(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	page, err := s.Search(ctx, orgID, SearchQuery{Query: "connection pool", UserID: "alice@test.com"})
	require.NoError(t, err)
	for _, r := range page.Results {
		assert.Equal(t, "alice@test.com", r.UserID)
	}
}

func TestEmptySlices(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	orgID := ""

	// All read methods must return empty slice (not nil) when no data exists

	msgs, err := s.GetSessionMessages(ctx, orgID, "no-such-session")
	require.NoError(t, err)
	assert.NotNil(t, msgs)

	tcs, err := s.GetSessionToolCalls(ctx, orgID, "no-such-session")
	require.NoError(t, err)
	assert.NotNil(t, tcs)

	users, err := s.ListUsers(ctx, orgID)
	require.NoError(t, err)
	assert.NotNil(t, users)

	projects, err := s.ListProjects(ctx, orgID)
	require.NoError(t, err)
	assert.NotNil(t, projects)

	agents, err := s.ListAgents(ctx, orgID)
	require.NoError(t, err)
	assert.NotNil(t, agents)

	links, err := s.LookupGitLinks(ctx, orgID, "abc123", "")
	require.NoError(t, err)
	assert.NotNil(t, links)

	page, err := s.Search(ctx, orgID, SearchQuery{Query: "nothing here"})
	require.NoError(t, err)
	assert.NotNil(t, page.Results)

	sessPage, err := s.ListSessions(ctx, orgID, SessionFilter{})
	require.NoError(t, err)
	assert.NotNil(t, sessPage.Sessions)
}

func TestCursorPagination(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	// Fetch with limit=1, walk all pages
	seen := map[string]bool{}
	var cursor string

	for i := 0; i < 5; i++ {
		page, err := s.ListSessions(ctx, orgID, SessionFilter{Limit: 1, Cursor: cursor})
		require.NoError(t, err)
		assert.Equal(t, int64(3), page.Total, "total should be stable across pages")

		for _, sess := range page.Sessions {
			assert.False(t, seen[sess.ID], "no duplicate session: %s", sess.ID)
			seen[sess.ID] = true
		}

		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}

	assert.Len(t, seen, 3, "should have walked all 3 browsable sessions")
}

func TestListSessionsCommitCount(t *testing.T) {
	s := setupTestStore(t)
	seedTestData(t, s)
	ctx := context.Background()
	orgID := ""

	page, err := s.ListSessions(ctx, orgID, SessionFilter{})
	require.NoError(t, err)

	byID := map[string]Session{}
	for _, sess := range page.Sessions {
		byID[sess.ID] = sess
	}

	assert.Equal(t, 2, byID["session-2"].CommitCount, "session-2 has 2 git links")
	assert.Equal(t, 0, byID["session-1"].CommitCount, "session-1 has no git links")
}

func TestWriteBatch(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	orgID := ""

	sessions := []Session{
		{ID: "batch-1", UserID: "u1", UserName: "User1", ProjectID: "p1", ProjectName: "proj1",
			StartedAt: testTime(2026, 1, 1, 10), MessageCount: 2, UserMessageCount: 1, SourceCreatedAt: "2026-01-01T10:00:00Z"},
		{ID: "batch-2", UserID: "u2", UserName: "User2", ProjectID: "p2", ProjectName: "proj2",
			StartedAt: testTime(2026, 1, 2, 10), MessageCount: 1, UserMessageCount: 1, SourceCreatedAt: "2026-01-02T10:00:00Z"},
	}
	messages := []Message{
		{OrgID: "", SessionID: "batch-1", Ordinal: 0, Role: "user", Content: "hello", ContentLength: 5},
		{OrgID: "", SessionID: "batch-1", Ordinal: 1, Role: "assistant", Content: "hi", ContentLength: 2},
		{OrgID: "", SessionID: "batch-2", Ordinal: 0, Role: "user", Content: "world", ContentLength: 5},
	}
	toolCalls := []ToolCall{
		{OrgID: "", SessionID: "batch-1", MessageOrdinal: 1, ToolName: "Read", Category: "file", ToolUseID: "tc-1"},
	}

	require.NoError(t, s.WriteBatch(ctx, orgID, sessions, messages, toolCalls))

	// Verify both sessions readable
	s1, err := s.GetSession(ctx, orgID, "batch-1")
	require.NoError(t, err)
	assert.Equal(t, "batch-1", s1.ID)
	assert.Equal(t, "User1", s1.UserName)

	s2, err := s.GetSession(ctx, orgID, "batch-2")
	require.NoError(t, err)
	assert.Equal(t, "batch-2", s2.ID)

	// Verify messages for each session
	msgs1, err := s.GetSessionMessages(ctx, orgID, "batch-1")
	require.NoError(t, err)
	assert.Len(t, msgs1, 2)

	msgs2, err := s.GetSessionMessages(ctx, orgID, "batch-2")
	require.NoError(t, err)
	assert.Len(t, msgs2, 1)

	// Verify tool calls
	tcs, err := s.GetSessionToolCalls(ctx, orgID, "batch-1")
	require.NoError(t, err)
	assert.Len(t, tcs, 1)
}
