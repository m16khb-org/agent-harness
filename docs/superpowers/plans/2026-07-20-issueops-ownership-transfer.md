# IssueOps Orca Ownership Transfer Implementation Plan

> **For the implementation owner:** REQUIRED SUB-SKILL: use `superpowers:test-driven-development` for every behavior change, `superpowers:executing-plans` in the fresh isolated session, and `superpowers:verification-before-completion` before any completion claim.

**Goal:** Keep the source main worktree available for unrelated work at all times, let its session finish all IssueOps planning and setup in an isolated Orca worktree, then transfer the complete cycle lifecycle to a fresh Codex or Claude Code owner while reserving final resource cleanup for a later human decision executed by an explicitly selected authenticated source session.

**Architecture:** First make fence selection protocol-independent and structural: exact isolated worker root, exact cycle ID, exact owner identity, or exact persisted Orca resource, never source CWD alone. Add a schema-v8 `execution_workspace` record that owns physical Orca worktree provisioning without activating the handoff fence. Add protocol-v2 ownership-transfer states whose fence begins only at confirmed dispatch, whose claimed owner performs implementation through PR/MR and feedback, and whose completion stops at `cleanup_pending_human_decision`. Preserve protocol-v1 state/external-operation semantics without converting existing v1 records.

**Tech stack:** Go 1.26, sqlstore-backed IssueOps JSON records, Orca public CLI adapter, native Codex/Claude/GJC lifecycle hooks, Cobra/flag CLI, MCP stdio adapter, Go tests and response-contract goldens.

**Design source:** `docs/superpowers/specs/2026-07-20-issueops-ownership-transfer-design.md`

---

## Global execution contract

- Execute this plan in a fresh isolated worktree created from the implementation target branch. Do not implement it in the source checkout used to coordinate the work.
- Read the design source, the linked implementation issue, `AGENTS.md`, `.agent-harness/CONSTITUTION.md`, `.agent-harness/ARCHITECTURE.md`, `.agent-harness/TESTING.md`, `.agent-harness/CAUTIONS.md`, and `skills/issueops/SKILL.md` before editing.
- Treat `io-b9f8cd45e152` as read-only production evidence. Tests must use temporary state roots and fake Orca clients. Do not convert, dispatch, cancel, close, or clean the live cycle during implementation.
- Preserve unrelated worktree changes. Stop if any planned file already has overlapping user changes that cannot be isolated.
- Use one behavior per test. Run the named RED test before implementation, then the same named GREEN test after the minimum code change.
- Never run an Orca process while a cycle sqlstore lock is held.
- Never infer absence from a timeout. Persist a pending operation before each external mutation and require exact-one-candidate reconciliation.
- Protocol v1 state, role, and external-operation behavior is a compatibility contract. Protocol-v2 code must branch explicitly by protocol; do not reinterpret v1 states. The source-root fence correction in Task 1 deliberately applies to both protocols.
- Never classify intent from same-relative-path files. An ordinary source-only request is outside IssueOps authority; exact-cycle commands, isolated-root targets, IssueOps branch-topology changes, and persisted Orca resource mutations remain fenced.
- No Stop hook, daemon path, stale scan, completion projection, or owner command may execute cleanup.
- Every vertical task that changes a public CLI/MCP response, action schema, usage surface, or schema projection owns the corresponding golden updates and runs both contract-golden suites before its commit. The later parity sweep may detect drift but may not defer or repair an earlier task's contract breakage.
- Do not run `./scripts/install-native.sh` or mutate the installed runtime until a human separately approves dogfood installation.

## Required end-state matrix

| Stage | Ordinary source-root work | Exact-cycle authority from source | New-owner authority | Durable state |
|---|---|---|---|---|
| Plan/setup | allowed | prepare and commit plan, tools, reviews | none | `execution_workspace.ready`, no handoff |
| Dispatch | allowed | exact start/recover/status | none | `ownership_dispatching` then `ownership_dispatched` |
| Orientation | allowed | status only | read, heartbeat, acknowledge | `owner_orienting` |
| Execution | allowed | status only | phases, edit, verify, commit, publish, PR/MR, feedback | `owner_active` |
| Completion | allowed | status and cleanup preview only | status only | `cleanup_pending_human_decision` |
| Cleanup | allowed | exact approved cleanup sequence | status only | `cleanup_executing` then `closed` |

## Task 1: Scope every IssueOps fence to the isolated worktree or exact cycle

**Files:**

- Modify: `internal/core/lifecycle/lifecycle_handoff_authority.go`
- Modify: `internal/core/lifecycle/lifecycle_state.go`
- Create: `internal/core/lifecycle/lifecycle_handoff_scope.go`
- Create: `internal/core/lifecycle/lifecycle_handoff_scope_test.go`
- Create: `internal/core/lifecycle/lifecycle_handoff_resource_target.go`
- Create: `internal/core/lifecycle/lifecycle_handoff_resource_target_test.go`
- Modify: `internal/core/lifecycle/lifecycle_worktree_guard.go`
- Modify: `internal/core/lifecycle/lifecycle_worktree_mcp.go`
- Modify: `internal/core/lifecycle/dependencies.go`
- Create: `internal/core/lifecycle/dependencies_test.go`
- Modify: `internal/core/lifecycle/worktreepath/shell_paths.go`
- Create: `internal/core/lifecycle/worktreepath/shell_paths_test.go`
- Modify: `internal/core/lifecycle/worktreeguard/branch_creation.go`
- Modify: `internal/core/lifecycle/worktreeguard/branch_creation_test.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_fence_scope_test.go`
- Modify: `internal/core/lifecycle/lifecycle_pretool_worktree_test.go`
- Modify: `internal/core/lifecycle/lifecycle_worktree_guard_linked_test.go`
- Modify: `internal/core/lifecycle/lifecycle_worktree_misdirect_test.go`
- Modify: `cmd/harness/hookcli/hook_pre_tool_worktree_linked_test.go`

### Step 1: Replace source-fencing assertions with the RED boundary matrix

Add `TestIssueOpsFenceNeverCapturesOrdinarySourceMutation` as a table over every existing v1 handoff state, including `coordinator_preparing`, `dispatched`, `claimed`, `submitted`, `recovery_required`, and `closed`. For each state, use source CWD/repo and assert `allow` for:

- native edit/apply-patch targeting only the source root;
- a shell write whose resolved cwd/target stays in the source root;
- an ordinary source-root Git command with no exact lifecycle ID and no worker path;
- a source-bound filesystem MCP tool;
- an Orca terminal create/switch/close/send operation whose exact literal handle is proven absent from every IssueOps record;
- exact status/start/worktree-preparation commands for a different cycle or branch;
- the same cases with `ExpectedWorktree`, a session binding, and an existing same-relative-path file in the linked worktree.

Rewrite `TestFenceScopeNarrowingUnblocksDifferentCycleFromSource` and `TestFenceScopeNarrowingSelectionDirect`: ID-less source mutation must select no supervised handoff record and must be allowed by IssueOps. Replace the tests that currently require a same-relative-path source edit or explicit `ExpectedWorktree` source edit to block/ask.

Add `TestIssueOpsFenceAmbiguousCrossRootFailsClosed`, `TestIssueOpsFenceCanonicalizesSymlinkAndGitRootOverrides`, and `TestIssueOpsFenceResourceTargetsMatchCLIAndMCP`. Table cases must pair each safe literal source operation with its worker-targeting or unresolved counterpart so an allow cannot be obtained merely by changing CWD to source. Cover direct argv, `sh -c`, one nested shell, `python -c`/`node -e`, source symlinks into the worker root, `git -C`, `--work-tree`, `--git-dir`, CLI terminal/task/dispatch handles, MCP action fields, duplicate persisted handles, and unknown literal handles.

Do not weaken the companion deny cases. Add `TestIssueOpsFenceStillProtectsIsolatedRootAndCycleControl` and assert block for:

- source session mutation targeting the canonical worker root after dispatch;
- non-owner mutation from the worker CWD;
- owner mutation outside the canonical worker root;
- an exact lifecycle command for the fenced cycle by the wrong role/state;
- terminal/task/dispatch control naming the persisted Orca resource by an unauthorized session;
- unresolved terminal control issued from the canonical worker root;
- active expansion, nested shell, unresolved redirect/path, or non-literal Orca resource control that cannot prove source-only scope;
- symlinked worker targets, `git -C`/`--work-tree`/`--git-dir` overrides, shell wrappers, and CLI/MCP resource payloads whose literal surface does not prove the canonical target;
- checking an IssueOps branch owned by a ready workspace or active handoff out in the source checkout;
- deleting, moving, or changing permissions on the worker root or its Git metadata.

Run the named tests before implementation. Expected: RED because the source-CWD fallback, explicit expected-worktree guard, mirrored-path ask/block, and source-bound MCP guard still capture source work.

### Step 2: Remove source-CWD handoff selection

Add a pre-selector scope classifier with exactly three outcomes:

- `source_only`: every observable mutation cwd/target is parser-proven inside the source root, or the command belongs to the existing bounded root-local grammar;
- `worker_or_cycle_targeted`: exact worker path, cycle ID, or persisted resource evidence exists;
- `ambiguous_cross_root`: mutating active expansion, nested shell, unresolved path/redirect, dynamic branch selection, or non-literal resource control could reach a worker root.

Allow `source_only` to bypass IssueOps selection, fail `ambiguous_cross_root` with an exact literal-source rewrite, and pass only `worker_or_cycle_targeted` into `selectSupervisedHandoffRecord`. That selector chooses a supervised record only by this ordered evidence:

1. exact persisted Orca resource handle for terminal/task/dispatch control;
2. request CWD equal to the canonical worker root or a resolved mutation target inside it;
3. exact lifecycle ID in CLI/MCP input.

Add one literal-safe CLI/MCP resource extractor for terminal, task, and dispatch handles. Make record selection consume a normalized protected-resource inventory rather than reading one concrete envelope shape; Task 1 populates it from any `ExecutionHandoff.Orca` (v1 now and v2 automatically when introduced), and Task 3 adds `ExecutionWorkspace.Orca`. Delete the current “one active record means any terminal control belongs to it” fallback, the source-CWD fallback, and its explicit-different-ID exception. A parser-proven request that matches no record returns no supervised record. Do not use session binding, branch name, same-relative-path existence, source native identity, or a singleton active record as substitute selection evidence. A terminal-control request from the canonical worker root is still selected by rule 2 and fails closed when not explicitly allowed.

### Step 3: Make the generic worktree/MCP guards root-aware

Implement one shared structural predicate for “request runs from or targets the canonical worker root.” Use it consistently:

- `ExpectedWorktree` constrains mutations only when request CWD/repo is the worker root or a target resolves inside it;
- a source-only request is never redirected to `ExpectedWorktree`;
- source-bound filesystem/Serena MCP tools are blocked only from a worker execution context, not from the source checkout;
- remove the PreToolUse `ask` path for session-bound mirrored source files; retain `SourceCheckoutMisdirectWarning` as non-blocking context;
- keep cross-root target guards unchanged;
- extend the Git topology parser beyond `checkout -b/-B` and `switch -c/--create`: exact selection of an existing IssueOps branch owned by a ready workspace or active handoff via `git checkout <branch>` or `git switch <branch>` from the source root blocks, dynamic branch selection fails with literal-branch guidance, and `git checkout -- <path>` remains a path operation.

Do not add an “unrelated work” parser. Worker task scope remains the sealed cycle, branch, root, fence tuple, native owner, committed plan, and `worker_scope`; changed-file/readiness/completion checks catch owner drift.

### Step 4: Run the GREEN fence matrix

Run:

~~~bash
go test ./internal/core/lifecycle ./cmd/harness/hookcli \
  -run 'TestIssueOpsFence|TestFenceScopeNarrowing|TestPreToolUseWorktreeGuard|TestWorktreeGuard.*Source|TestRunHookPreToolUse.*Source|TestSourceCheckoutMisdirectWarning' \
  -count=1
~~~

Expected: PASS. Also rerun the existing worker-root, branch-topology, protected-root, terminal-control, and exact lifecycle allowlist tests to prove the deny set remains intact.

### Step 5: Commit the protocol-independent fence correction

~~~bash
git add internal/core/lifecycle/lifecycle_handoff_authority.go \
  internal/core/lifecycle/lifecycle_state.go \
  internal/core/lifecycle/lifecycle_handoff_scope.go \
  internal/core/lifecycle/lifecycle_handoff_scope_test.go \
  internal/core/lifecycle/lifecycle_handoff_resource_target.go \
  internal/core/lifecycle/lifecycle_handoff_resource_target_test.go \
  internal/core/lifecycle/lifecycle_worktree_guard.go \
  internal/core/lifecycle/lifecycle_worktree_mcp.go \
  internal/core/lifecycle/dependencies.go \
  internal/core/lifecycle/dependencies_test.go \
  internal/core/lifecycle/worktreepath/shell_paths.go \
  internal/core/lifecycle/worktreepath/shell_paths_test.go \
  internal/core/lifecycle/worktreeguard/branch_creation.go \
  internal/core/lifecycle/worktreeguard/branch_creation_test.go \
  internal/core/lifecycle/lifecycle_handoff_fence_scope_test.go \
  internal/core/lifecycle/lifecycle_pretool_worktree_test.go \
  internal/core/lifecycle/lifecycle_worktree_guard_linked_test.go \
  internal/core/lifecycle/lifecycle_worktree_misdirect_test.go \
  cmd/harness/hookcli/hook_pre_tool_worktree_linked_test.go
git commit -m "fix(issueops): scope fences to isolated worktrees" \
  -m "Lore:" \
  -m "Intent: Keep the source main worktree available while a parallel IssueOps worktree is owned." \
  -m "Why: A worker-root lease must not become a repository-wide source-checkout lease." \
  -m "Changes: Select fences by worker root, exact cycle, or persisted Orca resource and downgrade mirror collisions to warnings." \
  -m "Verify: Run the source-availability and isolated-root deny matrices across lifecycle and native hook packages." \
  -m "Risk: Source edits that were previously blocked or asked now rely on ordinary Git conflict handling and non-blocking orientation warnings."
~~~

## Task 2: Add schema-v8 workspace and protocol-v2 model vocabulary

**Files:**

- Modify: `internal/core/issueops/model/types.go`
- Modify: `internal/core/issueops/package.go`
- Modify: `internal/core/issueops/issueops_state.go`
- Modify: `internal/core/issueops/handoff/state.go`
- Modify: `internal/core/issueops/handoff/envelope.go`
- Modify: `internal/core/issueops/handoff/state_test.go`
- Modify: `internal/core/issueops/issueops_schema_version_test.go`
- Create: `internal/core/issueops/handoff/ownership_state.go`
- Create: `internal/core/issueops/handoff/ownership_envelope.go`
- Modify: `cmd/harness/testdata/response_contracts.golden.json`

### Step 1: Write schema and envelope RED tests

Add:

- `TestSchemaV8RoundTripsReadyExecutionWorkspaceWithoutHandoff`
- `TestSchemaV8PreservesProtocolV1Semantics`
- `TestOwnershipEnvelopeRejectsV1FieldsInProtocolV2`
- `TestOwnershipEnvelopeStateMatrix`

The state-matrix test must cover:

~~~text
ownership_dispatching
ownership_dispatched
owner_orienting
owner_active
cleanup_pending_human_decision
cleanup_executing
closed
recovery_required
~~~

For every state, assert the exact required and forbidden fields. Independently assert that Orca workspace `provisioning`, `ready`, and `recovery_required` require an authenticated `PreparationSession`, while legacy/inline workspace projections cannot smuggle one. Include future root schema, unsupported handoff protocol, mixed v1/v2 owner fields, and missing workspace fingerprint as fail-safe cases.

Run:

~~~bash
go test ./internal/core/issueops/handoff ./internal/core/issueops \
  -run 'TestSchemaV8|TestOwnershipEnvelope' \
  -count=1
~~~

Expected: RED because schema 8, workspace types, and protocol-v2 validators do not exist.

### Step 2: Add the model without changing behavior

In `model/types.go`:

- set `IssueOpsCurrentSchemaVersion = 8`;
- add `ExecutionWorkspace *IssueOpsExecutionWorkspace` to `IssueOpsRecord`;
- add workspace pending-operation types plus authenticated `PreparationSession` (host/session/agent);
- add `OwnerSession`, `Orientation`, `Completion`, and protocol-v2 cleanup fields additively to `IssueOpsExecutionHandoff`;
- keep `WorkerSession`, `Result`, `AcceptedAt`, and current cleanup fields for protocol v1;
- add protocol-v2 closed dispositions `owner_closed_workspace_retained` and `local_resources_removed`.

In `handoff/state.go`:

- retain `ProtocolVersion = 1` as the legacy compatibility alias;
- add `OwnershipTransferProtocolVersion = 2`;
- retain every v1 constant and transition unchanged.

Put protocol-v2 constructors/transitions in `ownership_state.go`. Do not add protocol branches to v1 transition bodies.

### Step 3: Split envelope validation by protocol

`ValidateEnvelope` must:

1. validate root `ExecutionWorkspace` independently;
2. route protocol 1 to the existing validator byte-for-byte;
3. route protocol 2 to `validateOwnershipEnvelope`;
4. reject all other protocol values;
5. reject workspace/handoff identity drift.

The v2 validator must require `WorkspaceEpoch` and `WorkspaceSHA256` and reject v1-only `submitted`, `accepted`, `WorkerSession`, `Result`, and `AcceptedAt` authority.

### Step 4: Implement root schema read/write compatibility

Update `issueops_state.go` so:

- missing/zero through schema 7 remain readable;
- the next write upgrades only the root schema number;
- protocol-v1 nested meaning is untouched;
- schema greater than 8 fails safe;
- schema 7 records cannot smuggle schema-8 workspace or owner authority.

### Step 5: Run the GREEN model slice

~~~bash
go test ./internal/core/issueops/handoff ./internal/core/issueops \
  -run 'TestSchemaV8|TestOwnershipEnvelope|TestIssueOpsSchema' \
  -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
~~~

Expected: PASS. If the schema projection changes the normalized response contract, update and review `response_contracts.golden.json` in this task before rerunning; no later task owns this delta.

### Step 6: Commit

~~~bash
git add internal/core/issueops/model/types.go \
  internal/core/issueops/package.go \
  internal/core/issueops/issueops_state.go \
  internal/core/issueops/handoff/state.go \
  internal/core/issueops/handoff/envelope.go \
  internal/core/issueops/handoff/ownership_state.go \
  internal/core/issueops/handoff/ownership_envelope.go \
  internal/core/issueops/handoff/state_test.go \
  internal/core/issueops/issueops_schema_version_test.go \
  cmd/harness/testdata/response_contracts.golden.json
git commit -m "feat(issueops): add ownership transfer state model" \
  -m "Lore:" \
  -m "Intent: Represent workspace provisioning and full ownership transfer as separate durable concepts." \
  -m "Why: A physical worktree must not activate a logical handoff fence." \
  -m "Changes: Add schema-v8 workspace state and protocol-v2 owner, completion, and cleanup vocabulary while preserving v1." \
  -m "Verify: Run schema/envelope state matrices and both response-contract golden suites." \
  -m "Risk: Root schema 8 intentionally fails safe on older binaries."
~~~

## Task 3: Move Orca worktree provisioning out of `ExecutionHandoff`

**Files:**

- Create: `internal/core/issueops/issueops_execution_workspace.go`
- Create: `internal/core/issueops/issueops_execution_workspace_recovery.go`
- Create: `internal/core/issueops/issueops_actor.go`
- Create: `internal/core/issueops/issueops_actor_test.go`
- Modify: `internal/core/issueops/issueops_handoff_prepare.go`
- Modify: `internal/core/issueops/issueops_handoff_prepare_test.go`
- Modify: `internal/core/issueops/issueops_handoff_recovery.go`
- Modify: `internal/core/issueops/issueops_handoff_plan.go`
- Create: `internal/core/issueops/issueops_handoff_plan_test.go`
- Modify: `internal/core/issueops/start/start.go`
- Create: `internal/core/issueops/start/start_workspace_authority_test.go`
- Modify: `internal/core/issueops/package.go`
- Modify: `internal/core/issueops/issueops_phase.go`
- Modify: `internal/core/issueops/issueops_decision.go`
- Modify: `internal/core/issueops/issueops_ledger_recorders.go`
- Modify: `internal/core/issueops/issueops_routing.go`
- Modify: `internal/core/issueops/issueops_regress.go`
- Modify: `internal/core/issueops/issueops_delegation.go`
- Create: `internal/core/lifecycle/lifecycle_workspace_guard.go`
- Create: `internal/core/lifecycle/lifecycle_workspace_guard_test.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_resource_target.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_resource_target_test.go`
- Modify: `cmd/harness/issueopscli/worktreecmd/worktree.go`
- Create: `cmd/harness/issueopscli/issueops_actor_flags.go`
- Create: `cmd/harness/issueopscli/issueops_actor_flags_test.go`
- Modify: `cmd/harness/issueopscli/issueops_subcommands.go`
- Modify: `cmd/harness/issueopscli/issueops_intent_design.go`
- Create: `cmd/harness/issueopscli/issueops_intent_design_actor_test.go`
- Modify: `cmd/harness/issueopscli/issueops_plan_prep.go`
- Modify: `cmd/harness/issueopscli/issueops_plan_prep_test.go`
- Modify: `cmd/harness/issueopscli/issueops_decision_cli.go`
- Modify: `cmd/harness/issueopscli/issueops_ledger_cli.go`
- Modify: `cmd/harness/issueopscli/issueops_ledger_cli_test.go`
- Modify: `cmd/harness/issueopscli/issueops_compatibility_cli.go`
- Modify: `cmd/harness/issueopscli/issueops_devilsadvocate_cli.go`
- Modify: `cmd/harness/issueopscli/issueops_execution_cli.go`
- Modify: `cmd/harness/issueopscli/issueops_test.go`
- Modify: `cmd/harness/issueopscli/remotecmd/remote.go`
- Modify: `cmd/harness/issueopscli/remotecmd/remote_test.go`
- Modify: `internal/adapter/mcp/issueops_catalog.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops_handlers.go`
- Modify: `cmd/harness/mcpcli/mcp_issueops_delegation_test.go`
- Modify: `cmd/harness/hookcli/hook_pre_tool_handoff_test.go`
- Modify: `cmd/harness/testdata/mcp_tools.golden.json`
- Modify: `cmd/harness/testdata/usage.golden.txt`
- Modify: `cmd/harness/testdata/response_contracts.golden.json`

### Step 1: Add workspace exactly-once and recovery RED tests

Add:

- `TestExecutionWorkspaceJournalsBeforeOrcaCreate`
- `TestExecutionWorkspaceTimeoutRequiresExplicitReconcile`
- `TestExecutionWorkspaceReconcileAdoptsExactlyOneCandidate`
- `TestExecutionWorkspaceReadyContainsNoOwnerIdentity`
- `TestOrcaWorktreePrepareKeepsPreparationUnfenced`
- `TestExecutionWorkspaceSealsExactPreparationSession`
- `TestExecutionWorkspaceRejectsSecondPreparerAndMissingActor`
- `TestExecutionWorkspaceEveryPreparationMutatorRequiresExactActor`
- `TestStartNeverStaleResetsExecutionWorkspaceAuthority`

For the unfenced-preparation test, assert that confirmed preparation links `WorktreePath`, persists `ExecutionWorkspace.state=ready`, leaves `ExecutionHandoff` nil, and keeps plan/tool/review setup available to the exact sealed preparation session. Assert that workspace Orca identity has no worker terminal, mailbox, task, or dispatch fields. A second source session remains free in the source root but fails when it targets the isolated root or preparation recorders.

Add this three-state authority table to the direct-core, CLI, MCP, and hook tests:

| Workspace state | `PreparationSession` exact-cycle/isolated-root authority | Other source sessions |
|---|---|---|
| `provisioning` | status and exact journal reconciliation only | ordinary source-root work only |
| `ready` | all enumerated preparation recorders and isolated-root setup | ordinary source-root work only |
| `recovery_required` | status, exact reconciliation, or human-approved rebind preview/confirm only | ordinary source-root work only |

Every setup recorder must reject `provisioning` and `recovery_required` before mutation. The external create/adopt callback is the only writer that can turn its exact pending journal into `ready`.

Run:

~~~bash
go test ./internal/core/issueops \
  -run 'TestExecutionWorkspace|TestOrcaWorktreePrepareKeepsPreparationUnfenced' \
  -count=1
~~~

Expected: RED because confirmed prepare still calls `handoff.Prepare`.

### Step 2: Implement workspace preparation

Refactor `PrepareIssueOpsHandoffWorktree`:

- preview stays byte-stable and non-mutating;
- confirmed Orca mode requires the existing approved-design and verified issue/branch/base prerequisites plus exact native host/session/agent and source cwd, then creates `ExecutionWorkspace{state: provisioning, PreparationSession: actor}` and its pending `worktree_create` journal under the cycle lock;
- perform Orca create/adopt outside the lock;
- exact-compare record, branch, path, base ref, head, marker, and inventory before persisting `ready`;
- link `record.WorktreePath` from the verified workspace;
- never create `ExecutionHandoff`.

Keep inline mode behavior unchanged.

If the first confirmed command omits native actor flags, the hook blocks once and emits one exact replacement from the authenticated event. The second command is directly executable. Core rejects empty or guessed identity even if hooks are bypassed.

### Step 3: Enforce the isolated-root preparation lease in hook and core

Treat repo, linked issue, provider branch/base, coordinator root, worker root, and workspace epoch as immutable physical identity once `ExecutionWorkspace` exists. Their core mutators reject with explicit workspace-recovery guidance even for `PreparationSession`.

Add shared `IssueOpsActor{Host, SessionID, AgentID, CWD}` parsing/validation and actor-aware core variants for **every** remaining exact-cycle pre-dispatch mutator: intent, domain review, durable decisions, plan-prep, design revision, routing, related/child topology and delegation repair, plan link, phase/replan, worktree tools, compatibility review, execution decision, and Brooks review. Existing actorless entry points remain compatible only when no schema-v8 workspace/handoff requires an actor; they return a precise missing-actor error otherwise. Add a table test that invokes every mutator directly with exact, missing, and different actors; the latter two must leave the record byte-identical.

Each actor-aware variant must acquire the cycle lock, re-read `workspace.state=ready`, the current workspace epoch, and `PreparationSession`, validate the actor/root/state, and perform the evidence mutation in that same critical section. Do not validate in an outer read and then call an actorless recorder, and do not let the actorless compatibility wrapper bypass the in-lock validator.

`lifecycle_workspace_guard.go` selects only requests whose exact cycle ID or resolved target is the provisioning/ready/recovery workspace. It permits isolated-root setup only in `ready` for `PreparationSession + source root + workspace epoch`; `provisioning` and `recovery_required` permit only their exact recovery rows. It denies a second or missing actor and never handles ordinary source-only requests. CLI and MCP expose the same actor fields. A native request missing them receives one exact hook-authored CLI/payload retry; core revalidates every write.

Extend Task 1's normalized protected-resource inventory with every handle persisted in `ExecutionWorkspace.Orca`; duplicate workspace/handoff handles are ambiguous and fail closed. This closes terminal/task control during ready-workspace preparation without introducing a source-CWD fallback.

Run RED then GREEN tests for exact-preparer plan edits and plan-only Git, CLI/MCP preparation recorders, direct calls with missing/different actor, a second source session's source-only versus worker-targeted work, and dynamic/cross-root requests.

Update generic `StartIssueOps` stale-reset eligibility to use a shared “durable workspace or handoff authority exists” predicate. A missing/invalid worktree with any `ExecutionWorkspace` state must return exact workspace-recovery guidance and preserve the record byte-for-byte; it may never pass through the legacy stale-reset path merely because `ExecutionHandoff` is nil.

### Step 4: Split workspace recovery

Move worktree-create reconciliation out of handoff recovery into the new workspace recovery core. Keep terminal/task/dispatch recovery in handoff recovery. A workspace reconciliation may return only `ready` or `recovery_required`; it may not create or dispatch an owner. It requires the sealed preparation session, or a separate explicit human-approved rebind preview/confirm after proving no handoff or external owner resource exists; rebind never runs automatically.

### Step 5: Update worktree command output

The CLI/MCP result must expose:

- `workspace_state`;
- `workspace_epoch`;
- redacted preparation-session identity/fingerprint;
- `handoff_state` only when a handoff actually exists;
- next preparation gates.

Keep the existing `worktree_path`, `branch`, and mode fields.

### Step 6: Run focused GREEN tests

~~~bash
go test ./internal/core/issueops ./internal/core/lifecycle ./cmd/harness/issueopscli ./cmd/harness/issueopscli/remotecmd ./cmd/harness/mcpcli ./cmd/harness/mcpcli/issueops ./cmd/harness/hookcli \
  -run 'TestExecutionWorkspace|TestOrcaWorktreePrepareKeepsPreparationUnfenced|TestPreparationActor|TestWorkspaceGuard|TestWorktreePrepare' \
  -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
~~~

Expected: PASS. Regenerate and review only the workspace/actor/tool/usage response deltas owned by this task before committing.

### Step 7: Commit

~~~bash
git add internal/core/issueops/issueops_execution_workspace.go \
  internal/core/issueops/issueops_execution_workspace_recovery.go \
  internal/core/issueops/issueops_actor.go \
  internal/core/issueops/issueops_actor_test.go \
  internal/core/issueops/issueops_handoff_prepare.go \
  internal/core/issueops/issueops_handoff_prepare_test.go \
  internal/core/issueops/issueops_handoff_recovery.go \
  internal/core/issueops/issueops_handoff_plan.go \
  internal/core/issueops/issueops_handoff_plan_test.go \
  internal/core/issueops/start/start.go \
  internal/core/issueops/start/start_workspace_authority_test.go \
  internal/core/issueops/package.go \
  internal/core/issueops/issueops_phase.go \
  internal/core/issueops/issueops_decision.go \
  internal/core/issueops/issueops_ledger_recorders.go \
  internal/core/issueops/issueops_routing.go \
  internal/core/issueops/issueops_regress.go \
  internal/core/issueops/issueops_delegation.go \
  internal/core/lifecycle/lifecycle_workspace_guard.go \
  internal/core/lifecycle/lifecycle_workspace_guard_test.go \
  internal/core/lifecycle/lifecycle_handoff_resource_target.go \
  internal/core/lifecycle/lifecycle_handoff_resource_target_test.go \
  cmd/harness/issueopscli/worktreecmd/worktree.go \
  cmd/harness/issueopscli/issueops_actor_flags.go \
  cmd/harness/issueopscli/issueops_actor_flags_test.go \
  cmd/harness/issueopscli/issueops_subcommands.go \
  cmd/harness/issueopscli/issueops_intent_design.go \
  cmd/harness/issueopscli/issueops_intent_design_actor_test.go \
  cmd/harness/issueopscli/issueops_plan_prep.go \
  cmd/harness/issueopscli/issueops_plan_prep_test.go \
  cmd/harness/issueopscli/issueops_decision_cli.go \
  cmd/harness/issueopscli/issueops_ledger_cli.go \
  cmd/harness/issueopscli/issueops_ledger_cli_test.go \
  cmd/harness/issueopscli/issueops_compatibility_cli.go \
  cmd/harness/issueopscli/issueops_devilsadvocate_cli.go \
  cmd/harness/issueopscli/issueops_execution_cli.go \
  cmd/harness/issueopscli/issueops_test.go \
  cmd/harness/issueopscli/remotecmd/remote.go \
  cmd/harness/issueopscli/remotecmd/remote_test.go \
  internal/adapter/mcp/issueops_catalog.go \
  cmd/harness/mcpcli/mcp_tool_issueops.go \
  cmd/harness/mcpcli/mcp_tool_issueops_handlers.go \
  cmd/harness/mcpcli/mcp_issueops_delegation_test.go \
  cmd/harness/hookcli/hook_pre_tool_handoff_test.go \
  cmd/harness/testdata/mcp_tools.golden.json \
  cmd/harness/testdata/usage.golden.txt \
  cmd/harness/testdata/response_contracts.golden.json
git commit -m "feat(issueops): separate Orca workspace provisioning" \
  -m "Lore:" \
  -m "Intent: Let the source session prepare a linked worktree without creating an ownership lease." \
  -m "Why: Early handoff state blocks the plan and review gates required for dispatch." \
  -m "Changes: Add actor-sealed workspace journaling, exact reconciliation, preparation guards, and unfenced ready-workspace results across CLI/MCP." \
  -m "Verify: Run workspace, actor, lifecycle, adapter, and contract-golden tests." \
  -m "Risk: Recovery ownership moves from the v1 handoff envelope to a new schema-v8 field."
~~~

## Task 4: Make protocol-v2 start preview read-only and confirmed dispatch atomic

**Files:**

- Modify: `internal/core/issueops/issueops_handoff_dispatch.go`
- Modify: `internal/core/issueops/issueops_handoff_dispatch_test.go`
- Modify: `internal/core/issueops/handoff/context.go`
- Modify: `internal/core/issueops/handoff/context_test.go`
- Modify: `cmd/harness/issueopscli/issueops_handoff_cli.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_authority.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_guard.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_coordinator_dispatch_test.go`
- Modify: `cmd/harness/hookcli/hook_pre_tool_handoff_test.go`
- Modify: `internal/adapter/mcp/issueops_lifecycle_catalog.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops_handlers.go`
- Modify: `cmd/harness/mcpcli/issueops/handoff_test.go`
- Modify: `cmd/harness/testdata/mcp_tools.golden.json`
- Modify: `cmd/harness/testdata/usage.golden.txt`
- Modify: `cmd/harness/testdata/response_contracts.golden.json`

### Step 1: Add start-boundary RED tests

Add:

- `TestOwnershipStartPreviewPersistsNothing`
- `TestOwnershipStartRejectsIncompletePreparation`
- `TestOwnershipStartRequiresPlanOnlyCommittedHead`
- `TestOwnershipStartSealsWorkspaceAndHeadBeforeDispatch`
- `TestOwnershipStartCreatesOneFreshConfiguredOwnerSession`
- `TestOwnershipStartAmbiguityNeverRetries`
- `TestOwnershipStartRequiresExactPreparationSessionAndWorkspaceEpoch`
- `TestOwnershipTransferPreparationAllowsCLIAndMCPBeforeDispatch`
- `TestOwnershipTransferBootstrapRendersRunnableNativeStart`

The plan-only test must reject an untracked plan, dirty worktree, plan absent from HEAD, and any first-attempt `base_sha..HEAD` path other than the linked plan.

The preparation-authority test models a ready isolated workspace with no handoff and asserts that source CLI/MCP plan link, phase, tool preparation, compatibility, execution-decision, and Brooks-review requests remain allowed. The bootstrap test feeds the minimal native command `handoff start --id <id> --source-cwd <source> --json`, verifies one hook-authored replacement with exact host/session/agent identity and no required guessed recipient, then feeds that replacement back and expects allow.

Run:

~~~bash
go test ./internal/core/issueops \
  -run 'TestOwnershipStart' \
  -count=1
~~~

Expected: RED because start still expects a pre-existing v1 handoff.

### Step 2: Build the v2 preview

When `ExecutionWorkspace.state=ready` and no handoff exists:

- require the authenticated source actor to equal `ExecutionWorkspace.PreparationSession`, its canonical CWD to equal `CoordinatorRoot`, and its expected workspace epoch to match;
- require phase `compatibility-review`;
- call pre-dispatch readiness without `handoff_worker_claim`;
- verify the exact clean worktree/branch/head and plan-only diff;
- resolve exactly one source Orca terminal when no recipient is supplied;
- render context with workspace fingerprint and current HEAD;
- return context SHA and exact confirm command;
- write nothing.

### Step 3: Build confirmed dispatch

On `--confirm --expected-context-sha256`:

1. re-render and exact-compare preview inputs, authenticated preparation actor, and workspace epoch;
2. create protocol-v2 `ownership_dispatching` with owner epoch, source session, source mailbox, workspace fingerprint, context, and first pending operation under the lock;
3. outside the lock, create exactly one fresh owner terminal in the canonical Orca worktree using the workspace-sealed `codex` or `claude` agent, create/update exactly one task, and dispatch the sealed context to that terminal; journal and read back each external identity before advancing;
4. persist `ownership_dispatched` only after exact readback;
5. route ambiguity to `recovery_required` without retry.

Do not create `submitted` or coordinator-accept state.

### Step 4: Fix source bootstrap reachability

The bootstrap recognizer must accept the minimal `id + source-cwd + optional json` probe when a ready workspace exists and no handoff exists. Render native identity flags from the authenticated hook event. Do not require the caller to guess a terminal handle.

Retain the current v1 bootstrap path for active v1 records.

### Step 5: Run GREEN slices

~~~bash
go test ./internal/core/issueops \
  -run 'TestOwnershipStart|TestHandoffStart' \
  -count=1
go test ./internal/core/lifecycle ./cmd/harness/hookcli \
  -run 'TestOwnershipTransferPreparationAllowsCLIAndMCPBeforeDispatch|TestOwnershipTransferBootstrapRendersRunnableNativeStart|TestCoordinatorDispatch' \
  -count=1
go test ./internal/adapter/mcp ./cmd/harness/mcpcli/issueops \
  -run 'Test.*OwnershipStart|Test.*Handoff' -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
~~~

Expected: PASS after reviewing only the start preview/confirm action, usage, and response golden deltas owned by this task.

### Step 6: Commit

~~~bash
git add internal/core/issueops/issueops_handoff_dispatch.go \
  internal/core/issueops/issueops_handoff_dispatch_test.go \
  internal/core/issueops/handoff/context.go \
  internal/core/issueops/handoff/context_test.go \
  cmd/harness/issueopscli/issueops_handoff_cli.go \
  internal/core/lifecycle/lifecycle_handoff_authority.go \
  internal/core/lifecycle/lifecycle_handoff_guard.go \
  internal/core/lifecycle/lifecycle_handoff_coordinator_dispatch_test.go \
  cmd/harness/hookcli/hook_pre_tool_handoff_test.go \
  internal/adapter/mcp/issueops_lifecycle_catalog.go \
  cmd/harness/mcpcli/mcp_tool_issueops_handlers.go \
  cmd/harness/mcpcli/issueops/handoff_test.go \
  cmd/harness/testdata/mcp_tools.golden.json \
  cmd/harness/testdata/usage.golden.txt \
  cmd/harness/testdata/response_contracts.golden.json
git commit -m "feat(issueops): dispatch ownership only after preparation" \
  -m "Lore:" \
  -m "Intent: Make confirmed dispatch the sole normal ownership-transfer boundary." \
  -m "Why: Planning must finish before isolated-worktree ownership transfers." \
  -m "Changes: Add read-only preview, plan-only head sealing, native bootstrap, and protocol-v2 dispatch journaling." \
  -m "Verify: Run ownership/legacy start, context, hook bootstrap, MCP, and contract-golden tests." \
  -m "Risk: Start now has protocol-specific behavior selected from durable workspace and handoff state."
~~~

## Task 5: Add owner claim, orientation acknowledgement, and role authority

**Files:**

- Create: `internal/core/issueops/issueops_handoff_orientation.go`
- Create: `internal/core/issueops/issueops_handoff_orientation_test.go`
- Modify: `internal/core/issueops/issueops_handoff_lifecycle.go`
- Modify: `internal/core/issueops/issueops_handoff_lifecycle_test.go`
- Modify: `internal/core/issueops/handoff/authority_table.go`
- Modify: `internal/core/issueops/handoff/authority_table_test.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_guard.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_authority.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_guard_test.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_fence_scope_test.go`
- Modify: `cmd/harness/issueopscli/issueops_handoff_cli.go`
- Modify: `cmd/harness/hookcli/hook_pre_tool_handoff_test.go`
- Modify: `internal/adapter/mcp/issueops_lifecycle_catalog.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops_handlers.go`
- Modify: `cmd/harness/mcpcli/issueops/handoff_test.go`
- Modify: `cmd/harness/testdata/mcp_tools.golden.json`
- Modify: `cmd/harness/testdata/usage.golden.txt`
- Modify: `cmd/harness/testdata/response_contracts.golden.json`

### Step 1: Write the role-transition RED matrix

Add:

- `TestOwnershipClaimEntersOrientationWithoutWriteLease`
- `TestOwnershipOrientationRequiresExactIssuePlanAndContext`
- `TestOwnershipAcknowledgementGrantsOwnerAndRevokesSource`
- `TestOwnershipRoleAuthorityMatrix`
- `TestOwnershipFenceNeverCapturesOrdinarySourceMutation`
- `TestOwnershipFenceStillProtectsWorkerRootAndCycleControl`

The authority matrix must test every state and both native sessions. Explicitly assert:

- ordinary source-root edit, shell write, Git, and source-bound MCP requests remain allowed in every protocol-v2 state, with and without session binding/`ExpectedWorktree`;
- source mutation targeting the canonical worker root and exact-cycle phase/publish/terminal steering are blocked after dispatch starts;
- unclaimed owner mutation is blocked;
- orienting owner may read, heartbeat, and acknowledge only;
- active owner may edit the canonical worker root;
- a different native session or cycle cannot mutate that root;
- active-owner cross-root, branch-mismatch, worker-root metadata mutation, and unscoped remote mutation remain blocked;
- protocol-v1 worker/coordinator lifecycle rows are unchanged, while Task 1 source-root availability remains in force.

Run:

~~~bash
go test ./internal/core/issueops ./internal/core/issueops/handoff ./internal/core/lifecycle \
  -run 'TestOwnershipClaim|TestOwnershipOrientation|TestOwnershipAcknowledgement|TestOwnershipRoleAuthorityMatrix|TestOwnershipFence' \
  -count=1
~~~

Expected: RED.

### Step 2: Implement protocol-v2 claim and orientation

Protocol-v2 claim validates the existing fence, Orca worktree, current branch, native owner session, and dispatch assignee, then enters `owner_orienting`.

Add `handoff acknowledge-context` with:

- fence triple;
- native owner identity and canonical cwd;
- exact issue URL;
- exact plan SHA-256;
- bounded understanding summary;
- bounded scope confirmation.

On exact match, persist the acknowledgement and enter `owner_active`. A repeat with identical content is idempotent; conflicting content fails.

### Step 3: Replace the state-only authority table with protocol/state/role rows

Keep native identity, cwd, target-root, and fence checks in the lifecycle layer. The shared table answers only exact-cycle/isolated-root authority, never ordinary source-root activity. It must answer the state dimension for:

- `legacy_coordinator`;
- `legacy_worker`;
- `source_owner_transfer`;
- `transferred_owner`.

Unknown protocol, role, state, or command defaults to deny.

The claimed owner's “own task” is the sealed cycle, branch, canonical worker root, fence tuple, native identity, committed plan, and `worker_scope`. Do not add a natural-language intent classifier or infer scope from source-path collisions. Enforce non-owner/cross-cycle/root escape at hook time and verify the owner's final committed changed-file set against the attempt diff, plan evidence, readiness gates, and completion evidence.

### Step 4: Improve narrow diagnostics

- For a protocol-v1 plan Git mismatch, return the exact absolute-path command instead of dispatch guidance.
- For MCP resume `bind=true`, say “omit bind or set bind=false”.
- For a preparation MCP tool before v2 transfer, do not route through a handoff block.

### Step 5: Run GREEN tests

~~~bash
go test ./internal/core/issueops ./internal/core/issueops/handoff ./internal/core/lifecycle ./cmd/harness/hookcli \
  -run 'TestOwnershipClaim|TestOwnershipOrientation|TestOwnershipAcknowledgement|TestOwnershipRoleAuthorityMatrix|TestOwnershipFence|TestOwnershipTransferPreparationAllowsCLIAndMCPBeforeDispatch' \
  -count=1
go test ./internal/adapter/mcp ./cmd/harness/mcpcli/issueops \
  -run 'Test.*Ownership.*Claim|Test.*Acknowledge|Test.*Handoff' -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
~~~

Expected: PASS after reviewing only claim/acknowledgement actor and response golden deltas owned by this task.

### Step 6: Commit

~~~bash
git add internal/core/issueops/issueops_handoff_orientation.go \
  internal/core/issueops/issueops_handoff_orientation_test.go \
  internal/core/issueops/issueops_handoff_lifecycle.go \
  internal/core/issueops/issueops_handoff_lifecycle_test.go \
  internal/core/issueops/handoff/authority_table.go \
  internal/core/issueops/handoff/authority_table_test.go \
  internal/core/lifecycle/lifecycle_handoff_guard.go \
  internal/core/lifecycle/lifecycle_handoff_authority.go \
  internal/core/lifecycle/lifecycle_handoff_guard_test.go \
  internal/core/lifecycle/lifecycle_handoff_fence_scope_test.go \
  cmd/harness/issueopscli/issueops_handoff_cli.go \
  cmd/harness/hookcli/hook_pre_tool_handoff_test.go \
  internal/adapter/mcp/issueops_lifecycle_catalog.go \
  cmd/harness/mcpcli/mcp_tool_issueops_handlers.go \
  cmd/harness/mcpcli/issueops/handoff_test.go \
  cmd/harness/testdata/mcp_tools.golden.json \
  cmd/harness/testdata/usage.golden.txt \
  cmd/harness/testdata/response_contracts.golden.json
git commit -m "feat(issueops): transfer lifecycle authority to owner" \
  -m "Lore:" \
  -m "Intent: Make the fresh claimed session the sole post-handoff workflow owner." \
  -m "Why: Full handoff requires source disengagement and an explicit issue-plan orientation gate." \
  -m "Changes: Add owner orientation and a protocol-aware, worker-root-scoped role authority matrix with precise diagnostics." \
  -m "Verify: Run claim, orientation, role matrix, CLI/MCP parity, and contract-golden tests." \
  -m "Risk: Exact-cycle operations remain default-deny, while ordinary source work is intentionally outside the IssueOps authority table."
~~~

## Task 6: Require owner identity for post-transfer phases and evidence

**Files:**

- Modify: `internal/core/issueops/model/types.go`
- Modify: `internal/core/issueops/issueops_phase.go`
- Modify: `internal/core/issueops/issueops_phase_lifecycle_test.go`
- Modify: `internal/core/issueops/issueops_ledger_recorders.go`
- Modify: `internal/core/issueops/issueops_feedback.go`
- Modify: `cmd/harness/issueopscli/issueops_subcommands.go`
- Modify: `cmd/harness/issueopscli/issueops_ledger_cli.go`
- Modify: `cmd/harness/issueopscli/feedbackcleanup/feedback_cleanup.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops_handlers.go`
- Modify: `internal/adapter/mcp/issueops_catalog.go`
- Modify: `internal/adapter/mcp/issueops_catalog_test.go`
- Modify: `cmd/harness/testdata/mcp_tools.golden.json`
- Modify: `cmd/harness/testdata/usage.golden.txt`
- Modify: `cmd/harness/testdata/response_contracts.golden.json`

### Step 1: Add actor-enforcement RED tests

Add `TestProtocolV2OwnerRequiredForPostTransferRecorders`. For `phase`, ai-slop-clean evidence, feedback add/resolve, and issue-update marking, assert:

- the exact active owner succeeds from the worker root;
- the source session's exact post-transfer recorder request fails, without affecting ordinary source-root work;
- a different worker session fails;
- omitted identity fails;
- inline and protocol-v1 behavior remains unchanged.

Run:

~~~bash
go test ./internal/core/issueops \
  -run 'TestProtocolV2OwnerRequiredForPostTransferRecorders' \
  -count=1
~~~

Expected: RED because these recorders do not accept an actor.

### Step 2: Add a shared actor request

Reuse the `IssueOpsActor{Host, SessionID, AgentID, CWD}` introduced in Task 3 and add one shared post-transfer validator:

- no active v2 handoff: preserve existing behavior;
- active v2 before `owner_active`: reject post-transfer recorder mutation;
- `owner_active`: require exact `OwnerSession` and canonical worker root;
- cleanup states: reject owner workflow mutation.

Keep old exported helpers as compatibility wrappers only where no active protocol-v2 handoff exists. CLI/MCP must call the actor-aware variants.

### Step 3: Wire native actor flags

Add `--host`, `--session-id`, optional `--agent-id`, and `--cwd` to the relevant CLI commands and the equivalent MCP schema. The PreToolUse hook exact-compares them with the native event; core exact-compares them with `OwnerSession`.

### Step 4: Run GREEN and regression tests

~~~bash
go test ./internal/core/issueops ./cmd/harness/issueopscli ./cmd/harness/mcpcli/issueops \
  -run 'TestProtocolV2OwnerRequiredForPostTransferRecorders|TestIssueOpsPhase|Test.*AISlop|Test.*Feedback' \
  -count=1
go test ./internal/adapter/mcp -run 'Test.*IssueOps.*Catalog' -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
~~~

Expected: PASS after reviewing only post-transfer actor fields and response golden deltas owned by this task.

### Step 5: Commit

~~~bash
git add internal/core/issueops/model/types.go \
  internal/core/issueops/issueops_phase.go \
  internal/core/issueops/issueops_phase_lifecycle_test.go \
  internal/core/issueops/issueops_ledger_recorders.go \
  internal/core/issueops/issueops_feedback.go \
  cmd/harness/issueopscli/issueops_subcommands.go \
  cmd/harness/issueopscli/issueops_ledger_cli.go \
  cmd/harness/issueopscli/feedbackcleanup/feedback_cleanup.go \
  cmd/harness/mcpcli/mcp_tool_issueops_handlers.go \
  internal/adapter/mcp/issueops_catalog.go \
  internal/adapter/mcp/issueops_catalog_test.go \
  cmd/harness/testdata/mcp_tools.golden.json \
  cmd/harness/testdata/usage.golden.txt \
  cmd/harness/testdata/response_contracts.golden.json
git commit -m "feat(issueops): bind execution phases to transferred owner" \
  -m "Lore:" \
  -m "Intent: Enforce owner identity below the hook for post-transfer lifecycle writes." \
  -m "Why: Source disengagement must survive direct CLI and MCP use." \
  -m "Changes: Add actor-aware phase, cleanup-evidence, and feedback recorders." \
  -m "Verify: Run owner recorder, phase, ai-slop, feedback, adapter, and contract-golden tests." \
  -m "Risk: Active protocol-v2 commands now require explicit native actor fields."
~~~

## Task 7: Move guarded publication and PR/MR creation to the owner

**Files:**

- Modify: `internal/core/issueops/issueops_handoff_publication.go`
- Modify: `internal/core/issueops/issueops_handoff_publication_test.go`
- Modify: `internal/core/issueops/issueops_remote_create_claim.go`
- Modify: `internal/core/issueops/issueops_remote_create_claim_test.go`
- Modify: `cmd/harness/issueopscli/remotecmd/remote.go`
- Modify: `cmd/harness/issueopscli/remotecmd/remote_test.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_guard.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_authority.go`
- Modify: `internal/adapter/mcp/issueops_lifecycle_catalog.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops_handlers.go`
- Modify: `cmd/harness/mcpcli/issueops/handoff_test.go`
- Modify: `cmd/harness/testdata/mcp_tools.golden.json`
- Modify: `cmd/harness/testdata/usage.golden.txt`
- Modify: `cmd/harness/testdata/response_contracts.golden.json`

### Step 1: Add publication RED tests

Add:

- `TestProtocolV2OwnerPublishesExactHeadWithoutAccept`
- `TestProtocolV2SourceCannotPublishOrCreatePR`
- `TestProtocolV2RemoteCreateRequiresOwnerAndLatestReceipt`
- `TestProtocolV2RepublishRequiresDescendantSameAuthority`
- `TestProtocolV1PublicationStillRequiresAcceptedCoordinator`

Run:

~~~bash
go test ./internal/core/issueops \
  -run 'TestProtocolV2.*Publish|TestProtocolV2.*RemoteCreate|TestProtocolV1Publication' \
  -count=1
~~~

Expected: RED because current publication requires `closed/accepted` and coordinator identity.

### Step 2: Branch publication authorization by protocol

For protocol v1, keep accepted coordinator rules unchanged.

For protocol v2, require:

- `owner_active`;
- exact owner actor and worker root;
- phase `pr`;
- clean exact local head;
- sealed provider/project/remote/branch/base/workspace;
- no unresolved external operation.

Allow replacing the latest receipt only when authority fields are identical and the new head is a descendant of the previous published head.

### Step 3: Branch remote-create claim authorization

For protocol v2, the owner may create/reconcile the PR/MR after the latest exact publish receipt. Add native actor fields to CLI/MCP remote create and reconcile requests. Preserve the existing canonical body, readback, claim, ambiguity, and no-retry rules.

### Step 4: Update lifecycle enforcement

Permit only the active owner to invoke the guarded publication and remote-create surfaces. Continue blocking raw `git push` and raw provider create commands.

### Step 5: Run GREEN tests

~~~bash
go test ./internal/core/issueops ./cmd/harness/issueopscli/remotecmd ./internal/core/lifecycle \
  -run 'TestProtocolV2.*Publish|TestProtocolV2.*RemoteCreate|TestProtocolV2Republish|TestProtocolV1Publication' \
  -count=1
go test ./internal/adapter/mcp ./cmd/harness/mcpcli/issueops \
  -run 'Test.*Publish|Test.*RemoteCreate|Test.*Handoff' -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
~~~

Expected: PASS after reviewing only publication/remote-create actor and response golden deltas owned by this task.

### Step 6: Commit

~~~bash
git add internal/core/issueops/issueops_handoff_publication.go \
  internal/core/issueops/issueops_handoff_publication_test.go \
  internal/core/issueops/issueops_remote_create_claim.go \
  internal/core/issueops/issueops_remote_create_claim_test.go \
  cmd/harness/issueopscli/remotecmd/remote.go \
  cmd/harness/issueopscli/remotecmd/remote_test.go \
  internal/core/lifecycle/lifecycle_handoff_guard.go \
  internal/core/lifecycle/lifecycle_handoff_authority.go \
  internal/adapter/mcp/issueops_lifecycle_catalog.go \
  cmd/harness/mcpcli/mcp_tool_issueops_handlers.go \
  cmd/harness/mcpcli/issueops/handoff_test.go \
  cmd/harness/testdata/mcp_tools.golden.json \
  cmd/harness/testdata/usage.golden.txt \
  cmd/harness/testdata/response_contracts.golden.json
git commit -m "feat(issueops): let transferred owner publish" \
  -m "Lore:" \
  -m "Intent: Keep push, PR/MR creation, and feedback updates with the handoff owner." \
  -m "Why: Coordinator acceptance and publication contradict full ownership transfer." \
  -m "Changes: Add protocol-aware owner publication, republish, and remote-create authority." \
  -m "Verify: Run v2 publish/create, v1 publication, adapter, and contract-golden regressions." \
  -m "Risk: Publication now has two explicit protocol-specific authority paths."
~~~

## Task 8: Complete directly into the human cleanup boundary

**Files:**

- Create: `internal/core/issueops/issueops_handoff_completion.go`
- Create: `internal/core/issueops/issueops_handoff_completion_test.go`
- Modify: `internal/core/issueops/issueops_handoff_projection.go`
- Modify: `internal/core/issueops/issueops_handoff_lifecycle.go`
- Modify: `internal/core/issueops/issueops_phase.go`
- Modify: `internal/core/issueops/issueops_terminal_phase_handoff_test.go`
- Modify: `cmd/harness/issueopscli/issueops_handoff_cli.go`
- Modify: `cmd/harness/hookcli/hook_stop_worker_done_suppression_test.go`
- Modify: `internal/core/operationalhealth/types.go`
- Modify: `internal/core/operationalhealth/classifier.go`
- Modify: `internal/core/operationalhealth/classifier_test.go`
- Modify: `internal/adapter/operationalhealth/collector.go`
- Modify: `internal/adapter/operationalhealth/collector_test.go`
- Modify: `internal/core/issueops/active/issueops_active.go`
- Modify: `internal/core/issueops/active/active_test.go`
- Modify: `internal/core/issueops/stalescan/stalescan.go`
- Modify: `internal/core/issueops/stalescan/stalescan_handoff_test.go`
- Modify: `internal/core/issueops/issueops_stale_scan.go`
- Modify: `internal/core/issueops/issueops_stale_scan_operational_test.go`
- Modify: `internal/core/issueops/issueops_stale_scan_apply_test.go`
- Modify: `internal/core/issueops/issueops_stale_scan_handoff_test.go`
- Modify: `internal/core/issueops/issueops_force_release.go`
- Modify: `internal/core/issueops/issueops_force_done_test.go`
- Modify: `internal/core/issueops/issueops_force_release_cas_test.go`
- Modify: `internal/adapter/mcp/issueops_lifecycle_catalog.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops_handlers.go`
- Modify: `cmd/harness/mcpcli/issueops/handoff_test.go`
- Modify: `cmd/harness/testdata/mcp_tools.golden.json`
- Modify: `cmd/harness/testdata/usage.golden.txt`
- Modify: `cmd/harness/testdata/response_contracts.golden.json`

### Step 1: Add completion RED tests

Add:

- `TestOwnershipCompleteRequiresFinalRemoteAndFeedbackEvidence`
- `TestOwnershipCompleteAtomicallyStampsDoneAndCleanupPending`
- `TestOwnershipCompletePersistsNotificationIntentBeforeSend`
- `TestOwnershipCompleteNeverInvokesCleanup`
- `TestOwnershipCompleteHasNoCoordinatorAcceptPath`
- `TestOwnershipCleanupPendingIsRetainedOperationalAuthority`
- `TestOwnershipCleanupPendingStaleScanApplyNeverMutates`
- `TestOwnershipNonClosedCannotUseGenericForceReleaseOrCAS`

The no-cleanup fake must panic if terminal close/stop, task update, worktree remove, local branch delete, remote branch delete, merge, or issue close is invoked.

Run:

~~~bash
go test ./internal/core/issueops \
  -run 'TestOwnershipComplete' \
  -count=1
~~~

Expected: RED because v2 complete does not exist.

### Step 2: Implement completion

Add `handoff complete`. Under the cycle lock, revalidate:

- exact owner and fence;
- `owner_active` and phase `pr`;
- clean final HEAD;
- latest publish receipt;
- verified remote artifact at final HEAD;
- no remote-create claim;
- no unresolved feedback;
- bounded changed files, Turing report, and verification.

One write stores completion, stamps phase `done`, enters `cleanup_pending_human_decision`, stores cleanup inventory fingerprint, and stores notification intent.

### Step 3: Project notification only

Reuse the argv-only Orca `worker_done` transport because it is the installed terminal message type, but change the v2 subject/body to state:

- ownership work is complete;
- no coordinator acceptance is required;
- cleanup is pending a human decision;
- no cleanup has run.

The projection may run at most once and may never invoke a cleanup adapter.

### Step 4: Update terminal-phase guard

Allow `done + cleanup_pending_human_decision` only for a valid protocol-v2 completion. Continue rejecting `done` with every other nonterminal v1/v2 handoff state.

### Step 5: Make operational and stale paths understand protocol v2

Extend `operationalhealth.Cycle` with workspace state, handoff protocol, preparation identity, and owner-session identity. Update both projections—`internal/adapter/operationalhealth.cycleFromRecord` and `issueOpsOperationalCycle`—to map `ExecutionWorkspace`, `OwnerSession`, owner terminal/task/dispatch, and canonical worker root. Make `recordOwnsOrca` include workspace inventory. Add an explicit retained-authority result used for ready/provisioning workspace, durable pre-owner, human-cleanup-pending, cleanup-executing, and recovery states; it owns exact resources and keeps bindings valid without pretending that a human-pending state has an owner heartbeat. `owner_active` alone uses `OwnerSession` and the heartbeat TTL. `closed` is dead. Unknown or incomplete envelopes remain unknown and fail safe.

Make every protocol-v2 state known to the classifier and include retained authority in resource-owner indexes. Replace the supervised-only active selector with a protocol-neutral non-closed-handoff selector while keeping the old exported name as a compatibility wrapper if callers require it. Map `OwnerSession`, owner terminal/task/dispatch, and canonical worker root in `issueOpsOperationalCycle`.

In stale scan:

- classify `done + cleanup_pending_human_decision` as `human-cleanup-pending`, `Releasable=false`, never as `handoff-nonterminal-on-terminal-phase`;
- treat every non-`closed` protocol-v2 handoff as live for binding retention and done-record pruning;
- do not force-release it even when the owner heartbeat is absent;
- make git orphan cleanup skip every non-`closed` handoff even if an orphan path field is present;
- preserve existing protocol-v1 stale/recovery semantics.

At the core boundary, make `ForceReleaseIssueOps`, `ForceReleaseIssueOpsCAS`, and `forceReleaseLocked` reject every non-closed protocol-v2 handoff and every live schema-v8 workspace. This protocol/workspace guard must run before the current `phase == done` no-op, otherwise the wrapper can still report success and unbind a cleanup-pending cycle. Generic force release may not stamp an orphan path, mark phase `done`, or unbind these records; only explicit workspace recovery before transfer or Task 9 cleanup after completion may change their resource authority. Preserve protocol-v1 force-release behavior.

Make the apply test a table over every non-`closed` v2 state: `ownership_dispatching`, `ownership_dispatched`, `owner_orienting`, `owner_active`, `cleanup_pending_human_decision`, `cleanup_executing`, and `recovery_required`. Use the state-valid phase/envelope, `Apply=true`, zero `MaxAge`, expired/missing original source and owner sessions, a positive elapsed `PruneDoneAge`, and an apparent orphan path. For every row assert no `Released`, no stale/pruned binding, no pruned record, no Git worktree removal, no terminal/task mutation, and intact workspace/handoff fingerprints. This test is the executable proof that only explicit recovery or Task 9's approved cleanup state machine can remove resources.

### Step 6: Run GREEN tests

~~~bash
go test ./internal/core/issueops ./cmd/harness/issueopscli ./cmd/harness/hookcli \
  -run 'TestOwnershipComplete|TestIssueOpsTerminalPhaseHandoff|Test.*WorkerDone' \
  -count=1
go test ./internal/core/operationalhealth ./internal/adapter/operationalhealth ./internal/core/issueops/active ./internal/core/issueops/stalescan ./internal/core/issueops \
  -run 'TestOwnershipCleanupPending|TestOwnershipNonClosed|Test.*Operational|Test.*StaleScan.*Handoff|Test.*StaleScan.*Apply|TestCycleFromRecord' \
  -count=1
go test ./internal/adapter/mcp ./cmd/harness/mcpcli/issueops \
  -run 'Test.*Complete|Test.*Handoff' -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
~~~

Expected: PASS after reviewing only completion/cleanup-pending action and response golden deltas owned by this task.

### Step 7: Commit

~~~bash
git add internal/core/issueops/issueops_handoff_completion.go \
  internal/core/issueops/issueops_handoff_completion_test.go \
  internal/core/issueops/issueops_handoff_projection.go \
  internal/core/issueops/issueops_handoff_lifecycle.go \
  internal/core/issueops/issueops_phase.go \
  internal/core/issueops/issueops_terminal_phase_handoff_test.go \
  cmd/harness/issueopscli/issueops_handoff_cli.go \
  cmd/harness/hookcli/hook_stop_worker_done_suppression_test.go \
  internal/core/operationalhealth/types.go \
  internal/core/operationalhealth/classifier.go \
  internal/core/operationalhealth/classifier_test.go \
  internal/adapter/operationalhealth/collector.go \
  internal/adapter/operationalhealth/collector_test.go \
  internal/core/issueops/active/issueops_active.go \
  internal/core/issueops/active/active_test.go \
  internal/core/issueops/stalescan/stalescan.go \
  internal/core/issueops/stalescan/stalescan_handoff_test.go \
  internal/core/issueops/issueops_stale_scan.go \
  internal/core/issueops/issueops_stale_scan_operational_test.go \
  internal/core/issueops/issueops_stale_scan_apply_test.go \
  internal/core/issueops/issueops_stale_scan_handoff_test.go \
  internal/core/issueops/issueops_force_release.go \
  internal/core/issueops/issueops_force_done_test.go \
  internal/core/issueops/issueops_force_release_cas_test.go \
  internal/adapter/mcp/issueops_lifecycle_catalog.go \
  cmd/harness/mcpcli/mcp_tool_issueops_handlers.go \
  cmd/harness/mcpcli/issueops/handoff_test.go \
  cmd/harness/testdata/mcp_tools.golden.json \
  cmd/harness/testdata/usage.golden.txt \
  cmd/harness/testdata/response_contracts.golden.json
git commit -m "feat(issueops): stop ownership completion at human cleanup" \
  -m "Lore:" \
  -m "Intent: Finish owner work without handing publication back or deleting resources." \
  -m "Why: Completion and cleanup are separate decisions with different owners." \
  -m "Changes: Add atomic done completion, retained cleanup-pending authority, notification-only projection, and stale-scan protection." \
  -m "Verify: Run completion, both operational projections, stale apply, force-release denial, adapter, hook, and contract-golden tests." \
  -m "Risk: Done cycles may intentionally remain non-closed while awaiting cleanup direction."
~~~

## Task 9: Enforce explicit human-directed cleanup

**Files:**

- Create: `internal/core/issueops/issueops_ownership_cleanup.go`
- Create: `internal/core/issueops/issueops_ownership_cleanup_test.go`
- Modify: `internal/core/issueops/issueops_handoff_recovery.go`
- Modify: `internal/core/issueops/handoff/envelope.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_guard.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_authority.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_guard_test.go`
- Modify: `cmd/harness/issueopscli/issueops_handoff_cli.go`
- Modify: `cmd/harness/hookcli/hook_stop.go`
- Create: `cmd/harness/hookcli/hook_stop_cleanup_decision_test.go`
- Modify: `internal/adapter/mcp/issueops_lifecycle_catalog.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops_handlers.go`
- Modify: `cmd/harness/mcpcli/issueops/handoff_test.go`
- Modify: `cmd/harness/testdata/mcp_tools.golden.json`
- Modify: `cmd/harness/testdata/usage.golden.txt`
- Modify: `cmd/harness/testdata/response_contracts.golden.json`

### Step 1: Add cleanup hard-boundary RED tests

Add:

- `TestOwnershipCleanupPreviewIsReadOnly`
- `TestOwnershipCleanupPreviewRequiresAuthenticatedExactSourceRoot`
- `TestOwnershipCleanupFreshSourceSessionCanBeExplicitlyApprovedAfterOriginalExits`
- `TestOwnershipCleanupNeverSilentlyInheritsOrRebindsAuthority`
- `TestOwnershipCleanupRejectsCompletedOwnerAsSourceCandidate`
- `TestOwnershipCleanupCannotApproveStaleInventory`
- `TestOwnershipCleanupCloseOwnerOrderedReceipts`
- `TestOwnershipCleanupRemoveLocalRequiresRemoteReachability`
- `TestOwnershipCompletionAndStopHooksNeverAutoCleanup`
- `TestCleanupPendingNeverFencesOrdinarySourceWork`

The source-availability test keeps an exact cycle in `cleanup_pending_human_decision` and `cleanup_executing`, then proves ordinary source edit/Git/MCP requests remain allowed while exact cleanup commands remain identity-, fingerprint-, disposition-, and state-gated.

Run:

~~~bash
go test ./internal/core/issueops ./internal/core/lifecycle ./cmd/harness/hookcli \
  -run 'TestOwnershipCleanup|TestOwnershipCompletionAndStopHooksNeverAutoCleanup|TestCleanupPendingNeverFencesOrdinarySourceWork' \
  -count=1
~~~

Expected: RED.

### Step 2: Implement read-only preview

`handoff cleanup-preview` returns:

- exact inventory and SHA-256 fingerprint;
- the authenticated candidate cleanup session and exact sealed source root;
- readiness blockers;
- exactly three user-facing choices;
- exact commands for choices 2 and 3;
- no default selected disposition;
- no state or external mutation.

Choice 1 executes no command and leaves `cleanup_pending_human_decision` unchanged.

Any authenticated native session other than `OwnerSession` whose canonical CWD is the sealed source root may preview, including a fresh session after the original preparation session has exited. Preview is observation only: it does not rebind the IssueOps session, change the coordinator identity, or create cleanup authority. The completed owner remains status-only and cannot become its own cleanup candidate by changing CWD.

### Step 3: Implement approval

`handoff cleanup-approve` accepts only:

- `close-owner`;
- `remove-local`.

Require the exact candidate session from preview, exact sealed source cwd, preview fingerprint, bounded reason, and `--confirm`. Under the cycle lock, re-read the completion HEAD, cycle/attempt/epoch, resource inventory, current state, and absence of a different cleanup executor before atomically writing `cleanup_executing` and `Cleanup.ApprovedBySession`.

The original preparation session has no implicit cleanup privilege and follows the same preview/confirm path. A fresh authenticated source session can be selected only by this explicit human-confirmed write. Do not reuse generic session binding as cleanup authority. Once any external cleanup receipt exists, approval cannot be rebound; recovery must stop for a new inventory preview and human confirmation, and must never replay an ambiguous mutation.

### Step 4: Gate and record the ordered external sequence

Reuse the existing explicit cleanup pattern: only `Cleanup.ApprovedBySession` invokes exact Orca/Git commands from the source root, then `handoff cleanup-record` verifies live state and writes one idempotent receipt.

For `close-owner`:

~~~text
task_terminal -> terminal_quiescent -> closed/owner_closed_workspace_retained
~~~

For `remove-local`:

~~~text
remote_head_safe -> task_terminal -> terminal_quiescent
-> worktree_removed -> local_branch_removed
-> closed/local_resources_removed
~~~

Each step requires all prior receipts. A timeout or ambiguous readback moves to `recovery_required` without guessing.

### Step 5: Make the Stop path a hard human gate

When an authenticated source-root session observes `cleanup_pending_human_decision`:

- inject that no cleanup has run;
- instruct the main agent to present the three preview choices;
- prohibit auto-proceed;
- never call preview, approve, record, Orca, Git, or provider tools from the hook.

The owner Stop suppression remains scoped to the completion notification and cannot grant cleanup authority.

### Step 6: Run GREEN tests

~~~bash
go test ./internal/core/issueops ./internal/core/lifecycle ./cmd/harness/hookcli \
  -run 'TestOwnershipCleanup|TestOwnershipCompletionAndStopHooksNeverAutoCleanup|TestCleanupPendingNeverFencesOrdinarySourceWork|TestHandoffCleanup' \
  -count=1
go test ./internal/adapter/mcp ./cmd/harness/mcpcli/issueops \
  -run 'Test.*Cleanup|Test.*Handoff' -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
~~~

Expected: PASS after reviewing only cleanup preview/approval/receipt actor and response golden deltas owned by this task.

### Step 7: Commit

~~~bash
git add internal/core/issueops/issueops_ownership_cleanup.go \
  internal/core/issueops/issueops_ownership_cleanup_test.go \
  internal/core/issueops/issueops_handoff_recovery.go \
  internal/core/issueops/handoff/envelope.go \
  internal/core/lifecycle/lifecycle_handoff_guard.go \
  internal/core/lifecycle/lifecycle_handoff_authority.go \
  internal/core/lifecycle/lifecycle_handoff_guard_test.go \
  cmd/harness/issueopscli/issueops_handoff_cli.go \
  cmd/harness/hookcli/hook_stop.go \
  cmd/harness/hookcli/hook_stop_cleanup_decision_test.go \
  internal/adapter/mcp/issueops_lifecycle_catalog.go \
  cmd/harness/mcpcli/mcp_tool_issueops_handlers.go \
  cmd/harness/mcpcli/issueops/handoff_test.go \
  cmd/harness/testdata/mcp_tools.golden.json \
  cmd/harness/testdata/usage.golden.txt \
  cmd/harness/testdata/response_contracts.golden.json
git commit -m "feat(issueops): require human-directed ownership cleanup" \
  -m "Lore:" \
  -m "Intent: Reserve final local resource cleanup for an explicit human choice and authenticated source-session approval." \
  -m "Why: Owner completion cannot decide whether terminals, worktrees, or branches should be retained." \
  -m "Changes: Add fresh-session-safe preview, fingerprinted actor approval, ordered receipts, and a hard Stop-hook human gate." \
  -m "Verify: Run ownership/legacy cleanup, succession, lifecycle, adapter, Stop-hook, and contract-golden tests." \
  -m "Risk: Cleanup remains pending indefinitely when the human chooses to retain resources."
~~~

## Task 10: Audit CLI/MCP/native-host contract parity

**Files:**

- Modify: `internal/adapter/mcp/issueops_catalog_test.go`
- Modify: `cmd/harness/mcpcli/issueops/handoff_test.go`
- Modify: `cmd/harness/issueopscli/issueops_handoff_cli_test.go`
- Modify: `cmd/harness/hookcli/hook_pre_tool_handoff_test.go`
- Modify: `internal/adapter/codex/install_test.go`
- Modify: `internal/adapter/claude/install_test.go`
- Modify: `internal/adapter/gjc/install_test.go`

### Step 1: Add one cross-surface matrix test

Add `TestOwnershipTransferCLIAndMCPActionParity`. For every action:

~~~text
start
claim
acknowledge-context
publish
complete
cleanup-preview
cleanup-approve
cleanup-record
recover
~~~

Assert CLI flag presence, MCP schema fields, handler dispatch, lifecycle action recognition, and response mode fields agree.

Add host-shape tests showing the same allow/block meaning through Codex flat block JSON, Claude `permissionDecision`, and GJC translation.

Run:

~~~bash
go test ./internal/adapter/mcp ./cmd/harness/mcpcli/issueops ./cmd/harness/hookcli \
  ./internal/adapter/codex ./internal/adapter/claude ./internal/adapter/gjc \
  -run 'TestOwnershipTransferCLIAndMCPActionParity|Test.*Ownership.*Host|Test.*PreToolUse.*Handoff' \
  -count=1
~~~

Expected after Tasks 3-9: PASS. If any row is RED, return to the vertical task that owns that action, add the focused RED assertion there, fix and update that task's goldens, then restart this audit. Do not repair production wiring in a catch-all parity commit.

### Step 2: Verify action-discriminated MCP inputs

Confirm the already-wired single `issueops_handoff` tool has action-conditional properties without separate handoff tools. Descriptions must name:

- which actions write;
- required native actor fields;
- preview versus confirm behavior;
- no automatic cleanup;
- result shape and handoff mode.

### Step 3: Verify goldens without updating them

Run:

~~~bash
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
~~~

Expected: PASS with no golden diff. A failure belongs to the earlier vertical task that introduced the mismatched surface.

### Step 4: Run cross-surface GREEN tests

~~~bash
go test ./internal/adapter/mcp ./cmd/harness/mcpcli/issueops ./cmd/harness/hookcli \
  ./internal/adapter/codex ./internal/adapter/claude ./internal/adapter/gjc \
  -run 'TestOwnershipTransferCLIAndMCPActionParity|Test.*Ownership.*Host|Test.*PreToolUse.*Handoff' \
  -count=1
~~~

Expected: PASS.

### Step 5: Commit

~~~bash
git add internal/adapter/mcp/issueops_catalog_test.go \
  cmd/harness/mcpcli/issueops/handoff_test.go \
  cmd/harness/issueopscli/issueops_handoff_cli_test.go \
  cmd/harness/hookcli/hook_pre_tool_handoff_test.go \
  internal/adapter/codex/install_test.go \
  internal/adapter/claude/install_test.go \
  internal/adapter/gjc/install_test.go
git commit -m "test(issueops): audit ownership transfer parity" \
  -m "Lore:" \
  -m "Intent: Expose identical ownership-transfer meaning across CLI, MCP, Codex, Claude, and GJC." \
  -m "Why: The live incident showed MCP and CLI authority divergence." \
  -m "Changes: Add one cross-surface action and host-translation audit over the already-wired vertical contracts." \
  -m "Verify: Run adapter parity and immutable contract golden tests." \
  -m "Risk: New parity rows may expose drift that must be fixed in the owning vertical commit."
~~~

## Task 11: Update IssueOps operating contracts

**Files:**

- Modify: `.agent-harness/ARCHITECTURE.md`
- Modify: `.agent-harness/ADR.md`
- Modify: `.agent-harness/OPERATIONS.md`
- Modify: `.agent-harness/AGENT_WORKFLOW.md`
- Modify: `.agent-harness/CAUTIONS.md`
- Modify: `skills/issueops/SKILL.md`
- Modify: `skills/issueops/references/orca-handoff.md`
- Modify: `skills/issueops/references/worktree-context.md`
- Modify: `skills/issueops/references/cleanup-state.md`
- Modify: `docs/superpowers/specs/2026-07-20-issueops-ownership-transfer-design.md`
- Modify: `internal/core/issueops/issueops_skill_contract_test.go`

### Step 1: Add docs-contract RED assertions

Extend `issueops_skill_contract_test.go` to require:

- workspace provisioning before ownership transfer;
- protocol-independent source-main availability before, during, and after handoff;
- fence selection by canonical worker root, exact cycle ID, native owner, or persisted Orca resource rather than source CWD/session binding;
- plan-only commit before confirmed start;
- protocol-v1 versus protocol-v2 role distinction;
- owner orientation and post-handoff authority;
- source disengagement from the exact cycle without blocking unrelated source-root work;
- no accept in v2;
- `cleanup_pending_human_decision`;
- no automatic cleanup;
- explicit three-choice cleanup preview;
- explicit cleanup-session selection/re-authentication when the original source session is gone;
- operational-health/stale-scan preservation of every non-`closed` v2 resource;
- existing v1 records remain v1 and are never schema- or background-converted.

Run:

~~~bash
go test ./internal/core/issueops \
  -run 'TestIssueOps.*SkillContract|TestIssueOps.*Ownership' \
  -count=1
~~~

Expected: RED until docs are updated.

### Step 2: Update architecture and ADR

Record:

- the incident/root cause;
- source-CWD fallback and mirrored-path blocking as the incorrect fence boundary;
- the separate workspace and ownership state machines;
- protocol-v1 compatibility;
- v2 source/owner authority;
- publication ownership;
- human cleanup boundary;
- rejected allowlist-only alternatives.

The docs must state that a session binding is routing metadata, same-relative-path detection is at most a non-blocking warning, and ordinary source file/Git/MCP work stays available in every v1/v2 state. They must separately list the still-fenced surfaces: canonical isolated root, exact cycle lifecycle commands, IssueOps branch topology, and persisted Orca resources.

The prior 2026-07-11 supervised design remains historical. Mark it as protocol-v1 rather than rewriting history.

### Step 3: Update operator and skill flows

Document the exact happy path:

~~~text
worktree prepare
-> plan and gates
-> plan-only commit
-> handoff start preview/confirm
-> owner claim
-> acknowledge-context
-> implement through PR/MR
-> handoff complete
-> human cleanup choice
~~~

At every arrow, the source main worktree may be used for other work. “Disengage” means no more mutation or steering of this exact cycle; it never means the source checkout becomes read-only.

Document `io-b9f8cd45e152` only as time-stamped, read-only incident evidence. Do not publish a conversion, dispatch, cancellation, or cleanup command for that live v1 record. Its future disposition requires a separate live readback and human decision after the new implementation is installed; this implementation plan does not authorize either.

Document cleanup succession explicitly: any authenticated session may preview from the exact sealed source root, but only the candidate session named in a later human-confirmed approval receives cleanup authority. Generic session binding, original coordinator identity, Stop hooks, stale scan, and elapsed time never grant or transfer it.

### Step 4: Validate the skill and docs tests

~~~bash
python3 scripts/validate-skill.py skills/issueops
go test ./internal/core/issueops \
  -run 'TestIssueOps.*SkillContract|TestIssueOps.*Ownership' \
  -count=1
~~~

Expected: PASS.

### Step 5: Commit

~~~bash
git add .agent-harness/ARCHITECTURE.md \
  .agent-harness/ADR.md \
  .agent-harness/OPERATIONS.md \
  .agent-harness/AGENT_WORKFLOW.md \
  .agent-harness/CAUTIONS.md \
  skills/issueops/SKILL.md \
  skills/issueops/references/orca-handoff.md \
  skills/issueops/references/worktree-context.md \
  skills/issueops/references/cleanup-state.md \
  docs/superpowers/specs/2026-07-20-issueops-ownership-transfer-design.md \
  internal/core/issueops/issueops_skill_contract_test.go
git commit -m "docs(issueops): define ownership transfer operations" \
  -m "Lore:" \
  -m "Intent: Make the new authority and cleanup contracts executable by future sessions." \
  -m "Why: The old supervised docs directly contributed to the coordinator-preparing deadlock." \
  -m "Changes: Document worker-root-scoped fences, source availability, v1/v2 roles, workspace ordering, owner publication, cleanup succession, and stale-path preservation." \
  -m "Verify: Validate the IssueOps skill and run docs-contract tests." \
  -m "Risk: Static examples must never guess native session identity."
~~~

## Task 12: Run the all-or-nothing verification gate

**Files:**

- Modify only if a test exposes an implementation defect in a file already listed above.
- Do not make opportunistic cleanup or unrelated refactors.

### Step 1: Check the diff and unresolved markers

~~~bash
git status --short
git diff --check
rg -n 'TODO|TBD|FIXME|DECISION NEEDED' \
  internal/core/issueops internal/core/lifecycle cmd/harness internal/adapter \
  skills/issueops .agent-harness \
  docs/superpowers/specs/2026-07-20-issueops-ownership-transfer-design.md
~~~

Expected:

- only planned files are changed;
- `git diff --check` is silent;
- no unresolved marker exists in executable code or operating contracts;
- no command proposes converting, dispatching, cancelling, or cleaning the live v1 incident cycle.

### Step 2: Run focused ownership suites

~~~bash
go test ./internal/core/issueops/handoff ./internal/core/issueops ./internal/core/issueops/active \
  ./internal/core/issueops/stalescan ./internal/core/operationalhealth ./internal/core/lifecycle \
  ./cmd/harness/issueopscli ./cmd/harness/mcpcli/issueops ./cmd/harness/hookcli \
  -run 'Ownership|Handoff|ExecutionWorkspace|SchemaV8|FenceScope|WorktreeGuard.*Source|SourceCheckoutMisdirect' \
  -count=1
~~~

Expected: PASS.

### Step 3: Run contract and skill suites

~~~bash
python3 scripts/validate-skill.py skills/issueops
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
~~~

Expected: PASS.

### Step 4: Run the repository gate from the beginning

~~~bash
go mod tidy
git diff --exit-code -- go.mod go.sum
go test ./... -count=1
go test -race ./... -count=1
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
~~~

Expected:

- module files unchanged unless an intentional standard-library-independent dependency was approved in advance;
- all tests and race tests pass;
- build succeeds;
- both self-verification runs meet or exceed 95.

If any step fails, fix the defect and restart this entire Step 4 sequence at `go mod tidy`. Do not combine partial passes from different runs.

### Step 5: Inspect binary behavior without installing it

Use a temporary state root and fake/test fixtures only. Run:

~~~bash
tmp_state="$(mktemp -d)"
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness issueops --help
rm -rf "$tmp_state"
~~~

Expected: usage includes the new handoff actions. This command must not contact Orca or mutate the installed native integration.

### Step 6: Commit verification fixes at their owning task

If Step 4 exposes a defect, return to the task that owns that behavior, add a focused RED test, make the minimum fix, and create that task's atomic fix commit with the exact changed files. Do not make a generic final catch-all commit, stage by directory, or use `git add -A`. Then restart Task 12 Step 4 from `go mod tidy`.

## Post-implementation human boundary

Do not install the binary or mutate, convert, dispatch, cancel, or clean `io-b9f8cd45e152` as part of this implementation plan.

After code review and a human-approved native installation, exercise the ownership-transfer protocol first on a new temporary-state or newly created cycle. The source main session should:

1. verify source-root ordinary work stays available while another isolated cycle is ready and active;
2. finish the new cycle's plan/setup gates as the sealed preparation session;
3. preview and confirm protocol-v2 ownership transfer;
4. disengage from that exact cycle until the owner completes, while remaining free to perform unrelated work in the source main worktree;
5. present cleanup choices and wait for a second explicit human direction;
6. if the original source session has exited, prove that a fresh authenticated source session receives cleanup authority only through preview plus explicit human-confirmed approval.

The existing v1 incident remains a separate human decision after a fresh live readback. This plan supplies no automatic or in-place conversion path for it.

No step in this post-implementation boundary is implied by code completion.
