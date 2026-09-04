# IssueOps Owner Restart Ledger Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Do not dispatch sub-agents unless the user explicitly authorizes them.

**Goal:** Make an interrupted supervised handoff restartable without overwriting prior ownership evidence, losing dirty WIP, reviving stale Orca identities, or treating ownership release as workflow completion.

**Architecture:** IssueOps schema v9 separates workflow `phase` from top-level `cycle_state` and replaces the single mutable `execution_workspace`/`execution_handoff` pair with an append-only ownership-attempt ledger. A source-authorized `handoff restart-owner` operation previews exact Git/Orca inventory, seals dirty non-ignored WIP into a hidden local Git recovery ref without changing the real branch/index/worktree, journals external intent, then appends a new attempt with fresh ownership/workspace epochs and fresh Orca task/dispatch identities. Previous attempts remain immutable audit evidence; mixed-version binaries fail closed.

**Tech Stack:** Go 1.26, SQLite-backed IssueOps JSON records, Git plumbing with a temporary index, Orca CLI adapter, CLI `flag`, MCP action-discriminated tools, lifecycle hooks, Go table/race tests, response-contract goldens.

## Global Constraints

- Preserve all existing `#68` commits and dirty files; implementation must run in its canonical worker worktree only after the new plan is committed and an official owner restart succeeds.
- Keep source/main available and observation-only for active worker attempts. `restart-owner` is the only source-side operation allowed to create a successor attempt.
- Never rewrite or delete a prior attempt. Attempt numbers are monotonic and ownership/workspace epochs are unique.
- Never use runtime, terminal title, elapsed time, or a stable-looking diff as ownership proof.
- Never hold the IssueOps cycle lock across Git plumbing or Orca calls. Persist intent first, release the lock, perform at most one external mutation, then CAS-finalize or remain recovery-required.
- Dirty WIP sealing must not change `HEAD`, the current branch ref, the real index, tracked/untracked worktree bytes, file modes, or submodule state. The first release rejects any staged/index divergence; the observed `#68` worktree contains only unstaged tracked and untracked changes.
- WIP sealing includes only non-ignored files inside the canonical worktree. Reject staged or unmerged entries, nested repositories, dirty submodules, unsafe symlinks, candidate paths rejected by the existing secret/path guard, and bounded-size violations before creating a Git object.
- No automatic force-push, merge, PR/MR creation, remote recovery-ref push, worktree deletion, or cycle close.
- Root schema v9 is a security boundary. A schema-v8 binary must reject v9 before writing; a v9 binary must migrate v8 only through the explicit deterministic rules below.
- CLI, MCP, daemon-backed MCP, Codex, Claude, and GJC must expose the same schema and lifecycle result.

## File and State Model

The schema-v9 record owns these top-level fields:

```go
type IssueOpsCycleState string

const (
    IssueOpsCycleActive IssueOpsCycleState = "active"
    IssueOpsCyclePaused IssueOpsCycleState = "paused"
    IssueOpsCycleClosed IssueOpsCycleState = "closed"
)

type IssueOpsOwnershipLedger struct {
    ActiveAttempt  int                         `json:"active_attempt,omitempty"`
    Attempts       []IssueOpsOwnershipAttempt  `json:"attempts,omitempty"`
    PendingRestart *IssueOpsOwnerRestartIntent `json:"pending_restart,omitempty"`
}

type IssueOpsOwnershipAttempt struct {
    Number      int                         `json:"number"`
    Workspace   *IssueOpsExecutionWorkspace `json:"workspace,omitempty"`
    Handoff     *IssueOpsExecutionHandoff   `json:"handoff,omitempty"`
    InheritedWIPSeal *IssueOpsOwnershipWIPSeal `json:"inherited_wip_seal,omitempty"`
    RestartedFrom int                       `json:"restarted_from,omitempty"`
    StartedAt   string                      `json:"started_at"`
    ClosedAt    string                      `json:"closed_at,omitempty"`
}

type IssueOpsOwnershipWIPSeal struct {
    Ref                string `json:"ref"`
    Commit             string `json:"commit"`
    Tree               string `json:"tree"`
    BaseHead           string `json:"base_head"`
    StatusSHA256       string `json:"status_sha256"`
    PathManifestSHA256 string `json:"path_manifest_sha256"`
    PathCount          int    `json:"path_count"`
    CreatedAt          string `json:"created_at"`
}

type IssueOpsOwnerRestartIntent struct {
    State                string                       `json:"state"` // intent|recovery_required
    FromAttempt          int                          `json:"from_attempt"`
    ToAttempt            int                          `json:"to_attempt"`
    InventoryFingerprint string                       `json:"inventory_fingerprint"`
    Head                 string                       `json:"head"`
    StatusSHA256         string                       `json:"status_sha256"`
    Dirty                bool                         `json:"dirty"`
    SealDirtyWIP         bool                         `json:"seal_dirty_wip"`
    WIPSeal              *IssueOpsOwnershipWIPSeal    `json:"wip_seal,omitempty"`
    RequestedBy          *IssueOpsHostSessionIdentity `json:"requested_by"`
    CoordinatorRecipient string                       `json:"coordinator_recipient"`
    PriorWorktreeID      string                       `json:"prior_worktree_id"`
    PriorTaskID          string                       `json:"prior_task_id"`
    PriorDispatchID      string                       `json:"prior_dispatch_id"`
    Failure              *IssueOpsExecutionHandoffFailure `json:"failure,omitempty"`
    StartedAt            string                       `json:"started_at"`
    UpdatedAt            string                       `json:"updated_at"`
}
```

`IssueOpsRecord` adds `CycleState IssueOpsCycleState` and `Ownership *IssueOpsOwnershipLedger`; schema-v9 records no longer serialize the top-level `execution_workspace` and `execution_handoff` fields. Existing `force_released_at`, `force_release_reason`, and `orphan_worktree_path` remain audit fields and never select live authority. `phase` remains the ordered problem→done workflow milestone. The valid combinations are:

| cycle_state | phase | active_attempt | Meaning |
|---|---|---:|---|
| `active` | any phase except `done` | zero before handoff, otherwise exactly one nonterminal attempt | workflow can advance |
| `paused` | any phase except `done` | zero | no mutation owner; restart or explicit close is allowed |
| `closed` | `done` | zero | workflow is terminal; restart is forbidden |

The ledger owns all mutation authority. Read-only projections use `CurrentOwnershipAttempt(record)` and `LastOwnershipAttempt(record)`; no caller may select `Attempts[len-1]` or consult a historical attempt as live authority. A paused cycle releases the source coordinator fence, but every historical worker root and Orca identity remains quarantined: no historical owner or source shell may mutate it. Only exact-source `restart-owner` preview/confirm and separately approved cleanup operations can act on those retained resources.

The dirty seal belongs to the successor attempt, never the predecessor. Restart from attempt 1 creates attempt 2 with `RestartedFrom=1` and `InheritedWIPSeal=<seal>`. Attempt 1 remains byte-identical, `pending_restart` can be cleared after CAS, and the successor has complete recovery provenance.

Receipt ownership is singular. `pending_restart` contains only pre-restart inventory, intent, optional WIP-seal receipt, and bounded failure. It never stores new terminal/task/dispatch identities. After successor creation, attempt 2's existing handoff `pending_operation` and Orca identity are the only durable writers/locations for terminal, task, and dispatch receipts.

## Deterministic v8 → v9 Migration

1. A v8 record without ownership fields becomes v9 with `cycle_state=active` unless `phase=done`, in which case it becomes `closed`.
2. A v8 record with ownership fields becomes one ledger attempt. Existing attempt/workspace/handoff bytes are structurally copied; the attempt number and epochs must agree or migration fails closed.
3. `phase=done + handoff.state=closed + closed_disposition=cancelled + force_released_at present + remote_artifact absent` becomes `cycle_state=paused`, `active_attempt=0`, and phase is restored to the highest entered non-`done` phase in `phase_ledger`. If no such phase exists, migration fails and requires operator repair; it must not guess `plan` or `implement`.
4. A verified owner completion plus a matching remote artifact at `phase=done` becomes `closed`.
5. Other `phase=done` v8 records become `closed`; they are not restart candidates.
6. Nonterminal v8 handoffs keep the corresponding attempt active only when their envelope validates completely. Invalid, ambiguous, or removed shapes remain byte-identical and fail closed.
7. Migration is in-memory on read and persisted only on the next authorized write or an explicit exact-ID `issueops migrate-v9 --id ID --preview/--confirm` operation. Preview reports that one record's classification without writing.

## Independent Review Resolution

The first independent Brooks pass returned `stop`. Its decisive finding was that the draft placed a newly created WIP seal on a predecessor attempt that was also required to remain byte-immutable. This revision resolves that contradiction by placing the seal on the successor attempt as `InheritedWIPSeal`, proving the transition in Task 0 before production schema work, and retaining the predecessor bytes unchanged.

The revision also applies the smaller-plan findings: exact-ID-only migration, no arbitrary attempt-count cap, staged/index divergence rejected in the first release, reuse of the existing staged Orca dispatcher, historical-resource quarantine during pause, host rollout before live v9 writes, and removal of `#51` cleanup from the implementation scope. The independent verifier's missing consumers and operational-health path corrections are included in Task 3. After the successor-owned seal and singular dispatcher-receipt corrections, the final independent verdicts are Brooks `proceed` and verifier `OKAY`; implementation may begin only from this reviewed revision.

---

### Task 0: Falsify the Transition and Reuse Assumptions with Disposable Spikes

**Files:**
- Create: `internal/core/issueops/issueops_owner_restart_spike_test.go`
- Modify: no production file

**Interfaces:**
- Produces executable evidence for the final state transition and dispatcher-reuse decision; the spike test is replaced by focused production tests in later tasks.

- [ ] **Step 1: Prove predecessor immutability and successor seal ownership**

Build a pure in-memory paused record with attempt 1 and no seal. Apply the proposed restart transition and assert all three conditions simultaneously: serialized attempt 1 bytes are unchanged, `pending_restart` is nil, and attempt 2 has `RestartedFrom=1` plus the inherited seal. If the test cannot express this without an exception, stop and redesign before schema work.

- [ ] **Step 2: Prove paused-resource quarantine**

Table-test that paused state releases ordinary source-cycle fencing while historical owner identity, worker-root mutation, and historical Orca task/terminal/dispatch control remain denied. The only allowed mutation is exact-source typed restart preview/confirm.

- [ ] **Step 3: Prove the hidden-ref mechanism on a copied #68-shaped worktree**

Create a disposable Git worktree with unstaged tracked and untracked changes only. Use a temporary index and hidden ref, then assert HEAD, branch ref, real index checksum, status manifest, tracked/untracked bytes, and modes are unchanged. Restore a second checkout from the ref and compare content. Reject staged entries before any Git object/ref mutation.

- [ ] **Step 4: Fault-inject the existing dispatcher**

Use its existing fake Orca seams to interrupt after terminal create, task create, dispatch, and record CAS. Confirm the current per-stage `pending_operation` and inventory reconciliation can resume a successor attempt without duplicate external mutation. If reuse fails, revise the plan with the smallest missing stage marker; do not add a parallel restart dispatcher.

- [ ] **Step 5: Verify the spike**

```bash
go test ./internal/core/issueops -run 'OwnerRestartSpike' -count=1
```

Expected: PASS before Task 1 begins. Remove or convert spike-only assertions when the same contracts are covered by production tests.

---

### Task 1: Add Schema-v9 Cycle and Ownership Ledger Contracts

**Files:**
- Modify: `internal/core/issueops/model/types.go`
- Modify: `internal/core/issueops/package.go`
- Modify: `internal/core/issueops/handoff/envelope.go`
- Modify: `internal/core/issueops/handoff/ownership_envelope.go`
- Modify: `internal/core/issueops/handoff/ownership_envelope_test.go`
- Create: `internal/core/issueops/issueops_ownership_ledger.go`
- Create: `internal/core/issueops/issueops_ownership_ledger_test.go`

**Interfaces:**
- Produces the exact schema types shown above.
- Produces `CurrentOwnershipAttempt(record)`, `LastOwnershipAttempt(record)`, and append-only/CAS helpers.
- Removes direct live-authority reads from top-level `ExecutionWorkspace` and `ExecutionHandoff` in schema-v9 code.

- [ ] **Step 1: Write RED model/envelope tests**

Cover empty ledgers, duplicate/non-monotonic attempt numbers, multiple active attempts, active pointer to a closed/missing attempt, historical attempt mutation, epoch reuse, malformed inherited WIP seals, and every invalid `cycle_state`/`phase` combination. Add a schema ownership test proving `pending_restart` has no terminal/task/dispatch result fields and those receipts exist only under the successor handoff. Rely on the existing bounded record-size gate rather than adding an incident-unrelated attempt-count policy.

- [ ] **Step 2: Verify RED**

Run:

```bash
go test ./internal/core/issueops/handoff ./internal/core/issueops -run 'SchemaV9|OwnershipLedger|CycleState' -count=1
```

Expected: FAIL because schema v9 and ledger helpers do not exist.

- [ ] **Step 3: Implement the minimum model and validators**

Set `IssueOpsCurrentSchemaVersion = 9`, add canonical enum validation, validate every historical attempt independently, and expose live authority only through `ActiveAttempt`. Keep clone/CAS helpers deep-copy-safe.

- [ ] **Step 4: Verify GREEN**

Run the focused command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit**

Suggested subject:

```text
feat(issueops): add ownership attempt ledger
```

---

### Task 2: Implement Explicit v8 Migration and Mixed-Version Fail-Closed Behavior

**Files:**
- Modify: `internal/core/issueops/issueops_state.go`
- Modify: `internal/core/issueops/issueops_schema_version_test.go`
- Create: `internal/core/issueops/issueops_schema_v9_migration.go`
- Create: `internal/core/issueops/issueops_schema_v9_migration_test.go`
- Modify: `cmd/issueops/issueopscli/issueops.go`
- Modify: `cmd/issueops/issueopscli/issueops_cli_support.go`
- Create: `cmd/issueops/issueopscli/issueops_migrate_v9_cli_test.go`

**Interfaces:**
- Produces `ClassifyIssueOpsV8Migration(raw)` and `MigrateIssueOpsV8Record(record)`.
- Adds exact-record-only `issueops migrate-v9 --id ID --preview --json` and `--id ID --confirm` with raw/canonical digest CAS. There is no bulk/all-record mode in the first release.

- [ ] **Step 1: Add immutable v8 fixtures**

Add fixtures for no handoff, active owner, completed owner, `#68`-shape cancelled/force-released/no-artifact, ambiguous done state, invalid epochs, and future schema 10. Assert v8 input bytes are unchanged on failed migration and schema 10 never reaches a write path. A separate operational cleanup must not be used as a migration fixture.

- [ ] **Step 2: Verify RED**

```bash
go test ./internal/core/issueops ./cmd/issueops/issueopscli -run 'SchemaV9Migration|MigrateV9|FutureSchema' -count=1
```

Expected: FAIL because migration classification and CLI do not exist.

- [ ] **Step 3: Implement deterministic classification and CAS persistence**

Apply the seven migration rules above exactly. Derive paused-record phase only from existing `phase_ledger`; return a machine-readable blocker otherwise. The confirm path re-reads raw and canonical SHA-256 under the cycle lock before writing schema 9.

- [ ] **Step 4: Verify GREEN and a real-state copy**

Copy the current IssueOps database to a temporary state root, run preview and confirm there, and prove the original database digest is unchanged.

- [ ] **Step 5: Commit**

Suggested subject:

```text
feat(issueops): migrate ownership records to schema v9
```

---

### Task 3: Move Active-Cycle, Guard, Health, and Cleanup Reads to the Ledger

**Files:**
- Modify: `internal/core/issueops/active/issueops_active.go`
- Modify: `internal/core/issueops/stalescan/stalescan.go`
- Modify: `internal/core/issueops/issueops_stale_scan.go`
- Modify: `internal/core/issueops/issueops_force_release.go`
- Modify: `internal/core/issueops/issueops_force_done.go`
- Modify: `internal/core/issueops/issueops_phase.go`
- Modify: `internal/core/issueops/issueops_actor.go`
- Modify: `internal/core/issueops/issueops_readiness.go`
- Modify: `internal/core/issueops/issueops_handoff_lifecycle.go`
- Modify: `internal/core/issueops/issueops_handoff_orientation.go`
- Modify: `internal/core/issueops/issueops_execution_workspace_recovery.go`
- Modify: `internal/core/issueops/issueops_handoff_publication.go`
- Modify: `internal/core/issueops/issueops_remote_create_claim.go`
- Modify: `internal/core/issueops/issueops_ownership_completion.go`
- Modify: `internal/core/issueops/issueops_ownership_cleanup.go`
- Modify: `internal/adapter/operationalhealth/collector.go`
- Modify: `internal/adapter/operationalhealth/collector_test.go`
- Modify: `internal/core/lifecycle/lifecycle_workspace_guard.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_guard.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_authority.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_resource_target.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_scope.go`
- Modify: `internal/core/lifecycle/lifecycle_ownership_cleanup_stop.go`
- Modify: `internal/core/lifecycle/lifecycle_stop_worker_done_suppression.go`
- Modify: `cmd/issueops/mcpcli/mcp_tool_issueops.go`
- Modify: `cmd/issueops/issueopscli/remotecmd/remote.go`
- Modify: `internal/core/issueops/active/active_test.go`
- Modify: `internal/core/issueops/stalescan/stalescan_test.go`
- Modify: `internal/core/issueops/issueops_force_release_cas_test.go`
- Modify: `internal/core/issueops/issueops_phase_lifecycle_test.go`
- Modify: `internal/core/issueops/issueops_ownership_completion_test.go`
- Modify: `internal/core/issueops/issueops_actor_test.go`
- Modify: `internal/core/issueops/issueops_readiness_test.go`
- Modify: `internal/core/issueops/issueops_handoff_orientation_test.go`
- Create: `internal/core/issueops/issueops_execution_workspace_recovery_ledger_test.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_ownership_authority_test.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_resource_target_test.go`
- Create: `internal/core/lifecycle/lifecycle_ownership_ledger_stop_test.go`
- Modify: `cmd/issueops/mcpcli/mcp_issueops_helpers_test.go`
- Modify: `cmd/issueops/issueopscli/remotecmd/remote_test.go`

**Interfaces:**
- All mutation authority resolves through `CurrentOwnershipAttempt`.
- `force-release` closes the current attempt and pauses the cycle; it no longer writes `phase=done`.
- `force-done` and verified ownership completion atomically write `phase=done`, `cycle_state=closed`, and `active_attempt=0`.

- [ ] **Step 1: Write cross-package RED tests**

Prove paused cycles release ordinary source-cycle fencing but keep historical worker roots and Orca identities quarantined, historical attempts never authorize current resources, an active current attempt does fence, force-release preserves the last workflow phase, stale scan classifies paused cycles separately, and only verified completion closes the workflow. Add focused claim/acknowledge/heartbeat, publication, remote-create, operational-health, and MCP-draft cases that place conflicting identities in historical and current attempts and require the current attempt to win.

- [ ] **Step 2: Verify RED**

```bash
go test ./internal/core/issueops/... ./internal/core/lifecycle ./internal/adapter/operationalhealth ./cmd/issueops/issueopscli/remotecmd ./cmd/issueops/mcpcli -run 'CycleState|OwnershipLedger|Paused|ForceRelease|HistoricalAttempt' -count=1
```

- [ ] **Step 3: Replace direct authority reads**

Use ledger helpers in production paths. Historical attempts remain visible in status/doctor output but cannot authorize Git, Orca, cleanup, heartbeat, publication, remote creation, MCP draft behavior, or hook bypass. Before GREEN, run CodeGraph plus `rg` for direct `.ExecutionHandoff`/`.ExecutionWorkspace` production reads; only schema migration, invalid future-schema projection, and the ledger accessor itself may retain top-level-v8 compatibility reads.

- [ ] **Step 4: Verify GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit**

Suggested subject:

```text
refactor(issueops): separate cycle and owner authority
```

---

### Task 4: Add Non-Destructive Dirty-WIP Sealing

**Files:**
- Create: `internal/core/issueops/issueops_wip_seal.go`
- Create: `internal/core/issueops/issueops_wip_seal_test.go`
- Modify: `internal/core/guard/helpers.go`
- Modify: `internal/core/guard/guard_test.go`
- Modify: `internal/core/issueops_facade.go`

**Interfaces:**
- Produces `PreviewIssueOpsWIPSeal`, `CreateIssueOpsWIPSeal`, and `VerifyIssueOpsWIPSeal`.
- Exposes the existing guard secret-path predicate for reuse; do not create a second regex or looser restart-only rule.
- Uses `refs/issueops/issueops/<cycle-id>/attempts/<N>/wip` and an operation-scoped temporary index outside the repository worktree.

- [ ] **Step 1: Write Git fixture RED tests**

Cover clean worktree (no seal), modified/deleted/renamed tracked files, executable-bit changes, safe untracked files, ignored files excluded, spaces/newlines in paths, any staged/index entry, unmerged index, dirty submodule, nested repository, unsafe symlink, oversized candidate, guard-rejected path, pre-existing mismatched ref, and process failure after tree/commit creation but before `update-ref`.

- [ ] **Step 2: Verify RED**

```bash
go test ./internal/core/issueops -run 'WIPSeal' -count=1
```

- [ ] **Step 3: Implement plumbing without touching the real checkout state**

Use porcelain-v2 `-z` status for a canonical manifest and reject staged/index divergence before mutation. Create a temporary index, `read-tree HEAD`, add only validated non-ignored worktree paths, `write-tree`, `commit-tree -p HEAD`, then CAS-create the hidden ref. Before and after, compare `HEAD`, branch ref, real index checksum, status manifest SHA-256, and worktree file hashes; any drift leaves the predecessor attempt unchanged and reports recovery-required evidence.

- [ ] **Step 4: Verify recovery and byte preservation**

Restore a disposable checkout from the hidden ref and prove tracked/untracked content and modes match the pre-seal manifest. Prove the original checkout remains byte- and status-identical.

- [ ] **Step 5: Commit**

Suggested subject:

```text
feat(issueops): seal dirty owner wip safely
```

---

### Task 5: Implement Restart Preview, Seal Journal, and Successor Attempt Bridge

**Files:**
- Create: `internal/core/issueops/issueops_owner_restart.go`
- Create: `internal/core/issueops/issueops_owner_restart_test.go`
- Modify: `internal/core/issueops/issueops_handoff_prepare.go`
- Modify: `internal/core/issueops/issueops_handoff_dispatch.go`
- Modify: `internal/core/issueops/issueops_handoff_recovery.go`
- Modify: `internal/core/issueops/handoff/state.go`
- Modify: `internal/core/issueops_facade.go`

**Interfaces:**
- Produces `PreviewIssueOpsOwnerRestart` and `RestartIssueOpsOwner`.
- Preview returns cycle/attempt, Git HEAD/status digest, dirty flag, WIP-seal requirement, exact Orca worktree/terminal/task/dispatch inventory, and one inventory fingerprint.
- Confirm requires the fingerprint, authenticated source identity, `--confirm`, and `--seal-dirty-wip` when dirty.

- [ ] **Step 1: Write authority and inventory RED tests**

Accept only `cycle_state=paused`, no active attempt, a closed/cancelled last attempt, exact source actor/root, exact branch/worktree, equal-or-descendant HEAD, terminal old task/dispatch, and no connected/writable possible writer. Reject closed workflows, active attempts, divergent HEAD, multiple worktrees, live/ambiguous task or dispatch, connected terminal, fingerprint drift, and missing dirty-seal consent.

- [ ] **Step 2: Write seal-journal and bridge RED tests**

Cover crash before seal, after seal, and before successor-attempt CAS. A retry must reconcile the exact hidden ref, leave attempt 1 byte-identical, and append attempt 2 once. Prove the restart journal changes only through `intent → optional seal receipt → cleared`. Separately reuse Task 0's fault injection for terminal create, task create, dispatch, and dispatcher CAS; the restart bridge must not introduce, copy, or synchronize a second set of Orca stage receipts.

- [ ] **Step 3: Verify RED**

```bash
go test ./internal/core/issueops -run 'OwnerRestart' -count=1
```

- [ ] **Step 4: Implement preview and intent-first seal bridge**

Under lock, revalidate the fingerprint and persist `pending_restart` with the prior attempt snapshot. Outside lock, create/verify only the optional WIP seal. Under lock, require exact journal equality, append attempt `N+1` in the existing ownership-dispatching state with `RestartedFrom=N`, `InheritedWIPSeal=<seal>`, fresh epochs, and fresh context; set `active_attempt=N+1`, clear the restart journal, and set `cycle_state=active`. Do not change any field of attempts `1..N`.

- [ ] **Step 5: Reuse the existing staged Orca dispatcher**

Pass the new current attempt to the existing terminal-create → task-create → dispatch state machine. Its existing `pending_operation`, inventory baselines, reconcile behavior, and failure states remain the single external-mutation protocol. Add only the attempt-aware accessor/adapter changes proven necessary by Task 0; do not create restart-specific terminal/task/dispatch functions.

- [ ] **Step 6: Verify GREEN**

Run the focused command from Step 3. Expected: PASS.

- [ ] **Step 7: Commit**

Suggested subject:

```text
feat(issueops): restart released handoff owners
```

---

### Task 6: Expose Exact CLI, MCP, and Hook Authority

**Files:**
- Modify: `cmd/issueops/issueopscli/issueops_handoff_cli.go`
- Modify: `cmd/issueops/issueopscli/issueops_handoff_cli_test.go`
- Modify: `cmd/issueops/issueopscli/dependencies.go`
- Modify: `cmd/issueops/mcpcli/mcp_tool_issueops_handlers.go`
- Create: `cmd/issueops/mcpcli/mcp_issueops_owner_restart_test.go`
- Modify: `internal/adapter/mcp/issueops_lifecycle_catalog.go`
- Modify: `internal/adapter/mcp/issueops_catalog_test.go`
- Modify: `internal/core/commandparse/issueops.go`
- Modify: `internal/core/commandparse/issueops_test.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_authority.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_ownership_authority_test.go`

**Interfaces:**
- CLI: `issueops handoff restart-owner --id ID --coordinator-host HOST --coordinator-session-id SESSION --source-cwd PATH [--coordinator-agent-id ID] [--inventory-fingerprint SHA --seal-dirty-wip --confirm] --json`.
- MCP: existing `issueops_handoff` tool gains action `restart-owner` with the same DTO and result.

- [ ] **Step 1: Write CLI/MCP parity RED tests**

Pin preview and confirm fields, JSON responses, unknown/mixed flags, missing confirmation, dirty-seal consent, and exact error codes.

- [ ] **Step 2: Write lifecycle RED tests**

Allow only the typed restart command/tool from the authenticated exact source session. Continue blocking raw Orca terminal/task/dispatch mutation, raw messaging, source-side worker edits, historical owner sessions, and foreign MCP namespace suffix collisions.

- [ ] **Step 3: Implement shared DTO wiring**

CLI and MCP call the same facade/core operation and adapter seams. Do not add a second restart implementation in the hook or adapter.

- [ ] **Step 4: Verify**

```bash
go test ./cmd/issueops/issueopscli ./cmd/issueops/mcpcli ./internal/adapter/mcp ./internal/core/commandparse ./internal/core/lifecycle -run 'RestartOwner|SchemaV9|Ownership' -count=1
```

- [ ] **Step 5: Commit**

Suggested subject:

```text
feat(issueops): expose bounded owner restart
```

---

### Task 7: Align Documentation, Goldens, and Operator Recovery

**Files:**
- Modify: `.issueops/ADR.md`
- Modify: `.issueops/ARCHITECTURE.md`
- Modify: `.issueops/AGENT_WORKFLOW.md`
- Modify: `.issueops/OPERATIONS.md`
- Modify: `.issueops/TESTING.md`
- Modify: `.issueops/CAUTIONS.md`
- Modify: `skills/issueops/SKILL.md`
- Modify: `skills/issueops/references/orca-handoff.md`
- Modify: `skills/issueops/references/cleanup-state.md`
- Modify: `skills/issueops/references/worktree-context.md`
- Modify: `cmd/issueops/testdata/usage.golden.txt`
- Modify: `cmd/issueops/testdata/mcp_tools.golden.json`
- Modify: `cmd/issueops/testdata/response_contracts.golden.json`

- [ ] **Step 1: Document one state machine**

Document phase versus cycle state, attempt ledger authority, paused-cycle semantics, restart preview/confirm, dirty-WIP hidden-ref recovery, mixed-version rollout order, and the rule that cleanup never deletes a WIP ref.

- [ ] **Step 2: Add the two incident regressions**

Record that a choice label cannot replace the original goal or omit actor/location/ownership transfer, and that a full handoff must leave the source coordinator live for bounded observation. Link these rules to the typed restart operation rather than raw terminal steering.

- [ ] **Step 3: Refresh only owning goldens**

```bash
go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -update -count=1
```

Inspect every golden diff; schema version changes outside IssueOps responses are a failure.

- [ ] **Step 4: Validate the skill and docs**

```bash
python3 scripts/validate-skill.py skills/issueops
git diff --check
```

- [ ] **Step 5: Commit**

Suggested subject:

```text
docs(issueops): define restartable ownership lifecycle
```

---

### Task 8: Prove the #68-Shaped Recovery in Isolated State

**Files:**
- Create: `internal/core/issueops/issueops_owner_restart_acceptance_test.go`

- [ ] **Step 1: Run a disposable acceptance fixture**

Seed a copy of the current `#68` v8 record and a disposable worktree with committed descendants plus dirty tracked/untracked WIP. Migrate to paused v9, preview, seal, restart as attempt 2, claim/acknowledge/heartbeat, and verify attempt 1 bytes remain unchanged.

- [ ] **Step 2: Test restart-time process loss**

Replace the fake/fixture Orca runtime between preview and confirm and between each existing dispatcher stage. Prove exact reconciliation, no second owner, no duplicate task/dispatch, and no WIP loss.

- [ ] **Step 3: Verify historical quarantine and current authority**

Attempt 1's owner, terminal, task, dispatch, and worker-root mutations must remain denied after attempt 2 becomes active. Attempt 2 must be able to claim, acknowledge context, heartbeat, commit, publish, and complete through the unchanged owner-only commands.

- [ ] **Step 4: Run the acceptance test**

```bash
go test ./internal/core/issueops ./internal/core/lifecycle -run 'OwnerRestartAcceptance|HistoricalAttemptQuarantine' -count=1
```

`#51` cleanup is not an implementation-plan task. It is a separate live operation with its own fresh status/terminal/task/dispatch/dirty/remote readback and must stop if active ownership cannot be terminally proven.

---

### Task 9: Full Verification, Native Rollout, and Rollback Proof

- [ ] **Step 1: Inspect scope**

```bash
git status --short
git diff --stat
git diff --check
```

- [ ] **Step 2: Run focused cross-layer verification**

```bash
go test ./internal/core/issueops/... ./internal/core/lifecycle ./internal/adapter/operationalhealth ./internal/adapter/mcp ./cmd/issueops/issueopscli ./cmd/issueops/mcpcli -count=1
```

- [ ] **Step 3: Run all-or-nothing project verification**

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
go build -o /tmp/issueops-owner-restart ./cmd/issueops
python3 scripts/validate-skill.py skills/issueops
```

Any failure invalidates the whole run; rerun from the first command after fixing it.

- [ ] **Step 4: Prove mixed-version rejection before rollout**

Run the pre-change v8 binary against a copied v9 row and prove it returns unsupported-schema without changing the raw row digest. Do not run the old binary against the live database.

- [ ] **Step 5: Roll out all host surfaces in order**

Stop new handoff starts, install the schema-v9 binary, refresh Codex/Claude/GJC native hooks and MCP registrations, restart the daemon, and verify every live binary reports the same build and schema before migrating production records.

- [ ] **Step 6: Prove rollback boundaries**

Rollback before any v9 write may reinstall v8. After any v9 record is written, do not reinstall v8; rollback means reverting behavior while retaining a v9-compatible reader and ledger. Prove a v8 binary rejects a copied v9 row byte-identically.

- [ ] **Step 7: Migrate and restart live #68 only**

After every host surface is verified at v9, migrate only `io-7030a5b52063` with raw/canonical digest CAS. Run restart preview and confirm, then require attempt-2 claim, context acknowledgement, and current heartbeat before sending implementation continuation. Preserve every existing commit and dirty file; stop on any inventory or fingerprint drift.

- [ ] **Step 8: Final readback**

Verify main is clean, the live daemon/MCP use the new build, `#68` has exactly one active attempt 2 with a current heartbeat, attempt 1 is byte-identical to its migrated snapshot, and attempt 2 owns the inherited WIP seal. Handle `#51` only in the separately approved cleanup run.
