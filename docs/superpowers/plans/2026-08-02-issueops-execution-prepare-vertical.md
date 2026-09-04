# IssueOps Execution Prepare Vertical Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the complete `issueops execution prepare --mode auto|direct|orca` capability behind one contract/domain/application/inbound/outbound boundary without changing public JSON, persisted schema-v1 bytes, recovery, or Codex/Claude hook authority.

**Architecture:** `internal/contract/issueopspreparation` owns the stable prepare command/result and the one prepare/resume Orca intent codec while reusing `internal/contract/issueopslease.Record`. A pure domain matrix selects preview, idempotent, deny, direct, or Orca behavior; an application service coordinates six consumer-owned ports. `cmd/issueops/issueopsapp` is the only production composition root, and `internal/core/issueops` ends with compatibility DTOs and granular raw-effect bridges, not a production prepare orchestrator.

**Tech Stack:** Go 1.26.3, standard library, existing `sqlstore`, `port.TransactionalRecordStore`, Git worktree adapter, Orca adapter, table-driven Go tests, GitHub Actions.

## Global Constraints

- Base exactly `739de96aeca540cf7d5cf6333b345a192afcfd59`; branch `199-issueops-execution-prepare-vertical`; PR target `117-hexagonal-architecture-migration`.
- Keep `ExecutionActionRequest`, `ExecutionPrepareRequest`, `ExecutionPrepareResult`, CLI/MCP usage, JSON/text output, errors, `next_command`, defaults, and response goldens compatible.
- Reuse `issueopslease.Record`, `Execution`, `Workspace`, `Lease`, `Actor`, and `OrcaBinding`; do not create another persisted record projection.
- One `issueopspreparation.IntentCodec` encodes, decodes, validates, seals, and canonicalizes prepare-shaped and resume-shaped payloads. Existing reconcile and resume bridges call it.
- Preserve `issueops_v1`, `lease_holder_v1`, `external_intent_v1`, raw CAS/error ordering, fixed stage bounds, and unknown-outcome rules.
- Preserve enabled-hook exact holder, PID-reuse-safe process identity, canonical root, source/structured workdir, symlink containment, generation, and lease-status results for Codex and Claude.
- Do not add a hook bypass or treat `--disable hooks` as product behavior or verification evidence.
- Exclude status, replace, sync-base, switch-mode, cleanup, general hook policy, transport schema, provider protocol, and OpenWiki.
- The main agent executes inline with `superpowers:executing-plans`; the repository working contract excludes implementation subagents.
- Each behavior follows RED → observed expected failure → minimal GREEN → focused regression → atomic Conventional Commit with Lore body.

---

## File Map

Create:

- `internal/contract/issueopspreparation/{prepare,intent}.go` and tests: stable command/result and the only persisted intent codec.
- `internal/domain/issueopspreparation/decision.go` and test: pure decision matrix.
- `internal/application/issueopspreparation/{ports,prepare}.go` and test: six ports and orchestration.
- `internal/adapter/inbound/issueopspreparation/prepare.go` and test: legacy DTO mapping.
- `internal/adapter/outbound/issueopspreparation/{repository,workspace,orca,evidence}.go` and tests: SQLite/Git/Orca/evidence adapters.
- `cmd/issueops/issueopsapp/issueops_preparation_wiring.go` and test: only production composition.
- `internal/core/issueops/execution_prepare_intent_codec_spike_test.go`: prepare/resume byte and recovery proof.
- `internal/core/issueops/execution_prepare_legacy_oracle_test.go`: predecessor orchestration used only by differential tests.
- `internal/core/issueops/execution_prepare_vertical_differential_test.go`: public/state/trace comparison.

Modify surgically:

- `internal/core/issueops/execution_api.go`, `execution_prepare.go`, `execution_orca_intent.go`, `execution_orca_marker.go`, `execution_resume_bridge.go`, `execution_reconcile_bridge.go`.
- `cmd/issueops/issueopscli/issueops_execution_cli.go`, `executioncmd/execution.go`, `issueops.go`.
- `cmd/issueops/mcpcli/mcp_tool_issueops_execution.go`, `mcp_sdk_server.go`.
- `cmd/issueops/issueopsapp/issueops_policy_facade.go`, `mcp_facade.go`.
- `internal/architecture/dependency_test.go`, `internal/architecture/testdata/legacy_imports.txt`.
- `cmd/issueops/hookcli/hook_execution_contract_test.go`.

---

### Task 0: Define the fail-closed prepare handler seam

**Files:** Modify `internal/core/issueops/execution_api.go`; test `internal/core/issueops/execution_contract_test.go`.

**Interfaces:** Produces `ExecutionPrepareHandler`, `ErrPrepareHandlerUnavailable`, and `invokeExecutionPrepareHandler`. The production switch is not cut over before the codec gate.

- [ ] **Step 1: Write the failing seam tests**

```go
func TestInvokeExecutionPrepareHandlerFailsClosed(t *testing.T) {
    got, err := invokeExecutionPrepareHandler(context.Background(), t.TempDir(), ExecutionPrepareRequest{ID: "io-prepare"}, nil)
    if !errors.Is(err, ErrPrepareHandlerUnavailable) || got.ID != "io-prepare" || got.OK {
        t.Fatalf("result=%+v err=%v", got, err)
    }
}

func TestInvokeExecutionPrepareHandlerCallsOnce(t *testing.T) {
    calls := 0
    handler := func(_ context.Context, _ string, request ExecutionPrepareRequest) (ExecutionPrepareResult, error) {
        calls++
        return ExecutionPrepareResult{OK: true, ID: request.ID, ResolvedMode: "direct"}, nil
    }
    got, err := invokeExecutionPrepareHandler(context.Background(), "/state", ExecutionPrepareRequest{ID: "io-prepare"}, handler)
    if err != nil || calls != 1 || !got.OK { t.Fatalf("result=%+v calls=%d err=%v", got, calls, err) }
}
```

- [ ] **Step 2: Verify RED** — Run `go test ./internal/core/issueops -run 'InvokeExecutionPrepareHandler' -count=1`; expect undefined seam symbols.

- [ ] **Step 3: Add the minimal seam**

```go
var ErrPrepareHandlerUnavailable = errors.New("issueops execution prepare handler is not configured")
type ExecutionPrepareHandler func(context.Context, string, ExecutionPrepareRequest) (ExecutionPrepareResult, error)

func invokeExecutionPrepareHandler(ctx context.Context, root string, request ExecutionPrepareRequest, handler ExecutionPrepareHandler) (ExecutionPrepareResult, error) {
    if handler == nil { return ExecutionPrepareResult{ID: request.ID}, ErrPrepareHandlerUnavailable }
    return handler(ctx, root, request)
}
```

- [ ] **Step 4: Verify GREEN** — Run `go test ./internal/core/issueops -run 'InvokeExecutionPrepareHandler|ExecutionPrepare|AutoFallback|RootCollision|ParentWorktree' -count=1`; expect PASS with legacy routing unchanged.

- [ ] **Step 5: Commit** — Stage the two files; run `git diff --cached --check`; commit `refactor(issueops): define prepare handler seam` with Lore covering intent, fail-closed reason, focused verification, and the still-unwired risk.

### Task 1: Establish one prepare/resume intent codec and recovery gate

**Files:** Create `internal/contract/issueopspreparation/intent.go`, its test, and `execution_prepare_intent_codec_spike_test.go`; modify Orca marker/intent and resume/reconcile bridge files.

**Interfaces:** Produces `Intent`, `IntentCodec`, `IssueIdentity`, `Decode`, `Encode`, `Seal`, and `Canonicalize`. `Intent.PriorBinding` and `Intent.ResumeLease` use `issueopslease` types.

- [ ] **Step 1: Write literal exact-byte tests**

Define the persisted intent with the existing JSON field order: schema/purpose/operation/lifecycle/generation/stage/marker/timestamps/invocation/workspace/probe/prepared/launch/digests/terminal/run/task/prior-binding/resume-lease. Use one prepare worktree fixture and one resume terminal fixture with nonempty prior binding and holderless claimable lease.

```go
decoded, err := codec.Decode(operationID, raw)
encoded, encodeErr := codec.Encode(decoded)
if err != nil || encodeErr != nil || !bytes.Equal(encoded, raw) {
    t.Fatalf("decode=%v encode=%v\nwant=%s\n got=%s", err, encodeErr, raw, encoded)
}
```

Also reject wrong operation ID, invalid prepare generation, missing resume authority, altered marker provider/issue, and attempts over `2` with existing error codes/messages.

- [ ] **Step 2: Verify RED** — Run `go test ./internal/contract/issueopspreparation -run Intent -count=1`; expect missing package/types.

- [ ] **Step 3: Implement the codec**

```go
type IntentCodec struct{}
func (IntentCodec) Decode(operationID string, raw []byte) (Intent, error)
func (IntentCodec) Encode(intent Intent) ([]byte, error)
func (IntentCodec) Seal(intent Intent, issue IssueIdentity) (Intent, error)
func (IntentCodec) Canonicalize(record issueopslease.Record, raw []byte) (Intent, []byte, bool, error)
```

Canonical markers remain exactly `issueops-v1 lifecycle=... operation=... provider=... issue=...` and the resume form adds `resume` and `generation=...`. Legacy canonicalization accepts only exact legacy markers with `not_invoked_proven`; identity or invocation ambiguity returns `legacy_intent_upgrade_unsafe`.

- [ ] **Step 4: Integrate existing recovery** — Alias the private payload to the contract intent, add explicit contract↔`internal/port` conversion helpers, and replace direct JSON/marker codec calls in prepare, resume, and reconcile bridges. Keep raw CAS operations unchanged.

- [ ] **Step 5: Add the cross-purpose spike** — Create predecessor prepare and resume intents, capture record/intent bytes, round-trip both, run `CanonicalizeExecutionReconcileIntent`, apply matching receipts, and compare next/terminal bytes. Assert a panic legacy prepare wrapper is never called.

- [ ] **Step 6: Verify GREEN**

```bash
go test ./internal/contract/issueopspreparation -run Intent -count=1
go test ./internal/core/issueops -run 'IntentCodecSpike|ReconcileLegacy|ResumeIntentSpike|OrcaMarker|OrcaIntent' -count=1
```

- [ ] **Step 7: Commit** — Commit `refactor(issueops): share preparation intent codec` with only codec and recovery integration files.

### Task 2: Add the stable prepare contract and pure decision matrix

**Files:** Create `prepare.go` in the preparation contract and `decision.go` plus test in the preparation domain.

**Interfaces:** Produces `Command`, `Result`, `Snapshot`, `DecisionInput`, `Decision`, and `Decide`.

- [ ] **Step 1: Write the decision RED table** — Cover no-execution auto/direct/orca preview+confirm, ready/unavailable Orca, pending, existing same/auto, explicit mismatch, claimable/released/revoking writerless states, and root conflict. Assert root conflict precedes external probe.

- [ ] **Step 2: Verify RED** — Run `go test ./internal/domain/issueopspreparation -run Decision -count=1`; expect missing package/types.

- [ ] **Step 3: Implement stable contract aliases and clones**

```go
type Command struct { ID, Mode, CWD, OwnerHost, OwnerModel, OwnerEffort string; Actor issueopslease.Actor; Confirm bool }
type Result struct {
    OK, Preview bool; ID, RequestedMode, ResolvedMode, FallbackCode string
    Workspace issueopslease.Workspace; Execution *issueopslease.Execution
    ClaimTokenPath, IssueBodySHA256, ContextPacketPath, ContextPacketSHA256 string
    OwnerPromptPath, OwnerPromptSHA256, IssueSnapshotSource, NextCommand string
}
type Snapshot struct { Record issueopslease.Record; RecordRaw []byte; CanonicalRoot string; RootConflict *RootClaim }
```

- [ ] **Step 4: Implement pure decisions** — Use codes `preview_direct`, `preview_orca`, `apply_direct`, `apply_orca`, `existing`, `pending_reconcile`, `mode_mismatch`, `writerless`, and `root_conflict`. Import only preparation and lease contracts; perform no clock, random, path, SQLite, Git, provider, or Orca call.

- [ ] **Step 5: Verify GREEN**

```bash
go test ./internal/domain/issueopspreparation -count=1
go test -race ./internal/domain/issueopspreparation -count=1
```

- [ ] **Step 6: Commit** — Commit `refactor(issueops): model preparation decisions` with the contract and domain files.

### Task 3: Implement direct preparation through six application ports

**Files:** Create application `ports.go`, `prepare.go`, tests; outbound `repository.go`, `workspace.go`, `evidence.go`, tests.

**Interfaces:** Exactly `Repository`, `Clock`, `OperationID`, `DirectWorkspace`, `OrcaGateway`, `PreparationEvidence`.

- [ ] **Step 1: Write fake-port RED tests** — Assert ordered traces for direct preview/confirm, access denial, provision/artifact/persistence failure, idempotency, pending, mismatch, writerless, and root collision. Preview must not commit or materialize.

- [ ] **Step 2: Define the ports**

```go
type Repository interface {
    Load(context.Context, string) (preparation.Snapshot, error)
    EnsureRootUnclaimed(context.Context, string, string) error
    CommitDirect(context.Context, DirectCommit) (preparation.Result, error)
    BeginIntent(context.Context, OrcaBegin) (IntentState, error)
    MarkInvoking(context.Context, IntentState) (IntentState, error)
    RecordFailure(context.Context, IntentState, string, error) error
    ApplyReceipt(context.Context, IntentState, preparation.IntentReceipt) (IntentProgress, error)
}
type Clock interface { Now() time.Time }
type OperationID interface { New() (string, error) }
type DirectWorkspace interface { ProbeAccess(context.Context, preparation.WorkspaceRequest, string) (preparation.AccessResult, error); Prepare(context.Context, preparation.WorkspaceRequest) (preparation.WorkspaceReceipt, error) }
type OrcaGateway interface { Probe(context.Context, preparation.ProbeRequest) (preparation.ProbeResult, error); Inspect(context.Context, preparation.IntentRequest) (preparation.IntentInventory, error); Invoke(context.Context, preparation.IntentRequest) (preparation.IntentReceipt, error) }
type PreparationEvidence interface { Workspace(preparation.Snapshot, bool) (preparation.WorkspaceRequest, error); ReadOwner(context.Context, preparation.Snapshot, preparation.Command) (preparation.OwnerEvidence, error); MaterializeDirect(context.Context, preparation.Snapshot, preparation.WorkspaceReceipt) error; PrepareOwner(context.Context, preparation.Snapshot, preparation.Command, preparation.Intent, preparation.IntentReceipt) (preparation.OwnerArtifacts, error) }
```

- [ ] **Step 3: Verify RED** — Run `go test ./internal/application/issueopspreparation -run 'Direct|Decision|Idempotent|RootCollision' -count=1`; expect missing service behavior.

- [ ] **Step 4: Implement `NewService(...six ports...)` and `Service.Prepare`** — Preserve order: mutation gate on confirm, load, owner defaults/mode, workspace, root collision, mode probe, pure decision, actor/CWD, access probe, workspace prepare, confirm-only artifacts, one atomic commit, stored result.

- [ ] **Step 5: Implement direct SQLite repository** — In one `WithSpan`, re-read record/root claims, require no execution, encode with `issueopslease`, and atomically apply `issueops_v1` plus `lease_holder_v1` `RequireAbsent`. Preserve all raw sidecars and exact active generation-1 PID-safe actor.

- [ ] **Step 6: Implement Git/evidence adapters** — Convert DTOs explicitly. Confirm requires `ExecutionWorkspaceAccessProber`; nil evidence callbacks fail closed with exact dependency errors.

- [ ] **Step 7: Verify GREEN**

```bash
go test ./internal/application/issueopspreparation -run 'Direct|Decision|Idempotent|RootCollision' -count=1
go test ./internal/adapter/outbound/issueopspreparation -run 'Direct|Repository|Holder|Root|CAS' -count=1
go test -race ./internal/application/issueopspreparation ./internal/adapter/outbound/issueopspreparation -count=1
```

- [ ] **Step 8: Commit** — Commit `refactor(issueops): implement direct preparation service` with application and outbound direct files.

### Task 4: Implement bounded Orca preparation stages

**Files:** Modify application prepare/service tests and repository/tests; create outbound `orca.go` and test.

**Interfaces:** Uses Task 1 codec and Task 3 ports; produces durable-before-effect six-stage preparation.

- [ ] **Step 1: Write Orca RED tests** — Cover auto ready/fallback, explicit unavailable, branch/parent validation, begin-before-effect, zero/one/multiple candidates, non-authoritative zero, `not_invoked_proven` retry, unknown no-repeat, two-attempt and six-stage bounds, worktree→terminal→run→bind→task→dispatch, issue drift, artifact failure, and terminal claimable authority.

- [ ] **Step 2: Verify RED** — Run `go test ./internal/application/issueopspreparation -run 'Orca|AutoFallback|Intent|Bounded' -count=1`; expect missing stages.

- [ ] **Step 3: Implement the bounded loop**

```go
progress, err := s.repository.BeginIntent(ctx, begin)
for step := 0; err == nil && progress.Pending; step++ {
    if step >= 6 { return preparation.Result{ID: command.ID}, fmt.Errorf("Orca prepare exceeded the fixed external intent stage count") }
    progress, err = s.advanceOrca(ctx, progress)
}
```

`advanceOrca` inspects, accepts one exact candidate, requires authoritative zero, refuses unknown repeats except existing run-bind semantics, marks invoking before invoke, records bounded diagnostics, and applies receipt by CAS.

- [ ] **Step 4: Implement intent CAS** — `BeginIntent` atomically writes pending record and compact codec bytes with `RequireAbsent`. Subsequent methods compare both `RecordRaw` and `IntentRaw`; dispatch atomically writes claimable generation 1 + Orca binding, clears pending/failure, and deletes the intent.

- [ ] **Step 5: Implement Orca adapter** — Map contract↔port DTOs explicitly; preserve `port.OrcaError.Invoked == false` as `not_invoked_proven`, all other invocation errors as `unknown`.

- [ ] **Step 6: Verify GREEN**

```bash
go test ./internal/application/issueopspreparation -run 'Orca|AutoFallback|Intent|Bounded' -count=1
go test ./internal/adapter/outbound/issueopspreparation -run 'Orca|Intent|CAS' -count=1
go test ./internal/core/issueops -run 'AutoFallback|OrcaIntent|OrcaBranch|ParentWorktree|Reconcile|ResumeIntentSpike' -count=1
go test -race ./internal/application/issueopspreparation ./internal/adapter/outbound/issueopspreparation -count=1
```

- [ ] **Step 7: Commit** — Commit `refactor(issueops): implement Orca preparation stages` with Orca application/repository/adapter changes.

### Task 5: Add inbound mapping and deterministic differential oracle

**Files:** Create inbound handler/test, legacy oracle test, differential test; modify production `execution_prepare.go` only to expose granular compatibility helpers.

**Interfaces:** Produces `NewHandler(service) issueops.ExecutionPrepareHandler`; predecessor is test-only.

- [ ] **Step 1: Write inbound RED tests** — Map every request field including PID-safe actor and every result field including digests/source/next command. Nil service returns `ErrPrepareHandlerUnavailable`.

- [ ] **Step 2: Verify RED** — Run `go test ./internal/adapter/inbound/issueopspreparation -count=1`; expect missing handler.

- [ ] **Step 3: Implement inbound handler**

```go
type service interface { Prepare(context.Context, preparation.Command) (preparation.Result, error) }
func NewHandler(service service) issueops.ExecutionPrepareHandler
```

Use explicit field mapping and preserve structured codec/schema errors as existing inbound verticals do.

- [ ] **Step 4: Freeze predecessor in `_test.go`** — Move the old orchestrator bodies under `prepareExecutionCompatibilityOracle`, `prepareDirectExecutionCompatibilityOracle`, and `prepareOrcaExecutionCompatibilityOracle`. Existing characterization tests use a test-only `PrepareExecution` shim.

- [ ] **Step 5: Add differential cases** — In isolated state roots with identical clock/operation ID/fakes, compare exact result JSON, error, record bytes, holder-index bytes, intent bytes, and ordered trace for preview, direct success/failures, Orca modes/stages/recovery, idempotent/pending/mismatch/writerless, root collision, and parent worktree.

- [ ] **Step 6: Verify GREEN**

```bash
go test ./internal/adapter/inbound/issueopspreparation -count=1
go test ./internal/core/issueops -run 'PreparationDifferential|ExecutionPrepare|AutoFallback|RootCollision|ParentWorktree|OrcaIntent' -count=1
```

- [ ] **Step 7: Commit** — Commit `test(issueops): prove preparation vertical parity` with inbound, oracle, and differential files.

### Task 6: Compose in issueopsapp and cut over handler-only routing

**Files:** Create issueopsapp preparation wiring/test; modify core route, CLI/MCP dependency structs/builders, and issueopsapp facades.

**Interfaces:** Produces one `issueOpsPrepareHandler`; CLI and daemon-backed MCP receive it plus composition-owned dependencies.

- [ ] **Step 1: Write routing RED tests** — Prove one prepare handler call, nil fail-closed without effects, identical CLI/MCP request/result, real direct preview, and same CLI/daemon constructor.

- [ ] **Step 2: Verify RED**

```bash
go test ./internal/core/issueops -run 'ExecutionAPI.*Prepare|PrepareHandler' -count=1
go test ./cmd/issueops/issueopscli ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp -run 'ExecutionPrepare|PrepareWiring|HandlerMissing' -count=1
```

- [ ] **Step 3: Build composition** — Open `sqlstore`; construct preparation repository/codec, Git workspace, Orca gateway, evidence callbacks, clock/ID, application service, and inbound handler. Granular callbacks may call exported core primitives but never a prepare orchestrator or wrapper.

- [ ] **Step 4: Move concrete dependencies** — Remove Git/Orca/provider construction/imports from CLI/MCP. Inject `Prepare`, `Orca`, `OrcaOwner`, and `ReadIssue` from `runIssueOps` and `issueOpsMCPDependencies` because non-prepare actions still consume some shared dependencies.

- [ ] **Step 5: Cut over core route**

```go
case ExecutionActionPrepare:
    return invokeExecutionPrepareHandler(ctx, stateRoot, ExecutionPrepareRequest{
        ID: req.ID, Mode: req.Mode, Actor: req.Actor, CWD: req.CWD,
        OwnerHost: req.OwnerHost, OwnerModel: req.OwnerModel, OwnerEffort: req.OwnerEffort, Confirm: req.Confirm,
    }, deps.Prepare)
```

No fallback branch exists.

- [ ] **Step 6: Remove production predecessor** — Delete production `PrepareExecution`, `prepareDirectExecution`, and `prepareOrcaExecution`; keep only the `_test.go` oracle. `rg` must find those names only in tests, docs, and architecture rejection fixtures.

- [ ] **Step 7: Verify GREEN**

```bash
go test ./internal/core/issueops -run 'ExecutionAPI.*Prepare|PrepareHandler|PreparationDifferential' -count=1
go test ./cmd/issueops/issueopscli ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp -run 'ExecutionPrepare|PrepareWiring|HandlerMissing|ResponseContract' -count=1
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
```

- [ ] **Step 8: Commit** — Commit `refactor(issueops): route prepare through application handler` with composition, injection, route, and predecessor removal.

### Task 7: Lock architecture and hook-enabled authority parity

**Files:** Modify architecture test/baseline, hook execution contract test, and issueopsapp wiring test.

**Interfaces:** Produces automated no-fallback/no-leak layer gates and actual PreToolUse-shaped Codex/Claude parity tests.

- [ ] **Step 1: Write architecture RED cases** — Preparation contract may import only lease contract; domain only preparation/lease contracts; application only preparation domain/contracts; outbound never core. Reject non-test predecessor identifiers, prepare routes not calling injected `Prepare` once, and CLI/MCP preparation builders constructing Git/Orca/provider.

- [ ] **Step 2: Verify RED** — Run `go test ./internal/architecture -run 'Preparation|ProductionGraph' -count=1`; expect new rule failures.

- [ ] **Step 3: Implement gates and baseline cleanup** — Add preparation path predicates and `productionPreparationRoutingViolations`; remove only concrete dependency edges made stale from `legacy_imports.txt`.

- [ ] **Step 4: Add enabled-hook policy tests** — Prepare direct and Orca records through the new handler, then feed existing hook evaluators Codex and Claude payloads. Assert exact-holder/canonical allow; structured target allow; foreign session/PID start/executable deny; subdirectory/symlink parity; claimable/released/revoking deny codes, generation, root, and next command. Do not mock a hook bypass.

- [ ] **Step 5: Verify GREEN**

```bash
go test ./internal/architecture -run 'Preparation|ProductionGraph' -count=1
go test ./cmd/issueops/hookcli -run 'ExecutionPrepare|AtomicPreflight|Canonical|Workdir|OwnerMutation' -count=1
go test ./internal/core/lifecycle -run 'AtomicPreflight|Canonical|Workdir|OwnerMutation' -count=1
go test -race ./cmd/issueops/hookcli ./internal/core/lifecycle -run 'ExecutionPrepare|AtomicPreflight|Canonical|Workdir|OwnerMutation' -count=1
```

- [ ] **Step 6: Commit** — Commit `test(issueops): enforce preparation hook parity` with architecture and hook gates.

### Task 8: Complete verification and publication readiness

**Files:** Modify required non-OpenWiki project docs only if code-proven drift exists. Never modify `openwiki/**`.

**Interfaces:** Produces a clean final HEAD, Turing report, IssueOps readiness, and reproducible evidence.

- [ ] **Step 1: Format/static check**

```bash
gofmt -w internal/contract/issueopspreparation internal/domain/issueopspreparation internal/application/issueopspreparation internal/adapter/inbound/issueopspreparation internal/adapter/outbound/issueopspreparation cmd/issueops/issueopsapp/issueops_preparation_wiring.go cmd/issueops/issueopsapp/issueops_preparation_wiring_test.go
git diff --check
go vet ./internal/contract/issueopspreparation/... ./internal/domain/issueopspreparation/... ./internal/application/issueopspreparation/... ./internal/adapter/inbound/issueopspreparation/... ./internal/adapter/outbound/issueopspreparation/...
```

- [ ] **Step 2: Focused verification**

```bash
go test ./internal/contract/issueopspreparation ./internal/domain/issueopspreparation ./internal/application/issueopspreparation ./internal/adapter/inbound/issueopspreparation ./internal/adapter/outbound/issueopspreparation -count=1
go test -race ./internal/contract/issueopspreparation ./internal/domain/issueopspreparation ./internal/application/issueopspreparation ./internal/adapter/inbound/issueopspreparation ./internal/adapter/outbound/issueopspreparation -count=1
go test ./internal/core/issueops -run 'ExecutionPrepare|PreparationDifferential|AutoFallback|RootCollision|ParentWorktree|OrcaIntent|Reconcile|ResumeIntent' -count=1
go test ./cmd/issueops/issueopscli ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp -run 'ExecutionPrepare|PrepareWiring|ExecutionHandler|ResponseContract' -count=1
go test ./internal/core/lifecycle ./cmd/issueops/hookcli -run 'ExecutionPrepare|AtomicPreflight|Canonical|Workdir|OwnerMutation' -count=1
go test ./internal/architecture -run 'Dependency|Preparation' -count=1
```

- [ ] **Step 3: Repository-wide verification**

```bash
go test ./... -count=1
go test -race ./... -count=1
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
go build -o bin/issueops ./cmd/issueops
```

- [ ] **Step 4: Official hook-command smoke** — With a temporary state root and built binary, start/link/prepare direct; invoke the product hook command using Codex- and Claude-shaped JSON. Expect exact-holder allow and foreign-holder deny. The current process flag `--disable hooks` is explicitly not evidence.

- [ ] **Step 5: Turing/self-verification**

```bash
./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json
./bin/issueops self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
```

Create the lifecycle-required Turing report mapping AC-199-01 through AC-199-10 to exact tests/commands/results; require score `>=95` and no blocker.

- [ ] **Step 6: Record only verified docs drift** — If `.issueops/CAUTIONS.md`, `ARCHITECTURE.md`, or `OPERATIONS.md` disagrees with code, make a surgical commit `docs(issueops): record preparation operation`; otherwise create no docs commit.

- [ ] **Step 7: Final readiness**

```bash
git status --short
git log --oneline 739de96aeca540cf7d5cf6333b345a192afcfd59..HEAD
./bin/issueops pr-readiness --id io-ab4d4c69d7e5 --strict --json
```

Expected: clean tree and no missing implementation gate after compatibility review, AI-slop clean, implementation review, Turing evidence, and final verification are recorded.

---

## Self-Review Results

- Spec coverage: Tasks 1–7 cover AC-199-01 through AC-199-10; Task 7 specifically covers enabled-hook parity; Task 8 covers focused/race/architecture/golden/build/final gates and excludes OpenWiki.
- Placeholder scan: no deferred implementation markers remain; every task names files, interfaces, RED/GREEN commands, and commit scope.
- Type consistency: `issueopspreparation.IntentCodec` and `issueopslease` are the single persisted types; `Service.Prepare`, `ExecutionPrepareHandler`, and the six port names stay consistent.
- Execution handoff: the user authorized automatic continuation and the project requires main-agent execution, so continue inline with `superpowers:executing-plans` after lifecycle plan and compatibility approval.
