# Orca Ownership Handoff

IssueOps has one ownership-transfer contract. The source session prepares an
Orca-managed isolated workspace and dispatches one native owner. The owner
claims the exact cycle, acknowledges the sealed context, performs implementation
inside that worker root, publishes the MR/PR, and completes the cycle directly
into the human cleanup boundary.

## Isolation invariant

- Fence only the canonical worker root, exact lifecycle ID, native owner, or a
  persisted Orca worktree/terminal/task/dispatch identity.
- An exact lifecycle ID always resolves before source-wide inference or
  multi-cycle inference. The selected record must still match the current
  source or worker context and native actor.
- A source checkout, session binding, shared branch name, or matching relative
  path is never a repository-wide fence.
- Unrelated active cycles, including dispatched, completed, or cleanup-pending
  cycles, never block a literal `issueops start --repo <exact-source>` for
  another cycle.
- A prep-only cycle with an exact linked worktree and lifecycle ID remains
  outside unrelated ownership fences, so its status and gate mutations route to
  that cycle before ownership dispatch.
- The owner may mutate only its selected cycle and worker root. It must never
  touch another worktree, branch, lifecycle ID, or Orca resource.

## Current lifecycle

```text
workspace ready
  -> ownership_dispatching
  -> ownership_dispatched
  -> owner_orienting
  -> owner_active
  -> cleanup_pending_human_decision
  -> cleanup_executing
  -> closed
```

`recovery_required` is the fail-closed state for an interrupted external
operation. Recovery actions are `reconcile`, `abandon`, `cancel`, and
`finalize-cancel`; do not repeat an ambiguous external mutation.

When worktree creation completed but the pre-dispatch workspace journal is
`recovery_required`, the sealed preparation session reconciles exactly one
marker-matching Orca worktree from the source checkout:

```bash
agent-harness issueops worktree reconcile \
  --id "$ISSUEOPS_ID" \
  --workspace-epoch "$WORKSPACE_EPOCH" \
  --host "$HOST" \
  --session-id "$SESSION_ID" \
  --source-cwd "$SOURCE_ROOT" \
  --json
```

Include `--agent-id` when the preparation session sealed one. This command
cannot create a worktree, dispatch an owner, or recover a post-dispatch
handoff; those remain under `issueops handoff recover`.

## Dispatch and claim

From the source checkout, preview `handoff start`, review the sealed context
hash and exact workspace epoch, then repeat the identical request with
`--expected-context-sha256` and `--confirm`. Confirmation creates/attaches the
owner terminal, task, and dispatch under the persisted operation journal.

The isolated session uses the exact claim command rendered by status/session
guidance:

```bash
agent-harness issueops handoff claim \
  --id "$ISSUEOPS_ID" \
  --attempt "$ATTEMPT" \
  --ownership-epoch "$OWNERSHIP_EPOCH" \
  --context-sha256 "$CONTEXT_SHA256" \
  --host "$HOST" \
  --session-id "$SESSION_ID" \
  --cwd "$WORKER_ROOT" \
  --orca-worktree-id "$ORCA_WORKTREE_ID" \
  --json
```

Then acknowledge the issue and plan context. Before acknowledgement the owner
is read-only. After acknowledgement, exact-ID IssueOps observations and gates
must route to this cycle even when other active cycles share the same source
checkout.

## Owner work and completion

The acknowledged owner runs TDD implementation, verification, local commits,
publication, and PR/MR creation from the canonical worker root after the source
has completed the required plan and dispatch gates. Provider writes still obey
the user's approval boundary. Direct
Orca steering, cross-worktree edits, merge, deploy, and cleanup are not owner
authority.

After final verification, the owner records the exact remote publication
receipt and completes:

```bash
agent-harness issueops handoff complete \
  --id "$ISSUEOPS_ID" \
  --attempt "$ATTEMPT" \
  --ownership-epoch "$OWNERSHIP_EPOCH" \
  --context-sha256 "$CONTEXT_SHA256" \
  --host "$HOST" \
  --session-id "$SESSION_ID" \
  --cwd "$WORKER_ROOT" \
  --final-head "$FINAL_HEAD" \
  --turing-report "$REPORT_PATH" \
  --verification "$VERIFICATION" \
  --json
```

Completion enters `cleanup_pending_human_decision`. It does not close a
terminal, remove a worktree, delete a branch, merge an MR/PR, or return control
of the exact cycle to the source session.

## Human cleanup after merge

After a human merges the MR/PR, any fresh authenticated session at the exact
source root may run `cleanup-preview`. Present exactly these choices:

1. retain resources;
2. close the owner terminal and retain the workspace;
3. remove local worker resources.

Only the human-approved source session may run `cleanup-approve --confirm` and
record the ordered receipts. The completed owner cannot approve its own
cleanup. Stale scan, elapsed time, Stop hooks, or the original source session's
absence never grant cleanup authority.

For `close-owner`, record `task_terminal` then `terminal_quiescent`. For
`remove-local`, first require the verified remote head, then record
`task_terminal`, `terminal_quiescent`, `worktree_removed`, and
`local_branch_removed` after each external operation is actually observed.
Incomplete or ambiguous inventory fails closed.

## Operational evidence

Run identity and inventory reads as separate commands: canonical cwd, Git root,
branch, HEAD, dirty status, source status, exact worktree terminals,
server-filtered dispatched tasks, and exact dispatch. A partial/yielded command
is unfinished evidence. Usage-limit, rate-limit, reset, and model-selection
prompts are user decision boundaries and must never be handled automatically.
