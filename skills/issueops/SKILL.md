---
name: issueops
description: Run an issue-driven work cycle from problem discovery through domain grilling, issue creation, planning, TDD/subagent implementation, feedback loops, and PR/MR drafting.
---

# IssueOps

Use this skill when the user wants a repeatable cycle from a vague problem to a GitHub/GitLab issue, implementation plan, tested change, feedback loop, and PR/MR.

## Contract

The workflow is advisory and agent-driven. Hooks may suggest this skill, but hooks must not create issues, edit files, run tests, or open PRs/MRs by themselves.

The cycle has one durable state record. Use `agent-harness issueops ... --json` or the matching MCP tools when the cycle needs to survive compaction, handoff, or another host.

Required phases:

1. Problem intake: use `superpowers:brainstorming` to clarify the actual problem, constraints, success criteria, and ambiguity.
2. Domain grill: use `grill-with-docs` to challenge terminology, existing domain model fit, and documentation updates before committing to an issue.
3. Issue contract: create or prepare a GitHub/GitLab issue that states the problem, acceptance criteria, non-goals, verification, and open decisions.
4. Plan: use `superpowers:writing-plans` to produce an issue-based implementation plan under the target repo's planning convention.
5. Implementation: use `superpowers:test-driven-development` for behavior changes and `superpowers:subagent-driven-development` when independent tasks can be split safely.
6. Feedback loop: collect user, review, QA, and CI feedback; classify each item; update the issue/plan when the contract changes; then continue implementation.
7. PR/MR: draft the PR/MR only after the issue URL and plan path are linked and the relevant verification has been run.

## Remote Issue First

When the user explicitly invokes `$issueops` and the repo remote, credentials, target project, branch target, and issue ownership are discoverable, create the remote GitHub/GitLab issue before planning or implementation. Then immediately link it with `agent-harness issueops link-issue`.

Only prepare a local issue draft instead of creating a remote issue when one of those values is unclear, credentials are unavailable, or the user explicitly asks not to create a remote issue.

If the agent realizes it implemented before creating or linking the issue, it must stop implementation, create or link the issue if possible, record corrective feedback in IssueOps state, and then resume from the issue-linked plan.

## Branch And Worktree Contract

After the issue is created or linked and before implementation, derive the working branch from the issue using a branch prefix convention. Use the target repo's convention when documented; otherwise choose the narrowest accurate prefix:

- `feature/` for new capabilities or integrations.
- `bugfix/` for ordinary defects.
- `hotfix/` only for urgent production patches.
- `release/` only for release preparation.
- `chore/` for tooling, documentation, maintenance, or workflow-only changes.

The branch slug must include the issue number when available and a short kebab-case issue title, for example `feature/3-headroom-upstream-integration` or `chore/12-tighten-issueops-worktree-contract`.

Create an isolated git worktree before implementation, TDD, subagent work, verification, commit, or PR/MR drafting:

```bash
branch_slug="feature/3-headroom-upstream-integration"
worktree_path="../$(basename "$PWD").worktrees/${branch_slug//\//-}"
git worktree add -b "$branch_slug" "$worktree_path"
```

Keep IssueOps worktrees as siblings of the source checkout under the fixed pattern `../<repo>.worktrees/<branch-slug-with-slashes-replaced>`. Do not create ad hoc worktree paths inside the repo or under temporary directories unless the user explicitly asks for a different location.

When the worktree needs large generated dependency directories such as `node_modules`, prefer reusing an existing dependency directory by symlink only after verifying the package manager, lockfile, platform, and dependency state match the source checkout. Example:

```bash
test -d "$PWD/node_modules"
test -f "$PWD/package-lock.json" || test -f "$PWD/pnpm-lock.yaml" || test -f "$PWD/yarn.lock"
ln -s "$PWD/node_modules" "$worktree_path/node_modules"
```

Do not symlink dependency directories when the worktree uses a different lockfile, package manager, Node version, platform-specific native modules, or when installing fresh dependencies would be safer. Never commit generated dependency symlinks or dependency directories; keep them ignored or remove them before PR/MR cleanup.

Local config files may also be symlinked into the worktree when the task needs them and the source checkout already has the correct local-only configuration. Common candidates include `.env`, `.env.local`, `.mcp.json`, `dbhub.toml`, and other documented untracked local config files. Verify each candidate exists, is intended for local development, and is ignored or otherwise excluded from commits before linking it:

```bash
for config in .env .env.local .mcp.json dbhub.toml; do
  if [[ -e "$PWD/$config" ]]; then
    git check-ignore -q "$config" || printf 'review before linking tracked or unignored config: %s\n' "$config" >&2
    ln -s "$PWD/$config" "$worktree_path/$config"
  fi
done
```

Do not symlink secret-bearing config into a worktree for review, PR/MR drafting, or artifact generation unless the command actually needs it. Never print config contents in prompts, logs, issue bodies, PR/MR bodies, or test output. If a config file is tracked, unignored, environment-specific in a way that changes behavior, or contains credentials that are not needed for the task, stop and ask before linking it.

Run implementation from the worktree path, not from the source checkout. Record the expected branch and worktree path in the issue-based plan and in any worker prompt. If the source checkout already contains implementation edits from before this gate, stop and ask how to move or reconcile those edits into the issue branch worktree.

## Context Routing

Use CodeGraph as the default context layer for structural work, with `rg` as the fallback and exact-search tool.

- Start with CodeGraph for functions, classes, call relationships, dependency paths, impact analysis, module boundaries, and route/controller/service relationships.
- Start with `rg` for exact strings: error messages, env keys, config values, filenames, TODOs, comments, logs, and literal function names.
- For natural-language feature location, use CodeGraph first, then run at least one targeted `rg` check before editing or claiming there are no usages.
- After edits, use `rg` plus the relevant tests to catch missed references or regressions.
- Treat CodeGraph as advisory when its index may be stale or when the target uses dynamic wiring such as runtime DI, reflection, dynamic imports, or framework provider registration. Refresh or verify the index before relying on it.
- Keep graph results small and targeted; oversized call/dependency graphs waste more context than direct text search.

## Worker And Review Gates

Every implementation, TDD, review, QA, or subagent worker prompt must require the worker to begin by reporting and verifying:

- `pwd`
- `git branch --show-current`
- `git rev-parse --short HEAD`
- the expected isolated worktree path

If any value does not match the IssueOps branch/worktree contract, the worker must stop and report the mismatch instead of reviewing or editing.

For short or narrow reviews, prefer `verifier` or a direct bounded review over `code-reviewer`. When `code-reviewer` is necessary, the prompt must set a clear time budget, forbid nested subagent fan-out, and require the reviewer to verify `pwd`, branch, `HEAD`, and worktree path before inspecting the diff.

## State Commands

Start:

```bash
agent-harness issueops start --repo "$PWD" --branch "$(git branch --show-current)" --json
```

Link the issue:

```bash
agent-harness issueops link-issue --id "$ISSUEOPS_ID" --issue-url "$ISSUE_URL" --json
```

Link the plan:

```bash
agent-harness issueops link-plan --id "$ISSUEOPS_ID" --plan-path "$PLAN_PATH" --json
```

Record feedback:

```bash
agent-harness issueops feedback add --id "$ISSUEOPS_ID" --source user --body "$FEEDBACK" --json
```

Check PR/MR readiness:

```bash
agent-harness issueops pr-readiness --id "$ISSUEOPS_ID" --json
```

Run the 100-point quality benchmark:

```bash
agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json
agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge agy --json
```

The benchmark passes only when every fixture has `average_score: 100`, `minimum_score: 100`, and `critical_failure_count: 0`. Use `--judge agy` for the real LLM gate when Antigravity quota is available; use `--judge none` only for deterministic local evidence.

## Issue Template

Use this structure unless the target project already has a stronger issue template:

```markdown
## Problem

## Current Evidence

## Acceptance Criteria

## Non-goals

## Plan Link

## Verification

## Feedback Log
```

## Stop Conditions

Stop and ask the user before creating or updating remote issues, PRs, or MRs if credentials, target project, branch target, or issue ownership are unclear.

Stop before implementation if brainstorming or grilling exposes materially different interpretations. Present the interpretations and ask for the intended one.

Do not move to PR/MR drafting when `issueops pr-readiness` reports missing `issue_url` or `plan_path`.
