# Supervised Handoff 다중 Coordinator Self-Heal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 하나 또는 여러 coordinator가 독립 IssueOps record/worktree에서 안전하게 handoff를 시작하고, pre-dispatch terminal 불일치는 제한적으로 self-heal한다.

**Architecture:** 권한과 mutation lease는 `IssueOpsRecord.ID` 및 `ExecutionHandoff.Orca.WorktreeID`에만 결속한다. source recipient는 routing-only이며 source worktree에서 connected+writable candidate가 정확히 하나일 때만 자동 선택한다. worker worktree terminal은 정확히 하나의 clean baseline만 adopt하고, 실행 흔적이 없는 불일치만 idempotent reconcile한다.

**Tech Stack:** Go 1.26, IssueOps durable JSON state, lifecycle hook guard, Orca dispatch port, Go `testing`/race detector.

## Global Constraints

- 전역 coordinator registry, process-wide lock, 다른 record/worktree terminal의 정리는 도입하지 않는다.
- coordinator identity는 authenticated hook event/CLI flag에서만 받고 추측하지 않는다.
- recipient handle은 routing-only이며 active record 간 중복 seal을 금지한다.
- task, dispatch, worker session, result, pending external mutation 중 하나라도 있으면 자동 cleanup하지 않는다.
- pre-dispatch reconcile은 receipt와 recovery reason을 durable record에 남기고 동일 상태 재실행은 no-op이어야 한다.
- source candidate가 0개 또는 2개 이상이면 선택하지 않고 concrete handle을 요구한다.
- 모든 Go 변경은 focused test → package test → race test → vet/build 순으로 검증한다.

---

## File Map

- Modify: `internal/core/lifecycle/lifecycle_handoff_guard.go` — late coordinator guidance가 current authenticated identity와 record ID를 사용하도록 한다.
- Modify: `internal/core/lifecycle/lifecycle_handoff_authority.go` — identity-filled start command와 record-scoped exact-command authority를 유지한다.
- Modify: `internal/core/lifecycle/lifecycle_handoff_coordinator_dispatch_test.go` — late context와 record isolation regression을 고정한다.
- Modify: `internal/core/issueops/issueops_handoff_dispatch.go` — source recipient candidate resolution, active-record collision rejection, baseline adopt/create, pre-dispatch reconcile을 구현한다.
- Modify: `internal/core/issueops/issueops_handoff_dispatch_test.go` — terminal inventory, same-record race, partial-state false case를 고정한다.
- Modify: `internal/core/issueops/issueops_handoff_prepare.go` — 동일 branch의 기존 Orca-managed Git worktree를 create collision으로 버리지 않고, exact identity를 검증한 뒤 IssueOps handoff로 adopt한다.
- Modify: `internal/adapter/orca/client.go` and `internal/port/orca.go` — 기존 Orca worktree의 issue/comment metadata를 조회·설정하는 좁은 adapter contract를 제공한다.
- Modify: `internal/core/issueops/issueops_handoff_prepare_test.go` — exact legacy adoption, metadata drift, duplicate/mismatched checkout의 fail-closed 경계를 고정한다.
- Modify: `.agent-harness/ADR.md` — record-scoped authority와 bounded self-heal 결정을 기록한다.
- Create: `docs/superpowers/plans/2026-07-15-supervised-handoff-multicoordinator-acceptance.md` — runtime/fixture acceptance matrix.

### Task 1: Late coordinator guidance and exact authority

**Files:**
- Modify: `internal/core/lifecycle/lifecycle_handoff_guard.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_authority.go`
- Test: `internal/core/lifecycle/lifecycle_handoff_coordinator_dispatch_test.go`

**Consumes:** `HookToolUseLifecycleRequest{Host, SessionID, AgentID, CWD}` and the selected record ID.

**Produces:** a `handoff start` guidance command containing `--id`, `--coordinator-host`, `--coordinator-session-id`, optional `--coordinator-agent-id`, and `--source-cwd`; recipient remains absent only until IssueOps candidate resolution can prove it unique.

- [ ] **Step 1: Write failing late-context guidance tests**

```go
func TestCoordinatorPreparingGuidanceUsesCurrentIdentityForRequestedRecord(t *testing.T) {
    // Create records A and B, invoke the hook event for A after prepare,
    // and require A's ID/current host-session-agent flags only.
}

func TestCoordinatorPreparingGuidanceDoesNotBorrowOtherRecordRecipient(t *testing.T) {
    // Seal B with term_b; A guidance must not contain term_b.
}
```

- [ ] **Step 2: Run the focused test and observe the old guidance failure**

Run: `go test ./internal/core/lifecycle -run 'TestCoordinatorPreparingGuidance(UsesCurrentIdentityForRequestedRecord|DoesNotBorrowOtherRecordRecipient)' -count=1`

Expected: FAIL before the production change because late guidance uses the short command or a different record field.

- [ ] **Step 3: Render guidance from the current authenticated event**

Keep `buildCoordinatorDispatchCommand(record, req.Host, req.SessionID, req.AgentID)` as the only command builder. Replace coordinator-preparing short guidance with its output and preserve `allowedExactHandoffLifecycleCommand` checks for exact ID, source path, host/session/agent.

- [ ] **Step 4: Run lifecycle regression tests**

Run: `go test ./internal/core/lifecycle -run 'Coordinator|Handoff.*Guidance|Handoff.*Authority' -count=1`

Expected: PASS; copied identity, source path, ID, and recipient remain rejected.

- [ ] **Step 5: Commit the isolated lifecycle slice**

```bash
git add internal/core/lifecycle/lifecycle_handoff_guard.go internal/core/lifecycle/lifecycle_handoff_authority.go internal/core/lifecycle/lifecycle_handoff_coordinator_dispatch_test.go
git commit -m "fix(handoff): scope coordinator guidance to record"
```

### Task 2: Source recipient selection and cross-record collision fence

**Files:**
- Modify: `internal/core/issueops/issueops_handoff_dispatch.go`
- Test: `internal/core/issueops/issueops_handoff_dispatch_test.go`

**Consumes:** `IssueOpsRecord.Repo`, supplied `CoordinatorRecipient`, `ListTerminals`, active IssueOps records in the same state root.

**Produces:** `resolveHandoffCoordinatorRecipient` that returns a supplied/sealed exact handle, auto-selects exactly one candidate for the source worktree, or fails with the candidate count; no recipient handle may be sealed by another active record.

- [ ] **Step 1: Write failing source-recipient tests**

```go
func TestHandoffStartAutoSealsOnlyUniqueSourceRecipient(t *testing.T) { /* one matching source terminal */ }
func TestHandoffStartRejectsAmbiguousSourceRecipientsWithoutDispatch(t *testing.T) { /* two matching terminals */ }
func TestHandoffStartRejectsRecipientSealedByAnotherActiveRecord(t *testing.T) { /* same term_a */ }
```

- [ ] **Step 2: Run and confirm old behavior**

Run: `go test ./internal/core/issueops -run 'TestHandoffStart(AutoSealsOnlyUniqueSourceRecipient|RejectsAmbiguousSourceRecipientsWithoutDispatch|RejectsRecipientSealedByAnotherActiveRecord)' -count=1`

Expected: FAIL because missing recipient is rejected before a source inventory is examined and cross-record handle ownership is not checked.

- [ ] **Step 3: Implement bounded resolution**

Change resolution to accept the Orca client and state root. Filter `ListTerminals` by `Connected`, `Writable`, and `terminalWorktreePathMatches(terminal, record.Repo)`. Accept only one candidate; reject zero/many with no state mutation. Before sealing, scan active handoff records and reject a different active record with the same `CoordinatorMailboxHandle`. Preserve the existing supplied-handle syntax and sealed-handle equality checks.

- [ ] **Step 4: Run recipient tests**

Run: `go test ./internal/core/issueops -run 'TestHandoffStart.*Recipient' -count=1`

Expected: PASS; the ambiguous case makes zero `CreateTask` and `Dispatch` calls.

- [ ] **Step 5: Commit the recipient slice**

```bash
git add internal/core/issueops/issueops_handoff_dispatch.go internal/core/issueops/issueops_handoff_dispatch_test.go
git commit -m "fix(handoff): isolate coordinator recipients by record"
```

### Task 3: Worker baseline adoption and bounded pre-dispatch reconcile

**Files:**
- Modify: `internal/core/issueops/issueops_handoff_dispatch.go`
- Test: `internal/core/issueops/issueops_handoff_dispatch_test.go`

**Consumes:** exact worktree ID/root, `ListTerminals`, `ListTasks`, `ListDispatchedTasks`, durable handoff checkpoint and operation journal.

**Produces:** worker terminal bootstrap that adopts exactly one clean matching baseline, creates only when none exists, and writes a durable no-op/recovery receipt only for no-task/no-dispatch/no-worker/no-result pre-dispatch state.

- [ ] **Step 1: Write failing baseline and recovery tests**

```go
func TestHandoffStartAdoptsExactlyOneCleanWorkerBaseline(t *testing.T) { /* no CreateTerminal */ }
func TestHandoffStartCreatesWorkerTerminalWhenBaselineMissing(t *testing.T) { /* one CreateTerminal */ }
func TestHandoffStartMarksRecoveryForMultipleWorkerBaselines(t *testing.T) { /* no task/dispatch */ }
func TestHandoffStartDoesNotSelfHealPartialDispatch(t *testing.T) { /* existing task or dispatch */ }
func TestHandoffRecoveryRejectsDispatchOnlyPartialIdentity(t *testing.T) { /* dispatch id without task id */ }
```

- [ ] **Step 2: Run the focused test and observe the create-before-attest failure**

Run: `go test ./internal/core/issueops -run 'TestHandoffStart(AdoptsExactlyOneCleanWorkerBaseline|CreatesWorkerTerminalWhenBaselineMissing|MarksRecoveryForMultipleWorkerBaselines|DoesNotSelfHealPartialDispatch)' -count=1`

Expected: FAIL before implementation because `ensureHandoffTerminal` creates a terminal before testing the baseline inventory.

- [ ] **Step 3: Implement the record-scoped reconcile transaction**

Before `createHandoffTerminal`, enumerate only the record's worker worktree. When no persisted worker checkpoint exists: adopt one validated terminal; create one if zero; otherwise call the existing recovery-state writer with a specific code and return without task/dispatch. Retain checkpoint revalidation before every external stage. Never stop terminals automatically.

Keep the existing exact recovery-command fence: an extra lifecycle/recovery flag must not turn a bounded pre-dispatch exception into a general mutation permission.

- [ ] **Step 4: Run focused tests and race test**

Run: `go test ./internal/core/issueops -run 'TestHandoff(Start|Dispatch|OperationJournal|SoleWriter)' -count=1`

Expected: PASS.

Run: `go test -race ./internal/core/issueops -run 'TestHandoffStart' -count=1`

Expected: PASS; same-record start contention yields one terminal/task/dispatch or a stable replay projection, never duplicates.

- [ ] **Step 5: Commit the bootstrap/reconcile slice**

```bash
git add internal/core/issueops/issueops_handoff_dispatch.go internal/core/issueops/issueops_handoff_dispatch_test.go
git commit -m "fix(handoff): reconcile pre-dispatch terminals"
```

### Task 4: Architecture decision and acceptance matrix

**Files:**
- Modify: `.agent-harness/ADR.md`
- Create: `docs/superpowers/plans/2026-07-15-supervised-handoff-multicoordinator-acceptance.md`

**Consumes:** Tasks 1–3 public behavior and #26 remote acceptance criteria.

**Produces:** durable rationale and matrix for one coordinator, multiple independent coordinators, same-record contention, zero/one/many source recipient candidates, zero/one/many worker baseline terminals, and partial-dispatch fail-closed conditions.

- [ ] **Step 1: Add the ADR decision**

State that coordinator authority is record/worktree scoped; recipient handle is routing-only; no global lock is introduced; self-heal may reconcile only before task/dispatch/worker/result existence.

- [ ] **Step 2: Add the executable acceptance matrix**

Include each fixture state, expected record state, allowed external calls, prohibited external calls, and exact Go test command.

- [ ] **Step 3: Verify documentation truth**

Run: `git diff --check && go test ./internal/core/lifecycle ./internal/core/issueops -count=1`

Expected: PASS.

- [ ] **Step 4: Commit the decision documentation**

```bash
git add .agent-harness/ADR.md docs/superpowers/plans/2026-07-15-supervised-handoff-multicoordinator-acceptance.md
git commit -m "docs(adr): scope supervised handoff coordinators"
```

### Task 5: Integration verification and real Orca preview

**Files:**
- Verify only: all Task 1–4 files and the IssueOps record.

- [ ] **Step 1: Run package, race, vet, and build verification in one clean worktree**

```bash
go test ./internal/core/lifecycle ./internal/core/issueops -count=1
go test -race ./internal/core/lifecycle ./internal/core/issueops -count=1
go vet ./...
go build -o bin/agent-harness ./cmd/harness
```

Expected: all commands PASS.

- [ ] **Step 2: Reproduce the safe real-runtime branch**

In a disposable IssueOps record, run a non-confirmed start preview. With the current three source candidates it must return an ambiguity requiring a concrete handle and must not create task/dispatch. With a fixture/isolated source terminal it must resolve exactly one handle and preserve record scope.

- [ ] **Step 3: Record evidence and update #26 only if the contract changed**

Use `issueops feedback add` and `feedback mark-issue-updated` for a contract change; otherwise retain the verified existing issue body.

### Task 6: Existing legacy worktree adoption

**Files:**
- Modify: `internal/core/issueops/issueops_handoff_prepare.go`
- Modify: `internal/core/issueops/issueops_handoff_prepare_test.go`
- Modify: `internal/port/orca.go`
- Modify: `internal/adapter/orca/client.go`
- Test: `internal/adapter/orca/client_test.go`

**Consumes:** an existing real Git worktree at the canonical IssueOps path and its Orca worktree inventory row.

**Produces:** a supervised handoff record attached to that exact checkout. It records the existing Orca identity after setting the current IssueOps issue link and attempt marker; it never removes or recreates the checkout as an adoption side effect.

- [ ] **Step 1: Write adoption tests before production changes**

```go
func TestWorktreePrepareAdoptsExactExistingOrcaWorktree(t *testing.T) {
    // Existing canonical Git checkout and a single matching Orca row must
    // persist coordinator_preparing without calling CreateWorktree.
}

func TestWorktreePrepareRejectsMismatchedOrAmbiguousExistingOrcaWorktree(t *testing.T) {
    // Path/branch/head/repo/instance or duplicate-row drift must make zero
    // metadata mutations and leave ExecutionHandoff absent.
}
```

- [ ] **Step 2: Run the focused test and observe the collision failure**

Run: `go test ./internal/core/issueops -run 'TestWorktreePrepareAdopts|TestWorktreePrepareRejectsMismatchedOrAmbiguousExisting' -count=1`

Expected: FAIL before the production change because the preflight classifies the exact existing checkout as an Orca create collision.

- [ ] **Step 3: Implement exact adoption**

Classify a canonical legacy checkout separately from a collision. Permit adoption only when one Orca row has the canonical path, repo ID, branch, instance ID, and local prepared HEAD. Persist the operation journal before `orca worktree set`; require the returned row to have the current issue link and exact attempt marker. Keep normal create behavior unchanged, reject a pre-existing active IssueOps owner, and do not issue `orca worktree rm` in this path.

- [ ] **Step 4: Verify adapter and recovery boundaries**

Run: `go test ./internal/adapter/orca ./internal/core/issueops -run 'Test.*(Adopt|Legacy|WorktreePrepare)' -count=1`

Expected: PASS; list/show mismatch, missing metadata update, duplicate rows, and update ambiguity remain recovery/fail-closed cases.

- [ ] **Step 5: Commit the adoption slice**

```bash
git add internal/core/issueops/issueops_handoff_prepare.go internal/core/issueops/issueops_handoff_prepare_test.go internal/port/orca.go internal/adapter/orca/client.go internal/adapter/orca/client_test.go docs/superpowers/plans/2026-07-15-supervised-handoff-multicoordinator-self-heal.md
git commit -m "fix(handoff): adopt verified legacy worktrees"
```

## Self-Review

- Spec coverage: Tasks 1–3 cover single/multi coordinator dispatch, recipient collision, baseline terminal handling, same-record race, self-heal boundaries, and false cases. Task 4 records the architecture decision; Task 5 verifies the whole contract.
- Placeholder scan: no deferred implementation or unspecified test assertions remain.
- Type consistency: Task 1 stays in lifecycle; Tasks 2–3 own IssueOps dispatch; task boundaries share only existing `IssueOpsHandoffStartRequest`, `IssueOpsOrcaDispatchClient`, and durable `IssueOpsRecord` contracts.
