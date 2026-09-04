# Hook Failure Diagnostics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Record redacted JSONL diagnostics whenever an `issueops hook ...` subcommand exits non-zero so future generic host messages can be traced to the failing hook and payload.

**Architecture:** Add a small core JSONL recorder that writes hook failure events under the existing user state directory, then wrap the top-level hook dispatcher so failures are recorded before the original error is returned. Keep host-facing stdout behavior unchanged for successful allow/block/no-op decisions.

**Tech Stack:** Go CLI, existing `internal/core` state directory conventions, JSONL files, focused Go unit tests and CLI smoke tests.

**IssueOps Context:** Issue `https://github.com/example/issueops/issues/1`; branch `feature/1-hook-failure-diagnostics`; worktree `/tmp/issueops.worktrees/feature-1-hook-failure-diagnostics`.

---

## File Structure

- Create: `internal/core/hook_failure_log.go` for event DTOs, redaction, bounded snippets, JSONL append, and recent-event reads.
- Create: `internal/core/hook_failure_log_test.go` for recorder/redaction behavior.
- Modify: `cmd/issueops/hook_user_prompt.go` to wrap hook subcommand execution and record failures.
- Modify: `cmd/issueops/hook_user_prompt_test.go` or the nearest existing hook test file to verify a failing hook path records a JSONL event.
- Modify: `cmd/issueops/main.go` and usage/golden files only if a new `issueops hook failures --json` inspection command is added.
- Modify: `.issueops/CAUTIONS.md` to keep the existing follow-up note in the implementation branch.

---

### Task 1: Add Core Hook Failure Recorder

**Files:**
- Create: `internal/core/hook_failure_log.go`
- Test: `internal/core/hook_failure_log_test.go`

- [ ] **Step 1: Write the failing recorder test**

Write `TestRecordHookFailureEventWritesRedactedJSONL` in `internal/core/hook_failure_log_test.go`. It must set `ISSUEOPS_STATE_DIR`, call `RecordHookFailureEvent`, read the JSONL path, assert the hook name is present, and assert a token-like value is redacted.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core -run TestRecordHookFailureEventWritesRedactedJSONL -count=1`

Expected: FAIL because `RecordHookFailureEvent` and `HookFailureEvent` are not defined.

- [ ] **Step 3: Implement the minimal recorder**

Implement `HookFailureEvent`, `HookFailureRecordResult`, `RecordHookFailureEvent`, and redaction in `internal/core/hook_failure_log.go`. Use `StateDir()` and write to `filepath.Join(StateDir(), "hook-failures.jsonl")` with `0600` file permissions. Redact token-like values with deterministic placeholders and bound snippets to a small fixed size such as 500 bytes.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core -run TestRecordHookFailureEventWritesRedactedJSONL -count=1`

Expected: PASS.

---

### Task 2: Wrap Hook Dispatcher Failure Paths

**Files:**
- Modify: `cmd/issueops/hook_user_prompt.go`
- Test: nearest hook test file under `cmd/issueops`

- [ ] **Step 1: Write the failing hook integration test**

Create a test that sets `ISSUEOPS_STATE_DIR` to a temp dir, calls `runHook([]string{"unknown-hook"})`, expects an error, and asserts `hook-failures.jsonl` contains a redacted event with `hook:"unknown-hook"` and the error message.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/issueops -run TestRunHookRecordsFailureEvent -count=1`

Expected: FAIL because `runHook` currently returns the error without recording a JSONL event.

- [ ] **Step 3: Implement the wrapper**

Refactor `runHook` into a wrapper plus private dispatcher:

```go
func runHook(args []string) error {
	err := runHookDispatch(args)
	if err != nil {
		recordHookFailure(args, err)
	}
	return err
}
```

`recordHookFailure` must best-effort only: if recording fails, it must not replace the original hook error or print secret-bearing details. Extract hook name from `args[0]` when present.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/issueops -run TestRunHookRecordsFailureEvent -count=1`

Expected: PASS.

---

### Task 3: Add Inspection Surface For Recent Failures

**Files:**
- Modify: `internal/core/hook_failure_log.go`
- Modify: `cmd/issueops/hook_user_prompt.go`
- Modify: `cmd/issueops/main.go` or hook usage text if dispatch requires it
- Test: focused CLI test under `cmd/issueops`

- [ ] **Step 1: Write the failing CLI inspection test**

Seed `hook-failures.jsonl` with one valid event, run `runHook([]string{"failures", "--json"})`, and assert JSON output includes `ok:true`, `path`, and an `events` array.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/issueops -run TestRunHookFailuresJSON -count=1`

Expected: FAIL because `hook failures --json` is not implemented.

- [ ] **Step 3: Implement `issueops hook failures --json`**

Add a read-only subcommand that reads recent JSONL records from `hook-failures.jsonl`, returns an empty list if the file does not exist, and defaults to a bounded recent count such as 20.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/issueops -run TestRunHookFailuresJSON -count=1`

Expected: PASS.

---

### Task 4: Preserve CAUTIONS Note And Verify Full Suite

**Files:**
- Modify: `.issueops/CAUTIONS.md`

- [ ] **Step 1: Apply the CAUTIONS follow-up in this worktree**

Add the same follow-up note from the source checkout under `2026-05-31 — PreToolUse false-positive risk` so the issue evidence remains tracked on the feature branch.

- [ ] **Step 2: Run focused tests**

Run:

```bash
go test ./internal/core -run HookFailure -count=1
go test ./cmd/issueops -run 'Hook.*Failure|HookFailures' -count=1
```

Expected: PASS.

- [ ] **Step 3: Run full verification**

Run:

```bash
go test ./... -count=1
go build -o bin/issueops ./cmd/issueops
```

Expected: PASS.

- [ ] **Step 4: Check worktree hygiene**

Run:

```bash
pwd
git branch --show-current
git rev-parse --short HEAD
git status --short
```

Expected: `pwd` is `/tmp/issueops.worktrees/feature-1-hook-failure-diagnostics`, branch is `feature/1-hook-failure-diagnostics`, and only planned files are modified.
