# Agent-Harness Whole-Project Audit

> Generated: 2026-06-14
> Path note (2026-08-26): the `internal/core/<pkg>` paths cited in the dated
> sections below predate the 2026-08-08 relocation (`ccee5d5f`). Current homes:
> worker/lifecycle/hookfailure/hookmetrics/policy/preflight/guard → `internal/adapter/<pkg>`,
> state → `internal/adapter/outbound/state`, structured JSON decoding →
> `internal/domain/judgement`, the draft-wiki queue lock and the Z.AI
> `externalllm` wrapper were removed with the draft-wiki worker, and the daemon
> lock lives at `cmd/harness/daemoncli/daemonlock/lock.go`. Section file:line
> references are kept as incident-time evidence.
> Scope: All 31 subsystems: daemon, MCP proxy, worker, command policy, state, lifecycle hooks (7 types), project bootstrap/docs, self-verify, self-augment, CLI, install, hook input, search routing, command guard, next-action relay, remote artifact gate, VCS linking, lint diagnose, project docs detection, prompt/compact, context region, API doc, draft wiki, trace, guard, contract CLI, preflight, external LLM, agy settings, commit suggest, repopath, docs index

---

## 1. Daemon Subsystem

### 1.1 No Connection Limit (P1)

**File:** `cmd/harness/daemoncli/daemon_server.go:97-113`

The accept loop spawns an unbounded goroutine per connection. A malicious or buggy client could open thousands of connections and exhaust memory/file descriptors.

**Fix:** Add a configurable max connection count with `sync.WaitGroup` + semaphore channel.

### 1.2 O_EXCL Lock on Network Filesystems (P2)

**File:** `cmd/harness/daemoncli/daemonlock/lock.go:14`

`os.O_EXCL` semantics are not guaranteed on NFS or certain FUSE filesystems. If the user's home directory is on a network mount, two daemons could start.

**Fix:** Add a `flock`-based fallback on Unix (like IssueOps lock), or document the NFS limitation.

### 1.3 No Graceful Shutdown of In-Flight Connections (P2)

**File:** `cmd/harness/daemoncli/daemon_server.go:97-113`

When the daemon is stopped (`SIGTERM` or `daemon stop`), in-flight MCP connections are dropped mid-stream. The accept loop exits when the listener closes, but goroutines serving active connections continue until their `conn.Read` fails.

**Fix:** Add a `sync.WaitGroup` tracking active connections; wait for them (with timeout) during shutdown.

---

## 2. Worker/Job System

### 2.1 Jobs Stuck in "running" on Process Crash (P1)

**File:** `internal/core/worker/read_only.go:14-22`

`RunReadOnlyWorkerJob` writes `status: running` to the job file, then executes the command. If the process crashes between the write and the final status update, the job is permanently stuck in `running`.

**Fix:** Add a heartbeat or PID field. On `worker list`, detect jobs with dead PIDs and mark them `failed`.

### 2.2 No Concurrent Job Execution Guard (P1)

**File:** `internal/core/worker/read_only.go:9-32`, `internal/core/worker/worker.go:73-88`

`RunReadOnlyWorkerJob` calls `EnqueueWorkerJob` (which creates a new job), then immediately marks it running. But `CancelWorkerJob` and `RunReadOnlyWorkerJob` have no mutual exclusion — two callers could try to cancel and run the same job simultaneously.

**Fix:** Add per-job advisory lock (same `flock` pattern as IssueOps).

### 2.3 No Retry, No Scheduling (P2)

Worker is explicitly MVP-only, but the data model already has `queued`/`running`/`failed` states. No mechanism exists to retry failed jobs or schedule them for later execution.

---

## 3. State Subsystem (Generic)

### 3.1 No Write Locking (P1)

**File:** `internal/core/state/state_io.go:26-47`

`StateWrite` uses `os.WriteFile` which is atomic on most local filesystems, but concurrent writes to the same key from different processes have undefined ordering — last write wins, and neither writer knows it lost.

**Fix:** Add optional advisory locking (same pattern as IssueOps `flock`).

### 3.2 No Atomic Multi-Key Transactions (P2)

Multiple state keys cannot be written atomically. Self-verify summary + checkpoint promotion writes multiple keys; if the process dies mid-way, state is partially updated.

---

## 4. Hook Failure Logging

### 4.1 Unbounded Log Growth (P1)

**File:** `internal/core/hookfailure/log.go:67-75`

Hook failures are appended to a single JSONL file with no rotation, pruning, or size limit. Over months of use, this file grows without bound.

**Fix:** Add `hook failures prune --max-age 720h` or rotate when file exceeds a size threshold.

### 4.2 Concurrent Append Safety (P2)

**File:** `internal/core/hookfailure/log.go:67-75`

Multiple hook invocations may append concurrently. Each write is `O_APPEND` + single `Write` call, which POSIX guarantees is atomic for writes under `PIPE_BUF`. But JSONL lines could exceed `PIPE_BUF` (typically 4096 bytes), leading to interleaved lines.

**Fix:** Use a file lock around the write, or truncate lines to fit within `PIPE_BUF`.

---

## 5. Project Lifecycle State

### 5.1 Init Race Condition (P1)

**File:** `internal/core/lifecycle/lifecycle_project_state_store.go:52-91`

`InitProjectLifecycleState` reads the project profile, checks if it exists, then writes. Two concurrent sessions initializing the same project for the first time could both write — last write wins, and no error is raised.

**Fix:** Use `O_EXCL` create for the initial profile file, or add advisory locking.

### 5.2 No Lock on Profile Updates (P2)

**File:** `internal/core/lifecycle/lifecycle_project_state_store.go:110-142`

`writeJSONAtomic` uses temp file + rename, which is atomic per-file. But read-modify-write cycles (init, doc-upkeep append, compact capsule write) on the same `project.json` have no mutual exclusion.

---

## 6. Draft Wiki Queue

### 6.1 No Stale Lock Detection (P1)

**File:** `internal/core/draftwiki/queue/lock.go:10-31`

`AcquireLock` uses `O_CREATE|O_EXCL`. If the process holding the lock crashes, the lock file remains forever. Unlike the daemon lock (which has PID-based stale detection), the draft wiki queue lock has no staleness check. The queue is permanently blocked until manual cleanup.

**Fix:** Add PID + timestamp to the lock file, and add stale detection (same pattern as `daemonlock`).

### 6.2 Lock Cleanup Function No-Op on Failure (P2)

**File:** `internal/core/draftwiki/queue/lock.go:16-18`

When `AcquireLock` returns `false` (already locked), the cleanup function is `func() {}` — a no-op. The caller must not call it (the API design expects `if acquired { defer release() }`), but if a caller mistakenly always calls it, nothing bad happens. Acceptable.

---

## 7. Compact Capsule

### 7.1 Capsule Overwrite on Double PreCompact (P2)

**File:** `internal/core/lifecycle/compact/compact.go:19-39`

If two `PreCompact` events fire without a `PostCompact` between them, the second overwrites the first capsule. The first capsule's pending doc-upkeep events are lost.

**Fix:** Check if a capsule already exists and merge/append instead of overwriting.

### 7.2 No Lock Between Read and Delete (P2)

**File:** `internal/core/lifecycle/compact/compact.go:50-68`

`BuildPostCompactReminder` reads the capsule file, then deletes it. A concurrent `PreCompact` between read and delete could create a new capsule that gets deleted by the `os.Remove`.

**Fix:** Rename the capsule file (to `.processed`) instead of deleting, or use a lock.

---

## 8. Next-Action Relay

### 8.1 Read-Then-Write Race (P2)

**File:** `internal/core/lifecycle/nextactionrelay/relay.go:48-69`

`Record` reads the existing relay record, checks for duplicates, then writes. Two concurrent Stop hooks firing within milliseconds could both decide to relay (both see no existing record). In practice, Stop hooks are serial per-session, so this is theoretical.

---

## 9. Self-Verify

### 9.1 Temp Directory Leaks on Kill (P2)

**File:** `cmd/harness/selfworkflow/verifyloop/loop.go:54,100,110`

`os.MkdirTemp` creates directories under `/tmp`. Cleanup via `os.RemoveAll` happens on both success and failure paths. However, if the process is killed (SIGKILL), temp directories accumulate. The CAUTIONS note says self-verify dirs are "properly cleaned" — true for normal termination, not for kill.

**Mitigation:** These are prefixed `agent-harness-self-verify-*` and can be cleaned by a periodic hygiene script. Low priority since the harness doesn't run as a persistent service.

---

## 10. Command Policy

### 10.1 Workspace Policy Overrides (Resolved)

**File:** `internal/core/policy/policy_catalog.go`

The built-in allow/deny lists remain the baseline, but workspace roots can now extend the catalog with `.agent-harness/policy.json`. The override is loaded per evaluation/root so same-process checks across two repositories do not leak the first root's policy into the second. Load and parse problems are surfaced through the existing `warnings` field instead of adding a new JSON top-level field.

**Verification:** `policy check`, `policy fake-run`, read-only command execution, and MCP policy state tests cover missing files, invalid JSON warnings, and two-root isolation.

### 10.2 No Chained Command Analysis (P2)

**File:** `internal/core/policy/policy_command_classification.go`

Commands with pipes (`|`), logical operators (`&&`, `||`), or command substitution (`$(...)`) are not decomposed. A command like `echo safe && rm -rf /` would only check `echo` in the allowlist.

**Fix is explicitly out of scope:** The harness enforces `shell_allowed=false` by default, which blocks shell interpreters. Shell metacharacters are only dangerous when `shell_allowed=true`, which requires `shell_reason`.

### 10.3 Shell Interpreter Detection Is argv[0]-Only (P2)

Shebang lines in scripts are not inspected. Running `./malicious.sh` where the shebang is `#!/bin/bash` would bypass the shell check if `./malicious.sh` isn't in the deny list.

---

## 11. External LLM

### 11.1 Single Provider (Z.AI Only) (Accepted)

**File:** `internal/core/externalllm/print.go`

All external LLM calls go through the Z.AI chat-completions API wrapper in `internal/core/externalllm`. There is no provider abstraction for OpenAI, Anthropic direct, Ollama, etc.

**Disposition:** Accepted YAGNI. The unused `port.ExternalLLM` interface was deleted on 2026-07-02; add a provider interface only after a second provider is actually required by a real caller.

### 11.2 No Retry on Malformed Responses (P2)

**File:** `internal/core/externalllm/structured.go`

`DecodeExternalLLMStructuredJSONObject` handles fenced code blocks and raw JSON, but if the LLM returns truly malformed output, it fails with no retry. The caller receives an error and must handle it.

---

## 12. MCP Proxy

### 12.1 Dual Transport Code Paths (P2)

**File:** `cmd/harness/mcpcli/mcp_sdk_server.go`, `cmd/harness/mcpcli/mcp_transport.go`

Two JSON-RPC implementations coexisted: the SDK-based transport (for daemon sockets) and a legacy hand-rolled parser (for test pipes). **Resolved 2026-08-03 (`034bda93`):** the hand-rolled path was removed and `serveMCPStreamSDK` (`cmd/harness/mcpcli/mcp_sdk_server.go`) now serves both split stdio and bidirectional daemon connections.

### 12.2 Tool Catalog Drift (P1)

**File:** `cmd/harness/mcpcli/mcp_sdk_server.go`

MCP tools are registered via string name matching in `handleIssueOpsMCPToolCall`. Adding a new tool requires updating: (1) the adapter catalog, (2) the tool name list, (3) the dispatch switch, (4) the golden test file. Missing any step produces a silent gap (tool not registered) or a golden test failure.

**Fix:** Generate the tool dispatch table from the adapter catalog at init time instead of manual switch statements.

---

## 13. Lifecycle Hooks

### 13.1 Hook Failure Log Grows Unbounded (P1)

Same as §4.1.

### 13.2 Hook Output Format Divergence (P1)

**File:** `cmd/harness/hookcli/hook_pre_tool_use.go:56-73`

Codex and Claude Code accept different JSON schemas for hook responses. The hook CLI has host-specific branches. Adding a third host requires another branch.

**Fix:** Abstract hook output into a `port.HookOutputFormatter` interface with host-specific adapters.

---

## Summary Matrix

> **Triage 2026-06-16** (evidence-verified against code/commits/tests; see the
> backlog-triage + quality-hardening workflows): of the original 24 P1/P2 items,
> **19 resolved**, **3 accepted/documented**, **3 out-of-scope/theoretical**, and
> **0 open** — including the one item (SA1) that the hardening triage itself
> surfaced and then fixed. `quality inspect`'s `audit_p1_p2_items` signal reads **0**
> (the "Open" table below has no bare `P1`/`P2` rows; the resolved/accepted/out-of-scope
> tables omit the severity column so the parser excludes them). The 7 prior
> open/partial items were closed in the 2026-06-16 hardening pass: H2/P2/C2/SA1 fixed,
> D2/L2/S2 accepted with documented rationale; M1 was later resolved on 2026-08-03
> (`034bda93` removed the legacy JSON-RPC transport).
>
> A follow-up **completeness audit** (2026-06-16) swept beyond this list and found
> 4 more: W3/W4/TC1 fixed (worker atomic write, hook-prune Close, StateUpdate test);
> the `quality inspect` **branch_candidate_functions** signal (261 over threshold 6,
> permanently `needs_attention`) is **accepted structural noise** — the counter sums
> raw branch nodes incl. every `case` arm (not cyclomatic complexity), so threshold 6
> fires on any non-trivial dispatcher; the high-complexity tier was triaged (all
> switch-dispatch routers or already well-tested, e.g. worktreeGuardBlockReason with
> 514 lines of tests). Disposition: informational, not an open work item.
>
> **Last reconciled against HEAD `116ebef` (2026-07-01).** No P1/P2 regressions since
> the 2026-06-16 triage; the recent IssueOps locking/TOCTOU and phase-ledger hardening
> (`1f7d077`, `805d622`, `b1354bd`, `878e04a`) is recorded in ISSUEOPS_AUDIT.md and
> reopens no item in this matrix.

### Open — the counted `audit_p1_p2_items`

_None. All triaged P1/P2 items are resolved or accepted-with-rationale below._

### Accepted / documented — excluded from the count (won't-fix with rationale)

| ID | Subsystem | Problem | Disposition |
|----|-----------|---------|-------------|
| D2 | Daemon | NFS lock safety (O_EXCL) | accepted — daemonlock is transient (deleted after handoff) so flock is inapplicable; O_EXCL + stale/PID detection adequate; NFS caveat documented (CAUTIONS.md §13) |
| L2 | External LLM | No retry on malformed output | accepted — decode is a pure decoder with no invoke handle; Z.AI response_format=json_object guarantees JSON; rationale comment added (structured.go); future retry belongs at the invoke layer |
| S2 | State | No atomic multi-key transactions | accepted — no caller writes 2+ keys atomically; single-key writes already temp+rename under flock; known-limitation documented (CAUTIONS.md §6) |

### Resolved — excluded from the count (verified fixed)

| ID | Subsystem | Problem | Resolved by |
|----|-----------|---------|-------------|
| H2 | Hook failure | Concurrent append > PIPE_BUF | 2026-06-16 hardening (flock the append via state.WithKeyLock; line can exceed PIPE_BUF) |
| P2 | Project state | Compact-capsule RMW lost update | 2026-06-16 hardening (state.WithKeyLock around BuildPreCompactCapsule read-merge-write — the one genuine cross-process RMW) |
| C2 | Compact | Read-delete race | f35dd6c (CAS on CreatedAt) + 2026-06-16 hardening (per-write Nonce conjunction closes the coarse-clock equality residual) |
| SA1 | Self-augment state | Snapshot write bypasses flock + atomic-rename | 2026-06-16 hardening (route WriteSelfAugmentSnapshotRecord through core.WriteStateRecord: locked temp+rename; byte-identical output) |
| W3 | Worker | Job store write not atomic (truncated job dropped on crash) | 2026-06-16 completeness audit (writeWorkerJob temp+rename; EnqueueWorkerJob now under withWorkerJobLock — lone unlocked writer) |
| W4 | Hook failure | Prune Close() error ignored before rename | 2026-06-16 completeness audit (fold tmp.Close() into writeErr so a flush failure aborts the rename) |
| TC1 | State | StateUpdate (exported cross-process RMW) untested + 0 callers | 2026-06-16 completeness audit (TestStateUpdateLockedReadModifyWrite: create / skip-sentinel / mutate / concurrent no-lost-update under -race) |
| W5 | Hook failure | Prune renames under no lock while append now locked (asymmetry) | 2026-06-16 convergence re-audit (PruneHookFailureLog now serializes under the same hook-failures WithKeyLock as the append) |
| TC2 | State | WriteStateRecord accepts a record.Key diverging from the write key | 2026-06-16 convergence re-audit (guard mirrors StateRead's key invariant + TestWriteStateRecordRejectsKeyMismatch) |
| D1 | Daemon | No connection limit | 5536cc8 (connSlots cap 64) |
| D3 | Daemon | No graceful shutdown | 5536cc8 (activeWG + 30s drain) |
| W1 | Worker | Stuck "running" jobs on crash | b89d311 (PID + isPIDAlive + DetectStuckWorkerJobs) |
| W2 | Worker | No concurrent job guard | b89d311 (per-job flock withWorkerJobLock) |
| S1 | State | No write locking | 40013f8 (withStateLock flock + StateUpdate) |
| H1 | Hook failure | Unbounded log growth | PruneHookFailureLog 720h on session start |
| P1 | Project state | Init race condition | 0800fca + a56d5e8 (O_EXCL hardlink createJSONAtomic) |
| Q1 | Draft wiki | No stale lock detection | 90409c9 (PID + staleLockMaxAge) |
| Q2 | Draft wiki | Capsule overwrite | 0cdeeb2 (merge instead of overwrite) |
| C1 | Compact | Double PreCompact overwrite | 0cdeeb2 (mergeDocUpkeepEvents) |
| CP1 | Command policy | Hardcoded catalog | resolved 2026-07-02 (`.agent-harness/policy.json` per workspace/root, warnings on load/parse failure) |
| L1 | External LLM | Single provider | accepted 2026-07-02 (Z.AI-only wrapper; unused `port.ExternalLLM` removed) |
| M2 | MCP proxy | Tool catalog drift | b8f2f11 (auto-generated DispatchMap) |
| M1 | MCP proxy | Dual transport code paths | 034bda93 (legacy hand-rolled JSON-RPC path removed; `serveMCPStreamSDK` in `mcp_sdk_server.go` serves both split stdio and bidirectional daemon connections) |
| HK1 | Hooks | Host output format divergence | ff0e668 (HostHookOutput interface) |

### Out-of-scope / theoretical — excluded from the count

| ID | Subsystem | Problem | Disposition |
|----|-----------|---------|-------------|
| N1 | Next-action | Read-write race | theoretical (so marked in detail §); no concrete loss path |
| V1 | Self-verify | Temp dir leak on kill | documentation-only; no automatic `/tmp` cleanup implemented; temp dirs are prefixed for explicit hygiene |
| CP2 | Command policy | No chained command analysis | out of scope (so marked in detail §) |

---

## Resolution Plan

### Phase A: Concurrent Safety (P1, Sequential — shared flock pattern)

| # | Task | Files |
|---|------|-------|
| A1 | Add `flock`-based write locking to `StateWrite` | `internal/core/state/state_io.go` |
| A2 | Add stale lock detection to draft wiki queue lock | `internal/core/draftwiki/queue/lock.go` |
| A3 | Add PID/heartbeat to worker jobs; detect stuck jobs | `internal/core/worker/worker.go`, `store.go` |
| A4 | Add per-job advisory lock to worker operations | `internal/core/worker/worker.go` |
| A5 | Fix project lifecycle state init race (O_EXCL create) | `internal/core/lifecycle/lifecycle_project_state_store.go` |

### Phase B: Resource Protection (P1, Parallelizable)

| # | Task | Files |
|---|------|-------|
| B1 | Daemon connection limit | `cmd/harness/daemoncli/daemon_server.go` |
| B2 | Daemon graceful shutdown | `cmd/harness/daemoncli/daemon_server.go` |
| B3 | Hook failure log pruning (`--max-age`) | `internal/core/hookfailure/log.go`, CLI |
| B4 | Compact capsule merge on double PreCompact | `internal/core/lifecycle/compact/compact.go` |

### Phase C: Extensibility (P1-P2, Parallelizable)

| # | Task | Files |
|---|------|-------|
| C1 | Generate MCP dispatch from adapter catalog | `internal/adapter/mcp/`, `cmd/harness/mcpcli/` |
| C2 | Abstract hook output format per host | `cmd/harness/hookcli/`, new `internal/adapter/hook/` |
| C3 | Load command policy from config file | `internal/core/policy/`, new config schema |
| C4 | ~~Add `port.ExternalLLM` interface~~ — REJECTED 2026-07-02: single Z.AI provider is intentional until a second real provider exists | `internal/core/externalllm/` |
| C5 | ~~Add daemon flock fallback for NFS~~ — SUPERSEDED: D2 accepted (flock inapplicable to the transient, deleted-after-handoff lock); see Accepted table + CAUTIONS.md §13 | `cmd/harness/daemoncli/daemonlock/` |

---

## Incident: Lock File Deletion Breaks Mutual Exclusion (IssueOps)

**Found during this audit, fixed 2026-06-14.**

`os.Remove(lockPath)` in `withIssueOpsLock` (added by a previous sub-agent implementation) caused `flock`-based mutual exclusion to break. `flock` locks are associated with the open file description (the inode). Deleting the lock file and recreating it via `O_CREATE` creates a new inode, so concurrent goroutines acquire locks on different inodes and run their critical sections simultaneously.

**Evidence:** `TestIssueOpsConcurrentFeedbackNoLostUpdate` (incident-time name; the lost-update coverage now lives in `TestPreCompactConcurrentMergeNoLostUpdate` and `TestStateUpdateLockedReadModifyWrite`) consistently lost 35-38 of 50 updates.

**Fix:** Reverted to `defer unix.Flock(LOCK_UN)` + `defer f.Close()` without `os.Remove`. Lock files persist. Orphaned lock files (no matching `.json`) are cleaned by the off-hot-path stale scan.
