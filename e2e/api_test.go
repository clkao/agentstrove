// ABOUTME: API-level E2E tests for the team conversation browser.
// ABOUTME: Seeds a ClickHouse store with test data, starts an httptest server, and validates all API endpoints.
package e2e

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/clkao/agentstrove/internal/api"
	"github.com/clkao/agentstrove/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T33: Ghost and subagent sessions are excluded from listing
func TestT33_GhostAndSubagentSessionsExcluded(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.server.URL + "/api/v1/sessions?limit=100")
	require.NoError(t, err)
	defer resp.Body.Close()

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))

	for _, s := range page.Sessions {
		assert.NotEqual(t, "sess-ghost", s.ID, "ghost session (0 messages) should be filtered out")
		assert.NotEqual(t, "sess-subagent", s.ID, "subagent session should be filtered out")
		assert.Greater(t, s.MessageCount, 0, "session %s has 0 messages", s.ID)
		assert.NotEmpty(t, s.FirstMessage, "session %s has empty first_message", s.ID)
	}
}

// T34: Every listed session has reasonable data
func TestT34_SessionDataQuality(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.server.URL + "/api/v1/sessions?limit=100")
	require.NoError(t, err)
	defer resp.Body.Close()

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))

	for _, s := range page.Sessions {
		assert.NotEmpty(t, s.ID, "session must have an ID")
		assert.NotEmpty(t, s.UserName, "session %s missing user_name", s.ID)
		assert.NotEmpty(t, s.UserID, "session %s missing user_id", s.ID)
		assert.NotEmpty(t, s.AgentType, "session %s missing agent_type", s.ID)
		assert.NotNil(t, s.StartedAt, "session %s missing started_at", s.ID)
		assert.Greater(t, s.MessageCount, 0, "session %s has 0 messages", s.ID)
	}
}

// T1: Sessions endpoint returns ingested data
func TestT1_SessionsEndpointReturnsData(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.server.URL + "/api/v1/sessions")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))

	assert.Greater(t, page.Total, int64(0), "total should be > 0")
	assert.NotEmpty(t, page.Sessions, "sessions array should be non-empty")

	// Each session must have required fields
	for _, s := range page.Sessions {
		assert.NotEmpty(t, s.ID, "session id must be non-empty")
		assert.NotEmpty(t, s.UserName, "user_name must be non-empty")
		assert.NotNil(t, s.StartedAt, "started_at must be non-nil")
	}

	// At least one session should have a non-empty first_message
	hasFirstMessage := false
	for _, s := range page.Sessions {
		if s.FirstMessage != "" {
			hasFirstMessage = true
			break
		}
	}
	assert.True(t, hasFirstMessage, "at least one session should have a non-empty first_message")
}

// T2: Session detail returns valid session
func TestT2_SessionDetailReturnsValidSession(t *testing.T) {
	env := setupTestEnv(t)

	// Get sessions list first
	resp, err := http.Get(env.server.URL + "/api/v1/sessions")
	require.NoError(t, err)
	defer resp.Body.Close()

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	require.NotEmpty(t, page.Sessions)

	targetID := page.Sessions[0].ID

	// Fetch detail
	resp2, err := http.Get(env.server.URL + "/api/v1/sessions/" + targetID)
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	var session store.Session
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&session))
	assert.Equal(t, targetID, session.ID)
	assert.NotEmpty(t, session.UserName)
}

// T3: Session messages with tool calls
func TestT3_SessionMessagesWithToolCalls(t *testing.T) {
	env := setupTestEnv(t)

	// sess-alpha has messages with tool calls
	resp, err := http.Get(env.server.URL + "/api/v1/sessions/sess-alpha/messages")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var messages []api.MessageWithToolCalls
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&messages))

	assert.NotEmpty(t, messages, "messages should be non-empty")

	// Verify ordered by ordinal
	for i := 1; i < len(messages); i++ {
		assert.Greater(t, messages[i].Ordinal, messages[i-1].Ordinal, "messages should be ordered by ordinal")
	}

	// At least one message should have tool_calls
	hasToolCalls := false
	for _, m := range messages {
		if len(m.ToolCalls) > 0 {
			hasToolCalls = true
			for _, tc := range m.ToolCalls {
				assert.NotEmpty(t, tc.ToolName, "tool_name must be non-empty")
				assert.NotEmpty(t, tc.Category, "category must be non-empty")
			}
			break
		}
	}
	assert.True(t, hasToolCalls, "at least one message should have tool_calls")
}

// T4: Pagination works
func TestT4_PaginationWorks(t *testing.T) {
	env := setupTestEnv(t)

	// Fetch first page with limit=2
	resp, err := http.Get(env.server.URL + "/api/v1/sessions?limit=2")
	require.NoError(t, err)
	defer resp.Body.Close()

	var page1 store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page1))
	assert.LessOrEqual(t, len(page1.Sessions), 2, "first page should have at most 2 sessions")

	if page1.NextCursor == "" {
		t.Skip("not enough sessions for pagination test")
	}

	// Fetch second page
	resp2, err := http.Get(env.server.URL + "/api/v1/sessions?limit=2&cursor=" + page1.NextCursor)
	require.NoError(t, err)
	defer resp2.Body.Close()

	var page2 store.SessionPage
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&page2))

	// Sessions on page 2 should differ from page 1
	page1IDs := make(map[string]bool)
	for _, s := range page1.Sessions {
		page1IDs[s.ID] = true
	}
	for _, s := range page2.Sessions {
		assert.False(t, page1IDs[s.ID], "page 2 session %s should not appear on page 1", s.ID)
	}
}

// T5: Filter by user
func TestT5_FilterByUser(t *testing.T) {
	env := setupTestEnv(t)

	// Get users list
	resp, err := http.Get(env.server.URL + "/api/v1/users")
	require.NoError(t, err)
	defer resp.Body.Close()

	var users []store.UserInfo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&users))
	require.NotEmpty(t, users)

	targetID := users[0].ID

	// Filter sessions by user
	resp2, err := http.Get(env.server.URL + "/api/v1/sessions?user_id=" + targetID)
	require.NoError(t, err)
	defer resp2.Body.Close()

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&page))

	for _, s := range page.Sessions {
		assert.Equal(t, targetID, s.UserID, "all sessions should match the user filter")
	}
}

// T6: Filter by project
func TestT6_FilterByProject(t *testing.T) {
	env := setupTestEnv(t)

	// Get projects list
	resp, err := http.Get(env.server.URL + "/api/v1/projects")
	require.NoError(t, err)
	defer resp.Body.Close()

	var projects []store.ProjectInfo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&projects))
	require.NotEmpty(t, projects)

	targetProjectID := projects[0].ID

	// Filter sessions by project
	resp2, err := http.Get(env.server.URL + "/api/v1/sessions?project_id=" + targetProjectID)
	require.NoError(t, err)
	defer resp2.Body.Close()

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&page))

	for _, s := range page.Sessions {
		assert.Equal(t, targetProjectID, s.ProjectID, "all sessions should match the project filter")
	}
}

// T7: Filter by agent
func TestT7_FilterByAgent(t *testing.T) {
	env := setupTestEnv(t)

	// Get agents list
	resp, err := http.Get(env.server.URL + "/api/v1/agents")
	require.NoError(t, err)
	defer resp.Body.Close()

	var agents []string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&agents))
	require.NotEmpty(t, agents)

	targetAgent := agents[0]

	// Filter sessions by agent
	resp2, err := http.Get(env.server.URL + "/api/v1/sessions?agent_type=" + targetAgent)
	require.NoError(t, err)
	defer resp2.Body.Close()

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&page))

	for _, s := range page.Sessions {
		assert.Equal(t, targetAgent, s.AgentType, "all sessions should match the agent filter")
	}
}

// T8: Date range filter
func TestT8_DateRangeFilter(t *testing.T) {
	env := setupTestEnv(t)

	// Get all sessions to find the date range
	resp, err := http.Get(env.server.URL + "/api/v1/sessions")
	require.NoError(t, err)
	defer resp.Body.Close()

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	require.NotEmpty(t, page.Sessions)

	// Sessions are ordered by started_at DESC, so the first one is the latest
	latestDate := page.Sessions[0].StartedAt.Format("2006-01-02")

	// Filter by date_from = latest date
	resp2, err := http.Get(env.server.URL + "/api/v1/sessions?date_from=" + latestDate)
	require.NoError(t, err)
	defer resp2.Body.Close()

	var filteredPage store.SessionPage
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&filteredPage))

	dateFrom, err := time.Parse("2006-01-02", latestDate)
	require.NoError(t, err)

	for _, s := range filteredPage.Sessions {
		require.NotNil(t, s.StartedAt)
		assert.True(t, !s.StartedAt.Before(dateFrom),
			"session %s started_at %v should be >= date_from %v", s.ID, s.StartedAt, dateFrom)
	}
}

// T9: Metadata endpoints return data
func TestT9_MetadataEndpointsReturnData(t *testing.T) {
	env := setupTestEnv(t)

	// Users
	resp, err := http.Get(env.server.URL + "/api/v1/users")
	require.NoError(t, err)
	defer resp.Body.Close()

	var users []store.UserInfo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&users))
	assert.NotEmpty(t, users, "users should be non-empty")
	for _, u := range users {
		assert.NotEmpty(t, u.Name, "user name must be non-empty")
		assert.NotEmpty(t, u.ID, "user id must be non-empty")
	}

	// Projects
	resp2, err := http.Get(env.server.URL + "/api/v1/projects")
	require.NoError(t, err)
	defer resp2.Body.Close()

	var projects []store.ProjectInfo
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&projects))
	assert.NotEmpty(t, projects)
	for _, p := range projects {
		assert.NotEmpty(t, p.Name, "project name must be non-empty")
		assert.NotEmpty(t, p.ID, "project id must be non-empty")
	}

	// Agents
	resp3, err := http.Get(env.server.URL + "/api/v1/agents")
	require.NoError(t, err)
	defer resp3.Body.Close()

	var agents []string
	require.NoError(t, json.NewDecoder(resp3.Body).Decode(&agents))
	assert.NotEmpty(t, agents, "agents should be non-empty")
}

// T10: Secret masking verified (no raw secrets in message content)
func TestT10_SecretMaskingVerified(t *testing.T) {
	env := setupTestEnv(t)

	// Patterns from internal/secrets/secrets.go
	secretPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\b((?:A3T[A-Z0-9]|AKIA|ASIA|ABIA|ACCA)[A-Z2-7]{16})\b`),
		regexp.MustCompile(`ghp_[0-9a-zA-Z]{36}`),
		regexp.MustCompile(`gho_[0-9a-zA-Z]{36}`),
		regexp.MustCompile(`(?:ghu|ghs)_[0-9a-zA-Z]{36}`),
		regexp.MustCompile(`ghr_[0-9a-zA-Z]{36}`),
		regexp.MustCompile(`xox[pboa]-[0-9]{12}-[0-9]{12}-[0-9a-zA-Z]{24}`),
		regexp.MustCompile(`-----BEGIN (?:RSA |DSA |EC |PGP )?PRIVATE KEY`),
		regexp.MustCompile(`sk-[a-zA-Z0-9]{20}T3BlbkFJ[a-zA-Z0-9]{20}`),
		regexp.MustCompile(`sk-ant-[a-zA-Z0-9-]{40,}`),
		regexp.MustCompile(`(?i)(?:postgres|mysql|mongodb)://[^:]+:[^@]+@[^\s]+`),
	}

	// Get all sessions
	resp, err := http.Get(env.server.URL + "/api/v1/sessions")
	require.NoError(t, err)
	defer resp.Body.Close()

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))

	// Check messages from each session
	for _, sess := range page.Sessions {
		resp2, err := http.Get(env.server.URL + "/api/v1/sessions/" + sess.ID + "/messages")
		require.NoError(t, err)

		var messages []api.MessageWithToolCalls
		require.NoError(t, json.NewDecoder(resp2.Body).Decode(&messages))
		resp2.Body.Close()

		for _, msg := range messages {
			for _, pat := range secretPatterns {
				assert.False(t, pat.MatchString(msg.Content),
					"session %s message ordinal %d contains a potential secret matching pattern %s",
					sess.ID, msg.Ordinal, pat.String())
			}
		}
	}
}

// T11: 404 for missing session
func TestT11_NotFoundForMissingSession(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.server.URL + "/api/v1/sessions/nonexistent-id-12345")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// T12: SPA fallback
func TestT12_SPAFallback(t *testing.T) {
	env := setupTestEnv(t)

	// Root path should return 200
	resp, err := http.Get(env.server.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Arbitrary SPA path should also return 200
	resp2, err := http.Get(env.server.URL + "/some/random/path")
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
}

// T13: Malformed cursor returns 400
func TestT13_MalformedCursorReturns400(t *testing.T) {
	env := setupTestEnv(t)

	cases := []struct {
		name   string
		cursor string
	}{
		{"garbage", "not-valid-base64!!!"},
		{"valid base64 but wrong format", "aGVsbG8="},       // "hello" — no pipe separator
		{"empty base64", ""},                                  // empty is not a cursor
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cursor == "" {
				t.Skip("empty cursor is treated as no cursor")
			}
			resp, err := http.Get(env.server.URL + "/api/v1/sessions?cursor=" + tc.cursor)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
				"malformed cursor %q should return 400", tc.cursor)
		})
	}
}

// T14: Limit edge cases are handled gracefully
func TestT14_LimitEdgeCases(t *testing.T) {
	env := setupTestEnv(t)

	cases := []struct {
		name     string
		query    string
		maxItems int
	}{
		{"limit=0 clamps to 1", "limit=0", 1},
		{"limit=-1 clamps to 1", "limit=-1", 1},
		{"limit=999 clamps to 100", "limit=999", 100},
		{"limit=abc uses default 50", "limit=abc", 50},
		{"limit=1", "limit=1", 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(env.server.URL + "/api/v1/sessions?" + tc.query)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var page store.SessionPage
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
			assert.LessOrEqual(t, len(page.Sessions), tc.maxItems,
				"with %s, should return at most %d items", tc.query, tc.maxItems)
		})
	}
}

// T15: date_to filter and combined date range
func TestT15_DateToAndCombinedDateRange(t *testing.T) {
	env := setupTestEnv(t)

	// date_to only: sessions before end of 2026-02-26
	resp, err := http.Get(env.server.URL + "/api/v1/sessions?date_to=2026-02-26")
	require.NoError(t, err)
	defer resp.Body.Close()

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))

	dateTo, err := time.Parse("2006-01-02", "2026-02-26")
	require.NoError(t, err)
	endOfDay := dateTo.Add(24 * time.Hour)
	for _, s := range page.Sessions {
		require.NotNil(t, s.StartedAt)
		assert.True(t, s.StartedAt.Before(endOfDay),
			"session %s started_at %v should be before end of 2026-02-26", s.ID, s.StartedAt)
	}

	// Combined: date_from=2026-02-26 AND date_to=2026-03-01
	resp2, err := http.Get(env.server.URL + "/api/v1/sessions?date_from=2026-02-26&date_to=2026-03-01")
	require.NoError(t, err)
	defer resp2.Body.Close()

	var rangePage store.SessionPage
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&rangePage))

	dateFrom, err := time.Parse("2006-01-02", "2026-02-26")
	require.NoError(t, err)
	rangeEnd, err := time.Parse("2006-01-02", "2026-03-01")
	require.NoError(t, err)
	rangeEndOfDay := rangeEnd.Add(24 * time.Hour)

	for _, s := range rangePage.Sessions {
		require.NotNil(t, s.StartedAt)
		assert.True(t, !s.StartedAt.Before(dateFrom),
			"session %s started_at %v should be >= 2026-02-26", s.ID, s.StartedAt)
		assert.True(t, s.StartedAt.Before(rangeEndOfDay),
			"session %s started_at %v should be < end of 2026-03-01", s.ID, s.StartedAt)
	}
}

// T16: Combined filters (user + project + agent)
func TestT16_CombinedFilters(t *testing.T) {
	env := setupTestEnv(t)

	// Alice + frontend + claude-code: should match sess-alpha and sess-gamma
	resp, err := http.Get(env.server.URL + "/api/v1/sessions?user_id=alice@dev.io&project_id=proj-frontend&agent_type=claude-code")
	require.NoError(t, err)
	defer resp.Body.Close()

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))

	assert.Equal(t, int64(2), page.Total, "Alice+frontend+claude-code should match 2 sessions")
	for _, s := range page.Sessions {
		assert.Equal(t, "alice@dev.io", s.UserID)
		assert.Equal(t, "proj-frontend", s.ProjectID)
		assert.Equal(t, "claude-code", s.AgentType)
	}

	// Bob + backend + cursor: should match sess-beta only
	resp2, err := http.Get(env.server.URL + "/api/v1/sessions?user_id=bob@dev.io&project_id=proj-backend&agent_type=cursor")
	require.NoError(t, err)
	defer resp2.Body.Close()

	var page2 store.SessionPage
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&page2))

	assert.Equal(t, int64(1), page2.Total, "Bob+backend+cursor should match 1 session")
	assert.Equal(t, "sess-beta", page2.Sessions[0].ID)
}

// T17: Empty result set (filter matches nothing)
func TestT17_EmptyResultSet(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.server.URL + "/api/v1/sessions?user_id=nobody@nowhere.com")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))

	assert.Equal(t, int64(0), page.Total)
	assert.Empty(t, page.Sessions)
	assert.Empty(t, page.NextCursor)
}

// T18: Messages for nonexistent session returns empty array
func TestT18_MessagesForNonexistentSession(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.server.URL + "/api/v1/sessions/nonexistent-id/messages")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var messages []api.MessageWithToolCalls
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&messages))
	assert.Empty(t, messages, "messages for nonexistent session should be empty array")
}

// T19: Full pagination walk exhausts all sessions
func TestT19_FullPaginationWalk(t *testing.T) {
	env := setupTestEnv(t)

	var allIDs []string
	cursor := ""
	pageCount := 0

	for {
		url := env.server.URL + "/api/v1/sessions?limit=2"
		if cursor != "" {
			url += "&cursor=" + cursor
		}

		resp, err := http.Get(url)
		require.NoError(t, err)

		var page store.SessionPage
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
		resp.Body.Close()

		for _, s := range page.Sessions {
			allIDs = append(allIDs, s.ID)
		}

		pageCount++
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor

		// Safety: prevent infinite loop
		require.Less(t, pageCount, 10, "pagination should not exceed 10 pages for 5 sessions with limit=2")
	}

	// We seeded 5 sessions, so we should have collected all 5
	assert.Len(t, allIDs, 5, "full pagination should return all 5 sessions")

	// Verify no duplicates
	seen := make(map[string]bool)
	for _, id := range allIDs {
		assert.False(t, seen[id], "session %s appeared more than once across pages", id)
		seen[id] = true
	}
}

// T20: Total count stays consistent across pages
func TestT20_TotalConsistentAcrossPages(t *testing.T) {
	env := setupTestEnv(t)

	// First page
	resp, err := http.Get(env.server.URL + "/api/v1/sessions?limit=2")
	require.NoError(t, err)

	var page1 store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page1))
	resp.Body.Close()

	require.NotEmpty(t, page1.NextCursor, "need at least 2 pages for this test")

	// Second page
	resp2, err := http.Get(env.server.URL + "/api/v1/sessions?limit=2&cursor=" + page1.NextCursor)
	require.NoError(t, err)

	var page2 store.SessionPage
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&page2))
	resp2.Body.Close()

	assert.Equal(t, page1.Total, page2.Total,
		"total should be consistent across pages: page1=%d, page2=%d", page1.Total, page2.Total)
}

// T21: Multiple tool calls grouped under the same message
func TestT21_ToolCallGrouping(t *testing.T) {
	env := setupTestEnv(t)

	// sess-alpha has 2 tool calls on message ordinal 1
	resp, err := http.Get(env.server.URL + "/api/v1/sessions/sess-alpha/messages")
	require.NoError(t, err)
	defer resp.Body.Close()

	var messages []api.MessageWithToolCalls
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&messages))

	// Find the assistant message at ordinal 1
	var toolMsg *api.MessageWithToolCalls
	for i := range messages {
		if messages[i].Ordinal == 1 {
			toolMsg = &messages[i]
			break
		}
	}
	require.NotNil(t, toolMsg, "should find message at ordinal 1")
	assert.Equal(t, "assistant", toolMsg.Role)
	assert.True(t, toolMsg.HasToolUse)
	assert.Len(t, toolMsg.ToolCalls, 2, "ordinal 1 should have 2 grouped tool calls")

	// Verify tool call details
	toolNames := make(map[string]bool)
	for _, tc := range toolMsg.ToolCalls {
		toolNames[tc.ToolName] = true
		assert.NotEmpty(t, tc.ToolUseID)
		assert.NotEmpty(t, tc.Category)
		assert.NotEmpty(t, tc.InputJSON)
	}
	assert.True(t, toolNames["Write"], "should have Write tool call")
	assert.True(t, toolNames["Bash"], "should have Bash tool call")
}

// T22: Sessions are ordered by started_at DESC
func TestT22_SessionOrdering(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.server.URL + "/api/v1/sessions")
	require.NoError(t, err)
	defer resp.Body.Close()

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))

	require.GreaterOrEqual(t, len(page.Sessions), 2, "need at least 2 sessions to test ordering")

	for i := 1; i < len(page.Sessions); i++ {
		prev := page.Sessions[i-1]
		curr := page.Sessions[i]
		require.NotNil(t, prev.StartedAt)
		require.NotNil(t, curr.StartedAt)
		assert.True(t, !prev.StartedAt.Before(*curr.StartedAt),
			"session %s (started %v) should be >= session %s (started %v) in DESC order",
			prev.ID, prev.StartedAt, curr.ID, curr.StartedAt)
	}
}

// T23: Content-Type header is application/json for API endpoints
func TestT23_ContentTypeJSON(t *testing.T) {
	env := setupTestEnv(t)

	endpoints := []string{
		"/api/v1/sessions",
		"/api/v1/sessions/sess-alpha",
		"/api/v1/sessions/sess-alpha/messages",
		"/api/v1/users",
		"/api/v1/projects",
		"/api/v1/agents",
	}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			resp, err := http.Get(env.server.URL + ep)
			require.NoError(t, err)
			defer resp.Body.Close()

			ct := resp.Header.Get("Content-Type")
			assert.Contains(t, ct, "application/json",
				"endpoint %s should return application/json, got %s", ep, ct)
		})
	}
}

// T24: Messages ToolCalls field is always an array, never null
func TestT24_ToolCallsNeverNull(t *testing.T) {
	env := setupTestEnv(t)

	// sess-gamma has no tool calls at all
	resp, err := http.Get(env.server.URL + "/api/v1/sessions/sess-gamma/messages")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Decode as raw JSON to check the actual shape
	var rawMessages []json.RawMessage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rawMessages))
	require.NotEmpty(t, rawMessages)

	for i, raw := range rawMessages {
		var m map[string]interface{}
		require.NoError(t, json.Unmarshal(raw, &m), "message %d should be valid JSON", i)

		tc, exists := m["tool_calls"]
		assert.True(t, exists, "message %d must have tool_calls field", i)
		assert.NotNil(t, tc, "message %d tool_calls must not be null", i)

		// Must be an array
		arr, ok := tc.([]interface{})
		assert.True(t, ok, "message %d tool_calls must be an array, got %T", i, tc)
		_ = arr // may be empty, that's fine
	}
}

// T25: Pagination works correctly with filters applied
func TestT25_PaginationWithFilters(t *testing.T) {
	env := setupTestEnv(t)

	// Alice has 2 sessions (sess-alpha, sess-gamma). Paginate with limit=1.
	resp, err := http.Get(env.server.URL + "/api/v1/sessions?user_id=alice@dev.io&limit=1")
	require.NoError(t, err)
	defer resp.Body.Close()

	var page1 store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page1))

	assert.Equal(t, int64(2), page1.Total, "total should reflect filtered count")
	assert.Len(t, page1.Sessions, 1, "first page should have 1 session")
	assert.NotEmpty(t, page1.NextCursor, "should have next cursor for page 2")

	// Fetch second page
	resp2, err := http.Get(env.server.URL + "/api/v1/sessions?user_id=alice@dev.io&limit=1&cursor=" + page1.NextCursor)
	require.NoError(t, err)
	defer resp2.Body.Close()

	var page2 store.SessionPage
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&page2))

	assert.Equal(t, int64(2), page2.Total, "total should be consistent on page 2")
	assert.Len(t, page2.Sessions, 1, "second page should have 1 session")
	assert.Empty(t, page2.NextCursor, "no more pages")

	// Both sessions should be Alice's and non-overlapping
	assert.NotEqual(t, page1.Sessions[0].ID, page2.Sessions[0].ID)
	assert.Equal(t, "alice@dev.io", page1.Sessions[0].UserID)
	assert.Equal(t, "alice@dev.io", page2.Sessions[0].UserID)
}

// T26: Invalid date format returns an error (not a 200 with wrong data)
func TestT26_InvalidDateFormat(t *testing.T) {
	env := setupTestEnv(t)

	cases := []struct {
		name  string
		query string
	}{
		{"bad date_from", "date_from=not-a-date"},
		{"bad date_to", "date_to=yesterday"},
		{"partial date_from", "date_from=2026-13-01"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(env.server.URL + "/api/v1/sessions?" + tc.query)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
				"invalid date %q should return 400", tc.query)
		})
	}
}

// T27: Error responses have consistent JSON shape with "error" field
func TestT27_ErrorResponseShape(t *testing.T) {
	env := setupTestEnv(t)

	t.Run("404 session not found", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/sessions/nonexistent-id-12345")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		var body map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Contains(t, body, "error", "404 response should have 'error' field")
		assert.NotEmpty(t, body["error"])
	})

	t.Run("400 malformed cursor", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/sessions?cursor=not-valid-base64!!!")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var body map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Contains(t, body, "error", "400 response should have 'error' field")
		assert.NotEmpty(t, body["error"])
	})
}

// T28: Empty sessions list is JSON [] not null
func TestT28_EmptySessionsArrayNotNull(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.server.URL + "/api/v1/sessions?user_id=nobody@nowhere.com")
	require.NoError(t, err)
	defer resp.Body.Close()

	var raw map[string]json.RawMessage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&raw))

	sessionsRaw, exists := raw["sessions"]
	require.True(t, exists, "response must have 'sessions' field")
	assert.Equal(t, "[]", string(sessionsRaw),
		"empty sessions should serialize as [] not null")
}

// T29: Session detail returns all expected fields
func TestT29_SessionDetailAllFields(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.server.URL + "/api/v1/sessions/sess-alpha")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var session store.Session
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&session))

	assert.Equal(t, "sess-alpha", session.ID)
	assert.Equal(t, "Alice", session.UserName)
	assert.Equal(t, "alice@dev.io", session.UserID)
	assert.Equal(t, "frontend", session.ProjectName)
	assert.Equal(t, "laptop", session.Machine)
	assert.Equal(t, "claude-code", session.AgentType)
	assert.Equal(t, "Build the login page", session.FirstMessage)
	assert.NotNil(t, session.StartedAt)
	assert.NotNil(t, session.EndedAt)
	assert.Equal(t, 3, session.MessageCount)
	assert.Equal(t, 2, session.UserMessageCount)
}

// T30: Metadata endpoints return sorted, deduplicated results
func TestT30_MetadataEndpointsSortedAndDeduped(t *testing.T) {
	env := setupTestEnv(t)

	t.Run("users sorted by name", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/users")
		require.NoError(t, err)
		defer resp.Body.Close()

		var users []store.UserInfo
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&users))
		require.Len(t, users, 3)

		// Alice appears in 2 sessions but should be deduplicated
		assert.Equal(t, "Alice", users[0].Name)
		assert.Equal(t, "Bob", users[1].Name)
		assert.Equal(t, "Carol", users[2].Name)
	})

	t.Run("projects sorted by name", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/projects")
		require.NoError(t, err)
		defer resp.Body.Close()

		var projects []store.ProjectInfo
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&projects))
		require.Len(t, projects, 3)

		for i := 1; i < len(projects); i++ {
			assert.True(t, projects[i-1].Name <= projects[i].Name,
				"projects should be sorted: %q should come before %q", projects[i-1].Name, projects[i].Name)
		}
	})

	t.Run("agents sorted alphabetically", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/agents")
		require.NoError(t, err)
		defer resp.Body.Close()

		var agents []string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&agents))
		require.Len(t, agents, 3)

		for i := 1; i < len(agents); i++ {
			assert.True(t, agents[i-1] <= agents[i],
				"agents should be sorted: %q should come before %q", agents[i-1], agents[i])
		}
	})
}

// T31: CORS headers present on API responses
func TestT31_CORSHeaders(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.server.URL + "/api/v1/sessions")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"),
		"API should have CORS Allow-Origin header")
	assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Methods"),
		"API should have CORS Allow-Methods header")
}

// T32: Message has_thinking field correctly reflects seeded data
func TestT32_MessageHasThinkingField(t *testing.T) {
	env := setupTestEnv(t)

	// sess-gamma has a message with has_thinking=true
	resp, err := http.Get(env.server.URL + "/api/v1/sessions/sess-gamma/messages")
	require.NoError(t, err)
	defer resp.Body.Close()

	var messages []api.MessageWithToolCalls
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&messages))
	require.Len(t, messages, 2)

	// The assistant message (ordinal 1) has has_thinking=true
	assert.False(t, messages[0].HasThinking, "user message should not have thinking")
	assert.True(t, messages[1].HasThinking, "assistant message in sess-gamma should have thinking")
	assert.False(t, messages[1].HasToolUse, "sess-gamma assistant has no tool use")
}

// T35: Inverted date range (date_from > date_to) returns empty, not an error
func TestT35_InvertedDateRangeReturnsEmpty(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.server.URL + "/api/v1/sessions?date_from=2026-03-10&date_to=2026-02-01")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var page store.SessionPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	assert.Equal(t, int64(0), page.Total)
	assert.Empty(t, page.Sessions)
}

// T36: Sessions with identical started_at paginate correctly via id DESC tiebreaker
func TestT36_SameTimestampPagination(t *testing.T) {
	env := setupTestEnv(t)

	// Seed 3 additional sessions with the exact same started_at
	ctx := context.Background()
	orgID := ""
	sameTime := ptr(time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC))
	for _, id := range []string{"sess-same-aaa", "sess-same-bbb", "sess-same-ccc"} {
		require.NoError(t, env.store.WriteSession(ctx, orgID, store.Session{
			ID: id, UserID: "dev@test.io", UserName: "Dev",
			ProjectID: "proj-sametest", ProjectName: "sametest", ProjectPath: "/home/dev/sametest",
			AgentType: "claude", FirstMessage: "test " + id,
			StartedAt: sameTime, MessageCount: 1, UserMessageCount: 1,
			SourceCreatedAt: "2026-04-01T12:00:00Z",
		}, []store.Message{
			{OrgID: "", SessionID: id, Ordinal: 0, Role: "user",
				Content: "test", Timestamp: sameTime, ContentLength: 4},
		}, nil))
	}

	// Filter to just the same-timestamp sessions and paginate with limit=1
	var allIDs []string
	cursor := ""
	for i := 0; i < 5; i++ {
		url := env.server.URL + "/api/v1/sessions?project_id=proj-sametest&limit=1"
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		resp, err := http.Get(url)
		require.NoError(t, err)
		var page store.SessionPage
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
		resp.Body.Close()
		for _, s := range page.Sessions {
			allIDs = append(allIDs, s.ID)
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}

	assert.Len(t, allIDs, 3, "should paginate all 3 same-timestamp sessions")
	// ORDER BY started_at DESC, id DESC → ccc, bbb, aaa
	assert.Equal(t, []string{"sess-same-ccc", "sess-same-bbb", "sess-same-aaa"}, allIDs)
}

// T37: Metadata endpoints should exclude hidden sessions (ghosts & subagents)
func TestT37_MetadataExcludesHiddenSessions(t *testing.T) {
	env := setupTestEnv(t)

	ctx := context.Background()
	orgID := ""

	require.NoError(t, env.store.WriteSession(ctx, orgID, store.Session{
		ID: "sess-hidden-ghost", UserID: "ghost@dev.io", UserName: "Ghost Dev",
		ProjectID: "proj-ghost", ProjectName: "ghost-project", ProjectPath: "/home/ghost",
		AgentType: "ghost-agent",
		StartedAt:    ptr(time.Date(2026, 4, 1, 11, 0, 0, 0, time.UTC)),
		MessageCount: 0, UserMessageCount: 0,
		SourceCreatedAt: "2026-04-01T11:00:00Z",
	}, nil, nil))

	require.NoError(t, env.store.WriteSession(ctx, orgID, store.Session{
		ID: "sess-hidden-sub", UserID: "sub@dev.io", UserName: "Sub Dev",
		ProjectID: "proj-sub", ProjectName: "sub-project", ProjectPath: "/home/sub",
		AgentType:       "sub-agent",
		ParentSessionID: "sess-alpha", RelationshipType: "subagent",
		FirstMessage:    "subtask",
		StartedAt:       ptr(time.Date(2026, 4, 1, 10, 30, 0, 0, time.UTC)),
		MessageCount:    1, UserMessageCount: 1,
		SourceCreatedAt: "2026-04-01T10:30:00Z",
	}, []store.Message{
		{OrgID: "", SessionID: "sess-hidden-sub", Ordinal: 0, Role: "user",
			Content: "subtask", Timestamp: ptr(time.Date(2026, 4, 1, 10, 30, 0, 0, time.UTC)), ContentLength: 7},
	}, nil))

	// Users from hidden sessions should NOT appear
	resp, err := http.Get(env.server.URL + "/api/v1/users")
	require.NoError(t, err)
	var users []store.UserInfo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&users))
	resp.Body.Close()

	userNames := make(map[string]bool)
	for _, u := range users {
		userNames[u.Name] = true
	}
	assert.False(t, userNames["Ghost Dev"], "ghost user should not appear in metadata")
	assert.False(t, userNames["Sub Dev"], "subagent user should not appear in metadata")
	assert.True(t, userNames["Alice"], "browsable user should appear")
	assert.True(t, userNames["Bob"], "browsable user should appear")
	assert.True(t, userNames["Carol"], "browsable user should appear")

	// Projects from hidden sessions should NOT appear
	resp, err = http.Get(env.server.URL + "/api/v1/projects")
	require.NoError(t, err)
	var projects []store.ProjectInfo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&projects))
	resp.Body.Close()

	projectSet := make(map[string]bool)
	for _, p := range projects {
		projectSet[p.Name] = true
	}
	assert.False(t, projectSet["ghost-project"], "ghost project should not appear in metadata")
	assert.False(t, projectSet["sub-project"], "subagent project should not appear in metadata")

	// Agents from hidden sessions should NOT appear
	resp, err = http.Get(env.server.URL + "/api/v1/agents")
	require.NoError(t, err)
	var agents []string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&agents))
	resp.Body.Close()

	agentSet := make(map[string]bool)
	for _, a := range agents {
		agentSet[a] = true
	}
	assert.False(t, agentSet["ghost-agent"], "ghost agent should not appear in metadata")
	assert.False(t, agentSet["sub-agent"], "subagent agent should not appear in metadata")
}

// T38: Empty filter params are treated as no filter
func TestT38_EmptyFilterParamsAreNoOps(t *testing.T) {
	env := setupTestEnv(t)

	// No filters
	resp1, err := http.Get(env.server.URL + "/api/v1/sessions")
	require.NoError(t, err)
	var page1 store.SessionPage
	require.NoError(t, json.NewDecoder(resp1.Body).Decode(&page1))
	resp1.Body.Close()

	// Empty string filters
	resp2, err := http.Get(env.server.URL + "/api/v1/sessions?user_id=&project_id=&agent_type=")
	require.NoError(t, err)
	var page2 store.SessionPage
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&page2))
	resp2.Body.Close()

	assert.Equal(t, page1.Total, page2.Total, "empty filter params should not affect results")
	require.Len(t, page2.Sessions, len(page1.Sessions))
	for i := range page1.Sessions {
		assert.Equal(t, page1.Sessions[i].ID, page2.Sessions[i].ID)
	}
}

// --- Search E2E Tests ---

// TestSearch runs all seeded search E2E tests sharing a single environment.
func TestSearch(t *testing.T) {
	env := setupTestEnv(t)

	// S1: Search returns results for known content
	t.Run("S1_ReturnsResults", func(t *testing.T) {
		status, body := httpGet(t, env, "/api/v1/search?q=login")
		assert.Equal(t, http.StatusOK, status)

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))

		assert.NotEmpty(t, page.Results, "search for 'login' should return results")
		assert.Greater(t, page.Total, 0, "total should be > 0")

		found := false
		for _, r := range page.Results {
			if r.SessionID == "sess-alpha" {
				found = true
				break
			}
		}
		assert.True(t, found, "results should include sess-alpha (has 'login' in content)")
		assert.NotEmpty(t, page.Results[0].Snippet, "snippet should not be empty")
		assert.NotNil(t, page.Results[0].Highlights, "highlights should not be nil")
	})

	// S2: Search requires q parameter
	t.Run("S2_RequiresQParam", func(t *testing.T) {
		status, body := httpGet(t, env, "/api/v1/search")
		assert.Equal(t, http.StatusBadRequest, status)

		var errBody map[string]string
		require.NoError(t, json.Unmarshal(body, &errBody))
		assert.Contains(t, errBody, "error")
	})

	// S3: Search with empty q parameter
	t.Run("S3_EmptyQParam", func(t *testing.T) {
		status, _ := httpGet(t, env, "/api/v1/search?q=")
		assert.Equal(t, http.StatusBadRequest, status)
	})

	// S4: Search returns empty results for unknown term
	t.Run("S4_NoResultsForUnknownTerm", func(t *testing.T) {
		status, body := httpGet(t, env, "/api/v1/search?q=zzzznonexistentterm")
		assert.Equal(t, http.StatusOK, status)

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		assert.Empty(t, page.Results)
		assert.Equal(t, 0, page.Total)
	})

	// S5: Search results array never null
	t.Run("S5_ResultsArrayNeverNull", func(t *testing.T) {
		_, body := httpGet(t, env, "/api/v1/search?q=zzzznonexistentterm")

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(body, &raw))

		resultsRaw, exists := raw["results"]
		require.True(t, exists, "response must have 'results' field")
		assert.Equal(t, "[]", string(resultsRaw), "empty results should serialize as [] not null")
	})

	// S6: Filter by user
	t.Run("S6_FilterByUser", func(t *testing.T) {
		status, body := httpGet(t, env, "/api/v1/search?q=login&user_id=alice@dev.io")
		assert.Equal(t, http.StatusOK, status)

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		for _, r := range page.Results {
			assert.Equal(t, "alice@dev.io", r.UserID, "all results should match user filter")
		}
	})

	// S7: Filter by project
	t.Run("S7_FilterByProject", func(t *testing.T) {
		status, body := httpGet(t, env, "/api/v1/search?q=login&project_id=proj-frontend")
		assert.Equal(t, http.StatusOK, status)

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		assert.NotEmpty(t, page.Results, "should find results in frontend project")
		for _, r := range page.Results {
			assert.Equal(t, "frontend", r.ProjectName, "all results should match project filter")
		}
	})

	// S8: Filter by agent
	t.Run("S8_FilterByAgent", func(t *testing.T) {
		status, body := httpGet(t, env, "/api/v1/search?q=login&agent_type=claude-code")
		assert.Equal(t, http.StatusOK, status)

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		assert.NotEmpty(t, page.Results, "should find results for claude-code agent")
		for _, r := range page.Results {
			assert.Equal(t, "claude-code", r.AgentType, "all results should match agent filter")
		}
	})

	// S9: Filter by date range
	t.Run("S9_FilterByDateRange", func(t *testing.T) {
		status, body := httpGet(t, env, "/api/v1/search?q=dark&date_from=2026-03-01&date_to=2026-03-01")
		assert.Equal(t, http.StatusOK, status)

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))

		dateFrom, _ := time.Parse("2006-01-02", "2026-03-01")
		dateToEnd := dateFrom.Add(24 * time.Hour)
		for _, r := range page.Results {
			require.NotNil(t, r.StartedAt)
			assert.True(t, !r.StartedAt.Before(dateFrom), "result started_at should be >= date_from")
			assert.True(t, r.StartedAt.Before(dateToEnd), "result started_at should be < date_to+1day")
		}
	})

	// S10: Combined filters (user + project)
	t.Run("S10_CombinedFilters", func(t *testing.T) {
		status, body := httpGet(t, env, "/api/v1/search?q=login&user_id=alice@dev.io&project_id=proj-frontend")
		assert.Equal(t, http.StatusOK, status)

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		for _, r := range page.Results {
			assert.Equal(t, "alice@dev.io", r.UserID)
			assert.Equal(t, "frontend", r.ProjectName)
		}
	})

	// S11: Filter matching nothing returns empty
	t.Run("S11_FilterMatchingNothing", func(t *testing.T) {
		status, body := httpGet(t, env, "/api/v1/search?q=login&user_id=nobody@nowhere.com")
		assert.Equal(t, http.StatusOK, status)

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		assert.Empty(t, page.Results)
		assert.Equal(t, 0, page.Total)
	})

	// S12: Snippet contains the matched term
	t.Run("S12_SnippetContainsTerm", func(t *testing.T) {
		_, body := httpGet(t, env, "/api/v1/search?q=login")

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		require.NotEmpty(t, page.Results)

		for _, r := range page.Results {
			assert.Contains(t, strings.ToLower(r.Snippet), "login",
				"snippet should contain the search term (case-insensitive)")
		}
	})

	// S13: Highlights point to valid positions in snippet
	t.Run("S13_HighlightsValidPositions", func(t *testing.T) {
		_, body := httpGet(t, env, "/api/v1/search?q=login")

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		require.NotEmpty(t, page.Results)

		for _, r := range page.Results {
			for _, h := range r.Highlights {
				require.LessOrEqual(t, h.End, len(r.Snippet),
					"highlight [%d:%d] exceeds snippet length %d", h.Start, h.End, len(r.Snippet))
				highlighted := r.Snippet[h.Start:h.End]
				assert.Contains(t, strings.ToLower(highlighted), "login",
					"highlighted text %q should match query term", highlighted)
			}
		}
	})

	// S14: Snippet has reasonable length
	t.Run("S14_SnippetReasonableLength", func(t *testing.T) {
		_, body := httpGet(t, env, "/api/v1/search?q=login")

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		require.NotEmpty(t, page.Results)

		for _, r := range page.Results {
			assert.LessOrEqual(t, len(r.Snippet), 210,
				"snippet length %d exceeds max", len(r.Snippet))
		}
	})

	// S15: Search matches tool call input content (tool_name not indexed, but input_json is)
	t.Run("S15_MatchesToolInput_NPM", func(t *testing.T) {
		_, body := httpGet(t, env, "/api/v1/search?q=npm+test")

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		assert.NotEmpty(t, page.Results, "search for 'npm test' should return results from tool call input_json")

		found := false
		for _, r := range page.Results {
			if r.SessionID == "sess-alpha" {
				found = true
				break
			}
		}
		assert.True(t, found, "results should include sess-alpha (has tool call with 'npm test' in input)")
	})

	// S16: Search matches tool call input content
	t.Run("S16_MatchesToolInput", func(t *testing.T) {
		_, body := httpGet(t, env, "/api/v1/search?q=login.tsx")

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		assert.NotEmpty(t, page.Results, "search for tool input 'login.tsx' should return results")
	})

	// S17: Default limit returns reasonable count
	t.Run("S17_DefaultLimit", func(t *testing.T) {
		_, body := httpGet(t, env, "/api/v1/search?q=login")

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		assert.LessOrEqual(t, len(page.Results), 50, "default limit should be <= 50")
	})

	// S18: Custom limit is respected
	t.Run("S18_CustomLimit", func(t *testing.T) {
		_, body := httpGet(t, env, "/api/v1/search?q=login&limit=1")

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		assert.LessOrEqual(t, len(page.Results), 1, "limit=1 should return at most 1 result")
	})

	// S19: Limit edge cases
	t.Run("S19_LimitEdgeCases", func(t *testing.T) {
		cases := []struct {
			name  string
			query string
			max   int
		}{
			{"limit=0 clamps to 1", "limit=0", 1},
			{"limit=-1 clamps to 1", "limit=-1", 1},
			{"limit=999 clamps to 100", "limit=999", 100},
			{"limit=abc uses default 50", "limit=abc", 50},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				status, body := httpGet(t, env, "/api/v1/search?q=login&"+tc.query)
				assert.Equal(t, http.StatusOK, status)

				var page store.SearchPage
				require.NoError(t, json.Unmarshal(body, &page))
				assert.LessOrEqual(t, len(page.Results), tc.max,
					"with %s, should return at most %d results", tc.query, tc.max)
			})
		}
	})

	// S20: Response has correct Content-Type
	t.Run("S20_ContentType", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/search?q=login")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	})

	// S21: Response has CORS headers
	t.Run("S21_CORSHeaders", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/search?q=login")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	})

	// S22: Result fields are all present
	t.Run("S22_ResultFieldsPresent", func(t *testing.T) {
		_, body := httpGet(t, env, "/api/v1/search?q=login")

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(body, &raw))

		var results []json.RawMessage
		require.NoError(t, json.Unmarshal(raw["results"], &results))
		require.NotEmpty(t, results)

		var firstResult map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(results[0], &firstResult))

		expectedKeys := []string{
			"session_id", "ordinal", "role",
			"user_id", "user_name",
			"project_name", "agent_type", "started_at",
			"first_message", "snippet", "highlights",
		}
		for _, key := range expectedKeys {
			_, exists := firstResult[key]
			assert.True(t, exists, "result should have %q field", key)
		}
	})

	// S23: highlights is always an array, never null
	t.Run("S23_HighlightsAlwaysArray", func(t *testing.T) {
		_, body := httpGet(t, env, "/api/v1/search?q=login")

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(body, &raw))

		var results []json.RawMessage
		require.NoError(t, json.Unmarshal(raw["results"], &results))
		require.NotEmpty(t, results)

		for i, r := range results {
			var result map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(r, &result))

			hlRaw, exists := result["highlights"]
			require.True(t, exists, "result %d must have highlights field", i)
			assert.NotEqual(t, "null", string(hlRaw), "result %d highlights must not be null", i)
			trimmed := strings.TrimSpace(string(hlRaw))
			assert.True(t, len(trimmed) > 0 && trimmed[0] == '[',
				"result %d highlights must be an array, got %s", i, trimmed)
		}
	})

	// S25: Search results coexist with session endpoints
	t.Run("S25_CoexistsWithSessions", func(t *testing.T) {
		status, _ := httpGet(t, env, "/api/v1/search?q=login")
		assert.Equal(t, http.StatusOK, status)

		status, body := httpGet(t, env, "/api/v1/sessions")
		assert.Equal(t, http.StatusOK, status)

		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))
		assert.Greater(t, page.Total, int64(0), "sessions should still be accessible after search")
	})
}

// --- Git Links E2E Tests ---

// TestGitLinks runs all seeded git link E2E tests sharing one environment.
func TestGitLinks(t *testing.T) {
	env := setupGitLinkTestEnv(t)

	// --- Core Lookup Tests ---

	t.Run("GL1_LookupByShortSHA", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/gitlinks?sha=a1b2c3d")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		var results []store.GitLinkResult
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&results))
		require.Len(t, results, 1)
		assert.Equal(t, "sess-alpha", results[0].SessionID)
		assert.Equal(t, "commit", results[0].LinkType)
		assert.Equal(t, "high", results[0].Confidence)
		assert.Equal(t, "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4a1b2c3d4", results[0].CommitSHA)
	})

	t.Run("GL2_LookupByFullSHA", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/gitlinks?sha=a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4a1b2c3d4")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		var results []store.GitLinkResult
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&results))
		require.Len(t, results, 1)
		assert.Equal(t, "sess-alpha", results[0].SessionID)
	})

	t.Run("GL3_LookupByPRURL", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/gitlinks?pr=https://github.com/alice/frontend/pull/10")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		var results []store.GitLinkResult
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&results))
		require.Len(t, results, 1)
		assert.Equal(t, "sess-alpha", results[0].SessionID)
		assert.Equal(t, "pr", results[0].LinkType)
		assert.Equal(t, "high", results[0].Confidence)
		assert.Equal(t, "https://github.com/alice/frontend/pull/10", results[0].PRURL)
	})

	t.Run("GL4_LookupNoParams400", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/gitlinks")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 400, resp.StatusCode)
		var body map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Contains(t, body, "error")
	})

	t.Run("GL5_LookupNonexistentSHA", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/gitlinks?sha=0000000")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		// Verify it's [] not null
		assert.Equal(t, "[]\n", string(body))
	})

	t.Run("GL6_LookupNonexistentPRURL", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/gitlinks?pr=https://github.com/nobody/nope/pull/999")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "[]\n", string(body))
	})

	// --- Result Shape & Metadata Tests ---

	t.Run("GL7_ResultHasAllExpectedFields", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/gitlinks?sha=a1b2c3d")
		require.NoError(t, err)
		defer resp.Body.Close()

		var rawResults []map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&rawResults))
		require.Len(t, rawResults, 1)

		expectedKeys := []string{
			"session_id", "user_name", "user_id", "project_id", "project_name", "agent_type",
			"started_at", "first_message", "commit_sha", "pr_url", "link_type",
			"confidence", "message_ordinal",
		}
		for _, key := range expectedKeys {
			_, exists := rawResults[0][key]
			assert.True(t, exists, "result should have key %q", key)
		}
	})

	t.Run("GL8_ResultContainsCorrectMetadata", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/gitlinks?sha=a1b2c3d")
		require.NoError(t, err)
		defer resp.Body.Close()

		var results []store.GitLinkResult
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&results))
		require.Len(t, results, 1)

		r := results[0]
		assert.Equal(t, "Alice", r.UserName)
		assert.Equal(t, "alice@dev.io", r.UserID)
		assert.Equal(t, "frontend", r.ProjectName)
		assert.Equal(t, "claude-code", r.AgentType)
		assert.Equal(t, "Build the login page", r.FirstMessage)
		assert.NotNil(t, r.StartedAt)
		assert.Equal(t, 1, r.MessageOrdinal)
	})

	t.Run("GL9_MediumConfidencePRLink", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/gitlinks?pr=https://github.com/carol/infra/pull/99")
		require.NoError(t, err)
		defer resp.Body.Close()

		var results []store.GitLinkResult
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&results))
		require.Len(t, results, 1)
		assert.Equal(t, "medium", results[0].Confidence)
		assert.Equal(t, "pr", results[0].LinkType)
	})

	// --- Commit Count Tests ---

	t.Run("GL10_ListSessionsIncludesCommitCount", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/sessions?limit=100")
		require.NoError(t, err)
		defer resp.Body.Close()

		var page store.SessionPage
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))

		sessionMap := make(map[string]store.Session)
		for _, s := range page.Sessions {
			sessionMap[s.ID] = s
		}

		assert.Equal(t, 2, sessionMap["sess-alpha"].CommitCount, "sess-alpha: 1 commit + 1 PR = 2")
		assert.Equal(t, 1, sessionMap["sess-gitcommit"].CommitCount, "sess-gitcommit: 1 commit")
		assert.Equal(t, 4, sessionMap["sess-multicommit"].CommitCount, "sess-multicommit: 3 commits + 1 PR = 4")
	})

	t.Run("GL11_ListSessionsCommitCountZeroForNoLinks", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/sessions?limit=100")
		require.NoError(t, err)
		defer resp.Body.Close()

		var page store.SessionPage
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))

		noLinkSessions := []string{"sess-beta", "sess-gamma", "sess-delta", "sess-epsilon"}
		sessionMap := make(map[string]store.Session)
		for _, s := range page.Sessions {
			sessionMap[s.ID] = s
		}

		for _, id := range noLinkSessions {
			sess, ok := sessionMap[id]
			require.True(t, ok, "session %s should be in results", id)
			assert.Equal(t, 0, sess.CommitCount, "session %s should have commit_count 0", id)
		}
	})

	t.Run("GL12_GetSessionIncludesCommitCount", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/sessions/sess-alpha")
		require.NoError(t, err)
		defer resp.Body.Close()

		var sess store.Session
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&sess))
		assert.Equal(t, 2, sess.CommitCount)
	})

	t.Run("GL13_GetSessionCommitCountZero", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/sessions/sess-beta")
		require.NoError(t, err)
		defer resp.Body.Close()

		var sess store.Session
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&sess))
		assert.Equal(t, 0, sess.CommitCount)
	})

	// --- Multiple Links Per Session ---

	t.Run("GL14_MultipleCommitsDistinctSHALookups", func(t *testing.T) {
		shas := []string{"1111111", "2222222", "3333333"}
		for _, sha := range shas {
			resp, err := http.Get(env.server.URL + "/api/v1/gitlinks?sha=" + sha)
			require.NoError(t, err)
			defer resp.Body.Close()

			var results []store.GitLinkResult
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&results))
			require.Len(t, results, 1, "SHA %s should match one result", sha)
			assert.Equal(t, "sess-multicommit", results[0].SessionID)
		}
	})

	t.Run("GL15_CommitCountMatchesTotalLinkCount", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/sessions/sess-multicommit")
		require.NoError(t, err)
		defer resp.Body.Close()

		var sess store.Session
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&sess))
		assert.Equal(t, 4, sess.CommitCount, "3 commits + 1 PR = 4")
	})

	// --- Search Auto-Recognition Tests ---

	t.Run("GL16_SearchSHAAutoRecognizes", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/search?q=a1b2c3d")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		var page store.SearchPage
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
		require.NotEmpty(t, page.Results)
		assert.Equal(t, "sess-alpha", page.Results[0].SessionID)
		assert.Contains(t, page.Results[0].Snippet, "Commit")
		assert.Contains(t, page.Results[0].Snippet, "confidence")
		assert.Empty(t, page.Results[0].Highlights, "synthetic results have no FTS highlights")
	})

	t.Run("GL17_SearchFullSHAAutoRecognizes", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/search?q=f7e8d9c0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		var page store.SearchPage
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
		require.NotEmpty(t, page.Results)
		assert.Equal(t, "sess-gitcommit", page.Results[0].SessionID)
	})

	t.Run("GL18_SearchPRURLAutoRecognizes", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/search?q=https://github.com/alice/frontend/pull/10")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		var page store.SearchPage
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
		require.NotEmpty(t, page.Results)
		assert.Equal(t, "sess-alpha", page.Results[0].SessionID)
		assert.Contains(t, page.Results[0].Snippet, "PR")
		assert.Contains(t, page.Results[0].Snippet, "confidence")
	})

	t.Run("GL19_SearchNonMatchingSHAFallsThrough", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/search?q=0000000deadbeef")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		// Should not error — falls through to FTS (may return empty or FTS results)
	})

	t.Run("GL20_SearchNonHexDoesNotTriggerSHARecognition", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/search?q=login")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		var page store.SearchPage
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))

		// If sess-alpha appears in results, verify it has FTS-style snippet (not "Commit...")
		for _, r := range page.Results {
			if r.SessionID == "sess-alpha" {
				assert.NotContains(t, r.Snippet, "Commit a1b2c3d",
					"FTS search for 'login' should not produce git link snippet")
				break
			}
		}
	})

	t.Run("GL21_SearchAutoRecognitionResponseShape", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/search?q=a1b2c3d")
		require.NoError(t, err)
		defer resp.Body.Close()

		var raw map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&raw))

		assert.Contains(t, raw, "results")
		assert.Contains(t, raw, "total")

		results := raw["results"].([]interface{})
		require.NotEmpty(t, results)

		first := results[0].(map[string]interface{})
		expectedKeys := []string{
			"session_id", "ordinal", "role", "user_id", "user_name",
			"project_name", "agent_type", "started_at", "first_message", "snippet", "highlights",
		}
		for _, key := range expectedKeys {
			_, exists := first[key]
			assert.True(t, exists, "search result should have key %q", key)
		}
	})

	// --- Response Format Tests ---

	t.Run("GL23_ContentTypeJSON", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/gitlinks?sha=a1b2c3d")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	})

	t.Run("GL24_CORSHeaders", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/gitlinks?sha=a1b2c3d")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Origin"))
	})

	t.Run("GL25_EmptyResultIsArrayNotNull", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/gitlinks?sha=0000000")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var raw json.RawMessage
		require.NoError(t, json.Unmarshal(body, &raw))
		assert.Equal(t, "[]", strings.TrimSpace(string(raw)))
	})

	// --- Edge Cases ---

	t.Run("GL26_SHATooShortStillQueries", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/gitlinks?sha=a1b2c3")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		// 6-char prefix may or may not match — the API doesn't enforce minimum length
	})

	t.Run("GL27_BothSHAandPRParams", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/gitlinks?sha=a1b2c3d&pr=https://github.com/alice/frontend/pull/10")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		// Implementation uses sha when both provided (sha branch takes priority in LookupGitLinks)
		var results []store.GitLinkResult
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&results))
		// Should return at least the sha match
		require.NotEmpty(t, results)
		t.Logf("GL27: both params provided, got %d results (sha takes priority)", len(results))
	})

	t.Run("GL28_PaginationWithCommitCount", func(t *testing.T) {
		// Collect all sessions via pagination with limit=2
		var allSessions []store.Session
		cursor := ""
		for {
			path := "/api/v1/sessions?limit=2"
			if cursor != "" {
				path += "&cursor=" + cursor
			}
			resp, err := http.Get(env.server.URL + path)
			require.NoError(t, err)

			var page store.SessionPage
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
			resp.Body.Close()

			allSessions = append(allSessions, page.Sessions...)
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}

		// Build a map of commit counts
		sessionMap := make(map[string]int)
		for _, s := range allSessions {
			sessionMap[s.ID] = s.CommitCount
		}

		// Verify specific sessions have correct commit_count across pages
		assert.Equal(t, 2, sessionMap["sess-alpha"])
		assert.Equal(t, 4, sessionMap["sess-multicommit"])
		assert.Equal(t, 0, sessionMap["sess-beta"])
	})

	t.Run("GL29_FilterByProjectCommitCountCorrect", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/sessions?project_id=proj-frontend")
		require.NoError(t, err)
		defer resp.Body.Close()

		var page store.SessionPage
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))

		sessionMap := make(map[string]store.Session)
		for _, s := range page.Sessions {
			sessionMap[s.ID] = s
		}

		if sess, ok := sessionMap["sess-alpha"]; ok {
			assert.Equal(t, 2, sess.CommitCount, "sess-alpha commit_count should be 2 when filtered by project")
		}
	})
}
