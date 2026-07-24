# SQLite Store Hardening Design

**Date:** 2026-07-13
**Scope:** `.agent-harness/plans/agent-harness-stability-concurrency-multisession-hardening.md` T18 remaining work
**Status:** written spec approved with ordered cross-root correction; implementation planning in progress

## 1. Purpose

`internal/core/sqlstore` already provides per-root SQLite storage, cross-process span serialization, WAL maintenance, and a process-lifetime handle cache. T18 identified four remaining contract gaps:

1. `Open` repairs the main database files but does not guarantee an existing root is `0700` or repair every existing SQLite sidecar before returning.
2. `WithSpan` waits through a non-cancellable `sync.Mutex` and `Begin()`, so callers cannot stop either local or SQLite lock contention.
3. A nested span blocks forever instead of returning a typed contract error.
4. Cross-process crash recovery and process-lifetime FD behavior are described but not proven by real subprocess and measurement tests.

This change hardens those four boundaries without changing row schemas, CLI/MCP response shapes, autocommit visibility, or the process-lifetime handle-cache policy.

## 2. Goals

- When `Open` returns successfully, the exact store root is `0700` and every known SQLite file that exists at that moment is a regular `0600` file.
- Make local in-process span contention and SQLite `BEGIN IMMEDIATE` contention cancellable through one `context.Context`.
- Reject a span when its canonical root already appears in the active span chain, including `A -> B -> A`, with a typed error before waiting.
- Preserve the existing ordered cross-root production path from `remote-create-live/<id>` to the main IssueOps root.
- Migrate all eight production `WithSpan` call sites to the context-bearing callback contract.
- Prove holder death releases the SQLite span lock by using real helper subprocesses and bounded handshakes.
- Measure repeated-open connection/FD behavior without adding eviction, close, or cache-size policy.

## 3. Non-goals

- No SQLite row, bucket, schema, or serialization change.
- No CLI, MCP, hook, or response-contract field change.
- No conversion of data writes to a transaction; writes inside a span remain autocommit and immediately visible.
- No handle eviction, LRU, explicit public `Close`, or cache-size limit.
- No recursive chmod, parent-directory chmod, unrelated-file chmod, `VACUUM`, or live user-state maintenance.
- No goroutine-ID extraction, `runtime.Stack` ownership inference, or fail-fast behavior for legitimate non-nested contention.
- No global lock-order registry or inference of semantic root priority inside `sqlstore`; distinct-root ordering remains an explicit caller contract.
- No general filesystem symlink race solution. Paths observed as symlinks at validation time are rejected; platform-specific `O_NOFOLLOW` work is outside this scope.

## 4. Preserved Invariants

- One absolute state root maps to one cached `*DB` per process.
- Spans serialize per root in-process and through `harness.lock.db` across processes.
- A root may appear at most once in one propagated span context chain. Distinct roots may nest only under a documented acyclic caller order.
- The retained production cross-root order is `remote-create-live/<id>` outer span followed by the main IssueOps-root span; the reverse order is not introduced.
- The SQLite transaction is used only as the span lock; data writes continue through `harness.db` autocommit operations.
- Callback errors retain their identity.
- Transaction rollback and local-gate release run on every normal callback return path.
- A callback panic is not recovered; deferred rollback and local-gate release still run while the panic unwinds.
- Existing fixed-root and project-store maintenance discovery remains unchanged.
- The handle map remains process-lifetime state.

## 5. Span API

### 5.1 Public package contract

Replace the current callback signature with:

```go
func (d *DB) WithSpan(
	ctx context.Context,
	fn func(context.Context) error,
) error
```

`ctx` and `fn` are required. A nil argument returns an argument error before lock acquisition.

The callback receives `spanCtx`, a child context containing an unexported ordered chain of canonical active roots. Before waiting, `WithSpan` checks the requested root against the whole chain. If that root is already active, it returns `*NestedSpanError` immediately. This rejects same-root re-entry and cycles such as `A -> B -> A` while allowing an explicitly ordered distinct-root path such as the existing `remote-create-live/<id> -> main IssueOps root` flow.

```go
type NestedSpanError struct {
	ActiveDirs   []string
	RequestedDir string
}
```

`NestedSpanError` implements `error`. `ActiveDirs` is a defensive copy in outer-to-inner order. Callers and tests identify the error with `errors.As`; no error-string matching is required.

For a distinct requested root, `WithSpan` appends that canonical root to a copied chain before invoking the callback. `sqlstore` does not decide whether unrelated roots have a valid semantic order. The caller that introduces a cross-root path must document one acyclic direction and add a regression test. The current scope retains only the already-shipped remote-create child-root-to-main-root direction.

### 5.2 Context-cancellable local gate

Replace `DB.mu sync.Mutex` with a private capacity-one token channel initialized by `newDB`. Acquisition is:

```go
select {
case <-ctx.Done():
	return ctx.Err()
case <-d.spanGate:
}
```

The token is returned with `defer` after acquisition. This keeps ordinary goroutines queued while allowing a cancelled waiter to return without entering the callback. It does not convert legitimate contention into a busy error.

### 5.3 Context-cancellable SQLite gate

After acquiring the local token, use:

```go
tx, err := d.span.BeginTx(ctx, nil)
```

The configured `_txlock=immediate` remains unchanged. modernc SQLite's `BeginTx` passes the context into the `BEGIN IMMEDIATE` execution and interrupts the connection when the context is cancelled.

If `BeginTx` returns after cancellation, `WithSpan` wraps `ctx.Err()` so `errors.Is` matches `context.Canceled` or `context.DeadlineExceeded`. Non-cancellation SQLite errors remain wrapped with the store directory. The transaction is rolled back after the callback exactly as before.

### 5.4 Caller migration

The eight production call sites in these seven files migrate in the same change:

- `internal/core/worker/worker_lock.go`
- `internal/core/state/state_lock.go`
- `internal/core/looprun/lifecycle.go`
- `internal/core/issueops/session/session.go`
- `internal/core/issueops/issueops_lock.go`
- `internal/core/issueops/issueops_remote_create_claim.go`

Each lock helper accepts a context and a `func(context.Context) error` callback. The outermost production operation uses its existing request context when one exists; an operation with no context-bearing surface uses `context.Background()`. Inside a locked callback, code uses the supplied `spanCtx` for any context-bearing work and must forward it to any attempted span acquisition. This propagation is what exposes root re-entry to `NestedSpanError` instead of hiding it behind a fresh background context.

`CreateIssueOpsRemotePullRequest` and `ReconcileIssueOpsRemoteCreate` pass the outer `remote-create-live/<id>` `spanCtx` through their claim, clear, mark-unknown, and finalize transitions. Those transitions acquire the distinct main IssueOps root and therefore remain allowed. Regression tests must prove the existing create and reconcile flows still complete and that no main-root-to-remote-create reverse acquisition is introduced.

The migration is internal to this repository. It may change unexported helper signatures and `internal/` package APIs, but it must not change CLI flags, MCP schemas, JSON DTOs, or exit-code behavior.

## 6. Open-time Private Modes

### 6.1 Exact path set

Permission repair is bounded to:

- the exact absolute store root: `0700`
- `harness.db`: `0600`
- `harness.lock.db`: `0600`
- `-wal`, `-shm`, and `-journal` sidecars for both database files: `0600`

No directory traversal or wildcard discovery is used. An unrelated file in the same root retains its original mode.

### 6.2 Root validation

The root helper performs these steps:

1. `MkdirAll(root, 0700)` for an absent root.
2. `Lstat(root)` and reject a symlink or non-directory.
3. `Chmod(root, 0700)`.

Only the exact root is changed; its parent and children other than the fixed SQLite path set are untouched.

### 6.3 File validation and repair

For each fixed SQLite path:

- missing is allowed;
- a symlink or non-regular file is an error;
- a regular file is changed to `0600` only when needed.

`touchPrivate` validates an existing main DB path before opening it. `newDB` repairs root/main-file modes before SQLite open, initializes both schemas, then repairs the exact path set again so sidecars created during open are covered.

`Open` also runs the exact repair before returning a cached handle. Therefore a mode drift introduced after the first `Open` is corrected by the next `Open` without requiring maintenance.

The guarantee covers files present when `Open` returns. A sidecar created later by a new SQLite connection is repaired by the next `Open` or `Maintain`; the main-file mode still provides the driver's normal inheritance baseline.

### 6.4 Maintain reuse

`Maintain` reuses the exact-file permission helper and continues to report repaired file basenames in `PermissionsFixed`. Root-mode repair is an `Open` invariant and does not add a new response field or synthetic basename to `MaintainResult`.

## 7. Process-crash Contract Test

The test binary serves as a helper subprocess selected by a dedicated environment variable and `-test.run` target. It has holder and contender modes.

1. The holder opens the temp store, enters `WithSpan`, and emits `locked` only after both local and SQLite locks are held.
2. The contender emits `attempting` immediately before `WithSpan`, then emits `acquired` from inside the callback.
3. The parent waits for `locked`, starts the contender, and waits for `attempting`.
4. A short pre-kill observation verifies the contender has not already emitted `acquired`. This is supplemental evidence, not the only synchronization mechanism.
5. The parent kills and waits for the holder process.
6. The same contender must emit `acquired` and exit successfully within a bounded timeout.

Every subprocess has explicit kill-and-wait cleanup registered before assertions that can fail. The test uses only a temporary store. It never opens or checkpoints `~/.local/state/agent-harness`.

## 8. FD-growth Measurement

The FD test warms one temporary absolute root, records the cached handle and both `sql.DB.Stats().OpenConnections` values, then repeats `Open` and a lightweight read many times.

Required assertions:

- every repeated `Open` returns the same `*DB`;
- data and span `OpenConnections` do not grow after warm-up;
- the number of entries in the process-lifetime handle map grows by zero for the repeated root.

When `/dev/fd` is readable, the test also records before/after OS FD counts with `t.Logf`. OS FD count is observational because the Go test process can own unrelated descriptors; the stable cached-handle and pool statistics are the deterministic gate.

The test does not open hundreds of unique roots and does not introduce eviction. Unexpected growth fails with the measured handle and pool statistics; it does not trigger an automatic production-policy change.

## 9. Error and Cleanup Matrix

| Stage | Failure | Required result |
|---|---|---|
| argument validation | nil context/callback | argument error, no gate acquisition |
| nested validation | requested root already appears in active chain | `*NestedSpanError`, no waiting |
| ordered cross-root validation | requested root differs from every active root | append root to copied chain and continue |
| local gate | context cancelled | `ctx.Err()`, callback not called |
| SQLite gate | context cancelled/busy interrupted | error wraps `ctx.Err()`, local token returned |
| SQLite gate | other driver error | error includes requested directory |
| callback | callback returns error | exact callback error identity retained |
| callback completes | success | rollback releases SQLite lock, local token returned |
| callback panics | panic unwinds | panic preserved, rollback attempted, local token returned |
| root/file repair | symlink/non-regular path | path-specific error, external target unchanged |
| chmod | permission failure | path-specific error, `Open` fails closed |
| helper process | timeout/test failure | all started children kill/wait cleaned |

## 10. TDD Sequence

1. Add RED tests for the new `WithSpan` signature, local cancellation, SQLite cancellation, typed root re-entry rejection, allowed distinct-root nesting, and permissive-mode repair.
2. Implement the context gate, `NestedSpanError`, and `BeginTx(ctx)` path.
3. Migrate all production direct callers and compile-test each affected package.
4. Implement exact root/file validation and repair; reuse it from `Maintain`.
5. Add the real holder/contender crash-recovery test.
6. Add repeated-open handle/pool/FD measurement.
7. Update ADR, CAUTIONS, and T18 evidence only after behavior is verified.

The crash-recovery and FD tests may be characterization tests that pass against part of the current implementation. The change still follows TDD because the new API, cancellation, root-chain error, and open-time permission tests fail before production code changes.

## 11. Verification

Focused verification must cover sqlstore plus every migrated production caller:

```bash
go test ./internal/core/sqlstore ./internal/core/state ./internal/core/issueops/... ./internal/core/looprun ./internal/core/worker -count=1
```

Final verification uses serialized full-suite commands because this repository's broad parallel suite can exceed local memory independently of the changed packages:

```bash
go test -p 1 -timeout 20m ./... -count=1
go test -race -p 1 -timeout 20m ./... -count=1
go vet ./...
tmp_bin="$(mktemp -d)/agent-harness" && go build -o "$tmp_bin" ./cmd/harness
git diff --check
```

No command may target live user state, overwrite tracked `bin/agent-harness`, or push without a separate explicit request.

## 12. Acceptance Criteria

- A permissive existing root is `0700` after `Open`; existing known SQLite files are `0600`; an unrelated file is unchanged.
- A symlink root or known SQLite file is rejected without chmod of its target.
- A local waiter returns on context cancellation without entering its callback.
- An independent handle waiting on SQLite returns on context cancellation without waiting for the 60-second busy timeout.
- Same-root re-entry and `A -> B -> A` cycles return `*NestedSpanError` immediately; a documented `A -> B` distinct-root chain completes.
- Supervised remote create and reconcile retain the existing `remote-create-live/<id> -> main IssueOps root` acquisition order and pass their focused regression tests.
- Existing concurrent spans still serialize and enter after release when their context remains active.
- A killed real holder process releases the lock and an already-waiting contender acquires it within the test deadline.
- Repeated `Open` of one root preserves handle identity and stable data/span connection counts; OS FD observations are recorded when supported.
- Existing state, IssueOps, session, loop, and worker behavior remains green.
- Full tests, race tests, vet, and temporary binary build pass.

## 13. Rollback

The behavior change is isolated to `internal/core/sqlstore`, its direct callers, tests, and aligned project documentation. If the context API migration causes an unanticipated regression, revert the behavior commit and its caller-migration commit together; the design document can remain as rejected/rolled-back history. No persistent data migration or schema rollback is required.
