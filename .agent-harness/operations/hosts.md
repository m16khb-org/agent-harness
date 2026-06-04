---
name: hosts.md
description: Codex and Claude native skills, MCP registration, and lifecycle hook operations.
---

# Host Operations

## Codex

Native skill examples:

```text
Use $atomic-commit-push to review my changes, split them into atomic commits, and push safely.
Use $issueops to run a problem -> issue -> plan -> TDD/subagent -> feedback -> PR/MR cycle.
Use $workflows in Codex to run an explicit dynamic workflow with batched subagents.
```

Install checks:

```bash
test -f ~/.codex/skills/atomic-commit-push/SKILL.md && echo ok
test -f ~/.codex/skills/issueops/SKILL.md && echo ok
codex mcp list
codex mcp get agent_harness
```

Codex lifecycle hooks live in `~/.codex/hooks.json`. `UserPromptSubmit` invokes `agent-harness hook user-prompt --host codex`; `PreToolUse`, `PostToolUse`, `PreCompact`, `PostCompact`, and `Stop` call the shared hook CLI.

Hook behavior:

- `UserPromptSubmit`: dynamic routing/profile/upkeep hint only.
- `PreToolUse`: installed with worktree, Korean remote artifact, VCS issue-linking, and GitOps kubectl guards. The worktree guard is a no-op unless an IssueOps cycle or `HARNESS_EXPECTED_WORKTREE` applies; the kubectl guard blocks direct mutating cluster commands, asks for confirmation on live access such as `exec` and `port-forward`, and allows read-only inspection plus dry-runs. Raw `--json` exposes diagnostics.
- `--enforce-search-routing`: optional deterministic block for obvious CodeGraph/rg routing mismatches.
- `PostToolUse`: records only successful mutating tool evidence into repo-scoped user state.
- `PreCompact`/`PostCompact`: save and restore a small pending-upkeep capsule once.
- `Stop`: installed with `--enforce-numbered-next-actions`; it is a no-op unless `HARNESS_EXPECT_NUMBERED_NEXT_ACTIONS=1` and the host exposes the final assistant message.

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

Claude hooks live in `~/.claude/settings.json`. They call the same shared CLI/core as Codex, but Claude can separate readable `systemMessage` from model-facing `hookSpecificOutput.additionalContext`. `PreToolUse` and `PostToolUse` use matcher `*`; `PreToolUse` runs deterministic worktree, remote artifact, issue-linking, and GitOps kubectl guards. `Stop` can enforce numbered next-action choices when `HARNESS_EXPECT_NUMBERED_NEXT_ACTIONS=1`.

Claude project-local hooks can be committed, so do not create `.claude/settings.json` in target repos without explicit opt-in.

## IssueOps Host Rule

Hooks may suggest IssueOps but must not create issues, edit files, run tests, wait on background jobs, or open PRs/MRs. The main agent loop owns remote writes and must pass Korean artifact gates before issue or PR/MR mutation.
