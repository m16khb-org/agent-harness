---
name: gitlab-usecase
description: Use before GitLab, glab, GitLab MCP, IssueOps remote issue/MR, linked-item, child-item, work-item, Kody/Kodus review, issue-body update, branch-target, or cleanup work. Prevents repeated GitLab mistakes by distinguishing native linked items from child items, enforcing body-of-record rules, authenticated CLI/MCP fallback, labels/assignee/target-branch checks, and review-thread verification.
---

# GitLab Usecase

Use this skill before GitLab work starts. It is a guardrail for repeated mistakes in GitLab IssueOps, MRs, work items, review feedback, and cleanup.

## Start Protocol

1. Identify the exact GitLab object type before acting:
   - Issue: `/issues/:iid`
   - Work item / Task: `/work_items/:iid`
   - MR: `/merge_requests/:iid`
   - Native linked item: non-hierarchical relation such as `relates_to`, `blocks`, `is_blocked_by`
   - Child item: parent-child hierarchy, not a link relation
2. Pick the authenticated surface whose credentials and scope match the target GitLab host and project before the first call: generic `glab` CLI, or a configured GitLab MCP tool discovered by capability.
3. Do not report completion from local state alone. Re-read the remote issue/MR/work item after mutation.

## Portable VCS Snapshot Discovery

IssueOps execution이 GitLab issue 본문을 봉인해야 할 때는 다음 순서를 지킨다.

1. canonical worktree가 있으면 먼저 선택 문서 `.agent-harness/VCS.md`를
   `project_docs_read`로 읽는다. 문서가 없거나 해당 provider recipe가 없으면 현재
   host에 등록된 trusted tool을 탐색한다.
2. GitLab MCP는 서버 이름이 아니라 semantic leaf `glab_api`와 실제 input schema로
   식별한다. server namespace는 등록 세부사항이지 capability identity가 아니다.
   같은 leaf와 호환 schema를 노출하는 개인 wrapper도 지원한다. 특정 wrapper 이름,
   설치 경로, profile, token을 코드나 공용 문서에 고정하지 않는다.
3. 실제 schema에 맞춰 `projects/<URL-escaped-project>/issues/<iid>`를 읽고,
   schema가 지원하면 `flags.hostname`으로 target host를 명시해 `web_url`,
   `description`, `state`를 받는다. 여러 후보가 있으면 target host/project 권한과
   schema가 맞는 후보만 사용한다.
4. 응답의 HTTPS authority(명시 port 포함), project path, IID를 linked issue와 정확히
   대조한다. `/issues/:iid`와 `/work_items/:iid`는 이 세 값이 모두 같을 때만 같은
   identity다.
5. 성공한 MCP 응답은
   `provider=gitlab`, `source=glab_mcp`, `web_url`, `body=description`,
   `state=opened|closed`의 host-neutral `issue_snapshot`으로 정규화한다. MCP
   `issueops_execution`에는 이 객체를 직접 넘기고, CLI로 IssueOps를 호출할 때는
   mode `0600` private JSON file과 `--issue-snapshot-file`을 사용한다.
6. 후보 부재뿐 아니라 auth/permission/transport/schema 호출 실패 뒤에도
   successful exact-identity MCP evidence를 얻지 못했을 때만 `issue_snapshot`을
   생략한다. 그러면 agent-harness provider adapter가 일반 `glab api` CLI를
   사용하고 결과에 `glab_cli`를 기록한다.
   이미 공급한 invalid evidence는 CLI fallback하지 않고 fail-closed한다.
7. 실제로 성공한 provider recipe는 canonical worktree에서 `project_docs_read`로
   최신 content/SHA를 다시 읽고 `project_docs_update`의 SHA-CAS로
   `.agent-harness/VCS.md`에 기록한다. tool leaf, 관찰한 schema, endpoint/필드,
   CLI fallback만 남기고 secret과 개인 server namespace는 남기지 않는다.
   canonical worktree가 아직 없으면 생성 뒤로 기록을 미룬다. OpenWiki 자동 update를
   실행하지 않는다.
8. `.agent-harness/VCS.md`는 provider-neutral하다. GitHub repo에서는 검증된
   `gh issue view <url> --json url,body,state`를 기록하거나 실제로 관찰한 MCP
   schema만 기록한다. 존재하지 않는 GitHub MCP 이름을 추측해서 만들지 않는다.

## Profile-Scoped glab MCP Servers (when configured)

Some environments expose `glab` through profile-scoped MCP servers, where each server pins one GitLab host, one token, and one default project workdir (server selection = target project + credential declaration). In that setup:

- Before any call, confirm the server profile matches the task's target project. A wrong-profile call is not always an error — it can silently succeed against the wrong project or fail with a misleading 404.
- Do not rely on repo autodetection. Without an explicit `-R <group/project>` or explicit API endpoint, `glab` resolves the repo from the current session's working directory, not the profile workdir — in a non-GitLab checkout it errors ("none of the git remotes correspond"), and in a different GitLab checkout it silently targets that repo. Verified 2026-07-09 against profile-scoped servers.
- Profile tokens are often project-scoped bot tokens. A 404 on another project usually means token scope, not a missing project; switch to the matching profile instead of forcing `-R`.
- Do not switch profiles mid-task without re-verifying the target object (issue/MR/work item) on the new profile.
- These MCP tools run the same `glab` CLI under the profile's credentials; CLI-vs-MCP is a credential/scope choice, not a capability fallback order.

## Non-Negotiable Distinctions

| Need | Correct GitLab surface | Wrong surface |
| --- | --- | --- |
| Related issue | Native linked item through issue links API | Body-only URL, child item |
| Work breakdown | Child `Task` work item through work-item hierarchy | `relates_to`, issue link, sibling issue |
| Parent task index | Parent issue body `## 하위 Task` plus real hierarchy | Comments-only checklist |
| Existing child proof | Work-item hierarchy / parent widget | REST `has_tasks`, body reference, linked item |

For IssueOps child tasks, use `agent-harness issueops remote create-child --confirm --json` when creating new children. It must create a GitLab `Task` work item, attach it with `workItemHierarchyAddChildrenItems`, verify hierarchy/labels/assignees, and record the child. Use `link-child` only for an already existing provider-native child that was verified separately.

Do not create child work by calling `glab api projects/:id/issues/:iid/links`, `glab issue link`, or `issueops link-related`. Those create or record linked items, not child items.

If a Task is already a linked item and must become a child item, remove the linked relation first, then set/attach the parent hierarchy, then verify by GraphQL/work-item hierarchy.

## Remote Artifact Rules

- GitLab related issues belong in native linked items, not a `Related Issues` body section.
- The GitLab issue body is the source of truth for scope, acceptance criteria, child task list, and durable evidence. If the user says not to comment, update the issue body.
- For long issue-body updates, fetch the current description, patch it additively, submit with file-backed description input when possible, then re-read and verify.
- Remote issue/MR text should stay Korean-centered except code identifiers, branch names, commands, and URLs.

## Branch And MR Rules

- Treat user-named branches such as `release/stg`, `development`, parent issue branches, and child branches as part of the contract.
- Child MR targets the parent issue branch, not the default branch, unless the user explicitly says otherwise.
- Copy or pass labels and assign the MR/issue to the concrete current user. Do not use `@me`.
- For `glab mr for`, use a numeric assignee id.

## Review And Cleanup Rules

- For Kody/Kodus/Gemini review feedback, inspect the real discussion thread, reply in-thread with evidence, then re-fetch discussions and resolve every remaining `resolvable=true` and `resolved=false` note.
- A merged MR does not prove the child Task is closed. Verify the Task state after merge and close it explicitly when needed.
- Cleanup sequence: verify MR merged, verify remote/source branch state, verify child Task state, verify worktree cleanliness, then remove worktree/local branch.

## Stop Conditions

Stop and ask or switch to GitLab MCP/API when:

- local `glab` or a glab MCP profile returns `401`, `404`, missing scope, or permission errors;
- the resolved project of the auth surface does not match the task's target project (wrong profile/workdir/token);
- linked item and child item state disagree;
- target/base branch does not match the recorded parent branch;
- issue body update would overwrite unknown current content;
- review thread state cannot be re-read after a reply.
