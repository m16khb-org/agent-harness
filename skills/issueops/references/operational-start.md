# IssueOps Operational Start Reference

Load this reference only when actively starting, resuming, or documenting an IssueOps cycle.

## Start Or Resume

The IssueOps branch must be the issue branch, not the source checkout's current branch.

```bash
branch_slug="3-webhook-delivery"
agent-harness issueops start --repo "$PWD" --branch "$branch_slug" --json
agent-harness issueops status --id "$ISSUEOPS_ID" --json
```

## Intent Contract

Record the intent before plan phase or issue-link auto-advance:

```bash
agent-harness issueops intent record --id "$ISSUEOPS_ID" \
  --raw-request "$RAW_USER_REQUEST" \
  --interpreted-intent "$INTERPRETED_INTENT" \
  --success-criteria "$SUCCESS_CRITERION" \
  --constraint "$CONSTRAINT" \
  --ambiguity "$AMBIGUITY_LEDGER_ENTRY" \
  --non-goal "$NON_GOAL" \
  --intent-class "$INTENT_CLASS" \
  --json
```

## Plan Prep

Record evidence before entering `plan`. Each item takes evidence or a mutually exclusive waive reason:

```bash
agent-harness issueops plan-prep record --id "$ISSUEOPS_ID" \
  --decisions-evidence "$PRIOR_DECISION_LINK_OR_ADR" \
  --related-score-ref "$REMOTE_SCORE_SUMMARY" \
  --web-research-evidence "$RESEARCH_FILE_OR_SOURCE" \
  --json
```

Waiver examples:

```bash
agent-harness issueops plan-prep record --id "$ISSUEOPS_ID" \
  --decisions-waive "no prior decisions touch this area" \
  --related-waive "no comparable issues exist" \
  --web-research-waive "purely internal refactor, no external semantics" \
  --json
```

## Remote Issue And Worktree Linkage

```bash
agent-harness issueops link-issue --id "$ISSUEOPS_ID" --issue-url "$ISSUE_URL" --json
agent-harness issueops branch prepare --id "$ISSUEOPS_ID" --provider "$PROVIDER" --issue-url "$ISSUE_URL" --branch "$branch_slug" --base-branch "$BASE_BRANCH" --link-verified --json
agent-harness issueops link-worktree --id "$ISSUEOPS_ID" --worktree-path "$EXPECTED_WORKTREE" --json
agent-harness issueops design review --id "$ISSUEOPS_ID" \
  --problem-summary "$PROBLEM_SUMMARY" \
  --proposed-design "$PROPOSED_DESIGN" \
  --refactor-plan "$REFACTOR_PLAN" \
  --risk "$RISK" \
  --alternative "$ALTERNATIVE" \
  --verification "$VERIFICATION_STEP" \
  --approved \
  --json
agent-harness issueops link-plan --id "$ISSUEOPS_ID" --plan-path "$EXPECTED_WORKTREE/$PLAN_REL_PATH" --json
agent-harness issueops link-child --id "$ISSUEOPS_ID" --child-url "$CHILD_ISSUE_URL" --title "$CHILD_TITLE" --json
agent-harness issueops pr-readiness --id "$ISSUEOPS_ID" --strict --json
```

`branch prepare` records the provider-linked branch contract before local worktree creation. Use provider MCP first, provider API/CLI fallback second, and fail closed if neither can create a branch that the issue shows as linked. For GitLab, branch names must start with the issue or task number followed by a hyphen, for example `123-fix-login`.

`link-worktree` fails closed until issue-linked branch evidence exists and the worktree path already exists on disk. `link-plan` fails closed until the issue is linked, `branch prepare --link-verified` has provider-visible branch evidence, the worktree is linked, the design review is approved with no open questions, and the plan path exists inside that linked worktree.

## Implementation Entry

Run `worktree prepare-tools` after `link-plan`, record compatibility review, then record execution decision:

```bash
agent-harness issueops compatibility review --id "$ISSUEOPS_ID" \
  --backward-compatibility "existing state/CLI/MCP/API contracts checked" \
  --side-effect "side effects are limited to the reviewed implementation surface" \
  --rollback-plan "$ROLLBACK_PLAN" \
  --verification "$COMPATIBILITY_VERIFICATION" \
  --approved \
  --json
```

```bash
agent-harness issueops execution decide --id "$ISSUEOPS_ID" \
  --auto "implementation may proceed after linked worktree readiness is durable" \
  --hook-block "hooks do not create issues, prepare worktrees, run tests, or decide sub-agent usage" \
  --human-gate "ask before destructive cleanup, live access, or unclear product behavior" \
  --subagent-use none \
  --subagent-rationale "main agent owns this focused implementation" \
  --json
```

## Phase Advancement

The `ai-slop-clean` phase requires linked issue, provider-linked branch, plan, an existing linked worktree, and implementation changes under that worktree. The `pr` phase requires strict PR readiness, including ai-slop-clean evidence. The `done` phase requires the loop to have already entered `pr` and a verified remote PR/MR artifact with provider URL, label, and assignee evidence:

```bash
agent-harness issueops phase --id "$ISSUEOPS_ID" --to grill --json
agent-harness issueops phase --id "$ISSUEOPS_ID" --to ai-slop-clean --json
agent-harness issueops phase --id "$ISSUEOPS_ID" --to pr --json
agent-harness issueops remote verify-artifact --id "$ISSUEOPS_ID" --provider "$PROVIDER" --kind pr|mr --url "$PR_URL" --label "$LABEL" --assignee "$ASSIGNEE" --json
agent-harness issueops phase --id "$ISSUEOPS_ID" --to done --json
```

## Cleanup And Feedback

Post-merge cleanup status is read-only until the user chooses deletion:

```bash
agent-harness issueops cleanup status --id "$ISSUEOPS_ID" --merged --json
```

Close linked child tasks separately after the child PR/MR is verified merged into the parent work branch:

```bash
agent-harness issueops cleanup close-children --id "$ISSUEOPS_ID" --merged --json
agent-harness issueops cleanup close-children --id "$ISSUEOPS_ID" --merged --confirm --json
```

Record feedback with classification when contract-changing feedback must be distinguished:

```bash
agent-harness issueops feedback add --id "$ISSUEOPS_ID" --source review --body "$FEEDBACK" --classification contract_change --json
agent-harness issueops feedback mark-issue-updated --id "$ISSUEOPS_ID" --json
```
