# IssueOps Handoff Hardening: Invariant Consolidation, Generic Reconcile, Parser Extraction, Cross-Process Lock Tests, Threat-Model Docs

## TL;DR
> Summary:      Consolidate the handoff subsystem's triple-enforced invariants into single declarative sources, generalize the exact-one-candidate reconcile principle, extract command parsing out of the authority layer, add real multi-process lock contention tests, and promote the handoff threat model into ARCHITECTURE.md — without changing any fail-closed semantics.
> Origin:       2026-07-15 full-repo analysis + handoff deep-dive (normal-path and failure-path/bug-history reports). Bug history shows three recurring fix clusters: lock-mechanism generation changes (flock→advisory→sqlstore span, regression each time), schema-version bumps destroying lease fields (v1→v2→v3), and per-artifact re-application of the exact-one-candidate rule (worktree→terminal→task→dispatch→runtime).
> Deliverables:
> - Task A: declarative state×role×command authority table shared by envelope validation, hook authority, and lifecycle execution checks.
> - Task B: generic "baseline+marker → exactly-one" reconcile matcher replacing five per-artifact implementations.
> - Task C: command parsing/security-filter helpers extracted from `lifecycle_handoff_authority.go` into the existing `internal/core/commandparse` package.
> - Task D: real subprocess-based cross-process lock contention tests for sqlstore span / issueops cycle lock.
> - Task E: handoff threat-model + invariants section in `.agent-harness/ARCHITECTURE.md`, cross-linked from CAUTIONS.md.
> Effort:       Large (5 independent tasks, each individually shippable)
> Risk:         Medium — refactors touch the most fail-closed-critical files in the repo; mitigated by behavior-freeze tests before each move and per-task atomic commits.

## Evidence base (why these five, with anchors)

Complexity hotspots (4 of repo top-5 files are handoff-family):
- `internal/core/lifecycle/lifecycle_handoff_authority.go` — 1441 LOC (hook PreToolUse enforcement, default-deny)
- `internal/core/issueops/issueops_handoff_dispatch.go` — 1234 LOC
- `internal/core/issueops/issueops_handoff_recovery.go` — 1185 LOC
- `internal/core/issueops/handoff/envelope.go` — 943 LOC (`validateHandoffExternalStringBounds` alone ~227 LOC)

Root cause of bloat (deep-dive conclusion): the same state+fence+session+cwd invariants are re-implemented in three layers —
(a) storage-time `handoff.ValidateEnvelope` (envelope.go:35) via `fencedCopy` (state.go:331),
(b) hook-time authority (`coordinatorLifecycleStateAllows` authority.go:257, `mcpFenceMatches` :1421, `mcpEventIdentityMatches` :1429),
(c) execution-time lifecycle checks (`validateHandoffClaimIdentity` lifecycle.go:131, result-identity validation on finish/accept).

Recurring-bug clusters from `git log --follow` mining:
- Cluster C (lock scope/TOCTOU): 9374205 → 07ebc31 (revert: lockfile deletion broke flock mutual exclusion) → 1f7d077 → 2ad4246 → 653bcbf/64429bb (sqlstore span) → 3c04b2d → 9d262d2. Three lock generations, regression at each transition.
- Cluster D (lease/schema): a0a30ef → c6cf42e → 67bda37 (v1 decoder silently destroyed lease fields → root schema v2) → 14b051c (v3). Same failure class per version bump.
- Clusters A/B (crash fencing + identity): exact-one-candidate narrowing re-implemented per artifact kind; unidentified-inventory-row-as-absence bug recurred in force-abandon (fixed by `requireStableInventoryIdentities` recovery.go:310).

Test coverage today: handoff-family tests ≈ 10,471 LOC. Known gaps: real OS-level multi-process lock contention (current tests inject competitors via `BeforeLockedRevalidation` hooks, single-process), rollover×cancel interleaving, cleanup-receipt mid-sequence failure resume.

## Scope

### Must have
- Task A (invariant table): a single declarative module under `internal/core/issueops/handoff/` mapping (state × role × command/tool surface) → allowed, with fence/identity predicates as named reusable checks. All three layers consult the same table; authority keeps its default-deny wrapper. No allowed/denied outcome may change — freeze current behavior first with a table-driven characterization test generated from existing authority/guard tests.
- Task B (generic reconcile): one parameterized matcher implementing "collect inventory outside lock → match against baseline+fence marker → adopt iff exactly one candidate → CAS inside lock", used by WorktreeCreate (recovery.go:946), TerminalCreate (:974), TaskCreate (:995), Dispatch (:1015), RuntimeRefresh (:1030). Migrate one call site per commit; each migration keeps the existing `Reconcile*` function signature as a thin wrapper until all five are moved.
- Task C (parser extraction): move `parseExactIssueOpsCommand` (authority.go:20), `commandSpec` (:80), `exactReadOnlyShellCommand` (:278), `safeRipgrepArgs` (:364), `unresolvedNestedShellMutation` (:404) and the ASCII-control rejection (:1236) into `internal/core/commandparse` (package already exists — extend `tokens.go`, do not create a parallel package). Authority imports the parser; parsing behavior byte-identical (freeze with exhaustive accept/reject fixtures first).
- Task D (cross-process lock tests): subprocess-based tests (TestMain re-exec helper pattern, no external deps) proving (1) `sqlstore.WithSpan` BEGIN IMMEDIATE mutual exclusion across two OS processes, (2) lock release on process kill (SIGKILL mid-span), (3) `withIssueOpsLock` (issueops_lock.go:16) same-root reentry rejection across processes. Gate long-running cases behind `-short` skip so CI wall-time stays bounded.
- Task E (docs): new "IssueOps handoff: threat model and invariants" section in `.agent-harness/ARCHITECTURE.md` covering: adversarial multi-session model (coordinator/worker over shared state.json + external Orca runtime), fence triple (Attempt/OwnershipEpoch/ContextSHA256), state machine (coordinator_preparing→dispatched→claimed→submitted→closed), lock discipline ("CAS only inside cycle lock; no Orca process call while holding it" — currently only in CAUTIONS.md:504), pending-operation journal ("timeout ≠ absence"), exact-one-candidate rule, cleanup-receipt ordering. CAUTIONS.md entries link to the new section instead of duplicating it. Note in ISSUEOPS_AUDIT.md that the handoff subsystem postdates that audit.
- Every task lands as its own atomic commit (COMMIT_POLICY Conventional + Lore) with RED/GREEN or characterization evidence in the Lore Verify line.

### Must NOT have (guardrails, anti-slop, scope boundaries)
- Must not change any fail-closed outcome: no command/tool call that is denied today may become allowed, and vice versa. Characterization tests are the gate for A and C.
- Must not weaken default-deny in authority, the exactly-one adoption rule, quiescence gates, or receipt ordering.
- Must not bump `schema_version` (root v6 / handoff protocol v1) or alter `model.IssueOpsExecutionHandoff` field semantics; Task A/B are pure code-structure moves.
- Must not regenerate response-contract/MCP goldens except where a moved symbol forces an identical-content regeneration; any golden diff must be reviewed line-by-line and explained in the commit Lore.
- Must not touch dispatch/publication business flow (checkpoint→journal begin→execute→complete pattern stays byte-for-byte in behavior).
- Must not introduce external test dependencies (Docker, testcontainers); Task D uses stdlib re-exec only.
- Must not "improve" adjacent code, comments, or naming outside the five task boundaries (AGENTS.md §3 surgical changes).
- Must not store any real session/host identifiers or secrets in new fixtures.

## Verification strategy
> Zero human intervention — all verification is agent-executed.
- Behavior freeze first: Tasks A and C begin by generating characterization fixtures from the CURRENT implementation (allowed/denied matrix for authority; accept/reject corpus for the parser) and committing them as tests before any code moves. The move commit must keep those tests green with zero fixture edits.
- Standard gate per task: `go test ./... -count=1`, `go test -race ./internal/core/...`, `go build -o bin/agent-harness ./cmd/harness`, `./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json`.
- Golden gate: `go test ./cmd/harness/contractgolden -run Golden -count=1` and `go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1` must pass without `-update` for Tasks A–D (Task E is docs-only).
- Task D proof: test output must show both processes attempting the span, exactly one holding at a time, and lock recovery after SIGKILL (assert on ordered timestamps/markers written by the helper processes).
- Evidence: `evidence/handoff-hardening/task-<letter>-<slug>.<ext>` (git-ignored evidence tree per repo convention).

## Execution strategy

### Task decomposition

Task A — Invariant table consolidation
1. Enumerate current rules: extract every (state, role, command/tool) decision from `coordinatorLifecycleStateAllows`, `allowedExactHandoffLifecycleCommand`, `allowedHandoffMCPTool`, and the transition guards in `handoff/state.go` into a fixture matrix; commit as characterization test.
2. Introduce `internal/core/issueops/handoff/authoritytable` (declarative rows + named predicates for fence/identity/cwd checks).
3. Re-point authority (hook layer) to the table; delete inlined switches; characterization test stays green.
4. Re-point `state.go` transition preconditions and `lifecycle.go` execution checks to the same named predicates.
Anchors: authority.go:257/:1413/:1421/:1429/:1436, state.go:161/:178/:221/:261/:331, lifecycle.go:79/:131/:194/:353.

Task B — Generic exact-one reconcile
1. Define matcher: `reconcileExactlyOne(kind, baseline, fenceMarker, inventory) → (candidate, fail-closed reason)` with the stable-identity requirement from `requireStableInventoryIdentities` (recovery.go:310) built in — unidentifiable rows are never absence evidence.
2. Migrate call sites in order of blast radius: TaskCreate → TerminalCreate (PTY-optional variant) → WorktreeCreate (+ cleanup-only tombstone path recovery.go:1121) → Dispatch → RuntimeRefresh (tab/leaf stable-identity join).
3. After all five: collapse thin wrappers, keep public function names.
Anchors: recovery.go:927–1078, dispatch.go Reconcile* helpers, crash-matrix test `TestHandoffStartCrashMatrixNeverRepeatsCreate`.

Task C — Parser extraction to commandparse
1. Build accept/reject corpus from existing authority/guard tests plus targeted additions (nested-shell mutation, ASCII control chars, ripgrep flag edge cases); commit as characterization test against current code.
2. Move parser/security-filter functions into `internal/core/commandparse`; authority becomes a consumer.
3. Confirm guard behavior unchanged: `go test ./internal/core/lifecycle/... ./internal/core/commandparse/... -count=1`.
Anchors: authority.go:20/:80/:278/:364/:404/:1236, internal/core/commandparse/tokens.go.

Task D — Cross-process lock tests
1. Helper-process pattern: `TestMain` re-exec with env flag; helpers write ordered hold/release markers to a temp file.
2. Cases: concurrent WithSpan mutual exclusion; SIGKILL mid-span then successor acquires; issueops same-root reentry rejection from a second process.
3. Wire into CI via existing `go test ./...` (no new CI job); long cases behind `testing.Short()`.
Anchors: internal/core/sqlstore, issueops_lock.go:16.

Task E — Threat-model documentation
1. Draft ARCHITECTURE.md section from the two deep-dive reports (state machine, fence triple, lock discipline, journal, exact-one rule, receipt ordering).
2. Replace duplicated prose in CAUTIONS.md (~:497–:569 handoff entries) with links to the section, keeping each CAUTIONS entry's one-line lesson.
3. Add a dated pointer note to ISSUEOPS_AUDIT.md scope.

### Parallel execution waves
> File-ownership over worker count: A and C both edit `lifecycle_handoff_authority.go`, so they must not run concurrently.

Wave 1 (no dependencies):
- Task E: docs — establishes the written invariants that A encodes; zero code risk.
- Task C: parser characterization corpus + extraction (owns authority.go first).
- Task D: lock tests (independent files).

Wave 2 (after C releases authority.go):
- Task A: characterization matrix, then table consolidation (owns authority.go, handoff/state.go, lifecycle.go).

Wave 3 (after A stabilizes shared predicates):
- Task B: generic reconcile migration (owns recovery.go/dispatch.go; benefits from A's named fence predicates but can start after Wave 2 begins if predicates are frozen early).

Critical path: C → A → B. Estimated commit count: 10–14 atomic commits (freeze + move pairs per task, one per Task-B call site).

### Rollback
Each task is an independent revert unit; freeze-test commits stay valid after a revert of the corresponding move commit. No state or schema migration anywhere in this plan, so rollback is `git revert` only.
