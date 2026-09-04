# Orca live handoff spike — 2026-07-11

## Purpose

Resolve the Brooks `revise` finding against `2026-07-11-orca-aware-issueops-handoff-design.md`: verify the installed Orca mutation/result/recovery contract before fixing production states, commands, and DTOs.

The spike used the already registered `issueops` repo, a unique temporary branch/worktree marker, a bare Bash terminal, and two disposable orchestration tasks. It did not run an implementation agent or modify the source checkout.

## Run identity

```text
marker: io-orca-spike-20260710163913
base: main @ 2ba240b94477190071598b3f1c7278312b296611
runtime: b5b60c39-4aff-4197-90fe-c0a0db1b3253
```

## Observations

### Worktree create result

`orca worktree create --repo path:<repo> --name <marker> --base-branch main --no-parent --setup skip --comment <attempt-marker> --json` returned:

```text
result.worktree.id
result.worktree.instanceId
result.worktree.repoId/projectId/hostId/projectHostSetupId
result.worktree.path
result.worktree.head
result.worktree.branch = refs/heads/<marker>
result.worktree.displayName
result.worktree.comment
result.worktree.baseRef
result.timing/phases
result.lineage/workspaceLineage
_meta.runtimeId
```

The exact branch matched the requested name and the head matched `main`.

### Worktree collision is not idempotent

Repeating the exact create command succeeded instead of rejecting the collision. Orca created:

```text
first:  .../io-orca-spike-20260710163913
second: .../io-orca-spike-20260710163913-2

first branch:  refs/heads/io-orca-spike-20260710163913
second branch: refs/heads/io-orca-spike-20260710163913-2
```

Both records retained the same caller-supplied comment. A deterministic name/comment is therefore a reconciliation hint, not an idempotency key. Agent-harness must persist pre-mutation worktree IDs and invoke create at most once per attempt. After an ambiguous response, exactly one new ID may be reconciled; zero or multiple new IDs remain fail-closed.

### Worktree create also creates a shell terminal

Immediately after plain worktree creation, `terminal list --worktree path:<path>` returned one connected/writable terminal. This confirms the installed help contract that worktree creation creates a first terminal even without `--agent`.

### Terminal create/list identity mismatch

`terminal create --worktree path:<path> --title AH-SPIKE-AGENT-... --command bash --json` returned:

```text
result.terminal.handle
result.terminal.tabId
result.terminal.paneKey
result.terminal.ptyId
result.terminal.worktreeId
result.terminal.title = requested custom title
result.terminal.surface
```

A subsequent `terminal list` kept the handle, PTY ID, and worktree ID but changed the observable identity:

```text
title: requested custom title -> bash
tabId: create UUID -> pty:<host@@pty-id>
leafId: pty:<host@@pty-id>
```

Custom title, create-time tab ID, pane key, and list-time tab/leaf ID are not suitable as one durable identity tuple. Before terminal creation, the harness must snapshot terminal PTY IDs for the worktree. An ambiguous create can be reconciled only when exactly one new terminal appears. Live control still refreshes the handle from the current list.

### Task title/display name collision is allowed

Two `task-create` calls with the same task title and display name both succeeded with different IDs:

```text
task_f120bf93f655
task_6d917d1032e1
```

Task title/display name is not unique. The task-create result ID is authoritative. Before creation, snapshot task IDs; after an ambiguous response, reconcile only an exact one-item delta. Never retry task creation automatically.

### Dispatch is recoverable by persisted task ID

Dispatching the first task without injection returned:

```text
result.dispatch.id = ctx_9a7baa855b4d
result.dispatch.task_id = task_f120bf93f655
result.dispatch.assignee_handle = worker terminal handle
result.dispatch.status = dispatched
result.injected = false
result.preamble = worker lifecycle contract + task
```

`dispatch-show --task task_f120bf93f655` reproduced the same dispatch ID, assignee, status, and timestamps. Once task ID is known, dispatch reconciliation is unambiguous. If task creation itself was ambiguous and no unique task delta exists, dispatch must not proceed.

The generated preamble contained a refreshed coordinator terminal handle rather than the stale `ORCA_TERMINAL_HANDLE` inherited by the current shell. This reinforces that runtime handles are routing values, not durable ownership authority.

### Bare-terminal delivery works

`terminal send --text "touch .orca-spike-delivered" --enter` returned `accepted:true` and `bytesWritten:28`. The marker appeared only in the temporary worktree. This validates the non-inject delivery fallback without launching an autonomous agent.

### Task update closes the current dispatch

`task-update --status completed` changed both the task and its latest dispatch to `completed`. `--result` is stored as a string; it is not validated as JSON by the installed CLI. The adapter must pass argv directly and treat the result field as an opaque bounded string/projection.

## Architectural consequences

1. The first release cannot promise transparent automatic retry of create operations.
2. Recovery is a set-difference reconciliation against a pre-mutation baseline:

   ```text
   new IDs = after IDs - before IDs
   exactly one -> reconcile
   zero or more than one -> recovery_required
   ```

3. `status` remains read-only. Only explicit `recover` may persist a reconciled identity.
4. Recovery never “runs the next missing step”; after reconciliation it returns the next explicit command.
5. The durable Orca projection can be reduced to runtime ID, worktree ID/instance/path, pre-mutation ID baselines, worker PTY ID, dispatch-time worker mailbox handle, task ID, and dispatch ID. Live terminal handles are refreshed, not authoritative.
6. Worktree/task marker collisions are expected behavior and must be covered by tests.

## Cleanup receipt

```text
orca worktree rm --worktree path:<first> --force --json  -> removed:true
orca worktree rm --worktree path:<second> --force --json -> removed:true
orca worktree list marker query -> []
both temporary directories -> absent
git worktree list marker query -> no match
git branch --list marker* -> no match
```

Orca has no per-task delete command. Both spike tasks were moved to `completed` and remain as bounded historical evidence:

```text
task_f120bf93f655   completed
task_6d917d1032e1   completed
```

The global `orchestration reset --tasks` command was deliberately not used because it could delete unrelated concurrent task history.

## Turing evidence

```text
Criterion: raw installed Orca worktree/terminal/task/dispatch/recovery behavior is observed before production DTO design
Observable evidence: exact JSON field projections and collision/reconciliation results above
Adversarial cases: duplicate worktree name/comment, duplicate task title/display, discarded-result recovery by list/dispatch-show, bare-terminal delivery
Cleanup: both worktrees/directories/branches removed; disposable tasks terminally completed; no global reset
Verdict: supervised handoff is viable only with no automatic mutation retry and exact-one baseline-delta reconciliation
```
