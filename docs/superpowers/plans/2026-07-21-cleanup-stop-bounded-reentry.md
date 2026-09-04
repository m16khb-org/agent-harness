# Cleanup Stop Bounded Re-entry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Current execution:** The user explicitly selected the main session and main worktree. Implementation sub-agents and isolated-worktree edits are not authorized for this run.

**Goal:** Prevent `cleanup_pending_human_decision` from re-blocking the same Stop episode forever while preserving the initial human choice and pending state.

**Architecture:** Keep the lifecycle cleanup gate read-only and reuse the Stop-episode signals already computed by `runHookStop`. Add direct hook-level regression coverage, constrain only the cleanup block condition, and document the finite-exit invariant.

**Tech Stack:** Go 1.26, standard `testing`, issueops Stop hook adapter, Markdown project contracts.

## Global Constraints

- Do not add persistent acknowledgement state or change IssueOps schema.
- Do not mutate cleanup state from a Stop hook.
- Do not change the three cleanup choices or cleanup authority.
- Do not refactor unrelated Stop gates.
- Human-input-only waits end the turn; they do not invoke agent wait primitives.

---

### Task 1: Reproduce the cleanup Stop re-entry

**Files:**
- Modify: `cmd/issueops/hookcli/hook_stop_test.go`

**Interfaces:**
- Consumes: `runHookStop`, `runHookCapture`, `issueops.WriteIssueOps`, `issueops.ReadIssueOpsExisting`.
- Produces: a pending-cleanup fixture and assertions for first-contact no-auto no-op, ordinary initial block, relay-enabled choice continuation no-op, later fresh block, and unchanged record.

- [x] **Step 1: Add a pending-cleanup fixture**

```go
func seedPendingOwnershipCleanupForStop(t *testing.T) (string, string, []byte) {
    t.Helper()
    t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
    repo := t.TempDir()
    now := "2026-07-21T00:00:00Z"
    record := issueops.IssueOpsRecord{
        SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
        ID: issueops.NewIssueOpsID(repo, "cleanup-stop-reentry"), Repo: repo,
        Branch: "cleanup-stop-reentry", Phase: issueops.IssueOpsPhaseDone,
        ExecutionHandoff: &issueops.IssueOpsExecutionHandoff{
            State: handoff.StateCleanupPendingHumanDecision,
            Completion: &issueops.IssueOpsOwnershipCompletion{
                FinalHead: strings.Repeat("f", 40), CompletedAt: now,
            },
        },
        CreatedAt: now, UpdatedAt: now,
    }
    if _, err := issueops.WriteIssueOps(issueops.IssueOpsStateRoot(), record); err != nil { t.Fatal(err) }
    before, err := json.Marshal(record)
    if err != nil { t.Fatal(err) }
    return repo, record.ID, before
}
```

- [x] **Step 2: Add the regression sequence**

Create `TestRunHookStopBoundsOwnershipCleanupRelay` which:

1. calls a first-contact Stop whose message starts `자동진행하지 않음` and asserts an empty object;
2. calls an ordinary fresh Stop and asserts `decision == "block"` and the reason names the cycle and three human choices;
3. calls the same Stop with `stop_hook_active=true`, `--relay-next-action-judgement`, and a valid three-choice message, then asserts an empty object;
4. calls a later fresh Stop and asserts it may block once again;
5. marshals `ReadIssueOpsExisting` and compares it with the pre-Stop record bytes.

- [x] **Step 3: Verify RED**

Run: `go test ./cmd/issueops/hookcli -run TestRunHookStopBoundsOwnershipCleanupRelay -count=1`

Expected: FAIL because the continuation still returns the cleanup `decision=block`.

### Task 2: Bound the cleanup block

**Files:**
- Modify: `cmd/issueops/hookcli/hook_stop.go`

**Interfaces:**
- Consumes: existing `cleanupPending`, `stopHookActive`, and `noAutoProceedJudgement` booleans.
- Produces: the initial cleanup block or an immediate host no-op that cannot reach the generic next-action relay.

- [x] **Step 1: Constrain the branch**

```go
if cleanupPending {
    if stopHookActive || noAutoProceedJudgement {
        return printJSON(ho.FormatNoop())
    }
    markHookMetricBlocked()
    return printJSON(ho.FormatStopBlock("IssueOps " + cleanupID + " is at cleanup_pending_human_decision. No cleanup has run. Do not auto-proceed or invoke preview, approve, record, Orca, Git, or provider tools from Stop; the source/main session must present the three human choices: retain resources, close owner while retaining the workspace, or remove local resources."))
}
```

- [x] **Step 2: Verify GREEN**

Run: `go test ./cmd/issueops/hookcli -run 'TestRunHookStopBoundsOwnershipCleanupRelay|TestRunHookStopAllowsStopWhenStopHookActiveMissingChoices|TestRunHookStopAllowsNoAutoProceedJudgementWithoutChoices' -count=1`

Expected: PASS.

### Task 3: Codify the finite-exit invariant

**Files:**
- Modify: `.issueops/CONSTITUTION.md`
- Modify: `.issueops/CAUTIONS.md`

**Interfaces:**
- Produces: repository-wide rules forbidding unbounded agent/hook/relay loops and documenting the gate-ordering failure.

- [x] **Step 1: Add the constitutional invariant**

Under `문제 해결 원칙`, require a finite bound and explicit success, failure,
cancellation, timeout, or no-op exit for every agent/hook/relay/retry/monitor/
orchestration loop. State that user stop/no-auto-proceed outranks unchanged
durable state and that human-input waits end the turn without background waits.

- [x] **Step 2: Add the Stop caution**

Document that every `decision:"block"` branch must evaluate continuation and
no-auto exits before re-blocking, and that tests must cover initial block,
continuation no-op, no-auto no-op, and later independent reminder.

- [x] **Step 3: Verify docs-sensitive packages**

Run: `go test ./cmd/issueops/hookcli ./internal/core/skillcontract -count=1`

Expected: PASS.

### Task 4: Verify the exact change

**Files:**
- Modify only if verification exposes a scoped defect.

**Interfaces:**
- Produces: focused, full, race, build, and diff evidence.

- [x] Run `gofmt -w cmd/issueops/hookcli/hook_stop.go cmd/issueops/hookcli/hook_stop_test.go`.
- [x] Run `git diff --check`; expect no output.
- [x] Run `go test ./cmd/issueops/hookcli ./internal/core/lifecycle ./internal/core/skillcontract -count=1`; expect PASS.
- [x] Run `go test ./... -count=1`; expect PASS.
- [x] Run `go test -race ./... -count=1`; expect PASS.
- [x] Run `go build -o /tmp/issueops-issue65 ./cmd/issueops`; expect exit 0.
- [x] Review `git diff --stat` and the exact hook/test/constitution/caution diff; expect no unrelated changes.
