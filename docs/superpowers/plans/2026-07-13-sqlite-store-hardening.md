# SQLite Store Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete T18 by making SQLite span waits context-cancellable and root-chain-safe, repairing exact private store modes on every open, and proving process-crash recovery plus repeated-open resource stability.

**Architecture:** `internal/core/sqlstore` remains the single owner of handle caching, SQLite lock transactions, permission repair, and active-root-chain propagation. A temporary `WithSpanContext` bridge keeps every intermediate commit buildable while lock helpers migrate package by package; the bridge is removed when the final `WithSpan(context.Context, func(context.Context) error)` API lands. Distinct-root nesting remains caller-ordered, with the existing `remote-create-live/<id> -> main IssueOps root` path explicitly propagated and regression-tested.

**Tech Stack:** Go 1.26.3, `database/sql`, `modernc.org/sqlite` v1.53.0, standard-library `context`, `os/exec`, and Go test/race/vet/build tooling.

## Global Constraints

- Use [the approved design](../specs/2026-07-13-sqlite-store-hardening-design.md) as the behavior source of truth.
- Preserve SQLite row schemas, JSON DTOs, CLI/MCP schemas, flags, exit codes, and autocommit visibility.
- Preserve one cached `*sqlstore.DB` per absolute root for the process lifetime; do not add eviction, LRU, public `Close`, or a cache-size limit.
- A canonical root may appear at most once in a propagated span chain; distinct roots are allowed only under a documented acyclic caller order.
- Preserve the existing `remote-create-live/<id>` outer root followed by the main IssueOps root; do not introduce the reverse acquisition.
- Repair only the exact store root and fixed SQLite main/sidecar path set. Do not recursively chmod, follow symlinks, chmod parent directories, or touch unrelated files.
- Use only temporary state roots in tests. Do not inspect, checkpoint, chmod, or mutate live user state.
- Build to a temporary path. Do not overwrite tracked `bin/agent-harness`.
- Keep every intermediate commit compilable and focused. Do not push without separate approval.
- Run `gofmt` on every changed Go file before its focused test command.

## Planning Contract

- **Input contract:** repository at commit `3d9ddb7`, the approved design plus ordered cross-root correction, and current production callers discovered with `rg`.
- **Output contract:** nine executable tasks with exact files, signatures, RED/GREEN commands, commit boundaries, and final verification; no production code is changed by this planning document.
- **Sanity cases:** same-root re-entry fails; `A -> B` succeeds; `A -> B -> A` fails; a cancelled local or SQLite waiter returns; remote create/reconcile still succeeds; permissive exact paths are repaired; unrelated paths are unchanged.
- **Boundary cases:** nil context/callback, callback panic cleanup, symlink/non-regular paths, holder process death, unsupported `/dev/fd`, and cached-handle permission drift.
- **Privacy/tool truth:** only repository shell tools and Go tooling are required; no hidden reasoning, invented helper, live cluster, external secret, or network write is part of execution.

## File Structure

### New focused tests

- `internal/core/sqlstore/span_context_test.go`: cancellation, active-root-chain, callback error, and panic cleanup contracts.
- `internal/core/sqlstore/permissions_test.go`: exact root/file repair and invalid path rejection.
- `internal/core/sqlstore/process_crash_test.go`: real holder/contender subprocess handshake and cleanup.
- `internal/core/sqlstore/resource_test.go`: cached-handle, connection-pool, and observational `/dev/fd` measurements.

### Core implementation

- `internal/core/sqlstore/sqlstore.go`: span API, root chain, channel gate, `BeginTx`, and open-time repair.
- `internal/core/sqlstore/maintain.go`: shared exact-file repair without changing `MaintainResult`.
- `internal/core/sqlstore/sqlstore_test.go`: final API migration for existing serialization/autocommit tests.
- `internal/core/sqlstore/maintain_test.go`: checkpoint and permission-reporting coverage.

### Context migration

- State and consumers: `internal/core/state/state_lock.go`, `internal/core/state/state_io.go`, `internal/core/state/state_test.go`, `internal/core/hookfailure/log.go`, `internal/core/hookmetrics/metrics.go`, `internal/core/hookmetrics/metrics_test.go`, `internal/core/lifecycle/compact/compact.go`, `internal/core/lifecycle/docupkeep/store.go`, `internal/core/lifecycle/docupkeep/store_test.go`.
- Loop, worker, and session: `internal/core/looprun/lifecycle.go`, `internal/core/worker/worker_lock.go`, `internal/core/worker/worker.go`, `internal/core/worker/store.go`, `internal/core/worker/read_only.go`, `internal/core/issueops/session/session.go`.
- IssueOps main-root callers: `internal/core/issueops/issueops_lock.go`, `issueops_decision.go`, `issueops_delegation.go`, `issueops_devilsadvocate_reflect.go`, `issueops_feedback.go`, `issueops_force_done.go`, `issueops_force_release.go`, `issueops_handoff_dispatch.go`, `issueops_handoff_lifecycle.go`, `issueops_handoff_plan.go`, `issueops_handoff_prepare.go`, `issueops_handoff_projection.go`, `issueops_handoff_publication.go`, `issueops_handoff_recovery.go`, `issueops_ledger_recorders.go`, `issueops_phase.go`, `issueops_regress.go`, `issueops_remote_create_claim.go`, `issueops_routing.go`, `issueops_stale_scan.go`, `issueops_stale_scan_apply_test.go`, `issueops_stale_scan_quickwin_test.go`, and `package.go` under `internal/core/issueops/`.
- Remote-create regression: `internal/core/issueops/issueops_remote_create_claim_test.go`.

### Aligned documentation

- `docs/superpowers/specs/2026-07-13-sqlite-store-hardening-design.md`: approved ordered cross-root correction.
- `.agent-harness/ADR.md`: final span-chain and remote-create lock-order decision.
- `.agent-harness/CAUTIONS.md`: root-chain and ordering guidance.
- `.agent-harness/issues/_unnumbered/agent-harness-stability-concurrency-multisession-hardening.md`: T18 evidence.

---

### Task 1: Commit the corrected design and executable plan

**Files:**
- Modify: `docs/superpowers/specs/2026-07-13-sqlite-store-hardening-design.md`
- Create: `docs/superpowers/plans/2026-07-13-sqlite-store-hardening.md`

**Interfaces:**
- Consumes: production evidence from `internal/core/issueops/issueops_remote_create_claim.go:129-289`.
- Produces: requirements consumed by Tasks 2-9.

- [ ] **Step 1: Verify the corrected contract and document integrity**

```bash
rg -n 'A -> B -> A|remote-create-live|ActiveDirs|distinct-root' docs/superpowers/specs/2026-07-13-sqlite-store-hardening-design.md
rg -n 'including cross-root|same-root and cross-root|all span nesting|OuterDir' docs/superpowers/specs/2026-07-13-sqlite-store-hardening-design.md
rg -n 'T[B]D|T[O]DO|F[I]XME|X[X]X|implement[[:space:]]+later|fill[[:space:]]+in[[:space:]]+details' docs/superpowers/specs/2026-07-13-sqlite-store-hardening-design.md docs/superpowers/plans/2026-07-13-sqlite-store-hardening.md
git diff --check
```

Expected: the first command reports the approved contract; both stale-language and marker scans have no output; diff check exits zero.

- [ ] **Step 2: Commit only the two documents**

```bash
git add -- docs/superpowers/specs/2026-07-13-sqlite-store-hardening-design.md docs/superpowers/plans/2026-07-13-sqlite-store-hardening.md
git diff --cached --check
git commit -m "docs(sqlstore): plan ordered span hardening" -m $'Lore:\n- Intent: Make T18 executable without breaking the shipped remote-create lock order.\n- Why: Blanket cross-root rejection conflicts with the remote-create-live to IssueOps chain.\n- Changes:\n  - Define active-root-chain re-entry detection and retain the documented distinct-root order.\n  - Add a TDD migration, crash, permission, resource, and verification plan.\n- Verify: Marker scan and git diff --cached --check passed.\n- Risk: Documentation only; production behavior remains unchanged.'
```

Expected: one documentation-only commit and an empty `git status --short`.

---

### Task 2: Add context-aware span behavior behind a migration bridge

**Files:**
- Modify: `internal/core/sqlstore/sqlstore.go`
- Create: `internal/core/sqlstore/span_context_test.go`

**Interfaces:**
- Consumes: existing `DB.dir`, `DB.span`, `_txlock=immediate`, and handle cache.
- Produces temporarily: `func (d *DB) WithSpanContext(context.Context, func(context.Context) error) error`.
- Produces: `type NestedSpanError struct { ActiveDirs []string; RequestedDir string }`.

- [ ] **Step 1: Write RED tests for arguments, cancellation, root chains, and cleanup**

Create these named tests in `span_context_test.go`:

```go
func TestWithSpanContextRejectsNilArguments(t *testing.T) {
	d := openTestDB(t)
	if err := d.WithSpanContext(nil, func(context.Context) error { return nil }); err == nil { t.Fatal("nil context was accepted") }
	if err := d.WithSpanContext(context.Background(), nil); err == nil { t.Fatal("nil callback was accepted") }
}

func TestWithSpanContextRejectsActiveRootReentry(t *testing.T) {
	d := openTestDB(t)
	err := d.WithSpanContext(context.Background(), func(spanCtx context.Context) error {
		return d.WithSpanContext(spanCtx, func(context.Context) error { return nil })
	})
	var nested *NestedSpanError
	if !errors.As(err, &nested) || nested.RequestedDir != d.dir || !reflect.DeepEqual(nested.ActiveDirs, []string{d.dir}) {
		t.Fatalf("nested error=%#v err=%v", nested, err)
	}
}

func TestWithSpanContextAllowsDistinctRootsAndRejectsCycle(t *testing.T) {
	a, b := openTestDB(t), openTestDB(t)
	err := a.WithSpanContext(context.Background(), func(aCtx context.Context) error {
		return b.WithSpanContext(aCtx, func(bCtx context.Context) error {
			err := a.WithSpanContext(bCtx, func(context.Context) error { return nil })
			var nested *NestedSpanError
			if !errors.As(err, &nested) || !reflect.DeepEqual(nested.ActiveDirs, []string{a.dir, b.dir}) {
				return fmt.Errorf("cycle error=%#v err=%v", nested, err)
			}
			return nil
		})
	})
	if err != nil { t.Fatal(err) }
}
```

Add the cancellation tests with explicit holder handshakes:

```go
func TestWithSpanContextCancelsLocalWaiter(t *testing.T) {
	d := openTestDB(t)
	entered, release, holderDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		holderDone <- d.WithSpanContext(context.Background(), func(context.Context) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var called atomic.Bool
	started := time.Now()
	err := d.WithSpanContext(ctx, func(context.Context) error { called.Store(true); return nil })
	if !errors.Is(err, context.Canceled) || called.Load() || time.Since(started) >= 2*time.Second {
		t.Fatalf("cancelled local wait: called=%v elapsed=%v err=%v", called.Load(), time.Since(started), err)
	}
	close(release)
	if err := <-holderDone; err != nil { t.Fatal(err) }
}

func TestWithSpanContextCancelsSQLiteWaiter(t *testing.T) {
	dir := t.TempDir()
	d1, err := newDB(dir)
	if err != nil { t.Fatal(err) }
	d2, err := newDB(dir)
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = d1.data.Close(); _ = d1.span.Close(); _ = d2.data.Close(); _ = d2.span.Close() })
	entered, release, holderDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		holderDone <- d1.WithSpanContext(context.Background(), func(context.Context) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	var called atomic.Bool
	started := time.Now()
	err = d2.WithSpanContext(ctx, func(context.Context) error { called.Store(true); return nil })
	if !errors.Is(err, context.DeadlineExceeded) || called.Load() || time.Since(started) >= 2*time.Second {
		t.Fatalf("cancelled sqlite wait: called=%v elapsed=%v err=%v", called.Load(), time.Since(started), err)
	}
	close(release)
	if err := <-holderDone; err != nil { t.Fatal(err) }
}
```

Add callback error and panic cleanup tests:

```go
func TestWithSpanContextPreservesCallbackError(t *testing.T) {
	d, want := openTestDB(t), errors.New("sentinel")
	if err := d.WithSpanContext(context.Background(), func(context.Context) error { return want }); err != want {
		t.Fatalf("callback error identity lost: %v", err)
	}
}

func TestWithSpanContextPanicReleasesGate(t *testing.T) {
	d, want := openTestDB(t), errors.New("panic sentinel")
	func() {
		defer func() {
			if got := recover(); got != want { t.Fatalf("panic=%v want=%v", got, want) }
		}()
		_ = d.WithSpanContext(context.Background(), func(context.Context) error { panic(want) })
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := d.WithSpanContext(ctx, func(context.Context) error { return nil }); err != nil { t.Fatal(err) }
}
```

- [ ] **Step 2: Run RED**

```bash
go test ./internal/core/sqlstore -run '^TestWithSpanContext' -count=1
```

Expected: compile failure because `WithSpanContext` and `NestedSpanError` do not exist.

- [ ] **Step 3: Implement the active chain and channel gate**

```go
type spanChainKey struct{}

type NestedSpanError struct {
	ActiveDirs   []string
	RequestedDir string
}

func (e *NestedSpanError) Error() string {
	return fmt.Sprintf("sqlstore nested span: root %q is already active in %v", e.RequestedDir, e.ActiveDirs)
}

type DB struct {
	dir      string
	data     *sql.DB
	span     *sql.DB
	spanGate chan struct{}
}

func newSpanGate() chan struct{} {
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	return gate
}
```

Initialize `spanGate: newSpanGate()` in `newDB`. Read the context value into a copied `[]string` and always put a copied appended chain into the callback context.

- [ ] **Step 4: Implement the temporary context method and legacy wrapper**

```go
func (d *DB) WithSpanContext(ctx context.Context, fn func(context.Context) error) error {
	if ctx == nil { return fmt.Errorf("sqlstore span context is required") }
	if fn == nil { return fmt.Errorf("sqlstore span callback is required") }
	chain, _ := ctx.Value(spanChainKey{}).([]string)
	chain = append([]string(nil), chain...)
	for _, active := range chain {
		if active == d.dir { return &NestedSpanError{ActiveDirs: chain, RequestedDir: d.dir} }
	}
	select {
	case <-ctx.Done(): return ctx.Err()
	case <-d.spanGate:
	}
	defer func() { d.spanGate <- struct{}{} }()
	tx, err := d.span.BeginTx(ctx, nil)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil { return fmt.Errorf("sqlstore span lock %s: %w", d.dir, ctxErr) }
		return fmt.Errorf("sqlstore span lock %s: %w", d.dir, err)
	}
	defer func() { _ = tx.Rollback() }()
	spanCtx := context.WithValue(ctx, spanChainKey{}, append(chain, d.dir))
	return fn(spanCtx)
}

func (d *DB) WithSpan(fn func() error) error {
	if fn == nil { return fmt.Errorf("sqlstore span callback is required") }
	return d.WithSpanContext(context.Background(), func(context.Context) error { return fn() })
}
```

The wrapper is a migration bridge and must be deleted in Task 6.

- [ ] **Step 5: Run GREEN, race, and commit**

```bash
gofmt -w internal/core/sqlstore/sqlstore.go internal/core/sqlstore/span_context_test.go
go test ./internal/core/sqlstore -run '^TestWithSpanContext|^TestWithSpanSerializes|^TestConcurrentSpans' -count=1
go test -race ./internal/core/sqlstore -run '^TestWithSpanContext|^TestConcurrentSpans' -count=1
git add -- internal/core/sqlstore/sqlstore.go internal/core/sqlstore/span_context_test.go
git diff --cached --check
git commit -m "fix(sqlstore): add cancellable root-chain spans" -m $'Lore:\n- Intent: Make local and SQLite span waits cancellable while detecting root re-entry.\n- Why: Mutex and Begin waits ignore cancellation, and root cycles can deadlock.\n- Changes:\n  - Add an ordered active-root chain and typed NestedSpanError.\n  - Replace the local mutex with a token gate and use BeginTx context cancellation.\n- Verify: Focused sqlstore and race tests pass.\n- Risk: A temporary wrapper remains until package migrations complete.'
```

---

### Task 3: Migrate state locks and cross-package consumers

**Files:**
- Modify: the nine state/consumer files listed in File Structure.

**Interfaces:**
- Consumes: `DB.WithSpanContext(ctx, fn)`.
- Produces: `WithKeyLock(ctx context.Context, dir, key string, fn func(context.Context) error) error` and matching `withStateLock`.

- [ ] **Step 1: Add RED propagation coverage**

```go
func TestWithKeyLockPropagatesActiveRoot(t *testing.T) {
	dir := t.TempDir()
	err := WithKeyLock(context.Background(), dir, "outer", func(spanCtx context.Context) error {
		db, err := openStateDB(dir)
		if err != nil { return err }
		return db.WithSpanContext(spanCtx, func(context.Context) error { return nil })
	})
	var nested *sqlstore.NestedSpanError
	if !errors.As(err, &nested) { t.Fatalf("expected NestedSpanError, got %v", err) }
}
```

Run `go test ./internal/core/state -run '^TestWithKeyLockPropagatesActiveRoot$' -count=1`; expect a signature compile failure.

- [ ] **Step 2: Change the state helper signatures**

```go
func WithKeyLock(ctx context.Context, dir, key string, fn func(context.Context) error) error {
	return withStateLock(ctx, dir, key, fn)
}

func withStateLock(ctx context.Context, dir, key string, fn func(context.Context) error) error {
	if _, err := NormalizeStateKey(key); err != nil { return err }
	db, err := openStateDB(dir)
	if err != nil { return err }
	return db.WithSpanContext(ctx, fn)
}
```

- [ ] **Step 3: Migrate every caller**

Pass an existing `ctx` when present; otherwise pass `context.Background()`. Change callbacks to `func(spanCtx context.Context) error` when they invoke context-bearing work and `func(_ context.Context) error` otherwise. Preserve all existing read/transform/write bodies and response assignments.

```bash
rg -n 'withStateLock\(|WithKeyLock\(' internal/core/state internal/core/hookfailure internal/core/hookmetrics internal/core/lifecycle
```

Expected: every call has a leading context and every callback accepts a context.

- [ ] **Step 4: Run tests and commit**

```bash
gofmt -w $(git diff --name-only -- internal/core/state internal/core/hookfailure internal/core/hookmetrics internal/core/lifecycle | rg '\.go$')
go test ./internal/core/state ./internal/core/hookfailure ./internal/core/hookmetrics ./internal/core/lifecycle/... -count=1
go test -race ./internal/core/state ./internal/core/hookfailure ./internal/core/hookmetrics ./internal/core/lifecycle/... -count=1
git add -- internal/core/state internal/core/hookfailure internal/core/hookmetrics internal/core/lifecycle
git diff --cached --check
git commit -m "refactor(state): propagate span contexts" -m $'Lore:\n- Intent: Carry sqlstore span contexts through state locks and consumers.\n- Why: Fresh backgrounds hide root re-entry and drop cancellation.\n- Changes:\n  - Make WithKeyLock and withStateLock context-bearing.\n  - Migrate state, hook, compact, and doc-upkeep callers.\n- Verify: Focused package and race suites pass.\n- Risk: Internal signatures change; external contracts do not.'
```

---

### Task 4: Migrate loop, worker, and session helpers

**Files:**
- Modify: the loop/worker/session files listed in File Structure.

**Interfaces:**
- Produces context-bearing `withLoopLock`, `withWorkerJobLock`, and `withSessionLock`, all using `func(context.Context) error`.

- [ ] **Step 1: Apply the common helper contract**

```go
func withLoopLock(ctx context.Context, loopID string, fn func(context.Context) error) error {
	if _, err := normalizeLoopID(loopID); err != nil { return err }
	db, err := openStore()
	if err != nil { return err }
	return db.WithSpanContext(ctx, fn)
}
```

Apply the same leading-context and callback contract to worker, pool, task, and session helpers. Pass `context.Background()` from context-free operations; forward an existing request context when present.

- [ ] **Step 2: Verify inventory, test, and commit**

```bash
rg -n 'withLoopLock\(|withWorkerJobLock\(|withSessionLock\(' internal/core/looprun internal/core/worker internal/core/issueops/session
gofmt -w $(git diff --name-only -- internal/core/looprun internal/core/worker internal/core/issueops/session | rg '\.go$')
go test ./internal/core/looprun ./internal/core/worker ./internal/core/issueops/session -count=1
go test -race ./internal/core/looprun ./internal/core/worker ./internal/core/issueops/session -count=1
git add -- internal/core/looprun internal/core/worker internal/core/issueops/session
git diff --cached --check
git commit -m "refactor(core): thread span contexts through stores" -m $'Lore:\n- Intent: Complete context propagation for loop, worker, and session spans.\n- Why: Every direct sqlstore caller must carry cancellation and root-chain metadata.\n- Changes:\n  - Convert three helper families to context callbacks.\n  - Migrate callers without changing external operations.\n- Verify: Focused package and race suites pass.\n- Risk: Mechanical internal signature migration.'
```

---

### Task 5: Migrate IssueOps and preserve ordered remote-create nesting

**Files:**
- Modify: all IssueOps main-root caller files and remote-create regression file listed in File Structure.

**Interfaces:**
- Produces: `withIssueOpsLock(ctx context.Context, stateRoot, id string, fn func(context.Context) error) error`.
- Produces: `withIssueOpsRemoteCreateLiveLock(ctx context.Context, stateRoot, id string, fn func(context.Context) error) error`.
- Produces context-bearing claim, clear, mark-unknown, finalize, and mutation transitions.

- [ ] **Step 1: Change the main helper and all 73 call sites**

```go
func withIssueOpsLock(ctx context.Context, stateRoot, id string, fn func(context.Context) error) error {
	if _, err := normalizeIssueOpsID(id); err != nil { return err }
	db, err := sqlstore.Open(stateRoot)
	if err != nil { return err }
	return db.WithSpanContext(ctx, fn)
}
```

At each call, use an existing lexical `ctx` or `context.Background()`. Inside callbacks, replace outer `ctx` uses with `spanCtx`. Do not change DTOs, schema versions, phase rules, or authority validation.

- [ ] **Step 2: Make remote durable transitions context-bearing**

Use these exact signatures:

```go
func ClaimIssueOpsRemoteCreate(ctx context.Context, stateRoot string, req IssueOpsRemoteCreateClaimRequest) (IssueOpsRecord, error)
func ClearIssueOpsRemoteCreateClaimPreInvocation(ctx context.Context, stateRoot string, expected IssueOpsRecord, liveClaimID string, proof *port.IssueProviderCreateError) error
func MarkIssueOpsRemoteCreateUnknown(ctx context.Context, stateRoot string, expected IssueOpsRecord, knownURL string) error
func FinalizeIssueOpsRemoteCreateClaim(ctx context.Context, stateRoot string, expected IssueOpsRecord, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error)
func mutateRemoteCreateClaim(ctx context.Context, stateRoot string, expected IssueOpsRecord, fn func(*IssueOpsRecord) error) error
```

Also add `ctx` to `clearIssueOpsRemoteCreateClaimAfterAuthoritativeZero` and the test seam `finalizeIssueOpsRemoteCreateClaimForReconcile`, including its two injected test functions.

- [ ] **Step 3: Preserve the child-root-to-main-root chain**

```go
func withIssueOpsRemoteCreateLiveLock(ctx context.Context, stateRoot, id string, fn func(context.Context) error) error {
	normalized, err := normalizeIssueOpsID(id)
	if err != nil { return err }
	db, err := sqlstore.Open(filepath.Join(stateRoot, "remote-create-live", normalized))
	if err != nil { return err }
	return db.WithSpanContext(ctx, fn)
}
```

`CreateIssueOpsRemotePullRequest` and `ReconcileIssueOpsRemoteCreate` pass their request `ctx` to the outer helper. Their callbacks pass `spanCtx` to every claim, clear, mark, finalize, publication validation, probe, and reconciliation operation accepting context.

- [ ] **Step 4: Pin ordered create and reconcile regressions while migrating tests**

Add `context.Background()` to direct transition calls in `issueops_remote_create_claim_test.go`. Extend `TestCreateRemotePullRequestProviderRequestEqualsCompleteDurableClaimProjection` after its result assertion with:

```go
persisted, readErr := ReadIssueOps(stateRoot, record.ID)
if readErr != nil { t.Fatal(readErr) }
if persisted.RemoteArtifact == nil || persisted.RemoteCreateClaim != nil {
	t.Fatalf("ordered cross-root create did not finalize: artifact=%#v claim=%#v", persisted.RemoteArtifact, persisted.RemoteCreateClaim)
}
```

Retain the existing `exactly one verified candidate finalizes` reconcile subtest; its successful completion is the second real ordered cross-root regression. These tests use the existing `publicationRefFake` and `handoffDispatchFake` fixtures and introduce no duplicate fake types.

- [ ] **Step 5: Test inventory and commit**

```bash
gofmt -w $(git diff --name-only -- internal/core/issueops | rg '\.go$')
rg -n 'withIssueOpsLock\(' internal/core/issueops --glob '*.go'
rg -n 'withIssueOpsRemoteCreateLiveLock\(' internal/core/issueops --glob '*.go'
go test ./internal/core/issueops/... -count=1
go test -race ./internal/core/issueops/... -count=1
git add -- internal/core/issueops
git diff --cached --check
git commit -m "fix(issueops): preserve ordered span contexts" -m $'Lore:\n- Intent: Propagate cancellation and root chains through IssueOps spans.\n- Why: Remote create intentionally acquires its child live root before the main root.\n- Changes:\n  - Convert main and remote lock helpers plus durable transitions to contexts.\n  - Preserve the ordered path with regression coverage.\n- Verify: Full IssueOps and race suites pass.\n- Risk: Broad internal API migration; external schemas remain unchanged.'
```

Expected: all 73 main-root calls carry context; the helper definition is the 74th `rg` match; the remote helper has two production callers plus its definition; IssueOps tests pass.

---

### Task 6: Cut over to final `WithSpan(ctx, fn)`

**Files:**
- Modify: `internal/core/sqlstore/sqlstore.go`
- Modify: `internal/core/sqlstore/sqlstore_test.go`
- Modify: `internal/core/sqlstore/span_context_test.go`
- Modify: the seven production helper files containing the eight direct calls.

**Interfaces:**
- Consumes: migrations from Tasks 3-5.
- Produces finally: `func (d *DB) WithSpan(context.Context, func(context.Context) error) error`.
- Removes: temporary `WithSpanContext` and legacy `WithSpan(func() error)`.

- [ ] **Step 1: Rename the implementation and remove the bridge**

Rename `WithSpanContext` to `WithSpan`, delete the legacy wrapper, and replace the method comment with:

```go
// WithSpan serializes a read-modify-write span for one root, propagates the
// ordered active-root chain, and makes both local and SQLite lock waits obey ctx.
```

- [ ] **Step 2: Update every direct call**

Replace `WithSpanContext(` with `WithSpan(` in the new context tests and these production helpers:

- `internal/core/state/state_lock.go`
- `internal/core/looprun/lifecycle.go`
- `internal/core/worker/worker_lock.go`
- `internal/core/issueops/session/session.go`
- `internal/core/issueops/issueops_lock.go`
- `internal/core/issueops/issueops_remote_create_claim.go`

Migrate the existing callbacks in `sqlstore_test.go` to `context.Background()` and `func(_ context.Context) error`.

- [ ] **Step 3: Prove no compatibility path remains**

```bash
rg -n 'WithSpanContext|WithSpan\(func\(' internal --glob '*.go'
rg -n '\.WithSpan\(' internal --glob '*.go' --glob '!**/*_test.go'
```

Expected: the first command has no output; the second reports exactly eight production calls in seven files.

- [ ] **Step 4: Test and commit the cutover**

```bash
gofmt -w $(git diff --name-only -- internal/core/sqlstore internal/core/state internal/core/looprun internal/core/worker internal/core/issueops | rg '\.go$')
go test ./internal/core/sqlstore ./internal/core/state ./internal/core/issueops/... ./internal/core/looprun ./internal/core/worker -count=1
go test -race ./internal/core/sqlstore ./internal/core/state ./internal/core/issueops/... ./internal/core/looprun ./internal/core/worker -count=1
git add -- internal/core/sqlstore internal/core/state internal/core/looprun internal/core/worker internal/core/issueops
git diff --cached --check
git commit -m "refactor(sqlstore): complete context span cutover" -m $'Lore:\n- Intent: Make the context callback the only sqlstore span API.\n- Why: The bridge would preserve an unsafe context-dropping path after migrations.\n- Changes:\n  - Rename WithSpanContext to final WithSpan.\n  - Remove the wrapper and migrate production and test calls.\n- Verify: Migrated package and race suites pass; legacy scans are empty.\n- Risk: Intentional breaking change limited to internal callers.'
```

---

### Task 7: Repair exact private modes during every open

**Files:**
- Modify: `internal/core/sqlstore/sqlstore.go`
- Modify: `internal/core/sqlstore/maintain.go`
- Modify: `internal/core/sqlstore/maintain_test.go`
- Create: `internal/core/sqlstore/permissions_test.go`

**Interfaces:**
- Produces: `ensurePrivateRoot(string) error`.
- Produces: `repairPrivateSQLiteFiles(string) ([]string, error)` returning repaired basenames.
- Preserves: `MaintainResult` JSON and checkpoint behavior.

- [ ] **Step 1: Write RED path and permission tests**

Create:

- `TestOpenRepairsPermissiveRootAndKnownFiles`: pre-create root `0755`, open it, drift root and every existing known SQLite file, cached-open it, assert root `0700`, known files `0600`, unrelated file unchanged.
- `TestOpenRejectsSymlinkRootWithoutChangingTarget`: assert failure and unchanged target mode.
- `TestOpenRejectsSymlinkKnownFileWithoutChangingTarget`: symlink `harness.db`, assert failure and unchanged target.
- `TestOpenRejectsNonRegularKnownFile`: place a directory at `harness.db`, assert path-specific failure.
- `TestMaintainUsesExactPermissionRepair`: assert only known repaired basenames are reported and unrelated mode is unchanged.

```bash
go test ./internal/core/sqlstore -run '^TestOpenRepairs|^TestOpenRejects|^TestMaintainUsesExact' -count=1
```

Expected: permissive root, cached drift, and invalid-path cases fail against current code.

- [ ] **Step 2: Implement the exact root helper**

```go
func ensurePrivateRoot(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil { return fmt.Errorf("sqlstore create root %s: %w", dir, err) }
	info, err := os.Lstat(dir)
	if err != nil { return fmt.Errorf("sqlstore inspect root %s: %w", dir, err) }
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() { return fmt.Errorf("sqlstore root %s is not a real directory", dir) }
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil { return fmt.Errorf("sqlstore chmod root %s: %w", dir, err) }
	}
	return nil
}
```

- [ ] **Step 3: Implement fixed-file validation and repair**

Define `var sqliteFileSuffixes = [...]string{"", "-wal", "-shm", "-journal"}`. `repairPrivateSQLiteFiles` iterates `dataDBFile` and `spanDBFile` times those suffixes. It skips only `os.IsNotExist`, returns other `Lstat` errors, rejects symlink/non-regular entries, chmods only differing modes, and returns basenames in deterministic iteration order.

Before `touchPrivate` opens an existing path, `Lstat` it and reject symlink/non-regular entries. Keep `O_CREATE|O_RDWR`, `0600`, explicit `f.Chmod(0600)`, and close handling.

- [ ] **Step 4: Wire repair into new and cached opens**

In `Open`, while holding `handlesMu`, call `ensurePrivateRoot(abs)` and `repairPrivateSQLiteFiles(abs)` before returning a cached handle.

In `newDB`, call root repair first, initialize both databases, then call fixed-file repair after both schema operations. On final repair error close both handles and return the path-specific error. Do not checkpoint from `Open`.

- [ ] **Step 5: Reuse file repair in `Maintain`**

```go
fixed, err := repairPrivateSQLiteFiles(d.dir)
if err != nil { return result, fmt.Errorf("sqlstore maintain permissions %s: %w", d.dir, err) }
result.PermissionsFixed = fixed
```

Do not report root chmod or add a result field.

- [ ] **Step 6: Test and commit permission hardening**

```bash
gofmt -w internal/core/sqlstore/sqlstore.go internal/core/sqlstore/maintain.go internal/core/sqlstore/maintain_test.go internal/core/sqlstore/permissions_test.go
go test ./internal/core/sqlstore -count=1
go test -race ./internal/core/sqlstore -count=1
git add -- internal/core/sqlstore
git diff --cached --check
git commit -m "fix(sqlstore): repair exact private store modes" -m $'Lore:\n- Intent: Guarantee private root and SQLite modes whenever Open returns.\n- Why: Existing roots and sidecars can drift between maintenance passes.\n- Changes:\n  - Validate and chmod the exact root and fixed SQLite paths.\n  - Reuse file repair in Maintain and reject invalid owned paths.\n- Verify: Full sqlstore and race suites pass.\n- Risk: Open now fails closed on invalid paths.'
```

---

### Task 8: Prove process-crash recovery

**Files:**
- Create: `internal/core/sqlstore/process_crash_test.go`

**Interfaces:**
- Consumes: final `WithSpan(ctx, fn)` and temporary roots.
- Produces: test-only `locked`, `attempting`, and `acquired` subprocess protocol.

- [ ] **Step 1: Implement single-wait child ownership**

Define `sqlstoreHelperProcess` with `cmd *exec.Cmd`, marker channel, captured stderr, `done chan struct{}`, mutex, and wait error. `startSQLStoreHelper` must call `StdoutPipe`, start, launch exactly one `cmd.Wait` goroutine, scan newline markers, and register cleanup immediately. Cleanup kills only a running child and waits for `done`; it never calls `Wait` twice.

- [ ] **Step 2: Implement holder and contender modes**

```go
switch os.Getenv("HARNESS_SQLSTORE_PROCESS_HELPER") {
case "holder":
	d, err := Open(os.Getenv("HARNESS_SQLSTORE_PROCESS_DIR"))
	if err != nil { t.Fatal(err) }
	err = d.WithSpan(context.Background(), func(context.Context) error {
		fmt.Fprintln(os.Stdout, "locked")
		select {}
	})
	t.Fatal(err)
case "contender":
	d, err := Open(os.Getenv("HARNESS_SQLSTORE_PROCESS_DIR"))
	if err != nil { t.Fatal(err) }
	fmt.Fprintln(os.Stdout, "attempting")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := d.WithSpan(ctx, func(context.Context) error {
		fmt.Fprintln(os.Stdout, "acquired")
		return nil
	}); err != nil { t.Fatal(err) }
default:
	t.Skip("subprocess helper only")
}
```

- [ ] **Step 3: Implement the bounded parent handshake**

`TestWithSpanRecoversAfterHolderProcessIsKilled` must wait five seconds for holder `locked`, start contender, wait five seconds for `attempting`, observe for 200ms that `acquired` has not arrived, kill-and-wait holder, then require the same contender's `acquired` within five seconds and nil process exit. Include captured stderr in every failure. The 200ms check is supplemental; exact markers establish state.

- [ ] **Step 4: Repeat, inspect hygiene, and commit**

```bash
gofmt -w internal/core/sqlstore/process_crash_test.go
go test ./internal/core/sqlstore -run '^TestWithSpanRecoversAfterHolderProcessIsKilled$' -count=10
go test -race ./internal/core/sqlstore -run '^TestWithSpanRecoversAfterHolderProcessIsKilled$' -count=3
ps -axo pid,ppid,etime,command | rg 'TestWithSpanProcessHelper|HARNESS_SQLSTORE_PROCESS_HELPER' || true
git add -- internal/core/sqlstore/process_crash_test.go
git diff --cached --check
git commit -m "test(sqlstore): prove process crash lock recovery" -m $'Lore:\n- Intent: Prove a killed span holder cannot strand later processes.\n- Why: Goroutine tests do not exercise process death or driver teardown.\n- Changes:\n  - Add exact subprocess markers and bounded handshakes.\n  - Kill and wait every child on all paths.\n- Verify: Repeated crash and race tests pass with no helper left.\n- Risk: Test-only temporary state and bounded process timing.'
```

Expected: every repetition passes and no helper process survives.

---

### Task 9: Measure resources, align docs, and run final gates

**Files:**
- Create: `internal/core/sqlstore/resource_test.go`
- Modify: `.agent-harness/ADR.md`
- Modify: `.agent-harness/CAUTIONS.md`
- Modify: `.agent-harness/issues/_unnumbered/agent-harness-stability-concurrency-multisession-hardening.md`

**Interfaces:**
- Consumes: `handles`, `DB.data`, `DB.span`, and final Tasks 2-8 behavior.
- Produces: deterministic handle/pool checks, observational `/dev/fd`, and final T18 evidence.

- [ ] **Step 1: Add repeated-open measurement**

```go
func TestRepeatedOpenKeepsHandleAndConnectionCountsStable(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil { t.Fatal(err) }
	if err := d.WithSpan(context.Background(), func(context.Context) error { return nil }); err != nil { t.Fatal(err) }
	if _, _, err := d.Get("resource", "missing"); err != nil { t.Fatal(err) }
	handlesMu.Lock(); handlesBefore := len(handles); handlesMu.Unlock()
	dataBefore, spanBefore := d.data.Stats().OpenConnections, d.span.Stats().OpenConnections
	fdBefore, fdOK := readableFDCount()
	for range 200 {
		again, err := Open(dir)
		if err != nil { t.Fatal(err) }
		if again != d { t.Fatal("Open returned a different cached handle") }
		if _, _, err := again.Get("resource", "missing"); err != nil { t.Fatal(err) }
	}
	handlesMu.Lock(); handlesAfter := len(handles); handlesMu.Unlock()
	if handlesAfter != handlesBefore { t.Fatalf("handles: %d -> %d", handlesBefore, handlesAfter) }
	if got := d.data.Stats().OpenConnections; got != dataBefore { t.Fatalf("data: %d -> %d", dataBefore, got) }
	if got := d.span.Stats().OpenConnections; got != spanBefore { t.Fatalf("span: %d -> %d", spanBefore, got) }
	if fdAfter, ok := readableFDCount(); fdOK && ok { t.Logf("/dev/fd: before=%d after=%d delta=%d", fdBefore, fdAfter, fdAfter-fdBefore) }
}
```

`readableFDCount` reads `/dev/fd` and returns `(0, false)` when unsupported. OS FD delta is not a deterministic failure because the shared test process owns unrelated descriptors.

- [ ] **Step 2: Run resource tests**

```bash
gofmt -w internal/core/sqlstore/resource_test.go
go test ./internal/core/sqlstore -run '^TestRepeatedOpenKeepsHandleAndConnectionCountsStable$' -count=10 -v
go test -race ./internal/core/sqlstore -count=1
```

Expected: handle and warmed connection counts remain equal; supported systems log FD observations.

- [ ] **Step 3: Align ADR, cautions, and T18 evidence**

Record that one root may appear once per propagated chain, same-root/cyclic re-entry returns `NestedSpanError`, distinct roots require documented acyclic ordering, and the retained production order is remote-create child root then main root. Replace the obsolete blanket no-nesting caution. In T18, cite exact test names and measured permission, cancellation, chain, crash, and repeated-open results; do not claim an OS-wide FD bound.

- [ ] **Step 4: Run final serialized verification**

```bash
go test ./internal/core/sqlstore ./internal/core/state ./internal/core/issueops/... ./internal/core/looprun ./internal/core/worker -count=1
go test -p 1 -timeout 20m ./... -count=1
go test -race -p 1 -timeout 20m ./... -count=1
go vet ./...
tmp_dir="$(mktemp -d)"
go build -o "$tmp_dir/agent-harness" ./cmd/harness
rm -rf "$tmp_dir"
git diff --check
python3 skills/atomic-commit-push/scripts/api_doc_gate.py .
```

Expected: every command exits zero, API gate has no candidate files, and tracked `bin/agent-harness` is unchanged.

- [ ] **Step 5: Verify final invariants and hygiene**

```bash
rg -n 'WithSpanContext|WithSpan\(func\(' internal --glob '*.go'
rg -n '\.WithSpan\(' internal --glob '*.go' --glob '!**/*_test.go'
git diff -- bin/agent-harness
ps -axo pid,ppid,etime,command | rg 'TestWithSpanProcessHelper|HARNESS_SQLSTORE_PROCESS_HELPER' || true
```

Expected: no legacy API; exactly eight production calls; no binary diff; no helper child.

- [ ] **Step 6: Commit evidence and docs**

```bash
git add -- internal/core/sqlstore/resource_test.go .agent-harness/ADR.md .agent-harness/CAUTIONS.md .agent-harness/issues/_unnumbered/agent-harness-stability-concurrency-multisession-hardening.md
git diff --cached --check
git commit -m "docs(sqlstore): record T18 hardening evidence" -m $'Lore:\n- Intent: Close T18 with measured resources and aligned lock-order guidance.\n- Why: Docs must match root-chain, permission, cancellation, and crash contracts.\n- Changes:\n  - Add deterministic handle and pool measurements with FD logging.\n  - Record verified evidence in ADR, cautions, and the hardening plan.\n- Verify: Focused, full, race, vet, build, hygiene, and API gates pass.\n- Risk: Test and docs only; eviction stays out of scope.'
```

- [ ] **Step 7: Confirm local-only completion**

```bash
git status --short --branch
git log --oneline --decorate -10
git rev-list --left-right --count origin/main...HEAD
```

Expected: clean tree, local commits ahead of `origin/main`, and no push.

## Final Acceptance Checklist

- [ ] `Open` enforces exact root `0700` and existing known SQLite files `0600` before return.
- [ ] Invalid owned paths fail closed without changing symlink targets or unrelated files.
- [ ] Local and SQLite waiters obey cancellation and never enter cancelled callbacks.
- [ ] Same-root and `A -> B -> A` re-entry fail with `*NestedSpanError` and a defensive chain.
- [ ] Documented `A -> B` succeeds; remote create/reconcile preserves child-root-to-main-root order.
- [ ] Callback error identity and panic cleanup remain intact.
- [ ] A killed holder releases the lock to the already-waiting contender within the deadline.
- [ ] Repeated open preserves handle identity and warmed connection counts; FD is logged when supported.
- [ ] All eight production direct callers use `WithSpan(ctx, fn)` and no bridge remains.
- [ ] Focused, full, race, vet, temporary build, API, and hygiene gates pass.
- [ ] Working tree is clean and no push or live-state mutation occurred.
