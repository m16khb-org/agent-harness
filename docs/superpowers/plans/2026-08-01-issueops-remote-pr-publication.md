# IssueOps Remote PR/MR Publication Vertical Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` (recommended for this repository) to implement this plan task-by-task. Use `superpowers:subagent-driven-development` only if the coordinator records a repository-approved net-positive sub-agent pattern, bounded scope, verification, and fallback. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move `remote_pr_create` creation and durable recovery into one capability-local hexagonal vertical without changing public CLI/MCP behavior, schema v1 bytes, provider semantics, or retry authority.

**Architecture:** Add `issueopspublication` contract, domain, application, inbound, and outbound packages. `CreateService` and `ReconcileService` share application-owned repository, provider, and verifier ports; `cmd/issueops/issueopsapp` is the only production composition root and bridges the new vertical to purpose-bound legacy raw CAS primitives. Core retains public DTO/facade routing and #194 Orca routing, but migrated `remote_pr_create` handlers fail closed instead of falling back to the legacy orchestration.

**Tech Stack:** Go 1.26.3, standard library, existing SQLite-backed IssueOps store, existing GitHub/GitLab provider adapters, table-driven Go tests, contract goldens, architecture dependency tests.

## Global Constraints

- Work only in `/Users/m16khb/Workspace/issueops.worktrees/195-issueops-remote-pr-publication-vertical` on branch `195-issueops-remote-pr-publication-vertical`.
- The sealed base is `667e5d15b0773e2550cfbf5bc2780506e9eb2896`; do not rebase, merge, reset, or change the base without the IssueOps sync-base protocol.
- Preserve `RemotePullRequestRequest`, provider create result, `ExecutionReconcileRequest/Result`, CLI text, MCP `isError`, and all existing error/code meanings.
- Preserve schema v1 `IssueOpsRecord`, bucket `external_intent_v1`, `externalRemotePRPayload` field order/tags/omitempty/bytes, and `RemoteArtifact` bytes.
- Persist intent before provider invocation. Never hold the SQLite cycle lock during provider calls, inventory calls, or live artifact verification.
- Permit exactly one retry only after authoritative zero, `not_invoked_proven`, `retry_count=0`, and a successful retry-marker CAS.
- Preserve pending on ambiguous outcomes, multiple candidates, non-authoritative zero, verification failure, and receipt CAS failure. Seal an observed canonical URL as `known_url` when authority still permits the failure receipt CAS.
- Preserve #194 routing for Orca `worktree_create`, `owner_launch`, `dispatch`, preview, no-pending, and unsupported kinds.
- Do not add a new MCP initial-create action. The existing surfaces are CLI `remote create-pr` plus CLI/MCP `execution reconcile`, whose bounded retry may call create.
- Do not change GitHub/GitLab concrete adapter semantics, provider authentication, branch preparation, push authority, issue completion, or cleanup.
- Run named RED before production code for every behavior task. Keep commits atomic with Conventional Commit subject plus Lore body.
- Run focused tests/race locally. Run full `go test ./...` and full race only in GitHub Actions per parent #117 policy.
- Do not auto-update OpenWiki.

## Source of Truth

- Issue: `https://github.com/m16khb-org/issueops/issues/195`
- Design: `docs/superpowers/specs/2026-08-01-issueops-remote-pr-publication-design.md`
- Legacy create oracle: `internal/core/issueops/execution_remote.go`
- Legacy reconcile oracle: `internal/core/issueops/execution_reconcile.go`
- #194 vertical pattern: `internal/{contract,domain,application,adapter}/.../issueopslease` and `cmd/issueops/issueopsapp/issueops_reconcile_wiring.go`

## File Structure

| Path | Responsibility |
|---|---|
| `internal/contract/issueopspublication/publication.go` | Stable create request/result, provider request/result, candidate/inventory, intent, record snapshot, invocation state projections |
| `internal/domain/issueopspublication/decision.go` | Pure create eligibility and reconcile `adopt|retry|preserve` decisions |
| `internal/application/issueopspublication/ports.go` | Consumer-owned `Repository`, `Provider`, and `Verifier` ports |
| `internal/application/issueopspublication/create.go` | Intent-first create orchestration |
| `internal/application/issueopspublication/reconcile.go` | Exact candidate adoption and bounded retry orchestration |
| `internal/adapter/inbound/issueopspublication/create.go` | Existing core create DTO/result mapping |
| `internal/adapter/inbound/issueopspublication/reconcile.go` | Existing core reconcile DTO/result mapping |
| `internal/adapter/outbound/issueopspublication/repository.go` | Application repository adapter over narrow effects |
| `internal/adapter/outbound/issueopspublication/provider.go` | Existing provider create/inventory function adapter |
| `internal/adapter/outbound/issueopspublication/verifier.go` | Exact candidate and live artifact verification adapter |
| `internal/core/issueops/execution_remote_bridge.go` | Purpose-bound raw read/CAS bridge; no provider orchestration |
| `internal/core/issueops/execution_remote.go` | Public create router plus current legacy orchestration until caller-zero removal |
| `internal/core/issueops/execution_remote_legacy_oracle_test.go` | Frozen, test-only snapshot of the base create/reconcile orchestration for final differential tests |
| `internal/core/issueops/execution_reconcile.go` | Public kind router; preserve #194 paths and forward `remote_pr_create` |
| `internal/core/issueops/execution_api.go` | Handler types, fail-closed errors, dependency slots |
| `cmd/issueops/issueopsapp/issueops_publication_wiring.go` | Only production composition root for both services |
| `cmd/issueops/issueopscli/remotecmd/remote.go` | CLI initial-create handler injection; preserve output formatting |
| `cmd/issueops/issueopscli/issueops_execution_cli.go` | CLI reconcile dependency injection without concrete provider closure |
| `cmd/issueops/mcpcli/mcp_tool_issueops_execution.go` | MCP reconcile dependency injection without concrete provider closure |
| `cmd/issueops/issueopsapp/{issueops_policy_facade.go,mcp_facade.go}` | Pass publication handlers into CLI/MCP adapters |
| `internal/architecture/dependency_test.go` | Forbidden dependency, composition, zero-fallback, caller-zero ratchets |
| `.issueops/verified-execution/issue195-report.md` | Final focused verification and CI evidence |

---

### Task 0: Prove a deterministic exact-base byte oracle before designing against it

**Files:**
- Modify: `internal/core/issueops/execution_remote.go`
- Modify: `internal/core/issueops/execution_remote_legacy_oracle_test.go`
- Create: `internal/core/issueops/testdata/remote_publication_v1/intent.golden.json`
- Create: `internal/core/issueops/testdata/remote_publication_v1/failure_record.golden.json`

**Interfaces:**
- Consumes: Exact-base `beginRemotePullRequestIntent`, `recordRemotePullRequestFailure`, fixed `Now`, and the current schema v1 serializer only.
- Produces: A behavior-preserving operation-ID seam, a frozen test-only legacy oracle identified by base SHA `667e5d15b0773e2550cfbf5bc2780506e9eb2896`, and immutable raw-byte goldens that later vertical tests must match.

- [ ] **Step 1: Write the deterministic legacy oracle RED**

In the `_test.go` file, freeze the base create sequence as `legacyCreateRemotePullRequestWithOperationID`. It is a verbatim test-only snapshot of the pre-vertical orchestration except that it receives `operationID` and fixed `Now`; it must call the same raw begin/failure/finish primitives and never call vertical code. Use operation ID `0123456789abcdef0123456789abcdef`, `2026-08-01T00:00:00Z`, and a manually persisted record whose repo/workspace/actor/process fields are fixed literals so temp directory, PID, and host process identity cannot enter the bytes.

Add `TestLegacyRemotePublicationV1Goldens` that creates one intent and one ambiguous failure receipt, reads the raw `external_intent_v1` row and raw cycle record, and compares them with `bytes.Equal` to sentinel golden files containing `{}`. Do not add an update flag, regenerate mode, or any path that lets vertical output overwrite the goldens.

- [ ] **Step 2: Run the oracle test and observe RED**

Run: `go test ./internal/core/issueops -run '^TestLegacyRemotePublicationV1Goldens$' -count=1`

Expected: FAIL because `beginRemotePullRequestIntentWithOperationID` is undefined or the immutable goldens are not populated.

- [ ] **Step 3: Add the minimal behavior-preserving identity seam**

Extract only the operation-ID choice:

```go
func beginRemotePullRequestIntent(stateRoot string, expected IssueOpsRecord, actor model.NativeActor, cwd string, expectedGeneration uint64, providerReq port.IssueProviderCreatePullRequestRequest, provider, kind string, now func() time.Time) (IssueOpsRecord, externalRemotePRPayload, error) {
    operationID, err := newExecutionOperationID()
    if err != nil { return IssueOpsRecord{}, externalRemotePRPayload{}, err }
    return beginRemotePullRequestIntentWithOperationID(stateRoot, expected, actor, cwd, expectedGeneration, providerReq, provider, kind, operationID, now)
}

func beginRemotePullRequestIntentWithOperationID(stateRoot string, expected IssueOpsRecord, actor model.NativeActor, cwd string, expectedGeneration uint64, providerReq port.IssueProviderCreatePullRequestRequest, provider, kind, operationID string, now func() time.Time) (IssueOpsRecord, externalRemotePRPayload, error) {
    // Move exact-base execution_remote.go:221-258 here byte-for-byte.
}
```

Validate that the supplied ID is exactly 32 lowercase hexadecimal characters before persistence. Production still calls `beginRemotePullRequestIntent` and therefore keeps cryptographic randomness and all public behavior. The test-only oracle calls the fixed-ID helper. No package global variable or mutable hook is permitted.

- [ ] **Step 4: Capture exact-base bytes once, then prove oracle stability**

Run the named test once after Step 3. Expected: FAIL against the sentinel files and print the observed fixed-ID bytes in hex. Decode/review those bytes, write them once to the two golden files with `apply_patch`, and rerun the same test. The goldens are now immutable compatibility input; later tasks may read but never regenerate them.

Run: `go test ./internal/core/issueops -run 'LegacyRemotePublicationV1Goldens|RemotePullRequest' -count=1`

Run: `go test -race ./internal/core/issueops -run 'LegacyRemotePublicationV1Goldens|RemotePullRequest' -count=1`

Expected: PASS; the literal raw bytes match, existing remote publication tests remain green, and the test oracle imports no vertical package.

- [ ] **Step 5: Commit Task 0**

```bash
git add internal/core/issueops/execution_remote.go internal/core/issueops/execution_remote_legacy_oracle_test.go internal/core/issueops/testdata/remote_publication_v1
git commit -m "refactor(issueops): freeze publication byte oracle" -m "Lore:
- Intent: Prove schema v1 byte compatibility before building the publication vertical.
- Why: The exact-base create path generated a non-injectable random operation ID, making direct byte comparison impossible.
- Changes:
  - Split random ID selection from the existing intent-begin body.
  - Add fixed-ID legacy intent and failure-record goldens.
- Verify: focused core publication tests and race.
- Risk: Low; production still uses the same random ID generator and orchestration."
```

---

### Task 1: Freeze the publication contract and pure decision matrix

**Files:**
- Create: `internal/contract/issueopspublication/publication.go`
- Create: `internal/contract/issueopspublication/publication_test.go`
- Create: `internal/contract/issueopspublication/publication_port_parity_test.go`
- Create: `internal/domain/issueopspublication/decision.go`
- Create: `internal/domain/issueopspublication/decision_test.go`

**Interfaces:**
- Consumes: No repository or provider code. Copy the exact provider projection fields from `internal/port/provider.go:31-98` and invocation strings from `internal/core/issueops/execution_remote.go:25-29`.
- Produces: `contract.CreateCommand`, `contract.PreparedCreate`, `contract.CreateEligibility`, `contract.ProviderCreateRequest`, `contract.ProviderCreateResult`, `contract.Candidate`, `contract.Inventory`, `contract.Intent`, `contract.RecordSnapshot`, `domain.ValidateCreateEligibility`, `domain.ReconcileFacts`, `domain.DecideReconcile`, and `domain.DecideRetryOutcome`.

- [ ] **Step 1: Write provider-DTO parity and copy-isolation tests**

Give `ProviderCreateRequest` and `ProviderCreateResult` the exact JSON tags/omitempty behavior from `internal/port/provider.go`. In the external test package `issueopspublication_test`, populate both the contract and port DTOs field-for-field and compare their marshalled bytes; this test-only import is permitted, while `publication.go` itself may not import port. The other application-only projections are never public/persisted JSON, and production code must not marshal `Intent` or `RecordSnapshot`. Prove every slice, pointer, and raw byte field is cloned rather than aliased:

```go
func TestIntentClonePreservesAllAuthorityWithoutAliasing(t *testing.T) {
    original := Intent{
        Record: RecordSnapshot{ID: "io-1", Raw: []byte(`{"schema_version":1}`)},
        OperationID: "op-1", Generation: 7, Provider: "github", Kind: "pr",
        Request: ProviderCreateRequest{
            Repo: "/repo", ProjectKey: "github.com/acme/repo", Title: "title", Body: "body",
            HeadBranch: "195-branch", BaseBranch: "117-parent", Labels: []string{"enhancement"},
            Assignees: []string{"maintainer"}, Draft: true, ExpectedHeadSHA: strings.Repeat("a", 40), Confirm: true,
            Host: "codex", SessionID: "session", CWD: "/repo.worktrees/195-branch",
        },
        InvocationState: InvocationUnknown, RetryCount: 0, Raw: []byte(`{"schema_version":1}`),
    }
    cloned := original.Clone()
    original.Record.Raw[0], original.Raw[0], original.Request.Labels[0] = 'x', 'x', "changed"
    if string(cloned.Record.Raw) != `{"schema_version":1}` || string(cloned.Raw) != `{"schema_version":1}` || cloned.Request.Labels[0] != "enhancement" {
        t.Fatalf("clone aliased mutable input: %#v", cloned)
    }
}
```

- [ ] **Step 2: Run the contract test and observe RED**

Run: `go test ./internal/contract/issueopspublication -run TestIntentClonePreservesAllAuthorityWithoutAliasing -count=1`

Expected: FAIL because package/types do not exist.

- [ ] **Step 3: Define the stable projections**

Use these complete field sets; do not import `internal/core`, `internal/port`, adapters, SQLite, CLI, or MCP:

```go
type InvocationState string

const (
    InvocationUnknown          InvocationState = "unknown"
    InvocationNotInvokedProven InvocationState = "not_invoked_proven"
)

type ProcessReceipt struct { PID int; StartedAt, Executable string }
type Actor struct {
    Host, SessionID, AgentID string
    SessionProcess *ProcessReceipt
    ProcessAncestry []ProcessReceipt
}
type CreateCommand struct {
    ID, Provider, Title, Body, Head, Base string
    Labels, Assignees []string
    ExpectedGeneration uint64
    Actor Actor
    CWD string
    Confirm bool
}
type ProviderCreateRequest struct {
    Repo, ProjectKey, Title, Body, HeadBranch, BaseBranch string
    Labels, Assignees []string
    Draft bool
    ExpectedHeadSHA string
    Confirm bool
    Host, SessionID, AgentID, CWD string
}
type ProviderCreateResult struct { OK bool; URL, Number, Preview string }
type CreateEligibility struct {
    Provider, Kind string
    Confirm bool
    PhasePR, ExecutionActive, NoPending, NoArtifact bool
    BranchAuthority, CanonicalLabelsAssignees bool
}
type PreparedCreate struct {
    Request ProviderCreateRequest
    Eligibility CreateEligibility
}
type Candidate struct {
    URL, ProjectKey, SourceProjectKey, HeadBranch, BaseBranch, HeadSHA string
    Title, BodySHA256 string
    Labels, Assignees []string
    Draft bool
    State string
}
type Inventory struct { Candidates []Candidate; AuthoritativeZero bool }
type RecordSnapshot struct { ID string; Raw []byte }
type Intent struct {
    Record RecordSnapshot
    OperationID string
    Generation uint64
    Provider, Kind string
    Request ProviderCreateRequest
    InvocationState InvocationState
    RetryCount int
    KnownURL string
    Eligibility CreateEligibility // projection only; never marshalled into schema v1 payload
    Raw []byte
}
type ReconcileResult struct {
    Record RecordSnapshot
    Reconciled bool
    Code string
    ExternalStateInspected bool
}
```

Implement `Clone` methods for every projection containing slices, pointers, or raw bytes.

- [ ] **Step 4: Write the pure decision table tests**

```go
func TestDecideReconcile(t *testing.T) {
    cases := []struct {
        name string
        facts ReconcileFacts
        action Action
        reason string
    }{
        {"one candidate", ReconcileFacts{CandidateCount: 1}, ActionAdopt, ""},
        {"multiple", ReconcileFacts{CandidateCount: 2}, ActionPreserve, "multiple-candidates"},
        {"ambiguous zero", ReconcileFacts{}, ActionPreserve, "non-authoritative-zero"},
        {"unknown invocation", ReconcileFacts{AuthoritativeZero: true, Invocation: contract.InvocationUnknown}, ActionPreserve, "unknown-invocation"},
        {"first proven retry", ReconcileFacts{AuthoritativeZero: true, Invocation: contract.InvocationNotInvokedProven}, ActionRetry, ""},
        {"retry exhausted", ReconcileFacts{AuthoritativeZero: true, Invocation: contract.InvocationNotInvokedProven, RetryCount: 1}, ActionPreserve, "retry-exhausted"},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got, err := DecideReconcile(tc.facts)
            if err != nil || got.Action != tc.action || got.Reason != tc.reason { t.Fatalf("got=%#v err=%v", got, err) }
        })
    }
}
```

Add `TestValidateCreateEligibility` for GitHub/GitLab only, matching PR/MR kind, phase PR, no artifact, branch authority, and non-empty canonical labels/assignees. Confirm additionally requires active execution and no pending; preview explicitly does not, matching the current `prepareRemotePullRequest` order. It returns a typed reason enum, not compatibility-layer error text; the existing core bridge remains responsible for the exact public validation errors. Also add `TestDecideRetryOutcome` proving `not_invoked_proven` maps to terminal-not-invoked, while unknown invocation preserves the consumed retry and a successful call advances to verification.

- [ ] **Step 5: Run domain tests and observe RED**

Run: `go test ./internal/domain/issueopspublication -run 'TestDecideReconcile|TestValidateCreateEligibility|TestDecideRetryOutcome' -count=1`

Expected: FAIL with undefined `ReconcileFacts`, `DecideReconcile`, `ValidateCreateEligibility`, and `DecideRetryOutcome`.

- [ ] **Step 6: Implement the minimal pure decisions**

```go
type Action string
const ( ActionAdopt Action = "adopt"; ActionRetry Action = "retry"; ActionPreserve Action = "preserve" )
type ReconcileFacts struct { CandidateCount int; AuthoritativeZero bool; Invocation contract.InvocationState; RetryCount int }
type Decision struct { Action Action; CandidateIndex int; Reason string }

func DecideReconcile(f ReconcileFacts) (Decision, error) {
    if f.CandidateCount < 0 { return Decision{}, fmt.Errorf("candidate count must not be negative") }
    if f.CandidateCount > 1 { return Decision{Action: ActionPreserve, Reason: "multiple-candidates"}, nil }
    if f.CandidateCount == 1 { return Decision{Action: ActionAdopt, CandidateIndex: 0}, nil }
    if !f.AuthoritativeZero { return Decision{Action: ActionPreserve, Reason: "non-authoritative-zero"}, nil }
    if f.Invocation != contract.InvocationNotInvokedProven { return Decision{Action: ActionPreserve, Reason: "unknown-invocation"}, nil }
    if f.RetryCount != 0 { return Decision{Action: ActionPreserve, Reason: "retry-exhausted"}, nil }
    return Decision{Action: ActionRetry}, nil
}

type EligibilityReason string

func ValidateCreateEligibility(f contract.CreateEligibility) (EligibilityReason, error) {
    if f.Provider != "github" && f.Provider != "gitlab" { return "provider", fmt.Errorf("publication eligibility: provider") }
    if f.Provider == "github" && f.Kind != "pr" || f.Provider == "gitlab" && f.Kind != "mr" { return "kind", fmt.Errorf("publication eligibility: kind") }
    checks := []struct { ok bool; reason EligibilityReason }{
        {f.PhasePR, "phase"}, {f.NoArtifact, "artifact"}, {f.BranchAuthority, "branch-authority"},
        {f.CanonicalLabelsAssignees, "labels-assignees"},
    }
    for _, check := range checks {
        if !check.ok { return check.reason, fmt.Errorf("publication eligibility: %s", check.reason) }
    }
    if f.Confirm && !f.ExecutionActive { return "execution", fmt.Errorf("publication eligibility: execution") }
    if f.Confirm && !f.NoPending { return "pending", fmt.Errorf("publication eligibility: pending") }
    return "", nil
}

type RetryOutcomeFacts struct {
    Invocation contract.InvocationState
    CallFailed bool
}

type RetryOutcome string
const (
    RetryOutcomeVerify RetryOutcome = "verify"
    RetryOutcomePreserve RetryOutcome = "preserve"
    RetryOutcomeTerminalNotInvoked RetryOutcome = "terminal-not-invoked"
)

func DecideRetryOutcome(f RetryOutcomeFacts) RetryOutcome {
    if !f.CallFailed { return RetryOutcomeVerify }
    if f.Invocation == contract.InvocationNotInvokedProven { return RetryOutcomeTerminalNotInvoked }
    return RetryOutcomePreserve
}
```

- [ ] **Step 7: Run focused contract/domain tests and race**

Run: `go test ./internal/contract/issueopspublication ./internal/domain/issueopspublication -count=1`

Run: `go test -race ./internal/contract/issueopspublication ./internal/domain/issueopspublication -count=1`

Expected: PASS, zero data races.

- [ ] **Step 8: Commit Task 1**

```bash
git add internal/contract/issueopspublication internal/domain/issueopspublication
git commit -m "refactor(issueops): add publication decision contract" -m "Lore:
- Intent: Define provider-neutral publication facts and pure recovery decisions.
- Why: Creation and recovery need one consumer-owned contract before orchestration moves.
- Changes:
  - Add stable publication projections with copy isolation.
  - Add create eligibility and bounded reconcile decisions.
- Verify: go test and go test -race for contract/domain issueopspublication packages.
- Risk: Low; no production routing changes."
```

---

### Task 2: Implement intent-first CreateService

**Files:**
- Create: `internal/application/issueopspublication/ports.go`
- Create: `internal/application/issueopspublication/create.go`
- Create: `internal/application/issueopspublication/test_helpers_test.go`
- Create: `internal/application/issueopspublication/create_test.go`

**Interfaces:**
- Consumes: Task 1 contract projections and `domain.ValidateCreateEligibility`.
- Produces: `Repository.PreviewCreate`, `Repository.BeginCreate`, `Repository.RecordFailure`, `Repository.Complete`, `Provider.Create`, `Verifier.VerifyLive`, `CreateService.Create`.

- [ ] **Step 1: Define fake ports and write the RED event-order test**

In `test_helpers_test.go`, define one complete package-local fake for each port. Every function field has the exact corresponding port signature; every unconfigured method calls `t.Fatalf` rather than returning a permissive zero value. Define these shared immutable fixture constructors in the same file so later snippets never depend on undeclared helpers:

```go
func validCreateCommand(confirm bool) contract.CreateCommand
func validIntent() contract.Intent
func validRecord() contract.RecordSnapshot
func provenZeroIntent() contract.Intent
func retryIntent() contract.Intent
func successfulResult() contract.ProviderCreateResult
func authoritativeZero(context.Context, contract.Intent) (contract.Inventory, bool, error)
func acceptingVerifier(t *testing.T) *fakeVerifier
func newFakeRepository(t *testing.T) *fakeRepository
func newFakeProvider(t *testing.T) *fakeProvider
```

`validIntent` and `retryIntent` must use fixed operation IDs, generation, provider, request, and raw JSON byte slices. Each test receives a clone so mutation in one row cannot contaminate another. `fakeRepository` implements every `Repository` method, `fakeProvider` implements both provider methods, and `fakeVerifier` implements both verifier methods. Add compile-time assertions (`var _ Repository = (*fakeRepository)(nil)`) for all three.

```go
func TestCreatePersistsIntentBeforeProviderAndCompletesAfterVerify(t *testing.T) {
    events := []string{}
    repo := newFakeRepository(t)
    repo.begin = func(context.Context, contract.CreateCommand) (contract.Intent, error) { events = append(events, "intent"); return validIntent(), nil }
    repo.complete = func(context.Context, contract.Intent, string, bool) (contract.RecordSnapshot, error) { events = append(events, "receipt"); return validRecord(), nil }
    provider := newFakeProvider(t)
    provider.create = func(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) {
        events = append(events, "provider"); return contract.ProviderCreateResult{OK: true, URL: "https://github.com/acme/repo/pull/1"}, contract.InvocationUnknown, nil
    }
    verifier := acceptingVerifier(t)
    verifier.live = func(context.Context, contract.Intent, string) error { events = append(events, "verify"); return nil }
    result, err := NewCreateService(repo, provider, verifier).Create(context.Background(), validCreateCommand(true))
    if err != nil || result.URL == "" || strings.Join(events, ",") != "intent,provider,verify,receipt" { t.Fatalf("events=%v result=%#v err=%v", events, result, err) }
}
```

Add named tests for preview no-write, typed pre-invocation failure, ambiguous error, empty URL, verification failure, success-receipt failure, and known URL propagation.

- [ ] **Step 2: Run CreateService tests and observe RED**

Run: `go test ./internal/application/issueopspublication -run '^TestCreate' -count=1`

Expected: FAIL because ports and service are undefined.

- [ ] **Step 3: Define consumer-owned ports**

```go
type Repository interface {
    PreviewCreate(context.Context, contract.CreateCommand) (contract.PreparedCreate, error)
    BeginCreate(context.Context, contract.CreateCommand) (contract.Intent, error)
    LoadIntent(context.Context, string) (contract.Intent, error)
    MarkRetry(context.Context, contract.Intent) (contract.Intent, error)
    RecordFailure(context.Context, contract.Intent, contract.InvocationState, string, error) error
    Complete(context.Context, contract.Intent, string, bool) (contract.RecordSnapshot, error)
    CompleteNotInvoked(context.Context, contract.Intent, error) (contract.RecordSnapshot, error)
    Latest(context.Context, string) (contract.RecordSnapshot, error)
}
type Provider interface {
    Create(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error)
    Inspect(context.Context, contract.Intent) (contract.Inventory, bool, error)
}
type Verifier interface {
    VerifyCandidate(context.Context, contract.Intent, contract.Candidate) error
    VerifyLive(context.Context, contract.Intent, string) error
}
```

- [ ] **Step 4: Implement minimal CreateService orchestration**

`Create` must use this exact branch order: dependency check; preview preparation and pure eligibility check; or `BeginCreate` and pure eligibility check; provider call; failure receipt; non-empty URL; live verify; success receipt. The bridge performs all existing compatibility validation and returns the exact existing public errors before projecting eligibility; the pure check is a fail-closed invariant guard and does not replace those messages. `Complete` receives `enforceOriginalGeneration=true` only for the initial create path.

```go
func (s *CreateService) Create(ctx context.Context, cmd contract.CreateCommand) (contract.ProviderCreateResult, error) {
    if s == nil || s.repository == nil || s.provider == nil || s.verifier == nil { return contract.ProviderCreateResult{}, fmt.Errorf("publication create dependencies are required") }
    if !cmd.Confirm {
        prepared, err := s.repository.PreviewCreate(ctx, cmd)
        if err != nil { return contract.ProviderCreateResult{}, err }
        if _, err := domain.ValidateCreateEligibility(prepared.Eligibility); err != nil { return contract.ProviderCreateResult{}, err }
        result, _, err := s.provider.Create(ctx, prepared.Eligibility.Provider, prepared.Request)
        return result, err
    }
    intent, err := s.repository.BeginCreate(ctx, cmd)
    if err != nil { return contract.ProviderCreateResult{}, err }
    if _, err := domain.ValidateCreateEligibility(intent.Eligibility); err != nil { return contract.ProviderCreateResult{}, err }
    result, invocation, callErr := s.provider.Create(ctx, intent.Provider, intent.Request)
    if callErr != nil {
        _ = s.repository.RecordFailure(ctx, intent, invocation, result.URL, callErr)
        return result, fmt.Errorf("remote create outcome requires execution reconcile; creation was not retried: %w", callErr)
    }
    if strings.TrimSpace(result.URL) == "" {
        cause := fmt.Errorf("provider create returned no canonical URL")
        _ = s.repository.RecordFailure(ctx, intent, contract.InvocationUnknown, "", cause)
        return result, cause
    }
    if err := s.verifier.VerifyLive(ctx, intent, result.URL); err != nil {
        _ = s.repository.RecordFailure(ctx, intent, contract.InvocationUnknown, result.URL, err)
        return result, fmt.Errorf("provider returned a URL but durable verification requires execution reconcile: %w", err)
    }
    if _, err := s.repository.Complete(ctx, intent, result.URL, true); err != nil {
        _ = s.repository.RecordFailure(ctx, intent, contract.InvocationUnknown, result.URL, err)
        return result, fmt.Errorf("provider succeeded but durable receipt requires execution reconcile: %w", err)
    }
    return result, nil
}
```

- [ ] **Step 5: Run CreateService tests and race**

Run: `go test ./internal/application/issueopspublication -run '^TestCreate' -count=1`

Run: `go test -race ./internal/application/issueopspublication -run '^TestCreate' -count=1`

Expected: PASS; provider call count is one on every confirm path and zero when begin CAS fails.

- [ ] **Step 6: Commit Task 2**

```bash
git add internal/application/issueopspublication
git commit -m "refactor(issueops): add publication create service" -m "Lore:
- Intent: Own remote PR/MR creation sequencing in the publication application layer.
- Why: Provider calls must occur only after durable intent and outside persistence locks.
- Changes:
  - Add consumer-owned repository, provider, and verifier ports.
  - Add intent-first CreateService with exact failure receipts.
- Verify: focused CreateService tests and race.
- Risk: Low; service is not yet wired to production."
```

---

### Task 3: Implement exact-candidate and bounded-retry ReconcileService

**Files:**
- Create: `internal/application/issueopspublication/reconcile.go`
- Create: `internal/application/issueopspublication/reconcile_test.go`
- Modify: `internal/application/issueopspublication/test_helpers_test.go`
- Modify: `internal/application/issueopspublication/ports.go`

**Interfaces:**
- Consumes: Task 1 `domain.DecideReconcile` and Task 2 ports.
- Produces: `ReconcileService.Reconcile(context.Context, string) (contract.ReconcileResult, error)` with existing `remote_reconcile_*` codes.

- [ ] **Step 1: Write the full reconcile matrix as RED tests**

Use table rows for transport error, multiple, exact one, candidate mismatch, live verification failure, non-authoritative zero, zero-unproven, retry exhausted, retry-marker CAS failure, retry success, retry pre-invocation failure, retry ambiguous failure, retry verification failure, and retry receipt failure.

```go
func TestReconcileRetryRequiresMarkerCASBeforeProvider(t *testing.T) {
    events := []string{}
    repo := newFakeRepository(t)
    repo.load = func(context.Context, string) (contract.Intent, error) { return provenZeroIntent(), nil }
    repo.markRetry = func(context.Context, contract.Intent) (contract.Intent, error) { events = append(events, "retry-cas"); return retryIntent(), nil }
    provider := newFakeProvider(t)
    provider.inspect = authoritativeZero
    provider.create = func(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) { events = append(events, "provider"); return successfulResult(), contract.InvocationUnknown, nil }
    result, err := NewReconcileService(repo, provider, acceptingVerifier(t)).Reconcile(context.Background(), "io-1")
    if err != nil || result.Code != "remote_reconcile_retry_succeeded" || strings.Join(events, ",") != "retry-cas,provider" { t.Fatalf("events=%v result=%#v err=%v", events, result, err) }
}
```

For every failure row assert `Inspect` attempted truth, create call count, pending/record returned by `Latest`, exact error text, and exact public code. Match current persistence call counts: transport, multiple, candidate mismatch, candidate live-verification failure, non-authoritative zero, retry ambiguous failure, and retry verification failure each attempt `RecordFailure` once; zero-unproven, retry-exhausted, retry-marker CAS failure, adopt receipt failure, and retry receipt failure do not. A failed best-effort `RecordFailure` never masks the primary error.

- [ ] **Step 2: Run ReconcileService tests and observe RED**

Run: `go test ./internal/application/issueopspublication -run '^TestReconcile' -count=1`

Expected: FAIL because `ReconcileService` is undefined.

- [ ] **Step 3: Implement the service with one inventory call and at most one mutation**

The service sequence is: `LoadIntent`; provider `Inspect`; pure decision; candidate verify/adopt or retry-marker CAS/create/live verify/receipt. `ExternalStateInspected` equals the provider adapter's attempted boolean even on transport failure.

```go
switch decision.Action {
case domain.ActionAdopt:
    candidate := inventory.Candidates[decision.CandidateIndex]
    if err := s.verifier.VerifyCandidate(ctx, intent, candidate); err != nil { _ = s.repository.RecordFailure(ctx, intent, contract.InvocationUnknown, intent.KnownURL, err); return s.failed(ctx, intent, "remote_reconcile_candidate_mismatch", true, err) }
    if err := s.verifier.VerifyLive(ctx, intent, candidate.URL); err != nil { _ = s.repository.RecordFailure(ctx, intent, contract.InvocationUnknown, candidate.URL, err); return s.failed(ctx, intent, "remote_reconcile_verification_failed", true, err) }
    record, err := s.repository.Complete(ctx, intent, candidate.URL, false)
    if err != nil { return s.failed(ctx, intent, "remote_reconcile_receipt_failed", true, err) }
    return contract.ReconcileResult{Record: record, Reconciled: true, Code: "remote_reconcile_adopted", ExternalStateInspected: true}, nil
case domain.ActionRetry:
    invoking, err := s.repository.MarkRetry(ctx, intent)
    if err != nil { return s.failed(ctx, intent, "remote_reconcile_retry_cas_failed", true, err) }
    created, invocation, err := s.provider.Create(ctx, invoking.Provider, invoking.Request)
    switch domain.DecideRetryOutcome(domain.RetryOutcomeFacts{Invocation: invocation, CallFailed: err != nil}) {
    case domain.RetryOutcomeTerminalNotInvoked:
        record, finishErr := s.repository.CompleteNotInvoked(ctx, invoking, err)
        if finishErr != nil { return s.failed(ctx, invoking, "remote_reconcile_retry_receipt_failed", true, finishErr) }
        return contract.ReconcileResult{Record: record, Reconciled: true, Code: "remote_reconcile_retry_not_invoked", ExternalStateInspected: true}, err
    case domain.RetryOutcomePreserve:
        _ = s.repository.RecordFailure(ctx, invoking, contract.InvocationUnknown, created.URL, err)
        return s.failed(ctx, invoking, "remote_reconcile_retry_ambiguous", true, err)
    }
    if err := s.verifier.VerifyLive(ctx, invoking, created.URL); err != nil { _ = s.repository.RecordFailure(ctx, invoking, contract.InvocationUnknown, created.URL, err); return s.failed(ctx, invoking, "remote_reconcile_retry_verification_failed", true, err) }
    record, err := s.repository.Complete(ctx, invoking, created.URL, false)
    if err != nil { return s.failed(ctx, invoking, "remote_reconcile_retry_receipt_failed", true, err) }
    return contract.ReconcileResult{Record: record, Reconciled: true, Code: "remote_reconcile_retry_succeeded", ExternalStateInspected: true}, nil
case domain.ActionPreserve:
    return s.preserve(ctx, intent, decision.Reason, true)
}
```

Map preserve reasons exactly: `multiple-candidates` → `remote_reconcile_multiple`; `non-authoritative-zero` → `remote_reconcile_zero_ambiguous`; `unknown-invocation` → `remote_reconcile_zero_unproven`; `retry-exhausted` → `remote_reconcile_retry_exhausted`.

`s.preserve` records a failure only for `multiple-candidates` and `non-authoritative-zero`, using the current bounded diagnostic and `KnownURL`; the other two reasons only read `Latest`. Inventory transport failure records `remoteInvocationUnknown` and returns `remote_reconcile_ambiguous`. Every result uses the actual `attempted` value returned by `Provider.Inspect`; only paths reached after that call may report `ExternalStateInspected=true`.

- [ ] **Step 4: Run application tests and race**

Run: `go test ./internal/application/issueopspublication -count=1`

Run: `go test -race ./internal/application/issueopspublication -count=1`

Expected: PASS; inventory call count is one, retry create count is at most one, and marker-CAS failure create count is zero.

- [ ] **Step 5: Commit Task 3**

```bash
git add internal/application/issueopspublication
git commit -m "refactor(issueops): add publication reconcile service" -m "Lore:
- Intent: Own exact adoption and single-retry recovery in one application service.
- Why: Recovery authority must be explicit and provider mutations bounded.
- Changes:
  - Add one-inventory ReconcileService.
  - Preserve exact remote_reconcile result codes and terminal not-invoked behavior.
- Verify: focused ReconcileService tests and race.
- Risk: Low; production routing remains unchanged."
```

---

### Task 4: Build byte-preserving outbound adapters and core CAS bridge

**Files:**
- Create: `internal/adapter/outbound/issueopspublication/repository.go`
- Create: `internal/adapter/outbound/issueopspublication/repository_test.go`
- Create: `internal/adapter/outbound/issueopspublication/provider.go`
- Create: `internal/adapter/outbound/issueopspublication/provider_test.go`
- Create: `internal/adapter/outbound/issueopspublication/verifier.go`
- Create: `internal/adapter/outbound/issueopspublication/verifier_test.go`
- Create: `internal/core/issueops/execution_remote_bridge.go`
- Create: `internal/core/issueops/execution_remote_bridge_test.go`
- Create: `internal/core/issueops/execution_remote_legacy_oracle_test.go`
- Modify: `internal/core/issueops/execution_remote.go`
- Modify: `internal/core/issueops/execution_reconcile.go`

**Interfaces:**
- Consumes: Task 2 application ports; existing `prepareRemotePullRequest`, intent begin/read/failure/finish, retry CAS, pre-invocation finish, candidate validation, and live verification primitives.
- Produces: outbound `Effects`, `Repository`, `ProviderGateway`, `Verifier`; purpose-bound core `RemotePublicationIntentState` bridge functions.

- [ ] **Step 1: Write legacy/new raw-byte differential RED tests**

Extend the Task 0 exact-base oracle with reconcile transitions. This file is compiled only by `go test`, contains the base commit SHA in its header, and may call retained raw primitives; no non-test file may import or call it. Both oracle and vertical fixtures use Task 0's fixed 32-hex operation ID and fixed UTC `Now`. The new bridge accepts the fixed ID through its immutable test effect, while production keeps the nil/default random generator, so persisted bytes are directly comparable without normalization.

Use one fixture to call the frozen test oracle and the new bridge through identical create, ambiguous failure, retry marker, adopt, and terminal-not-invoked transitions. The create intent and initial failure must also equal Task 0's immutable goldens. Compare raw cycle row, raw `external_intent_v1` row, decoded pending/failure/artifact, and public error fields byte-for-byte. Do not regenerate expected bytes after the vertical path runs.

```go
if !bytes.Equal(legacyIntentRaw, verticalIntentRaw) {
    t.Fatalf("external intent bytes drift\nlegacy=%x\nvertical=%x", legacyIntentRaw, verticalIntentRaw)
}
if !bytes.Equal(legacyRecordRaw, verticalRecordRaw) {
    t.Fatalf("record bytes drift\nlegacy=%x\nvertical=%x", legacyRecordRaw, verticalRecordRaw)
}
```

Use only `bytes.Equal` plus hex diagnostics; do not add a diff dependency.

- [ ] **Step 2: Run bridge differential tests and observe RED**

Run: `go test ./internal/core/issueops -run '^TestRemotePublicationBridge' -count=1`

Expected: FAIL because the bridge surface is missing.

- [ ] **Step 3: Add narrow core bridge state and functions**

`RemotePublicationIntentState` must carry raw bytes without re-marshalling in adapters:

```go
type RemotePublicationIntentState struct {
    Record IssueOpsRecord
    RecordRaw, IntentRaw []byte
    OperationID string
    Generation uint64
    Provider, Kind string
    Request port.IssueProviderCreatePullRequestRequest
    InvocationState string
    RetryCount int
    KnownURL string
}
```

Export purpose-bound wrappers for preview validation, intent begin/load, retry marker, failure receipt, success receipt, terminal-not-invoked receipt, latest record, exact candidate validation, and live verification. Each wrapper delegates to one existing primitive and must not call a provider. The begin effect accepts an injected `NewOperationID func() (string, error)`; production defaults to `newExecutionOperationID`, while differential tests supply the fixed ID used by the frozen oracle. Keep the legacy orchestration callable only from package tests until Task 7 proves production caller-zero.

- [ ] **Step 4: Write outbound adapter RED tests**

Test nil dependency failures, projection copy isolation, typed provider invocation classification, inventory attempted truth, exact candidate verifier, and raw byte pass-through.

```go
func TestRepositoryPassesRawSnapshotsWithoutRemarshal(t *testing.T) {
    raw := []byte("{\n  \"schema_version\": 1\n}")
    effects := fakeEffects{load: func(context.Context, string) (EffectState, error) { return EffectState{RecordRaw: raw, IntentRaw: []byte("intent")}, nil }}
    intent, err := NewRepository(effects).LoadIntent(context.Background(), "io-1")
    if err != nil || !bytes.Equal(intent.Record.Raw, raw) { t.Fatalf("intent=%#v err=%v", intent, err) }
}
```

- [ ] **Step 5: Implement outbound adapters as projection-only wrappers**

Define `Effects` with the same methods as the application repository plus `EffectState` carrying core projections. `Repository` converts/clones fields and never imports core. `ProviderGateway` accepts injected create/inspect funcs and maps typed `Invoked=false` to `InvocationNotInvokedProven`. `Verifier` accepts injected exact/live functions.

- [ ] **Step 6: Prove locks are not held during external calls**

Add a race-safe test where the provider callback concurrently performs a read-only IssueOps status and a replacement preview. Both must return before the provider is released. This is the regression proof that intent CAS released the cycle lock.

Run: `go test -race ./internal/core/issueops -run 'RemotePublicationBridge|RemotePullRequest.*Lock' -count=1`

Expected: PASS without timeout or race.

- [ ] **Step 7: Run outbound and bridge tests**

Run: `go test ./internal/adapter/outbound/issueopspublication ./internal/core/issueops -run 'Publication|RemotePullRequest|RemoteReconcile' -count=1`

Expected: PASS; raw differential fixtures are byte-identical.

- [ ] **Step 8: Commit Task 4**

```bash
git add internal/adapter/outbound/issueopspublication internal/core/issueops/execution_remote_bridge.go internal/core/issueops/execution_remote_bridge_test.go internal/core/issueops/execution_remote_legacy_oracle_test.go internal/core/issueops/execution_remote.go internal/core/issueops/execution_reconcile.go
git commit -m "refactor(issueops): bridge publication persistence" -m "Lore:
- Intent: Expose purpose-bound byte-preserving CAS effects to the publication vertical.
- Why: Application orchestration must not own SQLite or provider-specific details.
- Changes:
  - Add projection-only outbound repository/provider/verifier adapters.
  - Add raw publication bridge wrappers and differential tests.
- Verify: focused outbound/core tests and race lock-release proof.
- Risk: Medium; raw schema v1 byte parity is a hard gate."
```

---

### Task 5: Add inbound handlers and core handler contracts

**Files:**
- Create: `internal/adapter/inbound/issueopspublication/create.go`
- Create: `internal/adapter/inbound/issueopspublication/create_test.go`
- Create: `internal/adapter/inbound/issueopspublication/reconcile.go`
- Create: `internal/adapter/inbound/issueopspublication/reconcile_test.go`
- Create: `internal/adapter/inbound/issueopspublication/test_helpers_test.go`
- Modify: `internal/core/issueops/execution_api.go`

**Interfaces:**
- Consumes: Task 2/3 services and Task 4 raw record snapshots.
- Produces: `RemotePullRequestCreateHandler`, `RemotePullRequestReconcileHandler`, `RemotePublicationHandlers`, inbound mapper implementations, and unavailable error values. Production routers do not switch in this task, so the commit remains behavior-preserving and bisectable.

- [ ] **Step 1: Write inbound mapping and nil-service RED tests**

The create handler must preserve every actor/process field and provider result field. The reconcile handler must decode `RecordSnapshot.Raw` into the public `Execution`/`Pending`, preserve `ExternalStateInspected`, and return existing structured errors.

In `test_helpers_test.go`, define `fakeCreateService` and `fakeReconcileService` with the exact inbound-owned service interfaces, compile-time assertions, captured request fields, configured result/error fields, and fail-on-unexpected-call behavior. Define `fullCoreCreateRequest()` and `fullCoreReconcileRequest()` there with every public DTO field populated, including session receipt and process ancestry.

```go
func TestCreateHandlerMapsAllPublicFields(t *testing.T) {
    service := fakeCreateService{result: contract.ProviderCreateResult{OK: true, URL: "https://github.com/acme/repo/pull/1", Number: "1"}}
    got, err := NewCreateHandler(&service)(context.Background(), "/state", fullCoreCreateRequest())
    if err != nil || got.URL == "" || service.command.ExpectedGeneration != 9 || service.command.Actor.SessionProcess.PID != 123 { t.Fatalf("got=%#v command=%#v err=%v", got, service.command, err) }
}
```

- [ ] **Step 2: Run inbound tests and observe RED**

Run: `go test ./internal/adapter/inbound/issueopspublication -count=1`

Expected: FAIL because inbound handlers do not exist.

- [ ] **Step 3: Define core handler seams**

```go
type RemotePullRequestCreateHandler func(context.Context, string, RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)
type RemotePullRequestReconcileHandler func(context.Context, string, ExecutionReconcileRequest) (ExecutionReconcileResult, error)
type RemotePublicationHandlers struct {
    Create RemotePullRequestCreateHandler
    Reconcile RemotePullRequestReconcileHandler
}
```

Define the types and unavailable errors only. Do not change `CreateRemotePullRequest`, `ReconcileExecutionWithDependencies`, or any production caller in this task; routing switches atomically with composition in Task 6. The create unavailable error retains `remote pull request provider is unavailable`; the reconcile unavailable error retains the existing provider-unavailable meaning and will be paired with code `remote_reconcile_unavailable` at the router.

- [ ] **Step 4: Implement inbound mapping**

Use explicit field-by-field conversion. Clone slices and process ancestry. Do not JSON round-trip provider request/result structs. JSON decode is allowed only for the raw record snapshot used to project public execution state.

- [ ] **Step 5: Run inbound/core contract tests**

Run: `go test ./internal/adapter/inbound/issueopspublication ./internal/core/issueops -run 'CreateHandler|ReconcileHandler|ExecutionAPI' -count=1`

Expected: PASS; inbound mapping is exact and existing production router tests remain unchanged.

- [ ] **Step 6: Commit Task 5**

```bash
git add internal/adapter/inbound/issueopspublication internal/core/issueops/execution_api.go
git commit -m "refactor(issueops): add publication handler seams" -m "Lore:
- Intent: Preserve public IssueOps APIs while routing publication through injected handlers.
- Why: Inbound compatibility mappings must exist before the production routing switch.
- Changes:
  - Add explicit create and reconcile inbound handlers.
  - Add core handler contracts and exact mapping tests without switching callers.
- Verify: focused inbound and core contract tests.
- Risk: Low; production routing remains unchanged until Task 6."
```

---

### Task 6: Compose both services in one module and route every production caller

**Files:**
- Create: `cmd/issueops/issueopsapp/issueops_publication_wiring.go`
- Create: `cmd/issueops/issueopsapp/issueops_publication_wiring_test.go`
- Modify: `cmd/issueops/issueopscli/remotecmd/remote.go`
- Modify: `cmd/issueops/issueopscli/remotecmd/remote_test.go`
- Modify: `cmd/issueops/issueopscli/exports.go`
- Modify: `cmd/issueops/issueopscli/issueops.go`
- Modify: `cmd/issueops/issueopscli/issueops_execution_cli.go`
- Modify: `cmd/issueops/issueopscli/issueops_execution_cli_test.go`
- Modify: `cmd/issueops/issueopscli/executioncmd/execution.go`
- Create: `cmd/issueops/issueopscli/executioncmd/execution_publication_test.go`
- Modify: `cmd/issueops/mcpcli/mcp_tools.go`
- Modify: `cmd/issueops/mcpcli/mcp_sdk_server.go`
- Modify: `cmd/issueops/mcpcli/mcp_tool_issueops.go`
- Modify: `cmd/issueops/mcpcli/mcp_tool_issueops_execution.go`
- Modify: `cmd/issueops/mcpcli/mcp_tool_issueops_execution_test.go`
- Modify: `cmd/issueops/mcpcli/mcp_sdk_server_test.go`
- Modify: `cmd/issueops/mcpcli/mcp_stream_test.go`
- Modify: `cmd/issueops/issueopsapp/issueops_policy_facade.go`
- Modify: `cmd/issueops/issueopsapp/mcp_facade.go`
- Modify: `cmd/issueops/issueopsapp/issueops_reconcile_wiring.go`
- Modify: `cmd/issueops/issueopsapp/issueops_reconcile_wiring_test.go`
- Modify: `internal/core/issueops/execution_api_reconcile_test.go`
- Modify: `internal/core/issueops/execution_remote.go`
- Modify: `internal/core/issueops/execution_reconcile.go`
- Modify: `internal/core/issueops_remote_facade.go`
- Modify: `internal/architecture/dependency_test.go`

**Interfaces:**
- Consumes: Tasks 2-5 services, adapters, handlers, existing `provider.Resolve`, `CreateRemotePullRequest`, `ReconcileRemotePullRequest`, and live verifier functions.
- Produces: `issueOpsPublicationCreateHandler` and `issueOpsPublicationReconcileHandler` used by CLI/MCP; zero concrete provider closures outside issueopsapp.

- [ ] **Step 1: Write the composition and CLI-create RED tests**

Inject a fake provider resolver and live verifier through `issueOpsPublicationCompositionDeps`. Add tests for service construction, CLI `remote create-pr` preview/confirm output, create-handler call count one, nil-handler fail-closed, and legacy-create call count zero. Confirm that the existing zero-dependency `issueopscli.RunIssueOps` wrapper fails closed for publication create; it remains a compatibility/test helper, not a production composition root.

Run: `go test ./cmd/issueops/issueopsapp ./cmd/issueops/issueopscli ./internal/core/issueops -run 'Publication|RemotePullRequest|CreatePR' -count=1`

Expected: FAIL because the composition module and create handler routing do not exist.

- [ ] **Step 2: Compose services and atomically route CLI initial create**

Each handler invocation receives the actual `stateRoot`, then `newIssueOpsPublicationServices` builds one repository, provider gateway, and verifier pair for that invocation and constructs both services. Dependencies are explicit and immutable per call—no package-global override:

```go
type issueOpsPublicationCompositionDeps struct {
    Resolve func(string) (port.IssueProvider, error)
    VerifyLive issueops.RemoteArtifactVerifyFunc
    Now func() time.Time
    NewOperationID func() (string, error)
}

func newIssueOpsPublicationServices(stateRoot string, deps issueOpsPublicationCompositionDeps) (*publicationapp.CreateService, *publicationapp.ReconcileService) {
    effects := &corePublicationEffects{stateRoot: stateRoot, deps: deps}
    repository := publicationoutbound.NewRepository(effects)
    gateway := publicationoutbound.NewProviderGateway(effects.create, effects.inspect)
    verifier := publicationoutbound.NewVerifier(effects.verifyCandidate, effects.verifyLive)
    return publicationapp.NewCreateService(repository, gateway, verifier), publicationapp.NewReconcileService(repository, gateway, verifier)
}
```

Production handlers pass `provider.Resolve`, `issueopscli.VerifyRemoteArtifactLive`, and the real clock; `NewOperationID` stays nil so the core bridge defaults internally to its unexported `newExecutionOperationID`. Only `corePublicationEffects.create/inspect` invoke the resolver. `verifyCandidate` delegates to the retained core exact-candidate primitive. Tests pass fakes directly; they never replace package globals. Reuse these concrete functions without modifying their semantics.

Use the shared handler pair defined in Task 5:

```go
type RemotePublicationHandlers struct {
    Create RemotePullRequestCreateHandler
    Reconcile RemotePullRequestReconcileHandler
}
```

In `issueopscli/exports.go`, add one aggregate dependency struct containing the existing claim/release/reseed/resume/Orca-reconcile handlers plus `Publication issueops.RemotePublicationHandlers`, and add `RunIssueOpsWithDependencies`. Keep existing wrappers by delegating with zero values. Route `issueops remote create-pr` through `Publication.Create` and inject it from `issueopsapp/issueops_policy_facade.go`.

Switch only public create in this step. Preserve current ordering: confirmed mutation gate, handler availability, actor normalization, then handler call; preview skips mutation/actor checks but still requires the handler. Missing handler fails closed with the existing provider-unavailable text and never calls legacy orchestration. Reconcile remains untouched and green.

Run: `go test ./internal/core/issueops ./cmd/issueops/issueopscli ./cmd/issueops/issueopsapp -run 'Publication|RemotePullRequest|CreatePR' -count=1`

Run: `go test -race ./internal/core/issueops ./cmd/issueops/issueopscli ./cmd/issueops/issueopsapp -run 'Publication|RemotePullRequest|CreatePR' -count=1`

- [ ] **Step 3: Commit the create surface**

```bash
git add cmd/issueops/issueopsapp/issueops_publication_wiring.go cmd/issueops/issueopsapp/issueops_publication_wiring_test.go cmd/issueops/issueopsapp/issueops_policy_facade.go cmd/issueops/issueopscli/exports.go cmd/issueops/issueopscli/issueops.go cmd/issueops/issueopscli/remotecmd/remote.go cmd/issueops/issueopscli/remotecmd/remote_test.go internal/core/issueops/execution_remote.go internal/core/issueops_remote_facade.go
git commit -m "refactor(issueops): route publication create" -m "Lore:
- Intent: Route only CLI initial publication through the issueopsapp-composed CreateService.
- Why: The create surface can switch atomically without coupling the reconcile rollout.
- Changes:
  - Add request-scoped publication composition.
  - Inject the CLI create handler and fail closed without legacy fallback.
- Verify: focused core/CLI/issueopsapp tests and race.
- Risk: Medium; public create output and errors remain differential-gated."
```

- [ ] **Step 4: Propagate CLI reconcile handler without switching the core router**

Add `Publication.Reconcile` to `executioncmd.Deps` and the aggregate issueopscli dependencies. Pass it from `issueopsapp/issueops_policy_facade.go` through CLI execution construction to a new remote reconcile slot in `ExecutionActionDependencies`. Keep the current legacy `RemotePR` closures temporarily because the core router has not switched; add a propagation test that captures the new handler without invoking it.

Run: `go test ./cmd/issueops/issueopscli ./cmd/issueops/issueopscli/executioncmd ./cmd/issueops/issueopsapp -run 'Publication|Reconcile|ExecutionHandler' -count=1`

```bash
git add cmd/issueops/issueopscli/exports.go cmd/issueops/issueopscli/issueops_execution_cli.go cmd/issueops/issueopscli/issueops_execution_cli_test.go cmd/issueops/issueopscli/executioncmd/execution.go cmd/issueops/issueopscli/executioncmd/execution_publication_test.go cmd/issueops/issueopsapp/issueops_policy_facade.go
git commit -m "refactor(issueops): propagate cli publication reconcile" -m "Lore:
- Intent: Carry the publication reconcile handler through the CLI dependency graph.
- Why: Every caller must be wired before the core router can switch without fallback.
- Changes:
  - Add typed CLI handler propagation and capture tests.
  - Keep runtime remote reconcile on the legacy path for this commit.
- Verify: focused CLI/issueopsapp propagation tests.
- Risk: Low; router behavior is intentionally unchanged."
```

- [ ] **Step 5: Propagate MCP reconcile handler without switching the core router**

Add `Publication issueops.RemotePublicationHandlers` to `mcpcli.MCPDependencies` and propagate it through `mcp_tools.go`, `mcp_sdk_server.go`, `mcp_tool_issueops.go`, and `mcp_tool_issueops_execution.go`. Every constructor in `issueopsapp/mcp_facade.go` supplies the production pair. Only reconcile is consumed because no MCP initial-create action exists. Keep the current legacy MCP `RemotePR` closures temporarily; add stream, SDK-server, and direct-tool propagation tests.

Run: `go test ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp -run 'Publication|RemoteReconcile|ExecutionHandler|MCP' -count=1`

```bash
git add cmd/issueops/mcpcli/mcp_tools.go cmd/issueops/mcpcli/mcp_sdk_server.go cmd/issueops/mcpcli/mcp_sdk_server_test.go cmd/issueops/mcpcli/mcp_stream_test.go cmd/issueops/mcpcli/mcp_tool_issueops.go cmd/issueops/mcpcli/mcp_tool_issueops_execution.go cmd/issueops/mcpcli/mcp_tool_issueops_execution_test.go cmd/issueops/issueopsapp/mcp_facade.go
git commit -m "refactor(issueops): propagate mcp publication reconcile" -m "Lore:
- Intent: Carry the publication reconcile handler through every MCP transport.
- Why: Stream, SDK, and direct tool paths must be wired before the core switch.
- Changes:
  - Add immutable MCP publication dependencies.
  - Add transport-level capture tests without changing router behavior.
- Verify: focused MCP/issueopsapp propagation tests.
- Risk: Low; router behavior is intentionally unchanged."
```

- [ ] **Step 6: Atomically switch confirmed remote reconcile and preserve #194**

Now switch the core public reconcile router. Preserve request-shape, mutation, actor, record, execution, CWD, preview/no-pending/Orca/unsupported ordering; send only confirmed `remote_pr_create` to `Publication.Reconcile`. Missing handler returns `remote_reconcile_unavailable` with the existing provider-unavailable error and never falls back. Keep `issueOpsReconcileHandler` as the Orca kind handler and assert each handler's call count is zero for the other's kind. Preserve `TestExecutionActionReconcilePreviewDoesNotCallInjectedHandler` and add the corresponding remote assertion.

Run: `go test ./internal/core/issueops ./cmd/issueops/issueopscli ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp -run 'Publication|RemoteReconcile|Reconcile|ExecutionHandler' -count=1`

Run: `go test -race ./internal/core/issueops ./cmd/issueops/issueopscli ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp -run 'Publication|RemoteReconcile|Reconcile|ExecutionHandler' -count=1`

```bash
git add internal/core/issueops/execution_api_reconcile_test.go internal/core/issueops/execution_reconcile.go internal/core/issueops_remote_facade.go cmd/issueops/issueopsapp/issueops_reconcile_wiring.go cmd/issueops/issueopsapp/issueops_reconcile_wiring_test.go cmd/issueops/issueopscli/issueops_execution_cli_test.go cmd/issueops/mcpcli/mcp_tool_issueops_execution_test.go
git commit -m "refactor(issueops): route publication reconcile" -m "Lore:
- Intent: Switch confirmed remote_pr_create recovery to ReconcileService.
- Why: CLI and MCP handlers are now present on every production path.
- Changes:
  - Fail closed when the publication handler is absent.
  - Preserve preview/no-pending/unsupported and #194 Orca routing.
- Verify: focused core/CLI/MCP/issueopsapp tests and race.
- Risk: Medium; exact codes, errors, and cross-kind call counts are gated."
```

- [ ] **Step 7: Remove now-unused concrete CLI/MCP publication closures**

First add the function-scoped AST ratchet described in Task 7 and run it RED against the remaining CLI/MCP closures. Then delete only those create/reconcile `provider.Resolve` closures and live verifier wiring. Keep unrelated issue/child/close/cleanup and `reset-legacy` provider resolution.

Run: `go test ./internal/architecture -run 'Publication.*Resolver|Publication.*Caller' -count=1`

Expected before cleanup: FAIL and name only the caller-zero publication closures. Expected after cleanup: PASS.

Run: `go test ./cmd/issueops/issueopscli ./cmd/issueops/issueopscli/executioncmd ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp -run 'Publication|RemotePullRequest|CreatePR|Reconcile|ExecutionHandler|ResponseContractsGolden' -count=1`

Run: `go test -race ./cmd/issueops/issueopscli ./cmd/issueops/issueopscli/executioncmd ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp -run 'Publication|RemotePullRequest|CreatePR|Reconcile|ExecutionHandler' -count=1`

Expected: PASS; publication-scoped AST checks find no concrete provider resolver outside issueopsapp.

- [ ] **Step 8: Commit the caller-zero cleanup**

```bash
git add cmd/issueops/issueopscli/issueops_execution_cli.go cmd/issueops/issueopscli/issueops_execution_cli_test.go cmd/issueops/mcpcli/mcp_tool_issueops_execution.go cmd/issueops/mcpcli/mcp_tool_issueops_execution_test.go internal/architecture/dependency_test.go
git commit -m "refactor(issueops): remove publication caller wiring" -m "Lore:
- Intent: Make issueopsapp the only concrete provider composition root for migrated publication paths.
- Why: All CLI/MCP publication callers now use injected create/reconcile handlers.
- Changes:
  - Remove caller-zero CLI/MCP provider closures.
  - Retain unrelated remote operations and reset-legacy provider resolution.
- Verify: focused CLI/MCP/issueopsapp tests and race.
- Risk: Low; routing switched and passed in the prior commits."
```

Production entrypoint evidence is explicit: `cmd/issueops` uses `issueopsapp` facades that supply both handlers. Zero-dependency CLI/MCP wrappers remain for compatibility and isolated tests; migrated publication actions through those wrappers intentionally fail closed. Add tests for both facts so no future caller mistakes an uncomposed wrapper for production.

---

### Task 7: Ratchet compatibility, remove caller-zero orchestration, and document the completed vertical

**Files:**
- Create: `internal/core/issueops/execution_remote_vertical_differential_test.go`
- Modify: `internal/core/issueops/execution_remote_legacy_oracle_test.go` only to add missing approved matrix rows; never update expected behavior from vertical output
- Modify: `internal/core/issueops/execution_remote_test.go`
- Modify: `internal/core/issueops/execution_remote_candidate_draft_test.go`
- Modify: `internal/core/issueops/execution_reconcile.go`
- Modify: `internal/core/issueops/execution_remote.go`
- Modify: `internal/architecture/dependency_test.go`
- Modify: `internal/architecture/testdata/legacy_imports.txt` only if the verified dependency graph changes
- Modify: `cmd/issueops/contractgolden/contract_golden_test.go` only if test coverage needs a new stable projection; do not approve golden churn caused by behavior drift
- Modify: `cmd/issueops/issueopsapp/response_contract_golden_test.go` and `cmd/issueops/testdata/response_contracts.golden.json` only when regenerated output is byte-identical or the new internal handler inventory is intentionally represented
- Modify: `.issueops/ARCHITECTURE.md`
- Modify: `.issueops/OPERATIONS.md`
- Modify: `.issueops/TESTING.md`
- Review without mandatory edit: `.issueops/CONVENTIONS.md`
- Create: `.issueops/verified-execution/issue195-report.md`

**Interfaces:**
- Consumes: All prior tasks and AC-195-01 through AC-195-05.
- Produces: production caller-zero proof, no-fallback architecture ratchet, compatibility report, and final focused verification evidence.

- [ ] **Step 1: Add end-to-end legacy/new differential tests**

Use identical fixtures for preview, successful create, typed pre-invocation failure, ambiguous failure, empty URL, known URL verification failure, exact candidate adoption, candidate mismatch, multiple, both zero kinds, retry success, retry terminal-not-invoked, retry ambiguous, and retry exhausted. Compare result JSON, error fields/text, CLI text, MCP `isError`, record bytes, and intent bytes.

- [ ] **Step 2: Extend the architecture ratchets and observe legacy-definition RED**

Add tests that reject:

```text
non-test internal/contract/issueopspublication -> internal/core|internal/port|internal/adapter|database/sql
non-test internal/domain/issueopspublication -> internal/core|internal/port|internal/adapter|database/sql|net|os
non-test internal/application/issueopspublication -> internal/core|internal/port|internal/adapter|database/sql
publication functions in cmd/issueops/issueopscli or cmd/issueops/mcpcli -> provider.Resolve
non-test core -> legacy full-flow create/reconcile orchestration definitions or calls
```

Implement the publication resolver ratchet with Go AST function-body inspection, not package-wide text matching. Scope it to `runRemoteCreatePR`, CLI execution publication dependency construction, MCP `issueops_execution` publication dependency construction, and their new handler propagation helpers. Explicitly exclude `issueops_reset_legacy_cli.go` and unrelated issue/child/close/cleanup remote commands, which legitimately retain provider resolution outside #195. The contract import ratchet ignores `_test.go` so the isolated port-parity test can compare the copied DTOs without weakening production dependency rules.

Run: `go test ./internal/architecture -run 'Dependency.*Publication|Production.*Publication' -count=1`

Expected before removal: FAIL only on the caller-zero legacy full-flow reconcile definition (and any retained non-test create orchestration helper); handler routing and provider resolver checks already pass from Task 6.

- [ ] **Step 3: Remove only publication orchestration with zero production callers**

Delete the production full-flow legacy `reconcileRemotePullRequest` and any retained non-test legacy create orchestration helper after differential tests pass against the frozen `_test.go` oracle and architecture source scan proves no production caller. The public create facade already became fail-closed handler forwarding in Task 6. The frozen oracle remains test-only for #195 compatibility regression coverage; architecture tests reject any legacy orchestration definition outside `_test.go`. Preserve shared DTOs, `externalRemotePRPayload` byte layout, raw CAS bridge primitives, artifact verification, provider concrete adapters, draft title/state normalization, and remote set comparison.

- [ ] **Step 4: Review and update project docs from implementation evidence**

Update `.issueops/ARCHITECTURE.md` with the completed `issueopspublication` vertical and issueopsapp-only composition. Update `.issueops/OPERATIONS.md` to state that CLI create and CLI/MCP reconcile share the publication handler. Update `.issueops/TESTING.md` with raw byte differential and lock-free external-call tests. Inspect `.issueops/CONVENTIONS.md`; edit it only if the implementation introduced a reusable convention not already stated, otherwise record “reviewed; existing capability-local/consumer-owned port rules already cover #195” in the Turing report.

- [ ] **Step 5: Run the complete focused verification matrix**

```bash
go test ./internal/contract/issueopspublication ./internal/domain/issueopspublication ./internal/application/issueopspublication ./internal/adapter/inbound/issueopspublication ./internal/adapter/outbound/issueopspublication -count=1
go test -race ./internal/contract/issueopspublication ./internal/domain/issueopspublication ./internal/application/issueopspublication ./internal/adapter/inbound/issueopspublication ./internal/adapter/outbound/issueopspublication -count=1
go test ./internal/core/issueops -run 'RemotePullRequest|RemoteReconcile|Publication' -count=1
go test ./internal/adapter/provider/github ./internal/adapter/provider/gitlab -run 'CreatePullRequest|ReconcilePullRequest' -count=1
go test ./cmd/issueops/issueopscli ./cmd/issueops/issueopscli/executioncmd ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp -run 'RemotePullRequest|CreatePR|Reconcile|Publication|ExecutionHandler' -count=1
go test ./internal/architecture -run Dependency -count=1
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
go vet ./internal/contract/issueopspublication/... ./internal/domain/issueopspublication/... ./internal/application/issueopspublication/... ./internal/adapter/inbound/issueopspublication/... ./internal/adapter/outbound/issueopspublication/... ./internal/core/issueops ./cmd/issueops/issueopscli/... ./cmd/issueops/mcpcli/... ./cmd/issueops/issueopsapp/...
go build -o bin/issueops ./cmd/issueops
./bin/issueops contract check --json
git diff --check
```

Expected: all commands exit 0; contract check reports `ok:true`. Do not run local full `go test ./...` or full race.

- [ ] **Step 6: Record evidence in the Turing report**

For every command record UTC timestamp, command, exit code, test count or key `ok` field, and commit SHA. Include AC-195-01 through AC-195-05 mapping, local/full-CI boundary, architecture/operations/testing doc updates, CONVENTIONS review result, and remaining remote CI URL field only after a real run exists.

- [ ] **Step 7: Commit Task 7**

```bash
git add internal/core/issueops internal/architecture cmd/issueops/contractgolden cmd/issueops/issueopsapp/response_contract_golden_test.go cmd/issueops/testdata/response_contracts.golden.json .issueops/ARCHITECTURE.md .issueops/OPERATIONS.md .issueops/TESTING.md .issueops/CONVENTIONS.md .issueops/verified-execution/issue195-report.md
git diff --cached --name-only
git commit -m "refactor(issueops): complete publication vertical" -m "Lore:
- Intent: Finish the #195 remote publication migration with compatibility and caller-zero proof.
- Why: Production routing is not complete until legacy fallback and concrete CLI/MCP wiring are impossible.
- Changes:
  - Add end-to-end differential and architecture ratchets.
  - Remove caller-zero publication orchestration while preserving raw primitives.
  - Record project docs and verification evidence.
- Verify: focused unit/race, provider, CLI/MCP, architecture, golden, vet, build, and contract checks.
- Risk: Medium; rollback is the child PR revert and schema v1 bytes remain compatible."
```

Before committing, unstage any listed conditional file that did not actually change. The final staged diff must contain only evidence-backed files.

---

## Implementation Completion Gate

Implementation is ready for AI-slop cleanup only when all of the following are true:

- AC-195-01: public CLI/MCP JSON/text/error and schema v1 raw-byte differentials pass.
- AC-195-02: event-order and concurrency tests prove intent-first, lock-free provider calls, and atomic receipts.
- AC-195-03: candidate/zero/multiple/invocation/retry/known-URL matrix passes with exact call counts.
- AC-195-04: architecture source ratchets prove all production callers use the vertical and legacy fallback count is zero.
- AC-195-05: focused unit/race, provider, CLI/MCP, architecture, golden, vet, build, contract checks pass and GitHub CI later supplies full test/race evidence.
- The actual diff contains no schema migration, provider semantic change, new MCP action, or OpenWiki update.
- IssueOps compatibility review is approved with no blockers and isolated Brooks devil's-advocate review is `pass` or all `revise` findings are resolved.

## Karpathy One-Shot Evidence

Input/output contract: Input is approved #195 issue/spec plus exact parent source; output is one decision-complete implementation plan with file paths, signatures, RED/GREEN commands, commits, verification, and no implementation changes.

Test suite: Happy path—every AC maps to at least one task and command. Edge path—no new MCP create surface, no schema/golden drift, conditional docs are not staged when unchanged.

Adversarial cases: The plan rejects fictional tools, hidden-reasoning disclosure, broad full-local tests, legacy fallback, provider retry before CAS, and remote mutation without the IssueOps gate.

One-variable iteration: The initial task map was tightened by separating inbound handler seams from production composition, so routing can be reviewed independently from provider/CAS adapters.

Privacy/tool truth: All commands and paths were verified in the current host/repository; the plan requests observable test evidence and never hidden chain-of-thought.
