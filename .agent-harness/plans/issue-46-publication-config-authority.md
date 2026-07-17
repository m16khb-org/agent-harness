# Issue #46 Publication Config Authority Implementation Plan

> **For agentic workers:** Execute with the repository `turing` and `issueops` contracts. The claimed worker is the only implementation writer; the source coordinator checkout remains observation-only.

**Goal:** Fix B-45 through B-48 so IssueOps publication and phase completion work on a normal macOS Git installation, then install and prove the repaired hooks in a fresh Codex runtime.

**Architecture:** Preserve Git's `.lock` protocol for every config authority writable by the current process. Treat an effective config whose parent is not writable by the current process as trusted read-only authority, fingerprint its file identity and content before the operation, and require the same snapshot after the operation. Read config origin/include/rewrite inventories through a separate bounded-complete command result with explicit truncation instead of the 4096-byte diagnostic buffer. Compute implementation evidence from the persisted immutable `branch_prepare.base_sha` when present; retain moving-ref lookup only for legacy records without a valid base SHA.

**Tech Stack:** Go 1.26, Git CLI real-process fixtures, IssueOps SQLite state, Codex app-server JSONL hooks API, GitHub Actions.

## Global Constraints

- Issue: https://github.com/m16khb/agent-harness/issues/46
- Cycle: `io-65b25b19728b`
- Branch: `46-fix-publication-config-authority`
- Source checkout: `/Users/m16khb/Workspace/agent-harness` is observation-only during the supervised attempt.
- Worker checkout: `/Users/m16khb/Workspace/agent-harness.worktrees/46-fix-publication-config-authority` is the only implementation root.
- Do not weaken pre-existing writable-config lock collision or URL rewrite race tests.
- Do not mutate an accepted handoff envelope or fabricate historical evidence.
- Do not delete or replace co-resident Orca hooks; native install must preserve their relative positions.
- Do not print config values, credentials, or broad environment dumps in evidence.
- Every behavior change follows visible characterization/RED/GREEN evidence before production edits.

## Context

### Original request

The user temporarily moved `~/.codex/hooks.json`, resumed Codex, and asked to resolve every investigated blocker so agent-harness works normally after hooks are enabled again. The prior blanket instruction “어떤 방법을 쓰든 알아서해” authorizes the recommended bounded design without another preference gate.

### Repo grounding

- `.agent-harness/ISSUEOPS_ORCA_BLOCKERS_2026-07-16.md:428-462` records B-45 through B-48.
- `internal/core/issueops/issueops_handoff_publication.go:272-456` uses one 4096-byte helper and tries to create `.lock` beside every effective origin.
- `internal/core/issueops/implementation/evidence.go:104-130` compares against mutable `origin/<base>` before `<base>` and ignores `BranchPrepare.BaseSHA`.
- `internal/core/issueops/issueops_handoff_publication_test.go:254-418` pins writable lock collisions, rewrite race prevention, and missing-authority failure.
- Fresh `codex app-server --stdio` `hooks/list` on 2026-07-17 reported 13 enabled/trusted hooks, empty warnings/errors, but commands target the stale repo binary built at `18a8083…+dirty`.

### Decision-complete design

1. Add a config-inventory command path with a 1 MiB maximum and an explicit `truncated` result. Existing diagnostic commands retain 4096-byte redacted output.
2. Enumerate effective origins with `git config --show-origin --includes --name-only --list` and enumerate include path values separately. Both reads must be complete before parsing.
3. For each effective config path, inspect the parent. If an exclusive sibling lock can be created, keep it through target resolution/push/readback. If lock creation fails because the parent is not writable by the current process, admit only an existing regular file and capture a SHA-256 snapshot over canonical path, mode, size, modification time, and bytes. Any other lock failure remains fatal.
4. Re-enumerate URL rules and origin paths after lock acquisition, run the publication operation, then re-enumerate and re-fingerprint read-only authorities before releasing locks. Any drift fails closed.
5. In implementation evidence, validate `BranchPrepare.BaseSHA` as a full Git object and use `<baseSHA>..HEAD` first. Only records without that usable SHA fall back to `origin/<base>` and `<base>`.
6. Do not add a schema, CLI, or MCP command for coordinator evidence; B-47 is an immutable-base selection defect, not missing durable data.
7. Build and install only after source verification. Then prove file readback, build metadata, fresh hooks/list trust, direct hook allow/deny, and a fresh hooks-enabled Codex scenario.

### Assumptions/defaults

- A root administrator changing a system file during the operation is outside the current-user Git lock threat model. The operation still detects persistent before/after drift.
- One MiB is the bounded inventory ceiling. Overflow is an explicit error and never a partial parse.
- A missing optional default user/XDG config remains allowed exactly as current tests require; an effective origin must exist as a regular file.
- The issue remains one work item because all four blockers form one publication-to-install E2E and one rollback unit.

### Unresolved questions

None blocking. The root-administrator boundary is documented as a threat-model limit, not an implementation ambiguity.

## Turing Success Criteria

| ID | Binary pass condition | Evidence artifact |
| --- | --- | --- |
| G1-C1 | A real Git repo using a current-user read-only effective system config resolves exactly one push target without creating a sibling lock, and a changed snapshot is rejected. | `.agent-harness/turing/evidence/issue46-G1-C1-readonly-config.txt` |
| G1-C2 | An effective config inventory larger than 4096 bytes is parsed completely; an inventory larger than 1 MiB returns explicit truncation and no partial origins. | `.agent-harness/turing/evidence/issue46-G1-C2-config-inventory.txt` |
| G1-C3 | Existing writable config `.lock` collision and rewrite-race tests remain GREEN. | `.agent-harness/turing/evidence/issue46-G1-C3-writable-lock.txt` |
| G2-C1 | A record whose `base_sha` predates implementation and whose `origin/main` already contains it still reports implementation evidence; an unchanged SHA does not. | `.agent-harness/turing/evidence/issue46-G2-C1-immutable-base.txt` |
| G3-C1 | B-45/B-46/B-47 focused tests, full Go/race/vet/build/goldens, and deterministic self-verify all exit 0 from the final worker HEAD. | `.agent-harness/turing/evidence/issue46-G3-C1-full-gates.txt` |
| G4-C1 | Installed binary revision/hash matches the verified source build; native install preserves co-resident hooks; fresh exact-cwd hooks/list has required enabled/trusted hooks with empty warnings/errors. | `.agent-harness/turing/evidence/issue46-G4-C1-native-hooks.txt` |
| G4-C2 | A fresh hooks-enabled Codex process executes an allowed observation and blocks an unauthorized source mutation before execution; the IssueOps publication/phase scenario completes from durable state. | `.agent-harness/turing/evidence/issue46-G4-C2-live-e2e.txt` |
| G5-C1 | Independent reviewer returns unconditional approval, PR CI is green, PR is merged, issue #46 is closed, and local `main == origin/main`. | `.agent-harness/turing/evidence/issue46-G5-C1-remote-completion.txt` |

## Task 1: Pin publication failures

**Files:**
- Modify: `internal/core/issueops/issueops_handoff_publication_test.go`

- [ ] Add a real-Git test that supplies an effective config in a non-writable parent and expects `PushTarget` success with no sibling `.lock` left behind.
- [ ] Add a seam test that changes the read-only snapshot during the protected callback and expects a fail-closed drift error.
- [ ] Add an inventory test with more than 4096 bytes and require the last origin to appear.
- [ ] Add an over-1-MiB inventory test requiring an explicit truncation error.
- [ ] Run only the named tests and capture expected RED failures caused by lock unavailability, incomplete origin, and missing overflow detection.

## Task 2: Seal writable and read-only config authority

**Files:**
- Modify: `internal/core/issueops/issueops_handoff_publication.go`
- Test: `internal/core/issueops/issueops_handoff_publication_test.go`

- [ ] Add the bounded-complete config inventory result without changing diagnostic command output limits.
- [ ] Split name-only origin enumeration from include-path value enumeration.
- [ ] Add regular-file snapshot and current-user read-only classification helpers.
- [ ] Keep exclusive `.lock` files for writable authorities and fail on ambiguous errors or pre-existing locks.
- [ ] Verify rules/origins after acquisition and after the operation; verify read-only fingerprints after the operation.
- [ ] Run the new tests to GREEN, then rerun all existing publication tests including lock collision and global rewrite race.

## Task 3: Use immutable branch base evidence

**Files:**
- Modify: `internal/core/issueops/implementation/evidence.go`
- Modify: `internal/core/issueops/implementation/evidence_test.go`

- [ ] Add a test repo where `base_sha` is the original commit, feature HEAD changes a non-plan file, and `origin/main` is moved to feature HEAD; confirm current code returns false.
- [ ] Add unchanged and malformed/missing `base_sha` compatibility cases.
- [ ] Select a verified persisted `base_sha` before moving refs, preserving the legacy fallback only when no valid immutable SHA exists.
- [ ] Run the named tests RED then GREEN and verify old evidence helper tests.

## Task 4: Record operational lessons and Turing evidence

**Files:**
- Modify: `.agent-harness/ISSUEOPS_ORCA_BLOCKERS_2026-07-16.md`
- Modify: `.agent-harness/CAUTIONS.md`
- Create/modify: `.agent-harness/turing/evidence/issue46-*.txt`
- Modify: `.agent-harness/turing/goals.json`
- Modify: `.agent-harness/turing/ledger.jsonl`

- [ ] Update B-45/B-46/B-47 status with the actual root fix and RED/GREEN evidence; add every newly encountered blocker with symptom, cause, impact, recovery, state, and exact evidence.
- [ ] Document writable-lock/read-only-fingerprint threat-model boundary and immutable-base rule in CAUTIONS.
- [ ] Persist each success criterion artifact and cleanup receipt without config values or credentials.
- [ ] Recompute evidence coverage, rework rate, cycle efficiency, and cleanup compliance.

## Task 5: Final source verification and review

**Files:** all changed paths from Tasks 1-4.

- [ ] Run `gofmt` on the exact changed Go files and `git diff --check`.
- [ ] Run the focused publication and implementation tests with `-v` and require named `=== RUN` lines.
- [ ] Run `go test ./... -count=1`, `go test -race ./... -count=1`, `go vet ./...`, response-contract goldens, and `go build`.
- [ ] Run deterministic isolated `self-verify` with seed 100, target 95, and LLM evaluation disabled; capture cleanup of the exact temp state root.
- [ ] Run IssueOps ai-slop-clean and Shannon baseline/final checks without widening scope.
- [ ] Give the final diff, issue criteria, evidence, and cleanup receipts to a fresh adversarial reviewer; fix and restart affected/full gates until unconditional approval.

## Task 6: Install and prove live hooks

**Files/state:**
- `bin/agent-harness`
- `/Users/m16khb/.local/bin/agent-harness`
- `/Users/m16khb/.codex/hooks.json`
- host-native MCP/skill integrations owned by `install-native`

- [ ] Record pre-install hashes/build metadata and a dry-run install plan.
- [ ] Run the repository-native install/update path once from the verified worker source; preserve unrelated hook groups.
- [ ] Verify installed and repo binary metadata/hash correspond to the final source commit.
- [ ] Start a fresh `codex app-server --stdio`, send initialize/initialized/hooks-list separately, and capture exact-cwd enabled/trusted inventory with empty warnings/errors.
- [ ] Run direct full-payload SessionStart/PreToolUse smokes and a fresh hooks-enabled Codex allow/deny scenario.
- [ ] Verify no hook test process, temp state, terminal, task, lock, or worktree artifact remains beyond the current accepted IssueOps worktree.

## Task 7: Publish and merge

**Files/state:** Git branch, IssueOps record, GitHub issue #46, PR.

- [ ] Commit one atomic fix with Conventional subject and Lore body after all local gates.
- [ ] Finish and accept the supervised handoff at its exact final HEAD.
- [ ] Run fixed `handoff publish --confirm`, create the draft PR through the supervised wrapper with Korean body, `bug` label, `m16khb` assignee, and base `main`.
- [ ] Wait for all CI checks, address verified feedback, and merge only the checked head.
- [ ] Verify issue #46 closed, no unexpected open PR, `main == origin/main`, installed source remains aligned, and cleanup status is recorded.

## Final Verification Wave

Run from the final worker HEAD in this order; any edit restarts the wave:

1. Focused named publication and immutable-base tests.
2. `git diff --check` and exact `gofmt` verification.
3. `go test ./... -count=1`.
4. `go test -race ./... -count=1`.
5. `go vet ./...`.
6. Contract golden commands and `go build`.
7. Isolated deterministic self-verify.
8. Independent adversarial review to unconditional approval.
9. Native install dry-run and real install.
10. Fresh hooks/list plus hooks-enabled allow/deny/publication scenario.
11. PR CI, checked-head merge, main/install/issue reconciliation.

## IssueOps Benchmark Evidence

Repo grounding: `issueops_handoff_publication.go`, publication real-Git tests, `implementation/evidence.go`, B-45~B-48 ledger, live Codex hooks/list, installed binary metadata.

Decision-complete plan: writable `.lock` plus read-only before/after fingerprint; 1 MiB complete inventory with explicit truncation; persisted base SHA first; one supervised Orca worker and one final reviewer.

Assumptions/defaults: root-admin mutation is outside current-user lock authority; missing optional configs retain existing behavior; public schema/CLI/MCP remain unchanged.

Unresolved questions: none blocking.

Acceptance criteria: G1-C1 through G5-C1 above, with exact evidence paths and the ordered final verification wave.

Success criteria: eight binary criteria, each requiring a named artifact and cleanup receipt.

Evidence artifact: `.agent-harness/turing/evidence/issue46-*.txt`, IssueOps `io-65b25b19728b`, GitHub issue #46, final PR/CI/merge metadata.

Cleanup receipt: every temp repo/state/process/hook probe is paired with exact removal or process-exit evidence.

Verification mode: full Turing loop because publication authority and live hook installation are security- and workflow-critical.

Skipped checks: none planned.
