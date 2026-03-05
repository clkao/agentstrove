# Agentstrove v1 Port — Design Spec

**Date:** 2026-03-05
**Status:** Complete — all 21 tasks implemented and tested

## Goal

Port the v0 prototype to v1 with ClickHouse storage, shipping as Phase 1 OSS release: sync daemon, conversation browser, full-text search, git linking, secret masking. Self-hosted, no auth.

## Key Design Decisions

### 1. Store Interfaces: Separate Read and Write

`Store` (write) and `ReadStore` (read) are independent interfaces. The sync engine receives `Store`, the API server receives `ReadStore`. The ClickHouse implementation satisfies both. This gives compile-time enforcement of read-only vs read-write access.

### 2. Identity Resolution: Git-Derived

At sync time, identity is resolved from git config (with config file override):
- `user_id` = git email, `user_name` = git user.name
- `project_id` = git remote URL (empty if unavailable)
- `project_name` = last path component of remote URL, or directory basename
- `project_path` = raw absolute path from agentsview

### 3. Sync Strategy: Incremental Message Append

When a session's `file_hash` changes (detected via watermark comparison):
- Session row is always re-inserted (metadata changes)
- Only messages with `ordinal > lastOrdinal` are inserted
- Only tool calls for those new messages are inserted
- Git links are re-extracted from new tool calls

Watermark tracks `{fileHash, lastOrdinal}` per session, plus a sync version for forced resync on schema changes.

### 4. Sync Version for Forward Compatibility

`const SyncVersion = 1` hardcoded in the binary. If `watermark.Version < SyncVersion`, all session hashes are reset → full resync. Bump when reader or transformation logic changes (new agentsview fields, new secret patterns). `sync --force` flag for manual override.

### 5. Search in ClickHouse (No Separate Index)

v0's separate `search.duckdb` FTS index is eliminated. ClickHouse's built-in FTS handles search. The `Search` method lives on `ReadStore` — no separate search package.

### 6. Schema: v0 Fields + v1 Identity Model

All v0 fields carried forward (first_message, parent_session_id, user_message_count, etc.) plus v1 identity changes (user_id/user_name, project_id/project_name/project_path, org_id). `model` and `cost_usd` deferred until agentsview provides them.

Full schema in `docs/roadmap.md` and `scripts/clickhouse-schema.sql`.

## Package Structure

```
cmd/agentstrove/          CLI (sync, serve, daemon)
internal/
  config/                 Config + git-based identity resolution
  reader/                 agentsview SQLite reader (port verbatim)
  secrets/                Secret masking (port verbatim)
  store/
    store.go              Domain types + Store/ReadStore interfaces
    clickhouse.go         ClickHouse impl (both interfaces, includes FTS)
  sync/
    engine.go             Read → mask → write (incremental append)
    watermark.go          Per-session {fileHash, lastOrdinal} + sync version
    daemon.go             fsnotify + reconcile loop
  gitlinks/               Git link extraction (port verbatim)
  api/                    HTTP server + REST handlers
  web/embed.go            Embedded frontend dist
frontend/                 Svelte 5 SPA
scripts/clickhouse-schema.sql
e2e/                      E2E tests (seeded + dogfood)
```

## Build Order (10 Phases)

All phases target Phase 1 OSS launch. Each is tested before the next.

| # | Phase | Port strategy | Depends on |
|---|-------|--------------|------------|
| 1 | Domain types + interfaces | Adapt from v0 `lake/store.go` | — |
| 2 | ClickHouse store + schema | Rewrite queries for CH dialect | Phase 1 |
| 3 | Reader | Verbatim from v0 | — |
| 4 | Secrets | Verbatim from v0 | — |
| 5 | Sync engine | Adapt for incremental append | Phases 1–4 |
| 6 | GitLinks | Verbatim from v0 (type renames) | Phase 1 |
| 7 | Config + CLI | Adapt (drop DuckLake, add CH) | Phases 1–6 |
| 8 | API server | Port handlers, rename fields | Phases 1–2 |
| 9 | Frontend | Port, update API types | Phase 8 |
| 10 | E2E tests | Adapt from v0 E2E plans | Phases 1–9 |

Detailed phase descriptions in `docs/roadmap.md`.

## Testing

- **ClickHouse from devcontainer**: `host.docker.internal:$port`, `CLICKHOUSE_ADDR` env var
- **Test isolation**: each test creates a unique temp database in ClickHouse
- **No mocks for storage**: real ClickHouse for store tests, real SQLite for reader tests
- **E2E test plan**: 70+ seeded API tests, 24 dogfood golden paths, Playwright smoke tests

Full test plan in `docs/testing.md`.

## API Endpoints

```
GET /api/v1/sessions                 (user_id, project_id, agent_type, date_from, date_to, cursor, limit)
GET /api/v1/sessions/{id}
GET /api/v1/sessions/{id}/messages   (returns MessageWithToolCalls[])
GET /api/v1/users
GET /api/v1/projects
GET /api/v1/agents
GET /api/v1/search                   (q, plus filters; auto-routes SHA/PR to gitlinks)
GET /api/v1/gitlinks                 (sha, pr)
/                                    SPA fallback
```

## What's NOT Included

- Auth / multi-tenant access control (separate project)
- Analytics dashboard (post-launch)
- `model` / `cost_usd` fields (deferred until agentsview provides them)

## Reference Documents

- `CLAUDE.md` — project context, conventions, test prerequisites
- `docs/roadmap.md` — schema, 10-phase build plan with details
- `docs/testing.md` — E2E test plan, dogfood golden paths, v0 bug lessons
- `../agentstrove/` — v0 prototype (source of truth for porting)
