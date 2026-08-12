# IssueOps Orchestration Reference

Use this reference when an IssueOps parent cycle delegates bounded work to child cycles. The harness stores coordination state; the main agent still owns dispatch, validation, and every safety decision.

Ephemeral independent fan-out uses the host's native subagent concurrency controls. Durable delegated work uses IssueOps child cycles, isolated canonical worktrees, generation-fenced execution ownership, and parent accept/reject validation.

## S1 Walkthrough: Delegated Child Cycles

1. Confirm the parent is in `implement phase`.
2. Confirm approved reviews are recorded: design review, compatibility review, and devil's-advocate review are pass or explicitly waived.
3. Confirm the parent plan or durable evidence records a documented pattern slug, scope, verification, fallback, tradeoffs, and net-positive rationale.
4. Start a child with `issueops child start --parent "$ISSUEOPS_ID" --branch "$CHILD_BRANCH" --title "$TITLE" --scope "$SCOPE" --acceptance "$CRITERION" --json`.
5. Bind and run the child in its own isolated worktree. Its exact execution generation and native process receipt fence writes; inspect them with `issueops execution status --id "$CHILD_ID"`.
6. Inspect progress with `issueops child status --parent "$ISSUEOPS_ID" --json`.
7. When the child reaches done, validate the evidence with `issueops child accept --parent "$ISSUEOPS_ID" --child "$CHILD_ID" --evidence "$EVIDENCE" --json`, or use `issueops child reject` / `issueops child drop` with a reason.

### Omo Native Worktree Dispatch

For durable parallel children in Omo, use a resident native team and bind every
member to its child execution's canonical absolute worktree:

```json
{
  "name": "issueops-parallel",
  "members": [
    {
      "name": "child-a",
      "kind": "category",
      "category": "<configured execution category>",
      "worktreePath": "/absolute/canonical/child-a-worktree",
      "prompt": "<child contract prompt>",
      "task_summary": "<bounded child scope>"
    }
  ]
}
```

Omo maps `members[].worktreePath` to the resident child process `cwd` and
sandbox worktree boundary. Do not use plain `task`, which has no
worktree-binding field, and do not rely on a prompt-only `cd` instruction.
After spawn, require each member to report `pwd`, `git rev-parse
--show-toplevel`, `PI_SESSION_ID`, the live `omo` process receipt, execution
generation, and `issueops execution whoami --json`. All roots must equal the
sealed canonical child worktree before mutation.

If a configured team category cannot start, report that routing failure and
start an independent branded `omo -p` process from the canonical worktree as
the bounded fallback. The fallback still must pass the same cwd, native actor,
generation, and process-receipt gates; running tests directly in the lead
session is not equivalent evidence.

### Child Contract Prompt Template

```text
You are executing a delegated IssueOps child cycle.

child contract:
- Parent cycle: <parent id>
- Child cycle: <child id>
- Branch: <child branch>
- Expected worktree: <absolute child worktree>
- Export before work: export HARNESS_EXPECTED_WORKTREE=<absolute child worktree>
- Scope: <bounded child scope>
- Acceptance criteria: <criteria>
- Owner command: issueops child status --parent <parent id> --json

Rules:
- Work only inside the expected worktree.
- Run TDD for behavior changes.
- Before each mutation, retain the exact child lifecycle ID, generation, native actor, and canonical cwd from execution status.
- Stop and report on scope drift instead of expanding the child contract.
- Do not mutate the parent record directly. The parent accepts, rejects, or drops your result.

Output:
- Summary of changed files.
- Evidence commands and results.
- Any blockers or scope drift.
```

### Scope-Drift Stop Rule

Stop and report on scope drift when the child discovers work outside the child contract, a required parent decision, a product behavior change, a new migration, a new remote artifact write, or a verification requirement not named in the parent plan. Do not widen the child silently. The parent may create a new child, revise the plan, or drop the child.

### Validation Rubric

Accept a child only when all items are true:

- The child stayed within the delegated scope and expected worktree.
- The child evidence proves every acceptance criterion.
- The child ran the stated verification commands, or explicitly reported why a command could not run.
- The diff contains no unrelated refactor, generated drift, secrets, or stale scaffold.
- The parent can integrate the result without guessing what changed.

Reject a child when the result is fixable but incomplete, out of date, weakly verified, or missing required evidence. Drop a child only when the parent intentionally removes that work from the contract or replaces it with another path, and record the reason.

## Missing Key Owner Commands

| Missing key | Owner command | Meaning |
|---|---|---|
| `child_incomplete` | `issueops child status` | A child is not done or cannot be read; inspect and continue/recover the child. |
| `child_unvalidated` | `issueops child accept` | A child is done but has no accepted verdict; validate its evidence. |
| `child_rejected_unresolved` | `issueops child accept` or `issueops child drop` | A rejected child still blocks the parent until corrected evidence is accepted or the child is dropped. |
| `children_active` | `issueops child status` | Active children block parent regression/cleanup shortcuts. |

## Verdict Semantics

- `accepted`: evidence satisfies the child or task contract; the parent gate may proceed.
- `rejected`: evidence is insufficient or scope drifted; redo, revise, or replace the work.
- `dropped`: the parent intentionally removes the work from the contract with a durable reason.
