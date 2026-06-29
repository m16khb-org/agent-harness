# IssueOps Remote Issue And Artifact Rules

## Remote Issue First

When the user explicitly invokes `$issueops` and the repo remote, credentials, target project, branch target, and issue ownership are discoverable, create the remote GitHub/GitLab issue before planning or implementation. Then immediately link it with `agent-harness issueops link-issue`.

Assign the created remote issue to the currently authenticated user. For GitHub, resolve the login with `gh api user --jq .login` and pass it to `gh issue create --assignee "$login"` or apply it immediately with `gh issue edit "$ISSUE_URL" --add-assignee "$login"` before linking the issue. For GitLab, resolve the concrete current user first with `glab api user --jq .username` for normal issue/MR create commands, or `glab api user --jq .id` for issue-based `glab mr for`, then pass that value through the supported assignee flag. Do not use placeholders such as `@me`.

Before creating or editing the remote issue, proactively score related issues and labels. Gather candidate issues and labels from the target provider, build an `issueops remote score` request, and apply only selected candidates whose score is at or above the threshold. The count is not fixed; the threshold decides the set.

Provider candidate gathering:

- GitHub: use `gh issue list --state all --limit N --json number,title,body,labels,url,state` and `gh label list --json name,description,color`.
- GitLab: use the equivalent `glab issue list`/GitLab API issue fields and project label list.

Always inspect the current issue list and label list before deciding whether to create a new issue, link an existing issue, or create/update labels. If an existing issue already matches the requested work, link or update that issue instead of creating a duplicate. If the scoring gate selects a label that does not exist on the provider, create the missing label before issue creation or issue edit, then apply it. Do not create labels that were not selected by the scoring gate.

Run the scoring gate with the external LLM judge when available:

```bash
agent-harness issueops remote score --input issueops-remote-score.json --judge llm --model glm-5-turbo --json
```

Use deterministic fallback only when the external LLM is unavailable or intentionally disabled:

```bash
agent-harness issueops remote score --input issueops-remote-score.json --judge none --json
```

Default threshold is `0.70` unless the repo or user sets a stronger threshold. Attach selected related issues with the provider-native mechanism described in "Provider-Specific Linking And Hierarchy" below (GitHub body references vs GitLab linked items) — do not reuse one provider's style for the other. Include a compact scoring summary when it helps future reviewers understand why those links and labels were chosen, and apply selected labels with provider CLI/API commands. Do not apply rejected labels, create rejected labels, or link rejected issues. If label candidates existed but none met threshold, do not create an unlabeled remote artifact; stop before remote writes and either rerun scoring with corrected candidates or choose an explicit manual label with the reason recorded in IssueOps feedback.

The scoring summary is the **threshold-based label decision**. It must name selected labels, rejected labels, and manual override reason if the agent chooses or applies a label outside the scorer's selected set. A manual override is allowed only when the reason is evidence-backed, recorded in the issue draft or IssueOps feedback, and still passes the Korean Remote Artifact Gate before the remote write.

The agent must propose the operational choice instead of leaving the user to invent it. Example:

```text
관련 이슈/라벨 후보를 점수화하고 threshold 이상만 이슈 본문과 라벨에 반영하겠습니다. 기본은 Z.AI `glm-5-turbo` LLM judge, 실패 시 deterministic fallback으로 진행합니다.
```

Only prepare a local issue draft instead of creating a remote issue when credentials, target provider, ownership, or branch target are unclear, or when the user explicitly asks not to create a remote issue.

If the agent realizes it implemented before creating or linking the issue, it must stop implementation, create or link the issue if possible, record corrective feedback in IssueOps state, and resume from the issue-linked plan.

## Issue Template

Use this structure unless the target project already has a stronger issue template:

```markdown
## 문제

## 현재 근거

## 관련 이슈/라벨 판단

## 완료 기준

## 비목표

## 구현 범위

## 검증

## 위험과 트레이드오프

## 피드백 기록
```

Bug issues additionally include `재현 절차`, `기대 동작`, `실제 동작`, `환경`, and `로그/출력`. Child tasks use the narrower provider-native child contract: `부모 이슈`, `작업 목표`, `완료 기준`, `비목표`, `검증`, `부모 브랜치 병합 조건`, `child-only cleanup 규칙`.

Prefer the shared renderer/validator when preparing a remote body:

```bash
agent-harness issueops remote render-template --kind issue --template implementation_task --provider github --title "$TITLE" --field problem="$PROBLEM" --json
agent-harness issueops remote create-issue --id "$ISSUEOPS_ID" --template implementation_task --field problem="$PROBLEM" --label "$LABEL" --assignee "$ASSIGNEE" --json
agent-harness issueops remote create-child --id "$ISSUEOPS_ID" --template child_task --field parent_issue="$PARENT_URL" --field goal="$GOAL" --label "$LABEL" --assignee "$ASSIGNEE" --json
agent-harness issueops remote create-pr --id "$ISSUEOPS_ID" --template pull_request --label "$LABEL" --assignee "$ASSIGNEE" --json
```

`--body` and `--body-file` are mutually exclusive. If `--template` is set without a body, the core renderer produces the body. If `--template` is set with a body or body file, the body is validated against the canonical section policy. Confirmed remote writes fail closed on critical validation failures, missing label, missing assignee, Korean artifact failure, and PR/MR target/base branch mismatch.

`--field` accepts canonical template keys and these documented aliases:

| Template scope | Alias | Canonical key |
| --- | --- | --- |
| Any | `goal` | `task_goal` |
| Any | `parent_merge` | `merge_condition` |
| Bug issue | `logs_output` | `logs` |
| PR/MR | `automation` | `automation_evidence` |
| PR/MR | `cleanup` | `worktree_cleanup` |
| PR/MR | `docs`, `document`, `documents`, `documentation` | `docs_migration` |
| PR/MR | `risk`, `risks`, `rollback` | `risk_rollback` |
| PR/MR | `scope` | `scope_management` |

Do not add a `## Plan Link` / `## Plan` section or a `TBD` placeholder to the remote issue body. Plan tracking lives in `agent-harness issueops link-plan` state and, when needed, the PR/MR body — never as an issue-body section (see the Korean gate's plan-path rule below).

## Provider-Specific Linking And Hierarchy

For GitLab remote work, use the `gitlab-usecase` skill before creating or mutating issues, linked items, child items, MRs, review discussions, or cleanup state.

GitHub and GitLab expose similar concepts through different mechanisms. Never apply one provider's mechanism to the other; detect the provider first, then use its native feature. Use the authenticated provider CLI first (`gh api` for GitHub, `glab api` for GitLab); if auth, token scope, or permission errors block the CLI path in a host that provides a VCS MCP fallback, use that MCP fallback and verify the same remote fields afterward.

| Concept | GitHub mechanism | GitLab mechanism |
| --- | --- | --- |
| Related / non-hierarchical link | Cross-reference in the issue body (`#123` or full URL). GitHub has no native "linked items" relation, so body references are correct. | Native **linked items** (relation), not a body section. Create with `glab api projects/:id/issues/:iid/links -X POST -f target_project_id=<id> -f target_issue_iid=<iid> -f link_type=relates_to` (`relates_to` \| `blocks` \| `is_blocked_by`). |
| Parent → child work breakdown (tasks) | **GitHub: create sub-issues**. Prefer current `gh` support: `gh issue create --parent <parent>` for new children and `gh issue edit <parent> --add-sub-issue <child>` when attaching an existing issue. If that path is unavailable or lacks permission, fall back to `gh api` against the repository sub-issues endpoint and verify the parent sub-issues list afterward. | **GitLab: create child `Task` work items**. Use `glab api` against the GitLab work-items hierarchy for the parent issue/work item. The hierarchy must create child Task work items under the parent; ordinary sibling issues, `relates_to` links, and body-only task lists are not enough. |
| Labels | `gh issue create --label` / `gh issue edit --add-label`. | `glab issue create --label` / GitLab issue labels field. |
| Assignee | `gh issue create --assignee` / `gh issue edit --add-assignee`. | GitLab concrete username for normal create, numeric current-user id for `glab mr for`; never `@me`. |

Rules:

- When the scoring gate selects related issues, attach them as **GitLab linked items** on GitLab and as **body cross-references** on GitHub. Do not put a `## Related Issues` body section on GitLab when a linked item is the correct home; do not invent a linked-items relation on GitHub where none exists.
- When breaking work into tasks/subtasks on the remote, add them as **GitHub sub-issues** or **GitLab child items** for the parent issue — match the provider. Prefer `agent-harness issueops remote create-child --id "$ISSUEOPS_ID" --title "$CHILD_TITLE" --body "$CHILD_BODY" --label "$LABEL" --assignee "$ASSIGNEE" --confirm --json`; it creates, attaches, verifies, and records the child link in IssueOps state. Use `agent-harness issueops link-child --id "$ISSUEOPS_ID" --child-url "$CHILD_ISSUE_URL" --title "$CHILD_TITLE" --json` only for an already-created provider-native child that was verified separately. GitLab child URLs may be `/-/work_items/:iid` Task URLs, not only `/-/issues/:iid`. Do not flatten a hierarchy into plain `relates_to` links or body bullet lists when the provider supports a real parent/child relation.
- GitLab child Task creation is not a trial-and-error REST issue flow. `remote create-child --confirm` must use this sequence: resolve the `Task` `workItemTypeId`, label IDs, and assignee IDs; run GraphQL `workItemCreate`; look up the parent issue work-item ID; run `workItemHierarchyAddChildrenItems`; then verify hierarchy, labels, and assignees. Do not fall back to the REST Issues API `issue_type=task` path or ordinary `glab issue create` when GraphQL creation or hierarchy attachment fails. A REST issue response with `issue_type=task` may be used only as verification evidence for an already-existing child URL; GraphQL failure is a blocker before reporting child creation complete.
- Before creating child tasks, classify the child execution graph. Every child title, child task body, and parent child-task section must use a mandatory `[p]` or `[s]` prefix and say whether the child is `[p] parallelizable` or `[s] sequential`, list prerequisites/dependencies (`none` when there are none), and place the child in execution waves. Use `[p] parallelizable` only for work that can start without another child task's output and can be verified independently. Use `[s] sequential` when the task depends on another child's code, schema, migration, remote state, fixture, or decision output. If the dependency graph is unclear, stop for a user or owner decision before remote writes.
- When child work is merged into its parent work branch, close only the linked child task with `issueops cleanup close-children --merged --confirm`; leave the parent issue open as the umbrella until the full parent scope is merged to the mainstream target.
- After creating child tasks, verify the provider-native hierarchy, labels, assignee, and parent body update before reporting `parent body updated`. GitHub verification should inspect the parent sub-issues list and each child issue's labels/assignees. GitLab verification should inspect the parent work-item children/Tasks list and each child work item's labels/assignees.
- Run the **Large Issue Breakdown Gate** before implementation with a default decision of no split. Create provider-native child work items only when one issue would be unsafe because it would hide genuinely large/risky independent delivery boundaries, or when the user/owners explicitly requested for collaboration, parallel ownership, or separate assignees. Supporting signals are not sufficient by themselves: independent acceptance criteria, multiple modules, rollout steps, and long task lists only justify a split when they prove one of those primary triggers. The gate requires provider-native child work items or an explicit non-split reason, and every created child must be recorded with `link-child`.
- When creating a PR/MR, copy labels from the linked issue into the provider create command. If the linked issue is unlabeled, apply an explicit manual label to the issue first or stop and record why no label can be chosen; do not create an unlabeled PR/MR. Label-copy flags such as `--copy-issue-labels` or GitLab issue-based MR flags such as `--with-labels` satisfy only the label requirement; the create command must still include an assignee flag for the current user.
- If a provider mechanism is unavailable (API/permission/feature flag), say so explicitly, fall back to the closest documented mechanism, and record the limitation in IssueOps feedback rather than silently using the other provider's style.

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

Installed PreToolUse hooks include `--enforce-korean-remote-artifacts`. The hook blocks `gh issue create/edit` and `gh pr create/edit` when it can inspect `--title` plus `--body-file` or `--body` and the text fails the same Korean language threshold. If the title/body is not inspectable, prepare the artifact in a body file, run this gate explicitly, then retry the remote command.

Installed PreToolUse hooks also include `--enforce-vcs-issue-linking`. It inspects the same body (gh `--body`/`--body-file`, glab `--description`) and blocks issue create/edit when the body contains a `Plan Link` section (on any provider) or a `Related Issues` section on GitLab (where related issues belong in native linked items per the table above). It also blocks issue/PR/MR create commands when no label flag or assignee flag is inspectable, including deprecated GitLab issue-based MR commands such as `glab mr for` and structured MCP `glab_mr_for`. When an active IssueOps cycle has `branch_prepare.base_branch`, PR/MR create must also expose a target/base branch matching that recorded parent work branch; missing or mismatched `gh --base` / `glab --target-branch` is blocked. `--copy-issue-labels` and `--with-labels` count as label evidence for PR/MR create, including structured MCP-style tool input such as `flags.copy_issue_labels: true` or `flags.with_labels: true`, but they do not imply assignment; pass the current authenticated user through an assignee flag or structured field such as `--assignee`, `assignee`, or `flags.assignee`. GitLab placeholders such as `@me` are rejected, and `glab mr for` requires a numeric assignee id. Attach GitLab related issues with the issue links API, drop the Plan Link section, and include copied or explicit labels plus the current authenticated user assignee before retrying.

원격 issue 본문에는 repo-local plan path를 넣지 않는다. plan 파일은 ignored/untracked일 수 있으므로 `agent-harness issueops link-plan` state와 PR/MR 본문에서 필요한 경우에만 추적한다.
