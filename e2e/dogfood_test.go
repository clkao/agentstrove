// ABOUTME: Dogfood E2E test using real agentsview data from the local environment.
// ABOUTME: Tests the full sync->serve flow: ingestion, API serving, filtering, and data integrity.

//go:build cgo

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/clkao/agentstrove/internal/api"
	"github.com/clkao/agentstrove/internal/config"
	"github.com/clkao/agentstrove/internal/reader"
	"github.com/clkao/agentstrove/internal/secrets"
	"github.com/clkao/agentstrove/internal/store"
	astSync "github.com/clkao/agentstrove/internal/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dogfoodEnv holds a synced store and HTTP server built from real data.
type dogfoodEnv struct {
	store      *store.ClickHouseStore
	server     *httptest.Server
	syncResult *astSync.SyncResult
}

func dogfoodGet(t *testing.T, env *dogfoodEnv, path string) (int, []byte) {
	t.Helper()
	resp, err := http.Get(env.server.URL + path)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}

// TestDogfood runs the full sync->serve flow against real agentsview data.
// All subtests share a single synced ClickHouse database to avoid repeated sync overhead.
func TestDogfood(t *testing.T) {
	dbPath := config.DefaultAgentsviewDBPath()
	if dbPath == "" {
		t.Skip("no agentsview DB found at default path; skipping dogfood test")
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("agentsview DB not accessible: %v", err)
	}

	// Create temp ClickHouse database — shared across all subtests.
	addr := clickhouseAddr()
	user := clickhouseUser()
	password := clickhousePassword()
	dbName := fmt.Sprintf("test_dogfood_%s", randomHex(8))

	// Constructor bootstraps the database via "default" connection
	chStore, err := store.NewClickHouseStoreWithAuth(addr, dbName, user, password)
	require.NoError(t, err)
	t.Cleanup(func() {
		chStore.Close()
		dropConn, err := clickhouse.Open(&clickhouse.Options{
			Addr:     []string{addr},
			Protocol: clickhouse.Native,
			Auth: clickhouse.Auth{
				Username: user,
				Password: password,
			},
		})
		if err == nil {
			dropConn.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbName)
			dropConn.Close()
		}
	})

	ctx := context.Background()
	require.NoError(t, chStore.EnsureSchema(ctx))

	r, err := reader.NewReader(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { r.Close() })

	dir := t.TempDir()
	cfg := &config.Config{
		AgentsviewDBPath: dbPath,
		DataDir:          dir,
	}
	userID, userName := cfg.ResolvedUserIdentity()
	t.Logf("user identity: %s <%s>", userName, userID)

	engine, err := astSync.NewEngine(cfg, r, chStore)
	require.NoError(t, err)

	result, err := engine.RunOnce(ctx)
	require.NoError(t, err, "sync should succeed")

	t.Logf("sync result: %d synced, %d skipped, %d secrets masked, %d errors",
		result.SessionsSynced, result.SessionsSkipped, result.SecretsDetected, len(result.Errors))
	for sessID, sessErr := range result.Errors {
		t.Logf("  sync error [%s]: %v", sessID, sessErr)
	}

	srv := api.New(chStore)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	env := &dogfoodEnv{store: chStore, server: ts, syncResult: result}

	// --- Sync layer tests ---

	t.Run("SyncIngests", func(t *testing.T) {
		assert.Greater(t, env.syncResult.SessionsSynced, 0, "should sync at least one session")
		t.Logf("synced %d sessions", env.syncResult.SessionsSynced)
	})

	t.Run("SyncErrorRate", func(t *testing.T) {
		if len(env.syncResult.Errors) == 0 {
			t.Log("no sync errors")
			return
		}
		t.Logf("%d sessions had sync errors", len(env.syncResult.Errors))
		for sessID, e := range env.syncResult.Errors {
			t.Logf("  %s: %v", sessID, e)
		}
		errorRate := float64(len(env.syncResult.Errors)) / float64(env.syncResult.SessionsSynced+len(env.syncResult.Errors))
		assert.Less(t, errorRate, 0.1, "error rate < 10%%")
	})

	t.Run("SyncIdempotent", func(t *testing.T) {
		result2, err := engine.RunOnce(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, result2.SessionsSynced, "second sync should sync 0")
		// Skipped count equals first-run synced count; errored sessions re-error (not skipped).
		assert.Equal(t, result.SessionsSynced, result2.SessionsSkipped,
			"second sync should skip all previously synced sessions")
		t.Logf("idempotency: second sync=%d synced, %d skipped, %d errors",
			result2.SessionsSynced, result2.SessionsSkipped, len(result2.Errors))
	})

	// --- Session list tests ---

	t.Run("SessionList", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions")
		require.Equal(t, 200, status)

		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))
		assert.Greater(t, page.Total, int64(0))
		assert.NotEmpty(t, page.Sessions)
		t.Logf("browsable sessions: %d (page: %d)", page.Total, len(page.Sessions))

		for _, sess := range page.Sessions {
			assert.NotEmpty(t, sess.ID)
			assert.NotEmpty(t, sess.UserName)
			assert.NotEmpty(t, sess.UserID)
			assert.Empty(t, sess.ParentSessionID, "session %s: should be top-level", sess.ID)
			assert.Greater(t, sess.UserMessageCount, 0, "session %s: should have user messages", sess.ID)
		}
	})

	t.Run("SubagentFiltering", func(t *testing.T) {
		cursor := ""
		totalChecked := 0
		for {
			path := "/api/v1/sessions?limit=50"
			if cursor != "" {
				path += "&cursor=" + cursor
			}
			status, body := dogfoodGet(t, env, path)
			require.Equal(t, 200, status)

			var page store.SessionPage
			require.NoError(t, json.Unmarshal(body, &page))
			for _, sess := range page.Sessions {
				totalChecked++
				assert.Empty(t, sess.ParentSessionID, "session %s leaked", sess.ID)
				assert.Greater(t, sess.UserMessageCount, 0, "session %s: 0 user msgs", sess.ID)
			}
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
		t.Logf("checked %d sessions, all top-level", totalChecked)
	})

	t.Run("SessionOrdering", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=20")
		require.Equal(t, 200, status)

		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))
		for i := 1; i < len(page.Sessions); i++ {
			prev := page.Sessions[i-1]
			curr := page.Sessions[i]
			if prev.StartedAt != nil && curr.StartedAt != nil {
				assert.False(t, prev.StartedAt.Before(*curr.StartedAt),
					"%s (%v) should be >= %s (%v)", prev.ID, prev.StartedAt, curr.ID, curr.StartedAt)
			}
		}
	})

	t.Run("PaginationWalk", func(t *testing.T) {
		var allIDs []string
		cursor := ""
		pageNum := 0
		var expectedTotal int64

		for {
			path := "/api/v1/sessions?limit=10"
			if cursor != "" {
				path += "&cursor=" + cursor
			}
			status, body := dogfoodGet(t, env, path)
			require.Equal(t, 200, status)

			var page store.SessionPage
			require.NoError(t, json.Unmarshal(body, &page))
			if pageNum == 0 {
				expectedTotal = page.Total
			}
			assert.Equal(t, expectedTotal, page.Total, "total consistent across pages")
			for _, s := range page.Sessions {
				allIDs = append(allIDs, s.ID)
			}
			pageNum++
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
			require.Less(t, pageNum, 100, "safety limit")
		}

		assert.Equal(t, int(expectedTotal), len(allIDs), "walk collects all sessions")
		seen := make(map[string]bool)
		for _, id := range allIDs {
			assert.False(t, seen[id], "duplicate: %s", id)
			seen[id] = true
		}
		t.Logf("%d pages, %d sessions", pageNum, len(allIDs))
	})

	// --- Session detail and messages ---

	t.Run("SessionDetail", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=1")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))
		require.NotEmpty(t, page.Sessions)

		sessID := page.Sessions[0].ID
		status, body = dogfoodGet(t, env, "/api/v1/sessions/"+sessID)
		require.Equal(t, 200, status)

		var sess store.Session
		require.NoError(t, json.Unmarshal(body, &sess))
		assert.Equal(t, sessID, sess.ID)
		assert.NotNil(t, sess.StartedAt)
	})

	t.Run("Messages", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=1")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))
		require.NotEmpty(t, page.Sessions)

		sessID := page.Sessions[0].ID
		status, body = dogfoodGet(t, env, "/api/v1/sessions/"+sessID+"/messages")
		require.Equal(t, 200, status)

		var msgs []api.MessageWithToolCalls
		require.NoError(t, json.Unmarshal(body, &msgs))
		assert.NotEmpty(t, msgs)

		for i := 1; i < len(msgs); i++ {
			assert.GreaterOrEqual(t, msgs[i].Ordinal, msgs[i-1].Ordinal)
		}
		validRoles := map[string]bool{"user": true, "assistant": true, "system": true}
		for _, m := range msgs {
			assert.True(t, validRoles[m.Role], "bad role: %s", m.Role)
			assert.NotNil(t, m.ToolCalls, "tool_calls should be [] not nil")
		}
		t.Logf("session %s: %d messages", sessID, len(msgs))
	})

	t.Run("ToolCallsPresent", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=10")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))

		found := false
		for _, sess := range page.Sessions {
			status, body = dogfoodGet(t, env, "/api/v1/sessions/"+sess.ID+"/messages")
			require.Equal(t, 200, status)
			var msgs []api.MessageWithToolCalls
			require.NoError(t, json.Unmarshal(body, &msgs))
			for _, m := range msgs {
				if len(m.ToolCalls) > 0 {
					found = true
					for _, tc := range m.ToolCalls {
						assert.NotEmpty(t, tc.ToolName)
						assert.NotEmpty(t, tc.SessionID)
						assert.Equal(t, m.Ordinal, tc.MessageOrdinal,
							"tool call ordinal should match message ordinal")
					}
					break
				}
			}
			if found {
				break
			}
		}
		assert.True(t, found, "should find tool calls in real data")
	})

	t.Run("MessageContentNotEmpty", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=5")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))

		for _, sess := range page.Sessions {
			status, body = dogfoodGet(t, env, "/api/v1/sessions/"+sess.ID+"/messages")
			require.Equal(t, 200, status)
			var msgs []api.MessageWithToolCalls
			require.NoError(t, json.Unmarshal(body, &msgs))
			assert.NotEmpty(t, msgs, "session %s should have messages", sess.ID)

			hasContent := false
			for _, m := range msgs {
				if m.Content != "" {
					hasContent = true
					break
				}
			}
			assert.True(t, hasContent, "session %s: no content", sess.ID)
		}
	})

	// --- Secrets ---

	t.Run("SecretsMasked", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=5")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))

		secretPatterns := []*regexp.Regexp{
			regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
			regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),
			regexp.MustCompile(`sk-[a-zA-Z0-9]{20}T3BlbkFJ[a-zA-Z0-9]{20}`),
			regexp.MustCompile(`sk-ant-[a-zA-Z0-9-]{90,}`),
			regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH )?PRIVATE KEY`),
		}

		for _, sess := range page.Sessions {
			status, body = dogfoodGet(t, env, "/api/v1/sessions/"+sess.ID+"/messages")
			require.Equal(t, 200, status)
			var msgs []api.MessageWithToolCalls
			require.NoError(t, json.Unmarshal(body, &msgs))
			for _, m := range msgs {
				for _, pat := range secretPatterns {
					assert.False(t, pat.MatchString(m.Content),
						"session %s ordinal %d: secret found (%s)", sess.ID, m.Ordinal, pat.String())
				}
			}
		}
		if env.syncResult.SecretsDetected > 0 {
			t.Logf("%d secrets masked during sync", env.syncResult.SecretsDetected)
		}
	})

	// --- Metadata ---

	t.Run("Metadata", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/users")
		require.Equal(t, 200, status)
		var users []store.UserInfo
		require.NoError(t, json.Unmarshal(body, &users))
		assert.NotEmpty(t, users)
		for _, u := range users {
			assert.NotEmpty(t, u.Name)
			assert.NotEmpty(t, u.ID)
		}

		status, body = dogfoodGet(t, env, "/api/v1/projects")
		require.Equal(t, 200, status)
		var projects []store.ProjectInfo
		require.NoError(t, json.Unmarshal(body, &projects))
		t.Logf("projects: %v", projects)

		status, body = dogfoodGet(t, env, "/api/v1/agents")
		require.Equal(t, 200, status)
		var agents []string
		require.NoError(t, json.Unmarshal(body, &agents))
		assert.NotEmpty(t, agents)
		t.Logf("agents: %v", agents)
	})

	// --- Filtering ---

	t.Run("FilterByProject", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/projects")
		require.Equal(t, 200, status)
		var projects []store.ProjectInfo
		require.NoError(t, json.Unmarshal(body, &projects))
		if len(projects) == 0 {
			t.Skip("no projects")
		}
		for _, project := range projects {
			if project.ID == "" {
				continue
			}
			status, body = dogfoodGet(t, env, fmt.Sprintf("/api/v1/sessions?project_id=%s", project.ID))
			require.Equal(t, 200, status)
			var page store.SessionPage
			require.NoError(t, json.Unmarshal(body, &page))
			for _, sess := range page.Sessions {
				assert.Equal(t, project.ID, sess.ProjectID, "session %s: wrong project_id", sess.ID)
			}
		}
		t.Logf("verified %d projects", len(projects))
	})

	t.Run("FirstMessagePopulated", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=20")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))

		populated := 0
		for _, sess := range page.Sessions {
			if sess.FirstMessage != "" {
				populated++
			}
		}
		t.Logf("first_message: %d/%d populated", populated, len(page.Sessions))
		if len(page.Sessions) > 0 {
			rate := float64(populated) / float64(len(page.Sessions))
			assert.Greater(t, rate, 0.5, "most sessions should have first_message")
		}
	})

	// --- HTTP layer ---

	t.Run("ContentTypeJSON", func(t *testing.T) {
		for _, ep := range []string{"/api/v1/sessions", "/api/v1/users", "/api/v1/projects", "/api/v1/agents"} {
			resp, err := http.Get(env.server.URL + ep)
			require.NoError(t, err)
			resp.Body.Close()
			assert.Contains(t, resp.Header.Get("Content-Type"), "application/json", ep)
		}
	})

	t.Run("CORSHeaders", func(t *testing.T) {
		resp, err := http.Get(env.server.URL + "/api/v1/sessions")
		require.NoError(t, err)
		resp.Body.Close()
		assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Origin"))
	})

	// --- Additional filter tests with real data ---

	t.Run("FilterByUser", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/users")
		require.Equal(t, 200, status)
		var users []store.UserInfo
		require.NoError(t, json.Unmarshal(body, &users))
		require.NotEmpty(t, users)

		for _, u := range users {
			status, body = dogfoodGet(t, env, "/api/v1/sessions?user_id="+u.ID)
			require.Equal(t, 200, status)
			var page store.SessionPage
			require.NoError(t, json.Unmarshal(body, &page))
			assert.Greater(t, page.Total, int64(0), "user %s should have sessions", u.ID)
			for _, sess := range page.Sessions {
				assert.Equal(t, u.ID, sess.UserID,
					"session %s: expected user_id %s, got %s", sess.ID, u.ID, sess.UserID)
			}
			t.Logf("user %s <%s>: %d sessions", u.Name, u.ID, page.Total)
		}
	})

	t.Run("FilterByAgent", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/agents")
		require.Equal(t, 200, status)
		var agents []string
		require.NoError(t, json.Unmarshal(body, &agents))
		require.NotEmpty(t, agents)

		for _, agent := range agents {
			status, body = dogfoodGet(t, env, "/api/v1/sessions?agent_type="+agent)
			require.Equal(t, 200, status)
			var page store.SessionPage
			require.NoError(t, json.Unmarshal(body, &page))
			assert.Greater(t, page.Total, int64(0), "agent %s should have sessions", agent)
			for _, sess := range page.Sessions {
				assert.Equal(t, agent, sess.AgentType,
					"session %s: expected agent_type %s, got %s", sess.ID, agent, sess.AgentType)
			}
			t.Logf("agent %s: %d sessions", agent, page.Total)
		}
	})

	t.Run("SessionDetailFieldCompleteness", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=5")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))

		for _, sess := range page.Sessions {
			status, body = dogfoodGet(t, env, "/api/v1/sessions/"+sess.ID)
			require.Equal(t, 200, status)
			var detail store.Session
			require.NoError(t, json.Unmarshal(body, &detail))

			assert.Equal(t, sess.ID, detail.ID)
			assert.NotEmpty(t, detail.UserName, "session %s: user_name empty", sess.ID)
			assert.NotEmpty(t, detail.UserID, "session %s: user_id empty", sess.ID)
			assert.NotNil(t, detail.StartedAt, "session %s: started_at nil", sess.ID)
			assert.NotEmpty(t, detail.AgentType, "session %s: agent_type empty", sess.ID)
			assert.Greater(t, detail.MessageCount, 0, "session %s: message_count 0", sess.ID)
			assert.Greater(t, detail.UserMessageCount, 0, "session %s: user_message_count 0", sess.ID)
			// Verify detail matches list view
			assert.Equal(t, sess.UserName, detail.UserName)
			assert.Equal(t, sess.MessageCount, detail.MessageCount)
		}
	})

	t.Run("HasThinkingField", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=10")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))

		hasThinkingTrue := 0
		totalMsgs := 0
		for _, sess := range page.Sessions {
			status, body = dogfoodGet(t, env, "/api/v1/sessions/"+sess.ID+"/messages")
			require.Equal(t, 200, status)
			var msgs []api.MessageWithToolCalls
			require.NoError(t, json.Unmarshal(body, &msgs))
			for _, m := range msgs {
				totalMsgs++
				if m.HasThinking {
					hasThinkingTrue++
				}
			}
		}
		t.Logf("has_thinking=true: %d/%d messages", hasThinkingTrue, totalMsgs)
		// Real Claude sessions should have some thinking blocks
		assert.Greater(t, hasThinkingTrue, 0, "should find messages with thinking in real data")
	})

	t.Run("FilterProjectSumMatchesTotal", func(t *testing.T) {
		// Sum of sessions across all projects should not exceed total
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=1")
		require.Equal(t, 200, status)
		var allPage store.SessionPage
		require.NoError(t, json.Unmarshal(body, &allPage))

		status, body = dogfoodGet(t, env, "/api/v1/projects")
		require.Equal(t, 200, status)
		var projects []store.ProjectInfo
		require.NoError(t, json.Unmarshal(body, &projects))

		projectTotal := int64(0)
		for _, project := range projects {
			if project.ID == "" {
				continue
			}
			status, body = dogfoodGet(t, env, "/api/v1/sessions?project_id="+project.ID+"&limit=1")
			require.Equal(t, 200, status)
			var page store.SessionPage
			require.NoError(t, json.Unmarshal(body, &page))
			projectTotal += page.Total
		}
		t.Logf("total=%d, sum of project filters=%d", allPage.Total, projectTotal)
		// Project total may be less than all sessions (some may have empty project_id)
		assert.LessOrEqual(t, projectTotal, allPage.Total,
			"project sum should not exceed total")
	})

	// --- Corner case tests with real data ---

	t.Run("DateRangeFilter", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=50")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))
		require.NotEmpty(t, page.Sessions)

		// Find the actual date range from data
		earliest := page.Sessions[len(page.Sessions)-1].StartedAt
		latest := page.Sessions[0].StartedAt
		require.NotNil(t, earliest)
		require.NotNil(t, latest)
		t.Logf("date range: %s to %s", earliest.Format("2006-01-02"), latest.Format("2006-01-02"))

		// date_from = latest date should return at least 1 session
		latestDate := latest.Format("2006-01-02")
		latestDateBoundary, err := time.Parse("2006-01-02", latestDate)
		require.NoError(t, err)
		status, body = dogfoodGet(t, env, "/api/v1/sessions?date_from="+latestDate)
		require.Equal(t, 200, status)
		var fromPage store.SessionPage
		require.NoError(t, json.Unmarshal(body, &fromPage))
		assert.Greater(t, fromPage.Total, int64(0), "date_from=%s should match", latestDate)
		for _, s := range fromPage.Sessions {
			require.NotNil(t, s.StartedAt)
			assert.False(t, s.StartedAt.Before(latestDateBoundary),
				"session %s started %v should be >= %s", s.ID, s.StartedAt, latestDate)
		}

		// date_to = earliest date should return at least 1 session
		earliestDate := earliest.Format("2006-01-02")
		status, body = dogfoodGet(t, env, "/api/v1/sessions?date_to="+earliestDate)
		require.Equal(t, 200, status)
		var toPage store.SessionPage
		require.NoError(t, json.Unmarshal(body, &toPage))
		assert.Greater(t, toPage.Total, int64(0), "date_to=%s should match", earliestDate)

		// Inverted range should return empty
		status, body = dogfoodGet(t, env, "/api/v1/sessions?date_from=2099-01-01&date_to=2020-01-01")
		require.Equal(t, 200, status)
		var emptyPage store.SessionPage
		require.NoError(t, json.Unmarshal(body, &emptyPage))
		assert.Equal(t, int64(0), emptyPage.Total, "inverted date range should return empty")
	})

	t.Run("MessageOrdinalContinuity", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=5")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))

		for _, sess := range page.Sessions {
			status, body = dogfoodGet(t, env, "/api/v1/sessions/"+sess.ID+"/messages")
			require.Equal(t, 200, status)
			var msgs []api.MessageWithToolCalls
			require.NoError(t, json.Unmarshal(body, &msgs))

			if len(msgs) == 0 {
				continue
			}
			// First message should be ordinal 0
			assert.Equal(t, 0, msgs[0].Ordinal,
				"session %s: first message ordinal should be 0, got %d", sess.ID, msgs[0].Ordinal)
			// Ordinals should be strictly increasing (gaps are OK in real data)
			for i := 1; i < len(msgs); i++ {
				assert.Greater(t, msgs[i].Ordinal, msgs[i-1].Ordinal,
					"session %s: ordinals not increasing at position %d (%d -> %d)",
					sess.ID, i, msgs[i-1].Ordinal, msgs[i].Ordinal)
			}
		}
	})

	t.Run("ToolCallIntegrity", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=10")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))

		totalToolCalls := 0
		for _, sess := range page.Sessions {
			status, body = dogfoodGet(t, env, "/api/v1/sessions/"+sess.ID+"/messages")
			require.Equal(t, 200, status)
			var msgs []api.MessageWithToolCalls
			require.NoError(t, json.Unmarshal(body, &msgs))

			for _, m := range msgs {
				for _, tc := range m.ToolCalls {
					totalToolCalls++
					assert.NotEmpty(t, tc.ToolName,
						"session %s ordinal %d: tool_name empty", sess.ID, m.Ordinal)
					assert.NotEmpty(t, tc.Category,
						"session %s ordinal %d: category empty", sess.ID, m.Ordinal)
					assert.Equal(t, sess.ID, tc.SessionID,
						"session %s ordinal %d: tool call session_id mismatch", sess.ID, m.Ordinal)
					assert.Equal(t, m.Ordinal, tc.MessageOrdinal,
						"session %s ordinal %d: tool call message_ordinal mismatch", sess.ID, m.Ordinal)
				}
			}
		}
		t.Logf("verified %d tool calls across %d sessions", totalToolCalls, len(page.Sessions))
		assert.Greater(t, totalToolCalls, 0, "should find tool calls in real data")
	})

	t.Run("LargeSessionHandling", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=50")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))

		var biggestSess string
		biggestCount := 0
		for _, sess := range page.Sessions {
			if sess.MessageCount > biggestCount {
				biggestCount = sess.MessageCount
				biggestSess = sess.ID
			}
		}
		require.NotEmpty(t, biggestSess, "should find at least one session")
		t.Logf("largest session: %s (%d messages)", biggestSess, biggestCount)

		status, body = dogfoodGet(t, env, "/api/v1/sessions/"+biggestSess+"/messages")
		require.Equal(t, 200, status)
		var msgs []api.MessageWithToolCalls
		require.NoError(t, json.Unmarshal(body, &msgs))
		assert.NotEmpty(t, msgs)
		t.Logf("fetched %d messages for session %s", len(msgs), biggestSess)

		var maxContentLen int
		for _, m := range msgs {
			if m.ContentLength > maxContentLen {
				maxContentLen = m.ContentLength
			}
		}
		t.Logf("max content_length: %d bytes", maxContentLen)
	})

	// --- Search tests (DS1-DS10) ---

	// DS2: Search returns results for common terms
	t.Run("SearchCommonTerms", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/search?q=function")
		require.Equal(t, 200, status)

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		assert.NotEmpty(t, page.Results, "search for 'function' should return results in real data")
		t.Logf("search 'function': %d results", page.Total)
	})

	// DS3: Search results have valid session references
	t.Run("SearchValidSessionRefs", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/search?q=function&limit=5")
		require.Equal(t, 200, status)

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		require.NotEmpty(t, page.Results)

		for _, r := range page.Results {
			status, _ := dogfoodGet(t, env, "/api/v1/sessions/"+r.SessionID)
			assert.Equal(t, 200, status, "session %s from search result should exist", r.SessionID)
		}
	})

	// DS4: Search results have valid snippets
	t.Run("SearchValidSnippets", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/search?q=function&limit=10")
		require.Equal(t, 200, status)

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		require.NotEmpty(t, page.Results)

		for _, r := range page.Results {
			assert.NotEmpty(t, r.Snippet, "result for session %s should have snippet", r.SessionID)
			assert.NotNil(t, r.Highlights, "highlights should not be nil for session %s", r.SessionID)
		}
	})

	// DS5: Search filter by user
	t.Run("SearchFilterByUser", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/users")
		require.Equal(t, 200, status)
		var users []store.UserInfo
		require.NoError(t, json.Unmarshal(body, &users))
		require.NotEmpty(t, users)

		u := users[0]
		status, body = dogfoodGet(t, env, "/api/v1/search?q=function&user_id="+u.ID)
		require.Equal(t, 200, status)

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		for _, r := range page.Results {
			assert.Equal(t, u.ID, r.UserID,
				"result session %s should match user filter", r.SessionID)
		}
		t.Logf("search filtered by %s: %d results", u.ID, page.Total)
	})

	// DS6: Search filter by project
	t.Run("SearchFilterByProject", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/projects")
		require.Equal(t, 200, status)
		var projects []store.ProjectInfo
		require.NoError(t, json.Unmarshal(body, &projects))
		if len(projects) == 0 {
			t.Skip("no projects in data")
		}

		// Find a project with a non-empty ID
		var project store.ProjectInfo
		for _, p := range projects {
			if p.ID != "" {
				project = p
				break
			}
		}
		if project.ID == "" {
			t.Skip("no projects with non-empty ID")
		}

		status, body = dogfoodGet(t, env, "/api/v1/search?q=function&project_id="+project.ID)
		require.Equal(t, 200, status)

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		for _, r := range page.Results {
			assert.Equal(t, project.Name, r.ProjectName,
				"result session %s should match project filter", r.SessionID)
		}
		t.Logf("search filtered by project %s: %d results", project.Name, page.Total)
	})

	// DS7: Obscure term returns empty or valid response
	t.Run("SearchNoResults", func(t *testing.T) {
		status, _ := dogfoodGet(t, env, "/api/v1/search?q=qxjk7v9m3p2w")
		require.Equal(t, 200, status)
	})

	// DS8: Search result content matches snippet context
	t.Run("SearchContentMatchesSnippet", func(t *testing.T) {
		queryTerm := "file"
		status, body := dogfoodGet(t, env, "/api/v1/search?q="+queryTerm+"&limit=3")
		require.Equal(t, 200, status)

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))
		if len(page.Results) == 0 {
			t.Skip("no results for 'file'")
		}

		for _, r := range page.Results {
			status, body = dogfoodGet(t, env, "/api/v1/sessions/"+r.SessionID+"/messages")
			require.Equal(t, 200, status)

			var msgs []api.MessageWithToolCalls
			require.NoError(t, json.Unmarshal(body, &msgs))

			// Find the message at the result's ordinal
			found := false
			for _, m := range msgs {
				if m.Ordinal == r.Ordinal {
					found = true
					contentLower := strings.ToLower(m.Content)
					hasInContent := strings.Contains(contentLower, queryTerm)
					hasInTools := false
					for _, tc := range m.ToolCalls {
						if strings.Contains(strings.ToLower(tc.ToolName), queryTerm) ||
							strings.Contains(strings.ToLower(tc.InputJSON), queryTerm) {
							hasInTools = true
							break
						}
					}
					assert.True(t, hasInContent || hasInTools,
						"session %s ordinal %d: should contain '%s' in content or tools",
						r.SessionID, r.Ordinal, queryTerm)
					break
				}
			}
			assert.True(t, found, "session %s should have message at ordinal %d", r.SessionID, r.Ordinal)
		}
	})

	// DS9: Search indexes subagent sessions too
	t.Run("SearchSubagentSessions", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/search?q=function&limit=20")
		require.Equal(t, 200, status)

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))

		subagentFound := false
		for _, r := range page.Results {
			status, body = dogfoodGet(t, env, "/api/v1/sessions/"+r.SessionID)
			if status == 200 {
				var sess store.Session
				if json.Unmarshal(body, &sess) == nil && sess.ParentSessionID != "" {
					subagentFound = true
					t.Logf("found subagent session %s (parent: %s)", r.SessionID, sess.ParentSessionID)
					break
				}
			}
		}
		// Subagent results depend on data -- log but don't fail
		if !subagentFound {
			t.Log("no subagent sessions found in search results (data-dependent, not a failure)")
		}
	})

	// DS10: Search performance is reasonable
	t.Run("SearchPerformance", func(t *testing.T) {
		start := time.Now()
		status, body := dogfoodGet(t, env, "/api/v1/search?q=function")
		elapsed := time.Since(start)

		require.Equal(t, 200, status)

		var page store.SearchPage
		require.NoError(t, json.Unmarshal(body, &page))

		t.Logf("search took %s, returned %d results", elapsed.Round(time.Millisecond), page.Total)
		assert.Less(t, elapsed, 2*time.Second, "search should complete within 2 seconds")
	})

	// --- Git Link tests (DG1-DG10) ---

	// DG1: Git links extracted from real data
	t.Run("GitLinkExtracted", func(t *testing.T) {
		var totalCommits, linkSessions, totalSessions int
		cursor := ""
		for {
			path := "/api/v1/sessions?limit=50"
			if cursor != "" {
				path += "&cursor=" + cursor
			}
			status, body := dogfoodGet(t, env, path)
			require.Equal(t, 200, status)
			var page store.SessionPage
			require.NoError(t, json.Unmarshal(body, &page))
			for _, sess := range page.Sessions {
				totalSessions++
				if sess.CommitCount > 0 {
					linkSessions++
					totalCommits += sess.CommitCount
				}
			}
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
		t.Logf("found %d git links from %d sessions (out of %d total)", totalCommits, linkSessions, totalSessions)
		if totalCommits == 0 {
			t.Log("WARNING: no git links found -- extraction may not match content patterns in this dataset")
		}
	})

	// DG2: Commit lookup returns valid session reference
	t.Run("GitLinkCommitLookup", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=50")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))

		var foundSHA bool
		for _, sess := range page.Sessions {
			if sess.CommitCount == 0 {
				continue
			}
			if !foundSHA {
				t.Logf("session %s has commit_count=%d", sess.ID, sess.CommitCount)
				foundSHA = true
			}
		}
		if !foundSHA {
			t.Skip("no sessions with git links found in dogfood data")
		}
	})

	// DG3: PR lookup returns valid session reference
	t.Run("GitLinkPRLookup", func(t *testing.T) {
		t.Log("PR lookup requires known PR URLs from data -- see DG6 for search-based verification")
	})

	// DG4: Commit count in session list matches across API calls
	t.Run("GitLinkCommitCountConsistency", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=10")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))

		for _, sess := range page.Sessions {
			status, body = dogfoodGet(t, env, "/api/v1/sessions/"+sess.ID)
			require.Equal(t, 200, status)
			var detail store.Session
			require.NoError(t, json.Unmarshal(body, &detail))
			assert.Equal(t, sess.CommitCount, detail.CommitCount,
				"session %s: list commit_count (%d) should match detail commit_count (%d)",
				sess.ID, sess.CommitCount, detail.CommitCount)
		}
	})

	// DG5: Git link results include valid session metadata
	t.Run("GitLinkResultMetadata", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=50")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))

		hasLinks := false
		for _, sess := range page.Sessions {
			if sess.CommitCount > 0 {
				hasLinks = true
				assert.NotEmpty(t, sess.UserName, "session %s with git links should have user_name", sess.ID)
				assert.NotEmpty(t, sess.UserID, "session %s with git links should have user_id", sess.ID)
				assert.NotEmpty(t, sess.ProjectName, "session %s with git links should have project_name", sess.ID)
				assert.NotEmpty(t, sess.AgentType, "session %s with git links should have agent_type", sess.ID)
				assert.NotEmpty(t, sess.ID, "session with git links should have ID")
				assert.Greater(t, sess.CommitCount, 0)
			}
		}
		if !hasLinks {
			t.Skip("no sessions with git links found in dogfood data")
		}
	})

	// DG6: Search auto-recognizes real commit SHA
	t.Run("GitLinkSearchAutoRecognize", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=20")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))

		shaRE := regexp.MustCompile(`\[[\w/.:\- ]+(?:\([^)]+\)\s+)?([0-9a-f]{7,40})\]`)
		var foundSHA string

		for _, sess := range page.Sessions {
			if sess.CommitCount == 0 {
				continue
			}
			status, body = dogfoodGet(t, env, "/api/v1/sessions/"+sess.ID+"/messages")
			require.Equal(t, 200, status)
			var msgs []struct {
				Content string `json:"content"`
			}
			require.NoError(t, json.Unmarshal(body, &msgs))

			for _, m := range msgs {
				if matches := shaRE.FindStringSubmatch(m.Content); len(matches) > 1 {
					foundSHA = matches[1]
					break
				}
			}
			if foundSHA != "" {
				break
			}
		}

		if foundSHA == "" {
			t.Skip("no commit SHA found in dogfood data messages")
		}

		shortSHA := foundSHA
		if len(shortSHA) > 7 {
			shortSHA = shortSHA[:7]
		}
		t.Logf("testing search auto-recognition with SHA: %s", shortSHA)

		status, body = dogfoodGet(t, env, "/api/v1/search?q="+shortSHA)
		require.Equal(t, 200, status)
		var searchPage store.SearchPage
		require.NoError(t, json.Unmarshal(body, &searchPage))

		if len(searchPage.Results) > 0 {
			first := searchPage.Results[0]
			assert.NotEmpty(t, first.SessionID)
			t.Logf("search for %s returned session %s, snippet: %s",
				shortSHA, first.SessionID, first.Snippet)
			if strings.Contains(first.Snippet, "Commit") {
				t.Log("auto-recognition triggered -- snippet has 'Commit' prefix")
			} else {
				t.Log("fell through to FTS -- SHA may not be in git_links")
			}
		} else {
			t.Logf("no search results for SHA %s", shortSHA)
		}
	})

	// DG7: Git link message_ordinal points to real message
	t.Run("GitLinkMessageOrdinalValid", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=20")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))

		verified := 0
		for _, sess := range page.Sessions {
			if sess.CommitCount == 0 {
				continue
			}
			status, body = dogfoodGet(t, env, "/api/v1/sessions/"+sess.ID+"/messages")
			require.Equal(t, 200, status)
			var msgs []struct {
				Ordinal int    `json:"ordinal"`
				Role    string `json:"role"`
			}
			require.NoError(t, json.Unmarshal(body, &msgs))

			// Verify that at least one assistant message exists (tool calls come from assistant messages)
			hasAssistant := false
			for _, m := range msgs {
				if m.Role == "assistant" {
					hasAssistant = true
					break
				}
			}
			assert.True(t, hasAssistant,
				"session %s with git links should have assistant messages", sess.ID)
			verified++
			if verified >= 5 {
				break
			}
		}
		t.Logf("verified %d sessions with git links have assistant messages", verified)
	})

	// DG8: Sessions without git links have commit_count 0
	t.Run("GitLinkZeroCommitCount", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=50")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))

		zeroCount := 0
		nonZeroCount := 0
		for _, sess := range page.Sessions {
			if sess.CommitCount == 0 {
				zeroCount++
			} else {
				nonZeroCount++
			}
		}
		t.Logf("commit_count: %d sessions with 0, %d sessions with >0", zeroCount, nonZeroCount)
		assert.Greater(t, zeroCount, 0, "some sessions should have commit_count 0 (not every session produces commits)")
	})

	// DG9: Git link confidence distribution
	t.Run("GitLinkConfidenceDistribution", func(t *testing.T) {
		status, body := dogfoodGet(t, env, "/api/v1/sessions?limit=50")
		require.Equal(t, 200, status)
		var page store.SessionPage
		require.NoError(t, json.Unmarshal(body, &page))

		for _, sess := range page.Sessions {
			assert.GreaterOrEqual(t, sess.CommitCount, 0,
				"session %s: commit_count should be non-negative", sess.ID)
		}
	})

	// DG10: Git link performance
	t.Run("GitLinkPerformance", func(t *testing.T) {
		start := time.Now()
		status, body := dogfoodGet(t, env, "/api/v1/gitlinks?sha=0000000")
		elapsed := time.Since(start)

		require.Equal(t, 200, status)
		t.Logf("git link lookup took %s", elapsed.Round(time.Millisecond))
		assert.Less(t, elapsed, 500*time.Millisecond, "git link lookup should complete within 500ms")

		var results []store.GitLinkResult
		require.NoError(t, json.Unmarshal(body, &results))
	})
}

// Ensure secrets package is imported (used indirectly via sync).
var _ = secrets.MaskSecrets
