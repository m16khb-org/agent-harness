# IssueOps Cleanup And State Commands

## Post-Merge Cleanup

After a PR/MR is merged, do not stop at reporting the merge. Verify merge status, remote branch status, and worktree cleanliness, then explicitly offer numbered cleanup choices to the user.

When merging an IssueOps PR/MR from an isolated feature worktree, keep provider merge and branch/worktree cleanup as separate phases. Do not use merge commands that also try to delete local branches from the feature worktree. For GitHub, avoid:

```bash
gh pr merge "$PR_NUMBER" --merge --delete-branch
```

Merge first without local cleanup:

```bash
gh pr merge "$PR_NUMBER" --merge
```

Then run post-merge verification. For GitHub:

```bash
gh pr view "$PR_NUMBER" --json state,mergedAt,headRefName,url
git -C "$WORKTREE_PATH" status --short --branch
remote_name="$(git -C "$WORKTREE_PATH" remote | head -n 1)"
git -C "$WORKTREE_PATH" fetch "$remote_name" --prune
git -C "$WORKTREE_PATH" ls-remote --heads "$remote_name" "$HEAD_REF_NAME"
```

For GitLab, use equivalent `glab mr view` or GitLab API fields for merged state and source branch, then run the same local worktree and remote branch checks.

After provider merge evidence is confirmed, run the harness cleanup status check. It is read-only: it does not remove branches or worktrees. It blocks cleanup readiness when merge evidence is missing, the worktree is dirty, the worktree branch does not match the IssueOps branch, the remote artifact was not verified, or the remote source branch still exists.

```bash
agent-harness issueops cleanup status --id "$ISSUEOPS_ID" --merged --json
```

Present cleanup choices in `1.`, `2.`, `3.` form:

```text
선택지:
1. 정리 진행: merged PR/MR worktree와 local branch를 삭제합니다. (추천)
2. 보류: worktree는 유지하고 나중에 확인합니다.
3. 확장 정리: merged/stale IssueOps worktree 전체를 점검하고 정리 후보를 제시합니다.
```

Only run cleanup after the user chooses the proceed option or has explicitly instructed automatic cleanup. Before deleting, verify the target worktree is clean and the PR/MR is merged.

```bash
git push "$remote_name" --delete "$HEAD_REF_NAME"  # only when the remote source branch still exists and should be removed
git worktree remove "$WORKTREE_PATH"
git branch -d "$BRANCH_NAME"
```

Do not use an unconditional fallback such as `git branch -d "$BRANCH_NAME" || git branch -D "$BRANCH_NAME"`. If `git branch -d` fails after the PR/MR is verified merged and the worktree is clean, report the reason and offer numbered choices.

If the worktree is dirty, the PR/MR is not merged, the remote source branch still exists unexpectedly, or the branch contains unmerged commits that are not explained by a verified squash/rebase merge, do not force-remove. Report the blocker and offer numbered choices.

## State Commands

Start:

```bash
agent-harness issueops start --repo "$PWD" --branch "$(git branch --show-current)" --json
```

Record the intent contract:

```bash
agent-harness issueops intent record --id "$ISSUEOPS_ID" \
  --raw-request "$RAW_USER_REQUEST" \
  --interpreted-intent "$INTERPRETED_INTENT" \
  --success-criteria "$SUCCESS_CRITERION" \
  --json
```

Link the issue:

```bash
agent-harness issueops link-issue --id "$ISSUEOPS_ID" --issue-url "$ISSUE_URL" --json
```

Record branch and worktree evidence:

```bash
agent-harness issueops branch prepare --id "$ISSUEOPS_ID" --provider "$PROVIDER" --issue-url "$ISSUE_URL" --branch "$BRANCH" --base-branch "$BASE_BRANCH" --link-verified --json
agent-harness issueops link-worktree --id "$ISSUEOPS_ID" --worktree-path "$EXPECTED_WORKTREE" --json
```

Record the approved design review:

```bash
agent-harness issueops design review --id "$ISSUEOPS_ID" \
  --problem-summary "$PROBLEM_SUMMARY" \
  --proposed-design "$PROPOSED_DESIGN" \
  --verification "$VERIFICATION_STEP" \
  --approved \
  --json
```

Link the plan:

```bash
agent-harness issueops link-plan --id "$ISSUEOPS_ID" --plan-path "$PLAN_PATH" --json
```

State recovery must preserve the redacted intent contract and design review fields. Do not reconstruct them from hook recommendations alone; re-record main-agent judgment when the saved state is missing those sections.

Record feedback:

```bash
agent-harness issueops feedback add --id "$ISSUEOPS_ID" --source user --body "$FEEDBACK" --json
agent-harness issueops feedback mark-issue-updated --id "$ISSUEOPS_ID" --json
```

Check PR/MR readiness:

```bash
agent-harness issueops pr-readiness --id "$ISSUEOPS_ID" --strict --json
```

## Benchmark Commands

Run the 100-point quality benchmark:

```bash
agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json
agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge agy --json
```

The benchmark passes only when every fixture has `average_score: 100`, `minimum_score: 100`, and `critical_failure_count: 0`. Use `--judge agy` for the real LLM gate when quota is available; use `--judge none` only for deterministic local evidence.

Run the autoresearch keep/discard gate for IssueOps improvement candidates:

```bash
agent-harness issueops benchmark gate --baseline "$BASELINE_ID" --candidate "$CANDIDATE_ID" --candidate-file candidate.json --changed-path skills/issueops/SKILL.md --json
```

The candidate file records the hypothesis, target dimensions, edit surface, and keep/discard criteria. The gate keeps a candidate only when the candidate benchmark passes, baseline comparison has no regression, target dimensions do not regress, and every changed path is inside the declared edit surface.

All `agy -p` usage must go through the shared external LLM wrapper in the harness core. The wrapper invokes `agy --dangerously-skip-permissions -p <prompt>` so IssueOps gates do not block on permission prompts.
