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
cmd/agentstrove/        CLI entry point (sync, serve subcommands)
internal/
  config/               Config loading, identity detection
  reader/               agentsview SQLite reader (read-only)
  secrets/              Secret detection + masking (regex-based)
  store/                ClickHouse Store/ReadStore interfaces + implementation
  sync/                 Sync engine: read → mask → write pipeline
  gitlinks/             Git commit/PR extraction from tool calls
  api/                  HTTP server + REST handlers
frontend/               Svelte 5 SPA
scripts/                Build scripts, ClickHouse schema SQL
docs/                   Project documentation
```

## Key Interfaces

The `Store` and `ReadStore` interfaces in `internal/store/store.go` are the central abstraction. All storage access goes through these interfaces — sync engine writes to `Store`, API server reads from `ReadStore`.

Every store method takes `orgID` as first parameter. In self-hosted mode this is always `""`. This makes the data model ready for multi-tenant use without schema changes.

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

```bash
make build          # Frontend + Go binary
make test           # go test ./internal/...
make test-e2e       # go test ./e2e/...
make test-all       # All tests
```

## Auth Is a Separate Project

Authentication, org gating, and multi-tenant access control live in a separate repo. The interface between agentstrove and the auth layer is JWT: the auth service issues tokens, ClickHouse validates them via JWKS. Agentstrove (this repo) is the OSS self-hosted tool with no auth dependency.
