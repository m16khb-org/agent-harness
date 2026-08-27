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

After provider merge evidence is confirmed, run the harness cleanup status check. It is read-only: it does not remove branches or worktrees. It blocks cleanup readiness when merge evidence is missing, the worktree is dirty, the worktree branch does not match the IssueOps branch, the remote artifact was not verified, or the remote source branch still exists. Processes occupying the worktree and Orca terminals bound to it are not blockers: status reports them as warnings (`pid:command:started_at` plus "apply가 프로세스 N개와 Orca 터미널 M개를 종료합니다") because the typed apply stops them itself. Only the requester's own occupancy/terminal (`requester_occupies_worktree`, `requester_terminal_outside_worktree`, `requester_terminal_unresolved`), the source checkout (`worktree_is_source_checkout`), and observation failures (`workspace_processes_observable`, `orca_terminals_observable`, `orca_runtime_ready`) still block.

```bash
agent-harness issueops cleanup status --id "$ISSUEOPS_ID" --merged --json
```

### Recordless merged orphan worktree

When a merged PR/MR worktree has no IssueOps record, do not create a replacement
lifecycle and do not call `git worktree remove` directly. Use the typed orphan
cleanup preview with the exact source repository, worktree, local branch, and
remote artifact identity:

```bash
agent-harness issueops cleanup orphan \
  --id "$ORPHAN_ID" --repo "$REPO_ROOT" --worktree "$WORKTREE_PATH" \
  --branch "$BRANCH_NAME" --provider github --kind pr \
  --artifact-url "$PR_URL" --json
```

The default is read-only. It re-reads Git/IssueOps/lease/Orca inventory and
provider merge evidence, requires the target exactly once in inventory, rejects
the canonical worktree, dirty or mismatched targets, and fails closed for any
record, lifecycle/lease, or Orca authority. A ready result includes `head_sha`,
a recovery path, and a fingerprint.

After the preview is ready and the user has explicitly approved the exact
target, apply the same request with its fingerprint:

```bash
agent-harness issueops cleanup orphan \
  --id "$ORPHAN_ID" --repo "$REPO_ROOT" --worktree "$WORKTREE_PATH" \
  --branch "$BRANCH_NAME" --provider github --kind pr \
  --artifact-url "$PR_URL" --apply --confirm --fingerprint "$FINGERPRINT" --json
```

Apply re-verifies the complete preview and removes only the local worktree and
local branch with the preview HEAD as a CAS. It never creates IssueOps state and
never deletes the remote branch; remote deletion is a separate explicit approval
boundary.

If the IssueOps record has linked child tasks, cleanup status also requires verified child-close evidence. After a child PR/MR has been verified merged into the parent work branch, close only the linked child tasks. Do not close the parent issue at this step; the parent remains the umbrella coordination issue until the full umbrella is merged to the mainstream target such as main or release.

Dry-run child cleanup first:

```bash
agent-harness issueops cleanup close-children --id "$ISSUEOPS_ID" --merged --json
```

Execute only after the dry-run matches the intended linked children:

```bash
agent-harness issueops cleanup close-children --id "$ISSUEOPS_ID" --merged --confirm --json
```

Provider behavior:

- GitHub verifies the child is still listed in the parent sub-issues, closes the child issue with completed reason, then re-reads the child state.
- GitLab verifies the child `Task` work item is still in the parent hierarchy, runs `workItemUpdate(stateEvent: CLOSE)`, then re-reads `state == CLOSED`.
- Already-closed child tasks count as successful verification and record close evidence in IssueOps state.

Present cleanup choices in `1.`, `2.`, `3.` form:

```text
선택지:
1. 정리 진행: merged PR/MR worktree와 local branch를 삭제합니다. (추천)
2. 보류: worktree는 유지하고 나중에 확인합니다.
3. 확장 정리: merged/stale IssueOps worktree 전체를 점검하고 정리 후보를 제시합니다.
```

Only run recordless orphan cleanup after the user chooses the proceed option or has explicitly instructed automatic cleanup. The typed `issueops cleanup orphan --apply --confirm --fingerprint` path is its only local mutation owner; direct `git worktree remove` remains hook-blocked. Do not combine it with remote branch deletion.

For a recordless orphan, do not use an unconditional raw fallback such as `git branch -d "$BRANCH_NAME" || git branch -D "$BRANCH_NAME"`. If the typed apply cannot remove the local branch with its preview HEAD CAS, report the reason and rerun preview; do not force-delete it.

For ordinary record-backed cleanup status, if the worktree is dirty, the PR/MR is not merged, the remote source branch still exists unexpectedly, or the branch contains unmerged commits that are not explained by a verified squash/rebase merge, do not force-remove. Report the blocker and offer numbered choices. Do not stop occupying processes or Orca terminals by hand to satisfy a preview: the apply step owns that (`workspace_processes_stop`), binds it to the preview fingerprint, and refuses when the requester itself occupies the worktree.

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

Record branch evidence, then provision the canonical worktree through execution
v1. Preview first and repeat the same request with `--confirm`:

```bash
agent-harness issueops branch prepare --id "$ISSUEOPS_ID" --provider "$PROVIDER" --issue-url "$ISSUE_URL" --branch "$BRANCH" --base-branch "$BASE_BRANCH" --link-verified --json
agent-harness issueops execution prepare --id "$ISSUEOPS_ID" --mode auto --owner-host "$OWNER_HOST" --owner-model "$OWNER_MODEL" $ACTOR_FLAGS --json
agent-harness issueops execution prepare --id "$ISSUEOPS_ID" --mode auto --owner-host "$OWNER_HOST" --owner-model "$OWNER_MODEL" $ACTOR_FLAGS --confirm --json
```

Record the approved design review:

```bash
agent-harness issueops design review --id "$ISSUEOPS_ID" \
  --problem-summary "$PROBLEM_SUMMARY" \
  --proposed-design "$PROPOSED_DESIGN" \
  --refactor-plan "$REFACTOR_PLAN" \
  --alternative "$ALTERNATIVE_CONSIDERED" \
  --risk "$DESIGN_RISK" \
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
agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge file --judge-file issueops-benchmark-judge.json --json
```

The benchmark passes only when every fixture has `average_score: 100`, `minimum_score: 100`, and `critical_failure_count: 0`. Run `--judge none` first for deterministic evidence. For the semantic gate, give a fresh independent host agent the rubric, artifact fields, and required score-map schema described in the parent skill, save its provenanced JSON result, and validate it with `--judge file`.

Run the autoresearch keep/discard gate for IssueOps improvement candidates:

```bash
agent-harness issueops benchmark gate --baseline "$BASELINE_ID" --candidate "$CANDIDATE_ID" --candidate-file candidate.json --changed-path skills/issueops/SKILL.md --json
```

The candidate file records the hypothesis, target dimensions, edit surface, and keep/discard criteria. The gate keeps a candidate only when the candidate benchmark passes, baseline comparison has no regression, target dimensions do not regress, and every changed path is inside the declared edit surface.

IssueOps does not call an external LLM service in-process. Semantic judgments come from a fresh independent host agent and enter the harness only through strict `--judge file` decoding.

## Human cleanup boundary

`issueops execution complete` records the completion receipt, moves the cycle to
`done`, and releases the generation. It does not merge, close a terminal, remove
a worktree, or delete a branch. After verified merge evidence, present the
numbered cleanup choices above and perform only the explicitly authorized
operations. A Stop hook, elapsed time, or missing prior session never grants
destructive cleanup authority.

## Retiring a cycle that never entered execution v1

A cycle that was merged and closed on the provider without `issueops execution
prepare` has no `Execution` and no recorded `remote_artifact`, so `cleanup
finish` cannot verify an artifact and `cleanup orphan` requires the record to be
absent. Retire it with `cleanup abandon`: a worktree or branch that the record
linked (`worktree_path` from `link-worktree`/`branch prepare`) counts as
record-owned residue and must still pass the canonical-path, branch-match, HEAD,
and clean checks. Run `issueops cleanup abandon --id ID --reason TEXT --preview
--json`, then execute the emitted `--apply --confirm --fingerprint` command.
Residue the record never linked stays blocked as `local_residue_execution`.
Abandon deletes the record without writing completion evidence to the issue; the
provider MR/issue remain the evidence, so keep `--reason` factual (no `#` or
control characters).
