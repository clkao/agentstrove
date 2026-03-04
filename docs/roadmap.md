# Roadmap

## Approach

Agentstrove v1 is a structured port from the v0 prototype. Proven code (reader, secrets, gitlinks, API handlers, frontend) is carried forward. The storage backend is replaced: DuckLake → ClickHouse. Each ship is independently useful, testable, and deployable.

Auth and multi-tenant access control live in a separate project. The data model is future-proofed for multi-tenancy via `org_id` on every table, but agentstrove itself ships without auth.

## Data Model

### Design Principles

1. **`org_id` everywhere.** Every table has `org_id` in PARTITION BY and ORDER BY. Self-hosted uses `""`. Multi-tenant uses real org IDs. No schema migration needed to add tenancy.

2. **Users are entities, not strings.** `user_id` + `user_name` replace raw `developer_email` / `developer_name`. The daemon resolves identity at sync time from git config or explicit configuration. Auth service can map `user_id` to authenticated accounts later.

3. **Projects are reconcilable.** `project_id` + `project_name` + `project_path` on sessions. The daemon resolves `project_path` → git remote URL → `project_id` when possible. Raw `project_path` is always stored so reconciliation can happen after the fact (e.g., admin maps multiple paths to one project).

4. **Insert-only writes.** ClickHouse ReplacingMergeTree deduplicates by keeping the highest `_version`. No delete-then-insert. No application-level dedup logic needed.

### Schema Overview

```sql
CREATE TABLE sessions (
    org_id          String DEFAULT '',
    id              String,
    user_id         String,
    user_name       String,
    project_id      String,
    project_name    String,
    project_path    String,
    agent_type      String,
    model           String,
    started_at      DateTime64(3),
    updated_at      DateTime64(3),
    message_count   UInt32,
    cost_usd        Float64,
    _version        UInt64
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY org_id
ORDER BY (org_id, id);

CREATE TABLE messages (
    org_id          String DEFAULT '',
    session_id      String,
    ordinal         UInt32,
    role            String,
    content         String,
    has_thinking    Bool,
    has_tool_use    Bool,
    created_at      DateTime64(3),
    _version        UInt64
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY org_id
ORDER BY (org_id, session_id, ordinal);

CREATE TABLE tool_calls (
    org_id              String DEFAULT '',
    session_id          String,
    message_ordinal     UInt32,
    tool_use_id         String,
    tool_name           String,
    tool_category       String,
    input_json          String,
    result_content      String,
    _version            UInt64
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY org_id
ORDER BY (org_id, session_id, message_ordinal, tool_use_id);

CREATE TABLE git_links (
    org_id          String DEFAULT '',
    session_id      String,
    user_id         String,
    commit_sha      String DEFAULT '',
    pr_url          String DEFAULT '',
    confidence      String,
    detected_at     DateTime64(3),
    _version        UInt64
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY org_id
ORDER BY (org_id, session_id, commit_sha, pr_url);
```

### Reconciliation

Project and user reconciliation can happen after initial sync:

- **Project reconciliation:** Re-insert sessions with corrected `project_id` and incremented `_version`. ClickHouse merges away the old row. Can be triggered by an admin mapping local paths to canonical project IDs.
- **User reconciliation:** Same pattern. Re-insert sessions with corrected `user_id` after auth service maps git identities to authenticated users.
- **No data loss:** Raw `project_path`, `user_name` are always preserved alongside the reconciled IDs.

## Incremental Ship Plan

### Ship 1: Sync Daemon

**What ships:** Reader, secret masking, ClickHouse store, sync engine, CLI.

**What it does:** Developer runs `agentstrove sync`, local agentsview sessions flow to ClickHouse with secrets masked.

**Ported from v0:**
- `internal/reader/` — direct port
- `internal/secrets/` — direct port
- `internal/config/` — simplified (one canonical config path, add user/project resolution)
- `internal/sync/` — simplified (insert-only, no delete-then-insert, no watermark tracking, no search rebuild post-hook)

**New:**
- `internal/store/` — ClickHouse Store implementation (replaces DuckLake)
- Schema init via migration files (replaces sentinel-split DDL)
- `compose.yml` with ClickHouse

**Validates:** Core pipeline end-to-end. ClickHouse write performance. Secret masking. Idempotent re-sync.

---

### Ship 2: Browse + API Server

**What ships:** ReadStore queries, REST API, Svelte frontend, `agentstrove serve`.

**What it does:** Team opens web UI, browses all synced conversations with filtering.

**Ported from v0:**
- `internal/api/` — REST handlers (adjust store method signatures for org_id, user_id, project_id)
- `frontend/` — Svelte SPA (update API types for new field names, otherwise direct port)

**Validates:** ReadStore query performance on ClickHouse. Frontend renders correctly with new data model.

---

### Ship 3: Search

**What ships:** Full-text search via ClickHouse built-in FTS.

**What it does:** User searches across all team conversations. Results show context snippets with highlighted matches.

**Different from v0:** No separate search.duckdb. No index rebuild pipeline. No `internal/search/` package. ClickHouse FTS queries go through the same Store interface.

**Ported from v0:**
- API search endpoint shape (same request/response contract)
- Frontend search components (SearchBar, SearchResults, snippet highlighting)

**Validates:** ClickHouse FTS query performance and relevance ranking.

---

### Ship 4: Git Linking

**What ships:** Git commit/PR extraction, storage, lookup API, frontend lookup UI.

**Ported from v0:**
- `internal/gitlinks/` — direct port (pure regex extraction, storage-agnostic)
- API gitlinks endpoint
- Frontend GitLinkLookup component

**Validates:** End-to-end commit → conversation navigation.

---

### Ship 5: Analytics

**What ships:** Team usage dashboard with activity metrics.

**New:** ClickHouse aggregate queries (native OLAP). Dashboard UI with charts and heatmap.

**Not in v0:** This is the one feature that doesn't exist in the prototype.

**Validates:** ClickHouse materialized views for analytics. Dashboard usability.

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
