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

Assign the created remote issue to the currently authenticated user. For GitHub, resolve the login with `gh api user --jq .login` and pass it to `gh issue create --assignee "$login"` or apply it immediately with `gh issue edit "$ISSUE_URL" --add-assignee "$login"` before linking the issue. For GitLab, use the equivalent current-user assignee field supported by the target project's CLI/API.

Before creating or editing the remote issue, proactively score related issues and labels. Do not wait for the user to ask for this. Gather candidate issues and labels from the target provider, build an `issueops remote score` request, and apply only selected candidates whose score is at or above the threshold. The count is not fixed; the threshold decides the set.

Provider candidate gathering:

- GitHub: use `gh issue list --state all --limit N --json number,title,body,labels,url,state` and `gh label list --json name,description,color`.
- GitLab: use the equivalent `glab issue list`/GitLab API issue fields and project label list.

Run the scoring gate with the external LLM judge when available:

```bash
agent-harness issueops remote score --input issueops-remote-score.json --judge agy --json
```

Use the deterministic fallback only when the external LLM is unavailable or intentionally disabled:

```bash
agent-harness issueops remote score --input issueops-remote-score.json --judge none --json
```

Default threshold is `0.70` unless the repo or user sets a stronger threshold. Include selected related issue references/URLs in the issue body, include a compact scoring summary when it helps future reviewers understand why those links and labels were chosen, and apply selected labels with `gh issue create --label`/`gh issue edit --add-label` or the GitLab equivalent. Do not apply rejected labels or link rejected issues.

The agent must propose the operational choice instead of leaving the user to invent it. For example, after validating a need, offer: "관련 이슈/라벨 후보를 점수화하고 threshold 이상만 이슈 본문과 라벨에 반영하겠습니다. 기본은 agy judge, 실패 시 deterministic fallback으로 진행합니다."

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

## Remote Review Feedback

When creating or editing a PR/MR, assign it to the currently authenticated user before reporting that the PR/MR is ready. For GitHub, resolve the login with `gh api user --jq .login` and run `gh pr edit "$PR_URL" --add-assignee "$login"` immediately after `gh pr create`, then verify with `gh pr view "$PR_URL" --json assignees`. For GitLab, use the equivalent current-user assignee field supported by the target project's CLI/API and verify the resulting assignee list.

When handling remote PR/MR review feedback, first verify each reviewer claim against the diff, code, and commands before changing files. Apply only confirmed fixes, then reply in the original review thread with the commit and verification evidence.

The remote issue is the source of truth for IssueOps scope. If user feedback, review feedback, QA, CI evidence, or agent analysis changes the problem statement, acceptance criteria, non-goals, verification, implementation scope, related issue links, or labels, update the issue body before continuing. A thread/comment may record discussion, but it is not enough; the issue body must match the implementation contract. Run the Korean Remote Artifact Gate before every remote issue body edit.

When the user asks only for review-validity verification, do not end with a bare conclusion such as "the next step is to add tests and fix it." After the evidence-based verdict, explicitly present the available next actions so the user can choose or confirm direction. Include one recommended action, one narrower/safer alternative, and one stop/defer option when applicable. Example:

```text
검증 결론: 두 리뷰 모두 타당합니다.

선택지:
1. 진행: 테스트를 먼저 추가하고 두 결함을 수정합니다. (추천)
2. 축소 진행: target dimension 검증만 먼저 수정하고 path separator는 별도 PR로 분리합니다.
3. 보류: 현재 PR에는 수정하지 않고 리뷰 스레드에 검증 결과만 답변합니다.

제가 추천하는 건 1번입니다. 진행할까요?
```

For GitHub inline review comments, replying to a thread is not the same as resolving it. If branch protection requires conversation resolution, query review threads and resolve the fixed ones after the correction is pushed:

```bash
gh api graphql -f owner="$OWNER" -f repo="$REPO" -F number="$PR_NUMBER" -f query='query($owner:String!, $repo:String!, $number:Int!) { repository(owner:$owner, name:$repo) { pullRequest(number:$number) { reviewThreads(first:100) { nodes { id isResolved isOutdated path } } } } }'
gh api graphql -f threadId="$THREAD_ID" -f query='mutation($threadId:ID!) { resolveReviewThread(input:{threadId:$threadId}) { thread { id isResolved } } }'
```

Resolve only threads whose feedback has actually been addressed or is obsolete for a verified reason. After resolving, re-check `reviewThreads` and PR/MR readiness; do not claim merge blockage is cleared until unresolved required conversations are gone and the host reports a clean merge state.

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

Run the autoresearch keep/discard gate for IssueOps improvement candidates:

```bash
agent-harness issueops benchmark gate --baseline "$BASELINE_ID" --candidate "$CANDIDATE_ID" --candidate-file candidate.json --changed-path skills/issueops/SKILL.md --json
```

The candidate file records the hypothesis, target dimensions, edit surface, and keep/discard criteria. The gate keeps a candidate only when the candidate benchmark passes, baseline comparison has no regression, target dimensions do not regress, and every changed path is inside the declared edit surface.

Run the remote issue related-link and label scoring gate:

```bash
agent-harness issueops remote score --input issueops-remote-score.json --judge agy --json
agent-harness issueops remote score --input issueops-remote-score.json --judge none --json
```

All `agy -p` usage must go through the shared external LLM wrapper in the harness core. The wrapper invokes `agy --dangerously-skip-permissions -p <prompt>` so IssueOps gates do not block on permission prompts.

## Issue Template

Use this structure unless the target project already has a stronger issue template:

```markdown
## Problem

## Current Evidence

## Acceptance Criteria

## Non-goals


## Verification

## Feedback Log
```

## Korean Remote Artifact Gate

IssueOps가 원격에 생성하거나 수정하는 issue와 PR/MR 제목·본문은 한글 중심이어야 한다. 명령어, 코드 식별자, 파일 경로, URL, upstream/project 이름은 영어 원문을 유지할 수 있다.

IssueOps cycle에서 `gh issue create`, `gh issue edit`, `gh pr create`, `gh pr edit` 또는 GitLab equivalent를 실행하기 전에는 매번 다음 gate를 통과해야 한다.

1. 제목과 본문을 임시 파일 또는 heredoc으로 준비한다.
2. bundled language gate를 실행한다.

```bash
python3 skills/issueops/scripts/remote_artifact_gate.py --kind issue --title "$TITLE" --body-file "$BODY_FILE"
python3 skills/issueops/scripts/remote_artifact_gate.py --kind pr --title "$TITLE" --body-file "$BODY_FILE"
```

3. gate가 실패하면 원격 artifact를 생성하거나 수정하지 말고 한글 중심으로 다시 작성한다.

이 gate는 issue/PR/MR에 영어 section label, command output, code identifier, URL, 외부 project 이름이 포함되어도 반드시 실행한다.

원격 issue 본문에는 repo-local plan path를 넣지 않는다. plan 파일은 ignored/untracked일 수 있으므로 `agent-harness issueops link-plan` state와 PR/MR 본문에서 필요한 경우에만 추적한다.

## Stop Conditions

Stop and ask the user before creating or updating remote issues, PRs, or MRs if credentials, target project, branch target, or issue ownership are unclear.

Stop before implementation if brainstorming or grilling exposes materially different interpretations. Present the interpretations and ask for the intended one.

Do not move to PR/MR drafting when `issueops pr-readiness` reports missing `issue_url` or `plan_path`.
