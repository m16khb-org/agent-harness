# IssueOps Audit: Multi-Session, Multi-Worktree, and State Management Analysis

> Generated: 2026-06-14
> Scope: Full-stack analysis of IssueOps skill, MCP tools, lifecycle hooks, state management, worktree isolation, and cross-session continuity.

---

## 1. Lock & Concurrency Analysis

### 1.1 Start() Lacks Locking (P0)

**File:** `internal/core/issueops/start/start.go:38-41`

The `Start()` function performs a read-modify-write (check if cycle exists, then resume/reset or create) but does NOT acquire `withIssueOpsLock`. The code itself acknowledges this with a comment:

```go
// NOTE: this read-modify-write (resumeOrReset may overwrite the record) is not
// locked. writeIssueOps is atomic per write (temp+rename) but offers no
// compare-and-swap, so concurrent Start/set-phase/link calls on the same
// repo+branch can lose updates. Pre-existing; a per-id lock or UpdatedAt
// version check would be the proper fix.
```

**Impact:** Two sessions starting on the same (repo, branch) simultaneously can:
- Both create fresh records (duplicate writes, last write wins)
- One session resets a stale cycle while the other resumes it → lost `StaleResetAt` audit trail
- Phase advancement in session A overwritten by session B's reset

**Fix:** Wrap `Start()` with `withIssueOpsLock`. The lock file already exists; just not used in this path.

### 1.2 Non-Unix Lock Fallback Is In-Process Only (P1)

**File:** `internal/core/issueops/issueops_lock_other.go`

The non-Unix fallback uses `sync.Mutex` scoped to a single process. On Windows/WSL, concurrent CLI invocations, MCP daemon, and hook subprocesses would NOT serialize. The comment acknowledges this:

```go
// concurrent multi-session invocations on the same host may still race.
```

**Impact:** On non-Unix platforms, all the TOCTOU protections provided by `flock` are absent across processes. The stale-scan re-read+re-classify under lock becomes in-process-only.

**Fix Options:**
- Use a file-based lock on Windows (e.g., `LockFileEx` via `golang.org/x/sys/windows`)
- Document that Windows requires single-session IssueOps usage
- Ship a `flock`-equivalent via a temp-file+rename based lock with backoff

### 1.3 Orphaned .lock Files Never Cleaned (P2)

**File:** `internal/core/issueops/issueops_lock_unix.go:25`

The lock file is created with `O_CREATE` and never deleted. After process death, the kernel releases the `flock`, but the `.lock` file remains on disk. Over many cycles, `~/.local/state/agent-harness/issueops/` accumulates `io-*.lock` files.

**Impact:** Cosmetic clutter; no functional issue since `flock` auto-releases.

**Fix:** Delete the lock file after `LOCK_UN` in `withIssueOpsLock`, or clean orphaned `.lock` files in `ScanStaleIssueOpsCycles`.

### 1.4 TOCTOU in Stale Scan Snapshot (P1)

**File:** `internal/core/issueops/issueops_stale_scan.go:50-57`

`ScanStaleIssueOpsCycles` snapshots all non-done cycles via `NonDoneCyclesForRepo`, then iterates. Between the snapshot and the `withIssueOpsLock`-guarded force-release, a cycle could advance to `done` in another session. The re-read+re-classify under lock closes the window for each individual cycle, but the snapshot itself could miss a newly-created cycle, or include a cycle that becomes done before the lock is acquired.

**Impact:** Low. The re-read under lock would correctly skip a now-done cycle. A newly created cycle would simply not be scanned this run.

---

## 2. Multi-Session State Continuity

### 2.1 No Session-to-Cycle Binding (P0) — RESOLVED 2026-06-12

**Resolution:** `LinkIssueOpsWorktree` now persists the binding via `BindIssueOpsSession`
(repo→cycle/branch/worktree), and every cycle-closing path (`AdvanceIssueOpsPhase` to done,
`ForceDoneIssueOps`, `ForceReleaseIssueOps`) unbinds it cycle-guarded
(`unbindIssueOpsSessionForCycle`). Hook guards resolve the expected worktree from the binding
only when the session is on the bound branch (`expectedWorktreeFromSessionBinding`), so one
cycle's binding never blocks unrelated work in the same repo. Pinned by
`TestLinkWorktreeBindsSessionAndDoneUnbinds`.

There is no mechanism to record "which session is working on which IssueOps cycle." The hook guard discovers the active cycle by reading the current git branch and looking up the cycle by (repo, branch). This works when:
- The session's shell cwd is in the worktree (not the source checkout)
- `HARNESS_EXPECTED_WORKTREE` is set

But fails when:
- The user opens a new session in the source checkout and doesn't set env vars
- The agent was working on cycle A in worktree A, but the new session opens in the source checkout
- The hook guard finds cycle B (if another branch is checked out) or no cycle at all

**Impact:** 
- Agent edits in the source checkout without worktree guard enforcement
- Wrong cycle's worktree guard applied
- Lost context: agent doesn't know which cycle to resume

**Fix Options:**
- Add a lightweight "current session" marker in harness state (`session-current-<repo-hash>` → cycle ID)
- SessionStart hook records the active cycle
- `issueops resume` command that restores the expected context

### 2.2 ExpectedWorktree Env Var Is Ephemeral (P1) — RESOLVED 2026-06-12

**Resolution:** the persisted session binding (2.1) now survives session restarts; the
lifecycle MCP guard falls back env → branch-matched session binding → active cycle records.
The hookcli-side duplicate fallback was removed in favor of the branch-guarded lifecycle path.

**File:** `cmd/harness/hookcli/hook_pre_tool_use.go:23`

`HARNESS_EXPECTED_WORKTREE` is read from the environment at hook invocation time. If a session is compacted and restored, the env var is lost. The hook falls back to branch-based cycle discovery, which may find the wrong cycle.

**Impact:** After compaction/restart, the worktree guard may be weaker (branch-based instead of exact path) or point at the wrong worktree.

**Fix:** Persist the "expected worktree" association in the IssueOps record or session state. Have the hook check both the env var AND the persisted state.

### 2.3 Phase Continuity After Session Interruption (P1)

When a session is interrupted mid-phase (e.g., during `implement`), the next session must:
1. Discover the active cycle
2. Read the phase
3. Set up the worktree context
4. Resume implementation

Currently, steps 1-3 are manual or rely on the hook guard. There's no `issueops resume` command that outputs the environment variables and context needed to continue.

**Fix:** Add `issueops resume --repo --branch --json` that returns the cycle state, expected worktree path, and recommended next actions.

### 2.4 Multiple Parallel Cycles in Same Repo (P1)

**Files:** `internal/core/lifecycle/lifecycle_worktree_guard.go:109-130`, `internal/core/issueops/active/issueops_active.go:65-114`

When multiple IssueOps cycles exist for the same repo (e.g., two different branches being worked on simultaneously), the worktree guard correctly enumerates all holders. However:

- The MCP worktree guard (`mcpWorktreeRootBlockReason`) blocks ALL filesystem/serena tools when ANY worktree-phase cycle exists, even if the agent is working in one of the valid worktrees
- The guard allows edits in any of the linked worktrees for the repo, not just the one matching the current branch

**Impact:** When cycle A and cycle B are both in `implement` phase, an agent working in worktree A could accidentally edit files in worktree B without being blocked, because both are valid linked worktrees.

**Fix:** The edit guard should verify the target is inside the SPECIFIC worktree for the cycle the agent is currently working on, not just any linked worktree. However, determining "which cycle the agent is working on" requires the session-to-cycle binding (see 2.1).

---

## 3. Worktree Lifecycle & Cleanup

### 3.1 Stale Cycle Detection Relies on Worktree Existence (P1)

**File:** `internal/core/issueops/stalescan/stalescan.go:95-116`

A cycle is `confirmed-stale` only when:
- The worktree directory is deleted
- The worktree is no longer git-tracked
- The worktree branch differs from the cycle's branch

If a session creates a worktree, works in it, then the user stops and never cleans up, the cycle stays in its current phase indefinitely. The stale scan only catches it after `max_age` (default 14 days), and even then it's `needs-review` (report-only, not auto-released).

**Impact:** Accumulation of paused cycles. The worktree guard for the source checkout is correctly bypassed when the worktree is missing, so these don't deadlock the source checkout. But they consume mental space and state directory entries.

### 3.2 No Inactivity-Based Session Abandonment Detection (P2)

There's no signal to distinguish "active but paused" from "abandoned." The stale scan uses only:
1. Worktree deleted? (confirmed-stale)
2. Remote branch merged/deleted? (likely-done)
3. Age > max_age? (needs-review)

A cycle that's 3 days old with an intact worktree and remote branch is considered live, even if the user has no intention of returning to it.

**Fix Options:**
- Add a "heartbeat" timestamp that the agent updates when actively working on a cycle
- Use git reflog to detect recent activity in the worktree
- Add a `issueops pause` command that explicitly marks a cycle as paused

### 3.3 Orphan Worktree Cleanup Is Off-Hot-Path Only (P2)

**File:** `internal/core/issueops/issueops_stale_scan.go:86-88`

`git worktree prune` and `git worktree remove --force` only run during `issueops cleanup stale --apply`. If the user never runs this, orphaned worktree directories from force-released cycles accumulate.

**Fix:** Add a lightweight `issueops cleanup worktrees --dry-run` that can be run more frequently.

### 3.4 Worktree Branch Mismatch = Stale (P2)

**File:** `internal/core/issueops/stalescan/stalescan.go:111-113`

If the worktree HEAD differs from `record.Branch`, it's classified as `confirmed-stale` (releasable). But this could be legitimate — the agent might checkout a different branch in the worktree for comparison or testing. Force-releasing in this case would destroy active work.

**Fix:** `worktree_branch_mismatch` should be `needs-review` (not auto-releasable) unless additional signals confirm abandonment (e.g., worktree HEAD is on `main` or a different IssueOps branch).

---

## 4. Hook Guard Coverage

### 4.1 MCP Guard Only Covers Known Tool Patterns (P2)

**File:** `internal/core/lifecycle/lifecycle_worktree_mcp.go:16-27`

The MCP worktree guard checks:
- `codegraph` tools → requires `projectPath` matching expected worktree
- `filesystem`/`serena` tools → blocked entirely

But other MCP tools that can read/write files are not covered. Examples: `replace_in_file`, `write_to_file`, terminal tools, custom MCP servers. The guard relies on the edit-target guard (`worktreeGuardBlockReason`) for these, which checks tool names and command strings.

### 4.2 Edit Guard Only Blocks "Mutating" Tools (P2)

**File:** `internal/core/lifecycle/lifecycle_worktree_guard.go:13`

`toolUseMayMutateLifecycleFiles` determines which tools are blocked. Read-only tools like `read_file`, `grep`, `codegraph_search` are allowed regardless of worktree. This is correct behavior, but the classification of "mutating" depends on tool name matching.

### 4.3 Hook Input Parsing Varies by Host (P1)

**File:** `cmd/harness/hookcli/hookinput/`

The hook input format differs between Codex and Claude Code. The hook input parser must handle both. If a new host is added or an existing host changes its hook format, the parser must be updated. This is a maintenance risk.

---

## 5. State Accumulation & Pruning

### 5.1 Done Cycles Are Never Deleted (P2)

**File:** `internal/core/issueops/issueops_state.go`

Once a cycle reaches `done`, its JSON file persists forever. `NonDoneCyclesForRepo` filters them out, but they still occupy disk space and appear in state listings.

**Fix:** Add an optional `--prune-done` flag to `issueops cleanup stale` that deletes done cycle records older than a threshold.

### 5.2 Global State Directory Grows Unbounded (P2)

`IssueOpsStateRoot()` returns a single global directory. All repos' cycles share this space. A user working across many repos will accumulate many cycle records.

**Fix:** Periodic maintenance via `issueops cleanup stale --apply --max-age 720h` or add a `--prune-done` flag.

### 5.3 IssueOps Schema Versioning Is Minimal (P2)

IssueOps records now write `schema_version=6`. Schema v5 remains the historical boundary for publish/cleanup authority; v6 adds the exact effective push-target fingerprint and durable `remote_create_claim` identity needed for crash-safe provider mutation. Missing/zero through v4, plus v5 rows with no new authority, read as v6 in memory and are stamped only on a later authorized write. Raw v5 claim rows and old v5 publish receipts fail before rewrite with bounded re-attest/reconcile guidance, v6 is accepted, and v7+ fails closed before phase logic.

**Compatibility:** Each prior-version boundary rejects its next authority-bearing schema before any write and preserves bytes. Future-schema reads retain only a bounded identifiable handoff projection plus an in-memory invalid marker so hooks keep ownership guards fail-closed without interpreting unsupported state.

---

## 6. Error Handling & Edge Cases

### 6.1 Remote Artifact Verification Is Trust-Based (P2)

**File:** `internal/core/issueops/cleanupstatus/cleanup_status.go:16-37`

`VerifyIssueOpsRemoteArtifact` records whatever the agent provides. There's no server-side validation that the PR/MR actually exists at the given URL. The agent could record a fake URL and advance to `done`.

**Fix:** Add optional server-side verification (e.g., `gh pr view` or `glab mr view`).

### 6.2 Cleanup Status `ls-remote` Failure = Blocked (P2)

**File:** `internal/core/issueops/cleanupstatus/cleanup_status.go:92-93`

If `git ls-remote` fails (network issue, VPN down, token expired), cleanup reports `remote_branch_check_failed` and blocks cleanup. This is fail-safe but can be frustrating.

**Fix:** Distinguish transient failures from permanent ones. Allow override with `--skip-remote-check`.

### 6.3 Force-Release Reason Validation Is Minimal (P2)

**File:** `internal/core/issueops/issueops_force_release.go:24-26`

The force-release reason must be non-empty, but there's no minimum length or content validation. A single character suffices.

**Fix:** Require a minimum meaningful reason (e.g., 20 characters).

### 6.4 Concurrent Phase Advancement on Stale Cycles (P1)

If session A detects a stale cycle and begins resetting it while session B force-releases it, the lock serializes them. But `Start()` (which triggers `resumeOrReset`) doesn't acquire the lock, so it could race with `ForceReleaseIssueOps`.

**Impact:** `Start()` could reset a cycle that was just force-released, creating a new `problem`-phase record that overwrites the `done` record.

**Fix:** Add locking to `Start()` (same as 1.1).

---

## 7. Summary: Problem Severity Matrix

| ID | Problem | Severity | Impact |
|----|---------|----------|--------|
| 1.1 | Start() lacks locking | **P0** | Data loss, lost audit trail |
| 2.1 | No session-to-cycle binding | **P0** | Wrong cycle context, bypassed guards |
| 1.2 | Non-Unix lock is in-process only | **P1** | Race conditions on Windows |
| 1.4 | TOCTOU in stale scan snapshot | **P1** | Theoretical race (mitigated) |
| 2.2 | ExpectedWorktree env var ephemeral | **P1** | Guard weakening after restart |
| 2.3 | No resume command | **P1** | Manual context reconstruction |
| 2.4 | Parallel cycle edit confusion | **P1** | Edits in wrong worktree |
| 3.1 | Stale detection needs worktree deletion | **P1** | Paused cycle accumulation |
| 4.3 | Hook input format varies by host | **P1** | Maintenance burden |
| 6.4 | Start() + ForceRelease race | **P1** | Record overwrite |
| 1.3 | Orphaned .lock files | **P2** | Cosmetic clutter |
| 3.2 | No inactivity detection | **P2** | State accumulation |
| 3.3 | Orphan worktree cleanup off-hot-path | **P2** | Worktree accumulation |
| 3.4 | Worktree branch mismatch = stale | **P2** | False positive on force-release |
| 4.1 | MCP guard only covers known tools | **P2** | Partial coverage |
| 4.2 | Mutating tool classification pattern-based | **P2** | Coverage depends on naming |
| 5.1 | Done cycles never deleted | **P2** | Disk space |
| 5.2 | Global state directory unbounded | **P2** | Disk space |
| 5.3 | No IssueOps schema migration | **P2** | Future-proofing |
| 6.1 | Remote artifact verification trust-based | **P2** | Integrity |
| 6.2 | ls-remote failure blocks cleanup | **P2** | Usability |
| 6.3 | Force-release reason validation weak | **P2** | Audit quality |

---

## 8. Resolution Plan

### Phase 1: Critical Fixes (P0) — Must be sequential due to shared state paths

| # | Task | Files | Verification |
|---|------|-------|-------------|
| 1 | Add `withIssueOpsLock` to `Start()` | `start/start.go`, `package.go` | `go test ./internal/core/issueops/... -count=1 -race` |
| 2 | Add session-to-cycle binding via state key | New: `internal/core/issueops/session/`, `lifecycle/lifecycle_session.go` | Hook guard integration test |
| 3 | Add `issueops resume` command | `cmd/harness/`, `internal/core/issueops/` | Golden test + manual multi-session QA |

### Phase 2: High-Priority Improvements (P1) — Can be parallelized

| # | Task | Files | Can Parallel? |
|---|------|-------|---------------|
| 4 | Persist ExpectedWorktree in session state | `lifecycle/lifecycle_worktree_guard.go`, `session/` | After #2 |
| 5 | Add `issueops resume --json` with context output | `cmd/harness/`, new CLI command | After #4 |
| 6 | Tighten parallel cycle edit guard (target-specific worktree) | `lifecycle/lifecycle_worktree_guard.go` | After #2 |
| 7 | Windows file-lock implementation | `issueops_lock_other.go` (rename to `issueops_lock_windows.go`) | Independent |
| 8 | Add heartbeat/inactivity tracking | `issueops_phase.go`, new `internal/core/issueops/heartbeat/` | Independent |
| 9 | Hook input format versioning and compatibility test | `hookinput/`, new test fixtures | Independent |

### Phase 3: Polish & Maintenance (P2) — Fully parallelizable

| # | Task | Files | Can Parallel? |
|---|------|-------|---------------|
| 10 | Clean orphaned .lock files in stale scan | `issueops_stale_scan.go` | Independent |
| 11 | Add `--prune-done` flag to cleanup | `issueops_stale_scan.go`, CLI | Independent |
| 12 | Weekly state maintenance command | New CLI command | Independent |
| 13 | Tune branch-mismatch classification to needs-review | `stalescan/stalescan.go` | Independent |
| 14 | Add optional remote artifact verification | `cleanupstatus/`, `remoteartifact/` | Independent |
| 15 | Distinguish transient vs permanent ls-remote failures | `cleanupstatus/cleanup_status.go` | Independent |
| 16 | Minimum force-release reason length | `issueops_force_release.go` | Independent |
| 17 | Add IssueOpsRecord schema version + fail-safe read/write | `model/types.go`, `issueops_state.go` | Done 2026-07-02 |

---

## 9. Implementation Status (2026-06-14)

### Completed

| ID | Problem | Fix |
|----|---------|-----|
| 1.1 | Start() locking | **Already fixed** — `package.go` `StartIssueOps` wraps `start.Start()` in `withIssueOpsLock`. Removed stale misleading comment. |
| 2.1 | No session-to-cycle binding | **Implemented** — New `internal/core/issueops/session/` package. Session binding persisted per-repo under IssueOps state root. Integrated into PreToolUse hook via `resolveExpectedWorktree()`. |
| 2.2 | ExpectedWorktree env var ephemeral | **Implemented** — MCP guard now checks session binding before env var/cycle fallback. Hook CLI uses `resolveExpectedWorktree()` that reads session state. |
| 2.3 | No resume command | **Implemented** — CLI `issueops resume --repo --json` and MCP `issueops_resume` tool. Returns cycle state, bind status, suggested cycles when unbound. |
| 3.1 | Stale detection needs worktree deletion | **Mitigated** — Added `LastHeartbeatAt` field and `recordHeartbeatLocked()`. Stale scan now uses `lastActiveAt()` (prefers heartbeat over UpdatedAt). |
| 1.3 | Orphaned .lock files | **Fixed properly** — Lock files must NOT be deleted between lock/unlock (inode-based flock breaks if file is recreated). Orphaned `.lock` files (no matching `.json`) are cleaned by the stale scan's worktree cleanup pass. |
| 3.3 | Orphan worktree cleanup off-hot-path | **Implementing** — `--prune-done` flag added, `pruneDoneCycles()` in stale scan deletes old done cycles. |
| 3.4 | Branch mismatch = stale | **Fixed** — `worktree_branch_mismatch` now classified as `needs-review` (not auto-releasable). |
| 5.1 | Done cycles never deleted | **Fixed** — `--prune-done` flag (default 720h). |
| 6.3 | Force-release reason weak | **Fixed** — Minimum 10-character reason required (was: non-empty only). |

### Critical Bug Found & Fixed: Lock File Deletion Breaks Mutual Exclusion

**Root cause:** `os.Remove(lockPath)` after `flock(LOCK_UN)` in `withIssueOpsLock`. 
When goroutine A unlocks and deletes the lock file while goroutine B has already `OpenFile`'d 
the same inode, goroutine C's `OpenFile(O_CREATE)` creates a NEW inode — C gets a lock on a 
different inode than B. Both run their critical sections concurrently.

**Evidence:** `TestIssueOpsConcurrentFeedbackNoLostUpdate` consistently lost 35-38 of 50 updates 
(only 12-16 succeeded). After reverting to defer-based unlock without `os.Remove`, the test 
passes 5/5 times.

**Fix:** Reverted to original `defer unix.Flock(LOCK_UN)` + `defer f.Close()` pattern. 
Lock files persist until orphaned cleanup removes only those with no matching `.json`.

### Remaining (Deferred)

| ID | Problem | Reason |
|----|---------|--------|
| 1.2 | Non-Unix lock in-process only | Windows not a target platform yet |
| 4.1/4.2 | MCP/edit guard coverage gaps | Requires per-host tool name catalog |
| 4.3 | Hook input format varies by host | Maintenance burden, not a bug |
| 5.3 | IssueOps schema migration | No schema changes yet; add when needed |
| 6.1 | Remote artifact trust-based | Requires provider API integration |
| 6.2 | ls-remote blocks cleanup | Design choice: fail-safe over convenience |

### Reconciliation (2026-07-01)

Last reconciled against HEAD `116ebef` (2026-07-01). Locking/TOCTOU and phase-gate
hardening landed since the 2026-06-14 status above; each row was verified against the
cited commit's diff before marking it resolved.

| ID | Problem | Resolved by |
|----|---------|-------------|
| 1.1 / 6.4 | Start() lock-id TOCTOU; Start()↔ForceRelease race | `1f7d077` — `StartIssueOps` abs-normalizes the repo before hashing the lock id (`issueOpsStartLockID` in `package.go`), so a relative path and its absolute equivalent serialize on the SAME record; this closes the residual lost-update window where `Start()` could overwrite a just-force-released cycle (LK-01). |
| 2.1 | Session-binding read-modify-write race | `1f7d077` — a per-repo advisory `flock` (`session/session_lock_unix.go`, `session/session_lock_other.go`) wraps bind/unbind, and unbind is a locked compare-and-delete, so two cycles cannot race the shared per-repo binding file. |
| 1.3 | Orphaned `.lock` sweep off-hot-path | `1f7d077` — the orphan-lock sweep now runs on any `issueops cleanup stale --apply`, not only when a cycle was released; `.lock` files are intentionally left for the sweep to preserve the flock inode invariant. |
| — | Fail-closed grill/plan phase gates; partial-ledger backfill | `805d622` (phase ledger with fail-closed grill/plan gates), `b1354bd` (backfill partial phase ledger, clear stale notes). |
| — | Stale-reset preserves analysis metadata and resets approval gates | `878e04a`. |

1.4 (stale-scan snapshot window) remains theoretical/mitigated by the
re-read-and-re-classify-under-lock design described in §1.4. Deferred items 1.2,
4.1/4.2, 4.3, 5.3, 6.1, and 6.2 remain as recorded in the tables above.

These scenarios should be manually verified after Phase 1+2 fixes:

### A1: Basic Multi-Session Continuity
1. Session A: `issueops start` → `link-issue` → `branch prepare` → create worktree → `link-worktree` → `phase --to implement`
2. Close Session A
3. Session B (same repo, source checkout): Verify hook guard detects cycle and blocks source-checkout edits
4. Session B: `issueops resume --repo --branch` → verify correct worktree path and phase

### A2: Concurrent Cycle Creation
1. Session A: `issueops start --repo X --branch feat-a`
2. Session B: `issueops start --repo X --branch feat-a` (same branch, simultaneous)
3. Verify exactly one cycle exists, no data loss

### A3: Parallel Cycles Different Branches
1. Session A: Cycle on branch `feat-a`, worktree at `../repo.worktrees/feat-a`
2. Session B: Cycle on branch `feat-b`, worktree at `../repo.worktrees/feat-b`
3. Session A edits in worktree A → allowed
4. Session A edits in worktree B → blocked
5. Session A edits in source checkout on `main` → blocked (correct)

### A4: Stale Cycle Recovery
1. Create cycle, link worktree, advance to `implement`
2. Delete the worktree directory (`rm -rf`)
3. Run `issueops cleanup stale --repo --json` → confirm `confirmed-stale`
4. Run `issueops start --repo --branch` (same branch) → verify reset to `problem` with `StaleResetAt` set

### A5: Force-Release Audit Trail
1. Create cycle, advance to `implement`
2. Force-release with minimal reason → verify rejected (after Phase 3 fix)
3. Force-release with proper reason → verify `done` phase, `ForceReleasedAt`, `ForceReleaseReason`, `OrphanWorktreePath`

### Reconciliation (2026-07-07)

Last reconciled against the local Task 16 working tree on 2026-07-07. Two
dogfood paths found real strict-readiness gaps and were fixed before the final
verification battery.

| ID | Finding | Resolution and evidence |
|----|---------|-------------------------|
| B1 | CLI/MCP `issueops pr-readiness --strict` used the record-only strict readiness path, so parent cycles did not report incomplete linked children. | Added `IssueOpsStrictPRReadinessWithState` and switched CLI/MCP strict handlers to use it. Regression: `TestCLIIssueOpsStrictPRReadinessReportsIncompleteChildren`. Dogfood transcript: `/var/folders/rz/75gxg1nj7qn2rtxt195j292w0000gn/T/tmp.MRaiV8w3i4/b1-s1.txt`; parent `io-0fe5ef5d6859`, children `io-bf0c579fad54` and `io-4e6b583e1029`; after one child completed, strict readiness returned `child_incomplete:io-4e6b583e1029`; after both completed, it returned ready. |
| B2 | `workpool close --force` still blocked parent strict readiness because `poolIncomplete` inspected unfinished tasks after a pool was closed. | Closed pools now clear the parent pool gate regardless of unfinished task status; open pools still block. Regression: `TestIssueOpsStrictPRReadinessClearsAfterForceClosedPool`. Dogfood transcript: `/var/folders/rz/75gxg1nj7qn2rtxt195j292w0000gn/T/tmp.5GoAXy59sc/b2-s2.txt`; parent `io-3cac24a57923`, pool `wp-fa1810391b61`; before close missing included `pool_incomplete:wp-fa1810391b61`, after force close it did not. |
| B3 | Standalone install/update still had old third-party compatibility cleanup surfaces: Codex plugin cache patching for companion tools, removed upstream flag handling in `install-native.sh`, a Stop-hook auto-proceed alias, and an unused external-LLM Stop gate. | Removed those surfaces rather than retaining deprecated/no-op paths. Verification included targeted adapter/update/hook tests and a runtime `agent-harness update --path-mode=skip --json` readback checked for old upstream/compatibility terms. |

Verification run:

```text
go test ./cmd/harness/issueopscli -run TestCLIIssueOpsStrictPRReadinessReportsIncompleteChildren -count=1
go test ./internal/core -run TestIssueOpsStrictPRReadinessClearsAfterForceClosedPool -count=1
go test ./cmd/harness/hookcli ./internal/core/nextaction ./internal/core/lifecycle ./cmd/harness/issueopscli ./internal/adapter/codex ./internal/adapter ./cmd/harness/updatecli ./cmd/harness/installcli -count=1
Z_AI_API_KEY= go test ./... -count=1
go test -p 1 -timeout 20m ./... -count=1
go test -race -p 1 -timeout 20m ./... -count=1
go build -o bin/agent-harness ./cmd/harness
```

---

## Appendix B: Files Touched by This Audit

```
internal/core/issueops/
├── issueops_lock_unix.go          # flock implementation
├── issueops_lock_other.go         # !unix fallback (in-process only)
├── issueops_state.go              # Read/Write/normalize
├── issueops_phase.go              # Phase transitions
├── issueops_force_release.go      # Force-release
├── issueops_force_done.go         # Force-done
├── issueops_stale_scan.go         # Stale scan + worktree cleanup
├── issueops_readiness.go          # Plan/Implement/AISlopClean readiness
├── issueops_pr_readiness.go       # PR readiness
├── start/start.go                 # Start (missing lock!)
├── stalescan/stalescan.go         # Multi-signal classification
├── active/issueops_active.go      # Active cycle queries
├── cleanupstatus/cleanup_status.go # Cleanup readiness
├── linking/worktree.go            # Worktree validation
├── model/types.go                 # All DTOs
├── model/phase.go                 # Phase constants + helpers
internal/core/lifecycle/
├── lifecycle_state.go             # PreToolUse decision chain
├── lifecycle_worktree_guard.go    # Worktree edit guard
├── lifecycle_worktree_mcp.go      # MCP worktree root guard
cmd/harness/hookcli/
├── hook_pre_tool_use.go           # PreToolUse hook CLI
├── hook_stop.go                   # Stop hook CLI
├── hookinput/                     # Hook stdin parsing
skills/issueops/
├── SKILL.md                       # Phase router
├── references/cleanup-state.md    # Cleanup docs
├── references/worktree-context.md # Worktree contract
.agent-harness/
├── CAUTIONS.md                    # Section 21: Worktree guard lessons
├── ISSUEOPS_AUDIT.md             # This document
```

### Reconciliation (2026-07-07, sqlite state store)

The JSON-file + flock state layout audited in sections 1 and 3 has been
replaced by the SQLite store (`internal/core/sqlstore`; see the ADR "State
storage moves from JSON files + flock to SQLite"). Matrix items affected:

| ID | Status after migration |
|----|------------------------|
| 1.2 Non-Unix lock is in-process only | Resolved — the sqlstore span holds a `BEGIN IMMEDIATE` transaction on `harness.lock.db`; SQLite file locking is cross-process on every platform, and the `!unix` in-process fallback files are deleted. |
| 1.3 Orphaned .lock files | Obsolete — no per-entity lock files exist; the two SQLite files per state root are persistent by design. Legacy `.lock`/`.state-lock` files are ignored (fresh start). |
| 1.1 / 1.4 / 6.4 lock-based mitigations | Carried over — the same span discipline (no nesting, full read-modify-write span, sequential multi-entity steps with read-repair) now runs on sqlstore spans instead of flock. |
| 5.1 / 5.2 state growth | Unchanged in policy; records are rows, `state prune` / `cleanup stale --prune-done` delete rows instead of files. |

Fresh-start note: pre-migration `*.json` records are not read or migrated. The
state doctor treats legacy record/lock files as inert harness-owned leftovers.
