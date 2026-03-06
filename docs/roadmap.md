# Roadmap

## Current State

Agentlore is functional for self-hosted team use: sync daemon, conversation browser, full-text search, git linking, and secret masking. No auth dependency.

## Data Model

### Design Principles

1. **`org_id` everywhere.** Every table has `org_id` in PARTITION BY and ORDER BY. Self-hosted uses `""`. Multi-tenant uses real org IDs. No schema migration needed to add tenancy.

2. **Users are entities, not strings.** `user_id` + `user_name` resolved from git config at sync time. Auth service can map `user_id` to authenticated accounts later.

3. **Projects are reconcilable.** `project_id` + `project_name` + `project_path` on sessions. The daemon resolves `project_path` → git remote URL → `project_id` when possible. Raw `project_path` is always stored so reconciliation can happen after the fact.

4. **Insert-only writes.** ClickHouse ReplacingMergeTree deduplicates by keeping the highest `_version`. No delete-then-insert. No application-level dedup logic needed.

### Reconciliation

Project and user reconciliation can happen after initial sync:

- **Project reconciliation:** Re-insert sessions with corrected `project_id` and incremented `_version`. ClickHouse merges away the old row.
- **User reconciliation:** Same pattern. Re-insert sessions with corrected `user_id` after auth service maps git identities to authenticated users.
- **No data loss:** Raw `project_path`, `user_name` are always preserved alongside the reconciled IDs.

## Future Work

### Analytics

Team usage dashboard with activity metrics. ClickHouse aggregate queries (native OLAP) with dashboard UI.

### Auth Integration

Authentication, org gating, and multi-tenant access control live in a separate project. The interface is JWT: the auth service issues tokens, ClickHouse validates via JWKS.

| Concern | Interface |
|---------|-----------|
| Authentication (GitHub OAuth) | Issues JWTs with `{user_id, org_id}` claims |
| Org membership gating | Validates org membership before issuing JWT |
| Authenticated ingestion | Daemon gets JWT, ClickHouse validates via JWKS |
| Multi-tenant access control | API middleware extracts org_id from JWT |

The connection point is narrow: JWT issuance and validation. Agentlore stores `org_id` and `user_id` on every record but doesn't know or care where they come from.

## Dependencies

- [agentsview](https://github.com/clkao/agentsview) — local agent session collector. Agentlore reads its SQLite database.
- ClickHouse — shared conversation storage. Self-hosted via Docker or managed (ClickHouse Cloud).
