# IssueOps Orchestration Reference

Use this reference when an IssueOps parent cycle delegates bounded work to child cycles or a workpool. The harness stores coordination state; the main agent still owns dispatch, validation, and every safety decision.

## S1 Walkthrough: Delegated Child Cycles

1. Confirm the parent is in `implement phase`.
2. Confirm approved reviews are recorded: design review, compatibility review, and devil's-advocate review are pass or explicitly waived.
3. Confirm the parent has a recorded sub-agent plan in `execution_decision` with a documented pattern slug, scope, verification, fallback, and net-positive rationale.
4. Start a child with `issueops child start --parent "$ISSUEOPS_ID" --branch "$CHILD_BRANCH" --title "$TITLE" --scope "$SCOPE" --acceptance "$CRITERION" --json`.
5. Bind and run the child in its own isolated worktree. The child must heartbeat with `issueops heartbeat --id "$CHILD_ID"` during long work.
6. Inspect progress with `issueops child status --parent "$ISSUEOPS_ID" --json`.
7. When the child reaches done, validate the evidence with `issueops child accept --parent "$ISSUEOPS_ID" --child "$CHILD_ID" --evidence "$EVIDENCE" --json`, or use `issueops child reject` / `issueops child drop` with a reason.

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
- Heartbeat during long work: issueops heartbeat --id <child id> --json.
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

## S2 Walkthrough: Worker Pool

Use a workpool for repeated fan-out tasks with the same owner contract. A pool is not an agent launcher; it is a lease and validation ledger.

1. Create the pool: `workpool create --repo "$PWD" --name "$POOL" --parent-cycle "$ISSUEOPS_ID" --json`.
2. Add tasks: `workpool add-task --pool "$POOL_ID" --title "$TITLE" --instructions "$INSTRUCTIONS" --acceptance "$CRITERION" --json`.
3. Worker claims one task: `workpool claim --pool "$POOL_ID" --worker "$WORKER_ID" --json`.
4. Worker prepares an isolated worktree and records the expected worktree in its own run context.
5. Worker heartbeats during long work: `workpool heartbeat --pool "$POOL_ID" --task "$TASK_ID" --worker "$WORKER_ID" --json`.
6. Worker submits evidence: `workpool submit --pool "$POOL_ID" --task "$TASK_ID" --worker "$WORKER_ID" --evidence "$EVIDENCE" --branch "$BRANCH" --worktree "$WORKTREE" --json`.
7. Main agent validates with `workpool accept --pool "$POOL_ID" --task "$TASK_ID" --evidence "$EVIDENCE" --json` or `workpool reject --pool "$POOL_ID" --task "$TASK_ID" --reason "$REASON" --json`.
8. Inspect owner state with `workpool status --pool "$POOL_ID" --json`; close only when terminal or when a force close is explicitly approved and reasoned.

### Pool Worker Loop Prompt Template

```text
You are executing one leased workpool task.

pool worker loop:
- Pool: <pool id>
- Task: <task id>
- Worker: <worker id>
- Branch: <recommended branch>
- Expected worktree: <absolute task worktree>
- Instructions: <task instructions>
- Acceptance criteria: <criteria>

Rules:
- Work only in the task worktree.
- Heartbeat before and during long work: workpool heartbeat --pool <pool id> --task <task id> --worker <worker id> --json.
- Submit only when evidence is ready: workpool submit --pool <pool id> --task <task id> --worker <worker id> --evidence "<evidence>" --branch <branch> --worktree <worktree> --json.
- If the lease expires or worker id mismatches, stop; do not keep editing under a lost lease.
- Apply the scope-drift stop rule. Report drift instead of broadening the task.

Output:
- Task id and worker id.
- Evidence submitted.
- Verification commands and results.
- Scope drift or blockers.
```

## Missing Key Owner Commands

| Missing key | Owner command | Meaning |
|---|---|---|
| `child_incomplete` | `issueops child status` | A child is not done or cannot be read; inspect and continue/recover the child. |
| `child_unvalidated` | `issueops child accept` | A child is done but has no accepted verdict; validate its evidence. |
| `child_rejected_unresolved` | `issueops child accept` or `issueops child drop` | A rejected child still blocks the parent until corrected evidence is accepted or the child is dropped. |
| `pool_incomplete` | `workpool status` | A linked pool still has non-terminal tasks or an open pool state. |
| `children_active` | `issueops child status` | Active children block parent regression/cleanup shortcuts. |

## Verdict Semantics

- `accepted`: evidence satisfies the child or task contract; the parent gate may proceed.
- `rejected`: evidence is insufficient or scope drifted; redo, revise, or replace the work.
- `dropped`: the parent intentionally removes the work from the contract with a durable reason.
