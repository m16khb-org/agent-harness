# Issue 1 Live IssueOps Audit Plan

## Goal

Verify the live IssueOps lifecycle with a real GitHub issue, branch, worktree, PR, label, and assignee.

## Steps

1. Link the real issue, provider branch, isolated worktree, and this plan.
2. Add a tiny audit-only file so the PR path has a real implementation diff.
3. Run strict readiness gates before and after AI slop cleanup.
4. Create a temporary PR with `bug` label and `m16khb` assignee, verify it through IssueOps, then close and clean it.

## Verification

- `agent-harness issueops pr-readiness --strict`
- `agent-harness issueops remote verify-artifact`
- `agent-harness issueops cleanup status`
