# IssueOps Publication Identity and Skill Read Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let an exact IssueOps worker read installed skill sources, publish from its live Orca owner terminal without false conflicts, and verify the resulting private PR/MR state through bounded read-only remote observations.

**Architecture:** Keep ownership enforcement fail-closed for mutations and competing dispatched agents. Extend the exact shell observation grammar only for bounded `wc` reads and numeric-range `sed -n` output. During publication, reconcile a reissued owner handle only when the live terminal matches the sealed PTY, tab, leaf, worktree, and path; permit only the structurally identified Orca lazygit sidecar while retaining exact dispatch conflict checks.

**Tech Stack:** Go 1.26, table-driven unit tests, Orca CLI inventory, IssueOps durable state.

## Global Constraints

- Do not relax shell control, expansion, redirection, remote mutation, or arbitrary remote URL rejection.
- Do not allow arbitrary extra connected or writable terminals.
- Do not change the sealed worker mailbox handle or dispatch identity when refreshing the live terminal handle.
- Preserve competing task/dispatch rejection on the exact worktree.
- Do not merge, clean up, or close IssueOps resources.

---

### Task 1: Bounded skill-source observation

**Files:**
- Modify: `internal/core/commandparse/issueops_test.go`
- Modify: `internal/core/commandparse/issueops.go`

**Interfaces:**
- Consumes: `ExactReadOnlyShellCommand(command string) bool`
- Produces: strict acceptance of `wc` with read-only count flags and explicit file operands, plus `sed -n` with a numeric print range and explicit file operands.

- [x] Add corpus cases allowing `wc -l /absolute/SKILL.md` and `sed -n '1,240p' /absolute/SKILL.md`, while rejecting control operators, redirection, stdin-only invocation, mutating sed flags, and unknown options.
- [x] Run `go test ./internal/core/commandparse -run TestExactReadOnlyShellCommandCorpus -count=1` and confirm the new allow case fails.
- [x] Implement the smallest exact `wc` argument validator.
- [x] Re-run the focused commandparse test and confirm it passes.

### Task 2: Stable publication owner identity

**Files:**
- Modify: `internal/core/issueops/issueops_handoff_sole_writer_test.go`
- Modify: `internal/core/issueops/issueops_handoff_dispatch.go`
- Modify: `internal/core/issueops/issueops_handoff_publication.go`

**Interfaces:**
- Consumes: sealed `WorkerPTYID`, `WorkerTabID`, `WorkerLeafID`, `WorkerTerminalHandle`, `WorkerMailboxHandle` and live `port.OrcaTerminal` inventory.
- Produces: an atomically refreshed live worker terminal handle while preserving mailbox/dispatch identity, plus strict recognition of the canonical Orca lazygit sidecar.

- [x] Add a failing test for a reissued owner handle with identical PTY/tab/leaf and an unchanged sealed dispatch assignee.
- [x] Add a failing test allowing the canonical lazygit sidecar and a regression test rejecting a non-sidecar writable terminal.
- [x] Run `go test ./internal/core/issueops -run 'TestHandoffSoleWriter|TestIssueOpsPublication' -count=1` and confirm the new cases fail for the expected false-conflict reason.
- [x] Implement stable terminal reconciliation and the narrow sidecar predicate without weakening dispatch conflict checks.
- [x] Re-run the focused IssueOps tests and confirm they pass.

### Task 3: Integration verification and live retry

**Files:**
- Verify all modified files above.

**Interfaces:**
- Consumes: built `bin/issueops`, installed native hooks, lifecycle `io-a370d531cf9f`.
- Produces: clean test/build results and a #51 publication attempt that clears the stale recovery marker or exposes a new evidence-backed blocker.

- [x] Run `gofmt` on changed Go files and `git diff --check`.
- [ ] Run focused tests, `go test ./... -count=1`, `go test -race ./... -count=1`, and `go build -o bin/issueops ./cmd/issueops`.
- [x] Run native install dry-run and actual install so all supported hosts receive the corrected hook/core binary.
- [x] Re-read the exact #51 worker identity and terminal inventory, then retry the supervised publication from the owner session path.
- [x] Verify the remote branch/PR head and durable IssueOps publication state; do not merge or clean up.

### Task 4: Bounded private-PR readback

**Files:**
- Modify: `internal/core/commandparse/issueops_test.go`
- Modify: `internal/core/commandparse/issueops.go`

**Interfaces:**
- Consumes: exact `gh pr view`, `gh pr checks`, `gh run view`, and `git ls-remote` argv.
- Produces: authenticated read-only PR/check/log/ref observation without permitting merge, workflow rerun/deletion, API mutation, push, arbitrary remote URLs, or upload-pack overrides.

- [x] Add allow cases for `gh pr view`, `gh pr checks`, `gh run view --log-failed`, and `git ls-remote --heads origin refs/heads/<branch>`.
- [x] Add deny cases for `gh pr merge`, `gh run rerun/delete`, `gh api`, `--web`, arbitrary ls-remote URLs, upload-pack overrides, control operators, and redirection.
- [x] Run the commandparse corpus and confirm the new readback cases fail before implementation.
- [x] Implement exact argv validators and re-run commandparse plus lifecycle tests to GREEN.

### Task 5: Owner publication completion

**Files:**
- Modify: `internal/core/commandparse/issueops.go`
- Modify: `internal/core/commandparse/issueops_test.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_authority.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_guard.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_ownership_authority_test.go`

**Interfaces:**
- Consumes: sealed coordinator and owner sessions, exact Orca owner handle, authenticated GitHub PR observation, and the existing `issueops remote verify-artifact` recorder.
- Produces: exact source-to-owner resume, owner-only existing-PR verification, durable PR-phase transition, and completion without source-session takeover.

- [x] Add exact source coordinator resume tests and retain rejection of arbitrary prompts, worker steering, and mismatched source sessions.
- [x] Implement and live-prove `계속 진행` plus Enter against the sealed #51 owner handle.
- [x] Allow bounded `--repo owner/name` on read-only GitHub observations.
- [x] Add an exact owner-only `remote verify-artifact` CLI path for an existing PR/MR.
- [ ] Reinstall, resume #51, and verify durable handoff completion reaches the human cleanup boundary.
