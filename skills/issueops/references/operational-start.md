# IssueOps v1 Operational Start

Load this reference only while starting, resuming, or documenting an IssueOps
cycle. The lifecycle branch is the provider-linked issue branch, not whatever
branch the source checkout currently has.

## Start And Record The Contract

```bash
branch_slug="69-issueops-v1"
agent-harness issueops start --repo "$PWD" --branch "$branch_slug" --json
agent-harness issueops status --id "$ISSUEOPS_ID" --json

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

For non-trivial work, record prior-decision, related-issue, and external
research evidence or an explicit waiver before plan entry:

```bash
agent-harness issueops plan-prep record --id "$ISSUEOPS_ID" \
  --decisions-evidence "$PRIOR_DECISION_LINK_OR_ADR" \
  --related-score-ref "$REMOTE_SCORE_SUMMARY" \
  --web-research-evidence "$RESEARCH_FILE_OR_SOURCE" \
  --codebase-survey-evidence "$TOOLS_AND_TOUCHED_SYMBOLS_FILES" \
  --json
```

## Issue, Branch, And Design

Link the remote issue and pin the exact base SHA before execution preparation.
Direct mode and GitLab create and verify the provider-visible branch first.
GitHub Orca records the matching issue identity and base SHA first but delays
the linked branch until Orca has created its local-only branch. The approved
design has no open questions:

```bash
agent-harness issueops link-issue --id "$ISSUEOPS_ID" --issue-url "$ISSUE_URL" --json
agent-harness issueops branch prepare --id "$ISSUEOPS_ID" \
  --provider "$PROVIDER" --issue-url "$ISSUE_URL" \
  --branch "$branch_slug" --base-branch "$BASE_BRANCH" \
  --link-verified --json
agent-harness issueops design review --id "$ISSUEOPS_ID" \
  --problem-summary "$PROBLEM_SUMMARY" \
  --proposed-design "$PROPOSED_DESIGN" \
  --refactor-plan "$REFACTOR_PLAN" \
  --alternative "$ALTERNATIVE" \
  --risk "$RISK" \
  --verification "$VERIFICATION_STEP" \
  --approved --json
```

`branch prepare` must persist `base_sha`. Except for the GitHub Orca ordering
below, if provider linkage or the base SHA cannot be verified, stop before
creating a local workspace.

For GitHub Orca, omit `--link-verified` from the first `branch prepare` call.
After `execution prepare` creates the local-only branch and the owner claims the
lease, create the GitHub linked branch at the sealed base SHA and run the exact
`VerifyBranchLink` command from the owner packet. That command repeats
`branch prepare` with `--link-verified` and the current owner actor flags.
`link-plan` and implementation remain blocked until this update succeeds.

## Execution v1

Use one preview/confirm request. `auto` resolves to Orca only when readiness is
proven before mutation; otherwise it resolves to direct:

```bash
agent-harness issueops execution prepare --id "$ISSUEOPS_ID" --mode auto \
  --owner-host "$OWNER_HOST" --owner-model "$OWNER_MODEL" \
  --owner-effort "$OWNER_EFFORT" $ACTOR_FLAGS --json

agent-harness issueops execution prepare --id "$ISSUEOPS_ID" --mode auto \
  --owner-host "$OWNER_HOST" --owner-model "$OWNER_MODEL" \
  --owner-effort "$OWNER_EFFORT" $ACTOR_FLAGS --confirm --json
```

Inspect the durable state at any point:

```bash
agent-harness issueops execution status --id "$ISSUEOPS_ID" --json
```

For direct mode, the same main session is the generation holder and continues
from the canonical worktree. For Orca mode, the launched native owner reads the
private packet and prompt, verifies both sealed digests, and runs the exact
`issueops execution claim` command rendered by status. Load `execution.md`
for the full claim, replacement, reconciliation, publication, and completion
contract.

After workspace creation, write and link the issue-based plan inside the
canonical worktree, complete compatibility and devil's-advocate review, then
enter implementation. Sub-agent use follows the repository's documented
net-positive patterns; it is main-agent judgment, not a second IssueOps state
machine.

## Publication And Completion

The active holder advances through TDD, cleanup, feedback, and `pr`, then
creates a draft PR/MR with its exact generation and actor receipt:

```bash
agent-harness issueops remote create-pr \
  --id "$ISSUEOPS_ID" --expected-generation "$GENERATION" \
  --title "$TITLE" --head "$branch_slug" --base "$BASE_BRANCH" \
  --body "$BODY" --label "$LABEL" --assignee "$ASSIGNEE" \
  $ACTOR_FLAGS --confirm --json
```

Once the durable remote artifact has been read back and verified, complete and
release atomically:

```bash
agent-harness issueops execution complete \
  --id "$ISSUEOPS_ID" --generation "$GENERATION" \
  --final-head "$FINAL_HEAD" --turing-report "$TURING_REPORT" \
  --remote-artifact-url "$PR_URL" --verification "$VERIFICATION" \
  $ACTOR_FLAGS --confirm --json
```

Completion requires phase `pr`; it is not an escape from planning,
implementation, verification, or remote readback. Merge and destructive
worktree/branch cleanup remain separate human-authorized operations.
