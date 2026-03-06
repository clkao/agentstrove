// ABOUTME: Test helpers for E2E API tests against ClickHouse.
// ABOUTME: Provides test environment setup, data seeding, and HTTP helper functions.
package e2e

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/clkao/agentlore/internal/api"
	"github.com/clkao/agentlore/internal/store"
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
	return "agentlore"
}

func clickhousePassword() string {
	return os.Getenv("CLICKHOUSE_PASSWORD")
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func ptr[T any](v T) *T { return &v }

// testEnv holds a seeded store and running HTTP test server.
type testEnv struct {
	store  *store.ClickHouseStore
	server *httptest.Server
}

// setupTestEnv creates a ClickHouse store with seeded data and an httptest server.
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	addr := clickhouseAddr()
	user := clickhouseUser()
	password := clickhousePassword()
	dbName := fmt.Sprintf("e2e_%s", randomHex(8))

	// Constructor bootstraps the database via "default" connection
	s, err := store.NewClickHouseStoreWithAuth(addr, dbName, user, password)
	require.NoError(t, err, "create store")
	require.NoError(t, s.EnsureSchema(context.Background()))

	t.Cleanup(func() {
		_ = s.Close()
		dropConn, err := clickhouse.Open(&clickhouse.Options{
			Addr:     []string{addr},
			Protocol: clickhouse.Native,
			Auth:     clickhouse.Auth{Username: user, Password: password},
		})
		if err == nil {
			_ = dropConn.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbName)
			_ = dropConn.Close()
		}
	})

	seedData(t, s)

	srv := api.New(s)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	return &testEnv{store: s, server: ts}
}

// setupGitLinkTestEnv creates a test environment with both base data and git link data.
func setupGitLinkTestEnv(t *testing.T) *testEnv {
	t.Helper()
	env := setupTestEnv(t)
	seedGitLinkData(t, env.store)
	return env
}

// httpGet is a helper for HTTP GET calls returning status and body.
func httpGet(t *testing.T, env *testEnv, path string) (int, []byte) {
	t.Helper()
	resp, err := http.Get(env.server.URL + path)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}

// seedData populates the store with deterministic test data covering multiple users,
// projects, agents, sessions with messages and tool calls.
func seedData(t *testing.T, s *store.ClickHouseStore) {
	t.Helper()
	ctx := context.Background()
	orgID := ""

	sessions := []struct {
		session  store.Session
		messages []store.Message
		tools    []store.ToolCall
	}{
		{
			session: store.Session{
				ID: "sess-alpha", UserID: "alice@dev.io", UserName: "Alice",
				ProjectID: "proj-frontend", ProjectName: "frontend", ProjectPath: "/home/alice/frontend",
				Machine: "laptop", AgentType: "claude-code",
				FirstMessage: "Build the login page",
				StartedAt:    ptr(time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)),
				EndedAt:      ptr(time.Date(2026, 2, 25, 11, 0, 0, 0, time.UTC)),
				MessageCount: 3, UserMessageCount: 2,
				SourceCreatedAt: "2026-02-25T10:00:00Z",
			},
			messages: []store.Message{
				{OrgID: "", SessionID: "sess-alpha", Ordinal: 0, Role: "user", Content: "Build the login page", Timestamp: ptr(time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)), ContentLength: 20},
				{OrgID: "", SessionID: "sess-alpha", Ordinal: 1, Role: "assistant", Content: "I'll create the login component.", Timestamp: ptr(time.Date(2026, 2, 25, 10, 0, 30, 0, time.UTC)), HasToolUse: true, ContentLength: 31},
				{OrgID: "", SessionID: "sess-alpha", Ordinal: 2, Role: "user", Content: "Looks good, thanks", Timestamp: ptr(time.Date(2026, 2, 25, 10, 1, 0, 0, time.UTC)), ContentLength: 18},
			},
			tools: []store.ToolCall{
				{OrgID: "", MessageOrdinal: 1, SessionID: "sess-alpha", ToolName: "Write", Category: "file", ToolUseID: "tu-a1", InputJSON: `{"path":"login.tsx"}`, ResultContentLength: ptr(200)},
				{OrgID: "", MessageOrdinal: 1, SessionID: "sess-alpha", ToolName: "Bash", Category: "command", ToolUseID: "tu-a2", InputJSON: `{"command":"npm test"}`, ResultContentLength: ptr(50)},
			},
		},
		{
			session: store.Session{
				ID: "sess-beta", UserID: "bob@dev.io", UserName: "Bob",
				ProjectID: "proj-backend", ProjectName: "backend", ProjectPath: "/home/bob/backend",
				Machine: "desktop", AgentType: "cursor",
				FirstMessage: "Fix the API endpoint",
				StartedAt:    ptr(time.Date(2026, 2, 26, 14, 0, 0, 0, time.UTC)),
				EndedAt:      ptr(time.Date(2026, 2, 26, 15, 30, 0, 0, time.UTC)),
				MessageCount: 2, UserMessageCount: 1,
				SourceCreatedAt: "2026-02-26T14:00:00Z",
			},
			messages: []store.Message{
				{OrgID: "", SessionID: "sess-beta", Ordinal: 0, Role: "user", Content: "Fix the API endpoint", Timestamp: ptr(time.Date(2026, 2, 26, 14, 0, 0, 0, time.UTC)), ContentLength: 20},
				{OrgID: "", SessionID: "sess-beta", Ordinal: 1, Role: "assistant", Content: "Fixed the handler.", Timestamp: ptr(time.Date(2026, 2, 26, 14, 5, 0, 0, time.UTC)), HasToolUse: true, ContentLength: 18},
			},
			tools: []store.ToolCall{
				{OrgID: "", MessageOrdinal: 1, SessionID: "sess-beta", ToolName: "Edit", Category: "file", ToolUseID: "tu-b1", InputJSON: `{"path":"handler.go"}`, ResultContentLength: ptr(80)},
			},
		},
		{
			session: store.Session{
				ID: "sess-gamma", UserID: "alice@dev.io", UserName: "Alice",
				ProjectID: "proj-frontend", ProjectName: "frontend", ProjectPath: "/home/alice/frontend",
				Machine: "laptop", AgentType: "claude-code",
				FirstMessage: "Add dark mode support",
				StartedAt:    ptr(time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)),
				EndedAt:      ptr(time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)),
				MessageCount: 2, UserMessageCount: 1,
				SourceCreatedAt: "2026-03-01T09:00:00Z",
			},
			messages: []store.Message{
				{OrgID: "", SessionID: "sess-gamma", Ordinal: 0, Role: "user", Content: "Add dark mode support", Timestamp: ptr(time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)), ContentLength: 21},
				{OrgID: "", SessionID: "sess-gamma", Ordinal: 1, Role: "assistant", Content: "Added dark mode.", HasThinking: true, Timestamp: ptr(time.Date(2026, 3, 1, 9, 5, 0, 0, time.UTC)), ContentLength: 16},
			},
			tools: nil,
		},
		{
			session: store.Session{
				ID: "sess-delta", UserID: "carol@dev.io", UserName: "Carol",
				ProjectID: "proj-infra", ProjectName: "infra", ProjectPath: "/home/carol/infra",
				Machine: "server", AgentType: "copilot",
				FirstMessage: "Set up CI pipeline",
				StartedAt:    ptr(time.Date(2026, 3, 2, 16, 0, 0, 0, time.UTC)),
				EndedAt:      ptr(time.Date(2026, 3, 2, 17, 0, 0, 0, time.UTC)),
				MessageCount: 1, UserMessageCount: 1,
				SourceCreatedAt: "2026-03-02T16:00:00Z",
			},
			messages: []store.Message{
				{OrgID: "", SessionID: "sess-delta", Ordinal: 0, Role: "user", Content: "Set up CI pipeline", Timestamp: ptr(time.Date(2026, 3, 2, 16, 0, 0, 0, time.UTC)), ContentLength: 18},
			},
			tools: nil,
		},
		{
			session: store.Session{
				ID: "sess-epsilon", UserID: "bob@dev.io", UserName: "Bob",
				ProjectID: "proj-backend", ProjectName: "backend", ProjectPath: "/home/bob/backend",
				Machine: "desktop", AgentType: "claude-code",
				FirstMessage: "Optimize database queries",
				StartedAt:    ptr(time.Date(2026, 3, 3, 8, 0, 0, 0, time.UTC)),
				EndedAt:      ptr(time.Date(2026, 3, 3, 9, 0, 0, 0, time.UTC)),
				MessageCount: 2, UserMessageCount: 1,
				SourceCreatedAt: "2026-03-03T08:00:00Z",
			},
			messages: []store.Message{
				{OrgID: "", SessionID: "sess-epsilon", Ordinal: 0, Role: "user", Content: "Optimize database queries", Timestamp: ptr(time.Date(2026, 3, 3, 8, 0, 0, 0, time.UTC)), ContentLength: 25},
				{OrgID: "", SessionID: "sess-epsilon", Ordinal: 1, Role: "assistant", Content: "Optimized the slow queries.", Timestamp: ptr(time.Date(2026, 3, 3, 8, 10, 0, 0, time.UTC)), HasToolUse: true, ContentLength: 27},
			},
			tools: []store.ToolCall{
				{OrgID: "", MessageOrdinal: 1, SessionID: "sess-epsilon", ToolName: "Read", Category: "file", ToolUseID: "tu-e1", InputJSON: `{"path":"query.sql"}`, ResultContentLength: ptr(300)},
			},
		},
	}

	for _, sess := range sessions {
		require.NoError(t, s.WriteSession(ctx, orgID, sess.session, sess.messages, sess.tools))
	}

	// Ghost session (0 messages) — should be filtered out
	require.NoError(t, s.WriteSession(ctx, orgID, store.Session{
		ID: "sess-ghost", UserID: "alice@dev.io", UserName: "Alice",
		ProjectID: "proj-frontend", ProjectName: "frontend", ProjectPath: "/home/alice/frontend",
		Machine: "laptop", AgentType: "claude-code",
		StartedAt:    ptr(time.Date(2026, 3, 1, 7, 0, 0, 0, time.UTC)),
		MessageCount: 0, UserMessageCount: 0,
		SourceCreatedAt: "2026-03-01T07:00:00Z",
	}, nil, nil))

	// Subagent session — should be filtered out
	require.NoError(t, s.WriteSession(ctx, orgID, store.Session{
		ID: "sess-subagent", UserID: "alice@dev.io", UserName: "Alice",
		ParentSessionID: "sess-alpha", RelationshipType: "subagent",
		ProjectID: "proj-frontend", ProjectName: "frontend", ProjectPath: "/home/alice/frontend",
		Machine: "laptop", AgentType: "claude-code",
		FirstMessage:    "Subagent task",
		StartedAt:       ptr(time.Date(2026, 3, 1, 10, 30, 0, 0, time.UTC)),
		MessageCount:    2, UserMessageCount: 1,
		SourceCreatedAt: "2026-03-01T10:30:00Z",
	}, []store.Message{
		{OrgID: "", SessionID: "sess-subagent", Ordinal: 0, Role: "user", Content: "Subagent task", Timestamp: ptr(time.Date(2026, 3, 1, 10, 30, 0, 0, time.UTC)), ContentLength: 13},
		{OrgID: "", SessionID: "sess-subagent", Ordinal: 1, Role: "assistant", Content: "Done.", Timestamp: ptr(time.Date(2026, 3, 1, 10, 31, 0, 0, time.UTC)), ContentLength: 5},
	}, nil))
}

// seedGitLinkData populates the store with git link test data.
func seedGitLinkData(t *testing.T, s *store.ClickHouseStore) {
	t.Helper()
	ctx := context.Background()
	orgID := ""

	// Git links for sess-alpha
	require.NoError(t, s.WriteGitLinks(ctx, orgID, []store.GitLink{
		{SessionID: "sess-alpha", UserID: "alice@dev.io", MessageOrdinal: 1, CommitSHA: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4a1b2c3d4", LinkType: "commit", Confidence: "high"},
		{SessionID: "sess-alpha", UserID: "alice@dev.io", MessageOrdinal: 1, PRURL: "https://github.com/alice/frontend/pull/10", LinkType: "pr", Confidence: "high"},
	}))

	// sess-gitcommit (Bob, backend, commit)
	require.NoError(t, s.WriteSession(ctx, orgID, store.Session{
		ID: "sess-gitcommit", UserID: "bob@dev.io", UserName: "Bob",
		ProjectID: "proj-backend", ProjectName: "backend", ProjectPath: "/home/bob/backend",
		Machine: "desktop", AgentType: "claude-code",
		FirstMessage: "Fix API endpoint",
		StartedAt:    ptr(time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)),
		EndedAt:      ptr(time.Date(2026, 3, 3, 13, 0, 0, 0, time.UTC)),
		MessageCount: 2, UserMessageCount: 1,
		SourceCreatedAt: "2026-03-03T12:00:00Z",
	}, []store.Message{
		{OrgID: "", SessionID: "sess-gitcommit", Ordinal: 0, Role: "user", Content: "Fix API endpoint", Timestamp: ptr(time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)), ContentLength: 16},
		{OrgID: "", SessionID: "sess-gitcommit", Ordinal: 1, Role: "assistant", Content: "[main f7e8d9c] Fix API endpoint\n 2 files changed", Timestamp: ptr(time.Date(2026, 3, 3, 12, 5, 0, 0, time.UTC)), HasToolUse: true, ContentLength: 50},
	}, []store.ToolCall{
		{OrgID: "", MessageOrdinal: 1, SessionID: "sess-gitcommit", ToolName: "Bash", Category: "command", ToolUseID: "tu-gc1", InputJSON: `{"command":"git add . && git commit -m 'Fix API endpoint'"}`, ResultContentLength: ptr(60)},
	}))
	require.NoError(t, s.WriteGitLinks(ctx, orgID, []store.GitLink{
		{SessionID: "sess-gitcommit", UserID: "bob@dev.io", MessageOrdinal: 1, CommitSHA: "f7e8d9c0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6", LinkType: "commit", Confidence: "high"},
	}))

	// sess-pronly (Carol, infra, PR only)
	require.NoError(t, s.WriteSession(ctx, orgID, store.Session{
		ID: "sess-pronly", UserID: "carol@dev.io", UserName: "Carol",
		ProjectID: "proj-infra", ProjectName: "infra", ProjectPath: "/home/carol/infra",
		Machine: "server", AgentType: "claude-code",
		FirstMessage: "Deploy infra changes",
		StartedAt:    ptr(time.Date(2026, 3, 3, 14, 0, 0, 0, time.UTC)),
		EndedAt:      ptr(time.Date(2026, 3, 3, 15, 0, 0, 0, time.UTC)),
		MessageCount: 2, UserMessageCount: 1,
		SourceCreatedAt: "2026-03-03T14:00:00Z",
	}, []store.Message{
		{OrgID: "", SessionID: "sess-pronly", Ordinal: 0, Role: "user", Content: "Deploy infra changes", Timestamp: ptr(time.Date(2026, 3, 3, 14, 0, 0, 0, time.UTC)), ContentLength: 20},
		{OrgID: "", SessionID: "sess-pronly", Ordinal: 1, Role: "assistant", Content: "https://github.com/carol/infra/pull/99", Timestamp: ptr(time.Date(2026, 3, 3, 14, 5, 0, 0, time.UTC)), HasToolUse: true, ContentLength: 38},
	}, []store.ToolCall{
		{OrgID: "", MessageOrdinal: 1, SessionID: "sess-pronly", ToolName: "Bash", Category: "command", ToolUseID: "tu-pr1", InputJSON: `{"command":"echo done"}`, ResultContentLength: ptr(5)},
	}))
	require.NoError(t, s.WriteGitLinks(ctx, orgID, []store.GitLink{
		{SessionID: "sess-pronly", UserID: "carol@dev.io", MessageOrdinal: 1, PRURL: "https://github.com/carol/infra/pull/99", LinkType: "pr", Confidence: "medium"},
	}))

	// sess-multicommit (Alice, frontend, 3 commits + 1 PR)
	require.NoError(t, s.WriteSession(ctx, orgID, store.Session{
		ID: "sess-multicommit", UserID: "alice@dev.io", UserName: "Alice",
		ProjectID: "proj-frontend", ProjectName: "frontend", ProjectPath: "/home/alice/frontend",
		Machine: "laptop", AgentType: "claude-code",
		FirstMessage: "Implement feature flags",
		StartedAt:    ptr(time.Date(2026, 3, 3, 16, 0, 0, 0, time.UTC)),
		EndedAt:      ptr(time.Date(2026, 3, 3, 18, 0, 0, 0, time.UTC)),
		MessageCount: 4, UserMessageCount: 1,
		SourceCreatedAt: "2026-03-03T16:00:00Z",
	}, []store.Message{
		{OrgID: "", SessionID: "sess-multicommit", Ordinal: 0, Role: "user", Content: "Implement feature flags", Timestamp: ptr(time.Date(2026, 3, 3, 16, 0, 0, 0, time.UTC)), ContentLength: 23},
		{OrgID: "", SessionID: "sess-multicommit", Ordinal: 1, Role: "assistant", Content: "[main 1111111] Add feature flag config", Timestamp: ptr(time.Date(2026, 3, 3, 16, 10, 0, 0, time.UTC)), HasToolUse: true, ContentLength: 39},
		{OrgID: "", SessionID: "sess-multicommit", Ordinal: 2, Role: "assistant", Content: "[main 2222222] Add feature flag UI", Timestamp: ptr(time.Date(2026, 3, 3, 16, 20, 0, 0, time.UTC)), HasToolUse: true, ContentLength: 34},
		{OrgID: "", SessionID: "sess-multicommit", Ordinal: 3, Role: "assistant", Content: "[main 3333333] Add tests for flags\nhttps://github.com/alice/frontend/pull/42", Timestamp: ptr(time.Date(2026, 3, 3, 16, 30, 0, 0, time.UTC)), HasToolUse: true, ContentLength: 75},
	}, []store.ToolCall{
		{OrgID: "", MessageOrdinal: 1, SessionID: "sess-multicommit", ToolName: "Bash", Category: "command", ToolUseID: "tu-mc1", InputJSON: `{"command":"git commit -m 'Add feature flag config'"}`, ResultContentLength: ptr(40)},
		{OrgID: "", MessageOrdinal: 2, SessionID: "sess-multicommit", ToolName: "Bash", Category: "command", ToolUseID: "tu-mc2", InputJSON: `{"command":"git commit -m 'Add feature flag UI'"}`, ResultContentLength: ptr(40)},
		{OrgID: "", MessageOrdinal: 3, SessionID: "sess-multicommit", ToolName: "Bash", Category: "command", ToolUseID: "tu-mc3", InputJSON: `{"command":"git commit -m 'Add tests' && gh pr create"}`, ResultContentLength: ptr(80)},
	}))
	require.NoError(t, s.WriteGitLinks(ctx, orgID, []store.GitLink{
		{SessionID: "sess-multicommit", UserID: "alice@dev.io", MessageOrdinal: 1, CommitSHA: "1111111aaaaabbbbccccddddeeee1111111aaaaa", LinkType: "commit", Confidence: "high"},
		{SessionID: "sess-multicommit", UserID: "alice@dev.io", MessageOrdinal: 2, CommitSHA: "2222222aaaaabbbbccccddddeeee2222222aaaaa", LinkType: "commit", Confidence: "high"},
		{SessionID: "sess-multicommit", UserID: "alice@dev.io", MessageOrdinal: 3, CommitSHA: "3333333aaaaabbbbccccddddeeee3333333aaaaa", LinkType: "commit", Confidence: "high"},
		{SessionID: "sess-multicommit", UserID: "alice@dev.io", MessageOrdinal: 3, PRURL: "https://github.com/alice/frontend/pull/42", LinkType: "pr", Confidence: "high"},
	}))
}
