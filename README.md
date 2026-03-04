# Agentstrove

Open a PR, see the AI conversation that produced it. Agentstrove turns your team's agent conversations into a shared, searchable resource linked to your code.

## What It Does

A background daemon syncs conversations from local AI coding agents (via [agentsview](https://github.com/wesm/agentsview)) to shared ClickHouse storage. A web UI lets the team browse, search, and navigate from git commits/PRs to the conversations that produced them.

- **Auto-sync** — daemon watches local sessions and syncs to shared storage. Nothing changes in your workflow.
- **Secret masking** — credentials and keys are redacted before data leaves the developer's machine.
- **Full-text search** — search across all team conversations with filtered results and context snippets.
- **Git linking** — commits and PRs link back to originating conversations via tool call detection.
- **Works with your agents** — Claude Code, Cursor, GitHub Copilot, Gemini CLI, Codex, Amp, VSCode Copilot, OpenCode via agentsview.

## Architecture

```
Developer's machine                     Shared infrastructure
┌─────────────────┐                    ┌──────────────────┐
│ Claude Code /    │                   │                  │
│ Cursor / etc.    │                   │   ClickHouse     │
│       ↓          │                   │   (conversations,│
│  agentsview      │  ──── daemon ───→ │    search, git   │
│ (local sessions) │   (secret mask    │    links)        │
│                  │    + sync)        │       ↓          │
└─────────────────┘                    │   Web UI         │
                                       │   (browse,       │
                                       │    search, link) │
                                       └──────────────────┘
```

## Quick Start

```bash
# Start ClickHouse
docker compose up -d clickhouse

# Sync local conversations
agentstrove sync

# Browse conversations
agentstrove serve
# Open http://localhost:8080
```

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

## Project Status

Under active development. See [docs/roadmap.md](docs/roadmap.md) for the incremental ship plan.

## License

TBD
