# Agentlore

Open a PR, see the AI conversation that produced it. Agentlore turns your team's agent conversations into a shared, searchable resource linked to your code.

## Why

AI coding agents are part of everyday development, but conversations stay trapped on individual machines. Agentlore makes them a shared, searchable team resource — linked to your code.

## How It Works

1. **Developers use their agents as usual** — Claude Code, Cursor, Copilot, Gemini CLI, Codex, Amp, OpenCode. Nothing changes in the workflow.
2. **[agentsview](https://github.com/wesm/agentsview)** collects local sessions into a SQLite database on each developer's machine.
3. **The agentlore daemon** watches for new sessions, masks secrets (API keys, credentials), and syncs conversations to shared ClickHouse storage.
4. **The web UI** lets the team browse conversations, full-text search across all sessions, and navigate from git commits/PRs back to the conversations that produced them.

Git linking works by parsing tool calls in conversations for `git commit`, `git push`, and `gh pr create` invocations, extracting commit SHAs and PR references.

## Prerequisites

Install [agentsview](https://github.com/wesm/agentsview) to collect local agent sessions:

```bash
curl -fsSL https://raw.githubusercontent.com/wesm/agentsview/main/install.sh | bash
```

See the [agentsview README](https://github.com/wesm/agentsview) for other install methods.

## Install

```bash
git clone https://github.com/clkao/agentlore.git
cd agentlore
make build install    # builds frontend + Go binary, installs to ~/.local/bin
```

Requires Go 1.25+, Node 22+, and Docker (for ClickHouse).

Start ClickHouse:

```bash
docker compose up -d
```

## Quick Start

```bash
# Start agentsview to collect local sessions (runs on port 8080)
agentsview &

# Sync conversations to ClickHouse and start the web UI
agentlore sync
agentlore serve
# Open http://localhost:9090
```

## Configuration

Agentlore reads `~/.config/agentlore/config.json`. All fields are optional — defaults work for local use with `docker compose`.

```json
{
  "clickhouse_addr": "localhost:9000",
  "clickhouse_database": "agentlore",
  "clickhouse_user": "",
  "clickhouse_password": "",
  "agentsview_db_path": "~/.agentsview/sessions.db",
  "server_port": 9090
}
```

For remote or managed ClickHouse, set `clickhouse_addr`, `clickhouse_secure`, and credentials. The `agentsview_db_path` defaults to `~/.agentsview/sessions.db` or `~/.claude/agentsview/sessions.db`, whichever exists.

## Current Limitations

- **ClickHouse full-text search requires v26+** — the FTS features agentlore uses are not yet available in ClickHouse Cloud.
- **Some tool results are not stored** — agentsview filters out verbose tool outputs (Read, Glob, etc.) by default, so those won't appear in synced conversations.
- **Git commit linking is fragile** — SHA extraction from tool calls works for common patterns but can miss or misattribute commits in edge cases.
- **Syncs everything agentsview sees** — there's no per-project filter in agentlore itself. To limit scope, create a custom projects directory with symlinks to the sessions you want indexed, then point agentsview at it:
  ```bash
  mkdir -p ~/agentlore-projects
  ln -s ~/.claude/projects/-Users-you-work-myproject ~/agentlore-projects/
  CLAUDE_PROJECTS_DIR=~/agentlore-projects agentsview -no-browser
  ```
  Then set `agentsview_db_path` in agentlore's config to use that database.

## Roadmap

- Improve git commit and PR linking reliability
- Project identity reconciliation (map paths to canonical project IDs after the fact)
- Guides for setting up auth (Cloudflare Access, etc.)
- agentsview insights sharing

## Development

```bash
# Prerequisites: Go 1.25+, Node 22+, Docker

# Backend
go test ./internal/...

# Frontend
cd frontend && npm install && npm run dev

# Full build (frontend + Go binary)
make build
```

## Acknowledgements

- [agentsview](https://github.com/wesm/agentsview) by [Wes McKinney](https://github.com/wesm) — the local agent session collector that agentlore reads from.
- [ClickHouse](https://clickhouse.com/) — the storage engine powering conversation search and analytics.

## License

MIT
