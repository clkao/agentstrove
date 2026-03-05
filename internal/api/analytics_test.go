// ABOUTME: Integration tests for analytics API endpoints.
// ABOUTME: Validates usage overview, activity heatmap, and tool distribution responses.
package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/clkao/agentstrove/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedAnalyticsAPIData(t *testing.T, s *store.ClickHouseStore) {
	t.Helper()
	ctx := t.Context()

	sessions := []store.Session{
		{
			ID:               "analytics-sess-1",
			UserID:           "alice@example.com",
			UserName:         "Alice",
			ProjectID:        "proj-frontend",
			ProjectName:      "frontend",
			ProjectPath:      "/home/alice/frontend",
			Machine:          "laptop",
			AgentType:        "claude-code",
			FirstMessage:     "Build login",
			MessageCount:     4,
			UserMessageCount: 2,
			StartedAt:        ptr(time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)),
			EndedAt:          ptr(time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC)),
			SourceCreatedAt:  "2026-03-01T10:00:00Z",
		},
		{
			ID:               "analytics-sess-2",
			UserID:           "bob@example.com",
			UserName:         "Bob",
			ProjectID:        "proj-backend",
			ProjectName:      "backend",
			ProjectPath:      "/home/bob/backend",
			Machine:          "desktop",
			AgentType:        "cursor",
			FirstMessage:     "Fix API",
			MessageCount:     2,
			UserMessageCount: 1,
			StartedAt:        ptr(time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC)),
			EndedAt:          ptr(time.Date(2026, 3, 5, 15, 0, 0, 0, time.UTC)),
			SourceCreatedAt:  "2026-03-05T14:00:00Z",
		},
		{
			// Ghost session — user_message_count=0, excluded from analytics
			ID:               "analytics-sess-ghost",
			UserID:           "alice@example.com",
			UserName:         "Alice",
			ProjectID:        "proj-frontend",
			ProjectName:      "frontend",
			ProjectPath:      "/home/alice/frontend",
			Machine:          "laptop",
			AgentType:        "claude-code",
			FirstMessage:     "",
			MessageCount:     1,
			UserMessageCount: 0,
			StartedAt:        ptr(time.Date(2026, 3, 2, 8, 0, 0, 0, time.UTC)),
			EndedAt:          ptr(time.Date(2026, 3, 2, 8, 1, 0, 0, time.UTC)),
			SourceCreatedAt:  "2026-03-02T08:00:00Z",
		},
		{
			// Subagent session — has parent, excluded from analytics
			ID:               "analytics-sess-sub",
			UserID:           "alice@example.com",
			UserName:         "Alice",
			ProjectID:        "proj-frontend",
			ProjectName:      "frontend",
			ProjectPath:      "/home/alice/frontend",
			Machine:          "laptop",
			AgentType:        "claude-code",
			FirstMessage:     "Sub task",
			MessageCount:     3,
			UserMessageCount: 1,
			ParentSessionID:  "analytics-sess-1",
			RelationshipType: "subagent",
			StartedAt:        ptr(time.Date(2026, 3, 1, 10, 30, 0, 0, time.UTC)),
			EndedAt:          ptr(time.Date(2026, 3, 1, 10, 45, 0, 0, time.UTC)),
			SourceCreatedAt:  "2026-03-01T10:30:00Z",
		},
	}

	msgs := []store.Message{
		{OrgID: "", SessionID: "analytics-sess-1", Ordinal: 0, Role: "user", Content: "Build login", Timestamp: ptr(time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)), ContentLength: 11},
		{OrgID: "", SessionID: "analytics-sess-1", Ordinal: 1, Role: "assistant", Content: "Done.", Timestamp: ptr(time.Date(2026, 3, 1, 10, 0, 1, 0, time.UTC)), HasToolUse: true, ContentLength: 5},
		{OrgID: "", SessionID: "analytics-sess-2", Ordinal: 0, Role: "user", Content: "Fix API", Timestamp: ptr(time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC)), ContentLength: 7},
		{OrgID: "", SessionID: "analytics-sess-2", Ordinal: 1, Role: "assistant", Content: "Fixed.", Timestamp: ptr(time.Date(2026, 3, 5, 14, 0, 1, 0, time.UTC)), HasToolUse: true, ContentLength: 6},
	}

	tcs := []store.ToolCall{
		{OrgID: "", MessageOrdinal: 1, SessionID: "analytics-sess-1", ToolName: "Write", Category: "file", ToolUseID: "tu-a1", InputJSON: `{"path":"login.tsx"}`, ResultContentLength: ptr(100)},
		{OrgID: "", MessageOrdinal: 1, SessionID: "analytics-sess-1", ToolName: "Read", Category: "file", ToolUseID: "tu-a2", InputJSON: `{"path":"auth.ts"}`, ResultContentLength: ptr(50)},
		{OrgID: "", MessageOrdinal: 1, SessionID: "analytics-sess-2", ToolName: "Bash", Category: "shell", ToolUseID: "tu-b1", InputJSON: `{"cmd":"go test"}`, ResultContentLength: ptr(200)},
	}

	// Write sess-1 with messages and tool calls
	require.NoError(t, s.WriteSession(ctx, "", sessions[0], msgs[:2], tcs[:2]))
	// Write sess-2 with messages and tool calls
	require.NoError(t, s.WriteSession(ctx, "", sessions[1], msgs[2:], tcs[2:]))
	// Write ghost session (no messages/tool calls)
	require.NoError(t, s.WriteSession(ctx, "", sessions[2], nil, nil))
	// Write subagent session (no messages/tool calls)
	require.NoError(t, s.WriteSession(ctx, "", sessions[3], nil, nil))
}

func TestAnalyticsUsage_ReturnsJSON(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAnalyticsAPIData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/analytics/usage")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result []store.UserUsage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.GreaterOrEqual(t, len(result), 2)
}

func TestAnalyticsUsage_DateFilter(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAnalyticsAPIData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/analytics/usage?date_from=2026-03-05&date_to=2026-03-05")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []store.UserUsage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.Len(t, result, 1)
	assert.Equal(t, "bob@example.com", result[0].UserID)
}

func TestAnalyticsUsage_InvalidDate(t *testing.T) {
	ts, _ := setupTestAPI(t)

	resp, err := http.Get(ts.URL + "/api/v1/analytics/usage?date_from=bad")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAnalyticsUsage_Empty(t *testing.T) {
	ts, _ := setupTestAPI(t)

	resp, err := http.Get(ts.URL + "/api/v1/analytics/usage")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []store.UserUsage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Empty(t, result)
}

func TestAnalyticsHeatmap_ReturnsJSON(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAnalyticsAPIData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/analytics/heatmap")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result []store.HeatmapCell
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.NotEmpty(t, result)
}

func TestAnalyticsHeatmap_Empty(t *testing.T) {
	ts, _ := setupTestAPI(t)

	resp, err := http.Get(ts.URL + "/api/v1/analytics/heatmap")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []store.HeatmapCell
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Empty(t, result)
}

func TestAnalyticsTools_ReturnsJSON(t *testing.T) {
	ts, s := setupTestAPI(t)
	seedAnalyticsAPIData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/analytics/tools")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result []store.ToolUsageStat
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.NotEmpty(t, result)
}

func TestAnalyticsTools_Empty(t *testing.T) {
	ts, _ := setupTestAPI(t)

	resp, err := http.Get(ts.URL + "/api/v1/analytics/tools")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []store.ToolUsageStat
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Empty(t, result)
}
