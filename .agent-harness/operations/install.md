---
name: install.md
description: Bootstrap, sync, install-native, and upstream companion tool operations.
---

# Install And Sync

Public setup UX has two primary commands:

```bash
# Install or refresh user-level Codex/Claude integration for this checkout.
agent-harness bootstrap

# Also refresh opt-in upstream companion tools.
agent-harness bootstrap --sync

# Initialize project docs/profile for a target repository.
agent-harness project bootstrap --repo /path/to/repo

# Refresh existing project docs/profile from current templates and evidence.
agent-harness project bootstrap --repo /path/to/repo --sync
```

`bootstrap` uses the current `agent-harness` checkout. It builds `bin/agent-harness`, updates the `~/.local/bin/agent-harness` shim, and runs native host installation. It does not run `git pull`.

Default user-level install updates:

- Codex skill symlinks: `~/.codex/skills/* -> <agent-harness>/skills/*`
- Claude skill symlinks: `~/.claude/skills/* -> <agent-harness>/skills/*`
- Codex MCP config: `~/.codex/config.toml` `[mcp_servers.agent_harness]`
- Codex hooks: `~/.codex/hooks.json`
- Claude hooks: `~/.claude/settings.json`
- Claude user-scope MCP server: `claude mcp add-json -s user agent_harness ...`

Default install does not create target-repo `.claude/skills`, `.claude/settings.json`, or `.mcp.json`. Use explicit project-local options only when a repo should own those files.

Dry-run checks:

```bash
agent-harness bootstrap --dry-run --json
agent-harness install-native --dry-run --json
```

## `--sync`

`--sync` means "refresh from current evidence."

- `agent-harness bootstrap --sync` refreshes user-level host integration and opt-in upstream companion tools.
- `agent-harness project bootstrap --sync` refreshes target repo `AGENTS.md` routing block, `.agent-harness/*.md`, and user-state repo profile metadata.

Use low-level `scripts/install-native.sh` and `install-native` directly only for automation or focused installer debugging.

## Upstream Companion Tools

| Tool | Upstream | Operation |
|------|----------|-----------|
| LLM Wiki | `nvk/llm-wiki` | Adds or updates `wiki@llm-wiki` for Codex and Claude marketplace/plugin paths. |
| CodeGraph | `colbymchenry/codegraph` | Installs `@colbymchenry/codegraph`, registers Codex/Claude MCP, and initializes this repo's `.codegraph/` index when enabled. |
| claude-mem | `thedotmack/claude-mem` | Runs `npx claude-mem@latest install` for Codex/Claude hooks, MCP, and worker wiring. |
| Headroom | `chopratejas/headroom` / `headroom-ai` | Optional context optimization companion. Do not auto-proxy Codex/Claude traffic without explicit operator action. |

CodeGraph index creation can be skipped:

```bash
HARNESS_INIT_CODEGRAPH=0 agent-harness bootstrap --sync
```

Companion tool failures must not weaken the `agent-harness` core contract. Fix the upstream installation path or document the external requirement.
