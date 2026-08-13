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

Codex lifecycle hooks live in `~/.codex/hooks.json`. Default installation owns exactly `SessionStart` and `PostCompact`, both invoking the shared context CLI with `--host codex`.

Hook behavior:

- `SessionStart` and `PostCompact`: read the static project-doc catalog and emit host-compatible context. They do not read IssueOps, emit lifecycle reminders, inspect runtime state, write telemetry, maintain SQLite, recover workers, or mutate state.
- Legacy hook CLI subcommands (`user-prompt`, `pre-tool-use`, `post-tool-use`, `pre-compact`, `stop`) remain available only when explicitly invoked for diagnosis or compatibility; installer upgrade removes their managed registrations while preserving non-harness groups.

Hook smoke:

```bash
agent-harness hook session-start --host codex --json
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

Claude hooks live in `~/.claude/settings.json`. Default installation owns exactly `SessionStart` and `PostCompact`, both calling the same context CLI/core as Codex with `--host claude`; Claude can separate readable `systemMessage` from model-facing `hookSpecificOutput.additionalContext`.

Claude project-local hooks can be committed, so do not create `.claude/settings.json` in target repos without explicit opt-in.

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

Hooks may suggest IssueOps but must not create issues, edit files, run tests, wait on background jobs, prepare branches/worktrees, open PRs/MRs, reply to review threads, merge, or clean up branches/worktrees. The main agent loop owns the `problem -> grill -> issue -> plan -> compatibility-review -> implement -> ai-slop-clean -> feedback -> pr -> cleanup` state machine through `agent-harness issueops ...` CLI/MCP state.

Default installed hooks have no authority boundary beyond static context injection. Explicit `agent-harness issueops ...` commands own durable CAS, lease authority, remote writes, verification, and publication; any legacy diagnostic hook must not be treated as an installed enforcement path.
