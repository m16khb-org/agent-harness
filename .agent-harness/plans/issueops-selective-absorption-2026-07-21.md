# IssueOps Selective Absorption Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to execute this plan task-by-task. Do not dispatch sub-agents unless the user separately approves delegation and the repository sub-agent pattern gate is satisfied. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Selectively absorb Ouroboros-style verification receipts, versioned/sealed intent contracts, and replay-checkable critical-transition audit events without adding Ouroboros as a dependency or changing IssueOps snapshot authority.

**Architecture:** Deliver three independently reviewable and rollbackable contract increments. First, make each new `loop` attempt carry a structured receipt for its single stored `verify_argv`; second, version and explicitly seal the IssueOps intent before `plan`; third, atomically append a hash-chained event whenever the stable intent/phase/ownership projection changes while retaining the existing SQLite aggregate snapshot as the sole current-state authority.

**Tech Stack:** Go 1.26, standard library SHA-256/JSON, SQLite through `internal/core/sqlstore`, CLI `flag`, MCP JSON schemas, contract goldens, deterministic self-verify.

## Global Constraints

- Keep core behavior host-neutral. Codex, Claude Code, and GJC must observe the same core DTO and response semantics.
- The harness records and validates verification receipts but never executes `verify_argv`; the active agent remains the command executor.
- A required verification reported as `skipped`, `blocked`, missing, or structurally inconsistent must never satisfy loop success.
- Preserve the existing `records(bucket,id,data)` IssueOps snapshot as current-state authority; the audit journal is an integrity/audit projection, not a second reducer or scheduler.
- Do not install, vendor, invoke, or require Ouroboros, LiteLLM, another provider router, or a new daemon/worker.
- Do not make LLM semantic/consensus evaluation a completion gate. Deterministic command evidence remains authoritative.
- Every public CLI/MCP schema change must use shared core DTOs and update usage/MCP/response contract goldens in the same atomic commit.
- Every durable authority change must bump the owning schema and prove frozen older readers reject the newer bytes before mutation.
- All production changes follow named RED → minimal GREEN → focused regression → full verification. Do not synthesize RED after implementation.
- Preserve unrelated working-tree changes. Stage only the files named by the current task.
- No push, PR/MR creation, merge, installation, or live host update is authorized by this plan alone.

---

## TL;DR

> **Summary:** Harden existing IssueOps/loop contracts instead of introducing a second orchestrator. The sequence is receipt enforcement (`loop` schema v2), intent revision/sealing (IssueOps schema v9), then a narrow atomic audit chain (IssueOps schema v10).
>
> **Deliverables:** structured `LoopVerificationReceipt`; legacy-loop compatibility; intent revision/digest/history/seal; plan-entry seal gate; sealed intent in handoff context; atomic snapshot+append sqlstore primitive; critical projection event chain; `issueops audit verify`; CLI/MCP/golden/docs/tests.
>
> **Effort:** XL — three security-sensitive durable contract increments.
>
> **Parallel:** NO — schema/golden/state-machine changes share high-blast-radius contracts and must be accepted sequentially.
>
> **Critical Path:** T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8 → T9 → T10 → T11 → T12 → T13.

## Context

### Original Request

The user asked for a detailed plan to selectively absorb the useful Ouroboros philosophies and mechanisms identified in the prior comparison, rather than adopting Ouroboros wholesale.

### Repo Grounding

- `.agent-harness/ARCHITECTURE.md:69-71` makes IssueOps the durable authority and states that `loop` records `verify_argv` and evidence but does not execute commands.
- `internal/core/looprun/types.go:5-39` stores one `VerifyArgv` and pass/fail attempts with free-form evidence only.
- `internal/core/looprun/lifecycle.go:71-109` accepts `pass`/`fail` plus evidence without exit-code, cwd, skip, or block semantics.
- `internal/core/issueops/model/types.go:105-124` already contains the Seed-like intent content fields.
- `internal/core/issueops/intentdesign/intent_design.go:25-62` currently replaces `record.Intent` on every record call and preserves no revision history.
- `internal/core/issueops/issueops_readiness.go:14-30` gates plan entry on intent presence, issue URL, and plan-prep evidence but not on a sealed digest.
- `internal/core/issueops/issueops_regress.go:55-126` already provides the only valid plan/compatibility-review → grill regression path and invalidates design approval.
- `internal/core/issueops/handoff/context.go:82-166` seals a deterministic execution context but currently includes intent text without intent revision/digest.
- `internal/core/issueops/issueops_state.go:230-267` writes one aggregate JSON snapshot through `sqlstore.DB.Put`.
- `internal/core/sqlstore/sqlstore.go:132-137,428-432` uses a `(bucket,id)` primary key and per-record UPSERT.
- Current live read-only survey: `issueops` has 6 rows (2,102–20,429 bytes; average 6,781 bytes), `session` has 1,534 rows, and snapshot lookup uses `SEARCH records USING PRIMARY KEY (bucket=? AND id=?)`.
- `.agent-harness/CONVENTIONS.md:260-267` requires pure record-based state validation and injected nondeterministic inputs.
- `.agent-harness/TESTING.md:109-180` requires targeted tests first, full Go/race/vet/build verification, and intentional golden updates only.
- Planning baseline on 2026-07-21: `go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1` passed. `go test ./... -count=1` hit the existing 10-minute package timeout while `TestHandoffRetryRejectsUnsafeWorktreeCheckpointBeforeNewAttempt/dirty_worktree` waited in `preflight.GitCmd`; the exact isolated test then passed in 1.482s. Treat a repeated aggregate timeout as a baseline stability defect to diagnose separately—never as evidence that these changes pass or fail.

### Gap Analysis

- **Receipt truth gap:** A loop pass is currently a caller assertion. The harness cannot prove external execution, but it can reject internally inconsistent attestations and make skip/block visible.
- **Compatibility gap:** Immediately requiring receipts for existing active schema-v1 loops would strand durable state. New schema-v2 loops therefore use `structured_v1`; migrated legacy loops retain `legacy_evidence` until terminal.
- **Intent drift gap:** Simply adding a digest without a revision/history contract would still permit silent overwrite. Initial record, revision, seal, and regress invalidation must be separate transitions.
- **Artifact invalidation gap:** Revising intent after planning would stale design/compatibility artifacts. Revision is therefore allowed only in `problem` or `grill`; a planned cycle must use the existing Brooks stop→reflect→regress path before revising.
- **Dual-authority risk:** Rebuilding the full record from events would duplicate IssueOps authority. Events therefore cover only a stable critical projection, and `audit verify` compares that projection with the chain head.
- **Atomicity gap:** Snapshot UPSERT followed by event INSERT can split on process crash. A new sqlstore transaction must perform event `INSERT` and snapshot UPSERT atomically without external I/O inside the transaction.
- **Old-writer gap:** Older binaries could erase receipt, seal, or audit authority. Loop v2, IssueOps v9, and IssueOps v10 each need frozen-reader rejection tests and current-reader migration tests.
- **Handoff blast radius:** Auditing every pending-operation substep would turn the journal into a second event source. Only top-level ownership milestones enter the stable projection; existing detailed pending-operation journals remain unchanged.

## Work Objectives

### Core Objective

Strengthen completion truth, user-intent stability, and critical workflow auditability using the smallest host-neutral additions that preserve current execution and persistence boundaries.

### Deliverables

- `LoopVerificationReceipt` embedded atomically in each structured loop attempt.
- Loop schema v2 with explicit compatibility mode for old active loops.
- Intent revision numbers, content SHA-256, supersession metadata, bounded history, and an explicit seal.
- Plan/readiness and handoff-context binding to the exact sealed intent digest.
- IssueOps schema v9 migration and frozen-v8 writer rejection proof.
- Transactional `PutWithAppend` sqlstore primitive with insert-only event semantics.
- Stable critical projection and hash-chained `IssueOpsAuditEvent` rows.
- IssueOps schema v10 semantic gate requiring an audit event whenever the critical projection changes.
- Read-only `issueops audit verify` CLI and `issueops_audit_verify` MCP tool.
- Updated response contracts, usage, architecture/ADR/testing/operations/cautions, and focused/full tests.

### Definition of Done

- Every new loop rejects success when its latest attempt has no structured receipt or has `skipped`/`blocked`/`fail` status.
- A structured pass receipt records the stored command argv, canonical repo cwd, exit code 0, evidence, optional SHA-256 artifact digests, and harness-owned timestamp.
- Existing schema-v1 active loops remain operable through explicit legacy mode; all newly created loops require non-empty `verify_argv` and structured receipts.
- An IssueOps intent cannot be silently overwritten; revision requires a reason, material content change, and `problem`/`grill` phase.
- `plan` entry fails with `intent_seal` until the current revision is sealed; regression to `grill` clears the seal.
- Handoff context carries the sealed intent revision/digest and changes its context/source digest when the intent changes.
- Snapshot and critical audit event commit atomically; duplicate event IDs roll back both writes.
- `issueops audit verify` detects missing sequence, broken previous-event digest, event payload tampering, and current critical-projection mismatch.
- All targeted, full, race, vet, build, contract-golden, and deterministic self-verify gates pass from one final unchanged HEAD.

### Must Have

- Fail-closed receipt validation with exact error tokens.
- Deterministic canonical hashing that excludes timestamps and transient `OK`/diagnostic fields.
- Explicit schema compatibility and old-writer rejection tests.
- Redaction/bounds for every free-form reason and evidence value.
- Atomic event append and snapshot update in one SQLite data transaction.
- CLI/MCP parity and read-only audit verification.
- Three atomic rollback units matching the three increments.

### Must NOT Have

- Command execution, shell parsing, timers, schedulers, model routing, or automatic retries in the new code.
- Multiple verification commands inside one loop contract; use separate stable loop names if independent checks must be gated.
- Numeric LLM ambiguity/convergence thresholds.
- Revision while phase is `plan` or later; use Brooks regression first, or start a new cycle after implementation begins.
- Full-record event replay, event-driven current state, event compaction, partitioning, or a generic event bus.
- Additional database indexes before measured event cardinality/query plans justify them.
- Audit events for heartbeats, display-only timestamps, progress text, or every internal handoff pending-operation.

## Chosen Data Contracts

### Loop schema v2

```go
const LoopRunCurrentSchemaVersion = 2

const (
	LoopReceiptModeLegacy     = "legacy_evidence"
	LoopReceiptModeStructured = "structured_v1"
)

type LoopVerificationReceipt struct {
	CheckID        string   `json:"check_id"`
	Argv           []string `json:"argv"`
	CWD            string   `json:"cwd"`
	Status         string   `json:"status"` // pass|fail|skipped|blocked
	ExitCode       *int     `json:"exit_code,omitempty"`
	Evidence       []string `json:"evidence"`
	ArtifactSHA256 []string `json:"artifact_sha256,omitempty"`
	RecordedAt     string   `json:"recorded_at"`
}

type LoopAttempt struct {
	Seq      int                      `json:"seq"`
	Verdict  string                   `json:"verdict"`
	Evidence []string                 `json:"evidence"`
	Receipt  *LoopVerificationReceipt `json:"receipt,omitempty"`
	At       string                   `json:"at"`
}
```

- `CheckID` is deterministic: `verify-` plus the first 12 lowercase hex characters of SHA-256 over canonical repo, NUL, and NUL-separated stored argv.
- Core copies `LoopRun.VerifyArgv` into the receipt; callers cannot submit a different command.
- `pass` requires exit code 0; `fail` requires a non-zero exit code; `skipped`/`blocked` require no exit code.
- `Verdict=pass` requires receipt status `pass`; `Verdict=fail` accepts receipt status `fail|skipped|blocked`.
- New loops require a non-empty `VerifyArgv` and `ReceiptMode=structured_v1`.
- Migrated v0/v1 loops use `legacy_evidence`; their existing attempt/stop behavior stays intact until terminal.

### IssueOps schema v9 intent contract

```go
type IssueOpsIntentContract struct {
	RawRequest, InterpretedIntent string
	SuccessCriteria, Constraints, Ambiguities, NonGoals []string
	IntentClass      string
	Revision         int      `json:"revision"`
	SHA256           string   `json:"sha256"`
	SupersedesSHA256 string   `json:"supersedes_sha256,omitempty"`
	ChangeReason     string   `json:"change_reason,omitempty"`
	ChangedFields    []string `json:"changed_fields,omitempty"`
	RecordedAt       string   `json:"recorded_at"`
}

type IssueOpsIntentSeal struct {
	Revision int    `json:"revision"`
	SHA256   string `json:"sha256"`
	SealedAt string `json:"sealed_at"`
}
```

- `IssueOpsRecord.Intent` remains the current revision; `IntentHistory []IssueOpsIntentContract` retains prior revisions; `IntentSeal *IssueOpsIntentSeal` binds the current revision.
- Canonical intent SHA-256 includes only normalized semantic content fields and preserves list order; it excludes revision metadata and timestamps.
- First record is revision 1. A revision appends the old contract to history, increments revision, records `supersedes_sha256`, and uses sorted field names in `changed_fields`.
- `record` rejects an existing intent. `revise` requires a bounded redacted reason, material change, phase `problem|grill`, and no transferred ownership.
- `seal` is idempotent only for the same current revision/digest and is allowed only in `grill`.
- `plan` and every downstream readiness projection require a matching current seal.
- Brooks regression clears `IntentSeal` while retaining current intent/history; the next plan requires a new seal.

### IssueOps schema v10 critical audit chain

```go
type IssueOpsAuditHead struct {
	Sequence         uint64 `json:"sequence"`
	EventSHA256      string `json:"event_sha256"`
	ProjectionSHA256 string `json:"projection_sha256"`
}

type IssueOpsAuditEvent struct {
	SchemaVersion        int    `json:"schema_version"`
	IssueOpsID           string `json:"issueops_id"`
	Sequence             uint64 `json:"sequence"`
	Kind                 string `json:"kind"`
	PreviousEventSHA256  string `json:"previous_event_sha256,omitempty"`
	BeforeProjectionSHA256 string `json:"before_projection_sha256"`
	AfterProjectionSHA256  string `json:"after_projection_sha256"`
	Actor                string `json:"actor,omitempty"`
	RecordedAt           string `json:"recorded_at"`
	EventSHA256          string `json:"event_sha256"`
}
```

- Event bucket: `issueops-audit`; event ID: `<issueops-id>/<20-digit-zero-padded-sequence>`.
- Stable critical projection contains current intent revision/digest/seal, phase, execution workspace state/epoch, and top-level handoff protocol/state/attempt/ownership epoch/disposition/completion final head. It excludes timestamps, diagnostics, heartbeat, pending-operation details, free-form bodies, and `OK`.
- Event kinds are closed enums: `intent_recorded`, `intent_revised`, `intent_sealed`, `phase_advanced`, `phase_regressed`, `workspace_prepared`, `ownership_dispatched`, `ownership_claimed`, `ownership_oriented`, `ownership_completed`, `ownership_cleanup_started`, `ownership_closed`.
- Event hash is SHA-256 of canonical event fields excluding `EventSHA256`.
- A schema-v10 write that changes the stable critical projection without advancing `AuditHead` through the audited write path fails with `issueops_audit_event_required`.
- Existing schema-v9 records start at event sequence 1 on the next audited critical mutation; no historical events are fabricated.

## Verification Strategy

> ZERO HUMAN INTERVENTION — all planned verification is agent-executed. Remote publication and native installation are outside this plan.

- Test decision: strict Go TDD with named failing tests before each production change.
- Isolation: `t.Setenv("HARNESS_STATE_DIR", t.TempDir())`; SQLite fault tests use temporary state roots and subprocess helpers where crash behavior matters.
- Core assertions: exact error tokens, byte-preserving rejected writes, deterministic digest equality, atomic rollback on duplicate event ID.
- Adapter assertions: CLI JSON and MCP payloads decode to the same core structs.
- Golden policy: regenerate only after focused contract tests pass; review generated diff before staging.
- Evidence: terminal output from named RED/GREEN tests and final gate commands; do not commit transient evidence unless an active IssueOps/Turing cycle explicitly owns it.

## Execution Strategy

### Sequential Waves

- **Wave 1 — Receipt truth:** T1–T3. Ship and verify loop schema v2 independently.
- **Wave 2 — Intent stability:** T4–T7. Ship and verify IssueOps schema v9 independently.
- **Wave 3 — Critical auditability:** T8–T12. Ship atomic append, schema v10, critical transition integration, and verifier.
- **Final Wave:** T13 plus F1–F4. Re-run every gate from the unchanged final tree.

### Dependency Matrix

| Task | Depends On | Blocks | Can Parallelize With |
|---|---|---|---|
| T1 | — | T2 | — |
| T2 | T1 | T3 | — |
| T3 | T2 | T4 | — |
| T4 | T3 | T5 | — |
| T5 | T4 | T6 | — |
| T6 | T5 | T7 | — |
| T7 | T6 | T8 | — |
| T8 | T7 | T9 | — |
| T9 | T8 | T10 | — |
| T10 | T9 | T11 | — |
| T11 | T10 | T12 | — |
| T12 | T11 | T13 | — |
| T13 | T12 | Final Wave | — |

## TODOs

### Task 1: Add loop schema-v2 structured receipts in core

**Files:**
- Modify: `internal/core/looprun/types.go`
- Modify: `internal/core/looprun/lifecycle.go`
- Modify: `internal/core/looprun/store.go`
- Modify: `internal/core/looprun/lifecycle_test.go`
- Modify: `internal/core/looprun/store_test.go`

**Interfaces:**
- Consumes: existing `LoopRun.VerifyArgv`, normalized `LoopRun.Repo`, `RecordAttemptRequest`, and loop sqlstore lifecycle.
- Produces: `LoopVerificationReceipt`, `LoopRun.ReceiptMode`, `RecordAttemptRequest{Status,CWD,ExitCode,ArtifactSHA256}`, loop schema v2.

- [ ] **Step 1: Write named RED tests for structured receipt validation.**

  Add table-driven tests with these exact names and assertions:

  ```go
  func TestStructuredAttemptRequiresReceiptFields(t *testing.T)
  func TestStructuredAttemptStatusExitCodeMatrix(t *testing.T)
  func TestStructuredAttemptCopiesStoredArgvAndCanonicalRepo(t *testing.T)
  func TestStructuredAttemptRejectsArtifactDigestAndCWDDrift(t *testing.T)
  func TestStructuredLoopSuccessRequiresPassingReceipt(t *testing.T)
  func TestLegacyLoopRetainsEvidenceOnlyCompatibility(t *testing.T)
  func TestLoopSchemaV1MigratesAndFrozenV1RejectsV2(t *testing.T)
  ```

  Exact error tokens:

  ```text
  verify_argv_required
  receipt_status_required
  receipt_cwd_required
  receipt_cwd_mismatch
  receipt_exit_code_required
  receipt_exit_code_forbidden
  receipt_exit_code_mismatch
  receipt_status_verdict_mismatch
  receipt_artifact_sha256_invalid
  loop_success_requires_passing_receipt
  ```

- [ ] **Step 2: Run the receipt tests and capture the intended RED.**

  Run:

  ```bash
  go test ./internal/core/looprun -run 'Structured|LegacyLoop|SchemaV1' -count=1 -v
  ```

  Expected: tests compile-fail on missing receipt types/fields, or fail on the first current behavior that accepts evidence-only structured attempts. Record the exact failing test; do not edit production first.

- [ ] **Step 3: Add the schema-v2 types and request fields exactly as chosen above.**

  Add to `LoopRun`:

  ```go
  ReceiptMode string `json:"receipt_mode,omitempty"`
  ```

  Extend `RecordAttemptRequest`:

  ```go
  type RecordAttemptRequest struct {
	Verdict        string
	Status         string
	CWD            string
	ExitCode       *int
	Evidence       []string
	ArtifactSHA256 []string
  }
  ```

  Do not add start/end timestamps supplied by callers. `RecordedAt` and `LoopAttempt.At` use the same harness-owned injected clock value.

- [ ] **Step 4: Implement deterministic receipt construction and validation as pure helpers.**

  Add unexported helpers in `lifecycle.go`:

  ```go
  func loopVerificationCheckID(repo string, argv []string) string
  func buildLoopVerificationReceipt(loop LoopRun, req RecordAttemptRequest, now string) (LoopVerificationReceipt, error)
  func validateReceiptStatus(verdict, status string, exitCode *int) error
  func cleanArtifactSHA256(values []string) ([]string, error)
  ```

  `buildLoopVerificationReceipt` must canonicalize `req.CWD` using the same `normalizeRepo` path logic and require equality with `loop.Repo`. It copies stored `VerifyArgv`; it never accepts caller argv or executes a command. Artifact values are lowercase 64-character hex, deduplicated in input order, with a maximum of 32 values.

- [ ] **Step 5: Change lifecycle behavior only for `structured_v1`.**

  - New `Start` requires non-empty cleaned `VerifyArgv` and sets `ReceiptMode=structured_v1`.
  - A resumed active loop returns its stored mode unchanged.
  - `RecordAttempt` builds and embeds a receipt for structured loops before appending the attempt.
  - Legacy mode keeps current verdict/evidence behavior and leaves `Receipt=nil`.
  - `Stop(success=true)` requires the latest structured attempt to have both verdict and receipt status `pass` with exit code 0.
  - Existing max-attempt and terminal rules remain byte-semantically unchanged.

- [ ] **Step 6: Implement schema migration and frozen-reader proof.**

  `normalizeLoopSchemaVersion` accepts 0/1/2. For 0/1 it sets `ReceiptMode=legacy_evidence`, stamps schema 2 on the next successful write, and never fabricates receipts. Add a frozen local v1 decoder fixture that rejects raw schema-v2 JSON before any rewrite.

- [ ] **Step 7: Run focused GREEN and package race tests.**

  Run:

  ```bash
  go test ./internal/core/looprun -run 'Structured|LegacyLoop|SchemaV1|Start|RecordAttempt|Stop' -count=1 -v
  go test -race ./internal/core/looprun -count=1
  ```

  Expected: all PASS; no sleep, network, or real shell execution appears in tests.

**Must NOT do:** Add multiple checks, shell parsing, command output capture, a runner dependency, or a new loop subcommand.

**Recommended Agent:** deep — durable compatibility and state-machine validation share one atomic append path.

**Parallelization:** Can Parallel: NO | Wave 1 | Blocks: T2 | Blocked By: —

**Acceptance Criteria:**
- [ ] New loops cannot start without `verify_argv`.
- [ ] Every status/exit-code/verdict matrix case has a named binary assertion.
- [ ] Structured receipts copy stored argv and use canonical repo cwd.
- [ ] Legacy active loop behavior is covered and frozen v1 rejects v2.
- [ ] `go test -race ./internal/core/looprun -count=1` exits 0.

**QA Scenarios:**

```text
Scenario: Structured pass receipt
  Channel: bash/go test
  Steps: Run TestStructuredAttemptCopiesStoredArgvAndCanonicalRepo and TestStructuredLoopSuccessRequiresPassingReceipt.
  Expected: receipt argv equals stored argv, cwd equals canonical repo, exit_code=0, stop succeeds.
  Evidence: named test output.

Scenario: Skip cannot become pass
  Channel: bash/go test
  Steps: Submit verdict=pass,status=skipped,nil exit code with non-empty evidence.
  Expected: receipt_status_verdict_mismatch and record bytes unchanged.
  Evidence: TestStructuredAttemptStatusExitCodeMatrix output.
```

**Commit:** NO — stage with T2/T3 as one receipt-contract commit.

### Task 2: Expose receipt fields through CLI and MCP with parity

**Files:**
- Modify: `cmd/harness/loopcli/loop.go`
- Modify: `cmd/harness/loopcli/loop_cli_test.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_loop.go`
- Modify: `cmd/harness/mcpcli/mcp_loop_test.go`
- Modify: `internal/adapter/mcp/loop_catalog.go`
- Modify: `internal/adapter/mcp/catalog_test.go`
- Modify: `internal/adapter/cli/usage.go`
- Modify: `cmd/harness/testdata/usage.golden.txt`
- Modify: `cmd/harness/testdata/mcp_tools.golden.json`
- Modify: `cmd/harness/testdata/response_contracts.golden.json`

**Interfaces:**
- Consumes: Task 1 `RecordAttemptRequest` and receipt response.
- Produces: additive record-attempt flags/properties; unchanged four-tool loop catalog.

- [ ] **Step 1: Write CLI and MCP RED tests before adapter changes.**

  Add exact tests:

  ```go
  func TestLoopCLIRecordsStructuredReceipt(t *testing.T)
  func TestLoopCLIRejectsSkippedPass(t *testing.T)
  func TestMCPLoopRecordsStructuredReceipt(t *testing.T)
  func TestMCPLoopSchemaAdvertisesReceiptFields(t *testing.T)
  ```

  Both transports must decode the response into `looprun.LoopRun` and assert the same `Receipt` fields.

- [ ] **Step 2: Run adapter RED tests.**

  ```bash
  go test ./cmd/harness/loopcli ./cmd/harness/mcpcli ./internal/adapter/mcp -run 'Loop.*Receipt|Loop.*Skipped|LoopSchema' -count=1 -v
  ```

  Expected: missing flags/properties or missing receipt payload causes FAIL.

- [ ] **Step 3: Add CLI flags without changing the subcommand count.**

  Extend `loop record-attempt` usage and parsing with:

  ```text
  --status pass|fail|skipped|blocked
  --cwd PATH
  --exit-code N
  --artifact-sha256 HEX    (repeatable)
  ```

  Implement a local optional-int flag that records whether `--exit-code` was present, so exit code 0 is distinguishable from absence. Pass `nil` for absent; do not use a sentinel integer.

- [ ] **Step 4: Add MCP properties and mapping.**

  Add optional properties to `loop_record_attempt`:

  ```json
  {
    "status": {"type":"string","enum":["pass","fail","skipped","blocked"]},
    "cwd": {"type":"string"},
    "exit_code": {"type":"integer"},
    "artifact_sha256": {"type":"array","items":{"type":"string"},"maxItems":32}
  }
  ```

  Keep them optional in the advertised schema solely so legacy loops can be terminated; the description must state they are mandatory for new structured loops. In the handler, use `argmap.Set` to distinguish absent `exit_code` and pass a pointer only when present.

- [ ] **Step 5: Run adapter GREEN tests, then regenerate goldens once.**

  ```bash
  go test ./cmd/harness/loopcli ./cmd/harness/mcpcli ./internal/adapter/mcp -run 'Loop' -count=1
  go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -update -count=1
  git diff -- cmd/harness/testdata/usage.golden.txt cmd/harness/testdata/mcp_tools.golden.json cmd/harness/testdata/response_contracts.golden.json
  go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1
  ```

  Expected golden diff: only loop record-attempt usage/schema/response receipt fields and loop schema version/mode projections change.

**Must NOT do:** Add `loop_record_receipt`, make MCP execute commands, or silently coerce `skipped` to `fail`/`pass`.

**Recommended Agent:** deep — transport parity and golden review are one public-contract change.

**Parallelization:** Can Parallel: NO | Wave 1 | Blocks: T3 | Blocked By: T1

**Acceptance Criteria:**
- [ ] CLI and MCP produce equal receipt DTOs for the same input.
- [ ] Legacy loop inputs without receipt fields remain accepted only for legacy records.
- [ ] Exactly four loop tools remain advertised.
- [ ] Non-loop golden sections are unchanged.

**QA Scenarios:**

```text
Scenario: CLI/MCP parity
  Channel: bash/go test
  Steps: Run the two structured receipt adapter tests with status=pass,cwd=temp repo,exit_code=0,artifact SHA of 64 'a' characters.
  Expected: both return receipt status pass and identical check_id/argv/cwd/artifact values.
  Evidence: named test output and reviewed golden diff.

Scenario: Missing exit code
  Channel: bash/go test
  Steps: New structured loop; record status=pass without exit_code over CLI and MCP.
  Expected: both return receipt_exit_code_required and no attempt is appended.
  Evidence: adapter failure tests.
```

**Commit:** NO — commit after T3 verification.

### Task 3: Document and verify the receipt increment as an atomic rollback unit

**Files:**
- Modify: `.agent-harness/ARCHITECTURE.md`
- Modify: `.agent-harness/TESTING.md`
- Modify: `.agent-harness/OPERATIONS.md`
- Modify: `.agent-harness/CAUTIONS.md`
- Modify: `.agent-harness/ADR.md`
- Verify: all T1/T2 files and generated goldens.

**Interfaces:**
- Consumes: completed loop schema-v2 public contract.
- Produces: normative distinction between structured attestation and harness-executed proof; first atomic commit.

- [ ] **Step 1: Update normative docs with one owner per rule.**

  - ARCHITECTURE: loop still never executes commands; it validates structured receipts for new loops.
  - TESTING: required checks may not be reported complete from skip/block/missing receipt; external execution evidence remains necessary.
  - OPERATIONS: exact `loop start`, structured `record-attempt`, `status`, and `stop` examples.
  - CAUTIONS: receipt is caller-attested structured evidence, not cryptographic proof that a command ran.
  - ADR: record schema v2, legacy mode, rejected multi-check scheduler/new command, and old-writer boundary.

- [ ] **Step 2: Build and run an isolated CLI smoke.**

  ```bash
  go build -o bin/agent-harness ./cmd/harness
  receipt_state="$(mktemp -d)"
  receipt_repo="$(pwd -P)"
  HARNESS_STATE_DIR="$receipt_state" ./bin/agent-harness loop start --repo "$receipt_repo" --name receipt-smoke --goal "prove structured receipt" --json -- go test ./internal/core/looprun -count=1 >"$receipt_state/start.json"
  receipt_id="$(jq -r '.id' "$receipt_state/start.json")"
  HARNESS_STATE_DIR="$receipt_state" ./bin/agent-harness loop record-attempt --id "$receipt_id" --verdict pass --status pass --cwd "$receipt_repo" --exit-code 0 --evidence "go test ./internal/core/looprun -count=1 exit 0" --json >"$receipt_state/attempt.json"
  jq -e '.attempts[-1].receipt.status == "pass" and .attempts[-1].receipt.exit_code == 0 and .attempts[-1].receipt.argv[0] == "go"' "$receipt_state/attempt.json"
  HARNESS_STATE_DIR="$receipt_state" ./bin/agent-harness loop stop --id "$receipt_id" --success --json | jq -e '.status == "succeeded"'
  rm -rf "$receipt_state"
  ```

  Expected: every command exits 0 and the exact temp directory is removed after readback.

- [ ] **Step 3: Run the receipt increment full gate.**

  ```bash
  git diff --check
  gofmt -w internal/core/looprun/types.go internal/core/looprun/lifecycle.go internal/core/looprun/store.go cmd/harness/loopcli/loop.go cmd/harness/mcpcli/mcp_tool_loop.go internal/adapter/mcp/loop_catalog.go
  go test ./internal/core/looprun ./cmd/harness/loopcli ./cmd/harness/mcpcli ./internal/adapter/mcp -count=1
  go test ./... -count=1
  go test -race ./... -count=1
  go vet ./...
  go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1
  go build -o bin/agent-harness ./cmd/harness
  ```

  Expected: all exit 0 from one unchanged tree. If any edit follows, restart this task from focused tests.

- [ ] **Step 4: Commit only the receipt increment with Lore.**

  Stage exact T1–T3 paths. Commit subject:

  ```text
  feat(loop): record fail-closed verification receipts
  ```

  Lore must state: harness does not execute commands; legacy active loops remain evidence-only; new loops reject skip/block/missing receipts; verification commands actually run.

**Must NOT do:** Install the binary, push, or begin intent work before the receipt commit is locally verified.

**Recommended Agent:** deep — this is the rollback gate for a durable public contract.

**Parallelization:** Can Parallel: NO | Wave 1 | Blocks: T4 | Blocked By: T2

**Acceptance Criteria:**
- [ ] Isolated CLI smoke exits 0 and temp state is removed.
- [ ] Full/race/vet/build/golden gates exit 0.
- [ ] One atomic commit contains only receipt-contract paths.

**QA Scenarios:**

```text
Scenario: Fresh structured lifecycle
  Channel: bash
  Steps: Execute the isolated smoke exactly as written.
  Expected: start→attempt→success with receipt pass and no residual temp state.
  Evidence: command exits and JSON predicates.

Scenario: Honest skip failure
  Channel: bash/go test
  Steps: Run the CLI failure test for verdict=pass,status=skipped.
  Expected: nonzero error, no attempt append, no success stop.
  Evidence: named test output.
```

**Commit:** YES | Message: `feat(loop): record fail-closed verification receipts` | Files: exact T1–T3 paths.

### Task 4: Add deterministic intent revisions, history, seal, and schema v9

**Files:**
- Modify: `internal/core/issueops/model/types.go`
- Create: `internal/core/issueops/intentdesign/intent_contract.go`
- Create: `internal/core/issueops/intentdesign/intent_contract_test.go`
- Modify: `internal/core/issueops/intentdesign/intent_design.go`
- Modify: `internal/core/issueops/intentdesign/intent_design_test.go`
- Modify: `internal/core/issueops/issueops_state.go`
- Modify: `internal/core/issueops/issueops_schema_version_test.go`

**Interfaces:**
- Consumes: normalized/redacted intent content, IssueOps snapshot read/write, current schema v8 compatibility fixtures.
- Produces: revision-aware `IssueOpsIntentContract`, `IssueOpsIntentSeal`, `IntentHistory`, digest helpers, IssueOps schema v9.

- [ ] **Step 1: Write RED tests for canonical digest and revision metadata.**

  Add exact tests:

  ```go
  func TestIntentSHA256IsDeterministicAndExcludesMetadata(t *testing.T)
  func TestIntentChangedFieldsAreSortedAndContentBased(t *testing.T)
  func TestRecordIntentRejectsOverwrite(t *testing.T)
  func TestReviseIntentAppendsHistoryAndSupersedesDigest(t *testing.T)
  func TestReviseIntentRejectsNoMaterialChangeAndShortReason(t *testing.T)
  func TestSealIntentIsIdempotentOnlyForCurrentRevision(t *testing.T)
  func TestIntentRevisionLimitIsFailClosed(t *testing.T)
  func TestIssueOpsSchemaV8IntentMigratesAndFrozenV8RejectsV9(t *testing.T)
  ```

  Exact error tokens/messages include:

  ```text
  intent_already_recorded_use_revise
  intent_missing
  intent_revision_no_material_change
  intent_revision_reason_too_short
  intent_revision_limit_reached
  intent_seal_mismatch
  ```

- [ ] **Step 2: Run the intent model RED.**

  ```bash
  go test ./internal/core/issueops/intentdesign ./internal/core/issueops -run 'IntentSHA|IntentChanged|RecordIntentRejectsOverwrite|ReviseIntent|SealIntent|SchemaV8Intent' -count=1 -v
  ```

  Expected: missing fields/functions or current silent overwrite produces FAIL.

- [ ] **Step 3: Implement the canonical content projection.**

  In `intent_contract.go`, define a private struct containing exactly these fields in this order:

  ```go
  type intentContentProjection struct {
	RawRequest        string   `json:"raw_request"`
	InterpretedIntent string   `json:"interpreted_intent"`
	SuccessCriteria   []string `json:"success_criteria"`
	Constraints       []string `json:"constraints,omitempty"`
	Ambiguities       []string `json:"ambiguities,omitempty"`
	NonGoals          []string `json:"non_goals,omitempty"`
	IntentClass       string   `json:"intent_class,omitempty"`
  }
  ```

  Add:

  ```go
  func IntentSHA256(intent model.IssueOpsIntentContract) (string, error)
  func IntentChangedFields(before, after model.IssueOpsIntentContract) []string
  func NormalizeIntentMetadata(intent *model.IssueOpsIntentContract) error
  ```

  Use `json.Marshal`, SHA-256, and lowercase hex. Do not sort semantic lists after record time; their cleaned input order is part of the contract. `IntentChangedFields` returns sorted JSON field names from the projection.

- [ ] **Step 4: Extend model types and bound revisions.**

  Add the chosen metadata fields, `IssueOpsIntentSeal`, `IntentHistory`, and `IntentSeal`. Define:

  ```go
  const MaxIssueOpsIntentRevisions = 32
  const MaxIssueOpsIntentChangeReasonBytes = 1024
  ```

  History stores only prior revisions; the current revision remains `record.Intent`. Revision 33 is rejected rather than dropping old entries.

- [ ] **Step 5: Split initial record, revision, and seal core operations.**

  In `intent_design.go`:

  ```go
  func RecordIntent(store Store, stateRoot, id string, req model.IssueOpsIntentRecordRequest) (model.IssueOpsRecord, error)
  func ReviseIntent(store Store, stateRoot, id string, req model.IssueOpsIntentReviseRequest) (model.IssueOpsRecord, error)
  func SealIntent(store Store, stateRoot, id string) (model.IssueOpsRecord, error)
  ```

  Initial record sets revision 1 and digest. Revision reuses the exact same content validation/cleaning function, requires a redacted reason of at least 10 bytes after trimming, computes changed fields, appends the old value, and clears any old seal. Seal stores only current revision/digest/time and returns byte-equivalent state when already sealed to that pair.

- [ ] **Step 6: Bump IssueOps schema to 9 and migrate legacy intent metadata.**

  - Accept raw versions 0–9; reject 10+ until T9.
  - For v0–v8 with an intent lacking revision/digest, set revision 1 and compute digest deterministically during decode/normalization.
  - Do not fabricate history, change reason, changed fields, or a seal.
  - Stamp v9 only on a subsequent successful write.
  - Add a frozen v8 decoder/write fixture that rejects v9 bytes before changing the stored row.

- [ ] **Step 7: Run focused GREEN and schema regression.**

  ```bash
  go test ./internal/core/issueops/intentdesign -count=1 -v
  go test ./internal/core/issueops -run 'Intent|SchemaVersion' -count=1 -v
  go test -race ./internal/core/issueops/intentdesign ./internal/core/issueops -run 'Intent|SchemaVersion' -count=1
  ```

  Expected: all pass; existing v1–v8 migration tests remain green.

**Must NOT do:** Store full request history outside the record, sort semantic lists, truncate revision history, or infer a seal for legacy records.

**Recommended Agent:** deep — canonical hashing and durable schema compatibility are security-sensitive.

**Parallelization:** Can Parallel: NO | Wave 2 | Blocks: T5 | Blocked By: T3

**Acceptance Criteria:**
- [ ] Canonical digest is stable across timestamps/revision metadata.
- [ ] Overwrite is impossible through `record`.
- [ ] Revision history and supersession chain are exact and bounded.
- [ ] v8 migration is deterministic; frozen v8 rejects v9 byte-preservingly.

**QA Scenarios:**

```text
Scenario: Two material revisions
  Channel: bash/go test
  Steps: Record revision 1; revise success criteria with reason; revise constraint with reason.
  Expected: current revision=3, history length=2, each supersedes hash matches prior current hash, changed_fields exact.
  Evidence: TestReviseIntentAppendsHistoryAndSupersedesDigest.

Scenario: Metadata-only rewrite attempt
  Channel: bash/go test
  Steps: Submit normalized content identical to current with a different reason.
  Expected: intent_revision_no_material_change and stored bytes unchanged.
  Evidence: named negative test.
```

**Commit:** NO — stage with T5–T7.

### Task 5: Enforce intent phase rules, plan seal gate, regression invalidation, and handoff binding

**Files:**
- Modify: `internal/core/issueops/package.go`
- Modify: `internal/core/issueops/issueops_readiness.go`
- Modify: `internal/core/issueops/issueops_phase.go`
- Modify: `internal/core/issueops/issueops_regress.go`
- Modify: `internal/core/issueops/handoff/context.go`
- Modify: `internal/core/issueops/handoff/context_test.go`
- Modify: `internal/core/issueops/issueops_intent_design_test.go`
- Modify: `internal/core/issueops/issueops_regress_test.go`
- Modify: `internal/core/issueops/issueops_phase_gates_test.go`
- Modify: `internal/core/issueops/issueops_handoff_dispatch_test.go`

**Interfaces:**
- Consumes: Task 4 intent operations and current phase/readiness/handoff context.
- Produces: actor-aware revise/seal facades, phase restrictions, `intent_seal` readiness key, sealed digest in context projection.

- [ ] **Step 1: Write RED lifecycle tests.**

  Add exact tests:

  ```go
  func TestIssueOpsIntentRevisionAllowedOnlyBeforePlan(t *testing.T)
  func TestIssueOpsPlanEntryRequiresCurrentIntentSeal(t *testing.T)
  func TestIssueOpsIntentSealRequiresGrillAndIsIdempotent(t *testing.T)
  func TestIssueOpsRegressionClearsSealAndRequiresReseal(t *testing.T)
  func TestIssueOpsHandoffContextBindsIntentRevisionAndDigest(t *testing.T)
  func TestIssueOpsHandoffRejectsMissingOrStaleIntentSeal(t *testing.T)
  ```

- [ ] **Step 2: Run lifecycle RED.**

  ```bash
  go test ./internal/core/issueops -run 'IntentRevisionAllowed|PlanEntryRequiresCurrentIntentSeal|IntentSealRequires|RegressionClearsSeal|HandoffContextBindsIntent|HandoffRejects.*IntentSeal' -count=1 -v
  ```

  Expected: missing facade/gate/context fields or current plan entry acceptance produces FAIL.

- [ ] **Step 3: Add actor-aware core facades.**

  Add exported aliases/requests in `package.go` and functions matching the current `RecordIssueOpsIntentWithActor` pattern:

  ```go
  func ReviseIssueOpsIntent(stateRoot, id string, req IssueOpsIntentReviseRequest) (IssueOpsRecord, error)
  func ReviseIssueOpsIntentWithActor(stateRoot, id string, req IssueOpsIntentReviseRequest, actor IssueOpsActor) (IssueOpsRecord, error)
  func SealIssueOpsIntent(stateRoot, id string) (IssueOpsRecord, error)
  func SealIssueOpsIntentWithActor(stateRoot, id string, actor IssueOpsActor) (IssueOpsRecord, error)
  ```

  Under the existing IssueOps lock, run workspace/ownership actor validation first. Revision accepts only `problem` or `grill`; seal accepts only `grill`. Reject `plan` or later with guidance to complete Brooks stop→reflect→regress, and reject `implement` or later with guidance to start a new cycle.

- [ ] **Step 4: Add the pure seal readiness predicate.**

  Add:

  ```go
  func issueOpsIntentSealMissing(record IssueOpsRecord) []string
  ```

  It returns `intent_seal` unless intent and seal exist, revision matches, both hashes are valid lowercase SHA-256, and hashes match. Call it from `IssueOpsPlanReadiness` and `issueOpsBaseImplementationMissing`, ensuring compatibility-review/implement cannot bypass the plan seal through non-sequential calls.

- [ ] **Step 5: Invalidate only the seal during existing Brooks regression.**

  In `regressIssueOpsForReplanLocked`, after the stop/reflect/cap/children gates pass and before writing, set `record.IntentSeal=nil`. Retain current intent and history. Existing design approval invalidation, stale ledger, review clearing, and regress event behavior remain unchanged.

- [ ] **Step 6: Bind handoff context to the seal without changing context protocol version.**

  Add to `ContextProjection`:

  ```go
  IntentRevision int    `json:"intent_revision,omitempty"`
  IntentSHA256   string `json:"intent_sha256,omitempty"`
  ```

  `BuildContext` requires a current matching seal, copies its revision/hash, and leaves `ContextVersion=1`; this is an additive projection protected from old writers by root schema v9. `contextSourceProjection` must retain both fields so the immutable source digest changes if intent identity changes.

- [ ] **Step 7: Run focused GREEN and actor/handoff regressions.**

  ```bash
  go test ./internal/core/issueops -run 'Intent|PlanEntry|Regression|Handoff.*Context|OwnershipStart' -count=1 -v
  go test ./internal/core/lifecycle -run 'IssueOps|Handoff' -count=1
  go test -race ./internal/core/issueops -run 'Intent|Regression|Handoff' -count=1
  ```

  Expected: all pass; missing seal blocks before any external Orca client call.

**Must NOT do:** Create a second regression path, auto-revise during regress, clear intent history, or permit revision after execution ownership begins.

**Recommended Agent:** deep — high-blast-radius phase and handoff authority gates.

**Parallelization:** Can Parallel: NO | Wave 2 | Blocks: T6 | Blocked By: T4

**Acceptance Criteria:**
- [ ] Plan and downstream readiness require the current seal.
- [ ] Revision is limited to problem/grill under existing actor fences.
- [ ] Brooks regress clears seal and forces explicit reseal.
- [ ] Handoff context/source digest contains exact current intent identity.

**QA Scenarios:**

```text
Scenario: Valid seal-to-plan flow
  Channel: bash/go test
  Steps: Record intent→enter grill→record grill artifacts→seal→enter plan.
  Expected: plan succeeds; seal revision/hash equal current intent.
  Evidence: TestIssueOpsPlanEntryRequiresCurrentIntentSeal.

Scenario: Stale seal after regression
  Channel: bash/go test
  Steps: Seal→plan→record stop/reflection→regress→attempt plan without reseal.
  Expected: phase=grill, intent history retained, plan fails missing intent_seal.
  Evidence: TestIssueOpsRegressionClearsSealAndRequiresReseal.
```

**Commit:** NO — stage with T4/T6/T7.

### Task 6: Add intent revise/seal CLI and MCP surfaces with goldens

**Files:**
- Modify: `cmd/harness/issueopscli/issueops_intent_design.go`
- Modify: `cmd/harness/issueopscli/issueops_cli_support.go`
- Create: `cmd/harness/issueopscli/issueops_intent_design_test.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops_handlers.go`
- Modify: `cmd/harness/mcpcli/issueops/intent_design_test.go`
- Modify: `internal/adapter/mcp/issueops_catalog.go`
- Modify: `internal/adapter/mcp/issueops_catalog_test.go`
- Modify: `internal/adapter/cli/usage.go`
- Modify: `cmd/harness/testdata/usage.golden.txt`
- Modify: `cmd/harness/testdata/mcp_tools.golden.json`
- Modify: `cmd/harness/testdata/response_contracts.golden.json`

**Interfaces:**
- Consumes: Task 5 actor-aware core operations.
- Produces: `issueops intent revise`, `issueops intent seal`, `issueops_revise_intent`, `issueops_seal_intent`.

- [ ] **Step 1: Add adapter RED tests for the complete lifecycle.**

  Use the same isolated record fixture for CLI and MCP and assert record→duplicate record failure→revise→seal. Test actor fields are passed through unchanged in workspace-preparation mode.

- [ ] **Step 2: Run adapter RED.**

  ```bash
  go test ./cmd/harness/issueopscli ./cmd/harness/mcpcli/issueops ./internal/adapter/mcp -run 'Intent.*Revise|Intent.*Seal|DuplicateIntent' -count=1 -v
  ```

- [ ] **Step 3: Implement exact CLI commands.**

  ```text
  agent-harness issueops intent revise --id ID --raw-request TEXT --interpreted-intent TEXT --success-criteria TEXT --change-reason TEXT [--constraint TEXT] [--ambiguity TEXT] [--non-goal TEXT] [--intent-class CLASS] [actor flags] [--json]
  agent-harness issueops intent seal --id ID [actor flags] [--json]
  ```

  Reuse one flag-construction/parser helper for record/revise content; do not duplicate cleaning/validation in CLI.

- [ ] **Step 4: Implement exact MCP tools.**

  - `issueops_revise_intent`: same content properties as record plus required `change_reason` and standard actor properties.
  - `issueops_seal_intent`: required `id`, optional standard actor properties.
  - Descriptions must identify both as state writes, phase restrictions, and returned `IssueOpsRecord`.

- [ ] **Step 5: Run adapter GREEN and regenerate contract goldens once.**

  ```bash
  go test ./cmd/harness/issueopscli ./cmd/harness/mcpcli/issueops ./internal/adapter/mcp -run 'Intent' -count=1
  go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -update -count=1
  git diff -- cmd/harness/testdata/usage.golden.txt cmd/harness/testdata/mcp_tools.golden.json cmd/harness/testdata/response_contracts.golden.json
  go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1
  ```

  Expected: two command/tool additions and intent response fields only; unrelated tools/responses unchanged.

**Must NOT do:** Turn `record` into an implicit upsert, expose an unseal command, or let adapters choose phase behavior.

**Recommended Agent:** deep — public contract and actor parity must remain exact.

**Parallelization:** Can Parallel: NO | Wave 2 | Blocks: T7 | Blocked By: T5

**Acceptance Criteria:**
- [ ] CLI and MCP cover record/revise/seal with identical DTOs.
- [ ] Duplicate record fails identically on both transports.
- [ ] Two and only two new MCP tools are added.
- [ ] Golden diff contains no unrelated churn.

**QA Scenarios:**

```text
Scenario: CLI revision lifecycle
  Channel: bash/go test
  Steps: Start fixture→record→revise with reason→enter grill→seal.
  Expected: revision=2, history=1, seal matches revision/hash.
  Evidence: CLI intent lifecycle test.

Scenario: MCP revision after plan
  Channel: bash/go test
  Steps: Put fixture in plan and call issueops_revise_intent.
  Expected: error directs Brooks regression; record bytes unchanged.
  Evidence: MCP negative test.
```

**Commit:** NO — commit after T7.

### Task 7: Document and verify the intent increment as an atomic rollback unit

**Files:**
- Modify: `.agent-harness/ARCHITECTURE.md`
- Modify: `.agent-harness/CONVENTIONS.md`
- Modify: `.agent-harness/OPERATIONS.md`
- Modify: `.agent-harness/CAUTIONS.md`
- Modify: `.agent-harness/ADR.md`
- Modify: `.agent-harness/AGENT_WORKFLOW.md`
- Verify: all T4–T6 files and generated goldens.

**Interfaces:**
- Consumes: complete schema-v9 intent contract.
- Produces: normative workflow, second verified atomic commit.

- [ ] **Step 1: Update project docs.**

  - ARCHITECTURE/CONVENTIONS: current intent is versioned; plan requires explicit current seal; hashes exclude volatile metadata.
  - AGENT_WORKFLOW/OPERATIONS: missing `intent_seal` maps to `issueops intent seal`; post-plan changes require Brooks stop→reflect→regress, then revise/reseal.
  - CAUTIONS: do not lower schema or manually edit intent hash/history; seal is not user approval beyond the recorded workflow evidence.
  - ADR: schema v9, v8 migration, frozen-writer boundary, revision cap, rejected implicit upsert/unseal/automatic regression.

- [ ] **Step 2: Run an isolated CLI workflow smoke.**

  Build the binary, create an isolated IssueOps record through existing test/smoke helpers or CLI, then execute record→revise→grill prerequisites→seal→plan. Assert with `jq`:

  ```text
  .intent.revision == 2
  .intent_history | length == 1
  .intent_seal.revision == .intent.revision
  .intent_seal.sha256 == .intent.sha256
  .phase == "plan"
  ```

  Also attempt a second `intent record` and a `revise` in plan; both must return nonzero and preserve the prior `issueops status --json` canonical digest.

- [ ] **Step 3: Run schema-v9 full verification.**

  ```bash
  git diff --check
  gofmt -w internal/core/issueops/model/types.go internal/core/issueops/intentdesign/intent_contract.go internal/core/issueops/intentdesign/intent_design.go internal/core/issueops/issueops_state.go internal/core/issueops/package.go internal/core/issueops/issueops_readiness.go internal/core/issueops/issueops_phase.go internal/core/issueops/issueops_regress.go internal/core/issueops/handoff/context.go cmd/harness/issueopscli/issueops_intent_design.go cmd/harness/mcpcli/mcp_tool_issueops.go cmd/harness/mcpcli/mcp_tool_issueops_handlers.go internal/adapter/mcp/issueops_catalog.go
  go test ./internal/core/issueops/... ./cmd/harness/issueopscli ./cmd/harness/mcpcli/... ./internal/adapter/mcp -count=1
  go test ./... -count=1
  go test -race ./... -count=1
  go vet ./...
  go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1
  go build -o bin/agent-harness ./cmd/harness
  ```

- [ ] **Step 4: Commit exact intent paths with Lore.**

  ```text
  feat(issueops): version and seal intent contracts
  ```

  Lore records schema v9, legacy migration without fabricated seal, Brooks regress behavior, handoff context binding, commands run, and rollback boundary.

**Must NOT do:** Start audit-journal code before this commit passes; install or push.

**Recommended Agent:** deep — durable authority and handoff source identity changed.

**Parallelization:** Can Parallel: NO | Wave 2 | Blocks: T8 | Blocked By: T6

**Acceptance Criteria:**
- [ ] Isolated lifecycle smoke proves exact current seal and negative immutability cases.
- [ ] Full/race/vet/build/golden gates pass.
- [ ] One atomic intent commit contains only T4–T7 paths.

**QA Scenarios:**

```text
Scenario: Revision, seal, and plan
  Channel: bash
  Steps: Execute isolated workflow and the five jq assertions.
  Expected: all predicates true and commands exit 0.
  Evidence: smoke JSON files in disposable state before cleanup.

Scenario: Frozen v8 writer
  Channel: bash/go test
  Steps: Feed a v9 row to frozen v8 read-modify-write fixture.
  Expected: unsupported schema error and raw stored bytes unchanged.
  Evidence: TestIssueOpsSchemaV8IntentMigratesAndFrozenV8RejectsV9.
```

**Commit:** YES | Message: `feat(issueops): version and seal intent contracts` | Files: exact T4–T7 paths.

### Task 8: Add an atomic snapshot-upsert plus event-insert sqlstore primitive

**Files:**
- Modify: `internal/core/sqlstore/sqlstore.go`
- Modify: `internal/core/sqlstore/sqlstore_test.go`
- Create: `internal/core/sqlstore/put_with_append_test.go`

**Interfaces:**
- Consumes: one open SQLite data handle and existing `records` primary key.
- Produces: `DB.PutWithAppend`, which inserts one immutable event and upserts one snapshot in one transaction.

- [ ] **Step 1: Write atomicity RED tests.**

  Add exact tests:

  ```go
  func TestPutWithAppendCommitsEventAndSnapshotAtomically(t *testing.T)
  func TestPutWithAppendDuplicateEventRollsBackSnapshot(t *testing.T)
  func TestPutWithAppendSnapshotFailureRollsBackEvent(t *testing.T)
  func TestPutWithAppendDoesNotOverwriteExistingEvent(t *testing.T)
  ```

  The snapshot-failure test creates a disposable SQLite trigger that aborts updates for the test snapshot bucket, then verifies the event row is absent after failure.

- [ ] **Step 2: Run sqlstore RED.**

  ```bash
  go test ./internal/core/sqlstore -run 'PutWithAppend' -count=1 -v
  ```

  Expected: compile failure on missing method.

- [ ] **Step 3: Implement the minimal transaction API.**

  Exact signature:

  ```go
  func (d *DB) PutWithAppend(snapshotBucket, snapshotID string, snapshotData []byte, eventBucket, eventID string, eventData []byte) error
  ```

  Implementation order inside one `database/sql` transaction:

  ```sql
  INSERT INTO records (bucket,id,data) VALUES (eventBucket,eventID,eventData);
  INSERT INTO records (bucket,id,data) VALUES (snapshotBucket,snapshotID,snapshotData)
    ON CONFLICT (bucket,id) DO UPDATE SET data=excluded.data;
  COMMIT;
  ```

  Validate non-empty bucket/ID and non-nil data before `BeginTx`. On any statement or commit error, call rollback and return a wrapped error identifying `append event`, `upsert snapshot`, or `commit`. Do not retry or convert a duplicate event into success.

- [ ] **Step 4: Prove the existing primary key serves the new access pattern.**

  In a disposable test DB containing 10,000 `issueops-audit` rows, execute and capture:

  ```sql
  EXPLAIN QUERY PLAN
  SELECT id FROM records
  WHERE bucket='issueops-audit'
  ORDER BY id;
  ```

  Assert the plan uses the primary key and does not require a temp B-tree. Do not add another index.

- [ ] **Step 5: Run GREEN, race, and existing crash tests.**

  ```bash
  go test ./internal/core/sqlstore -run 'PutWithAppend' -count=1 -v
  go test ./internal/core/sqlstore -count=1
  go test -race ./internal/core/sqlstore -count=1
  go test ./internal/core/sqlstore -run 'Crash|Process|Span' -count=1
  ```

**Must NOT do:** Add a new table, index, generic unit-of-work abstraction, nested span, external I/O, retry loop, or event UPSERT.

**Recommended Agent:** deep — transaction rollback and insert-only semantics are foundational correctness.

**Parallelization:** Can Parallel: NO | Wave 3 | Blocks: T9 | Blocked By: T7

**Acceptance Criteria:**
- [ ] Duplicate event leaves the old snapshot byte-identical.
- [ ] Snapshot failure leaves no event row.
- [ ] Successful call commits both rows.
- [ ] Primary-key query plan is captured; no index is added.

**QA Scenarios:**

```text
Scenario: Atomic success
  Channel: bash/go test
  Steps: Call PutWithAppend on absent event and existing snapshot.
  Expected: new event bytes and new snapshot bytes both readable.
  Evidence: TestPutWithAppendCommitsEventAndSnapshotAtomically.

Scenario: Duplicate sequence
  Channel: bash/go test
  Steps: Insert event ID once; call PutWithAppend with same event ID and changed snapshot.
  Expected: unique constraint error; original event and snapshot bytes unchanged.
  Evidence: duplicate event tests.
```

**Commit:** NO — stage with T9–T12.

### Task 9: Implement the stable critical projection, event hash chain, verifier, and IssueOps schema v10

**Files:**
- Modify: `internal/core/issueops/model/types.go`
- Create: `internal/core/issueops/auditjournal/types.go`
- Create: `internal/core/issueops/auditjournal/projection.go`
- Create: `internal/core/issueops/auditjournal/journal.go`
- Create: `internal/core/issueops/auditjournal/projection_test.go`
- Create: `internal/core/issueops/auditjournal/journal_test.go`
- Modify: `internal/core/issueops/issueops_state.go`
- Create: `internal/core/issueops/issueops_audit.go`
- Create: `internal/core/issueops/issueops_audit_test.go`
- Modify: `internal/core/issueops/issueops_schema_version_test.go`

**Interfaces:**
- Consumes: Task 8 `PutWithAppend`, schema-v9 records, model-only critical fields.
- Produces: `IssueOpsAuditHead`, `IssueOpsAuditEvent`, projection/hash helpers, audited write path, read-only verification, IssueOps schema v10.

- [ ] **Step 1: Write projection and chain RED tests.**

  Add exact tests:

  ```go
  func TestCriticalProjectionIsDeterministicAndExcludesVolatileFields(t *testing.T)
  func TestCriticalProjectionChangesForEveryEnumeratedMilestone(t *testing.T)
  func TestAuditEventHashChainIsDeterministic(t *testing.T)
  func TestAuditedWriteCommitsSnapshotAndEventTogether(t *testing.T)
  func TestAuditVerifyDetectsGapTamperAndProjectionDrift(t *testing.T)
  func TestOrdinaryWriteRejectsUnauditedCriticalProjectionChange(t *testing.T)
  func TestOrdinaryWriteAllowsNonCriticalChangeWithoutEvent(t *testing.T)
  func TestIssueOpsSchemaV9MigratesProspectivelyAndFrozenV9RejectsV10(t *testing.T)
  ```

- [ ] **Step 2: Run audit RED.**

  ```bash
  go test ./internal/core/issueops/auditjournal ./internal/core/issueops -run 'CriticalProjection|AuditEvent|AuditedWrite|AuditVerify|Unaudited|SchemaV9' -count=1 -v
  ```

  Expected: missing packages/types/helpers cause compile failure.

- [ ] **Step 3: Implement the closed projection contract.**

  `CriticalProjection(record)` returns a JSON-marshalable struct with exactly:

  ```go
  type CriticalProjection struct {
	IntentRevision     int    `json:"intent_revision,omitempty"`
	IntentSHA256       string `json:"intent_sha256,omitempty"`
	IntentSealRevision int    `json:"intent_seal_revision,omitempty"`
	IntentSealSHA256   string `json:"intent_seal_sha256,omitempty"`
	Phase              string `json:"phase"`
	WorkspaceReady     bool   `json:"workspace_ready,omitempty"`
	WorkspaceEpoch     string `json:"workspace_epoch,omitempty"`
	OwnershipMilestone string `json:"ownership_milestone,omitempty"`
	ProtocolVersion    int    `json:"protocol_version,omitempty"`
	Attempt            int    `json:"attempt,omitempty"`
	OwnershipEpoch     string `json:"ownership_epoch,omitempty"`
	ClosedDisposition  string `json:"closed_disposition,omitempty"`
	CompletionFinalHead string `json:"completion_final_head,omitempty"`
  }
  ```

  `OwnershipMilestone` is derived only for protocol v2: `dispatched`, `claimed`, `oriented`, `completed`, `cleanup_started`, or `closed`. Pre-dispatch internal states and recovery details do not alter the projection. `WorkspaceReady` becomes true only for the final ready workspace, never provisioning.

- [ ] **Step 4: Implement event canonicalization and verification.**

  Add:

  ```go
  func ProjectionSHA256(record model.IssueOpsRecord) (string, error)
  func BuildEvent(before, after model.IssueOpsRecord, kind, actor, now string) (model.IssueOpsAuditEvent, model.IssueOpsAuditHead, error)
  func EventSHA256(event model.IssueOpsAuditEvent) (string, error)
  func VerifyChain(record model.IssueOpsRecord, events []model.IssueOpsAuditEvent) (model.IssueOpsAuditVerification, error)
  ```

  Validate event kind against the closed enum, require a projection change, require `before.AuditHead` to match the prior event, increment sequence exactly once, and cap actor at the existing redacted host/session identity projection. Empty history starts sequence 1 with empty previous-event hash.

- [ ] **Step 5: Add the audited IssueOps write path.**

  In `issueops_audit.go`, add:

  ```go
  func writeIssueOpsWithAudit(stateRoot string, before, after IssueOpsRecord, kind string, actor *IssueOpsActor, now string) (IssueOpsRecord, error)
  func VerifyIssueOpsAudit(stateRoot, id string) (IssueOpsAuditVerification, error)
  ```

  The write helper validates/normalizes `after`, builds event/head, assigns `after.AuditHead`, serializes both, then calls `db.PutWithAppend("issueops", id, snapshot, "issueops-audit", eventID, event)`. It runs only while the caller already owns the IssueOps span lock and performs no filesystem/network/Orca calls.

- [ ] **Step 6: Guard ordinary schema-v10 writes against missing events.**

  Before `db.Put` in `writeIssueOps`, read the previous row when present. If both are current/auditable and the stable projection digest changes without a valid one-step `AuditHead` advance, reject `issueops_audit_event_required`. Permit noncritical field changes when the critical digest and head remain unchanged. Initial record creation remains allowed without a fabricated event.

- [ ] **Step 7: Bump schema to 10 and add prospective migration.**

  Accept 0–10 and reject 11+. V9 records keep `AuditHead=nil` until their next audited critical transition. Do not scan old decisions/regress/handoff history to synthesize events. Add frozen v9 rejection and current v9→v10 next-write tests.

- [ ] **Step 8: Implement read-only verification and tamper fixtures.**

  `VerifyIssueOpsAudit` reads the snapshot and only `issueops-audit` IDs with exact `<id>/` prefix, sorts by zero-padded ID, decodes strictly, and validates sequence/hash/head/projection. It never creates a missing DB/root or rewrites state. Tests directly tamper a disposable event row to prove each failure class.

- [ ] **Step 9: Run focused GREEN, race, and query-plan evidence.**

  ```bash
  go test ./internal/core/issueops/auditjournal ./internal/core/issueops -run 'CriticalProjection|Audit|SchemaV9' -count=1 -v
  go test -race ./internal/core/issueops/auditjournal ./internal/core/issueops -run 'Audit' -count=1
  sqlite3 -readonly "$DISPOSABLE_AUDIT_DB" "EXPLAIN QUERY PLAN SELECT id FROM records WHERE bucket='issueops-audit' ORDER BY id;"
  ```

  Expected: all tests pass; plan uses the primary key; no new index/DDL appears.

**Must NOT do:** Reconstruct current state from events, log full snapshots/free-form intent/reasons, journal volatile fields, or auto-repair a broken chain.

**Recommended Agent:** deep — schema, hashing, transaction, and compatibility boundaries converge here.

**Parallelization:** Can Parallel: NO | Wave 3 | Blocks: T10 | Blocked By: T8

**Acceptance Criteria:**
- [ ] Projection changes exactly for selected critical fields and ignores volatile fields.
- [ ] Event and snapshot are atomic and insert-only.
- [ ] Ordinary writes cannot bypass a required event.
- [ ] Verifier detects all four corruption classes without writing.
- [ ] v9 migration is prospective; frozen v9 rejects v10.

**QA Scenarios:**

```text
Scenario: Valid three-event chain
  Channel: bash/go test
  Steps: Apply intent_recorded→intent_sealed→phase_advanced through audited writes.
  Expected: sequence 1..3, previous hash linkage exact, head equals event 3 and current projection.
  Evidence: TestAuditedWriteCommitsSnapshotAndEventTogether.

Scenario: Missing middle event
  Channel: bash/go test
  Steps: Delete sequence 2 directly in disposable DB; call VerifyIssueOpsAudit.
  Expected: issueops_audit_sequence_gap, no state rewrite.
  Evidence: TestAuditVerifyDetectsGapTamperAndProjectionDrift.
```

**Commit:** NO — stage with T8/T10–T12.

### Task 10: Route intent and phase milestones through the audited write path

**Files:**
- Modify: `internal/core/issueops/intentdesign/intent_design.go`
- Modify: `internal/core/issueops/intentdesign/intent_design_test.go`
- Modify: `internal/core/issueops/package.go`
- Modify: `internal/core/issueops/issueops_phase.go`
- Modify: `internal/core/issueops/issueops_regress.go`
- Modify: `internal/core/issueops/issueops_phase_lifecycle_test.go`
- Modify: `internal/core/issueops/issueops_regress_test.go`
- Modify: `internal/core/issueops/issueops_audit_test.go`

**Interfaces:**
- Consumes: Task 9 audited write helper and event kinds.
- Produces: events for intent record/revise/seal and phase advance/regress; bypass guard coverage.

- [ ] **Step 1: Write RED integration tests for all five transition kinds.**

  Each test performs the public operation, calls `VerifyIssueOpsAudit`, and asserts one exact event kind/sequence/projection change. Add a bypass test that invokes the old `TouchWrite` path with a changed intent or phase and expects `issueops_audit_event_required` with byte-identical snapshot/event rows.

- [ ] **Step 2: Run integration RED.**

  ```bash
  go test ./internal/core/issueops -run 'Audit.*Intent|Audit.*Phase|Audit.*Regress|AuditBypass' -count=1 -v
  ```

- [ ] **Step 3: Extend the intentdesign Store with a transition writer.**

  Add one callback:

  ```go
  WriteTransition func(stateRoot string, before, after model.IssueOpsRecord, kind, now string) (model.IssueOpsRecord, error)
  ```

  Record/revise/seal preserve the initially-read `before`, use one injected `now` for metadata and event, and call `WriteTransition` with `intent_recorded`, `intent_revised`, or `intent_sealed`. Do not call both `TouchWrite` and `WriteTransition`.

- [ ] **Step 4: Audit forward phase transitions and regressions.**

  - `advanceIssueOpsPhaseLocked` keeps `before`, applies the pure transition, and writes `phase_advanced` with the wrapper-owned time.
  - `regressIssueOpsForReplanLocked` keeps `before`, applies existing mutations plus seal clearing, and writes `phase_regressed` with the same `now` already used by the regress event/decision.
  - Same-phase read-only/idempotent paths append no event.

- [ ] **Step 5: Run focused GREEN and full IssueOps package regression.**

  ```bash
  go test ./internal/core/issueops -run 'Audit.*Intent|Audit.*Phase|Audit.*Regress|AuditBypass' -count=1 -v
  go test ./internal/core/issueops/... -count=1
  go test -race ./internal/core/issueops/... -count=1
  ```

**Must NOT do:** Add events for artifact recorders, status reads, heartbeats, or same-phase idempotent calls.

**Recommended Agent:** deep — this replaces writes at central phase/intent mutation boundaries.

**Parallelization:** Can Parallel: NO | Wave 3 | Blocks: T11 | Blocked By: T9

**Acceptance Criteria:**
- [ ] Five transition kinds append exactly once through public operations.
- [ ] Same-phase/idempotent operations append zero events.
- [ ] Old mutation paths fail closed for critical projection changes.
- [ ] Full IssueOps race suite passes.

**QA Scenarios:**

```text
Scenario: Intent-to-plan audit chain
  Channel: bash/go test
  Steps: record intent→enter grill→seal→enter plan.
  Expected: intent_recorded, phase_advanced, intent_sealed, phase_advanced in exact order.
  Evidence: integration chain test.

Scenario: Direct critical write bypass
  Channel: bash/go test
  Steps: Mutate current intent digest and call ordinary write.
  Expected: issueops_audit_event_required; snapshot and events byte-identical.
  Evidence: AuditBypass test.
```

**Commit:** NO — stage with T8–T12.

### Task 11: Journal the selected protocol-v2 workspace and ownership milestones

**Files:**
- Modify: `internal/core/issueops/issueops_handoff_prepare.go`
- Modify: `internal/core/issueops/issueops_handoff_prepare_test.go`
- Modify: `internal/core/issueops/issueops_handoff_dispatch.go`
- Modify: `internal/core/issueops/issueops_handoff_dispatch_test.go`
- Modify: `internal/core/issueops/issueops_handoff_lifecycle.go`
- Modify: `internal/core/issueops/issueops_handoff_lifecycle_test.go`
- Modify: `internal/core/issueops/issueops_handoff_orientation.go`
- Modify: `internal/core/issueops/issueops_handoff_orientation_test.go`
- Modify: `internal/core/issueops/issueops_ownership_completion.go`
- Modify: `internal/core/issueops/issueops_ownership_completion_test.go`
- Modify: `internal/core/issueops/issueops_ownership_cleanup.go`
- Modify: `internal/core/issueops/issueops_ownership_cleanup_test.go`
- Modify: `internal/core/issueops/issueops_audit_test.go`

**Interfaces:**
- Consumes: Task 9 audited write helper and the stable protocol-v2 ownership projection.
- Produces: exactly-once events for workspace readiness and public ownership milestones.

- [ ] **Step 1: Write RED milestone tests.**

  Add these named tests through public operations:

  ```text
  TestAuditWorkspacePreparedOnce
  TestAuditOwnershipDispatchClaimOrientComplete
  TestAuditOwnershipCleanupStartAndClose
  TestAuditOwnershipMilestonesAreIdempotent
  TestAuditProtocolV1DoesNotEmitProtocolV2Milestones
  TestAuditRecoveryAndHeartbeatDoNotChangeCriticalProjection
  ```

  For each mutation, capture the snapshot and `issueops-audit` rows before and after. Assert the exact event count, kind, sequence, projection digest, and unchanged rows on retry.

- [ ] **Step 2: Run ownership RED.**

  ```bash
  go test ./internal/core/issueops -run 'Audit.*Workspace|Audit.*Ownership|Audit.*ProtocolV1|Audit.*Recovery' -count=1 -v
  ```

- [ ] **Step 3: Replace only the final public milestone writes.**

  Route these existing mutation points through the audited helper:

  | Existing boundary | Event kind | Emit only when |
  |---|---|---|
  | `persistHandoffWorktreeCreate` | `workspace_prepared` | final workspace state becomes ready |
  | `finalizeHandoffDispatch` | `ownership_dispatched` | protocol v2 becomes dispatched |
  | `ClaimIssueOpsHandoff` | `ownership_claimed` | protocol v2 claim succeeds |
  | `AcknowledgeIssueOpsHandoffContext` | `ownership_oriented` | protocol v2 owner becomes active |
  | `CompleteIssueOpsOwnershipTransfer` | `ownership_completed` | completion is accepted |
  | `ApproveIssueOpsOwnershipCleanup` | `ownership_cleanup_started` | cleanup approval becomes executing |
  | `RecordIssueOpsOwnershipCleanup` | `ownership_closed` | cleanup closes the record |

  Each mutation preserves the pre-mutation record, reuses its existing injected `now` and bounded actor identity, and replaces one ordinary snapshot write with one atomic event+snapshot write. Do not add a second write.

- [ ] **Step 4: Preserve the exclusion boundary.**

  Protocol-v1 acceptance, provisioning/intermediate preparation, heartbeat, poll, lease refresh, recovery checkpoint, and pending cleanup operations remain ordinary writes because they do not change the stable critical projection. If one changes that projection, the Task 9 guard must fail rather than silently emit an inferred event.

- [ ] **Step 5: Run focused GREEN and the ownership race suite.**

  ```bash
  go test ./internal/core/issueops -run 'Audit.*Workspace|Audit.*Ownership|Audit.*ProtocolV1|Audit.*Recovery' -count=1 -v
  go test ./internal/core/issueops -run 'Handoff|Ownership|Cleanup|Recovery|Heartbeat' -count=1
  go test -race ./internal/core/issueops -run 'Handoff|Ownership|Cleanup' -count=1
  ```

**Must NOT do:** Journal protocol-v1 pseudo-milestones, low-level recovery states, heartbeats, or retry attempts; change Orca side effects; or emit before the existing external action has reached its current success boundary.

**Recommended Agent:** deep — multiple mutation sites share one closed event contract and must remain idempotent.

**Parallelization:** Can Parallel: NO | Wave 3 | Blocks: T12 | Blocked By: T10

**Acceptance Criteria:**
- [ ] Seven selected event kinds append once at their current authoritative success boundaries.
- [ ] Repeated calls append zero duplicate events.
- [ ] Protocol v1, recovery, and heartbeat paths remain outside the critical journal.
- [ ] Existing handoff side-effect order and failure recovery remain unchanged.

**QA Scenarios:**

```text
Scenario: Full protocol-v2 ownership lifecycle
  Channel: bash/go test
  Steps: prepare ready workspace→dispatch→claim→orient→complete→approve cleanup→close.
  Expected: seven ordered milestone events with a valid final head and no duplicate on retries.
  Evidence: TestAuditOwnershipDispatchClaimOrientComplete plus cleanup test.

Scenario: Recovery noise does not become authority history
  Channel: bash/go test
  Steps: record heartbeat and recovery checkpoints between claim and orientation.
  Expected: snapshot operational fields change; audit sequence and stable projection remain unchanged.
  Evidence: TestAuditRecoveryAndHeartbeatDoNotChangeCriticalProjection.
```

**Commit:** NO — stage with T8–T12.

### Task 12: Expose read-only audit verification through identical CLI and MCP contracts

**Files:**
- Modify: `internal/core/issueops/package.go`
- Modify: `cmd/harness/issueopscli/issueops.go`
- Modify: `cmd/harness/issueopscli/issueops_subcommands.go`
- Create: `cmd/harness/issueopscli/issueops_audit_cli.go`
- Create: `cmd/harness/issueopscli/issueops_audit_cli_test.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops_handlers.go`
- Create: `cmd/harness/mcpcli/issueops/audit_verify_test.go`
- Modify: `internal/adapter/mcp/issueops_catalog.go`
- Modify: `internal/adapter/mcp/issueops_catalog_test.go`
- Modify: `cmd/harness/testdata/usage.golden.txt`
- Modify: `cmd/harness/testdata/mcp_tools.golden.json`
- Modify: `.agent-harness/ARCHITECTURE.md`
- Modify: `.agent-harness/CONVENTIONS.md`
- Modify: `.agent-harness/TESTING.md`
- Modify: `.agent-harness/OPERATIONS.md`
- Modify: `.agent-harness/CAUTIONS.md`
- Modify: `.agent-harness/ADR.md`

**Interfaces:**
- CLI: `agent-harness issueops audit verify --id ID [--json]`
- MCP: read-only tool `issueops_audit_verify` with required string `id`.
- Shared response: Task 9 `IssueOpsAuditVerification`; no adapter-specific DTO.

- [ ] **Step 1: Write RED CLI/MCP parity tests.**

  Cover valid chain, missing record, tampered event, missing middle event, text output, JSON output, and MCP result content. Assert CLI JSON and MCP structured content decode to the same shared DTO and preserve the same stable error tokens.

- [ ] **Step 2: Run adapter RED.**

  ```bash
  go test ./cmd/harness/issueopscli ./cmd/harness/mcpcli/issueops ./internal/adapter/mcp -run 'AuditVerify|IssueOpsAudit' -count=1 -v
  ```

- [ ] **Step 3: Add the minimal read-only surfaces.**

  Add the nested `audit verify` dispatch and one MCP catalog/handler entry. Both call the same core facade and accept no state-root override beyond the repository's existing dependency injection/test seam. Text output reports `id`, `valid`, event count, head sequence, and failure token; JSON/MCP return the complete DTO.

- [ ] **Step 4: Keep verification strictly observational.**

  The command/tool must not create a missing state root or database, append an event, repair a head, export raw intent text, or expose event payloads. Do not add `list`, `show`, `repair`, or `replay` commands.

- [ ] **Step 5: Regenerate public contract goldens once.**

  Use the repository's existing golden update command, inspect the diff, and verify that only the new CLI command and MCP tool/schema appear.

  ```bash
  go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -update -count=1
  git diff -- cmd/harness/testdata/usage.golden.txt cmd/harness/testdata/mcp_tools.golden.json cmd/harness/testdata/response_contracts.golden.json
  go test ./cmd/harness/contractgolden -run Golden -count=1
  go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
  ```

- [ ] **Step 6: Document the authority and compatibility boundaries.**

  State consistently that the snapshot remains current-state authority; the audit stream is prospective evidence, not replay authority; pre-v10 history is not synthesized; verification is read-only; selected protocol-v2 milestones only are journaled; and CLI/MCP expose identical response semantics.

- [ ] **Step 7: Run adapter GREEN and documentation/contract tests.**

  ```bash
  go test ./cmd/harness/issueopscli ./cmd/harness/mcpcli/issueops ./internal/adapter/mcp -run 'AuditVerify|IssueOpsAudit' -count=1 -v
  go test ./cmd/harness/contractgolden -run Golden -count=1
  go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
  ```

**Must NOT do:** Add audit mutation APIs, raw event export, event replay, a dashboard, host-specific semantics, or a second response schema.

**Recommended Agent:** deep — public command/tool parity and golden scope must stay exact.

**Parallelization:** Can Parallel: NO | Wave 3 | Blocks: T13 | Blocked By: T11

**Acceptance Criteria:**
- [ ] CLI and MCP return the same shared verification DTO and error tokens.
- [ ] Verification performs zero writes, including for missing/corrupt state.
- [ ] Goldens change only for the new public surfaces.
- [ ] Six project documents describe the same prospective, snapshot-authoritative contract.

**QA Scenarios:**

```text
Scenario: CLI/MCP verification parity
  Channel: bash/go test
  Steps: Verify the same disposable valid chain through CLI and MCP.
  Expected: decoded DTOs are deeply equal and valid=true.
  Evidence: adapter parity test.

Scenario: Tampered chain is observational
  Channel: bash/go test
  Steps: Corrupt one event, hash the DB file, invoke CLI and MCP, hash again.
  Expected: both return the same failure token; DB hash is unchanged.
  Evidence: tamper parity/no-write test.
```

**Commit:** NO — stage with T8–T12.

### Task 13: Execute the final cross-contract verification wave and close the three atomic changes

**Files:**
- Review only: every file changed by T1–T12
- Update only if a verified defect requires it: the owning task's files and tests

**Interfaces:**
- Consumes: all three schema increments and their CLI/MCP/docs contracts.
- Produces: fresh command evidence, scope audit, and three reviewable local commits; no push/install.

- [ ] **Step 1: Format and inspect before tests.**

  ```bash
  gofmt -w $(git diff --name-only --diff-filter=ACM -- '*.go')
  git diff --check
  git status --short
  git diff --stat
  ```

  Review every changed path against T1–T12. Remove only unused code/imports introduced by this work; do not clean unrelated files.

- [ ] **Step 2: Run all focused contract suites.**

  ```bash
  go test ./internal/core/looprun ./cmd/harness/loopcli ./cmd/harness/mcpcli -run 'Loop|Receipt' -count=1
  go test ./internal/core/issueops/... ./cmd/harness/issueopscli ./cmd/harness/mcpcli/issueops ./internal/adapter/mcp -run 'Intent|Seal|Audit|Handoff|Ownership' -count=1
  go test ./internal/core/sqlstore -run 'PutWithAppend|Prefix' -count=1
  go test ./cmd/harness/contractgolden -run Golden -count=1
  go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
  ```

- [ ] **Step 3: Run repository-wide verification from clean caches.**

  ```bash
  go test ./... -count=1
  go test -race ./... -count=1
  go vet ./...
  go build -o bin/agent-harness ./cmd/harness
  ./bin/agent-harness contract conformance baseline --json
  ./bin/agent-harness contract check --json
  ```

- [ ] **Step 4: Run three isolated executable QA scenarios.**

  Use a distinct `mktemp -d` state root per scenario and delete only those explicit paths after capturing assertions:

  1. Start a loop with stored `verify_argv`, record a structured pass receipt, inspect JSON, and stop successfully; repeat with `skipped` and prove stop is rejected.
  2. Start IssueOps, record intent, revise in grill with a reason, seal revision 2, advance to plan, and prove the snapshot/handoff context carry the same revision/hash.
  3. Build a valid prospective audit chain, verify through CLI, tamper a disposable event via the test seam, and prove read-only verification rejects it without changing bytes.

- [ ] **Step 5: Run the isolated self-verification gate.**

  ```bash
  verification_state="$(mktemp -d)"
  HARNESS_STATE_DIR="$verification_state" ./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
  rm -rf "$verification_state"
  ```

  Require exit 0 and score at least 95. This is deterministic harness verification; it is not an LLM quality claim.

- [ ] **Step 6: Recheck scope and compatibility.**

  Confirm frozen schema readers reject future versions, legacy loop/IssueOps fixtures still load under the documented migration rule, no audit event contains raw intent/change reason/secret-like evidence, and no Ouroboros dependency, scheduler, model router, dashboard, or mandatory LLM gate appears in `go.mod`, source, configs, or docs.

- [ ] **Step 7: Create the three local commits only after their individual gates are green.**

  Stage exact task groups and commit in the order listed under Commit Strategy. Each commit uses the repository's Conventional subject plus `Lore:` body. After the third commit, rerun `go test ./... -count=1`, `go test -race ./... -count=1`, `go vet ./...`, both golden suites, build, contract baseline, and isolated self-verify. Do not push, merge, install, or mutate a remote issue.

**Must NOT do:** Weaken a failing test, skip the race suite, combine all work into one commit, stage unrelated changes, run `install-native`, push, merge, or create/update remote work items.

**Recommended Agent:** deep — one main agent retains cross-schema context and performs the final evidence audit.

**Parallelization:** Can Parallel: NO | Wave 4 | Blocks: none | Blocked By: T3, T7, T12

**Acceptance Criteria:**
- [ ] All focused, full, race, vet, build, golden, baseline, and self-verify commands pass freshly.
- [ ] All three executable QA scenarios produce exact expected JSON/error evidence.
- [ ] The final diff contains only T1–T12 scope and splits into three atomic local commits.
- [ ] Final report states no push/install/remote mutation occurred.

**QA Scenarios:**

```text
Scenario: Fail-closed end-to-end acceptance
  Channel: bash/CLI
  Steps: Run the three isolated scenarios and the full verification matrix.
  Expected: valid paths pass; skipped/tampered/unsealed paths fail with stable tokens; no state leaks between roots.
  Evidence: captured command outputs and final git diff/status.

Scenario: Scope exclusion audit
  Channel: bash/rg/git diff
  Steps: Search changed files for provider routing, scheduler, replay authority, raw secret fields, and install/remote mutations.
  Expected: no out-of-scope implementation or documentation claims.
  Evidence: zero-match searches plus reviewed diff.
```

**Commit:** YES — create the three commits in order only after their respective and final gates pass; never push automatically.

## Final Verification Wave

- [ ] F1. Plan Compliance Audit — map every deliverable and guardrail to a completed task, diff, and passing command.
- [ ] F2. Code Quality Review — reject duplicated DTO semantics, generic event-bus abstractions, unbounded strings/lists, dead compatibility code, or comments that overclaim external command proof.
- [ ] F3. Real CLI/MCP QA — run the three isolated scenarios from T13 and preserve exact JSON assertions until the final report.
- [ ] F4. Scope Fidelity Check — confirm no Ouroboros dependency, scheduler, mandatory LLM gate, provider router, native install, remote mutation, or unrelated cleanup entered the diff.

## Commit Strategy

1. `feat(loop): record fail-closed verification receipts`
   - Owns T1–T3, loop schema v2, adapters, goldens, and directly coupled docs.
2. `feat(issueops): version and seal intent contracts`
   - Owns T4–T7, IssueOps schema v9, handoff context binding, adapters, goldens, and directly coupled docs.
3. `feat(issueops): journal critical workflow transitions`
   - Owns T8–T12, sqlstore atomic append, IssueOps schema v10, audit integration/tooling, goldens, and directly coupled docs.
4. T13 makes no new commit unless verification uncovers a real defect; any fix is folded into the affected intent commit before the full wave restarts.

Every non-trivial commit uses the repository Conventional subject + `Lore:` body and stages only its named task files. Do not push without a separate user request.

## Rollback Strategy

- Roll back in reverse commit order only; never cherry-pick schema v10 without v9 or v9 without loop v2 adapter/golden synchronization.
- Reverting audit v10 removes prospective journal enforcement but leaves `issueops-audit` rows inert; no destructive cleanup is required.
- Reverting intent v9 is safe only before any v9 row is written. Afterward, older binaries correctly reject those rows; rollback requires restoring the v9-capable binary, exporting affected records, and an explicitly approved migration—not manual JSON deletion.
- Reverting loop v2 is safe only before any v2 loop is written. Existing v2 rows must remain handled by a v2-capable binary until terminal.
- Never lower stored `schema_version` or delete seal/audit fields as a rollback shortcut.

## Success Criteria

- All Definition of Done bullets are proven by named tests or exact CLI/MCP assertions.
- Each schema increment is independently buildable, testable, and revertible within its stated compatibility boundary.
- No task requires the implementer to choose a type, field, error token, phase rule, event kind, command surface, file location, or verification command.
- The final report distinguishes structured attestations from independently executed proof and does not claim the harness executed external verification.

## Planning Evidence Contracts

### Karpathy

Input/output contract: Existing IssueOps/loop/state contracts and the approved selective-absorption direction produce one implementation-ready plan; no source implementation, remote mutation, or hidden reasoning output is in scope.

Test suite: Happy paths cover structured pass, intent seal, and valid audit chain; edge paths cover skip/block, legacy rows, revision after plan, old-writer rejection, duplicate event, tamper, and missing event.

Adversarial cases: Caller-supplied command drift is impossible because core copies stored argv; forged pass with non-zero exit, project path drift, secret-like evidence, event overwrite, old-writer field erasure, and LLM self-approval are rejected or excluded.

One-variable iteration: Implement and measure receipt enforcement first; intent sealing and the audit journal start only after the preceding increment passes its full gate.

Privacy/tool truth: The plan uses only current repository symbols/tools; it requests bounded rationale and observable evidence, never private chain-of-thought or fictional tool results.

### Von Neumann

Repo grounding: Current files, symbols, docs, schema versions, live SQLite row counts, and primary-key query plan are listed under Context and each task References block.

Decision-complete plan: One sequential owner executes 13 tasks across three atomic rollback units with fixed DTOs, phase rules, event kinds, schema versions, commands, and commit boundaries.

Assumptions/defaults: Existing approved selective-absorption design, Go TDD, no sub-agent delegation, no live install/remote mutation, and prospective-only audit history.

Unresolved questions: None blocking. Multi-command manifests, semantic LLM gates, full replay, and audit partitioning are explicitly deferred.

Acceptance criteria: Task-local RED/GREEN commands plus the final full/race/vet/build/golden/self-verify wave.

### Codd

Schema/row count: SQLite `records(bucket TEXT,id TEXT,data BLOB,PRIMARY KEY(bucket,id)) WITHOUT ROWID`; live IssueOps 6 rows and session 1,534 rows. Planned audit events reuse the same normalized row key and are expected to remain tiny (<10k rows initially).

EXPLAIN evidence: Current snapshot lookup is `SEARCH records USING PRIMARY KEY (bucket=? AND id=?)`; T8/T9 must capture event-prefix read plans before and after implementation in a disposable database.

Index tradeoff: Use only the existing primary key for `issueops-audit` sequence order. Reject a timestamp index and separate events table initially; they add write/storage cost without a measured query. Reassess only after cardinality or query evidence changes.

Normalization rationale: Snapshot JSON intentionally denormalizes one aggregate for atomic current-state reads. Audit events are separate 1NF rows because they have independent identity/order and append-only semantics; embedding them in the snapshot would amplify every write and allow history replacement.
