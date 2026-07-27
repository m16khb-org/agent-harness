# IssueOps Orca Dogfood Recovery Implementation Plan

> **Execution owner:** the current #117 native coordinator. Keep #190
> claimable and preserve unrelated dirty bytes in the source `main` checkout.

**Goal:** Repair and activate the IssueOps/Orca path that blocked #190, then
reseed #190 from the repaired parent integration head.

**Architecture:** Keep validation in IssueOps core, Orca timing and worktree
arguments in the Orca adapter, shell admission in the lifecycle hook, and
judge guidance in the IssueOps skill. Every production change begins with a
focused failing test.

**Tech Stack:** Go 1.26, standard `testing`, GitHub CLI, public Orca CLI,
SQLite-backed IssueOps state.

---

### Task 1: Preview/confirm contract parity

**Files:**
- Modify: `internal/core/issueops/execution_prepare.go`
- Test: `internal/core/issueops/execution_prepare_preview_contract_test.go`

- [x] Add a test where Orca preview uses a noncanonical CWD and prove current
  preview succeeds while confirm rejects it.
- [x] Add a test where the remote issue omits acceptance IDs or the exact
  verification block and prove current preview succeeds while confirm rejects
  it.
- [x] Run the focused tests and capture RED for the expected mismatch.
- [x] Move only the read-only checks before the preview return.
- [x] Re-run the focused tests and existing auto/branch preview suites GREEN.

### Task 2: Restore local-first Orca worktree creation

**Files:**
- Modify: `internal/adapter/orca/execution.go`
- Test: `internal/adapter/orca/execution_worktree_upstream_test.go`

- [x] Add tests for both intent and legacy prepare paths asserting the create
  request uses sealed `BaseHead` and leaves `UpstreamBranch` empty.
- [x] Confirm RED because both paths currently inject
  `refs/remotes/origin/<branch>`.
- [x] Remove only the impossible upstream argument and its now-unused helper.
- [x] Re-run Orca adapter and IssueOps branch-precheck suites GREEN.

### Task 3: Wait for terminal inventory appearance

**Files:**
- Modify: `internal/adapter/orca/execution.go`
- Test: `internal/adapter/orca/execution_launch_timing_test.go`

- [x] Add an absent-then-present fixture that succeeds inside a millisecond
  test settle window.
- [x] Add a persistently absent fixture that returns bounded attempt/duration
  evidence.
- [x] Confirm current immediate-absence behavior RED.
- [x] Retry only absence; keep duplicate PTY inventory immediately fatal.
- [x] Re-run timing, fixture, and full Orca adapter tests GREEN.

### Task 4: Admit exact owner read/control commands

**Files:**
- Modify: `internal/core/lifecycle/lifecycle_execution_guard.go`
- Test: `internal/core/lifecycle/lifecycle_owner_control_plane_guard_test.go`
- Verify: `cmd/harness/hookcli` tests

- [x] Add table-driven RED cases for a literal GitHub issue body read and
  Orca `send`/`ask`/`check` commands from the canonical worker.
- [x] Add near-miss cases for DELETE, arbitrary GraphQL, unknown message
  types/flags, substitution, redirect, and detached forms; these must remain
  blocked.
- [x] Implement exact token/flag classifiers without widening general shell
  observation.
- [x] Re-run lifecycle and hook CLI suites GREEN.

### Task 5: Align remote-score documentation

**Files:**
- Modify: `cmd/harness/issueopscli/remotecmd/remote.go`
- Test: `cmd/harness/issueopscli/issueops_remote_score_cli_test.go`
- Modify: `skills/issueops/SKILL.md`
- Modify: `skills/issueops/references/remote-issue.md`
- Modify: `skills/issueops/references/cleanup-state.md`

- [x] Add a read-only `--judge prompt` envelope so the documented host-agent
  handoff is executable.
- [x] Replace active `--judge llm --model` instructions with host-agent prompt
  generation and `--judge file --judge-file`.
- [x] Preserve deterministic fallback and independent-review provenance.
- [x] Run `validate-skill.py` and search active IssueOps instructions for stale
  command examples.

### Task 6: Bound removed sqlstore roots

**Files:**
- Modify: `internal/core/sqlstore/sqlstore.go`
- Test: `internal/core/sqlstore/resource_test.go`

- [x] Reproduce the deleted temporary root remaining in the global handle cache.
- [x] Prune and close only handles whose roots no longer exist.
- [x] Prove the IssueOps package no longer exceeds its five-minute test budget.

### Task 7: Integrated verification and publication

- [x] Run `gofmt` on changed Go files.
- [x] Run `git diff --check`.
- [x] Run all focused packages.
- [x] Run `go test ./... -count=1`.
- [x] Run `go test -race ./... -count=1`.
- [x] Run response-contract golden tests.
- [x] Build `bin/agent-harness`.
- [x] Verify the source `main` dirty files are untouched.
- [x] Create atomic commits with Conventional Commit subject and Lore body.
- [ ] Push `117-hexagonal-architecture-migration`.

### Task 8: Activate and prove installed surfaces

- [ ] Run `ah update` from the parent canonical worktree.
- [ ] Restart the daemon through the supported CLI.
- [ ] Verify daemon status and installed CLI version/behavior.
- [ ] Verify `codex mcp get agent_harness` and `claude mcp list`.
- [ ] Start a fresh MCP process and exercise the corrected status/prepare
  surface.

### Task 9: Recover and resume #190

- [ ] Re-read lifecycle `io-17a57cc2b08b` and exact Orca inventory.
- [ ] Quiesce/retire its task, dispatch, terminal, worktree, and generation
  only through generation-fenced IssueOps/Orca recovery.
- [ ] Move the #190 linked branch to the new sealed parent integration base
  without losing remote issue linkage.
- [ ] Reseed and dispatch a fresh Orca owner.
- [ ] Confirm active claim and the first RED test before returning to the
  umbrella child DAG.
