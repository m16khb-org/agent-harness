---
name: hosts.md
description: Codex, Claude, and Omo native skills, MCP registration, and lifecycle hook operations.
---

# Host Operations

## Codex

Native skill examples:

```text
Use $atomic-commit-push to review my changes, split them into atomic commits, and push safely.
Use $issueops to run a problem -> issue -> plan -> TDD/subagent -> ai-slop-clean -> feedback -> PR/MR cycle.
```

Install checks:

```bash
test -f ~/.codex/skills/atomic-commit-push/SKILL.md && echo ok
test -f ~/.codex/skills/issueops/SKILL.md && echo ok
codex mcp list
codex mcp get agent_harness
```

Codex system skills under `~/.codex/skills/.system` are host-managed, not
agent-harness managed. Verified on 2026-06-23 with `codex-cli 0.142.0`:
`$HOME/Library/pnpm/codex` is a pnpm shim to `@openai/codex@0.142.0`, that
package contains no `skill-creator` or `quick_validate.py` source files,
`~/.codex/skills/.system/.codex-system-skills.marker` contains only the system
skills payload hash, and `~/.codex/vendor_imports/skills-curated-cache.json`
does not list `skill-creator`. Do not patch `~/.codex/skills/.system` as a
durable fix; it can be re-materialized by Codex. Agent-harness skill validation
must use `python3 scripts/validate-skill.py skills/<skill-name>` so local
verification does not depend on upstream Codex system-skill changes or local
PyYAML installation.

Codex lifecycle hooks live in `~/.codex/hooks.json`. Default installation owns exactly `SessionStart`, invoking the shared context CLI with `--host codex`.

Hook behavior:

- `SessionStart` reads the static project-doc catalog and emits host-compatible context for every source, including the `compact` re-run Codex performs after compaction (verified against codex-cli 0.150.1: `post-compact.command.output` carries no `hookSpecificOutput`, so `PostCompact` is not registered). It does not read IssueOps, emit lifecycle reminders, inspect runtime state, write telemetry, maintain SQLite, recover workers, or mutate state.
- `hook post-compact` stays available for Omo (`session_compact` has no SessionStart re-run there) and for diagnosis; installer upgrade removes any managed `PostCompact` group while preserving non-harness groups.
- There is no other hook subcommand. `HARNESS_DISABLE_HOOKS=1` turns the context hooks into a silent no-op.

Hook smoke:

```bash
printf '{"cwd":"%s","source":"compact"}' "$PWD" | agent-harness hook session-start --host codex
agent-harness hook post-compact --host codex --json
```

## Claude Code

Claude Code uses user skills by default:

- User: `~/.claude/skills/<skill>/SKILL.md`
- Project: `.claude/skills/<skill>/SKILL.md` only with explicit project-local opt-in.

Direct skill invocation example:

```text
/atomic-commit-push
```

MCP checks:

```bash
claude mcp list
```

Inside Claude Code:

```text
/mcp
```

Default install registers user-scope MCP server `agent_harness`. This repo's dogfood `.mcp.json` uses `agent_harness_project` to avoid scope collisions.

Claude hooks live in `~/.claude/settings.json`. Default installation owns exactly `SessionStart`, calling the same context CLI/core as Codex with `--host claude`; Claude separates the readable `systemMessage` from the model-facing `hookSpecificOutput.additionalContext`. Claude Code 2.1.247 re-runs `SessionStart` with `source:"compact"` after compaction and treats `PostCompact` stdout as a user display string only, so the catalog is re-established through `SessionStart` and `PostCompact` is not registered.

Claude project-local hooks can be committed, so do not create `.claude/settings.json` in target repos without explicit opt-in.

### Upstream plugins and skills

`configs/upstream.json` declares optional third-party plugins and Git skills
that `agent-harness install` (and therefore `update`) provisions for Claude Code
when missing. Codex and Omo receive only the first-party `skills/` links from
this installer path. Existing upstream entries are skipped, never reinstalled
or overwritten. The object below shows the schema; the complete catalog
currently contains four plugins and one skill.

```json
{
  "version": 1,
  "plugins": [{ "name": "eli5", "marketplace": "claude-community", "source": "anthropics/claude-plugins-community" }],
  "skills": [{ "name": "cua-driver", "repo": "https://github.com/trycua/cua", "path": "libs/cua-driver/rust/Skills/cua-driver", "ref": "main" }]
}
```

- Plugins are provisioned through the Claude Code CLI: the marketplace `source`
  is registered only when missing, then `claude plugin install <name>@<marketplace>
  --scope user --yes` runs. A plugin counts as present when
  `claude plugin list --json` already reports `<name>@<marketplace>`, in any scope.
- Skills are fetched with a shallow sparse `git clone` into
  `<state dir>/upstream/skills/<name>` and linked from `~/.claude/skills/<name>`,
  the same link shape the harness uses for its own skills. A skill counts as
  present when `~/.claude/skills/<name>` exists, whoever created it, so a
  harness-owned or hand-made skill of the same name is left alone. A fetched
  directory without `SKILL.md` is rejected and leaves no link behind.
- Provisioning runs only after the native activation receipt is sealed, and it
  never changes install success. A missing `claude` CLI, an unreachable remote,
  or a failed plugin install is reported as an `upstream ...` message on the
  install result while the harness install itself stays `ok`. This keeps the
  harness install path independent of third-party tooling.
- `agent-harness install --dry-run --json` reports the plan (`planned` /
  `skipped`) without touching the host.
- The cache lives outside `skills/`, so the stale-link prune that removes
  deleted harness skills never touches upstream skill links.

Choosing the entry kind. Declare an upstream as a `plugin` whenever the project
publishes a `.claude-plugin/marketplace.json`, and as a `skill` only when no
marketplace exists for it. Three reasons the plugin kind is preferred:

- It is the upstream author's own documented install path, and it carries a
  version the host can update.
- A plugin's skills are namespaced `<plugin>:<skill>` in the host, so a plugin
  whose skill shares a name with a harness skill coexists with it instead of
  competing for `~/.claude/skills/<name>`.
- The host materializes the whole plugin, so the upstream `LICENSE` comes with
  it. A `skill` entry sparse-fetches one subdirectory, which usually leaves the
  repository's root `LICENSE` behind — check the fetched directory carries its
  own notice before declaring a skill entry for third-party code.

A `skill` entry whose name matches a skill the harness itself ships can never
install: the harness links its own `skills/<name>` first, so the presence check
always skips the upstream. Declare such a project as a plugin, or accept that
the entry is a provenance record only.

## Omo Native

Omo stores runtime configuration in its branded roots and user skills in the
Senpi agent directory:

- User skills: `~/.omo/agent/skills/<skill>/SKILL.md`
- User MCP: `~/.omo/mcp.json`
- User lifecycle extension: `~/.omo/extensions/agent-harness.js`
- Project skills and MCP: `.omo/skills/*` and `.omo/mcp.json` only with
  explicit project-local opt-in.

The managed extension maps Omo `session_start` and accepted `session_compact`
events to `agent-harness hook session-start --json` and
`agent-harness hook post-compact --json`. It injects compact project-doc context
as a hidden custom message without triggering a new turn. Install/update strict
readback seals both the MCP file and extension bytes before activation commits.

Checks:

```bash
test -f ~/.omo/agent/skills/atomic-commit-push/SKILL.md
test -f ~/.omo/mcp.json
test -f ~/.omo/extensions/agent-harness.js
```

Agent-harness does not install or gate the external Omo runtime itself; install
Omo through its official distribution path.

## IssueOps Host Rule

Hooks only inject static project-doc context. They do not create issues, edit files, run tests, wait on background jobs, prepare branches/worktrees, open PRs/MRs, reply to review threads, merge, clean up branches/worktrees, or block tool events. The main agent loop owns the user-visible `problem -> grill -> issue linkage -> plan -> compatibility-review -> implement -> ai-slop-clean -> feedback -> pr -> done -> post-done cleanup` workflow through `agent-harness issueops ...` CLI/MCP state. The durable phase enum has no artifact-linkage or cleanup labels; explicit IssueOps commands own CAS, lease authority, remote writes, verification, publication, and cleanup. Hook output is neither an enforcement path nor ownership evidence.
