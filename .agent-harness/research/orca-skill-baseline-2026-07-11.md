# Orca handoff skill baseline — 2026-07-11

Purpose: RED evidence required before changing the `issueops` and `turing` skills. Three fresh-context agents answered without reading repository skills or files.

## Scenario 1 — Orca unavailable

Prompt shape: prepare an IssueOps issue/branch/worktree, optionally launch a fresh Orca worker, and explain the unavailable path.

Observed gaps:

- It proposed a version/health preflight, including a version command; the installed Orca contract has no reliable version command.
- It said any Orca timeout or unreachable result should “immediately continue ... using the current agent,” without distinguishing a read-only probe failure from an ambiguous post-mutation timeout.
- It did not require an absent durable handoff field for the inline fallback.
- It did preserve the already-created issue/branch/worktree and kept PR/cleanup authority with the coordinator.

## Scenario 2 — create timed out

Prompt shape: an Orca create command timed out before returning an artifact id under pressure to continue.

Observed gaps:

- It correctly treated the result as unknown and said “Do not immediately rerun `create`.”
- It invented a possible Orca idempotency/correlation key and allowed one create retry if such a key existed. The installed Orca spike proved no public idempotency contract; production guidance must prohibit automatic create retry unconditionally.
- It suggested selecting an already linked duplicate after delayed discovery. The required contract instead accepts only exact-one marker/PTY reconciliation and fails closed on zero or multiple candidates.
- It correctly retained reconciliation ownership in the coordinator.

## Scenario 3 — evidence-bound worker

Prompt shape: a coordinator prepares context and a fresh worker implements in the worktree.

Observed gaps:

- It used generic PID/session heartbeats instead of `issueops handoff claim`, fenced `issueops heartbeat`, and `issueops handoff finish`.
- It let the worker create a commit and exit without a durable `submitted` result followed by coordinator `accept`.
- It correctly kept push, PR, merge, and cleanup authority with the coordinator.

## Minimal skill delta justified by the RED run

1. IssueOps needs a conditional recipe keyed to the observable Orca capability probe and the pre-mutation fallback boundary.
2. IssueOps needs the exact coordinator/worker command sequence, no-create-retry rule, and exact-one recovery rule.
3. Turing needs the worker evidence/report/cleanup shape plus current `issueops heartbeat` guidance; its existing claim that the heartbeat command is stale is incorrect.
4. No other skill changes are justified by this baseline.

```text
Success criteria: baseline agents expose missing or unsafe Orca handoff behavior before skill edits
Evidence artifact: this file; three fresh-context agent outputs retained in the parent session trace
Cleanup receipt: no runtime or filesystem artifacts created by the baseline agents
Verification mode: writing-skills RED pressure scenarios
Skipped checks: revised-skill forward tests run after implementation, not during baseline
```
