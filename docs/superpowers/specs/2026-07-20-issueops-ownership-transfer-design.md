# IssueOps Orca Ownership Transfer Design

**Date:** 2026-07-20
**Status:** Proposed
**Scope:** Separate Orca workspace provisioning from a full IssueOps ownership transfer, scope every fence to the isolated worktree/cycle instead of the source checkout, preserve protocol-v1 lifecycle semantics, and make final resource cleanup a later human decision executed from an authenticated source session.

## 1. Outcome

IssueOps will support a new protocol-v2 **ownership transfer**:

1. The source main session establishes the approved design, linked issue, and provider branch/base gates, then creates and links the isolated Orca worktree.
2. The same sealed preparation session completes the remaining investigation, intent/plan-prep/domain/decision evidence, plan-only commit, tool preparation, compatibility review, execution decision, and Brooks review. Every exact-cycle write is actor-checked in core as well as the hook.
3. `handoff start` seals that completed preparation and dispatches a fresh Codex or Claude Code session.
4. The fresh session claims ownership, acknowledges the sealed issue/plan context, and owns every remaining workflow action: implementation, verification, phase transitions, commits, push, PR/MR creation, and feedback handling.
5. The source main session loses mutation authority **for that exact IssueOps cycle and isolated worktree** immediately after dispatch starts. Its ordinary file and Git work in the source checkout remains available before, during, and after handoff.
6. Owner completion enters durable `cleanup_pending_human_decision`. No task, terminal, worktree, branch, or remote resource is cleaned automatically.
7. A human chooses what to retain or remove. The authenticated source session explicitly selected during that cleanup decision executes the approved path; cleanup authority is never inherited or inferred automatically.

This supersedes the coordinator/worker ownership semantics for **new** handoffs only. Existing protocol-v1 records retain their supervised delegation lifecycle, but the source-checkout exemption is protocol-independent: neither v1 nor v2 may turn the source root into a repository-wide mutation lease.

## 2. Verified incident

The live cycle `io-b9f8cd45e152` was inspected read-only on 2026-07-20.

| Evidence | Observed value |
|---|---|
| Root schema | `7` |
| IssueOps phase at failure reproduction | `problem` |
| Latest readback phase | `compatibility-review` |
| Handoff protocol/state | `1 / coordinator_preparing` |
| Branch | `2589-vertex-prompt-to-be-role-structure` |
| Worktree | `/Users/habin/workspace/api-servers.worktrees/2589-vertex-prompt-to-be-role-structure` |
| Attempt base and live HEAD | `8062c82db06477b4ac25c3f97419915034c80ceb` |
| Missing gates at failure reproduction | `compatibility_review`, `devils_advocate_review`, `execution_decision`, `handoff_worker_claim`, `plan_path`, `worktree_tools_prepared` |
| Latest readiness blocker | `handoff_worker_claim` |
| Latest linked plan | `.agent-harness/plans/2589-vertex-prompt-to-be-role-structure.md` exists but is ignored/untracked |
| Source checkout | clean |
| Isolated worktree | clean |
| Installed/repo binary | identical SHA-256 |
| Daemon | reachable, identity verified, same build SHA |

The source session then hit this repeatable sequence:

1. Plan staging was blocked because the handoff was not dispatched.
2. The block message suggested a minimal `handoff start` command.
3. That exact suggested command was itself blocked because it lacked authenticated coordinator flags.
4. MCP `issueops_link_plan` and `issueops_set_phase` were blocked even though equivalent CLI commands are state-authorized.
5. MCP `issueops_resume` with `bind=true` was blocked; the same read-only resume without binding succeeded.

This investigation made no lifecycle or target-repository mutation. A separate external session later recorded the plan/review gates shown in the latest readback, but did not dispatch or claim the worker: the handoff remains `coordinator_preparing`, both checkouts are clean, and worker `HEAD` remains the sealed base SHA. The time-separated readback does not erase the reproduced hook deadlock; it shows that preparation state can advance independently while the ownership boundary remains unresolved.

## 3. Root-cause chain

### 3.1 Workspace creation activates the ownership fence too early

`PrepareIssueOpsHandoffWorktree` calls `beginHandoffWorktreeCreate` before the external Orca create. That helper calls `handoff.Prepare`, which immediately creates `ExecutionHandoff{state: coordinator_preparing}`. See:

- `internal/core/issueops/issueops_handoff_prepare.go:78-231`
- `internal/core/issueops/issueops_handoff_prepare.go:424-458`
- `internal/core/issueops/handoff/state.go:72-103`

The preparation prerequisites require only the approved design, linked issue, provider branch, and base SHA. They do **not** require the plan, prepared tools, compatibility review, execution decision, or Brooks review:

- `internal/core/issueops/issueops_handoff_prepare.go:625-641`

Therefore a physical workspace operation creates a logical ownership fence before ownership transfer is ready.

### 3.2 Dispatch requires the gates that the early fence blocks

`IssueOpsPreDispatchReadiness` derives implementation readiness, removes only `handoff_worker_claim`, and adds the Orca worktree requirements. `StartIssueOpsHandoff` refuses dispatch while any remaining gate is missing:

- `internal/core/issueops/issueops_handoff_dispatch.go:80-96`
- `internal/core/issueops/issueops_handoff_dispatch.go:103-215`

This creates a circular dependency:

~~~text
create worktree
  -> create coordinator_preparing handoff
  -> block ordinary setup mutation
  -> require setup gates before dispatch
  -> dispatch cannot happen
~~~

### 3.3 The documented preparation path and the hook authority path disagree

The IssueOps handoff reference explicitly requires the coordinator to write and commit the plan, prepare tools, and finish reviews while `coordinator_preparing`. The hook permits only two narrow exceptions:

- Markdown edit tools targeting an approved plan root.
- Exact Git argv with an **absolute** plan path.

See:

- `skills/issueops/references/orca-handoff.md`, “Current Cycle Plan Checkpoint”
- `internal/core/lifecycle/lifecycle_handoff_guard.go:268-293`
- `internal/core/lifecycle/lifecycle_handoff_guard.go:413-499`

The live staging command used the repository-relative plan path. The exact grammar requires:

~~~text
git -C <absolute-worker-root> add -- <absolute-plan-path>
~~~

Failing closed was correct, but the returned message incorrectly instructed the agent to dispatch instead of naming the allowed absolute-path form. This turned a narrow argv mismatch into an apparent lifecycle dead end.

### 3.4 Coordinator bootstrap guidance is unreachable from the emitted command

An allowed `handoff start` requires exact native host/session/agent flags and `source-cwd`:

- `internal/core/lifecycle/lifecycle_handoff_authority.go:81-134`

The bootstrap recognizer can fill those native flags, but only when the attempted command already contains a concrete `--coordinator-recipient term_*`:

- `internal/core/lifecycle/lifecycle_handoff_authority.go:773-807`

The generic mutation block emits a command without `--coordinator-recipient` or native identity flags:

- `internal/core/lifecycle/lifecycle_handoff_guard.go:284-293`

The cycle was created after the source session's SessionStart hook, so there was no later SessionStart opportunity to inject an identity-complete command. The suggested escape and the bootstrap recognizer cannot meet each other.

### 3.5 CLI and MCP authority are not equivalent

The exact CLI authority path permits `link-plan`, `phase`, compatibility review, execution decision, Brooks review, and worktree tool preparation during `coordinator_preparing`. The MCP handoff authority function recognizes only `issueops_handoff`, heartbeat, and remote publication tools:

- `internal/core/lifecycle/lifecycle_handoff_authority.go:81-134`
- `internal/core/lifecycle/lifecycle_handoff_authority.go:1286-1359`

Consequently `issueops_link_plan` and `issueops_set_phase` fall into the generic pre-claim mutation block. This violates the repository invariant that CLI and MCP expose the same host-neutral meaning.

### 3.6 `resume bind=true` is a separate, correctly blocked mutation

`issueops_resume` is an observation only when `bind` is absent or false:

- `internal/core/lifecycle/lifecycle_handoff_authority.go:843-892`
- `cmd/harness/mcpcli/mcp_tool_issueops_handlers.go:363-373`

Binding changes session state and must not transfer a supervised lease. The block itself is correct; the generic “payload does not match” diagnostic should instead say that `bind=true` is mutating and provide the binding-free payload.

### 3.7 The current protocol is supervised delegation, not full ownership transfer

Protocol v1 has this normal state path:

~~~text
coordinator_preparing -> dispatched -> claimed -> submitted -> closed/accepted
~~~

The claimed worker may implement, verify, and commit locally. It may not push, create a PR/MR, accept, recover, or clean up. The coordinator must accept the result and then publish:

- `internal/core/issueops/handoff/state.go:161-279`
- `internal/core/lifecycle/lifecycle_handoff_guard.go:310-313`
- `.agent-harness/ARCHITECTURE.md:283-334`

Even if every early allowlist defect were patched, protocol v1 would still contradict the requested ownership contract.

### 3.8 The supervised selector turns the source checkout into the fence target

`selectSupervisedHandoffRecord` first selects exact worker-root, lifecycle-ID, and worker-target matches. If none match, it falls back to any supervised record whose `record.Repo` equals the request CWD. That fallback deliberately captures an ID-less source mutation:

- `internal/core/lifecycle/lifecycle_handoff_authority.go:917-1012`
- `internal/core/lifecycle/lifecycle_handoff_fence_scope_test.go:11-75`

Once selected, `handoffOwnershipBlockReason` handles the request and blocks it while the handoff is unclaimed, recovery-required, or owned by a different native session. Because the request is marked handled, the ordinary worktree guard and mirror warning paths never get a chance to classify it as unrelated source work:

- `internal/core/lifecycle/lifecycle_handoff_guard.go:173-327`
- `internal/core/lifecycle/lifecycle_state.go:40-62`

The generic worktree guard has related source-root coupling: it can block a source path because the same relative path exists in a linked worktree, and a session binding can turn that collision into an `ask` decision:

- `internal/core/lifecycle/lifecycle_worktree_guard.go:25-65`
- `internal/core/lifecycle/lifecycle_worktree_guard.go:68-113`
- `internal/core/lifecycle/lifecycle_worktree_guard.go:138-210`

These rules protect against editing the wrong copy, but they implement the wrong ownership boundary. A linked worktree owns its own canonical root, branch, cycle, and native owner; it does not own the source checkout or every same-relative-path file in that checkout.

## 4. Hopper diagnosis

- **Symptom:** the source coordinator cannot finish pre-dispatch setup after creating the Orca worktree.
- **First failing boundary:** `worktree prepare --orchestrator orca --confirm` persists `ExecutionHandoff` before setup gates are complete.
- **Eliminated hypotheses:** stale binary, stale daemon, wrong worktree HEAD, dirty source checkout, and missing Orca runtime readiness.
- **Primary cause:** physical workspace provisioning and logical ownership transfer share one state object and one fence activation point.
- **Contributing causes:** source-root fallback selection, mirrored-path blocking, unreachable identity bootstrap, CLI/MCP authority asymmetry, and misleading narrow-grammar diagnostics.
- **Semantic cause:** the existing worker role is intentionally subordinate to a coordinator; it cannot satisfy full handoff by permission expansion alone.
- **Cheapest reliable correction:** make source-checkout availability a protocol-independent invariant, separate workspace state, and activate a new protocol-v2 fence only at confirmed dispatch.

## 5. Mandatory invariants

### 5.1 Source main availability

The source checkout is never part of an IssueOps mutation lease. Before and after handoff, in every v1 and v2 handoff state, ordinary tools and commands whose resolved targets stay inside the source root are not selected by an IssueOps fence.

This is a structural rule, not an intent heuristic:

- an exact cycle lifecycle command is selected by its cycle ID and remains role/state-gated;
- an operation targeting the canonical isolated worktree is selected by that worker root and remains owner-gated;
- an Orca terminal/task/dispatch mutation is selected by its persisted resource identity and remains lifecycle-gated;
- checking an IssueOps branch owned by a ready workspace or active handoff out in the source checkout, moving/deleting the isolated worktree, or using a cross-root command remains blocked;
- an ordinary source-root edit, test, branch, commit, or other Git operation with no isolated-worktree/cycle target is allowed even if a session binding, active handoff, mirrored relative path, recovery state, or cleanup state exists;
- an exact command for a different cycle, including starting and preparing another parallel isolated worktree, is not captured by the first cycle's fence;
- unrelated safety policies such as command policy, staged checks, VCS linking, and destructive-operation guards still apply independently.

“Source-only” must be proven from the hook event, not assumed from source CWD. Native file targets must resolve inside the source root; mutating shell/MCP requests must either have an exact parser-proven source-local cwd/target or belong to a bounded root-local command grammar. Active expansion, nested shell, unresolved redirect/path, or non-literal Orca resource control that could reach a worker root is an ambiguous cross-root request and fails closed with a literal source-root rewrite. A literal Orca handle proven not to belong to any IssueOps record remains unrelated and is allowed. This preserves normal source work without turning “unknown target” into a worker-fence bypass.

Session binding is routing metadata only. It cannot convert the source checkout into a worker root or require human approval for a same-relative-path source edit. The existing source-misdirection signal may remain as non-blocking context, but it cannot return `block` or `ask` solely because a linked worktree contains the same path.

“The owner's own task” is resolved structurally as the sealed cycle ID, canonical worker root, exact branch, fence tuple, native owner identity, committed plan, and sealed `worker_scope`. The hook does not guess semantic relevance from file contents. Non-owner/cross-cycle mutation is prevented at tool time; owner drift from the committed plan is rejected by changed-file, phase-readiness, and completion evidence checks.

### 5.2 Preparation ownership

Before confirmed handoff dispatch, the source main session owns:

- issue and related-item investigation;
- design and domain review;
- provider-linked branch and isolated worktree provisioning;
- plan creation, link, and plan-only commit;
- worktree tool preparation;
- compatibility review;
- execution decision;
- Brooks review;
- context options and verification commands.

No `ExecutionHandoff` exists during this preparation. A ready workspace alone must not activate the ownership-transfer hook fence.

Workspace confirmation requires the approved design, linked issue, and verified provider branch/base evidence, then seals `PreparationSession` from the authenticated native event. Repo, issue, provider branch/base, coordinator root, worker root, and workspace epoch are immutable physical identity while the workspace exists.

Only `execution_workspace.state=ready` grants `PreparationSession` an isolated-root setup lease and exact-cycle preparation authority: intent, domain review, decisions, plan-prep, design revisions, routing, related/child topology, plan link, phase, worktree tools, compatibility review, execution decision, Brooks review, and explicit replan. Actor validation and mutation occur under the same cycle lock. Actorless compatibility entry points reject when a workspace lease exists.

`provisioning` and `recovery_required` grant no setup mutation: they allow status plus exact journal reconciliation or human-approved preparation-session rebind appropriate to the sealed actor and current inventory. Source tools cannot race the external create/adopt journal or edit a workspace before exact ready readback. Other sessions in the source checkout remain free to do ordinary source-root work in all three states, but cannot become a second preparer. Missing native identity is bootstrapped into an exact replacement command; it is never guessed. An explicit human-directed workspace recovery is required if the preparation session disappears before transfer.

### 5.3 Transfer boundary

Confirmed `handoff start` is the only normal boundary that transfers authority. It must:

1. verify all pre-dispatch gates;
2. require phase `compatibility-review` with no implementation change;
3. verify an exact clean branch and worktree;
4. require the linked plan to be a tracked regular file in `HEAD`;
5. require `base_sha..HEAD` to contain only the linked plan path for the first attempt;
6. seal `HEAD` as `attempt_base_head`;
7. seal the workspace epoch/fingerprint, context hash, source native identity, and source Orca mailbox;
8. write the pending external operation before invoking Orca;
9. activate the fence only after that durable write;
10. create/adopt exactly one owner terminal, task, and dispatch;
11. never retry an ambiguous external mutation automatically.

Preview mode performs steps 1-7 read-only and persists nothing.

### 5.4 New-owner orientation

Claim moves protocol v2 to `owner_orienting`, not directly to writable execution. The owner must read the linked issue and plan, then record:

- exact cycle, attempt, epoch, and context hash;
- exact issue URL;
- exact linked plan SHA-256;
- a bounded understanding summary;
- a bounded scope confirmation.

Only then does the state become `owner_active` and permit implementation mutation.

### 5.5 Owner authority

While `owner_active`, the owner session alone may:

- enter and advance `implement -> ai-slop-clean -> feedback -> pr`;
- edit only the canonical isolated worktree;
- run verification and Turing evidence collection;
- create local commits;
- publish the exact branch through the guarded publication wrapper;
- create and verify the PR/MR;
- process and resolve feedback;
- refresh the exact publication receipt after additional feedback commits;
- complete the ownership attempt.

For this exact cycle, the source main session may run only status/resume and explicit recovery previews. It cannot phase, accept, publish, create cycle remote artifacts, steer the owner terminal, or send implementation instructions. This cycle-scoped loss of authority does not restrict ordinary source-checkout work unrelated to the cycle or isolated worktree.

### 5.6 Completion

Protocol-v2 completion has no coordinator accept step. A completed outcome requires:

- `owner_active` and exact owner identity;
- phase `pr`;
- clean canonical worktree;
- final local HEAD;
- latest publish receipt for that exact HEAD;
- verified PR/MR whose source, target, project, labels, assignees, title/body, and head SHA match;
- no unresolved remote-create claim;
- no unresolved durable feedback;
- bounded changed files, Turing report, and verification evidence.

The same cycle-lock write:

1. records the immutable completion;
2. stamps the IssueOps phase to `done`;
3. changes handoff state to `cleanup_pending_human_decision`;
4. records a cleanup inventory fingerprint;
5. records one no-retry completion-notification intent.

The external `worker_done` message is notification only. It does not accept, publish, close, stop, remove, or clean any resource.

### 5.7 Human-directed cleanup

`cleanup_pending_human_decision` has no default mutation. The source main session presents exactly these operational choices:

1. retain everything for now — execute no command and keep the state pending;
2. close the owner task/terminal but retain worktree and local branch;
3. close the owner task/terminal and remove the exact local worktree and local branch after remote-safety verification.

Any authenticated native session other than the sealed owner may run the read-only preview only when its canonical CWD equals the sealed source root. Preview returns the candidate session identity, exact completion/resource inventory, and a fingerprint; it grants no authority. This keeps cleanup reachable when the original preparation session has exited without silently transferring its lease or allowing the completed owner to clean its own resources.

Only choices 2 and 3 create a durable cleanup approval. Approval requires:

- an authenticated native session running from the exact sealed source root;
- a candidate identity different from `OwnerSession`;
- the exact candidate session identity emitted by preview;
- the exact cleanup inventory fingerprint shown to the human;
- an explicit disposition and bounded reason;
- `--confirm`;
- fresh inventory equal to the presented fingerprint.

The confirm write atomically seals that candidate as `Cleanup.ApprovedBySession` after revalidating the source root, cycle/attempt/epoch, completion HEAD, resource inventory, and absence of another cleanup executor. The original preparation session has no permanent privilege: it must go through the same preview/confirm path. An approval cannot be rebound once an external cleanup receipt exists; an interrupted approval requires explicit recovery and fresh human confirmation.

After approval, only `Cleanup.ApprovedBySession` may execute the exact ordered cleanup commands and receipt recorders. Every external mutation is followed by readback. No cleanup code runs from owner completion, Stop hooks, background jobs, daemon startup, stale scan, TTL, or operational-health checks.

Remote branch deletion, merge, issue closure, and deploy are outside these local cleanup dispositions. They require their own later human instruction and existing provider readiness checks.

## 6. Durable model

Root IssueOps schema advances from 7 to 8. Schema 8 accepts both handoff protocols. An older binary sees schema 8 as future and fails safe.

### 6.1 Workspace state

Add `execution_workspace` outside `execution_handoff`:

~~~go
type IssueOpsExecutionWorkspace struct {
    State           string // provisioning | ready | recovery_required
    WorkspaceEpoch  string
    Driver          string // orca
    Agent           string // codex | claude
    CoordinatorRoot string
    WorkerRoot      string
    PreparationSession *IssueOpsHostSessionIdentity
    BaseHead        string
    Orca            *IssueOpsOrcaIdentity
    PendingOperation *IssueOpsExecutionWorkspacePendingOperation
    Failure         *IssueOpsExecutionHandoffFailure
    PreparedAt      string
    ProvisionedAt   string
    UpdatedAt       string
}
~~~

The workspace Orca identity may contain repo/worktree identity and baseline terminal IDs. It must not contain owner terminal, task, dispatch, or mailbox authority.

Workspace create keeps the existing “journal before mutation, timeout is not absence, exactly one reconciliation candidate” rules. Its recovery command cannot dispatch an agent.

### 6.2 Protocol-v2 handoff state

Keep protocol-v1 fields and validators unchanged. Add protocol-v2 fields additively:

~~~go
type IssueOpsExecutionHandoff struct {
    ProtocolVersion int
    State           string
    ClosedDisposition string

    Attempt         int
    OwnershipEpoch  string
    AttemptBaseHead string
    WorkspaceEpoch  string
    WorkspaceSHA256 string
    ContextSHA256   string
    ContextSourceSHA256 string

    CoordinatorSession *IssueOpsHostSessionIdentity
    CoordinatorMailboxHandle string
    OwnerSession       *IssueOpsHostSessionIdentity
    Orientation        *IssueOpsOwnershipOrientation
    Completion         *IssueOpsOwnershipCompletion
    Cleanup            *IssueOpsOwnershipCleanup

    Orca             *IssueOpsOrcaIdentity
    PendingOperation *IssueOpsExecutionHandoffPendingOperation
    Failure          *IssueOpsExecutionHandoffFailure
    WorkerDoneProjection *IssueOpsExecutionHandoffWorkerDoneProjection
}
~~~

`IssueOpsOwnershipCleanup` includes the preview fingerprint, selected disposition, bounded reason, `ApprovedBySession`, ordered receipts, and recovery evidence. `ApprovedBySession` is absent in `cleanup_pending_human_decision` and is written only by the human-confirmed approval transition.

Protocol-v2 states:

~~~text
ownership_dispatching
  -> ownership_dispatched
  -> owner_orienting
  -> owner_active
  -> cleanup_pending_human_decision
  -> cleanup_executing
  -> closed

Any ambiguous external operation -> recovery_required
~~~

There is no `submitted` or `closed/accepted` state in protocol v2.

### 6.3 Protocol compatibility

- Keep protocol v1 readable and writable under its existing state machine.
- Never convert any protocol-v1 attempt as a side effect of schema migration, startup, stale scan, or another lifecycle command.
- New Orca handoffs use protocol v2.
- Root schema migration is structural only; it does not change protocol semantics.
- Response projections expose `handoff_mode: supervised_v1 | ownership_transfer_v2`.

Existing protocol-v1 records are not converted. A human may later finish them under the unchanged v1 contract, or explicitly cancel and start a new v2 cycle after separately reviewing the live resources. Schema migration alone never changes authority.

## 7. Hook and identity design

### 7.1 Fence selection is worker-root and cycle scoped

Every mutating request is first classified as `source_only`, `worker_or_cycle_targeted`, or `ambiguous_cross_root`. `source_only` bypasses IssueOps fence selection; `ambiguous_cross_root` fails closed without guessing a cycle. `worker_or_cycle_targeted` then selects a record in this order:

1. exact persisted Orca resource handle in `execution_workspace` or `execution_handoff` for terminal/task/dispatch control;
2. exact canonical worker CWD or resolved mutation target;
3. exact lifecycle cycle ID in a CLI/MCP request;
4. otherwise no handoff record is selected.

There is no source-CWD or singleton-active-record fallback. Literal-safe terminal/task/dispatch extractors must prove the resource handle against workspace and handoff inventory; unresolved resource control from a worker root remains fenced, while unresolved resource control from source fails as ambiguous rather than being assigned to an arbitrary cycle. A source-only request falls through to the remaining independent safety guards.

The protocol-independent worktree guard must likewise stop treating same-relative-path source files as leased. `ExpectedWorktree` and session binding constrain a worker session only when its CWD or target resolves to that worker root; they do not redirect or fence the source main session.

### 7.2 No fence while workspace is ready

`execution_workspace.state=ready` is not selected by the supervised handoff guard. IssueOps isolation prevents non-owner or cross-cycle mutation **inside the linked worktree** and prevents checking the issue branch out in the source checkout; it does not block unrelated source-root work.

`execution_workspace.state=recovery_required` has a separate narrow guard that permits only observation and explicit workspace reconciliation. Ambiguous physical provisioning must not be treated as a writable ready workspace.

The ready-workspace guard is an isolated-root preparation lease keyed by `WorkspaceEpoch + PreparationSession`; it is not a source-root fence. Exact preparation CLI/MCP recorders carry native actor fields and core revalidates them, so bypassing the native hook cannot mutate the plan, phase, tool-preparation, compatibility, execution-decision, or Brooks-review evidence.

Generic cycle start/stale-reset treats every `execution_workspace` state as durable resource authority. A missing worktree cannot erase or reset that record merely because `execution_handoff` is absent; it returns explicit workspace-recovery guidance and leaves the record unchanged.

### 7.3 Reachable source bootstrap

Before a v2 handoff exists, a source session may attempt:

~~~bash
agent-harness issueops handoff start \
  --id <cycle-id> \
  --source-cwd <source-checkout> \
  --json
~~~

PreToolUse blocks that unauthenticated probe once and renders an exact command containing the native host/session/agent identity from the hook event. `--coordinator-recipient` is optional; core resolves it read-only from exactly one connected+writable source terminal. The second identity-complete command is allowlisted.

This path works in a long-lived session and does not depend on SessionStart having occurred after cycle creation.

### 7.4 Protocol-v2 role matrix

| State | Exact-cycle authority from source main | New owner |
|---|---|---|
| no handoff / workspace ready | exact sealed-preparer commands | no role |
| `ownership_dispatching` | exact start/recover/status only | no role |
| `ownership_dispatched` | status/recovery preview only | claim/status |
| `owner_orienting` | status only | read-only exploration, heartbeat, context acknowledge |
| `owner_active` | status only | implementation, phases, verification, commit, publish, PR/MR, feedback, complete |
| `cleanup_pending_human_decision` | status and cleanup preview; explicit candidate approval | status only |
| `cleanup_executing` | exact approved cleanup sequence by `Cleanup.ApprovedBySession` | status only |
| `closed` | status | status |
| `recovery_required` | status and explicit human-directed recovery | status |

The table governs only exact-cycle and isolated-worktree operations. Ordinary source-root work is outside the table and remains available in every row. The shared authority table must include protocol, state, role, command/tool kind, native identity, root, and fence. Protocol-v1 lifecycle rows retain their current meaning, while the protocol-independent source-root exemption applies to v1 as well.

“Source main” in the cleanup rows means an authenticated session in the exact sealed source root, not necessarily the original preparation session. Preview is open to such a session; mutation authority begins only when the human-confirmed approval seals it as `Cleanup.ApprovedBySession`.

### 7.5 CLI/MCP parity

Every new action is available through the existing `issueops_handoff` MCP action-discriminated tool and CLI:

- `start`
- `claim`
- `acknowledge-context`
- `publish`
- `complete`
- `cleanup-preview`
- `cleanup-approve`
- `cleanup-record`
- `recover`

Generic IssueOps MCP tools used before handoff are not intercepted by a nonexistent handoff fence. Owner-only phase, ai-slop-clean, feedback, publication, and remote-create requests carry native actor fields when protocol v2 is active, and core revalidates them in addition to the hook.

Invalid observation payloads receive field-specific diagnostics. In particular, `issueops_resume {bind:true}` must say to omit `bind` or set it to false.

The Git topology guard parses both branch creation and selection. Literal `git checkout <issue-branch>` / `git switch <issue-branch>` in the source root is blocked when that branch belongs to a ready workspace or active isolated cycle. Dynamic/unresolved branch selection while isolated IssueOps branches exist fails with guidance to use one literal non-IssueOps branch; path checkout (`git checkout -- <path>`) is not misclassified.

## 8. Publication and feedback

Protocol v1 keeps `submit -> coordinator accept -> coordinator publish -> coordinator PR/MR`.

Protocol v2 permits the exact owner to:

1. enter `pr` after strict readiness;
2. run `handoff publish` from the canonical worker root;
3. create the draft PR/MR with native identity fields;
4. resolve review feedback and commit again;
5. replace the latest publication receipt only when project/remote/branch/base are unchanged and the new head is a descendant of the previous published head;
6. reverify the remote artifact at the new head;
7. call `handoff complete`.

Raw push and unscoped provider creation remain blocked. Provider create/reconcile retains its durable claim/no-retry behavior.

## 9. Cleanup protocol

Completion stores a canonical fingerprint over:

- cycle, protocol, attempt, epoch, and final head;
- source and owner native identities;
- Orca runtime/worktree instance/terminal/task/dispatch identities;
- local branch and worktree path;
- remote, remote branch, publish receipt, and PR/MR URL;
- current clean status.

The fingerprint excludes timestamps and presentation fields.

The source main runs a read-only preview first. The selected approval then authorizes one ordered sequence:

### 9.1 Close owner, retain worktree

1. mark exact Orca task terminal and verify;
2. close/stop exact owner terminal and verify absent or non-writable;
3. record receipts;
4. close with disposition `owner_closed_workspace_retained`.

### 9.2 Remove local resources

1. verify remote branch and PR/MR still contain the completion head;
2. mark exact Orca task terminal and verify;
3. close/stop exact owner terminal and verify;
4. remove exact Orca worktree without force and verify absence;
5. delete the exact local branch only when its full OID still equals the completion head and that head is remotely reachable;
6. record ordered receipts;
7. close with disposition `local_resources_removed`.

Any identity, OID, inventory, or fingerprint drift stops before the next mutation. A started ambiguous cleanup operation remains `recovery_required` and is never guessed complete.

### 9.3 Operational-health and stale-scan semantics

Protocol-v2 operational classification recognizes every ownership state. `ownership_dispatching`, `ownership_dispatched`, `owner_orienting`, and `owner_active` are live or recovery-evaluated using `OwnerSession`, pending-operation, and heartbeat evidence. `cleanup_pending_human_decision` is an intentional human-pending state, not dead, stale, releasable, or inconsistent merely because the IssueOps phase is `done`. `cleanup_executing` and cleanup `recovery_required` retain their exact executor/resource authority until explicit completion or recovery.

Stale scan, binding pruning, done-cycle pruning, daemon startup, and worktree orphan cleanup must preserve every non-`closed` protocol-v2 cycle and its Orca resources. Even with `apply=true`, zero age thresholds, or a missing original source session, these paths may report human cleanup pending but cannot release the fence, remove the binding, close a task/terminal, prune the record, remove a worktree, or delete a branch. Only the approved cleanup state machine may perform those mutations.

The same restriction applies to direct and CAS generic force release: neither may mark a non-`closed` protocol-v2 handoff done, stamp an orphan path, unbind it, or bypass its explicit recovery/cleanup state machine.

## 10. Rejected alternatives

### 10.1 Expand the `coordinator_preparing` allowlist

This would unblock the current plan command but retain the conceptual error that a worktree is an ownership lease. It would also retain worker submission/accept and coordinator publication, so it does not meet the user contract.

### 10.2 Make `coordinator_preparing` non-fencing inside the same v1 record

This reduces the deadlock but gives one state two incompatible meanings: physical workspace ready and logical handoff active. Recovery, envelope validation, stale classification, and hook authority would continue to infer the wrong owner from the same field.

### 10.3 Grant protocol-v1 workers push and PR permissions

Permission expansion leaves `submitted -> accept` and coordinator cleanup/steering semantics intact. It weakens an established protocol without producing a clean ownership-transfer boundary.

### 10.4 Auto-clean on owner completion

Completion does not prove merge state, human retention intent, terminal safety, or whether the worktree is still useful for review. Automatic cleanup is destructive, surprises the user, and contradicts the explicit human-in-the-loop requirement.

### 10.5 Auto-migrate live v1 records

A v1 record may own ambiguous external mutations or a real worker. Automatic or in-place conversion could create two writers and would make schema upgrade change authority. Existing v1 cycles stay v1; any cancel-and-restart decision is a separate, explicit human operation after live-resource review.

### 10.6 Ask or block when a source path has a worktree mirror

A same-relative-path collision is useful orientation context but is not proof that the source edit belongs to the cycle. Returning `ask` or `block` would still make the main worktree availability depend on a parallel fence. Keep an optional non-blocking warning and enforce exclusivity only at the isolated root, exact cycle command, or persisted Orca resource.

## 11. Karpathy handoff prompt contract

The v2 context packet must use the following strict order:

1. **Identity:** “You are the sole IssueOps owner for cycle/attempt/epoch after claim and context acknowledgement.”
2. **Objective:** exact interpreted intent and success criteria.
3. **Required first action:** read the linked issue and committed plan, then record the bounded acknowledgement.
4. **Scope:** allowed files/actions and explicit non-goals.
5. **Ownership:** owner performs implementation through PR/MR and feedback; source main is not a reviewer or publisher for this cycle but remains free to perform other source-root work.
6. **Safety:** the owner performs no source-checkout edits, raw push, unscoped remote writes, or cleanup.
7. **Phases:** exact commands and readiness gates.
8. **Verification:** exact commands and Turing evidence requirements.
9. **Completion:** exact `handoff complete` fields and remote artifact requirements.
10. **Stop conditions:** context drift, ambiguous inventory, unrelated WIP, failed required verification, or a needed human decision.
11. **Output contract:** orientation acknowledgement first; bounded progress; one final completion notification.

No transcript, environment dump, secret, cleanup authorization, or implicit coordinator acceptance is included.

## 12. Success criteria

- Confirmed Orca worktree preparation leaves `execution_handoff` absent and all planning gates usable.
- CLI and MCP can link plan, prepare tools, record reviews, and set preparation phases before transfer.
- Start preview is read-only; confirmed start is the first fence-activating write.
- The minimal source bootstrap always emits a runnable identity-complete command.
- In every v1/v2 handoff state, an ID-less mutation confined to the source root is not selected or blocked by IssueOps, even with a session binding or same-relative-path file in the isolated worktree.
- A source session can start, prepare, and hand off a different exact cycle while another isolated owner remains active.
- Exact cycle commands, isolated-worktree targets, IssueOps branch topology changes, and persisted Orca resource mutations remain fenced.
- Dynamic or nested mutations that cannot prove source-only scope fail closed with a literal-source rewrite; this cannot select or release an arbitrary cycle.
- A ready workspace seals one native preparation session, and every isolated-root/pre-dispatch recorder rejects a second source session or missing actor.
- A claimed owner cannot edit until context acknowledgement.
- After acknowledgement, only the owner can implement, phase, publish, create the PR/MR, and resolve feedback.
- The source main cannot accept, publish, steer, or mutate that cycle's workflow state after transfer; its unrelated source-root work remains available.
- Completion atomically enters `cleanup_pending_human_decision` and never invokes cleanup.
- Cleanup requires a later exact human-selected disposition and execution by the authenticated source session sealed by that cleanup approval; a new source session can be selected explicitly if the original one has exited.
- Operational health, stale scan, binding pruning, and done-cycle pruning preserve `cleanup_pending_human_decision` and all non-`closed` protocol-v2 resources even in apply mode.
- Protocol-v1 cycles retain their state machine, role ownership, and external-operation semantics; they also receive the protocol-independent source-root fence correction.
- Existing protocol-v1 cycles are never converted by schema migration or a background path.
- CLI, MCP, Codex, Claude Code, and GJC hook contracts agree.
- Golden contracts, focused tests, race tests, build, and self-verification pass.
