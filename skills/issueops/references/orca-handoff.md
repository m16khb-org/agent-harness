# Optional Orca Supervised Handoff

Choose the worktree mode after the IssueOps issue is linked, the provider branch is verified, and the design review is approved. After the resulting worktree exists, link the plan there and complete the normal compatibility, worktree-tool, execution-decision, and devil's-advocate gates before dispatch. IssueOps remains the single durable authority. Orca is an optional process/worktree driver and never owns phase state, evidence acceptance, PR creation, or cleanup decisions.

## Choose The Mode

Preview before mutation:

```bash
agent-harness issueops worktree prepare --id "$ISSUEOPS_ID" --orchestrator auto --agent codex --json
```

The three modes are intentionally small:

- `--orchestrator auto`: probe Orca. If the probe fails before any mutation, return the unchanged inline result and leave `execution_handoff` absent. If Orca is ready, preview or prepare the supervised worktree.
- `--orchestrator orca`: require a ready Orca installation and fail before mutation when the probe fails.
- `--orchestrator inline`: use the legacy sibling-worktree flow and leave `execution_handoff` absent.

Before confirming Orca mode, turn **Settings > General > Workspace > Nest Workspaces** off. IssueOps requires the flat canonical path `<repo>.worktrees/<branch>` and does not read private Orca settings, toggle global layout preferences, or accept the nested `<repo>.worktrees/<repo-name>/<branch>` form. The returned worktree path is validated after the exactly-once create call. A mismatch moves the handoff to `recovery_required` with an actionable diagnostic; it never adopts the mismatched path or falls back inline. Cancel the handoff, remove the mismatched resources, and start a fresh IssueOps cycle after correcting the setting.

The Orca repo projection must expose `gitRemoteIdentity.remoteName`. The confirmed create uses `refs/remotes/<remote>/<branch>` as its base so an already-linked provider branch keeps the exact branch and directory name without pre-creating or checking out a local branch. A missing remote name is a pre-mutation probe failure.

Do not infer Orca readiness from `orca version`; the adapter uses structured `orca status --json`. Run the confirmed command only after reviewing the preview:

```bash
agent-harness issueops worktree prepare \
  --id "$ISSUEOPS_ID" \
  --orchestrator auto \
  --agent codex \
  --confirm \
  --json
```

If `resolved_mode` is `inline`, continue the existing in-session bind, heartbeat, TDD, verification, and PR-readiness flow. Do not synthesize an `execution_handoff` record for inline fallback.

## Coordinator Dispatch

For `resolved_mode: orca`, the coordinator reviews readiness and creates the bounded context packet. Repeat flags are allowed where shown:

```bash
agent-harness issueops handoff start \
  --id "$ISSUEOPS_ID" \
  --criteria-id ORCA-01 \
  --criteria-id ORCA-02 \
  --criteria-id ORCA-03 \
  --criteria-id ORCA-04 \
  --criteria-id ORCA-05 \
  --criteria-id ORCA-06 \
  --criteria-id ORCA-07 \
  --criteria-id ORCA-08 \
  --criteria-id ORCA-09 \
  --criteria-id ORCA-10 \
  --criteria-id ORCA-11 \
  --criteria-id ORCA-12 \
  --criteria-id ORCA-13 \
  --criteria-id ORCA-14 \
  --required-doc "$PLAN_PATH" \
  --required-skill superpowers:test-driven-development \
  --required-skill superpowers:verification-before-completion \
  --worker-scope "$WORKER_SCOPE" \
  --verification "go test ./... -count=1" \
  --heartbeat-cadence "every 5 minutes and at task boundaries" \
  --stop-condition "do not push, open or merge a PR, or accept the handoff" \
  --result-format "final head, changed files, Turing report, verification, and cleanup receipts" \
  --confirm \
  --json
```

The returned `attempt`, `ownership_epoch`, `context_sha256`, Orca worktree id, task id, and dispatch id are a single fence. The coordinator passes that tuple to the fresh worker without copying credentials, conversation transcripts, or unbounded environment data.

## Worker Lease

The fresh worker starts in the exact Orca worktree and claims before any mutation:

```bash
agent-harness issueops handoff claim \
  --id "$ISSUEOPS_ID" \
  --attempt "$ATTEMPT" \
  --ownership-epoch "$OWNERSHIP_EPOCH" \
  --context-sha256 "$CONTEXT_SHA256" \
  --host "$HOST" \
  --session-id "$SESSION_ID" \
  --agent-id "$AGENT_ID" \
  --cwd "$WORKTREE_PATH" \
  --orca-worktree-id "$ORCA_WORKTREE_ID" \
  --json
```

Only that native host/session/agent identity may heartbeat or finish the claimed attempt:

```bash
agent-harness issueops heartbeat \
  --id "$ISSUEOPS_ID" \
  --attempt "$ATTEMPT" \
  --ownership-epoch "$OWNERSHIP_EPOCH" \
  --context-sha256 "$CONTEXT_SHA256" \
  --host "$HOST" \
  --session-id "$SESSION_ID" \
  --agent-id "$AGENT_ID" \
  --json
```

On completion, submit bounded evidence. A completed result requires the final head, Turing report, at least one verification entry, and at least one cleanup receipt:

```bash
agent-harness issueops handoff finish \
  --id "$ISSUEOPS_ID" \
  --attempt "$ATTEMPT" \
  --ownership-epoch "$OWNERSHIP_EPOCH" \
  --context-sha256 "$CONTEXT_SHA256" \
  --host "$HOST" \
  --session-id "$SESSION_ID" \
  --agent-id "$AGENT_ID" \
  --outcome completed \
  --final-head "$FINAL_HEAD" \
  --changed-file "$CHANGED_FILE" \
  --turing-report "$TURING_REPORT" \
  --verification "$VERIFICATION_RECEIPT" \
  --cleanup-receipt "$CLEANUP_RECEIPT" \
  --task-id "$ORCA_TASK_ID" \
  --dispatch-id "$ORCA_DISPATCH_ID" \
  --json
```

After a successful finish, the worker stops. The worker must not push, create or merge a PR/MR, accept its own result, delete the provider branch, or remove the coordinator-owned worktree. The coordinator owns PR, acceptance, and cleanup.

## Coordinator Acceptance

The coordinator inspects the submitted head and evidence, reruns the required checks, and then accepts the exact fence:

```bash
agent-harness issueops handoff accept \
  --id "$ISSUEOPS_ID" \
  --attempt "$ATTEMPT" \
  --ownership-epoch "$OWNERSHIP_EPOCH" \
  --context-sha256 "$CONTEXT_SHA256" \
  --final-head "$FINAL_HEAD" \
  --json
```

`issueops resume --bind` is intentionally read-only for supervised handoffs. It cannot transfer the lease; use the explicit claim/recovery commands.

## Failure And Recovery

IssueOps persists `pending_operation` before every external Orca create or dispatch. Once a mutation has been invoked, never retry a create operation automatically, even when the adapter reports a timeout or ambiguous error. The record moves to `recovery_required`, inline fallback is forbidden, and the coordinator inspects status before choosing an explicit action.

Reconcile first:

```bash
agent-harness issueops handoff recover --id "$ISSUEOPS_ID" --action reconcile --json
```

Recovery accepts exactly one candidate relative to the persisted baseline and marker. Zero or multiple matching worktrees, terminals, tasks, or dispatches fail closed and preserve the pending journal. Never guess an identity and never issue another create to discover whether the first one worked.

If reconciliation proves no safe continuation, close the attempt explicitly. A later retry is a new attempt and ownership epoch, and is allowed only after the prior attempt is safely closed with no ambiguous pending operation:

```bash
agent-harness issueops handoff recover --id "$ISSUEOPS_ID" --action cancel --confirm --json
agent-harness issueops handoff recover --id "$ISSUEOPS_ID" --action retry --confirm --json
```

`auto` fallback is allowed only after a pre-mutation probe failure. It is never a recovery strategy for `coordinator_preparing`, `dispatched`, `claimed`, `submitted`, or `recovery_required` state.
