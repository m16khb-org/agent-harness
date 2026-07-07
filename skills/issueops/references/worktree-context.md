# IssueOps Worktree And Context Rules

## Branch And Worktree Contract

After the issue is created or linked and before implementation, derive the working branch from the issue using GitLab's native linking convention: start the full branch name with the issue or task number followed by a hyphen. Use a short kebab-case issue title after the number, for example `3-webhook-delivery` or `12-tighten-issueops-worktree-contract`. Do not add `feature/`, `hotfix/`, or another branch prefix before the issue number; GitLab only auto-links branches whose full branch name starts with the issue number.

Create the provider-linked branch before creating a local worktree. The IssueOps branch preparation contract is MCP-first, provider API fallback, then fail closed:

- GitLab: use MCP `mcp__glab.glab_api` to create `POST projects/:fullpath/repository/branches` with `branch=<issue-number>-<slug>` and `ref=<base-branch>`. If MCP is unavailable or fails, use `glab api projects/:fullpath/repository/branches -X POST -f branch=<branch> -f ref=<base-branch>`. If both fail, stop.
- GitHub: use a GitHub MCP linked-branch tool only when one is exposed. If no such MCP tool is exposed, use `gh issue develop "$ISSUE_URL" --base "$BASE_BRANCH" --name "$branch_slug"`. If linked branch creation fails, stop.
- After creation, verify through the provider UI/API/CLI that the issue lists the branch before creating the local worktree.

Create an isolated git worktree before implementation, TDD, subagent work, verification, commit, or PR/MR drafting:

```bash
branch_slug="3-webhook-delivery"
base_branch="main"
agent-harness issueops start --repo "$PWD" --branch "$branch_slug" --json
agent-harness issueops intent record --id "$ISSUEOPS_ID" \
  --raw-request "$RAW_USER_REQUEST" \
  --interpreted-intent "$INTERPRETED_INTENT" \
  --success-criteria "$SUCCESS_CRITERION" \
  --json
agent-harness issueops link-issue --id "$ISSUEOPS_ID" --issue-url "$ISSUE_URL" --json
agent-harness issueops branch prepare --id "$ISSUEOPS_ID" --provider gitlab --issue-url "$ISSUE_URL" --branch "$branch_slug" --base-branch "$base_branch" --link-verified --json
worktree_path="../$(basename "$PWD").worktrees/${branch_slug//\//-}"
git fetch origin "$branch_slug"
git worktree add --track -b "$branch_slug" "$worktree_path" "origin/$branch_slug"
expected_worktree="$(cd "$worktree_path" && pwd)"
agent-harness issueops link-worktree --id "$ISSUEOPS_ID" --worktree-path "$expected_worktree" --json
agent-harness issueops design review --id "$ISSUEOPS_ID" \
  --problem-summary "$PROBLEM_SUMMARY" \
  --proposed-design "$PROPOSED_DESIGN" \
  --verification "$VERIFICATION_STEP" \
  --approved \
  --json
agent-harness issueops link-plan --id "$ISSUEOPS_ID" --plan-path "$expected_worktree/$PLAN_REL_PATH" --json
agent-harness issueops worktree prepare-tools --id "$ISSUEOPS_ID" --json
agent-harness issueops phase --id "$ISSUEOPS_ID" --to implement --json
```

Keep IssueOps worktrees as siblings of the source checkout under the fixed pattern `../<repo>.worktrees/<branch-slug-with-slashes-replaced>`. Do not create ad hoc worktree paths inside the repo or under temporary directories unless the user explicitly asks for a different location.

Run implementation from the worktree path, not from the source checkout. Record the expected branch and worktree path in the issue-based plan and in any worker prompt. `issueops phase --to plan` requires both linked issue and recorded intent contract. `issueops link-worktree` requires linked issue plus verified provider branch evidence and an existing worktree directory. `issueops link-plan` is recorded after `link-worktree` and approved design review, and requires the plan file to exist inside that linked worktree. If the source checkout already contains implementation edits from before this gate, stop and ask how to move or reconcile those edits into the issue branch worktree.

The linked worktree path is authoritative. Once `issueops link-worktree` is recorded, the lifecycle PreToolUse guard blocks mutating tool targets outside that exact path, including the source checkout on `main` and another sibling worktree for a different issue branch. During `implement`, `ai-slop-clean`, `feedback`, and `pr`, a cycle with no linked worktree is fail-closed for source/worktree edits: create the sibling worktree and run `issueops link-worktree` before changing implementation files. The lifecycle command refuses to enter `plan` until linked issue and intent contract are recorded, refuses `implement` until provider-linked branch, plan, existing worktree evidence, approved design review, and durable worktree tool preparation are recorded, refuses `ai-slop-clean` until implementation changes are also recorded, refuses `pr` until strict PR readiness is green, and refuses `done` until the loop has first entered `pr` and a verified remote PR/MR artifact is recorded.

## Edit Target Guard

Shell `pwd` and `workdir` checks do not prove that every editing tool will write to that same path. Before any manual edit, patch, generated file write, or formatting command that may rewrite files, verify the target path itself is under the expected isolated worktree.

Required guard before editing:

```bash
pwd
git branch --show-current
git rev-parse HEAD
test "$PWD" = "$EXPECTED_WORKTREE"
git -C "$SOURCE_CHECKOUT" status --short
git status --short
```

Editing rules:

- Prefer absolute file paths rooted at `$EXPECTED_WORKTREE` for patch/edit tools whose working directory is ambiguous.
- Do not use a relative patch path unless the tool is confirmed to apply relative to the expected worktree.
- Do not edit files from the source checkout/main branch during an IssueOps implementation.
- If a tool writes to the source checkout by mistake, stop immediately, move only your own changes into the isolated worktree, and restore the source checkout to its pre-edit clean state. Do not continue implementation until both statuses prove the source checkout is clean and the worktree contains the intended changes.
- After each edit batch, run `git -C "$SOURCE_CHECKOUT" status --short` and `git -C "$EXPECTED_WORKTREE" status --short`.

Worker and subagent prompts must include this guard whenever the worker may edit files. The worker must report the two status outputs before claiming it has changed files in the correct workspace.

## Deterministic Hook Guard

IssueOps sessions should enable the PreToolUse worktree guard in addition to prompt rules:

```bash
export HARNESS_SOURCE_CHECKOUT="/path/to/source-checkout"
export HARNESS_EXPECTED_WORKTREE="/path/to/../repo.worktrees/chore-19-example"
```

Installed Codex and Claude PreToolUse hooks include `--enforce-worktree`. When `HARNESS_EXPECTED_WORKTREE` is set, the hook blocks mutating tool events whose cwd or target path is outside that worktree. When an active IssueOps cycle has a linked worktree path, the hook also blocks mutating targets outside that exact linked path. Without `HARNESS_EXPECTED_WORKTREE`, the guard reads the current branch's IssueOps cycle. During code-editing phases it blocks source-checkout edits if no worktree is linked, blocks `git checkout -b`/`git switch -c` for a known IssueOps branch in the source checkout, and allows `git worktree add ../<repo>.worktrees/...` so the required sibling worktree can be created and linked. Branch creation commands must include an explicit source ref chosen by the user, such as `git switch -c 123-fix origin/main`; do not infer the current HEAD as the source. Outside active IssueOps code-editing phases it does not affect normal non-IssueOps work.

The guard is deterministic, but it only covers tool events that the host sends to PreToolUse. Keep the edit-target status checks above because some agent-side editing paths may not be represented as host hook events in every runtime.

## Numbered Next-Action Guard

Installed Codex and Claude Stop hooks include `--enforce-numbered-next-actions`. When the host provides `last_assistant_message` or a readable transcript path, the hook blocks final responses that do not include three numbered next-action choices. The hook reason tells the agent why it was blocked; the agent must then present context-specific choices.

Use this shape:

```text
선택지:
1. 진행: <recommended next action>. (추천)
2. 축소 진행: <narrower or lower-risk alternative>.
3. 보류: <pause/defer option and consequence>.
```

## Local Config And Dependency Links

`worktree prepare` and `worktree prepare-tools` are different commands: `worktree prepare` only reports the isolated worktree path and the next command — it does NOT satisfy any readiness gate. Only `worktree prepare-tools` records the `worktree_tools` evidence that gates implementation. "worktree exists" is not "worktree tools are prepared".

Run `agent-harness issueops worktree prepare-tools --id "$ISSUEOPS_ID" --json` after linking the plan and before implementation. For pnpm repositories it installs missing `node_modules` in the worktree with `pnpm install --frozen-lockfile --prefer-offline`, checks an existing CodeGraph index when one is available, and persists the result as `worktree_tools` on the IssueOps record. The `implement` phase remains blocked until durable dependency/worktree evidence, an approved compatibility review, and an execution decision are ready for the linked worktree; CodeGraph readiness is optional informational evidence.

When the worktree needs large generated dependency directories such as `node_modules` and `prepare-tools` cannot install them automatically, prefer reusing an existing dependency directory by symlink only after verifying the package manager, lockfile, platform, and dependency state match the source checkout.

```bash
test -d "$PWD/node_modules"
test -f "$PWD/package-lock.json" || test -f "$PWD/pnpm-lock.yaml" || test -f "$PWD/yarn.lock"
ln -s "$PWD/node_modules" "$worktree_path/node_modules"
```

Do not symlink dependency directories when the worktree uses a different lockfile, package manager, Node version, platform-specific native modules, or when installing fresh dependencies would be safer. Never commit generated dependency symlinks or dependency directories.

Local config files may be symlinked into the worktree when the task needs them and the source checkout already has the correct local-only configuration. Common candidates include `.env`, `.env.local`, `.mcp.json`, `dbhub.toml`, and other documented untracked local config files. Verify each candidate exists, is intended for local development, and is ignored or otherwise excluded from commits before linking it.

Do not symlink secret-bearing config into a worktree for review, PR/MR drafting, or artifact generation unless the command actually needs it. Never print config contents in prompts, logs, issue bodies, PR/MR bodies, or test output. If a config file is tracked, unignored, environment-specific, or contains credentials that are not needed for the task, stop and ask before linking it.

## Context Routing

Use CodeGraph as the default context layer for structural work, with `rg` as the fallback and exact-search tool.

- Start with CodeGraph for functions, classes, call relationships, dependency paths, impact analysis, module boundaries, and route/controller/service relationships.
- In an IssueOps worktree implementation phase, every CodeGraph call must set `projectPath` to the linked worktree path. Missing `projectPath` or a source-checkout `projectPath` is stale by construction and is blocked.
- Start with `rg` for exact strings: error messages, env keys, config values, filenames, TODOs, comments, logs, and literal function names.
- For natural-language feature location, use CodeGraph first, then run at least one targeted `rg` check before editing or claiming there are no usages.
- After edits, use `rg` plus relevant tests to catch missed references or regressions.
- Treat CodeGraph as advisory when its index may be stale or when the target uses dynamic wiring such as runtime DI, reflection, dynamic imports, or framework provider registration.
- Keep graph results small and targeted; oversized call/dependency graphs waste context.
