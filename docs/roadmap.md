# Roadmap

## Approach

Agentstrove v1 is a structured port from the v0 prototype. Proven code (reader, secrets, gitlinks, API handlers, frontend) is carried forward. The storage backend is replaced: DuckLake → ClickHouse. Each phase is testable independently, but all phases 1–9 target the Phase 1 OSS launch.

Auth and multi-tenant access control live in a separate project. The data model is future-proofed for multi-tenancy via `org_id` on every table, but agentstrove itself ships without auth.

## Data Model

### Design Principles

1. **`org_id` everywhere.** Every table has `org_id` in PARTITION BY and ORDER BY. Self-hosted uses `""`. Multi-tenant uses real org IDs. No schema migration needed to add tenancy.

2. **Users are entities, not strings.** `user_id` + `user_name` replace raw `developer_email` / `developer_name`. The daemon resolves identity at sync time from git config or explicit configuration. Auth service can map `user_id` to authenticated accounts later.

3. **Projects are reconcilable.** `project_id` + `project_name` + `project_path` on sessions. The daemon resolves `project_path` → git remote URL → `project_id` when possible. Raw `project_path` is always stored so reconciliation can happen after the fact (e.g., admin maps multiple paths to one project).

4. **Insert-only writes.** ClickHouse ReplacingMergeTree deduplicates by keeping the highest `_version`. No delete-then-insert. No application-level dedup logic needed.

### Schema Overview

Canonical DDL lives in `scripts/clickhouse-schema.sql`. Summary:

```sql
CREATE TABLE sessions (
    org_id              String DEFAULT '',
    id                  String,
    user_id             String DEFAULT '',
    user_name           String DEFAULT '',
    project_id          String DEFAULT '',
    project_name        String DEFAULT '',
    project_path        String DEFAULT '',
    agent_type          String DEFAULT '',
    first_message       String DEFAULT '',
    started_at          Nullable(DateTime64(3)),
    ended_at            Nullable(DateTime64(3)),
    message_count       UInt32 DEFAULT 0,
    user_message_count  UInt32 DEFAULT 0,
    parent_session_id   String DEFAULT '',
    relationship_type   String DEFAULT '',
    machine             String DEFAULT '',
    source_created_at   String DEFAULT '',
    _version            UInt64 DEFAULT toUnixTimestamp64Milli(now64(3))
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY org_id
ORDER BY (org_id, id);

CREATE TABLE messages (
    org_id          String DEFAULT '',
    session_id      String,
    ordinal         UInt32,
    role            String,
    content         String DEFAULT '',
    timestamp       Nullable(DateTime64(3)),
    has_thinking    Bool DEFAULT false,
    has_tool_use    Bool DEFAULT false,
    content_length  UInt32 DEFAULT 0,
    _version        UInt64 DEFAULT toUnixTimestamp64Milli(now64(3))
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY org_id
ORDER BY (org_id, session_id, ordinal);

CREATE TABLE tool_calls (
    org_id              String DEFAULT '',
    session_id          String,
    message_ordinal     UInt32,
    tool_use_id         String DEFAULT '',
    tool_name           String DEFAULT '',
    tool_category       String DEFAULT '',
    input_json          String DEFAULT '',
    skill_name          String DEFAULT '',
    result_content      String DEFAULT '',
    result_content_length Nullable(UInt32),
    subagent_session_id String DEFAULT '',
    _version            UInt64 DEFAULT toUnixTimestamp64Milli(now64(3))
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY org_id
ORDER BY (org_id, session_id, message_ordinal, tool_use_id);

CREATE TABLE git_links (
    org_id              String DEFAULT '',
    session_id          String,
    user_id             String DEFAULT '',
    message_ordinal     UInt32 DEFAULT 0,
    commit_sha          String DEFAULT '',
    pr_url              String DEFAULT '',
    link_type           String DEFAULT '',
    confidence          String DEFAULT '',
    detected_at         DateTime64(3) DEFAULT now64(3),
    _version            UInt64 DEFAULT toUnixTimestamp64Milli(now64(3))
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY org_id
ORDER BY (org_id, session_id, commit_sha, pr_url);
```

### Field Mapping: v0 → v1

| v0 field | v1 field | Notes |
|----------|----------|-------|
| `developer_email` | `user_id` | Resolved from git config at sync time |
| `developer_name` | `user_name` | Resolved from git config at sync time |
| `project` | `project_path` | Raw filesystem path from agentsview |
| — | `project_id` | Resolved from `git remote get-url origin` (empty if unavailable) |
| — | `project_name` | Last component of remote URL sans `.git`, or directory basename |
| `agent` | `agent_type` | Renamed for clarity |

Fields carried forward unchanged: `id`, `first_message`, `started_at`, `ended_at`, `message_count`, `user_message_count`, `parent_session_id`, `relationship_type`, `machine`, `source_created_at`, `ordinal`, `role`, `content`, `has_thinking`, `has_tool_use`, `content_length`, `tool_name`, `tool_use_id`, `input_json`, `skill_name`, `result_content`, `result_content_length`, `subagent_session_id`, `commit_sha`, `pr_url`, `link_type`, `confidence`.

### Reconciliation

Project and user reconciliation can happen after initial sync:

- **Project reconciliation:** Re-insert sessions with corrected `project_id` and incremented `_version`. ClickHouse merges away the old row. Can be triggered by an admin mapping local paths to canonical project IDs.
- **User reconciliation:** Same pattern. Re-insert sessions with corrected `user_id` after auth service maps git identities to authenticated users.
- **No data loss:** Raw `project_path`, `user_name` are always preserved alongside the reconciled IDs.

## Build Plan

All phases target the Phase 1 OSS launch. Each phase is built and tested before the next. Analytics (post-launch) is not included.

### Phase 1: Domain Types + Interfaces

**What:** Define all Go domain types and the `Store`/`ReadStore` interfaces in `internal/store/store.go`.

**Details:**
- `Session`, `Message`, `ToolCall`, `GitLink` domain structs with v1 field names
- `GitLinkResult` — joined type for API responses (session metadata + git link fields)
- `UserInfo{ID, Name}`, `ProjectInfo{ID, Name, Path}` for metadata endpoints
- `SessionFilter` with `UserID`, `ProjectID`, `AgentType`, `DateFrom`, `DateTo`, `Cursor`, `Limit`, `IncludeSubagents`
- `SessionPage{Sessions, NextCursor, Total}`
- `SearchQuery` and `SearchResult`/`SearchPage` types
- `Store` interface (write): `WriteSession`, `WriteGitLinks`, `Close`
- `ReadStore` interface (read): `ListSessions`, `GetSession`, `GetSessionMessages`, `GetSessionToolCalls`, `ListUsers`, `ListProjects`, `ListAgents`, `Search`, `LookupGitLinks`
- Store and ReadStore are **separate** interfaces (ReadStore does NOT embed Store)

**Port from v0:** `internal/lake/store.go` — adapt types and interface signatures.

**Tests:** Compile-only (interfaces have no implementation yet).

**Validates:** The interface contract that all other packages depend on.

---

### Phase 2: ClickHouse Store Implementation + Schema

**What:** `internal/store/clickhouse.go` — ClickHouse implementation satisfying both `Store` and `ReadStore` interfaces. `scripts/clickhouse-schema.sql` — canonical DDL. `compose.yml` with ClickHouse service.

**Details:**
- `ClickHouseStore` struct wrapping `clickhouse-go/v2` connection
- `NewClickHouseStore(addr, database string)` constructor
- `EnsureSchema(ctx)` — runs DDL from embedded SQL
- Write methods: batch INSERT for session + messages + tool_calls, INSERT for git_links
- Read methods: all ReadStore queries with `org_id` filtering, cursor-based pagination, `FINAL` keyword for consistent reads
- Search: ClickHouse FTS via `tokenbf_v1` or `ngramBF` index on `messages.content` — query returns `SearchResult` with snippet extraction
- Subagent/ghost filtering: `WHERE parent_session_id = '' AND user_message_count > 0` on browsable queries
- `commit_count` as a correlated subquery on `git_links`
- `compose.yml` with just ClickHouse (no postgres, no minio)

**Port from v0:** `internal/lake/duckdb.go` + `internal/lake/query.go` — rewrite queries for ClickHouse SQL dialect. Port `internal/lake/schema.go` → `scripts/clickhouse-schema.sql`.

**Tests:** Integration tests against real ClickHouse (`internal/store/clickhouse_test.go`). Each test creates a temporary database for isolation. Tests cover: write + read round-trip, pagination, all filters, search, git link lookup, empty result handling, `[]` not `null` for empty slices.

**Validates:** ClickHouse query correctness and performance. The Store/ReadStore interface contract holds with a real database.

---

### Phase 3: Reader

**What:** `internal/reader/` — reads agentsview SQLite database.

**Details:**
- `Reader` struct wrapping `go-sqlite3` connection (read-only mode)
- `ReadSessionsSince(createdAfter string) []Session` — full scan or incremental
- `ReadMessagesForSession(sessionID string) []Message` — ordered by ordinal ASC
- `ReadToolCallsForSession(sessionID string) []ToolCall` — JOINs tool_calls with messages to get ordinal
- Reader domain types are separate from store types (reader returns raw agentsview data, sync engine maps to store types)

**Port from v0:** `internal/reader/reader.go` — direct port. Field names stay as agentsview provides them (the mapping to v1 names happens in the sync engine).

**Tests:** Unit tests with a test SQLite database seeded with known data. Test all three read methods, empty results, ordering.

**Validates:** Reader correctly extracts data from agentsview schema.

---

### Phase 4: Secrets

**What:** `internal/secrets/` — regex-based secret detection and masking.

**Details:**
- `MaskSecrets(content string) MaskResult` — applies 12 regex patterns, replaces with `[REDACTED:pattern_name]`
- Returns `{Masked string, SecretCount int, Patterns []string}`
- Patterns: aws-access-key, github-pat/oauth/app/refresh, slack-token, jwt, private-key, generic-api-key, generic-secret, openai-key, anthropic-key, db-connection

**Port from v0:** `internal/secrets/secrets.go` — direct port. Pure regex, no external dependencies.

**Tests:** Unit tests for each pattern type, edge cases (partial matches, overlapping patterns, no secrets).

**Validates:** Secrets are reliably detected and masked before data leaves the developer's machine.

---

### Phase 5: Sync Engine

**What:** `internal/sync/engine.go` + `internal/sync/watermark.go` — the read → mask → write pipeline with incremental message append.

**Details:**
- `Engine` struct with `reader *reader.Reader`, `store store.Store`, `config *config.Config`
- `RunOnce(ctx)` flow:
  1. `reader.ReadSessionsSince("")` — get all sessions from agentsview
  2. For each session: compare `file_hash` against watermark
  3. If hash changed: read messages where `ordinal > lastOrdinal`
  4. Read tool calls for those new messages
  5. `secrets.MaskSecrets()` on new message content, tool call input_json, result_content
  6. `sanitizeUTF8()` on all text fields (handles truncated multi-byte chars from agentsview)
  7. Map reader types → store types (applying identity resolution from config)
  8. `store.WriteSession()` — session row with current `_version`, only new messages and tool calls
  9. `gitlinks.ExtractGitLinks()` from new tool calls + `store.WriteGitLinks()`
  10. Update watermark: `{fileHash, lastOrdinal = max(ordinal)}`
- `SyncState` watermark: `map[sessionID]{FileHash, LastOrdinal}`, persisted as JSON
- Identity mapping: reader's raw fields → v1 user_id/user_name/project_id/project_name/project_path via config

**Port from v0:** `internal/sync/engine.go` + `internal/sync/watermark.go` — adapt for incremental append (v0 does delete-then-insert and full message re-write). Remove search rebuild post-hook.

**Tests:** Unit tests with mock reader + mock store. Integration test: reader → real ClickHouse store round-trip. Test incremental sync (two RunOnce calls, second only syncs new messages). Test secret masking integration. Test UTF-8 sanitization.

**Validates:** Full sync pipeline end-to-end. Incremental append works correctly.

---

### Phase 6: GitLinks

**What:** `internal/gitlinks/` — git commit/PR extraction from tool call results.

**Details:**
- `ExtractGitLinks(toolCalls []store.ToolCall) []store.GitLink`
- Scans Bash-category tool calls where `ResultContent != ""`
- `isGitCommitCommand(inputJSON)` — checks if command is `git commit`
- `isGHPRCreateCommand(inputJSON)` — checks if command is `gh pr create`
- Extracts commit SHA from git output via regex: `\[[\w/.:\- ]+(?:\([^)]+\)\s+)?([0-9a-f]{7,40})\]`
- Extracts PR URL from output via regex: `https://github\.com/[\w.-]+/[\w.-]+/pull/\d+`
- Confidence: "high" for git commit / gh pr create, "medium" for PR URLs in other output
- Deduplicates by SHA/URL

**Port from v0:** `internal/gitlinks/extract.go` + `internal/gitlinks/patterns.go` — direct port. Only change: type references (`lake.ToolCall` → `store.ToolCall`, `lake.GitLink` → `store.GitLink`).

**Tests:** Unit tests with realistic tool call data. Test commit extraction, PR extraction, deduplication, empty input, non-Bash tool calls ignored.

**Validates:** Git links correctly extracted from real-world tool call patterns.

---

### Phase 7: Config + CLI

**What:** `internal/config/` + `cmd/agentstrove/main.go` — configuration loading, identity resolution, CLI entry point.

**Details:**
- `Config` struct: ClickHouse address, agentsview DB path, data dir, server port, user overrides
- `DefaultAgentsviewDBPath()` — checks `~/.agentsview/sessions.db` then `~/.claude/agentsview/sessions.db`
- `ResolvedUserIdentity()` — config overrides first, falls back to `git config user.name/email`
- `ResolveProjectIdentity(projectPath string)` — runs `git remote get-url origin` in the project directory
- CLI subcommands: `sync` (one-shot), `daemon` (watch + periodic reconcile), `serve` (HTTP server)
- `sync/daemon.go`: fsnotify watcher on agentsview DB directory + 5-minute reconcile ticker

**Port from v0:** `internal/config/config.go` — adapt (drop DuckLake/PG/S3 fields, add ClickHouse, add user/project resolution). `cmd/agentstrove/main.go` — adapt. `internal/daemon/daemon.go` → `internal/sync/daemon.go` — port, remove search rebuild post-hook.

**Tests:** Unit tests for config loading, identity resolution. Integration test for daemon (fsnotify triggers sync).

**Validates:** CLI works end-to-end. Daemon detects changes and syncs.

---

### Phase 8: API Server

**What:** `internal/api/` — HTTP REST handlers. `internal/web/embed.go` — embedded frontend.

**Details:**
- `Server` struct with `readStore store.ReadStore` (no separate search field — search is in ReadStore)
- Middleware: CORS (permissive `*`), request logging (method, path, status, duration)
- Routes:
  - `GET /api/v1/sessions` — `handleListSessions` (params: user_id, project_id, agent_type, date_from, date_to, cursor, limit)
  - `GET /api/v1/sessions/{id}` — `handleGetSession`
  - `GET /api/v1/sessions/{id}/messages` — `handleGetMessages` (returns `[]MessageWithToolCalls`)
  - `GET /api/v1/users` — `handleListUsers`
  - `GET /api/v1/projects` — `handleListProjects`
  - `GET /api/v1/agents` — `handleListAgents`
  - `GET /api/v1/search` — `handleSearch` (auto-routes SHA/PR to git_links before FTS)
  - `GET /api/v1/gitlinks` — `handleLookupGitLinks` (params: sha, pr)
  - `/` — SPA fallback (embedded dist)
- `MessageWithToolCalls`: message fields + `ToolCalls []store.ToolCall` grouped by ordinal (always `[]`, never `null`)
- Search auto-routing: `^[0-9a-f]{7,40}$` → SHA lookup; GitHub PR URL → PR lookup. Falls through to FTS if no results.
- Date validation: `time.Parse("2006-01-02", ...)` → 400 on invalid format
- Empty slices: always `[]` not `null` in JSON

**Port from v0:** `internal/api/` — port all handlers. Changes: remove `search *search.SearchIndex` field, search goes through `readStore.Search()`. Rename filter params (`developer` → `user_id`, `project` → `project_id`, `agent` → `agent_type`). Rename response fields to match v1 types. Rename endpoints (`/developers` → `/users`, `/projects` stays).

**Tests:** httptest-based tests with real ClickHouse store. Seed data (sessions, messages, tool calls, git links), test all endpoints, pagination, filters, error handling, response shape.

**Validates:** API contract matches frontend expectations. All v0 E2E test cases pass with v1 field names.

---

### Phase 9: Frontend

**What:** Svelte 5 SPA ported from v0 with updated API types.

**Details:**
- Same component structure as v0: Sidebar (GitLinkLookup + SearchBar + FilterBar + SessionList|SearchResults), DetailPanel (session header + MessageList)
- API client: update type names (`DeveloperInfo` → `UserInfo`, `developer_email` → `user_id`, `project` → `project_id`/`project_name`/`project_path`)
- FilterBar: filter by user (dropdown from `/users`), project (from `/projects`), agent (from `/agents`), date range
- Stores: Svelte 5 `$state`/`$derived` runes, singleton pattern
- Content parsing: thinking blocks, tool call blocks, code blocks, markdown rendering
- Vite config: `/api` proxy to backend in dev mode

**Port from v0:** `frontend/` — direct port with type renames. Component structure, styling, content parsing unchanged.

**Tests:** vitest + @testing-library/svelte for component tests. No ClickHouse needed.

**Validates:** Frontend renders correctly with v1 API responses.

---

### Phase 10: E2E Tests + Integration

**What:** `e2e/` — end-to-end tests covering the full stack.

**Details:**
- Seeded API tests: 5+ sessions, multiple users/projects/agents, git links. Tests all API endpoints, pagination, filters, search, git link lookup, error handling, response shape.
- Dogfood tests: real agentsview DB → sync → API. Validates the full pipeline with real data.
- Playwright smoke tests: page loads, session list visible, click session shows messages.
- Makefile: `build`, `test`, `test-store`, `test-e2e`, `test-all`, `serve`, `clean`
- `go.mod` + `go.sum` finalized

**Port from v0:** `e2e/api_test.go`, `e2e/dogfood_test.go` — adapt for v1 field names and ClickHouse store. `e2e-browser/` → `e2e/browser/`. Seed functions rewritten for ClickHouse.

**Tests:** This phase IS the tests.

**Validates:** Everything works together. All v0 E2E test cases pass in v1 context.

See [docs/testing.md](testing.md) for the detailed E2E test plan and dogfood golden paths.

---

## Post-Launch: Analytics

**What ships:** Team usage dashboard with activity metrics.

**New:** ClickHouse aggregate queries (native OLAP). Dashboard UI with charts and heatmap.

**Not in v0.** This is the one feature that doesn't exist in the prototype.

---

## What's NOT in This Repo

| Concern | Where it lives | Interface |
|---------|---------------|-----------|
| Authentication (GitHub OAuth) | Separate auth service repo | Issues JWTs with `{user_id, org_id}` claims |
| Org membership gating | Auth service | Validates org membership before issuing JWT |
| Authenticated ingestion | Auth service + daemon config | Daemon gets JWT, ClickHouse validates via JWKS |
| Multi-tenant access control | Auth service middleware | API middleware extracts org_id from JWT |
| User directory | Auth service database | Maps GitHub identity → user_id |

The connection point is narrow: JWT issuance and validation. Agentstrove stores `org_id` and `user_id` on every record but doesn't know or care where they come from.

## Dependencies

### v0 Reference Code

The v0 prototype at `../agentstrove` is the source of truth for porting. All code that isn't storage-specific can be carried forward with minimal changes.

### External

- [agentsview](https://github.com/wesm/agentsview) — local agent session collector. Agentstrove reads its SQLite database. Schema owned by agentsview.
- ClickHouse — shared conversation storage. Self-hosted via Docker or managed (ClickHouse Cloud).
