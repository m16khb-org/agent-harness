# IssueOps Handoff Hardening: Invariant Consolidation, Generic Reconcile, Parser Extraction, Cross-Process Lock Tests, Threat-Model Docs, Fence Scope & Escape Guidance

## TL;DR
> Summary:      Consolidate the handoff subsystem's triple-enforced invariants into single declarative sources, generalize the exact-one-candidate reconcile principle, extract command parsing out of the authority layer, add real multi-process lock contention tests, promote the handoff threat model into ARCHITECTURE.md — without changing any fail-closed semantics — and (Task F only, with an explicit reviewed carve-out) fix the supervised-handoff fence so a stranded handoff cannot silently deadlock the entire source checkout without naming a working escape.
> Origin:       2026-07-15 full-repo analysis + handoff deep-dive (normal-path and failure-path/bug-history reports). Bug history shows three recurring fix clusters: lock-mechanism generation changes (flock→advisory→sqlstore span, regression each time), schema-version bumps destroying lease fields (v1→v2→v3), and per-artifact re-application of the exact-one-candidate rule (worktree→terminal→task→dispatch→runtime). Amended same day with the #2581 fence incident (see Evidence base).
> Deliverables:
> - Task A: declarative state×role×command authority table shared by envelope validation, hook authority, and lifecycle execution checks.
> - Task B: generic "baseline+marker → exactly-one" reconcile matcher replacing five per-artifact implementations.
> - Task C: command parsing/security-filter helpers extracted from `lifecycle_handoff_authority.go` into the existing `internal/core/commandparse` package.
> - Task D: real subprocess-based cross-process lock contention tests for sqlstore span / issueops cycle lock.
> - Task E: handoff threat-model + invariants section in `.agent-harness/ARCHITECTURE.md`, cross-linked from CAUTIONS.md.
> - Task F: supervised-handoff fence scope + escape-guidance hardening — state-aware block messages that name the working recover command, declarative narrow allowances for provably-unrelated new work in the source checkout, and write-time prevention + doctor/cleanup detection of the phase-terminal/handoff-nonterminal inconsistency.
> Effort:       Large (6 tasks, each individually shippable)
> Risk:         Medium — refactors touch the most fail-closed-critical files in the repo; mitigated by behavior-freeze tests before each move and per-task atomic commits. Task F is the single place where allowed/denied outcomes may change, each delta enumerated and adversarially reviewed.

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

### The #2581 fence incident (why Task F was added, with anchors)

A supervised handoff for issue #2581 (`io-9bab890c4d4f`, branch `2581-vertex-cache-cost-settlement`) failed during worktree provisioning: Orca's "Nest Workspaces" setting produced a nested path (`api-servers.worktrees/api-servers/2581-…`) that did not match the canonical IssueOps path (`api-servers.worktrees/2581-…`), so the lease was preserved as `recovery_required` with a `cleanup_only` tombstone (the correct, designed behavior per CAUTIONS.md:497/:552 — "timeout ≠ absence"). The worktree was never live. Verified from the state DB: `phase=done`, `handoff.state=recovery_required`, and it is the *only* record fencing `/Users/habin/workspace/api-servers`.

From that correct fail-closed state, four defects turned a recoverable stall into an apparent hard deadlock of the **main** checkout:

1. **Fence covers the whole source checkout with no terminal-phase or TTL release.** `active/issueops_active.go:148 SupervisedHandoffCyclesForRepo` excludes only `state=="closed"`; unlike `LinkedWorktreeCyclesForRepo` it deliberately keeps authority when the worktree is gone, and it never consults `record.Phase` (so `phase=done` still fences) or any age bound. `lifecycle_handoff_authority.go:1056` then binds every session whose `cwd == record.Repo` (the source checkout) to that record. Net: one stranded lease blocks all sessions in the main checkout indefinitely.

2. **Unrelated new work is caught by the source-checkout fallback match.** `isHandoffLifecycleCommand` (`lifecycle_handoff_guard.go:425`) treats every `issueops` subcommand except `status`/`resume` as a lifecycle command, and `commandSpec` (`authority.go:80`) has no row for `start`, `cleanup stale`, or `force-release`. So starting a *different* issue's cycle, and the two documented stale-recovery escapes themselves, are all denied with the same message.

3. **The block message names no working escape — a direct CAUTIONS.md:288 regression.** `recovery_required` mutation is denied at `lifecycle_handoff_guard.go:206` with "remain read-only and poll: … resume", and out-of-whitelist commands with "flags do not match the native session and persisted fence" (`:177`). Neither names `issueops handoff recover`, which *is* allowed here. CAUTIONS.md:288 already codifies "a block message must name a working escape"; this path violates it. The misleading "native session" wording also caused the operator (this session) to wrongly conclude only the original coordinator could recover it.

4. **`phase=done` + non-terminal handoff is an unguarded inconsistency.** Nothing at write time prevents a record from being advanced to `phase=done` while its handoff is still `recovery_required`, and no `state doctor` / `cleanup stale` signal flags the combination. This is the state that makes defect 1 surprising: an operator reasonably reads `done` as "finished," not "still fencing."

Ground truth confirmed this session: `handoff recover --action cancel` then `--action finalize-cancel` is allowed from the source checkout with no session-identity check (`authority.go:231`/`coordinatorLifecycleStateAllows` :267–268) — the escape existed the whole time; only the guidance hid it. These four defects are NOT covered by Tasks A–E (A consolidates existing allow/deny rules without changing scope or messages; E documents the model but adds no guard behavior), so Task F is additive.

## Scope

### Must have
- Task A (invariant table): a single declarative module under `internal/core/issueops/handoff/` mapping (state × role × command/tool surface) → allowed, with fence/identity predicates as named reusable checks. All three layers consult the same table; authority keeps its default-deny wrapper. No allowed/denied outcome may change — freeze current behavior first with a table-driven characterization test generated from existing authority/guard tests.
- Task B (generic reconcile): one parameterized matcher implementing "collect inventory outside lock → match against baseline+fence marker → adopt iff exactly one candidate → CAS inside lock", used by WorktreeCreate (recovery.go:946), TerminalCreate (:974), TaskCreate (:995), Dispatch (:1015), RuntimeRefresh (:1030). Migrate one call site per commit; each migration keeps the existing `Reconcile*` function signature as a thin wrapper until all five are moved.
- Task C (parser extraction): move `parseExactIssueOpsCommand` (authority.go:20), `commandSpec` (:80), `exactReadOnlyShellCommand` (:278), `safeRipgrepArgs` (:364), `unresolvedNestedShellMutation` (:404) and the ASCII-control rejection (:1236) into `internal/core/commandparse` (package already exists — extend `tokens.go`, do not create a parallel package). Authority imports the parser; parsing behavior byte-identical (freeze with exhaustive accept/reject fixtures first).
- Task D (cross-process lock tests): subprocess-based tests (TestMain re-exec helper pattern, no external deps) proving (1) `sqlstore.WithSpan` BEGIN IMMEDIATE mutual exclusion across two OS processes, (2) lock release on process kill (SIGKILL mid-span), (3) `withIssueOpsLock` (issueops_lock.go:16) same-root reentry rejection across processes. Gate long-running cases behind `-short` skip so CI wall-time stays bounded.
- Task E (docs): new "IssueOps handoff: threat model and invariants" section in `.agent-harness/ARCHITECTURE.md` covering: adversarial multi-session model (coordinator/worker over shared state.json + external Orca runtime), fence triple (Attempt/OwnershipEpoch/ContextSHA256), state machine (coordinator_preparing→dispatched→claimed→submitted→closed), lock discipline ("CAS only inside cycle lock; no Orca process call while holding it" — currently only in CAUTIONS.md:504), pending-operation journal ("timeout ≠ absence"), exact-one-candidate rule, cleanup-receipt ordering. CAUTIONS.md entries link to the new section instead of duplicating it. Note in ISSUEOPS_AUDIT.md that the handoff subsystem postdates that audit.
- Task F (fence scope + escape guidance): the four #2581 defects fixed as three sub-parts, each preceded by a characterization test that pins the *current* deny/allow so the delta is explicit and reviewed:
  - F1 (escape-naming, lowest risk, no allow/deny change): every supervised-fence block message names the exact working escape for the current state. `recovery_required` → the precise `agent-harness issueops handoff recover --id <id> --action <cancel|finalize-cancel|reconcile|approve-cleanup> …` next-step for that record's sub-state (mirroring how `renderHandoffSessionGuidance` already builds exact claim/resume strings, `lifecycle_handoff_guard.go:61`); out-of-whitelist commands say "not in the supervised-fence allowlist; the working escape is <recover/force-release/resume>" instead of the identity-mismatch wording. Pin the guard-message↔actual-escape pairing with a test as CAUTIONS.md:288 requires.
  - F2 (fence-scope narrowing, the one deliberate allow-delta): a *provably unrelated* `issueops start`/`resume --bind`/`status` for a **different** `(repo,branch)` id, run from the source checkout, must not be denied by a stranded record it does not name. Implemented by tightening the source-checkout fallback in `selectSupervisedHandoffRecord` (`authority.go:1056`) so a fenced record only captures a command that (a) carries no explicit `--id`, or (b) carries an `--id`/target that resolves to that record; a command explicitly targeting a *different* existing cycle id is not bound to the stranded record. Default-deny is preserved for anything ambiguous or unnamed. This is the single allowed-set change in the whole plan and every newly-allowed command is enumerated in the characterization diff.
  - F3 (inconsistency prevention + detection): reject at write time any transition to `phase=done` (or other terminal phase) while `ExecutionHandoff` is non-terminal (state ∉ {closed}), pointing the caller at recover; and add a `state doctor` / `cleanup stale` classifier signal `handoff_nonterminal_on_terminal_phase` that reports (never auto-releases) the combination with the recover command to run. Existing stranded records like `io-9bab890c4d4f` are surfaced by the classifier, not silently mutated.
- Every task lands as its own atomic commit (COMMIT_POLICY Conventional + Lore) with RED/GREEN or characterization evidence in the Lore Verify line.

### Best-method selection for Task F (alternatives considered, with rejection rationale)

Each defect had ≥2 candidate fixes; the plan commits to the one that removes the recurrence class without weakening fail-closed. Recorded so the implementer does not re-litigate and reviewers see the rejected branches (ADR-style):

- Defect 1 (whole-checkout deadlock) — **Chosen: keep durable fence authority, fix it via F1 escape-naming + F3 detection.** Rejected (a) *auto-release the fence when `phase==done`*: unsafe — `phase` and handoff terminality are independent axes and a done-phase record can still own un-reconciled Orca artifacts (`cleanup_only`), so auto-release could abandon a real worktree/task (exactly the "timeout ≠ absence" class the subsystem exists to prevent). Rejected (b) *TTL/age-based expiry*: a stalled-but-live handoff is indistinguishable from a dead one by age alone; TTL would reintroduce the abandonment bug. The durable-authority design is correct; the failure was operability (no named escape), so the fix targets guidance + detection, not the fence lifetime.
- Defect 2 (unrelated work blocked) — **Chosen: F2 scope narrowing keyed on explicit different-id targeting, default-deny on ambiguity.** Rejected (a) *whitelist `start`/`cleanup`/`force-release` unconditionally in `commandSpec`*: too broad — it would let a fenced worker also run cleanup/force-release against the stranded id from the wrong session. Rejected (b) *only fix the message, leave the block*: does not restore the ability to start an unrelated cycle from the main checkout, so the operational deadlock for parallel work remains. Narrowing the fallback match is the minimal change that unblocks provably-unrelated work while a targeted or ambiguous command stays denied.
- Defect 3 (misleading message) — folded into F1; no standalone alternative (naming the escape is strictly additive information).
- Defect 4 (done + non-terminal handoff) — **Chosen: F3 write-time rejection + doctor detection.** Rejected *detection-only*: leaves the inconsistency creatable, so the surprising state recurs; write-time prevention closes the source while detection catches records that predate the fix. Rejected *auto-repair in doctor*: same abandonment risk as Defect-1 auto-release; classifier reports and hands the operator the recover command instead.

### Must NOT have (guardrails, anti-slop, scope boundaries)
- Must not change any fail-closed outcome **in Tasks A–E**: no command/tool call that is denied today may become allowed, and vice versa. Characterization tests are the gate for A and C. Task F is the sole exception and only for F2's enumerated different-id allowances; F1 and F3 add information and write-time denials but never widen an existing allow.
- Must not, in Task F, let a fenced *worker* session gain any new authority: F2 only unblocks commands that target a **different** existing cycle id (or carry no id) from the **source checkout**, and never `force-release`/`recover`/mutation against the stranded record from a non-source or worker context. The stranded record's own recovery still requires the exact allowed recover path.
- Must not auto-release or auto-repair any stranded/inconsistent handoff (no TTL expiry, no `phase==done`-triggered release, no doctor auto-fix) — detection reports and hands over the recover command; release stays an explicit operator action. This preserves "timeout ≠ absence."
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
- Task F proof:
  - F1: golden-string test asserting each supervised-fence block reason for a `recovery_required`/`coordinator_preparing`/`cleanup_only` record contains the exact runnable recover command; a regression test that reproduces the #2581 shape (`phase=done`, `state=recovery_required`, empty worktree) and asserts the message names `handoff recover`, not "flags do not match the native session."
  - F2: characterization test capturing the CURRENT deny for `issueops start`/`resume --bind`/`status` targeting a *different* id from the source checkout while a stranded record fences it; the fix flips exactly those enumerated rows from deny→allow and the diff is the complete allowed-set change for the plan. A companion test asserts a command targeting the stranded id, or an id-less mutation, stays denied.
  - F3: write-path test that `set-phase --to done` (and any terminal phase) is rejected while the handoff is non-terminal, with the recover pointer in the error; a `state doctor`/`cleanup stale` test asserting the `handoff_nonterminal_on_terminal_phase` signal fires for the #2581 shape and that `--apply` does NOT release it (report-only).
  - End-to-end: after F lands, a table test replays the #2581 incident from stranded state → operator runs the message's named recover command → fence clears → an unrelated `issueops start` in the source checkout succeeds; all without editing any fixture by hand.
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

Task F — Fence scope + escape guidance (the #2581 recurrence fix)
1. Characterize first: add a test that snapshots today's block reasons and deny/allow for the #2581 shape (stranded `recovery_required`, `phase=done`, empty worktree) across: source-checkout mutation, out-of-whitelist `issueops start`/`cleanup stale`/`force-release`, and a different-id `issueops start`. This freezes the buggy-but-current behavior so every F delta is visible.
2. F1 — escape-naming: add a state→escape resolver (reuse `renderHandoffSessionGuidance` construction style, `lifecycle_handoff_guard.go:61`) that, for each supervised block reason at `lifecycle_handoff_guard.go:177/:206/:209`, appends the exact `issueops handoff recover --id … --action …` (or `resume` for read-only) valid for that sub-state. Update the CAUTIONS.md:288 pin-test to cover these paths.
3. F2 — scope narrowing: tighten the source-checkout fallback in `selectSupervisedHandoffRecord` (`authority.go:1056`) and the `isHandoffLifecycleCommand`→`allowedExactHandoffLifecycleCommand` path so a lifecycle/cycle command carrying an explicit `--id` that names a *different* existing cycle is not bound to the stranded record; id-less commands and same-id commands keep current handling; anything ambiguous stays denied. Land the enumerated deny→allow diff as the characterization update.
4. F3 — inconsistency prevention + detection: add a write-time guard in the set-phase path (facade + `issueops/` writer) rejecting terminal-phase transitions while `ExecutionHandoff` state ∉ {closed}, with a recover pointer; add the `handoff_nonterminal_on_terminal_phase` classifier to `stalescan.Classify` (report-only, never releasable under `--apply`) and surface it in `state doctor`.
Anchors: `lifecycle_handoff_guard.go:61/:177/:206/:209/:425`, `authority.go:80/:209/:231/:1056`, `active/issueops_active.go:148`, `issueops/handoff/state.go` (phase/terminality), `stalescan.Classify`, CAUTIONS.md:288/:497/:552.

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

Task F placement: F1 (message text) and F3 (write-guard + classifier) are independent of A/C and can run in Wave 1 (they touch `lifecycle_handoff_guard.go` message construction, the set-phase writer, and `stalescan` — not the authority switches C/A move). F2 edits `selectSupervisedHandoffRecord`/`allowedExactHandoffLifecycleCommand` in `authority.go`, so it shares file ownership with C and A and must land **after** A's table consolidation (do F2 as the first change on top of the stabilized table so the one deliberate allow-delta is expressed as a table row, not a re-inlined switch). Prefer shipping F1+F3 early (they are the operator-facing safety net that would have prevented the #2581 dead-end) even before the A/B refactor completes.

Critical path: C → A → {B, F2}; F1 and F3 are off the critical path. Estimated commit count: 13–18 atomic commits (adds F1, F2, F3 characterization+fix pairs).

### Rollback
Each task is an independent revert unit; freeze-test commits stay valid after a revert of the corresponding move commit. No state or schema migration anywhere in this plan, so rollback is `git revert` only. Task F specifics: F1/F3 reverts restore the prior (less helpful) messages and drop the write-guard with no data effect; F2 revert re-narrows the allowed set back to today's over-broad deny — safe because it only ever *removes* an allowance, never leaves a widened one. No F sub-part writes or migrates state, so reverting cannot strand a record.
