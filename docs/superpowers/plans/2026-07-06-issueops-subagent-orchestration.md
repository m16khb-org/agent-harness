# IssueOps Sub-Agent Orchestration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the delegation graph (parent/child IssueOps cycles), scoped session bindings, the lease-based work pool, AND the upstream-independence removals (D6: CodeGraph gate, Z.AI external LLM, llm-wiki promote, upstream install/update wiring, codex spawn) specified in `docs/superpowers/specs/2026-07-06-issueops-subagent-orchestration-design.md`, with fail-closed parent gates, hook surfacing, skill/doc integration, and an adversarial concurrency test battery.

**Architecture:** All coordination state lives in host-neutral Go core (`internal/core/issueops`, new `internal/core/workpool`); CLI and MCP are thin adapters over the same core functions. The harness never spawns agent processes — the main agent spawns sub-agents host-natively; the harness provides durable state, fail-closed gates, lease bookkeeping, and deterministic hook reminders. Single-entity locking only (no nested locks), persistent-inode flocks, read-repair for the parent→child index.

**Tech Stack:** Go standard library + existing `unix.Flock` lock helpers, existing state store (`internal/core/state`), existing CLI dispatch (`cmd/harness/issueopscli`, new `cmd/harness/workpoolcli`), existing MCP registry (`cmd/harness/mcpcli`), golden contract fixtures.

## Global Constraints

- Read the design spec first: `docs/superpowers/specs/2026-07-06-issueops-subagent-orchestration-design.md`. Where this plan and the spec disagree, the spec wins; update both on any deviation.
- For this plan's non-ownership orchestration fields, the July 6 baseline used additive `omitempty` under IssueOps schema v1. Issue #16 supersedes that root-version decision with schema v3 for `execution_handoff` and stable terminal identity.
- **Never hold two entity locks at once** (cycle/cycle, cycle/pool, pool/task) **and never call a `with*Lock`-wrapped function from inside another lock callback — including the SAME entity's lock** (a second exclusive flock on the same lock file via a new fd self-deadlocks in-process). Multi-entity ops are sequential single-locked steps + read-repair.
- **No new upstream dependencies anywhere** (D6): no feature may require CodeGraph, llm-wiki, claude-mem, an external LLM API, or a spawned agent CLI. External intelligence is always a render-prompt → record-result contract performed by the host agent.
- Lock files are persistent inodes — never delete between lock/unlock (see `.agent-harness/ISSUEOPS_AUDIT.md` "Lock File Deletion Breaks Mutual Exclusion").
- Every new readiness missing key maps to exactly one owner command surfaced in status output.
- Hooks observe/relay only; no workflow work, no heartbeats, no state mutation from hooks.
- Timestamps RFC3339Nano UTC; freeform text redacted with the existing secret-redaction path before persisting.
- Commits follow `.agent-harness/COMMIT_POLICY.md` (Conventional Commit subject + Lore body). One commit per task below; run the task's tests before each commit.
- After any contract-surface change, update `cmd/harness/testdata/mcp_tools.golden.json`, `response_contracts.golden.json`, usage goldens, and `cmd/harness/contractgolden` fixtures in the same task.

---

### Task 1: Delegation model types

**Files:**
- Modify: `internal/core/issueops/model/types.go`
- Test: `internal/core/issueops/model/types_delegation_test.go` (new)

**Interfaces:**
- Produces: `IssueOpsDelegationContract{ParentCycleID, TaskScope, AcceptanceCriteria, ParentPlanPath, ChildIssueURL, DelegatedAt}`, `IssueOpsChildCycleRef{CycleID, Branch, Title, ChildIssueURL, CreatedAt, ValidationVerdict, ValidationEvidence, ValidatedAt}`, and `IssueOpsRecord` fields `Delegation *IssueOpsDelegationContract` (`json:"delegation,omitempty"`), `ChildCycles []IssueOpsChildCycleRef` (`json:"child_cycles,omitempty"`).

- [ ] **Step 1:** Write failing test `TestIssueOpsRecordDelegationRoundTrip`: marshal/unmarshal a record with `Delegation` + two `ChildCycles`; assert field preservation. Add `TestIssueOpsRecordWithoutDelegationOmitsKeys`: a record without delegation marshals with NO `delegation`/`child_cycles` keys (omitempty round-trip, mirrors the phase-ledger omitempty test).
- [ ] **Step 2:** Run `go test ./internal/core/issueops/model -run TestIssueOpsRecordDelegation -count=1` — expect FAIL (types undefined).
- [ ] **Step 3:** Add the two structs and two record fields per the spec's D1 State shape.
- [ ] **Step 4:** Run the same command — expect PASS. Also run `go test ./internal/core/issueops/... -count=1` (no regressions).
- [ ] **Step 5:** Commit `feat(issueops): add delegation contract and child cycle ref model`.

### Task 2: Child start (fail-closed) + delegated artifact profile

**Files:**
- Create: `internal/core/issueops/delegation/delegation.go` (pure helpers: precondition evaluation, delegated-artifact construction)
- Create: `internal/core/issueops/issueops_delegation.go` (facade: `StartIssueOpsChild`, locking, parent index append)
- Modify: `internal/core/issueops/start/start.go` (`resumeOrReset` allow-list: preserve `Delegation`, `ChildCycles`)
- Test: `internal/core/issueops/issueops_delegation_test.go`, `internal/core/issueops/delegation/delegation_test.go`

**Interfaces:**
- Consumes: `StartIssueOps`, `withIssueOpsLock`, `ReadIssueOps`, `touchAndWriteIssueOps`, `branchprepare.ValidateBranch`, readiness helpers (`issueops_readiness.go`).
- Produces: `StartIssueOpsChild(stateRoot string, req model.IssueOpsChildStartRequest) (model.IssueOpsChildStartResult, error)` with `IssueOpsChildStartRequest{ParentID, Branch, Title, TaskScope, AcceptanceCriteria []string, ChildIssueURL}` and result carrying the child record + parent ref. Precondition missing keys: `parent_phase_not_implement`, `parent_design_review_unapproved`, `parent_compatibility_unapproved`, `parent_devils_advocate_missing`, `execution_decision_subagent_plan`, `delegation_depth_exceeded`, `child_branch_equals_parent`.

- [ ] **Step 1:** Write failing tests:
  - `TestStartIssueOpsChildFailClosedPreconditions` — table-driven: parent in `plan` phase; unapproved design review; unapproved/blocked compatibility review; missing DA review; no sub-agent plan with slug in {`task-fan-out-coordination`,`isolated-worktree-work`,`background-long-running-work`}; parent itself has `Delegation != nil` (depth); child branch == parent branch. Each returns the named missing key and creates NO child record.
  - `TestStartIssueOpsChildCreatesDelegatedProfile` — a fully-ready implement-phase parent: child record exists with `Delegation`, `Intent{IntentClass:"delegated-child", InterpretedIntent: task scope}` (`issueOpsIntentMissing` requires `interpreted_intent`), waived `PlanPrep` (reason `delegated:<parent-id>`), `scope` decision, delegated-reference `DesignReview`/`CompatibilityReview` with `Approved=true` AND populated `RefactorPlan`/`Alternatives`/`Risks`/`Verification` evidence (`issueOpsDesignReviewMissing` demands them when approved), `DevilsAdvocateReview.Waived==true`, `IssueURL` fallback to parent. Acceptance is **actual phase advancement**, not readiness output: `AdvanceIssueOpsPhase` succeeds on the child for `problem`→`grill`→`plan`; entering `compatibility-review` FAILS with the child's own missing `branch_prepare`/`worktree_path`/`plan_in_worktree` (`IssueOpsCompatibilityReviewReadiness` + `issueOpsBaseImplementationMissing`); after the test records branch-prepare, links a worktree, and links an in-worktree plan, `compatibility-review` entry succeeds; `implement` entry then still FAILS with missing `worktree_tools`/`execution_decision` (delegated artifacts open the analysis gates; setup and implement gates stay child-earned).
  - `TestStartIssueOpsChildPerConditionRemedy` — from each single blocked precondition, fix ONLY that field and assert `StartIssueOpsChild` now succeeds (per-condition remedy, not aggregate-only).
  - `TestStartIssueOpsChildLinkFailureReturnsWarning` — a failing/cross-project `--child-issue-url` (remote `ValidateChildMatchesParent` rejects) leaves the created child cycle and parent ref intact and returns `child_link_warning` (partial success, retryable via `issueops link-child`), not an error that loses the created state.
  - `TestStaleResetPreservesDelegationGraph` — (modify `start/start.go` `resumeOrReset` allow-list) stale-resetting a PARENT preserves `ChildCycles` incl. verdicts; stale-resetting a CHILD preserves `Delegation` (parent pointer) and the child stays `child_incomplete` on the parent.
  - `TestStartIssueOpsChildAppendsParentRef` — parent record gains one deduped `IssueOpsChildCycleRef`; `--child-issue-url` also appends the remote `IssueLinks` child entry via the existing `LinkIssueOpsChild` path.
- [ ] **Step 2:** Run `go test ./internal/core/issueops -run TestStartIssueOpsChild -count=1` — expect FAIL.
- [ ] **Step 3:** Implement: (1) parent read+validate under parent lock, release; (2) child create via the existing start path, then stamp delegation + delegated artifacts under child lock; (3) parent ref append under parent lock; (4) `--child-issue-url` remote link via `LinkIssueOpsChild` as a SEPARATE sequential call AFTER step 3's lock releases — `LinkIssueOpsChild` internally wraps `withIssueOpsLock(parentID)` (`package.go:406-414`) and a second exclusive flock on the same lock file from a new fd self-deadlocks in-process; a step-4 failure downgrades to `child_link_warning` (partial success). Strictly sequential locks, never nested — including same-entity re-entry. BaseBranch guidance: record the parent branch as the child's intended base in the delegation result guidance (BranchPrepare itself is recorded later by the child's own `branch prepare`).
- [ ] **Step 4:** Run Step 2 command — expect PASS.
- [ ] **Step 5:** Write and pass the concurrency tests: `TestStartIssueOpsChildConcurrentSameBranch` (2 goroutines, same child branch → exactly one child record + one parent ref, ref fields consistent with the winning request) and `TestStartIssueOpsChildConcurrentSiblings` (5 goroutines, distinct branches → 5 refs AND each ref's `CycleID`/`Branch`/`Title` matches its originating request — content integrity, not count-only; modeled on `TestIssueOpsConcurrentFeedbackNoLostUpdate`). Run `go test -race ./internal/core/issueops -run TestStartIssueOpsChild -count=1` — expect PASS.
- [ ] **Step 6:** Commit `feat(issueops): add fail-closed delegated child cycle start`.

### Task 3: Child status/list/accept/reject with read-repair

**Files:**
- Modify: `internal/core/issueops/issueops_delegation.go`
- Test: `internal/core/issueops/issueops_delegation_status_test.go`

**Interfaces:**
- Produces: `IssueOpsChildStatus(stateRoot, parentID string, repair bool) (model.IssueOpsChildStatusResult, error)` (aggregates per-child: cycle id, phase, heartbeat age, worktree, verdict; scans child records by `Delegation.ParentCycleID` to reconcile the index — child pointer wins; `repair=true` appends missing refs under the parent lock and marks orphaned refs), `AcceptIssueOpsChild(stateRoot, parentID, childID string, evidence []string)`, `RejectIssueOpsChild(stateRoot, parentID, childID, reason string, evidence []string)`, `DropIssueOpsChild(stateRoot, parentID, childID, reason string)` (reject/drop reasons ≥ 10 chars; verdicts `accepted|rejected|dropped` stored on the parent's ref only — spec verdict table).

- [ ] **Step 1:** Write failing tests: `TestIssueOpsChildStatusAggregatesAndRepairs` (index missing one scanned child → surfaced; repair appends it; orphaned ref flagged), `TestAcceptIssueOpsChildRequiresDonePhaseAndEvidence` (accept refused while child not `done`; refused with zero evidence; recorded verdict + timestamp on success), `TestRejectIssueOpsChildRecordsVerdictOnValidReason` (short reason refused; valid reason records `rejected` verdict + timestamp), `TestDropIssueOpsChildRecordsAuditTrail` (valid drop records `dropped` + reason; short reason refused).
- [ ] **Step 2:** Run `go test ./internal/core/issueops -run 'TestIssueOpsChildStatus|TestAcceptIssueOpsChild|TestRejectIssueOpsChild|TestDropIssueOpsChild' -count=1` — FAIL.
- [ ] **Step 3:** Implement; scanning uses one `ReadDir` of the issueops state root filtered to `io-*.json` with same-repo check before decode.
- [ ] **Step 4:** Re-run — PASS. Also `go test -race ./internal/core/issueops -run TestIssueOpsChild -count=1`.
- [ ] **Step 5:** Commit `feat(issueops): child status aggregation with read-repair and validation verdicts`.

### Task 4: Parent aggregation gate + regress guard (strict PR readiness + ledger keys)

**Files:**
- Modify: `internal/core/issueops/issueops_pr_readiness_strict.go`, `internal/core/issueops/issueops_phase_ledger.go` (artifact keys), `internal/core/issueops/issueops_regress.go` (children_active guard), `internal/core/issueops/issueops_force_release.go` + `issueops_force_done.go` (active-children audit)
- Test: `internal/core/issueops/issueops_phase_gates_test.go` (extend), `internal/core/issueops/issueops_delegation_gate_test.go` (new), `internal/core/issueops/issueops_regress_test.go` (extend), `internal/core/issueops/issueops_force_done_test.go` (extend)

**Interfaces:**
- Produces: strict PR readiness missing keys `child_incomplete:<cycle-id>`, `child_unvalidated:<cycle-id>`, `child_rejected_unresolved:<cycle-id>`; ledger artifact `children_complete` indexed in the **`pr` artifact set** of `issueOpsPhaseArtifactKeys` (NOT `implement` — `issueOpsImplementCompletion` delegates to `IssueOpsAISlopCleanReadiness` which has no child check, so an implement-set key would derive-complete falsely in `DeriveIssueOpsPhaseLedger`); `RegressIssueOpsForReplan` refusal reason `children_active`, evaluated AFTER the existing preconditions (phase plan..compatibility-review, DA stop reflected, cap); force-done/force-release on a parent with active children appends the active child ids to the force audit trail; `IssueOpsChildStatus` marks children whose parent is `done` as `parent_closed`.

- [ ] **Step 1:** Write failing tests (every blocked case has its clearing counterpart):
  - `TestIssueOpsStrictPRReadinessBlocksIncompleteChildren` — parent with one `implement`-phase child → `child_incomplete:<id>`; child forced `done` without verdict → `child_unvalidated:<id>`; after `AcceptIssueOpsChild` → keys clear.
  - `TestIssueOpsStrictPRReadinessRejectedAndDroppedVerdicts` — `rejected` verdict → `child_rejected_unresolved:<id>` stays blocking; `DropIssueOpsChild` → all child keys clear (unblock with audit trail).
  - `TestIssueOpsParentWithoutChildrenUnaffected` — no children → no new keys; existing gate fixtures unchanged.
  - `TestIssueOpsPhaseLedgerIndexesChildrenComplete`.
  - `TestRegressRefusedForImplementPhaseParentWithChildren` — a delegating (implement-phase) parent's regress is refused by the EXISTING phase precondition (`issueops_regress.go:44-47`), pinning that delegation does not open a new backward path.
  - `TestRegressIssueOpsForReplanBlockedByActiveChildren` — constructed via the reachable stale-reset corner: implement-phase parent with children → delete parent worktree → `StartIssueOps` stale-resets it to `problem` (ChildCycles preserved per Task 2) → re-drive to `plan` and record a reflected DA `stop` → regress refused with `children_active`; after accepting/dropping every child, the SAME regress call succeeds (blocked → remedy → unblocked).
  - `TestForceDoneParentRecordsActiveChildrenAudit` — force-done/force-release on a parent with a live child succeeds (human escape hatch) but the force audit lists the active child ids, and `IssueOpsChildStatus` afterwards marks those children `parent_closed`.
- [ ] **Step 2:** Run `go test ./internal/core/issueops -run 'TestIssueOpsStrictPRReadiness|TestIssueOpsParentWithout|TestIssueOpsPhaseLedgerIndexesChildren|TestRegressIssueOpsForReplanBlocked' -count=1` — FAIL.
- [ ] **Step 3:** Implement: gate evaluation reads child records (authoritative) via the Task 3 scan, not the index alone; **non-terminal = `(phase != done OR ForceReleasedAt != "") AND verdict != dropped`** — force paths stamp `Phase=done` (`issueops_force_release.go:38`), so the `ForceReleasedAt` marker is what keeps a force-released child incomplete until `dropped`.
- [ ] **Step 4:** Re-run — PASS; run `go test ./internal/core/issueops/... -count=1` for gate regressions (existing regress cap tests must stay green).
- [ ] **Step 5:** Commit `feat(issueops): fail-closed parent pr gate and regress guard over delegated children`.

### Task 5: Delegation CLI

**Files:**
- Modify: `cmd/harness/issueopscli/issueops_subcommands.go` (dispatch), `internal/adapter/cli/usage.go`
- Test: `cmd/harness/issueopscli/issueops_delegation_cli_test.go` (new)

**Interfaces:**
- Produces: `issueops child start --parent --branch --title --scope --acceptance (repeatable) [--child-issue-url] --json`, `issueops child status --parent [--repair] --json`, `issueops child list --parent --json`, `issueops child accept --parent --child --evidence (repeatable) --json`, `issueops child reject --parent --child --reason --json`, `issueops child drop --parent --child --reason --json`. `child start` output includes the delegation prompt guidance block (recommended base branch = parent branch, worktree naming, `export HARNESS_EXPECTED_WORKTREE=` placeholder, owner-command contract) rendered from core, not composed in the CLI.

- [ ] **Step 1:** Write failing lifecycle test `TestRunIssueOpsChildLifecycle`: start parent → drive to ready-implement state via existing helpers → `child start` → `child status` shows the child → `child accept` after forcing the child through to done in-test → JSON asserted at each step; plus flag-validation failures (missing `--parent`, short `--reason`). Add the CLI-path gate test `TestCLIIssueOpsPhaseAdvanceToPRBlockedByChildren`: drive the REAL `issueops phase --to pr --json` dispatch on a parent with an unfinished child → refusal names `child_incomplete:<id>`; accept the child → the same CLI call succeeds (fail-closed pinned through the command surface a real agent uses, not core-only).
- [ ] **Step 2:** Run `go test ./cmd/harness/issueopscli -run TestRunIssueOpsChildLifecycle -count=1` — FAIL (unknown subcommand).
- [ ] **Step 3:** Implement dispatch + usage text.
- [ ] **Step 4:** Re-run — PASS; run `go test ./cmd/harness/... -run Golden -count=1` and update usage/contract goldens in this task.
- [ ] **Step 5:** Commit `feat(issueops): child cycle CLI surface`.

### Task 6: Delegation MCP tools

**Files:**
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops.go`, `cmd/harness/mcpcli/mcp_tool_issueops_handlers.go`
- Test: `cmd/harness/mcpcli/mcp_issueops_delegation_test.go` (new)
- Update: `cmd/harness/testdata/mcp_tools.golden.json`, `response_contracts.golden.json`, `cmd/harness/contractgolden`

**Interfaces:**
- Produces: MCP tools `issueops_child_start`, `issueops_child_status`, `issueops_child_accept`, `issueops_child_reject`, `issueops_child_drop` — same JSON shapes as CLI (`MCP tool schema와 CLI JSON 출력은 호스트별로 다르게 만들지 않는다`).

- [ ] **Step 1:** Write failing MCP test `TestMCPIssueOpsChildLifecycle` mirroring Task 5's flow through tool calls.
- [ ] **Step 2:** Run `go test ./cmd/harness/mcpcli -run TestMCPIssueOpsChildLifecycle -count=1` — FAIL.
- [ ] **Step 3:** Register tools (descriptions state purpose/when-to-use/writes/args/result per MCP tool design guidance) + handlers calling the Task 2/3 core functions.
- [ ] **Step 4:** Re-run + `go test ./cmd/harness/... -run Golden -count=1` (update goldens) — PASS.
- [ ] **Step 5:** Commit `feat(mcp): issueops child delegation tools`.

### Task 7: Scoped session bindings

**Files:**
- Modify: `internal/core/issueops/session/session.go`, `internal/core/issueops/package.go` (`LinkIssueOpsWorktree`, unbind paths, resume)
- Test: `internal/core/issueops/session/session_scoped_test.go` (new), extend `issueops_session_binding_wiring_test.go`

**Interfaces:**
- Produces: `BindScoped(store, repo, cycleID, branch, worktree)` / `ReadScoped(store, repo, cycleID)` / `UnbindScopedForCycle(store, repo, cycleID)` writing `issueops-session-<repoHash>-<cycleID>.json`; `ListBindings(store, repo)` (primary + scoped). All mutations run under the EXISTING per-repo session flock (one lock for all binding files of a repo). `LinkIssueOpsWorktree` routes: cycle with `Delegation != nil` → scoped binding (primary untouched); else legacy primary. Done/force paths unbind the matching scope.

- [ ] **Step 1:** Write failing tests: `TestScopedBindingDoesNotClobberPrimary` (parent binds primary; child worktree link writes scoped; primary unchanged; both readable), `TestUnbindScopedForCycleCompareAndDelete`, `TestScopedBindingConcurrentBindUnbind` (parent + 3 children bind/unbind concurrently → surviving bindings intact; `-race`).
- [ ] **Step 2:** Run `go test ./internal/core/issueops/session -count=1` — FAIL.
- [ ] **Step 3:** Implement (scoped path derives from the existing `bindingKey(repo)` + `-` + cycleID; cycleID already matches `^io-[0-9a-f]{12}$`, validate before path join).
- [ ] **Step 4:** Re-run with `-race` — PASS; run `go test ./internal/core/issueops/... -count=1`.
- [ ] **Step 5:** Commit `feat(issueops): per-cycle scoped session bindings`.

### Task 8: resume --id, bind --id, heartbeat --id + guard chain

**Files:**
- Modify: `internal/core/issueops/package.go` (`IssueOpsResume` id-path), `internal/core/lifecycle/lifecycle_worktree_guard.go` + `lifecycle_worktree_mcp.go` (scoped-binding fallback), `cmd/harness/issueopscli/issueops_subcommands.go` (flags + `issueops heartbeat --id` exposing `RecordIssueOpsHeartbeat`), MCP handler for `issueops_resume` (new `id` arg) + new `issueops_heartbeat` tool
- Test: extend `cmd/harness/issueopscli` lifecycle tests + `internal/core/lifecycle` guard tests

**Interfaces:**
- Produces: `issueops resume --id <cycle> [--bind] --json` (returns existing `IssueOpsResumeResult` + `HARNESS_EXPECTED_WORKTREE` guidance for that cycle; `--bind` writes scoped for delegated cycles, primary otherwise); `issueops heartbeat --id --json`; lifecycle MCP guard resolution order env → branch-matched scoped binding → primary binding → active cycles (hookcli PreToolUse stays env/flag-only — do NOT change `resolveExpectedWorktree`).

- [ ] **Step 1:** Write failing tests: `TestIssueOpsResumeByID` (bound + unbound; delegated child resume returns child worktree guidance), `TestIssueOpsHeartbeatCLIUpdatesLastHeartbeat`, `TestLifecycleGuardPrefersScopedBindingOnBranchMatch` (env unset, session on child branch → child worktree expected; on parent branch → parent worktree), `TestLifecycleGuardEnvBeatsScopedBinding` (env set to a DIFFERENT worktree than the scoped binding resolves → env wins; precedence order pinned), `TestForceReleasedChildStillIncompleteThenResumable` (force-release a child mid-`implement` → parent gate still reports `child_incomplete:<id>` → `issueops resume --id <child> --bind` restores context and the child can be driven to `done` → accept clears the gate).
- [ ] **Step 2:** Run targeted tests — FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Re-run + `go test ./internal/core/lifecycle ./internal/core/issueops/... ./cmd/harness/issueopscli -count=1` — PASS. Update MCP/usage goldens (resume arg, heartbeat tool).
- [ ] **Step 5:** Commit `feat(issueops): resume/bind/heartbeat by cycle id with scoped guard fallback`.

### Task 9: workpool core — types, store, locks, create/add-task

**Files:**
- Create: `internal/core/workpool/types.go`, `store.go`, `lock_unix.go`, `lock_other.go`, `workpool.go`
- Test: `internal/core/workpool/workpool_test.go`, `store_test.go`

**Interfaces:**
- Produces: `WorkPool`/`WorkTask` structs exactly per spec D3; `CreatePool(req)` (Size clamp 1..16 default 4, LeaseTTL parseable duration default 15m, MaxAttempts default 2; optional ParentCycleID validated with the SAME preconditions as child start incl. slug `task-fan-out-coordination`); `AddTask(poolID, req)` (redact instructions; cap 4096 tasks per pool → `pool_task_cap`); `ReadPool`, `ReadTask`, `ListTasks(poolID)`. Layout `<state>/workpool/<pool-id>.json` + `<pool-id>/<task-id>.json`; per-pool and per-task persistent-inode flocks copying `issueops_lock_unix.go` verbatim discipline; `WorkPoolCurrentSchemaVersion=1` fail-closed on future versions.

- [ ] **Step 1:** Write failing tests: round-trip + omitempty; table-driven `TestCreatePoolSizeClamp` with exact rows {0→4(default), 1→1, 16→16, 17→refused `pool_size_out_of_range`, -1→refused}; table-driven `TestCreatePoolLeaseTTLEdgeCases` with rows {""→15m default, "0s"→refused `lease_ttl_invalid`, "-5m"→refused, "15m"→accepted}; task cap (4097th add-task → `pool_task_cap`); future-schema read refusal; parent-precondition refusal (reuse a helper parent fixture from issueops tests via exported test helper or duplicated fixture builder).
- [ ] **Step 2:** `go test ./internal/core/workpool -count=1` — FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Re-run — PASS.
- [ ] **Step 5:** Commit `feat(workpool): durable pool and task store with fail-closed schema`.

### Task 10: workpool lifecycle — claim/reap/heartbeat/submit/accept/reject/close + race battery + bench

**Files:**
- Create: `internal/core/workpool/lifecycle.go`
- Test: `internal/core/workpool/lifecycle_test.go`, `lifecycle_race_test.go`, `lifecycle_bench_test.go`

**Interfaces:**
- Produces: `Claim(poolID, workerID)` (pool-locked: inline reap → count leased → `pool_saturated` refusal at Size → oldest-pending lease + rendered delegation prompt), `Heartbeat(poolID, taskID, workerID)` (task-locked; worker/lease fencing), `Submit(poolID, taskID, workerID, evidence, branch, worktree)` (≥1 evidence), `Accept(poolID, taskID, evidence)` / `Reject(poolID, taskID, reason, requeue)` (reason ≥10 chars; attempts++/dropped at cap), `Reap(poolID)` (idempotent), `Close(poolID, force, reason)` + `draining` status handling.

- [ ] **Step 1:** Write failing lifecycle tests covering every transition + refusal named in the spec's task lifecycle, including: fencing `TestHeartbeatAfterReapRefusesStaleWorker`, `TestSubmitAfterLeaseExpiryRefused`, and `TestSubmitRefusedAfterTaskReclaimedByAnotherWorker` (A claims → lease expires → reap → B reclaims → A's submit AND heartbeat refused by worker-id fencing, B's submit succeeds); expiry boundary `TestReapExactBoundaryIsExpired` (`now == LeaseExpiresAt` → reaped) and `TestReapOneTickBeforeExpiryNotReaped` (injected clock, spec boundary `now >= LeaseExpiresAt`); `TestDrainingRefusesClaimAllowsSubmit`; `TestClosedPoolRefusesAllMutations` (claim/heartbeat/submit/accept/reject each refused on a closed pool); `TestCloseEmptyPoolSucceedsTrivially` (zero-task pool).
- [ ] **Step 2:** `go test ./internal/core/workpool -run 'TestClaim|TestHeartbeat|TestSubmit|TestAccept|TestReject|TestReap|TestDraining|TestClose' -count=1` — FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Re-run — PASS.
- [ ] **Step 5:** Write and pass the race battery (all `-race`, content-integrity assertions per the spec — never count-only): `TestClaimConcurrentRespectsPoolSize` (10 goroutines, Size=3 → exactly 3 leases, 7 `pool_saturated`, AND each leased task's `WorkerID` equals exactly one claimant with a valid `LeaseExpiresAt`), `TestClaimVsReapNoDoubleLease`, `TestConcurrentSubmitAcceptNoLostUpdate` (50-goroutine lost-update pattern; every surviving task's `Evidence`/`WorkerID`/status internally consistent with its true origin).
- [ ] **Step 6:** Add the deterministic performance guard `TestClaimAt1000TasksCompletesWithinBudget` — seed a pool with 1000 tasks, one `Claim` call must complete within a generous fixed budget (e.g. 2s wall-clock via `time.Since`; fails on pathological regressions, tolerant of CI noise) — this runs under plain `go test`, unlike benchmarks. Also add informational `BenchmarkClaimAt1000Tasks` and record its result in the commit body.
- [ ] **Step 7:** `go test -race ./internal/core/workpool -count=1` — PASS.
- [ ] **Step 8:** Commit `feat(workpool): lease-based claim lifecycle with load control and fencing`.

### Task 11: pool parent gate + issueops linkage

**Files:**
- Modify: `internal/core/issueops/issueops_pr_readiness_strict.go` (+ small accessor in `internal/core/workpool` consumed via `internal/core` facade to avoid an import cycle — check direction: issueops must not import workpool if workpool imports issueops fixtures; if a cycle threatens, put the linkage check in the `internal/core` facade layer like `issueops_benchmark_facade.go` does)
- Test: `internal/core/issueops/issueops_pool_gate_test.go` (or facade-level test)

**Interfaces:**
- Produces: strict PR readiness missing key `pool_incomplete:<pool-id>` while any pool with `ParentCycleID == <parent>` has non-terminal tasks or status != closed; owner command `workpool status`.

- [ ] **Step 1:** Write failing tests `TestIssueOpsStrictPRReadinessBlocksOpenPool` (+ vacuous no-pool case) and `TestIssueOpsStrictPRReadinessClearsAfterPoolClose` (blocked while the linked pool has non-terminal tasks → all tasks terminal + `Close` → `pool_incomplete:<id>` clears; blocked→remedy→unblocked pair).
- [ ] **Step 2:** Run it — FAIL.
- [ ] **Step 3:** Implement (pool lookup by scanning `<state>/workpool/*.json` manifests for ParentCycleID; manifest-only reads).
- [ ] **Step 4:** Re-run + `go test ./internal/core/... -count=1` — PASS.
- [ ] **Step 5:** Commit `feat(issueops): parent pr gate over linked work pools`.

### Task 12: workpool CLI + MCP

**Files:**
- Create: `cmd/harness/workpoolcli/workpool.go` (+ dispatch registration in the top-level command catalog, `internal/adapter/cli/usage.go`)
- Modify: `cmd/harness/mcpcli` (tool registry + handlers), `internal/adapter/mcp/adapter_owned_catalog.go` if workpool tools are adapter-owned like worker_*
- Test: `cmd/harness/workpoolcli/workpool_cli_test.go`, `cmd/harness/mcpcli/mcp_workpool_test.go`
- Update: all contract goldens

**Interfaces:**
- Produces: CLI `workpool create|add-task|claim|heartbeat|submit|accept|reject|reap|status|close ... --json`; MCP `workpool_create`, `workpool_add_task`, `workpool_claim`, `workpool_heartbeat`, `workpool_submit`, `workpool_accept`, `workpool_reject`, `workpool_status`, `workpool_reap`, `workpool_close`. Identical JSON via both surfaces.

- [ ] **Step 1:** Write failing CLI lifecycle test `TestRunWorkpoolLifecycle` (create → add 3 tasks → claim → heartbeat → submit → accept → status → close) and MCP mirror `TestMCPWorkpoolLifecycle`. Add the CLI-path gate test `TestCLIIssueOpsPhaseAdvanceToPRBlockedByOpenPool`: real `issueops phase --to pr --json` dispatch on a parent with a linked open pool → refusal names `pool_incomplete:<id>`; close the pool → the same CLI call succeeds.
- [ ] **Step 2:** Run both — FAIL.
- [ ] **Step 3:** Implement thin adapters over Task 9/10 core.
- [ ] **Step 4:** Re-run + `go test ./cmd/harness/... -run Golden -count=1` (update goldens) — PASS.
- [ ] **Step 5:** Commit `feat(workpool): CLI and MCP surface`.

### Task 13: hook surfacing (UserPromptSubmit hint + Stop relay)

**Files:**
- Modify: `internal/core/hookprompt/worktree_reminder.go`-adjacent (new `orchestration_reminder.go`), `internal/core/hookprompt/render.go`, `internal/core/hookprompt/catalog.go`, Stop relay composition in `cmd/harness/hookcli/hook_stop.go` path (core-side fact provider)
- Test: `internal/core/hookprompt/orchestration_reminder_test.go`, extend hook stop tests

**Interfaces:**
- Produces: `orchestrationReminderValue(repo) string` — one line for children (`children: <done>/<total> done, <n> unvalidated - issueops child status --parent <id>`) and one for pools (`pool <name>: <leased> leased / <pending> pending / <expired> expired - workpool status`), emitted only when the repo has a bound cycle with children or a linked active pool; child display capped at 16, manifest+dir-count reads only. Stop relay includes the deterministic missing keys `child_incomplete`/`child_unvalidated`/`pool_incomplete` when present on the bound cycle.

- [ ] **Step 1:** Write failing tests: reminder renders for a fixture repo with children/pool; absent otherwise; cost guard test asserting no per-task JSON decode on the hint path (e.g. tasks dir with an intentionally corrupt task file still renders counts); stop relay names the keys.
- [ ] **Step 2:** Run `go test ./internal/core/hookprompt ./cmd/harness/hookcli -count=1` — FAIL.
- [ ] **Step 3:** Implement (read-only; no workflow work).
- [ ] **Step 4:** Re-run — PASS.
- [ ] **Step 5:** Commit `feat(hookprompt): orchestration reminders and stop relay facts`.

### Task 14: skills + references + contract tests

**Files:**
- Modify: `skills/issueops/SKILL.md`
- Create: `skills/issueops/references/orchestration.md`
- Modify: `internal/core/issueops/issueops_skill_contract_test.go`

- [ ] **Step 1:** Extend the skill contract test first (RED): new assertions that SKILL.md documents `issueops child start/status/accept/reject/drop`, `workpool claim/submit/accept`, the missing-key→owner-command rows (`child_incomplete` → `issueops child status`, `child_unvalidated` → `issueops child accept`, `child_rejected_unresolved` → `issueops child accept|drop`, `pool_incomplete` → `workpool status`, `children_active` → `issueops child status`), the delegation preconditions (implement phase + approved reviews + recorded sub-agent plan), the verdict table (accepted/rejected/dropped semantics), and that `references/orchestration.md` contains the delegation prompt template sections (child contract, pool worker loop, scope-drift stop rule, validation rubric).
- [ ] **Step 2:** `go test ./internal/core/issueops -run TestIssueOpsSkill -count=1` — FAIL.
- [ ] **Step 3:** Write the SKILL.md sections (Delegated Child Cycles; Worker Pool; extend the Gate Quick Reference and Concept→Command map) and `references/orchestration.md` (S1/S2 walkthroughs from the spec; prompt templates; per-loop heartbeat instruction `issueops heartbeat --id` / `workpool heartbeat`).
- [ ] **Step 4:** Re-run — PASS.
- [ ] **Step 5:** Commit `docs(skills): issueops orchestration sections and reference`.

### Task 15: project docs + ADR + CAUTIONS

**Files:**
- Modify: `.agent-harness/ARCHITECTURE.md` (state model + new namespace + actor model), `.agent-harness/AGENT_WORKFLOW.md` (resume/heartbeat/pool worker contract), `.agent-harness/SUB_AGENT_PATTERNS.md` (D1→#2/#7, D3→#7/#8 application notes), `.agent-harness/ADR.md` (decision + rejected alternatives per spec), `.agent-harness/CAUTIONS.md` (single-entity lock invariant INCLUDING same-entity `with*Lock` re-entry self-deadlock; mixed-binary additive-field caution; lease-TTL vs heartbeat contract)

- [ ] **Step 1:** Update each doc per the spec's D5 list (use MCP `project_docs_read`/`project_docs_update`/`project_docs_record` flow per AGENT_WORKFLOW).
- [ ] **Step 2:** Run `go test ./internal/core/issueops -run TestIssueOpsSkill -count=1` and `./bin/agent-harness docs --json` sanity — PASS/exit 0.
- [ ] **Step 3:** Commit `docs(agent-harness): orchestration architecture, workflow, ADR, cautions`.

### Task 17 (Upstream Independence U1 — recommended FIRST, before Task 2): remove the CodeGraph hard gate

**Files:**
- Modify: `internal/core/issueops/issueops_readiness.go` (drop `codegraph_ready` from `issueOpsWorktreeToolsMissing`, `issueops_readiness.go:193-195`), `cmd/harness/issueopscli/worktreetools` (prepare-tools skips CodeGraph silently when the CLI/`.codegraph/` is absent; fields stay informational)
- Test: extend `internal/core/issueops/issueops_readiness_test.go`, `cmd/harness/issueopscli/issueops_worktree_tools_test.go`

- [ ] **Step 1 (RED):** `TestImplementGateDoesNotRequireCodeGraph` — a record with `WorktreeTools{CodeGraphChecked:false, CodeGraphReady:false}` but deps ready + worktree match has NO `codegraph_ready` missing key and `AdvanceIssueOpsPhase` to implement succeeds (with all other artifacts present); `TestPrepareToolsWithoutCodeGraphSucceeds` — prepare-tools on a machine without codegraph reports `OK=true`.
- [ ] **Step 2:** Run both — FAIL. **Step 3:** Implement. **Step 4:** Re-run + `go test ./internal/core/issueops/... ./cmd/harness/issueopscli -count=1` — PASS (fix fixtures that asserted `codegraph_ready`). Update skill/docs mentions of CodeGraph readiness (skills/issueops references, `.agent-harness/ARCHITECTURE.md` implement gate description).
- [ ] **Step 5:** Commit `feat(issueops): make CodeGraph optional evidence, not an implement gate`.

### Task 18 (U2): remove the Z.AI external LLM client; host-agent judgement contract

**Files:**
- Delete: `internal/core/externalllm/` (Z.AI HTTP client, `api.z.ai`, `$Z_AI_API_KEY` — `print.go:15-21`)
- Modify every consumer to a render-prompt → accept-result-file contract (the existing judge-file input path is the template, cf. `issueops_judge_file_cli_test.go`): `internal/core/draftwiki` (suggest + queue-process emit the prompt; a new `draft-wiki submit --draft <file>` records the host-agent-authored draft), `internal/core/issueops/remote` (remote score judge takes `--judge-file`), `internal/core/issueops/benchmark` (judges/self-consistency via judge files only), `internal/core/lintgate`, `internal/core/qualitycatalog`, `internal/core/external_llm_usage.go` + `utility_facade.go`
- Test: per-consumer contract tests; a repo-wide negative check

- [ ] **Step 1 (RED):** per consumer, a test that the render path emits the full prompt+schema and the record path persists a supplied result WITHOUT any network call (`httptest`-style guards removed with the client); plus `TestNoExternalLLMEndpointReferences` — grep-equivalent test asserting no `api.z.ai`/`Z_AI_API_KEY` string remains in non-test sources.
- [ ] **Step 2-4:** RED → implement → `Z_AI_API_KEY= go test ./... -count=1` green. Update usage/MCP goldens for changed flags.
- [ ] **Step 5:** Commit `feat(core)!: remove external LLM service dependency; host-agent judgement contract`.

### Task 19 (U3): draft-wiki promote exports locally (no llm-wiki)

**Files:**
- Delete: `internal/core/draftwiki/llmpromote/`
- Modify: `internal/core/draftwiki/draft_wiki_promote.go` (promote moves approved drafts to `.agent-harness/draft-wiki/exported/` with a log append; no hub/registry resolution)
- Test: `internal/core/draftwiki/draft_wiki_promote_test.go` (rewrite)

- [ ] **Step 1 (RED):** `TestPromoteExportsApprovedDraftLocally` (+ refusal for unapproved). **Step 2-4:** implement → green. **Step 5:** Commit `feat(draftwiki): local export promote, drop llm-wiki hub dependency`.

### Task 20 (U4): remove upstream tool wiring from install/update

**Files:**
- Modify: `scripts/install-native.sh` (delete `install_upstream_tools` and its invocation, `:381-484`; `--with-upstream-tools`/`--skip-upstream-tools`/`HARNESS_INSTALL_UPSTREAM_TOOLS` become deprecated no-ops printing a warning), `cmd/harness/updatecli/update_bootstrap.go:23-54` (drop the `--with-upstream-tools` default-true wiring; same deprecated no-op treatment)
- Test: `cmd/harness/updatecli/update_bootstrap_test.go` + `update_bootstrap_edges_test.go` (update: no upstream args ever passed; deprecated flags warn and no-op)

- [ ] **Step 1 (RED):** `TestUpdateNeverPassesUpstreamFlags`, `TestDeprecatedUpstreamFlagsWarnAndNoop`; shell check `bash -n scripts/install-native.sh`. **Step 2-4:** implement → green; `./scripts/install-native.sh --dry-run` output contains no upstream installer lines. **Step 5:** Commit `feat(install)!: stop wiring upstream companion tools`.

### Task 21 (U5): api-doc review without spawning codex

**Files:**
- Modify: `cmd/harness/apidoc/api_doc_review_runner.go` (replace `exec.Command("codex", ...)` `:24` with prompt+schema render output and a `--result <file>` record path), CLI/MCP descriptors for `api-doc review`/`api_doc_review`
- Test: `cmd/harness/apidoc` tests (render includes OPEN_API_SPEC prompt source; result file recorded as review evidence; no exec)

- [ ] **Step 1 (RED):** `TestAPIDocReviewRendersPromptWithoutSpawning` + `TestAPIDocReviewRecordsSuppliedResult`. **Step 2-4:** implement → green; goldens updated. **Step 5:** Commit `feat(apidoc): host-agent review contract, no codex spawn`.

### Task 22: independence policy docs + ADR

**Files:**
- Modify: `.agent-harness/ARCHITECTURE.md` (delete/replace the "바퀴를 재발명하지 않는 companion tool 정책" and LLM Wiki policy sections with the standalone policy), root `AGENTS.md` invariants + `CLAUDE.md` philosophy line, `.agent-harness/ADR.md` (record the reversal: decision, rationale — independence/reproducibility/no external keys on core paths — and rejected alternative: opt-in upstream wiring), `.agent-harness/CAUTIONS.md` (never reintroduce a hard external dependency into a readiness gate), `.agent-harness/OPERATIONS.md` (install/update surface changes)

- [ ] **Step 1:** Update each doc; ADR entry per COMMIT_POLICY/ADR convention. **Step 2:** `go test ./internal/core/issueops -run TestIssueOpsSkill -count=1` + docs index sanity. **Step 3:** Commit `docs(agent-harness)!: standalone policy — upstream dependencies removed (ADR)`.

### Task 16 (runs LAST, after Tasks 1-15 and 17-22): full verification + dogfood

- [ ] **Step 1:** Run the full battery:

```bash
go mod tidy
go test ./internal/core/issueops/... -count=1
go test ./internal/core/workpool/... -count=1
go test -race ./internal/core/issueops/... ./internal/core/workpool/... -count=1
go test ./cmd/harness/... -count=1
go test ./cmd/harness/... -run Golden -count=1
go test ./... -count=1
go test -race ./... -count=1
go build -o bin/agent-harness ./cmd/harness
```

Expected: all exit 0.

- [ ] **Step 2:** Dogfood **B1 (S1)** in a scratch repo: umbrella cycle to implement-ready → `child start` ×2 → resume each child by `--id` in separate shells with separate `HARNESS_EXPECTED_WORKTREE` → drive one child to done → `child accept` → parent `pr-readiness --strict --json` shows the remaining `child_incomplete` key → finish + accept second child → key clears. Record transcript evidence.
- [ ] **Step 3:** Dogfood **B2 (S2)**: pool Size=2, 5 tasks; 3rd concurrent claim refused `pool_saturated`; abandon one lease → `reap` requeues; submit/accept/reject paths; `close` → parent `pool_incomplete` clears.
- [ ] **Step 4:** Install/update surfaces: `agent-harness update --path-mode=skip --json`, `./bin/agent-harness daemon status --json`, `claude mcp list`, `codex mcp get agent_harness` — all healthy, and the update output shows NO upstream tool wiring.
- [ ] **Step 5:** Independence verification (D6): `grep -rn "api.z.ai" internal cmd` → no matches; `grep -rn codegraph_ready internal/core/issueops/issueops_readiness.go` → no matches; `grep -n install_upstream_tools scripts/install-native.sh` → no matches; `Z_AI_API_KEY= go test ./... -count=1` → green. Run the benchmark package (`go test ./internal/core/issueops/benchmark/... -count=1`) and update its fixtures if readiness-key changes shifted expectations.
- [ ] **Step 6:** Update `.agent-harness/ISSUEOPS_AUDIT.md` with a dated reconciliation entry for the new concurrency surfaces and their test evidence.
- [ ] **Step 7:** Commit `test(orchestration): full verification battery and dogfood evidence`.

---

## Self-Review Notes

- Spec coverage: D1 → Tasks 1-6; D2 → Tasks 7-8; D3 → Tasks 9-12; D4 → Task 13; D5 → Tasks 14-15; D6 → Tasks 17-22; concurrency battery → Tasks 2/7/10/16; performance → Task 10 deterministic budget test + bench + Task 13 cost guard. Execution order: Task 17 first (simplifies worktree_tools fixtures), Tasks 1-15 next, Tasks 18-22 as a parallel track, Task 16 last.
- Verifier-pass hardening applied (2026-07-06 adversarial review, verdict revise → resolved): CLI-path gate tests (Tasks 5/12), `AdvanceIssueOpsPhase`-level profile acceptance (Task 2), regress `children_active` guard (Task 4 + spec), verdict model accepted/rejected/dropped with clearing counterparts (Tasks 3/4), content-integrity race assertions (Tasks 2/10), deterministic claim budget test (Task 10), lease boundary/reclaim fencing/closed-pool/empty-pool/env-precedence/per-condition-remedy/force-release-resume tests (Tasks 2/8/10).
- Critic-pass corrections applied (2026-07-07 adversarial review, verdict revise → resolved, all findings verified against source): delegated child gate walk corrected — compatibility-review entry requires child-earned branch/worktree/plan (Task 2, was BLOCKER 1); regress guard reachability fixed via the stale-reset corner + implement-phase refusal pinned (Task 4, was BLOCKER 2); `resumeOrReset` preserves `Delegation`/`ChildCycles` (Task 2, was MAJOR 3); force-released-child predicate uses `ForceReleasedAt` (Task 4, was MAJOR 4); `children_complete` indexed in the `pr` artifact set (Task 4, was MAJOR 5); `LinkIssueOpsChild` same-lock re-entry hazard sequenced + Global Constraint (Task 2, was MINOR 6); delegated Intent includes `InterpretedIntent` (Task 2, was MINOR 7); parent force paths audited with `parent_closed` surfacing (Task 4, was MINOR 8); child-link partial success + cross-project limitation (Task 2); benchmark fixtures check (Task 16).
- Type/name consistency: `IssueOpsDelegationContract`/`IssueOpsChildCycleRef` (Tasks 1-6), `StartIssueOpsChild`/`IssueOpsChildStatus`/`AcceptIssueOpsChild`/`RejectIssueOpsChild`/`DropIssueOpsChild` (Tasks 2-6), workpool `Claim/Heartbeat/Submit/Accept/Reject/Reap/Close` (Tasks 10-12) — verified consistent across tasks.
- Known judgement points left to the implementer (intentional, with guardrails stated): exact placement of the pool↔issueops linkage to avoid an import cycle (Task 11 names the facade fallback); whether workpool MCP descriptors are adapter-owned like worker_* (Task 12 points at both registries).
