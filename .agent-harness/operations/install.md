---
name: install.md
description: Bootstrap, sync, install-native, and upstream companion tool operations.
---

# Install And Sync

## Homebrew / single-binary install (no checkout, no token)

For end users who do not want to clone and build, the public tap installs a
self-contained binary (skills and config templates are embedded via `go:embed`):

```bash
brew tap m16khb/agent-harness https://github.com/m16khb/agent-harness
brew install agent-harness
agent-harness install-native
```

No login or token is required (the repo is public). `install-native` registers
the MCP server and hooks using the absolute binary path, so the agent-harness
daemon auto-starts when Codex/Claude Code launches the MCP server. Skills are
materialized from the embedded copy when no checkout is present; set
`HARNESS_ROOT=<repo>` for the developer workflow that symlinks live repo files.

Companion tools (codegraph, claude-mem, llm-wiki) and Claude Code marketplace
plugins are not installed by this path; they remain opt-in / host-managed.

Releases are produced by goreleaser on tag push (`.goreleaser.yaml`,
`.github/workflows/release.yml`). The Homebrew formula lives in this repo under
`Formula/`; goreleaser opens a pull request to update it on each release (no
separate tap repo, no PAT, protected main respected). Merge that PR to publish
the new version, then `brew upgrade agent-harness`.

## Developer / checkout install

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
