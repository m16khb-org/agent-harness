---
name: issueops
description: Run an issue-driven work cycle from problem discovery through domain grilling, issue creation, planning, TDD/subagent implementation, feedback loops, and PR/MR drafting.
---

# IssueOps

Use this skill when the user wants a repeatable cycle from a vague problem to a GitHub/GitLab issue, implementation plan, tested change, feedback loop, and PR/MR.

This file is the phase router. Load only the referenced phase document needed for the current step.

## Core Contract

The workflow is advisory and agent-driven. Hooks may suggest this skill, but hooks must not create issues, edit files, run tests, wait on background jobs, or open PRs/MRs by themselves.

The cycle has one durable state record. Use `agent-harness issueops ... --json` or matching MCP tools when the cycle needs to survive compaction, handoff, or another host.

Required phases:

1. Problem intake: use `superpowers:brainstorming` to clarify the actual problem, constraints, success criteria, and ambiguity.
2. Domain grill: challenge terminology, existing domain model fit, and documentation updates before committing to an issue.
3. Issue contract: create or prepare a GitHub/GitLab issue with problem, acceptance criteria, non-goals, verification, and open decisions.
4. Plan: produce an issue-based implementation plan under the target repo's planning convention.
5. Implementation: use TDD for behavior changes and subagents only for bounded independent work.
6. Feedback loop: collect user, review, QA, and CI feedback; classify each item; update the issue/plan when the contract changes; then continue implementation.
7. PR/MR: draft only after the issue URL and plan path are linked and relevant verification has run.

## Reference Map

Load these files only when the phase applies:

- `references/remote-issue.md`: remote issue first, related issue/label scoring, external LLM judge contract, Korean remote artifact gate, issue template.
- `references/worktree-context.md`: branch/worktree contract, local config symlink rules, context routing.
- `references/review-feedback.md`: worker prompt requirements, bounded subagent review rules, remote review feedback replies and thread resolution.
- `references/cleanup-state.md`: post-merge cleanup, state commands, benchmark commands, stop conditions.

## Always-On Rules

- Remote issue first: when `$issueops` is explicitly invoked and repo remote, credentials, target project, branch target, and issue ownership are discoverable, create or link the remote issue before planning or implementation.
- Worktree first: after issue link and before implementation, create an isolated worktree under `../<repo>.worktrees/<branch-slug-with-slashes-replaced>` and run implementation from that path.
- Edit-target guard: shell cwd checks are not enough. Before any file edit, ensure the edit tool target path is inside the expected isolated worktree; after the edit, verify the source checkout/main branch remains clean and the worktree owns the change.
- State first: link the issue and plan in IssueOps state before PR/MR drafting.
- TDD first: for behavior changes, write or update focused tests before production changes.
- Verify before remote writes: run the Korean Remote Artifact Gate before creating or editing remote issues, PRs, or MRs.
- No broad review sweeps: subagent reviews must have explicit included paths, excluded large/generated paths, a time budget, and a fallback direct verification path.
- Cleanup choices: after a PR/MR is merged, verify merge/worktree/branch status and present numbered cleanup choices before deleting local worktrees or branches.
- Numbered next actions: at user decision points and after reporting review/feedback/cleanup status, end with `선택지:` and three numbered choices. Strict sessions may set `HARNESS_EXPECT_NUMBERED_NEXT_ACTIONS=1`; installed Stop hooks are strict-ready and can block missing choices.
- Worker identity check: every implementation, TDD, review, QA, or subagent worker must first report and verify `pwd`, branch, `HEAD`, and the expected isolated worktree path before inspecting or changing anything.
- Remote artifact ownership: created issues and PRs/MRs must be assigned to the currently authenticated user when the provider supports assignment, and assignment must be verified before reporting readiness.
- Remote issue source of truth: when feedback changes scope, acceptance criteria, non-goals, verification, labels, related links, or implementation contract, update the remote issue body before continuing.
- Review thread accountability: remote review feedback must be answered in the original review thread/discussion with verdict, evidence, and next action; do not report feedback cleared until addressed threads are replied to, resolved when appropriate, and re-checked.
- External LLM wrapper: all IssueOps `agy -p` usage must go through the shared harness external LLM wrapper and remain read-only judgment.

Use this remote issue scoring choice shape before creating or editing an issue:

```text
관련 이슈/라벨 후보를 점수화하고 threshold 이상만 이슈 본문과 라벨에 반영하겠습니다. 기본은 agy judge, 실패 시 deterministic fallback으로 진행합니다.
```

Use this review thread reply shape:

```text
타당성: 타당

근거:
- <파일:라인 또는 명령 결과 근거>
- <계약/테스트 근거>

다음 조치: <수정 진행|별도 PR 분리|보류 사유>
```

After posting review-thread replies, report numbered next actions:

```text
선택지:
1. 진행: 테스트를 먼저 추가하고 결함을 수정합니다. (추천)
2. 축소 진행: 일부 검증만 먼저 수정하고 나머지는 별도 PR로 분리합니다.
3. 보류: 현재 PR에는 수정하지 않고 리뷰 스레드에 검증 결과만 답변합니다.
```

Use this cleanup choice shape:

```text
선택지:
1. 정리 진행: merged PR/MR worktree와 local branch를 삭제합니다. (추천)
2. 보류: worktree는 유지하고 나중에 확인합니다.
3. 확장 정리: merged/stale IssueOps worktree 전체를 점검하고 정리 후보를 제시합니다.
```

## Background LLM Gates

Remote scoring is a `background_join` LLM gate. It may run while local planning or implementation continues, but the main IssueOps loop must join the result before any remote artifact write: issue create/edit, label create/apply, PR/MR create/edit, assignment, or comment.

Do not put polling or waiting in lifecycle hooks. Hooks may surface a status hint only. Completion is decided by the main loop at the join point by checking the stored job/result status and requiring success before the remote write.

External LLM judges are read-only evaluators. Their prompts must forbid workspace inspection, tool execution, file changes, git actions, issue/label/PR/MR mutation, comments, assignment, closing/reopening, or state changes. They may only return judgment JSON that the main loop applies after validation.

## Operational Start

Start or resume state:

```bash
agent-harness issueops start --repo "$PWD" --branch "$(git branch --show-current)" --json
agent-harness issueops status --id "$ISSUEOPS_ID" --json
```

Remote issue and plan linkage:

```bash
agent-harness issueops link-issue --id "$ISSUEOPS_ID" --issue-url "$ISSUE_URL" --json
agent-harness issueops link-plan --id "$ISSUEOPS_ID" --plan-path "$PLAN_PATH" --json
agent-harness issueops pr-readiness --id "$ISSUEOPS_ID" --json
```

Record feedback:

```bash
agent-harness issueops feedback add --id "$ISSUEOPS_ID" --source user --body "$FEEDBACK" --json
```

## Stop Conditions

Stop and ask before creating or updating remote issues, PRs, or MRs if credentials, target project, branch target, or issue ownership are unclear.

Stop before implementation if brainstorming or grilling exposes materially different interpretations. Present the interpretations and ask for the intended one.

Do not move to PR/MR drafting when `issueops pr-readiness` reports missing `issue_url` or `plan_path`.
