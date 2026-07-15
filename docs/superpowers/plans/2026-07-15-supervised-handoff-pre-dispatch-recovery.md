# Supervised Handoff Pre-dispatch Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 정상 source coordinator의 pre-dispatch cancellation이 stale guidance 또는 cleanup receipt 검증 때문에 교착되지 않게 한다.

**Architecture:** 명시적 `resume --id`는 primary session binding이 아니라 요청 record의 persisted worktree를 안내한다. cleanup은 task/dispatch가 모두 없을 때 그 부재를 `task_terminal` receipt로 기록하되, 이후 기존 terminal quiescence 검증을 계속 강제한다. lifecycle fence는 `coordinator_preparing`에서 exact `recover --action cancel --confirm`만 source coordinator에 허용하고 나머지 mutation은 default-deny로 유지한다.

**Tech Stack:** Go 1.26, IssueOps durable state, lifecycle hook guard, Go testing.

## Global Constraints

- 기존 #18 Orca worktree와 GitHub child issues는 삭제·재작성하지 않는다.
- source checkout bootstrap 예외는 `~/.codex/hooks.json`의 `--enforce-worktree`를 원복할 때 끝난다.
- Orca task 실행 모델과 일반 source-checkout mutation guard는 변경하지 않는다.
- production code보다 regression test를 먼저 추가하고, 각 test가 기존 코드에서 실패함을 관측한다.

---

### Task 1: Explicit resume guidance isolation

**Files:**
- Modify: `internal/core/issueops/issueops_test.go`
- Modify: `internal/core/issueops/package.go:623-654`

**Interfaces:**
- Consumes: `IssueOpsResumeByID(repo, id)` and `BindIssueOpsSession(repo, cycleID, branch, expectedWorktree)`.
- Produces: an explicit-ID resume result whose `Guidance` derives from that record's `WorktreePath`.

- [ ] **Step 1: Write the failing test**

Add a test that creates two cycles in one repository, links each to a different worktree, binds the primary session to the first cycle, then invokes explicit resume for the second. Assert the second record's `WorktreePath` and `Guidance` name only the second worktree.

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `go test ./internal/core/issueops -run TestIssueOpsResumeByIDUsesRequestedCycleWorktree -count=1`

Expected: FAIL because `issueOpsResumeByID` calls `ExpectedWorktreeFromSession`, which returns the stale primary binding.

- [ ] **Step 3: Write the minimal implementation**

Keep `expectedWorktree := strings.TrimSpace(rec.WorktreePath)` for non-delegated explicit resume. Preserve scoped-binding lookup for delegated cycles; do not alter generic `IssueOpsResume` behavior.

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `go test ./internal/core/issueops -run TestIssueOpsResumeByIDUsesRequestedCycleWorktree -count=1`

Expected: PASS.

### Task 2: Durable cleanup for a taskless pre-dispatch cancellation

**Files:**
- Modify: `internal/core/issueops/issueops_handoff_cleanup_test.go`
- Modify: `internal/core/issueops/issueops_handoff_recovery.go:670-810`

**Interfaces:**
- Consumes: `RecoverIssueOpsHandoff` with `approve-cleanup` and ordered `record-cleanup` steps.
- Produces: a `task_terminal` receipt with empty task/dispatch IDs only when both persisted identity fields are empty; the existing terminal inventory check still runs next.

- [ ] **Step 1: Write the failing test**

Construct a closed cancelled handoff with a real Orca worktree/terminal identity but empty `TaskID` and `DispatchID`. Approve `remove`, record `task_terminal`, and assert that only then can `terminal_quiescent` run. Add a false case where exactly one of task/dispatch is empty and assert it remains rejected.

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `go test ./internal/core/issueops -run TestHandoffCleanupAllowsTasklessPreDispatchCancellation -count=1`

Expected: FAIL with `task cleanup verification dependency or identity is unavailable`.

- [ ] **Step 3: Write the minimal implementation**

In `verifyIssueOpsCleanupStep` for `task_terminal`, reject mismatched presence of `TaskID` and `DispatchID`; when both are empty, return the receipt without calling `ShowDispatch`. Keep the existing exact dispatch terminal verification unchanged when both exist.

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `go test ./internal/core/issueops -run TestHandoffCleanupAllowsTasklessPreDispatchCancellation -count=1`

Expected: PASS.

### Task 3: Narrow coordinator-preparing cancel escape

**Files:**
- Modify: `internal/core/lifecycle/lifecycle_handoff_authority.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_guard_test.go`

**Interfaces:**
- Consumes: exact parsed `handoff recover` command, hook request identity, and persisted `IssueOpsExecutionHandoff`.
- Produces: `allow` only for source checkout `handoff recover --id <record> --action cancel --confirm` when coordinator identity is exact and the handoff is pre-dispatch; all other `coordinator_preparing` recovery commands remain blocked.

- [ ] **Step 1: Write the failing test**

Create a `coordinator_preparing` handoff record with exact coordinator identity and no task, dispatch, worker session, result, or pending operation. Assert exact cancel is allowed, while missing `--confirm`, a mismatched coordinator session, `finalize-cancel`, and a source file edit are blocked.

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `go test ./internal/core/lifecycle -run TestCoordinatorPreparingAllowsOnlyExactPreDispatchCancel -count=1`

Expected: FAIL because the current authority table excludes `handoff recover` from `coordinator_preparing`.

- [ ] **Step 3: Write the minimal implementation**

Add a dedicated predicate for the exact cancel flags and a persisted pre-dispatch identity check. Invoke it only in the `handoff recover` case; leave the declarative table unchanged for every other recover action and state.

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `go test ./internal/core/lifecycle -run TestCoordinatorPreparingAllowsOnlyExactPreDispatchCancel -count=1`

Expected: PASS.

### Task 4: Whole-change verification and safety restoration

**Files:**
- Modify: `~/.codex/hooks.json` (restore from bootstrap backup; not repository content)

- [ ] **Step 1: Format and run focused packages**

Run: `gofmt -w internal/core/issueops/package.go internal/core/issueops/issueops_test.go internal/core/issueops/issueops_handoff_recovery.go internal/core/issueops/issueops_handoff_cleanup_test.go internal/core/lifecycle/lifecycle_handoff_authority.go internal/core/lifecycle/lifecycle_handoff_guard_test.go`

Then: `go test ./internal/core/issueops ./internal/core/lifecycle -count=1`

- [ ] **Step 2: Run required project checks**

Run: `go test ./... -count=1 && go test -race ./... -count=1 && go vet ./... && go build -o bin/agent-harness ./cmd/harness`

Expected: every command exits zero.

- [ ] **Step 3: Restore the worktree enforcement configuration**

Run: `cp /Users/m16khb/.codex/hooks.json.bootstrap-backup /Users/m16khb/.codex/hooks.json && rm /Users/m16khb/.codex/hooks.json.bootstrap-backup`

Expected: the temporary backup is absent and `rg --fixed-strings -- '--enforce-worktree' /Users/m16khb/.codex/hooks.json` finds the restored flag.
