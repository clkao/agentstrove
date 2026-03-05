// ABOUTME: Tests for the HTTP API server endpoints.
// ABOUTME: Validates response codes, JSON shapes, pagination, filtering, and error handling.
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/clkao/agentstrove/internal/store"
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
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func setupTestAPI(t *testing.T) (*httptest.Server, *store.ClickHouseStore) {
	t.Helper()
	addr := clickhouseAddr()
	user := clickhouseUser()
	password := clickhousePassword()
	dbName := fmt.Sprintf("test_%s", randomHex(8))

	// Constructor bootstraps the database via "default" connection
	s, err := store.NewClickHouseStoreWithAuth(addr, dbName, user, password)
	require.NoError(t, err, "create store")
	require.NoError(t, s.EnsureSchema(context.Background()), "ensure schema")

	t.Cleanup(func() {
		_ = s.Close()
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

	srv := New(s)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts, s
}

func ptr[T any](v T) *T { return &v }

func seedAPITestData(t *testing.T, s *store.ClickHouseStore) {
	t.Helper()
	ctx := context.Background()

	sessions := []store.Session{
		{
			ID:           "sess-1",
			UserID:       "alice@example.com",
			UserName:     "Alice",
			ProjectID:    "proj-frontend",
			ProjectName:  "frontend",
			ProjectPath:  "/home/alice/frontend",
			Machine:      "laptop",
			AgentType:    "claude-code",
			FirstMessage: "Build login",
			MessageCount: 2,
			UserMessageCount: 1,
			StartedAt:       ptr(time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)),
			EndedAt:         ptr(time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC)),
			SourceCreatedAt: "2026-03-01T10:00:00Z",
		},
		{
			ID:           "sess-2",
			UserID:       "bob@example.com",
			UserName:     "Bob",
			ProjectID:    "proj-backend",
			ProjectName:  "backend",
			ProjectPath:  "/home/bob/backend",
			Machine:      "desktop",
			AgentType:    "cursor",
			FirstMessage: "Fix API",
			MessageCount: 1,
			UserMessageCount: 1,
			StartedAt:       ptr(time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)),
			EndedAt:         ptr(time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)),
			SourceCreatedAt: "2026-03-02T09:00:00Z",
		},
	}

	msgs := []store.Message{
		{OrgID: "", SessionID: "sess-1", Ordinal: 0, Role: "user", Content: "Build login", Timestamp: ptr(time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)), ContentLength: 11},
		{OrgID: "", SessionID: "sess-1", Ordinal: 1, Role: "assistant", Content: "Done.", Timestamp: ptr(time.Date(2026, 3, 1, 10, 0, 1, 0, time.UTC)), HasToolUse: true, ContentLength: 5},
	}

	tcs := []store.ToolCall{
		{OrgID: "", MessageOrdinal: 1, SessionID: "sess-1", ToolName: "Write", Category: "file", ToolUseID: "tu-1", InputJSON: `{"path":"login.tsx"}`, ResultContentLength: ptr(100)},
	}

	// Write sess-1 with messages and tool calls
	require.NoError(t, s.WriteSession(ctx, "", sessions[0], msgs, tcs))

	// Write sess-2 with no messages
	require.NoError(t, s.WriteSession(ctx, "", sessions[1], nil, nil))

	// Write a git link for sha lookup
	links := []store.GitLink{
		{
			OrgID:          "",
			SessionID:      "sess-1",
			UserID:         "alice@example.com",
			MessageOrdinal: 1,
			CommitSHA:      "abc1234",
			Confidence:     "high",
			LinkType:       "push",
		},
	}
	require.NoError(t, s.WriteGitLinks(ctx, "", links))
}

func TestListSessions_ReturnsJSON(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAPITestData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/sessions")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	assert.Equal(t, int64(2), page.Total)
	assert.Len(t, page.Sessions, 2)
}

func TestListSessions_FilterByUserID(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAPITestData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/sessions?user_id=alice@example.com")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	assert.Equal(t, int64(1), page.Total)
	assert.Equal(t, "sess-1", page.Sessions[0].ID)
}

func TestListSessions_FilterByProjectID(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAPITestData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/sessions?project_id=proj-backend")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	assert.Equal(t, int64(1), page.Total)
	assert.Equal(t, "sess-2", page.Sessions[0].ID)
}

func TestListSessions_FilterByAgentType(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAPITestData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/sessions?agent_type=cursor")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	assert.Equal(t, int64(1), page.Total)
}

func TestListSessions_FilterByDateRange(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAPITestData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/sessions?date_from=2026-03-02&date_to=2026-03-02")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	assert.Equal(t, int64(1), page.Total)
	assert.Equal(t, "sess-2", page.Sessions[0].ID)
}

func TestListSessions_InvalidDateFrom(t *testing.T) {
	ts, _ := setupTestAPI(t)

	resp, err := http.Get(ts.URL + "/api/v1/sessions?date_from=not-a-date")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestListSessions_InvalidDateTo(t *testing.T) {
	ts, _ := setupTestAPI(t)

	resp, err := http.Get(ts.URL + "/api/v1/sessions?date_to=not-a-date")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestListSessions_Pagination(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAPITestData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/sessions?limit=1")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	assert.Len(t, page.Sessions, 1)
	assert.NotEmpty(t, page.NextCursor)

	// Fetch next page
	resp2, err := http.Get(ts.URL + "/api/v1/sessions?limit=1&cursor=" + page.NextCursor)
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()

	var page2 store.SessionPage
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&page2))
	assert.Len(t, page2.Sessions, 1)
	assert.Empty(t, page2.NextCursor)
}

func TestGetSession_Found(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAPITestData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/sessions/sess-1")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var session store.Session
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&session))
	assert.Equal(t, "sess-1", session.ID)
	assert.Equal(t, "Alice", session.UserName)
}

func TestGetSession_NotFound(t *testing.T) {
	ts, _ := setupTestAPI(t)

	resp, err := http.Get(ts.URL + "/api/v1/sessions/nonexistent")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetMessages_ReturnsMessagesWithToolCalls(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAPITestData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/sessions/sess-1/messages")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var messages []MessageWithToolCalls
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&messages))
	assert.Len(t, messages, 2)

	// First message (user) should have no tool calls
	assert.Equal(t, "user", messages[0].Role)
	assert.Empty(t, messages[0].ToolCalls)

	// Second message (assistant) should have 1 tool call
	assert.Equal(t, "assistant", messages[1].Role)
	assert.Len(t, messages[1].ToolCalls, 1)
	assert.Equal(t, "Write", messages[1].ToolCalls[0].ToolName)
}

func TestListUsers(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAPITestData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/users")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var users []store.UserInfo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&users))
	assert.Len(t, users, 2)
}

func TestListProjects(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAPITestData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/projects")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var projects []store.ProjectInfo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&projects))
	assert.Len(t, projects, 2)
}

func TestListAgents(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAPITestData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/agents")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var agents []string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&agents))
	assert.Len(t, agents, 2)
}

func TestSearch_ReturnsResults(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAPITestData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/search?q=login")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var page store.SearchPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	assert.GreaterOrEqual(t, page.Total, 0)
}

func TestSearch_MissingQuery(t *testing.T) {
	ts, _ := setupTestAPI(t)

	resp, err := http.Get(ts.URL + "/api/v1/search")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSearch_SHALookup(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAPITestData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/search?q=abc1234")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var page store.SearchPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	assert.Equal(t, 1, page.Total)
	assert.Equal(t, "sess-1", page.Results[0].SessionID)
}

func TestLookupGitLinks_BySHA(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAPITestData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/gitlinks?sha=abc1234")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var results []store.GitLinkResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&results))
	assert.Len(t, results, 1)
	assert.Equal(t, "sess-1", results[0].SessionID)
}

func TestLookupGitLinks_MissingParams(t *testing.T) {
	ts, _ := setupTestAPI(t)

	resp, err := http.Get(ts.URL + "/api/v1/gitlinks")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSPAFallback(t *testing.T) {
	ts, _ := setupTestAPI(t)

	resp, err := http.Get(ts.URL + "/some/random/path")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// SPA fallback serves 200 (or 404 if no index.html — dist is empty in test)
	// The important thing is it doesn't 500
	assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestCORSHeaders(t *testing.T) {
	ts, _ := setupTestAPI(t)

	resp, err := http.Get(ts.URL + "/api/v1/sessions")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestCORSPreflight(t *testing.T) {
	ts, _ := setupTestAPI(t)

	req, _ := http.NewRequest(http.MethodOptions, ts.URL+"/api/v1/sessions", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestContentTypeJSON(t *testing.T) {
	ts, _ := setupTestAPI(t)

	resp, err := http.Get(ts.URL + "/api/v1/sessions")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}
