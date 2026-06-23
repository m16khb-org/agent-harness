---
name: hosts.md
description: Codex and Claude native skills, MCP registration, and lifecycle hook operations.
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
`/Users/habin/Library/pnpm/codex` is a pnpm shim to `@openai/codex@0.142.0`,
that package contains no `skill-creator` or `quick_validate.py` source files,
`~/.codex/skills/.system/.codex-system-skills.marker` contains only the system
skills payload hash, and `~/.codex/vendor_imports/skills-curated-cache.json`
does not list `skill-creator`. Do not patch `~/.codex/skills/.system` as a
durable fix; it can be re-materialized by Codex. For issues such as
`skill-creator/scripts/quick_validate.py` requiring external `yaml`, keep a
local dependency workaround only as an operator fix and pursue the durable fix
through the upstream Codex system-skill distribution path.

Codex lifecycle hooks live in `~/.codex/hooks.json`. `UserPromptSubmit` invokes `agent-harness hook user-prompt --host codex`; `PreToolUse`, `PostToolUse`, `PreCompact`, `PostCompact`, and `Stop` call the shared hook CLI.

Hook behavior:

- `UserPromptSubmit`: dynamic routing/profile/upkeep hint only.
- `PreToolUse`: installed with worktree, Korean remote artifact, VCS issue-linking, staged-check, and GitOps kubectl guards. The Korean remote artifact and VCS issue-linking guards inspect both shell CLI commands and MCP tool-call shapes for issue/PR/MR create/edit/update requests. The worktree guard is a no-op unless an IssueOps cycle or `HARNESS_EXPECTED_WORKTREE` applies; the staged-check guard asks before broad `biome check apps libs` / broad package-script lint or format checks so agents use staged or changed-file scope first; the kubectl guard blocks direct mutating cluster commands, asks for confirmation on live access such as `exec` and `port-forward`, and allows read-only inspection plus dry-runs. Raw `--json` exposes diagnostics.
- `--enforce-search-routing`: optional deterministic block for obvious CodeGraph/rg routing mismatches.
- `PostToolUse`: records only successful mutating tool evidence into repo-scoped user state. It must not auto-queue draft-wiki material; the main agent explicitly queues judged reusable material with `agent-harness project draft-wiki queue --stdin` or `--input`.
- `PreCompact`/`PostCompact`: save and restore a small pending-upkeep capsule once.
- `Stop`: installed with `--enforce-numbered-next-actions`; when the host exposes the final assistant message or transcript, it blocks missing numbered next actions and tells the agent to explain the block before presenting context-specific choices. With `--relay-next-action-judgement`, it relays only observed next-action facts back to the main agent; the main agent must then state why it is auto-proceeding or why it is not auto-proceeding and needs user confirmation. Auto-proceed result reports still include `선택지:` with three options and exactly one recommendation. No-auto-proceed judgements stop without adding another choices block.

Hook smoke:

```bash
printf '{"prompt":"endpoint와 DTO를 추가해줘"}' | agent-harness hook user-prompt
agent-harness hook stop --json
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

Claude hooks live in `~/.claude/settings.json`. They call the same shared CLI/core as Codex, but Claude can separate readable `systemMessage` from model-facing `hookSpecificOutput.additionalContext`. `PreToolUse` and `PostToolUse` use matcher `*`; `PreToolUse` runs deterministic worktree, remote artifact, issue-linking, staged-check, and GitOps kubectl guards. `Stop` enforces numbered next-action choices through `--enforce-numbered-next-actions`.

Claude project-local hooks can be committed, so do not create `.claude/settings.json` in target repos without explicit opt-in.

## IssueOps Host Rule

Hooks may suggest IssueOps but must not create issues, edit files, run tests, wait on background jobs, or open PRs/MRs. The main agent loop owns remote writes and must pass Korean artifact gates before issue or PR/MR mutation.
