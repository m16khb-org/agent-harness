---
name: install.md
description: First-run install, sync, compatibility install-native, and upstream companion tool operations.
---

# Install And Sync

Public setup UX has two primary commands:

```bash
# First run from a fresh clone, before agent-harness is on PATH.
./install.sh

# Scriptable install or refresh after agent-harness is on PATH.
agent-harness install --dry-run --json
agent-harness install

# Ongoing refresh from the current checkout.
agent-harness update

# Also refresh opt-in upstream companion tools.
agent-harness bootstrap --sync

# Initialize project docs/profile for a target repository.
agent-harness project bootstrap --repo /path/to/repo

# Refresh existing project docs/profile from current templates and evidence.
agent-harness project bootstrap --repo /path/to/repo --sync
```

`./install.sh` computes the checkout root, builds `bin/agent-harness` when needed, and then runs `agent-harness install`. In a real terminal with no arguments it enters the interactive installer. Non-interactive automation can pass explicit flags such as `--dry-run --json`.

`install` owns environment setup. Normal users should not export `HARNESS_ROOT` manually; the installer writes it into Codex/Claude MCP configuration. `CODEX_HOME` is honored when already set and otherwise defaults to `~/.codex`. PATH setup is selected with `--path-mode=auto|manual|skip`, and the default `auto` mode plans or writes `~/.local/bin/agent-harness` plus a shell rc PATH line when needed.

`bootstrap` and `update` use the current `agent-harness` checkout. They build `bin/agent-harness`, refresh the `~/.local/bin/agent-harness` shim through the same installer path, run native host installation, refresh MCP registration, and restart the shared daemon when it is already running so the MCP backend uses the rebuilt binary. They do not run `git pull`.

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
./install.sh --dry-run --json
agent-harness install --dry-run --json
agent-harness bootstrap --dry-run --json
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
