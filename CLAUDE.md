# code-agent

GitHub issue agent dispatcher. Polls issues in the Meeting-Baas org, dispatches Claude Code agents to resolve them, and opens PRs.

## Build

```
make build
```

## Architecture

- **Config**: TOML with repo→path mappings + auto-discovery
- **State**: SQLite (issues, agent_runs, poll_state)
- **GitHub**: Shell out to `gh` and `git` (already authenticated)
- **Agent**: `claude -p` with sonnet model
- **MCP**: 5 tools for interactive use

## Key Commands

```
code-agent poll          # One-shot poll and dispatch
code-agent serve         # Long-running daemon
code-agent mcp           # MCP server (stdio)
code-agent mcp-http      # MCP server (HTTP)
code-agent runs          # Show recent agent runs
code-agent config        # Show current config
code-agent init-db       # Initialize database
```
