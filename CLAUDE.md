# Agentstrove — Project Context

## What This Is

Team visibility layer for AI agent conversations. Syncs local agent sessions (collected by agentsview) to shared ClickHouse storage. Provides browsing, full-text search, and git commit/PR-to-conversation linking via a web UI.

This is a structured port from the v0 prototype at `../agentstrove`. Proven code is carried forward; storage backend is replaced (DuckLake → ClickHouse).

## Tech Stack

- **Backend:** Go 1.25+, stdlib `net/http` (no framework)
- **Frontend:** Svelte 5, Vite, TypeScript
- **Storage:** ClickHouse (ReplacingMergeTree for dedup, built-in FTS)
- **Local data source:** agentsview SQLite (read-only)
- **Testing:** `go test` + testify (backend), vitest + @testing-library/svelte (frontend)

## Project Structure

```
cmd/agentstrove/        CLI entry point (sync, serve, daemon subcommands)
internal/
  config/               Config loading, git-based identity resolution
  reader/               agentsview SQLite reader (read-only)
  secrets/              Secret detection + masking (regex-based)
  store/
    store.go            Domain types + Store/ReadStore interfaces
    clickhouse.go       ClickHouse implementation (satisfies both interfaces, includes FTS)
  sync/
    engine.go           Read → mask → write pipeline (incremental message append)
    watermark.go        Per-session {fileHash, lastOrdinal} tracking
    daemon.go           fsnotify watcher + reconcile loop
  gitlinks/             Git commit/PR extraction from tool calls
  api/                  HTTP server + REST handlers
  web/
    embed.go            //go:embed frontend dist
frontend/               Svelte 5 SPA
scripts/
  clickhouse-schema.sql Canonical ClickHouse DDL
e2e/                    E2E tests (seeded + dogfood)
docs/                   Project documentation
```

## Key Interfaces

`Store` and `ReadStore` in `internal/store/store.go` are **separate** interfaces (ReadStore does NOT embed Store). The sync engine receives `Store` (write-only), the API server receives `ReadStore` (read-only). The ClickHouse implementation struct satisfies both.

Every store method takes `orgID` as first parameter. In self-hosted mode this is always `""`. This makes the data model ready for multi-tenant use without schema changes.

### Identity Resolution

At sync time, the daemon resolves identity from git config (with config file override):

- `user_id` = `git config user.email`
- `user_name` = `git config user.name`
- `project_path` = absolute path from agentsview
- `project_id` = `git remote get-url origin` for that path (empty if not a git repo or no remote)
- `project_name` = last path component of remote URL without `.git`, falling back to directory basename

### Sync Strategy

Incremental message append — when a session's `file_hash` (from agentsview) changes:

1. Re-insert session row with `_version = unix_millis(now)` (metadata always refreshed)
2. Insert only messages where `ordinal > lastOrdinal` (from watermark)
3. Insert only tool calls for those new messages
4. Re-extract and insert git links from new tool calls
5. Update watermark: `{fileHash, lastOrdinal = max(ordinal)}`

Watermark persisted as JSON at `$DATA_DIR/sync-state.json` with per-session `{fileHash, lastOrdinal}`.

## Data Model Principles

- `org_id` on every table, in PARTITION BY and ORDER BY — future-proofs for multi-tenant
- `user_id` + `user_name` instead of raw email/name — stable identity, reconcilable later
- `project_id` + `project_name` + `project_path` — project is a reconcilable entity, not just a filesystem path. Daemon resolves git remote → project_id at sync time when possible. Raw path always stored for after-the-fact reconciliation.
- ClickHouse ReplacingMergeTree handles dedup via insert-only writes with `_version`

## Reference: v0 Prototype

The v0 codebase at `../agentstrove` has working implementations of everything except analytics. When implementing a feature, check the v0 code first:

- `../agentstrove/internal/reader/` — agentsview SQLite reader (port directly)
- `../agentstrove/internal/secrets/` — secret masking (port directly)
- `../agentstrove/internal/gitlinks/` — git link extraction (port directly)
- `../agentstrove/internal/sync/` — sync engine (simplify for ClickHouse)
- `../agentstrove/internal/api/` — REST handlers (port, adjust store calls)
- `../agentstrove/frontend/` — Svelte frontend (port, same API shape)
- `../agentstrove/internal/lake/store.go` — original Store/ReadStore interfaces (adapt)
- `../agentstrove/internal/lake/duckdb.go` — DuckLake impl (replace with ClickHouse)
- `../agentstrove/internal/search/` — separate FTS index (eliminate — ClickHouse handles FTS)

## Conventions

- All Go files start with a 2-line `// ABOUTME:` comment
- Match surrounding code style; consistency within a file trumps external standards
- Tests use real databases (ClickHouse for store tests, SQLite for reader tests) — no mocks for storage
- API tests use httptest with real store instances
- Frontend uses Svelte 5 class-based stores with `$state`/`$derived` runes as singletons
- Cursor-based pagination (base64-encoded composite key, DESC ordering)

## Build & Test

### Prerequisites

ClickHouse must be running for store and E2E tests. From the host:

```bash
docker compose up -d clickhouse    # starts ClickHouse on ports 8123 (HTTP) + 9440 (native)
```

From the devcontainer, tests connect via `host.docker.internal`. Set `CLICKHOUSE_ADDR` to override (default: `host.docker.internal:9440`).

Each test suite creates a unique temporary database for isolation — no shared state between test runs.

### Commands

```bash
make build          # Frontend + Go binary
make test           # go test ./internal/... (unit tests — reader, secrets, gitlinks need no ClickHouse)
make test-store     # go test ./internal/store/... (needs ClickHouse)
make test-e2e       # go test ./e2e/... (needs ClickHouse)
make test-all       # All tests
```

See [docs/testing.md](docs/testing.md) for the full E2E test plan, dogfood golden paths, and test infrastructure details.

## Auth Is a Separate Project

Authentication, org gating, and multi-tenant access control live in a separate repo. The interface between agentstrove and the auth layer is JWT: the auth service issues tokens, ClickHouse validates them via JWKS. Agentstrove (this repo) is the OSS self-hosted tool with no auth dependency.
