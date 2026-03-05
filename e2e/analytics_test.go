// ABOUTME: E2E tests for team analytics endpoints (usage, heatmap, tool distribution).
// ABOUTME: Validates analytics queries against seeded ClickHouse data with date filtering and cross-endpoint consistency.
package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/clkao/agentstrove/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Usage Overview tests ---

func TestAnalyticsUsage_ReturnsJSON(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.server.URL + "/api/v1/analytics/usage")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")

	var results []store.UserUsage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&results))
	assert.Greater(t, len(results), 0)
}

func TestAnalyticsUsage_GroupsByUserAgentProject(t *testing.T) {
	env := setupTestEnv(t)
	status, body := httpGet(t, env, "/api/v1/analytics/usage")
	assert.Equal(t, http.StatusOK, status)

	var results []store.UserUsage
	require.NoError(t, json.Unmarshal(body, &results))
	assert.Len(t, results, 4, "should have 4 distinct user/agent/project groups")

	// Build a lookup for easy assertions
	type key struct{ userID, agent, project string }
	lookup := make(map[key]store.UserUsage)
	for _, r := range results {
		lookup[key{r.UserID, r.AgentType, r.ProjectName}] = r
	}

	alice := lookup[key{"alice@dev.io", "claude-code", "frontend"}]
	assert.Equal(t, 2, alice.SessionCount, "Alice/claude-code/frontend: 2 sessions")
	assert.Equal(t, 5, alice.MessageCount, "Alice/claude-code/frontend: 5 messages")

	bobCursor := lookup[key{"bob@dev.io", "cursor", "backend"}]
	assert.Equal(t, 1, bobCursor.SessionCount, "Bob/cursor/backend: 1 session")
	assert.Equal(t, 2, bobCursor.MessageCount, "Bob/cursor/backend: 2 messages")

	bobClaude := lookup[key{"bob@dev.io", "claude-code", "backend"}]
	assert.Equal(t, 1, bobClaude.SessionCount, "Bob/claude-code/backend: 1 session")
	assert.Equal(t, 2, bobClaude.MessageCount, "Bob/claude-code/backend: 2 messages")

	carol := lookup[key{"carol@dev.io", "copilot", "infra"}]
	assert.Equal(t, 1, carol.SessionCount, "Carol/copilot/infra: 1 session")
	assert.Equal(t, 1, carol.MessageCount, "Carol/copilot/infra: 1 message")
}

func TestAnalyticsUsage_ExcludesGhostAndSubagent(t *testing.T) {
	env := setupTestEnv(t)
	_, body := httpGet(t, env, "/api/v1/analytics/usage")

	var results []store.UserUsage
	require.NoError(t, json.Unmarshal(body, &results))

	totalSessions := 0
	for _, r := range results {
		totalSessions += r.SessionCount
	}
	assert.Equal(t, 5, totalSessions, "total sessions should be 5 (excluding ghost and subagent)")
}

func TestAnalyticsUsage_DateFilterSingleDay(t *testing.T) {
	env := setupTestEnv(t)
	status, body := httpGet(t, env, "/api/v1/analytics/usage?date_from=2026-02-25&date_to=2026-02-25")
	assert.Equal(t, http.StatusOK, status)

	var results []store.UserUsage
	require.NoError(t, json.Unmarshal(body, &results))
	assert.Len(t, results, 1, "single day 2026-02-25 should match only sess-alpha")
	assert.Equal(t, "alice@dev.io", results[0].UserID)
	assert.Equal(t, "claude-code", results[0].AgentType)
	assert.Equal(t, "frontend", results[0].ProjectName)
}

func TestAnalyticsUsage_DateFilterRange(t *testing.T) {
	env := setupTestEnv(t)
	status, body := httpGet(t, env, "/api/v1/analytics/usage?date_from=2026-03-01&date_to=2026-03-03")
	assert.Equal(t, http.StatusOK, status)

	var results []store.UserUsage
	require.NoError(t, json.Unmarshal(body, &results))
	assert.Len(t, results, 3, "range 2026-03-01 to 2026-03-03 should match gamma, delta, epsilon")
}

func TestAnalyticsUsage_DateToInclusive(t *testing.T) {
	env := setupTestEnv(t)
	status, body := httpGet(t, env, "/api/v1/analytics/usage?date_to=2026-03-03")
	assert.Equal(t, http.StatusOK, status)

	var results []store.UserUsage
	require.NoError(t, json.Unmarshal(body, &results))

	// sess-epsilon started at 2026-03-03 08:00 — should be included
	found := false
	for _, r := range results {
		if r.UserID == "bob@dev.io" && r.AgentType == "claude-code" {
			found = true
			break
		}
	}
	assert.True(t, found, "date_to=2026-03-03 should include sess-epsilon (Bob/claude-code)")
}

func TestAnalyticsUsage_InvalidDate(t *testing.T) {
	env := setupTestEnv(t)
	status, _ := httpGet(t, env, "/api/v1/analytics/usage?date_from=bad")
	assert.Equal(t, http.StatusBadRequest, status)
}

func TestAnalyticsUsage_EmptyResult(t *testing.T) {
	env := setupTestEnv(t)
	status, body := httpGet(t, env, "/api/v1/analytics/usage?date_from=2020-01-01&date_to=2020-01-01")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "[]", strings.TrimSpace(string(body)), "empty result should be JSON [] not null")
}

func TestAnalyticsUsage_OrderedBySessionCount(t *testing.T) {
	env := setupTestEnv(t)
	_, body := httpGet(t, env, "/api/v1/analytics/usage")

	var results []store.UserUsage
	require.NoError(t, json.Unmarshal(body, &results))
	require.NotEmpty(t, results)

	for i := 1; i < len(results); i++ {
		assert.GreaterOrEqual(t, results[i-1].SessionCount, results[i].SessionCount,
			"results should be ordered by session_count DESC")
	}
}

func TestAnalyticsUsage_AllFieldsPresent(t *testing.T) {
	env := setupTestEnv(t)
	_, body := httpGet(t, env, "/api/v1/analytics/usage")

	var results []store.UserUsage
	require.NoError(t, json.Unmarshal(body, &results))

	for _, r := range results {
		assert.NotEmpty(t, r.UserID, "user_id must be non-empty")
		assert.NotEmpty(t, r.UserName, "user_name must be non-empty")
		assert.NotEmpty(t, r.ProjectName, "project_name must be non-empty")
		assert.NotEmpty(t, r.AgentType, "agent_type must be non-empty")
		assert.Greater(t, r.SessionCount, 0, "session_count must be > 0")
		assert.Greater(t, r.MessageCount, 0, "message_count must be > 0")
	}
}

func TestAnalyticsUsage_CORSHeaders(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.server.URL + "/api/v1/analytics/usage")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"),
		"API should have CORS Allow-Origin header")
}

// --- Heatmap tests ---

func TestAnalyticsHeatmap_ReturnsJSON(t *testing.T) {
	env := setupTestEnv(t)
	status, body := httpGet(t, env, "/api/v1/analytics/heatmap")
	assert.Equal(t, http.StatusOK, status)

	var cells []store.HeatmapCell
	require.NoError(t, json.Unmarshal(body, &cells))
	assert.Greater(t, len(cells), 0)
}

func TestAnalyticsHeatmap_ValidDayAndHour(t *testing.T) {
	env := setupTestEnv(t)
	_, body := httpGet(t, env, "/api/v1/analytics/heatmap")

	var cells []store.HeatmapCell
	require.NoError(t, json.Unmarshal(body, &cells))

	for _, c := range cells {
		assert.GreaterOrEqual(t, c.DayOfWeek, 1, "day_of_week must be >= 1")
		assert.LessOrEqual(t, c.DayOfWeek, 7, "day_of_week must be <= 7")
		assert.GreaterOrEqual(t, c.Hour, 0, "hour must be >= 0")
		assert.LessOrEqual(t, c.Hour, 23, "hour must be <= 23")
	}
}

func TestAnalyticsHeatmap_KnownCellExists(t *testing.T) {
	env := setupTestEnv(t)
	_, body := httpGet(t, env, "/api/v1/analytics/heatmap")

	var cells []store.HeatmapCell
	require.NoError(t, json.Unmarshal(body, &cells))

	// sess-alpha: Tuesday 10:00 UTC → toDayOfWeek: Tue=2, hour=10
	found := false
	for _, c := range cells {
		if c.DayOfWeek == 2 && c.Hour == 10 {
			assert.GreaterOrEqual(t, c.SessionCount, 1, "Tue 10:00 cell should have >= 1 session")
			found = true
			break
		}
	}
	assert.True(t, found, "expected heatmap cell for dow=2 hour=10 (sess-alpha on Tuesday 10am)")
}

func TestAnalyticsHeatmap_DateFilter(t *testing.T) {
	env := setupTestEnv(t)
	status, body := httpGet(t, env, "/api/v1/analytics/heatmap?date_from=2026-02-25&date_to=2026-02-25")
	assert.Equal(t, http.StatusOK, status)

	var cells []store.HeatmapCell
	require.NoError(t, json.Unmarshal(body, &cells))
	assert.Len(t, cells, 1, "single day 2026-02-25 should produce exactly 1 heatmap cell")
}

func TestAnalyticsHeatmap_EmptyRange(t *testing.T) {
	env := setupTestEnv(t)
	status, body := httpGet(t, env, "/api/v1/analytics/heatmap?date_from=2020-01-01&date_to=2020-01-01")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "[]", strings.TrimSpace(string(body)), "empty result should be JSON [] not null")
}

func TestAnalyticsHeatmap_InvalidDate(t *testing.T) {
	env := setupTestEnv(t)
	status, _ := httpGet(t, env, "/api/v1/analytics/heatmap?date_from=xyz")
	assert.Equal(t, http.StatusBadRequest, status)
}

func TestAnalyticsHeatmap_ExcludesGhostAndSubagent(t *testing.T) {
	env := setupTestEnv(t)
	_, body := httpGet(t, env, "/api/v1/analytics/heatmap")

	var cells []store.HeatmapCell
	require.NoError(t, json.Unmarshal(body, &cells))

	totalSessions := 0
	for _, c := range cells {
		totalSessions += c.SessionCount
	}
	assert.Equal(t, 5, totalSessions, "heatmap total should be 5 browsable sessions")
}

func TestAnalyticsHeatmap_SameHourAggregation(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	// Write 2 extra sessions at the same dow+hour as sess-alpha (Tue 10:00 UTC)
	// 2026-02-25 is a Tuesday; pick two more Tuesdays at 10:00
	require.NoError(t, env.store.WriteSession(ctx, "", store.Session{
		ID: "sess-agg1", UserID: "dave@dev.io", UserName: "Dave",
		ProjectID: "proj-test", ProjectName: "test", ProjectPath: "/tmp/test",
		Machine: "vm", AgentType: "claude-code",
		StartedAt:       ptr(time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)), // Tue
		MessageCount:    1,
		UserMessageCount: 1,
		SourceCreatedAt: "2026-03-10T10:00:00Z",
	}, []store.Message{
		{OrgID: "", SessionID: "sess-agg1", Ordinal: 0, Role: "user", Content: "hi", Timestamp: ptr(time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)), ContentLength: 2},
	}, nil))

	require.NoError(t, env.store.WriteSession(ctx, "", store.Session{
		ID: "sess-agg2", UserID: "eve@dev.io", UserName: "Eve",
		ProjectID: "proj-test", ProjectName: "test", ProjectPath: "/tmp/test",
		Machine: "vm", AgentType: "claude-code",
		StartedAt:       ptr(time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC)), // Tue
		MessageCount:    1,
		UserMessageCount: 1,
		SourceCreatedAt: "2026-03-17T10:00:00Z",
	}, []store.Message{
		{OrgID: "", SessionID: "sess-agg2", Ordinal: 0, Role: "user", Content: "hi", Timestamp: ptr(time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC)), ContentLength: 2},
	}, nil))

	_, body := httpGet(t, env, "/api/v1/analytics/heatmap")

	var cells []store.HeatmapCell
	require.NoError(t, json.Unmarshal(body, &cells))

	for _, c := range cells {
		if c.DayOfWeek == 2 && c.Hour == 10 {
			assert.GreaterOrEqual(t, c.SessionCount, 3,
				"Tue 10:00 should aggregate sess-alpha + 2 extra sessions")
			return
		}
	}
	t.Fatal("expected heatmap cell for dow=2 hour=10")
}

// --- Tool Usage tests ---

func TestAnalyticsTools_ReturnsJSON(t *testing.T) {
	env := setupTestEnv(t)
	status, body := httpGet(t, env, "/api/v1/analytics/tools")
	assert.Equal(t, http.StatusOK, status)

	var stats []store.ToolUsageStat
	require.NoError(t, json.Unmarshal(body, &stats))
	assert.Greater(t, len(stats), 0)
}

func TestAnalyticsTools_OrderedByCount(t *testing.T) {
	env := setupTestEnv(t)
	_, body := httpGet(t, env, "/api/v1/analytics/tools")

	var stats []store.ToolUsageStat
	require.NoError(t, json.Unmarshal(body, &stats))
	require.NotEmpty(t, stats)

	for i := 1; i < len(stats); i++ {
		assert.GreaterOrEqual(t, stats[i-1].UsageCount, stats[i].UsageCount,
			"tool stats should be ordered by usage_count DESC")
	}
}

func TestAnalyticsTools_CorrectCounts(t *testing.T) {
	env := setupTestEnv(t)
	_, body := httpGet(t, env, "/api/v1/analytics/tools")

	var stats []store.ToolUsageStat
	require.NoError(t, json.Unmarshal(body, &stats))

	toolCounts := make(map[string]int)
	totalCount := 0
	for _, s := range stats {
		toolCounts[s.ToolName+"/"+s.Category] = s.UsageCount
		totalCount += s.UsageCount
	}

	assert.Equal(t, 1, toolCounts["Bash/command"], "Bash/command: 1 use (sess-alpha)")
	assert.Equal(t, 1, toolCounts["Write/file"], "Write/file: 1 use (sess-alpha)")
	assert.Equal(t, 1, toolCounts["Edit/file"], "Edit/file: 1 use (sess-beta)")
	assert.Equal(t, 1, toolCounts["Read/file"], "Read/file: 1 use (sess-epsilon)")
	assert.Equal(t, 4, totalCount, "total tool calls across browsable sessions should be 4")
}

func TestAnalyticsTools_DateFilter(t *testing.T) {
	env := setupTestEnv(t)
	status, body := httpGet(t, env, "/api/v1/analytics/tools?date_from=2026-02-25&date_to=2026-02-25")
	assert.Equal(t, http.StatusOK, status)

	var stats []store.ToolUsageStat
	require.NoError(t, json.Unmarshal(body, &stats))
	assert.Len(t, stats, 2, "2026-02-25 should have Write + Bash from sess-alpha")

	names := make(map[string]bool)
	for _, s := range stats {
		names[s.ToolName] = true
	}
	assert.True(t, names["Write"], "should include Write tool")
	assert.True(t, names["Bash"], "should include Bash tool")
}

func TestAnalyticsTools_EmptyRange(t *testing.T) {
	env := setupTestEnv(t)
	status, body := httpGet(t, env, "/api/v1/analytics/tools?date_from=2020-01-01&date_to=2020-01-01")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "[]", strings.TrimSpace(string(body)), "empty result should be JSON [] not null")
}

func TestAnalyticsTools_InvalidDate(t *testing.T) {
	env := setupTestEnv(t)
	status, _ := httpGet(t, env, "/api/v1/analytics/tools?date_from=bad")
	assert.Equal(t, http.StatusBadRequest, status)
}

func TestAnalyticsTools_ExcludesGhostSubagent(t *testing.T) {
	env := setupTestEnv(t)
	_, body := httpGet(t, env, "/api/v1/analytics/tools")

	var stats []store.ToolUsageStat
	require.NoError(t, json.Unmarshal(body, &stats))

	totalCount := 0
	for _, s := range stats {
		totalCount += s.UsageCount
	}
	assert.Equal(t, 4, totalCount, "total tool calls should be 4 (ghost and subagent excluded)")
}

// --- Cross-endpoint consistency tests ---

func TestAnalyticsCross_UsageTotalMatchesSessions(t *testing.T) {
	env := setupTestEnv(t)

	// Get total from sessions endpoint
	_, sessBody := httpGet(t, env, "/api/v1/sessions?limit=100")
	var page store.SessionPage
	require.NoError(t, json.Unmarshal(sessBody, &page))

	// Get total from usage endpoint
	_, usageBody := httpGet(t, env, "/api/v1/analytics/usage")
	var results []store.UserUsage
	require.NoError(t, json.Unmarshal(usageBody, &results))

	usageTotal := 0
	for _, r := range results {
		usageTotal += r.SessionCount
	}

	assert.Equal(t, int(page.Total), usageTotal,
		"sum of usage session_count should match sessions total")
}

func TestAnalyticsCross_HeatmapTotalMatchesSessions(t *testing.T) {
	env := setupTestEnv(t)

	// Get total from sessions endpoint
	_, sessBody := httpGet(t, env, "/api/v1/sessions?limit=100")
	var page store.SessionPage
	require.NoError(t, json.Unmarshal(sessBody, &page))

	// Get total from heatmap endpoint
	_, heatBody := httpGet(t, env, "/api/v1/analytics/heatmap")
	var cells []store.HeatmapCell
	require.NoError(t, json.Unmarshal(heatBody, &cells))

	heatTotal := 0
	for _, c := range cells {
		heatTotal += c.SessionCount
	}

	assert.Equal(t, int(page.Total), heatTotal,
		"sum of heatmap session_count should match sessions total")
}

func TestAnalyticsCross_EndpointsCoexist(t *testing.T) {
	env := setupTestEnv(t)

	statusSessions, _ := httpGet(t, env, "/api/v1/sessions")
	statusUsage, _ := httpGet(t, env, "/api/v1/analytics/usage")
	statusHeatmap, _ := httpGet(t, env, "/api/v1/analytics/heatmap")
	statusTools, _ := httpGet(t, env, "/api/v1/analytics/tools")

	assert.Equal(t, http.StatusOK, statusSessions)
	assert.Equal(t, http.StatusOK, statusUsage)
	assert.Equal(t, http.StatusOK, statusHeatmap)
	assert.Equal(t, http.StatusOK, statusTools)
}
