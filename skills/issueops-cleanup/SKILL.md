---
name: issueops-cleanup
description: Safely finish a merged IssueOps cycle by closing its linked issue, removing its local worktree, and deleting its local branch. Use when the user asks for IssueOps cleanup, post-merge cleanup, worktree/branch deletion plus issue closure, or says "이슈옵스 정리", "워크트리 지우고 브랜치 지우고 이슈 닫아줘".
---

# IssueOps Cleanup

Finish one record-backed, merged IssueOps cycle through the harness-owned
destructive surfaces.

**User's request:** $ARGUMENTS

This skill owns exactly three effects:

1. close the linked parent issue and verify its remote state;
2. remove the recorded local worktree;
3. delete the recorded local branch.

It never deletes the remote branch. Remote branch cleanup is a separate user
decision and a separate `issueops cleanup remote-branch` flow.

## Load first

- Run `issueops next --id "$ISSUEOPS_ID" --json` first and confirm
  `stage.key` is `done`. Anything else belongs to the stage that key names.
- Load **`issueops`** for lifecycle and cleanup-state rules.
- Load **`issueops-remote-write`** before the `remote reflect-completion` and
  `remote close-issue` writes below: it owns the preview, the identical confirm, the
  readback, and the reconcile-instead-of-retry rule for every governed remote mutation.
- Load **`git-operations`** for worktree and branch verification.
- For GitLab issues, also load **`gitlab-usecase`** before any provider call.

## Preconditions

1. Resolve one exact lifecycle ID:
   - prefer a user-supplied ID or `$ISSUEOPS_ID`;
   - otherwise run `issueops list --repo "$PWD" --json`;
   - if zero or multiple plausible cycles remain, ask which ID to clean.
2. Run from the record's source repository, never from the worktree that will
   be removed.
3. The cycle must be `done`, its execution lease must be released, and its
   PR/MR must be verified merged by the provider. Both the `done` phase and the
   released lease come from `issueops execution complete`, which
   **`issueops-complete`** owns; this skill never records completion itself.
4. Linked child tasks must already have verified close evidence.
5. The worktree must be clean. A dirty worktree, active Orca task resource,
   branch mismatch, unreadable provider state, or pending external intent
   blocks cleanup. Processes occupying the worktree and Orca terminals bound to
   it do NOT block: the preview lists them (`workspace_processes` with pid,
   command, start time, descendant/collateral counts; `orca_terminals`) and the
   typed apply stops them itself (fingerprinted handle별 `orca terminal close`, then
   HUP+TERM, then KILL, then re-observation) before removing anything. If the
   Orca terminal close receipt does not confirm the same handle and PTY death,
   apply stops at `workspace_processes_stop` without
   signalling any process. What still blocks is the requester itself: `requester_occupies_worktree` (this
   session or one of its ancestor processes occupies the worktree),
   `requester_terminal_outside_worktree` / `requester_terminal_unresolved`
   (this session's Orca terminal is bound to the target worktree, or the
   `ORCA_PANE_KEY`/`ORCA_TERMINAL_HANDLE` env cannot be matched), and
   `worktree_is_source_checkout`. Resolve those by running from a different
   terminal or worktree — never by killing processes by hand.
6. The remote source branch must be absent, or the user must have chosen to
   leave it. This skill never deletes a remote branch. If one remains, the
   preview blocks on `remote_branch_absent`; report the two exits and let the
   user pick, never pick for them and never run `git push origin --delete`:
   - delete it first through `issueops cleanup remote-branch` (a separate
     decision and a separate flow, out of this skill's authority), then finish;
   - or finish with `--keep-remote-branch`, which leaves the branch on the
     server. Because finish deletes the record, the typed deletion path can no
     longer reach that branch afterwards; the apply result's
     `kept_remote_branch` and the `remote_branch_kept=<branch>@<oid>` fragment
     in the issue audit line are the only remaining trace. Report both back.
   A remote that cannot be read blocks the same way and takes the same two
   exits; `--keep-remote-branch` records `state: "unreadable"` instead of an OID.
7. The merge must have landed on the base the cycle prepared. `cleanup finish`
   compares the record's prepared base with the provider-observed merged base and
   blocks on `base_branch_drifted`. One exemption is observed, not asserted: when
   the prepared base branch is **gone from `origin`** (a stacked PR whose parent
   merged and was deleted) **and** the observed base is the repository's default
   branch, the provider's automatic retarget is accepted and the preview reports
   `retargeted_base` with the prepared base, observed base, default branch, and
   the absence it observed. If either observation fails, the preview blocks on
   `merged_base_remote_unobserved` — there is no flag to assert a base by hand.
   A deliberate retarget (for example a child MR moved onto its umbrella branch
   while the original base is still alive) is recorded **before** finish with
   `issueops branch retarget --id ID --base-branch REF --reason TEXT`: it accepts
   only a base the provider currently shows as the PR/MR target and that exists on
   `origin`, then appends `branch_prepare.retargets[]` and moves the prepared base,
   so finish compares against the recorded decision instead of blocking.
   To check the exemption yourself use `git ls-remote --heads origin
   refs/heads/<prepared-base>` (empty output means gone); do not reach for
   `ls-remote --symref`, which the command policy rejects.

`cleanup abandon` is not this skill's path: it retires a cycle instead of finishing a
merged one. To retire an unmerged cycle — closing the draft PR/MR, the issue, and the
remote branch — use [`issueops-abandon`](../issueops-abandon/SKILL.md).

## Preview the exact targets

Read the lifecycle and cleanup inventory:

```text
issueops status --id "$ISSUEOPS_ID" --json
issueops cleanup status --id "$ISSUEOPS_ID" --merged --json
issueops remote close-issue --id "$ISSUEOPS_ID" \
  --provider "$PROVIDER" --json
```

The close command without `--confirm` is a dry-run. Verify the recorded targets
against live Git state:

```text
git -C "$WORKTREE_PATH" status --short
git -C "$WORKTREE_PATH" symbolic-ref --quiet --short HEAD
git -C "$WORKTREE_PATH" rev-parse --verify HEAD
git -C "$REPO_ROOT" rev-parse --verify "refs/heads/$BRANCH"
```

Require a clean status, a symbolic branch exactly equal to `$BRANCH`, and a
worktree HEAD OID exactly equal to the local branch OID. If the provider exposes
the merged artifact head OID, require it to equal the local branch OID too.
A detached, repurposed, mismatched, or advanced worktree blocks cleanup.

Before asking for confirmation, require `cleanup status.missing` to contain
nothing beyond the two completion gates named below. `cleanup status --merged`
reports the local/remote readiness gates (`pr_phase`, `remote_artifact_*`,
`child_tasks_closed`, `worktree_*`, `branch_match`, `remote_branch_*`), and once
the cycle is `done` with a verified remote artifact it also runs the finish gates,
so it can additionally report `completion_reflected` and `issue_closed`. Those two
are cleared by apply steps 1-2 below, not before them: do not read them as a
blocker to resolve first, and do not wait for an empty `missing` that this ordering
makes unreachable. `cleanup finish --preview` reports the same two entries (only
`issue_closed` once completion is already reflected) and renders the exact
`next_command` for each. After steps 1-2, both previews must report an empty
`missing`.

The close-issue dry-run must also report `ok: true`. Any other missing gate or
provider error blocks issue closure; report it and stop before any write.

From these results, state:

- lifecycle ID and issue URL;
- provider and verified merged PR/MR;
- exact local worktree path;
- exact local branch name, worktree HEAD OID, and local branch OID;
- whether the merged artifact head OID matches the local branch OID, when the
  provider exposes that artifact OID;
- that the remote branch will not be deleted;
- every process and Orca terminal the apply will stop (`workspace_processes`
  with pid/command/start time and descendant/collateral counts, and
  `orca_terminals`), because unsaved work in those processes is lost;
- every readiness blocker.

If any target is missing or ambiguous, or any blocker beyond the permitted
`issue_closed`/`completion_reflected` preview gates remains, stop. Never infer a path or branch from the issue
title.

## Confirmation boundary

Closing the issue and deleting local Git resources are destructive/external
writes. Obtain one explicit confirmation that names all three exact effects:

```text
이슈 <URL>을 닫고, HEAD <WORKTREE_HEAD_OID>인 로컬 워크트리 <PATH>와
OID <BRANCH_OID>인 로컬 브랜치 <BRANCH>를 삭제할까요?
apply가 먼저 종료하는 것: 프로세스 <N>개(<pid:command …>), Orca 터미널 <M>개.
원격 브랜치는 삭제하지 않습니다.
```

When the preview lists no processes or terminals, state that nothing will be
stopped instead of omitting the line.

The latest user message must confirm the exact targets. A prior generic
"cleanup" request authorizes preview only.

## Apply in fail-closed order

### 1. Reflect completion into the issue

`cleanup finish` fails closed with `completion_reflected` in `missing` until the
completion section has been written to the issue; the execution reference
(`skills/issueops/references/execution.md`) orders this before closure:
preserve first, then close.

```text
issueops remote reflect-completion --id "$ISSUEOPS_ID" \
  --provider "$PROVIDER" --json
issueops remote reflect-completion --id "$ISSUEOPS_ID" \
  --provider "$PROVIDER" --confirm --json
```

Continue only when the confirmed result reports `ok: true`. An already-reflected
completion is an idempotent success.

### 2. Close and verify the issue

```text
issueops remote close-issue --id "$ISSUEOPS_ID" \
  --provider "$PROVIDER" --confirm --json
```

Continue only when the result reports `ok: true`, `closed: true`, and a verified
closed state. Already-closed is an idempotent success.

### 3. Re-preview local cleanup

Issue closure changes cleanup readiness. Obtain a fresh fingerprint:

```text
issueops cleanup finish --id "$ISSUEOPS_ID" \
  --provider "$PROVIDER" --preview --json
```

When the user chose to leave the remote branch (precondition 6), pass
`--keep-remote-branch` to this preview too. Without it the preview blocks on
`remote_branch_absent` and issues no fingerprint. With it the preview reports
`kept_remote_branch` and carries the flag into `next_command`.

When `worktree_present` is true, repeat all four live Git commands from
**Preview the exact targets** after issue closure and before apply.

When a prior typed apply already removed the worktree and retained the record
after a later failure, `worktree_present` is false. In that recovery state:

- do not run `git -C "$WORKTREE_PATH"` commands;
- require the prior result or retained record to show the worktree-removal step
  succeeded;
- require `git worktree list --porcelain` not to contain `$WORKTREE_PATH`;
- when `branch_present` is true, re-read `refs/heads/$BRANCH` and require its
  OID to equal the OID previously confirmed by the user;
- when `branch_present` is false, require `git show-ref --verify --quiet
  "refs/heads/$BRANCH"` to exit `1`.

Require:

- `ok: true`;
- an empty `missing` list;
- a non-empty `fingerprint`;
- `worktree_path` and `branch` equal the targets the user confirmed;
- when the worktree remains present, its live HEAD OID and local branch OID
  equal the confirmed OIDs;
- during typed partial-failure recovery, every still-present branch OID equals
  the confirmed branch OID.

If any path, name, or OID changed, stop and request confirmation for the new
targets. Do not reuse a stale fingerprint.

### 4. Apply the emitted command

Execute the preview's `next_command` exactly. It must be the typed form:

```text
issueops cleanup finish --id "$ISSUEOPS_ID" \
  --apply --confirm --fingerprint "$FINGERPRINT" --json
```

Do not replace it with raw `git worktree remove`, `git branch -d`, `git branch
-D`, or `git update-ref`, and never stop processes or Orca terminals by hand
before apply. The harness binds deletion to the observed worktree, branch OID,
occupant receipts, and terminal handles; stops the bound Orca terminals and
occupying processes first (`workspace_processes_stop`); removes Orca ownership
next when present; and preserves the IssueOps record if a destructive step
fails. A `failed_step` of `workspace_processes_stop` means the Orca terminal
stop failed or something still occupies the worktree after HUP/TERM/KILL —
re-preview to see what remains.

Never add `issueops cleanup remote-branch` to this flow, and never delete a
remote branch by hand. When one survives, precondition 6 has the two exits.

## Verify the observable result

After apply:

1. Require `ok: true` and `record_deleted: true`.
2. If the worktree was present, require `worktree_removed: true`, and report
   `workspace_processes_stopped` and `orca_terminals_stopped` exactly as the
   apply returned them.
3. If the local branch was present, require `branch_deleted: true`.
4. From the source repository, verify:

```text
git worktree list --porcelain
git show-ref --verify --quiet "refs/heads/$BRANCH"
```

The removed worktree path must be absent, and `show-ref` must exit `1`.
5. Re-read the remote issue with the provider and require a closed state.
6. Report the issue URL, removed worktree, removed local branch, remote issue
   state, and explicitly state that the remote branch was untouched.

On partial failure, do not improvise. Report `failed_step` and `next_command`;
the retained IssueOps record plus the worktree-aware recovery rules above are
the recovery path.
