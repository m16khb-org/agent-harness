# #51 Draft PR completion handoff plan

## Goal and fixed identity

Complete GitHub #51 on the existing `51-p0-safety-critical-fixes` branch and Draft PR #63. The owner works only in `/Users/habin/workspace/agent-harness.worktrees/51-p0-safety-critical-fixes`, preserves the existing plan commit `46375a3` and implementation commit `4d347dd`, and appends only evidence-backed fixes. Do not amend, rebase, force-push, merge, deploy, close the issue, or clean up the worktree.

## Reproduce before changing

1. Verify the worker path, branch, HEAD, upstream, clean status, IssueOps ownership, PR #63 base/head/Draft state, and current review threads.
2. Reproduce the two live CI failures independently:
   - tracked `.agent-harness/evidence/51-p0-safety-critical-fixes.md` violating `TestEvidenceAnswerFilesNotForceTracked`;
   - `TestExecGitRunnerAppliesLocalDeadline` failing with exit 127. Compare the focused test with the base branch/control before changing production code; treat an unreproduced environment-only failure as evidence, not permission to refactor.
3. Reproduce the Shannon awk-regex review finding with positive and negative fixtures that prove backslashes and matching semantics survive argv transport.

## Minimal closure

1. Remove the tracked evidence answer file from the branch; do not move its content into another tracked answer artifact.
2. Change operational-health code only if the focused reproduction proves a production cause. Otherwise make the smallest test-fixture/environment correction that preserves the local-deadline contract.
3. Change Shannon pattern transport only when the positive/negative fixture is RED first. Do not revisit the already implemented Engelbart or Torvalds changes unless a focused regression proves they are causal.
4. Run focused regression tests and skill validation, then `go test ./... -count=1`, `go test -race ./... -count=1`, `go build ./cmd/harness`, and inspect the final diff for unrelated or generated artifacts.
5. Create one append-only atomic commit, push normally to the same branch, preserve Draft PR #63 and its base branch, then verify the remote head, review threads, and CI checks.

## Ownership and stop conditions

- One owner session owns this branch, worktree, and IssueOps cycle after claim and context acknowledgement. The source session freezes only this exact cycle after dispatch and remains free for unrelated research and new cycles.
- Stop and report identity drift, unexpected dirty files, branch/base mismatch, a required history rewrite, destructive cleanup, merge/deploy/issue-close requests, or model/usage-reset prompts.
- Completion receipt must include final HEAD, changed files, exact commands and results, PR publication/readback, unresolved risks, and confirmation that merge/deploy/issue close/cleanup were not performed.
