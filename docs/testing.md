# Testing

## Test Infrastructure

### ClickHouse Setup

Store integration tests and E2E tests require a running ClickHouse instance. From the host:

```bash
docker compose up -d clickhouse
```

This starts ClickHouse on:
- Port 8123 (HTTP protocol)
- Port 9440 (native protocol, remapped from 9000 to avoid conflicts)

### Connecting from the Devcontainer

Tests connect via `CLICKHOUSE_ADDR` environment variable:

```bash
export CLICKHOUSE_ADDR=host.docker.internal:9440    # native protocol
```

Default if unset: `host.docker.internal:9440`.

### Test Database Isolation

Each test suite creates a **unique temporary database** (e.g., `test_abc123`) in ClickHouse:

1. Generate random database name
2. `CREATE DATABASE test_abc123`
3. Run schema DDL against that database
4. Seed test data
5. Run tests
6. `DROP DATABASE test_abc123` in cleanup

This ensures no shared state between test runs and no pollution of the main `agentstrove` database.

### Test Layers

| Layer | What | Needs ClickHouse | Command |
|-------|------|-----------------|---------|
| Unit: reader | agentsview SQLite reader | No | `go test ./internal/reader/...` |
| Unit: secrets | Secret masking regex | No | `go test ./internal/secrets/...` |
| Unit: gitlinks | Git link extraction | No | `go test ./internal/gitlinks/...` |
| Unit: sync watermark | Watermark JSON state | No | `go test ./internal/sync/... -run Watermark` |
| Integration: store | ClickHouse read/write | Yes | `go test ./internal/store/...` |
| Integration: sync | Full sync pipeline | Yes | `go test ./internal/sync/... -run Engine` |
| E2E: seeded API | All API endpoints | Yes | `go test ./e2e/... -run "^Test[^D]"` |
| E2E: dogfood | Real agentsview data | Yes + agentsview DB | `go test ./e2e/... -run TestDogfood` |
| E2E: browser | Playwright smoke | Yes + built binary | `npx playwright test` |
| Frontend | Svelte components | No | `cd frontend && npm test` |

### Running Tests

```bash
# Unit tests only (no ClickHouse needed)
make test

# Store integration tests (needs ClickHouse)
make test-store

# All E2E tests (needs ClickHouse)
make test-e2e

# Everything
make test-all
```

---

## E2E Test Plan: Seeded API Tests

Uses seeded data: 5+ sessions, multiple users, projects, agents, git links. Runs against httptest server with real ClickHouse store.

### Sessions & Browse
| ID | Test | Assert |
|----|------|--------|
| T1 | `GET /api/v1/sessions` returns data | 200, total > 0, sessions non-empty, each has id/user_name/started_at |
| T2 | `GET /api/v1/sessions/{id}` returns valid session | 200, id matches, user_name non-empty |
| T3 | Session messages with tool calls | Messages ordered by ordinal, tool_calls array with entries |
| T4 | Pagination | limit=2, next_cursor, second page differs |
| T5 | Filter by user | `?user_id=...` → all results match |
| T6 | Filter by project | `?project_id=...` → all results match |
| T7 | Filter by agent | `?agent_type=...` → all results match |
| T8 | Date range filter | `?date_from=...` → all started_at >= date_from |
| T9 | Metadata endpoints | `/users` returns UserInfo[], `/projects` returns ProjectInfo[], `/agents` returns string[] |
| T10 | Secret masking verified | Scan messages, no content matches secret patterns |
| T11 | 404 for missing session | `GET /api/v1/sessions/nonexistent` → 404 |
| T12 | SPA fallback | `GET /` and `GET /random/path` → 200 |
| T13 | Malformed cursor → 400 | |
| T14 | Limit edge cases | 0, -1, 999, abc, 1 |
| T15 | date_to and combined range | |
| T16 | Combined filters | user_id + project_id + agent_type |
| T17 | Empty result set | 200, total=0, sessions=[] |
| T18 | Messages for nonexistent session | 200, empty array |
| T19 | Full pagination walk | Iterate all pages, collect all sessions, no duplicates |
| T20 | Total count consistent across pages | |
| T21 | Tool calls grouped under correct message | |
| T22 | Sessions ordered by started_at DESC | |
| T23 | Content-Type application/json | |
| T24 | ToolCalls always [] not null | |
| T25 | Pagination with filters applied | |
| T26 | Invalid date format → 400 | |
| T27 | Error responses have {"error": "..."} shape | |
| T28 | Empty sessions array serializes as [] | |
| T29 | Session detail all expected fields | |
| T30 | Metadata endpoints sorted, deduplicated | |
| T31 | CORS headers present | |
| T32 | has_thinking field correctness | |
| T35 | Inverted date range → empty | |
| T36 | Same-timestamp pagination via id DESC tiebreaker | |
| T37 | Metadata excludes ghost/subagent sessions | |
| T38 | Empty filter params are no-ops | |

### Search
| ID | Test | Assert |
|----|------|--------|
| S1 | Search returns results for known content | 200, results non-empty, snippet non-empty |
| S2 | Search requires q parameter | 400 |
| S3 | Empty q parameter | 400 |
| S4 | Unknown term → empty results | 200, results=[], total=0 |
| S5 | Results array never null | |
| S6 | Filter by user | All results match user_id |
| S7 | Filter by project | All results match project_id |
| S8 | Filter by agent | All results match agent_type |
| S9 | Filter by date range | |
| S10 | Combined filters | |
| S11 | Filter matching nothing → empty | |
| S12 | Snippet contains matched term | |
| S13 | Highlights point to valid snippet positions | |
| S14 | Snippet reasonable length (≤210 chars) | |
| S15 | Search matches tool call content | |
| S16 | Search matches tool call input | |
| S17 | Default limit ≤ 50 | |
| S18 | Custom limit respected | |
| S19 | Limit edge cases | |
| S20 | Content-Type application/json | |
| S21 | CORS headers | |
| S22 | Result fields all present | |
| S23 | Highlights always array, never null | |
| S25 | Search coexists with session endpoints | |
| S26 | BM25 ranking (more occurrences → higher rank) | |
| S27 | Search rebuild idempotent | |

### Git Linking
| ID | Test | Assert |
|----|------|--------|
| GL1 | Lookup by short SHA prefix | 200, result matches, link_type=commit, confidence=high |
| GL2 | Lookup by full SHA | |
| GL3 | Lookup by PR URL | |
| GL4 | No params → 400 | |
| GL5 | Nonexistent SHA → empty [] | |
| GL6 | Nonexistent PR URL → empty [] | |
| GL7 | Result includes all expected fields | |
| GL8 | Result contains correct session metadata | |
| GL9 | Medium confidence PR link | |
| GL10 | ListSessions includes commit_count | |
| GL11 | commit_count=0 for sessions without links | |
| GL12 | GetSession includes commit_count | |
| GL13 | GetSession commit_count=0 for no links | |
| GL14 | Multiple commits per session via distinct SHA lookups | |
| GL15 | Commit count matches total link count | |
| GL16 | Search auto-recognizes SHA | |
| GL17 | Search for full SHA auto-recognizes | |
| GL18 | Search for PR URL auto-recognizes | |
| GL19 | Non-matching SHA falls through to FTS | |
| GL20 | Non-hex string does NOT trigger SHA recognition | |
| GL21 | Search auto-recognition response shape matches FTS shape | |
| GL23 | Content-Type application/json | |
| GL24 | CORS headers | |
| GL25 | Empty result is [] not null | |
| GL26 | SHA prefix < 7 chars still queries (boundary) | |
| GL27 | Both sha and pr params provided | |
| GL28 | Pagination with commit_count | |
| GL29 | Filter sessions by project, commit_count still correct | |

---

## E2E Test Plan: Dogfood Tests

Runs against real agentsview database. Validates the full pipeline: reader → sync → store → API.

### Prerequisites

- Real agentsview DB at `~/.agentsview/sessions.db` or `~/.claude/agentsview/sessions.db`
- ClickHouse running
- Tests skip if agentsview DB not found

### Syncing Agentsview Data (Devcontainer)

To populate the agentsview SQLite DB and sync it to ClickHouse:

```bash
# 1. Run agentsview to sync sessions into ~/.agentsview/sessions.db
agentsview -no-browser -port 18923  # ctrl-c after initial sync completes

# 2. Create agentstrove config (one-time setup)
mkdir -p ~/.config/agentstrove/data
cat > ~/.config/agentstrove/config.json << 'EOF'
{
  "clickhouse_addr": "host.docker.internal:9440",
  "clickhouse_user": "agentstrove",
  "clickhouse_password": "agentstrove",
  "agentsview_db_path": "/home/vscode/.agentsview/sessions.db"
}
EOF

# 3. Build with CGO (arm64 devcontainer needs GOARCH override, see CLAUDE.md)
GOARCH=arm64 CGO_ENABLED=1 go build -o /tmp/agentstrove ./cmd/agentstrove

# 4. Sync to ClickHouse
/tmp/agentstrove sync
```

Re-run steps 1 and 4 to pick up new sessions after more Claude Code usage.

### Golden Paths

| # | Golden Path | How to Validate |
|---|-------------|-----------------|
| D1 | Sync ingests real sessions | Codified: asserts synced > 0 |
| D2 | Sync is idempotent | Codified: second run syncs 0, skips all |
| D3 | Sync error rate < 10% | Codified: logs errors, asserts rate |
| D4 | Session list excludes subagents | Codified: walks all pages, no parent_session_id |
| D5 | Session list ordered DESC | Codified: started_at descending |
| D6 | Pagination walks all sessions | Codified: no duplicates, matches total |
| D7 | Session detail valid | Codified |
| D8 | Messages ordered by ordinal | Codified: valid roles, tool_calls always [] |
| D9 | Tool calls grouped with messages | Codified: ordinals match |
| D10 | Secrets masked | Codified: regex scan for common patterns |
| D11 | Metadata endpoints return data | Codified: users, projects, agents |
| D12 | Project filter consistency | Codified |
| D13 | first_message populated (>50%) | Codified |
| D14 | JSON Content-Type | Codified |
| D15 | CORS headers | Codified |
| D16 | Incremental sync picks up new data | Manual |
| D17 | Sync handles DB locked gracefully | Manual |
| D18 | Serve with empty ClickHouse | Manual |
| D19 | Session with very long messages | Manual |
| D20 | UI loads session list | Playwright B1 |
| D21 | UI click shows messages | Playwright B2 |
| D22 | UI shows tool calls inline | Manual |
| D23 | UI filter dropdown works | Manual |
| D24 | Daemon fsnotify triggers re-sync | Manual |

### Dogfood Search Tests

| ID | Test | Assert |
|----|------|--------|
| DS1 | Search works on real data | Results non-empty for common terms |
| DS2 | Search results have valid session refs | session_id exists in store |
| DS3 | Search results have valid snippets | Non-empty, highlights always array |
| DS4 | Search filter by user works | All results match |
| DS5 | Search filter by project works | All results match |
| DS6 | Search performance < 2s | Timed assertion |

### Dogfood Git Link Tests

| ID | Test | Assert |
|----|------|--------|
| DG1 | Git links extracted from real data | count > 0 |
| DG2 | Commit lookup returns valid session | session_id exists |
| DG3 | PR lookup returns valid session | session_id exists (skip if no PR links) |
| DG4 | commit_count matches git_links table | API vs SQL cross-check |
| DG5 | Git link results include valid metadata | All fields present |
| DG6 | Search auto-recognizes real commit SHA | Snippet has "Commit" prefix |
| DG7 | Git link message_ordinal points to real message | Message exists at that ordinal |
| DG8 | Sessions without git links have commit_count=0 | Cross-check |
| DG9 | Confidence distribution | All values "high" or "medium" |
| DG10 | Lookup performance < 500ms | Timed assertion |

---

## Agent Instructions

When running an E2E validation session:

1. Ensure ClickHouse is running: `docker compose up -d clickhouse` (from host)
2. Run seeded API tests: `go test ./e2e/... -run "^Test[^D]" -count=1 -v -timeout 120s`
3. Run dogfood tests: `go test ./e2e/... -run TestDogfood -count=1 -v -timeout 600s`
4. Run browser tests: `npx playwright test`
5. Walk through manual golden paths (D16-D24) using CLI + curl

For manual paths:
```bash
# Build and sync
go build -o /tmp/agentstrove ./cmd/agentstrove/ && /tmp/agentstrove sync

# Serve
/tmp/agentstrove serve --port 8080

# Query API
curl -s localhost:8080/api/v1/sessions | jq '.total, (.sessions | length)'
curl -s localhost:8080/api/v1/sessions | jq '.sessions[0]'
```

## Known Edge Cases

These are validated by the test suite:

1. **Invalid date params** — validate with `time.Parse` before building filters, return 400
2. **Empty slices** — always `make([]T, 0)` for empty results, never nil slice (JSON `[]` not `null`)
3. **Tool call ordering** — always include `tool_use_id` as secondary sort key
4. **Ghost/subagent sessions** — apply browsable filter (`parent_session_id = '' AND user_message_count > 0`) to all metadata queries
5. **Invalid UTF-8** — `strings.ToValidUTF8(s, "\uFFFD")` on all text fields before writing to ClickHouse
6. **Tool call message_ordinal** — reader JOINs with messages table to get actual ordinal (agentsview's `message_id` is auto-increment PK, not ordinal)
7. **Git link extraction** — extract from `tool_calls.result_content`, not from `messages.content`
