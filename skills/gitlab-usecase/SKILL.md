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
2. Use authenticated `glab`/GitLab API first. If local `glab` returns auth, permission, or project errors, use the configured GitLab MCP/API fallback and verify the same fields there.
3. Do not report completion from local state alone. Re-read the remote issue/MR/work item after mutation.

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

- local `glab` returns `401`, `404`, missing scope, or permission errors;
- linked item and child item state disagree;
- target/base branch does not match the recorded parent branch;
- issue body update would overwrite unknown current content;
- review thread state cannot be re-read after a reply.
