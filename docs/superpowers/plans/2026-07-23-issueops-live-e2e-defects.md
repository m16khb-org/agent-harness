# IssueOps Live E2E Defects Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the four live-E2E defects without changing the execution-v1 public command surface or mutating live state.

**Architecture:** Keep each correction at its existing ownership boundary: Orca identity in the Orca adapter, actor construction in the remote CLI adapter, activation ordering in the native install wrapper, and recovery guidance in the lifecycle guard. Every behavior change begins with a focused failing test.

**Tech Stack:** Go 1.26, Bash, standard `testing`, existing fake Orca and IssueOps fixtures.

## Global Constraints

- Work only in `/Users/sample/workspace/issueops` on the current `main`.
- Preserve existing user changes.
- Do not commit, push, publish, mutate user configuration, or edit durable IssueOps state.
- Do not add an unsafe execution reset or abandonment command.
- Use `apply_patch` for edits and TDD for every production behavior change.

---

### Task 1: Bounded Orca task identity

**Files:**
- Modify: `internal/adapter/orca/execution_v1.go`
- Test: `internal/adapter/orca/execution_v1_test.go`

**Interfaces:**
- Consumes: execution marker and sealed prompt SHA-256.
- Produces: `executionV1TaskTitle` and exact current-title matching.

- [x] Add a failing test asserting that the created title is at most 80
  characters and changes when either the operation marker or prompt digest
  changes.
- [x] Add an intent-inventory test proving the legacy
  `77 characters + "..."` title is ignored.
- [x] Run `go test ./internal/adapter/orca -run 'TaskTitle|Intent' -count=1`
  and confirm the new assertions fail for the current 118-character title.
- [x] Implement the compact lifecycle plus intent-digest title without a
  legacy compatibility candidate.
- [x] Re-run the targeted Orca tests and confirm they pass.

### Task 2: Remote PR native ancestry

**Files:**
- Modify: `cmd/issueops/issueopscli/remotecmd/remote.go`
- Test: `cmd/issueops/issueopscli/remotecmd/remote_test.go`

**Interfaces:**
- Consumes: native actor CLI flags and current process ancestry.
- Produces: a `model.NativeActorV1` whose `ProcessAncestry` is populated.

- [x] Add failing tests asserting the current process receipt is present in
  `ProcessAncestry`, dry-run skips observation, and confirmed PR creation
  propagates observation failure before provider mutation.
- [x] Run `go test ./cmd/issueops/issueopscli/remotecmd -run NativeActor -count=1`
  and confirm failure because PR creation has no shared ancestry helper.
- [x] Introduce the smallest injectable observation helper, use it in both
  `verify-artifact` and confirmed `runRemoteCreatePR`, and propagate errors.
- [x] Re-run the remotecmd package tests and confirm they pass.

### Task 3: Seal final native configuration

**Files:**
- Modify: `scripts/install-native.sh`
- Test: `internal/adapter/install_contract_matrix_test.go`

**Interfaces:**
- Consumes: optional glab MCP wrapper availability.
- Produces: native activation evidence captured after every wrapper-managed
  configuration mutation.

- [x] Add a failing contract test comparing the byte offsets of
  `sync-glab-mcp.sh` and the non-dry-run `"$BIN" install-native` invocation.
- [x] Run `go test ./internal/adapter -run InstallNativeScript -count=1` and
  confirm the current post-seal ordering fails.
- [x] Move the existing best-effort sync block before the installer invocation
  without changing its availability or dry-run conditions.
- [x] Re-run the install contract tests and confirm they pass.

### Task 4: Runnable lifecycle next command

**Files:**
- Modify: `internal/core/lifecycle/lifecycle_execution_v1_guard.go`
- Test: `internal/core/lifecycle/lifecycle_execution_v1_matrix_test.go`

**Interfaces:**
- Consumes: writerless execution-v1 lease states.
- Produces: actor-free status guidance for claimable, revoking, and released
  states.

- [x] Update the revoking regression expectation and add claimable/released
  cases that require the exact status command.
- [x] Run
  `go test ./internal/core/lifecycle -run 'ExecutionV1.*Lease' -count=1` and
  confirm the current claim/replace guidance fails.
- [x] Change `executionV1MutationDenyReason` to use
  `executionV1StatusCommand(record.ID)` for all three writerless states while
  retaining their structured deny codes.
- [x] Re-run lifecycle tests and confirm they pass.

### Task 5: Integrated verification

**Files:**
- Modify only files already listed if verification exposes a direct regression.

**Interfaces:**
- Consumes: all four green task-level changes.
- Produces: build and regression evidence.

- [x] Run `gofmt` on changed Go files.
- [x] Run `git diff --check`.
- [x] Run `go test ./internal/adapter/orca ./cmd/issueops/issueopscli/remotecmd ./internal/adapter ./internal/core/lifecycle -count=1`.
- [x] Run `go test ./... -count=1`.
- [x] Run `go test -race ./... -count=1`.
- [x] Run `go build -o bin/issueops ./cmd/issueops`.
- [x] Inspect `git status --short` and `git diff --stat`; report local changes
  without committing or pushing.
