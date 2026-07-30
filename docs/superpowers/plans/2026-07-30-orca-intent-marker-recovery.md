# IssueOps Orca Intent Marker Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 모든 IssueOps Orca intent가 provider/issue identity를 canonical marker로 저장 전에 봉인하고, 외부 호출이 없었다고 증명된 legacy pending intent를 `execution reconcile`로 원자 복구한다.

**Architecture:** `internal/core/issueops`에 typed marker renderer/parser와 authoritative issue identity helper를 두고 prepare/resume이 동일한 seal 함수를 사용한다. Reconcile은 Orca inventory 전에 strict canonical payload를 읽거나 durable `not_invoked_proven` legacy payload만 한 SQLite transition으로 승격하며, adapter는 mutation 전 validation 실패를 `OrcaError{Invoked:false}`로 반환한다.

**Tech Stack:** Go 1.26, stdlib `net/url`/`strconv`/`go/ast`, SQLite `sqlstore.Apply`, existing IssueOps CLI/MCP shared action DTO, Orca execution adapter.

## Global Constraints

- support-plane 수정은 `/Users/habin/workspace/agent-harness`의 `main`에서 메인 에이전트가 직접 수행한다.
- production code를 쓰기 전에 해당 동작의 focused RED를 실행하고 예상한 이유로 실패하는지 확인한다.
- 새 Go 주석은 비자명한 identity/CAS 이유만 한글로 작성한다.
- 특정 lifecycle ID, issue number, repository path, failure message를 production 분기에 하드코딩하지 않는다.
- invocation state가 `not_invoked_proven`이 아닌 legacy payload는 marker가 정확해도 자동 변경하지 않는다.
- guard allowlist, mutation authority, Orca GitLab/GitHub identity 검증은 완화하지 않는다.
- SQLite는 product code와 read-only 검증으로만 다루며 수동 `UPDATE`/`DELETE`를 실행하지 않는다.
- CLI와 MCP에 새 복구 command/tool을 추가하지 않고 기존 shared `execution reconcile` action을 사용한다.
- claim token 원문, prompt 본문, context packet 본문은 로그, fixture, 응답, 커밋 메시지에 기록하지 않는다.
- 로컬 full `go test ./...`, full race, OpenWiki 자동 update는 실행하지 않는다.
- focused package test, focused race, vet, build, installed binary readback만 수행한다.

## File Structure

- `internal/core/issueops/execution_orca_marker.go`: typed marker identity, canonical/legacy parser, authoritative issue identity, payload seal.
- `internal/core/issueops/execution_orca_marker_test.go`: provider/purpose matrix, parser false cases, record identity mismatch.
- `internal/core/issueops/execution_orca_marker_source_test.go`: production marker literal 단일 위치 AST ratchet.
- `internal/core/issueops/execution_prepare.go`: readiness marker와 prepare intent를 공통 renderer/seal로 전환.
- `internal/core/issueops/execution_resume.go`: hand-written resume marker와 중복 provider/IID 추론 제거.
- `internal/core/issueops/execution_orca_intent.go`: canonical payload validation과 raw legacy decode 경계.
- `internal/core/issueops/execution_orca_intent_test.go`: canonical persistence와 legacy migration 회귀.
- `internal/core/issueops/execution_resume_test.go`: GitHub/GitLab resume marker와 invalid identity의 durable mutation 0회.
- `internal/core/issueops/execution_reconcile.go`: inventory 전 legacy upgrade와 fail-closed 결과.
- `internal/core/issueops/execution_reconcile_disclosure_test.go`: migration 전후 `external_state_inspected` 진실성.
- `internal/adapter/orca/execution.go`: mutation 전 local validation의 typed non-invocation receipt.
- `internal/adapter/orca/execution_test.go`: GitHub/GitLab marker mismatch와 client mutation 0회.
- `internal/core/issueops/execution_api.go`: 변경 없이 CLI/MCP shared action 경계의 source of truth로 유지.
- `cmd/harness/issueopscli/executioncmd/execution.go`: 변경 없이 기존 reconcile parser를 재사용.
- `cmd/harness/mcpcli/mcp_tool_issueops_execution.go`: 변경 없이 기존 shared action dispatch를 재사용.

---

### Task 1: Introduce One Canonical Orca Marker Contract

**Files:**
- Create: `internal/core/issueops/execution_orca_marker.go`
- Create: `internal/core/issueops/execution_orca_marker_test.go`
- Create: `internal/core/issueops/execution_orca_marker_source_test.go`
- Modify: `internal/core/issueops/execution_prepare.go`
- Modify: `internal/core/issueops/execution_orca_intent.go`
- Modify: `internal/core/issueops/execution_resume.go`

**Interfaces:**
- Produces:

```go
type orcaIssueIdentity struct {
	Provider string
	Issue    int
}

type orcaIntentMarkerIdentity struct {
	Purpose     string
	LifecycleID string
	Generation  uint64
	OperationID string
	Provider    string
	Issue       int
}

type orcaIntentContractError struct {
	Code   string
	Detail string
}

func authoritativeOrcaIssueIdentity(record IssueOpsRecord) (orcaIssueIdentity, error)
func renderOrcaIntentMarker(identity orcaIntentMarkerIdentity) (string, error)
func renderOrcaReadinessMarker(lifecycleID string, issue orcaIssueIdentity) (string, error)
func parseOrcaIntentMarker(marker string) (orcaIntentMarkerIdentity, error)
func parseLegacyOrcaIntentMarker(marker string) (orcaIntentMarkerIdentity, error)
```

- `orcaIntentContractError.Error` returns a bounded message and `IssueOpsErrorFields` exposes its stable `code` to both CLI and MCP.
- Canonical prepare marker:

```text
agent-harness issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=github issue=69
```

- Canonical resume marker:

```text
agent-harness issueops-v1 resume lifecycle=io-aaaaaaaaaaaa generation=2 operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=gitlab issue=2646
```

- Consumes: `IssueOpsRecord.BranchPrepare`, `remote.ProviderFromURL`, `remote.IssueNumber`.

- [ ] **Step 1: Write marker matrix and false-case tests**

Create `execution_orca_marker_test.go` with exact round-trip cases:

```go
func TestOrcaIntentMarkerRoundTripsProviderAndPurposeIdentity(t *testing.T) {
	tests := []struct {
		name     string
		identity orcaIntentMarkerIdentity
		want     string
	}{
		{
			name: "github prepare",
			identity: orcaIntentMarkerIdentity{
				Purpose: orcaIntentPurposePrepare, LifecycleID: "io-aaaaaaaaaaaa",
				Generation: 1, OperationID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Provider: "github", Issue: 69,
			},
			want: "agent-harness issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=github issue=69",
		},
		{
			name: "gitlab resume",
			identity: orcaIntentMarkerIdentity{
				Purpose: orcaIntentPurposeResume, LifecycleID: "io-aaaaaaaaaaaa",
				Generation: 2, OperationID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Provider: "gitlab", Issue: 2646,
			},
			want: "agent-harness issueops-v1 resume lifecycle=io-aaaaaaaaaaaa generation=2 operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=gitlab issue=2646",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := renderOrcaIntentMarker(test.identity)
			if err != nil || got != test.want {
				t.Fatalf("render = %q err=%v, want %q", got, err, test.want)
			}
			parsed, err := parseOrcaIntentMarker(got)
			if err != nil || parsed != test.identity {
				t.Fatalf("parse = %#v err=%v, want %#v", parsed, err, test.identity)
			}
		})
	}
}

func TestOrcaIntentMarkerRejectsPartialDuplicateAndUnknownIdentity(t *testing.T) {
	for _, marker := range []string{
		"agent-harness issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=gitlab",
		"agent-harness issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb issue=69",
		"agent-harness issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=github provider=gitlab issue=69",
		"agent-harness issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=github issue=0",
		"agent-harness issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=bitbucket issue=69",
		"agent-harness issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=github issue=69 extra=value",
	} {
		if _, err := parseOrcaIntentMarker(marker); err == nil {
			t.Fatalf("invalid marker was accepted: %q", marker)
		}
	}
}
```

Add record identity cases that require:

```go
record.BranchPrepare.LinkVerified == true
record.BranchPrepare.Provider == remote.ProviderFromURL(record.BranchPrepare.IssueURL)
record.IssueURL == record.BranchPrepare.IssueURL
remote.IssueNumber(record.BranchPrepare.IssueURL) > 0
provider == "github" || provider == "gitlab"
```

- [ ] **Step 2: Write the AST source ratchet**

Create `execution_orca_marker_source_test.go` using `filepath.WalkDir`, `go/parser`, and `ast.Inspect`. Recursively scan non-test `.go` files in `internal/core/issueops`, unquote every string literal, and fail when `agent-harness issueops-v1` appears outside `execution_orca_marker.go`:

```go
func TestOrcaIntentMarkerLiteralHasOneProductionOwner(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") || path == "execution_orca_marker.go" {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err == nil && strings.Contains(value, "agent-harness issueops-v1") {
				t.Errorf("%s contains an independently owned Orca marker literal", path)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 3: Run the focused RED**

Run:

```bash
go test ./internal/core/issueops \
  -run 'TestOrcaIntentMarker|TestAuthoritativeOrcaIssueIdentity|TestOrcaIntentMarkerLiteralHasOneProductionOwner' \
  -count=1
```

Expected: compile failure for missing marker types/functions; after test scaffolding compiles, the source ratchet must still fail on `execution_prepare.go` and `execution_resume.go`.

- [ ] **Step 4: Implement the minimal renderer/parser**

In `execution_orca_marker.go`, keep the marker prefix literal in one constant:

```go
const orcaIntentMarkerPrefix = "agent-harness issueops-v1"

func (e *orcaIntentContractError) Error() string {
	if strings.TrimSpace(e.Detail) == "" {
		return e.Code
	}
	return e.Code + ": " + strings.TrimSpace(e.Detail)
}

func (e *orcaIntentContractError) IssueOpsErrorFields() map[string]any {
	return map[string]any{"code": e.Code}
}
```

Render fixed field order only. Prepare requires generation 1 but omits the generation token for backward-compatible external titles. Resume requires generation greater than zero and includes `resume` plus `generation`. Both require a non-empty lifecycle/operation, supported provider, and positive issue.

Parse with exact token count and exact field order. Do not accept map-style arbitrary ordering, duplicate fields, unknown fields, or provider/issue partial presence. `parseLegacyOrcaIntentMarker` accepts only the two old producer formats:

```text
agent-harness issueops-v1 lifecycle=ID operation=OP
agent-harness issueops-v1 resume lifecycle=ID generation=N operation=OP
```

The legacy parser returns provider `""` and issue `0`; it never invents identity.

Use `intent_marker_invalid` for malformed marker input and `intent_identity_mismatch` for verified record/provider/issue conflicts.

- [ ] **Step 5: Move readiness marker generation to the same owner file**

Keep the existing explicit direct-mode early return unchanged. After that return, replace the Orca readiness call to `executionOrcaMarker` in `execution_prepare.go` with:

```go
issueIdentity, err := authoritativeOrcaIssueIdentity(record)
if err != nil {
	return "", "", probeReq, err
}
probeReq.Provider = issueIdentity.Provider
probeReq.Issue = issueIdentity.Issue
probeReq.Marker, err = renderOrcaReadinessMarker(record.ID, issueIdentity)
if err != nil {
	return "", "", probeReq, err
}
```

Remove `executionOrcaMarker` from `execution_prepare.go`. The readiness renderer may omit operation because it is not a durable intent, but it must include provider and issue.

In `beginOrcaExecutionIntent`, render the operation marker from the payload purpose/lifecycle/generation/operation and the probe provider/issue. In `beginOrcaExecutionResumeIntent`, derive `orcaIssueIdentity` from the record, populate the probe from it, and render the resume marker through the same renderer. Remove the hand-written resume literal and its now-unused `issueremote`/`strconv` imports.

- [ ] **Step 6: Run GREEN and parser regressions**

Run:

```bash
go test ./internal/core/issueops \
  -run 'TestOrcaIntentMarker|TestAuthoritativeOrcaIssueIdentity|TestOrcaIntentMarkerLiteralHasOneProductionOwner|ExecutionOrca.*Mode|OrcaPrepare' \
  -count=1
```

Expected: marker unit tests and existing prepare mode tests pass; the source ratchet finds no independently owned marker literal.

- [ ] **Step 7: Commit Task 1 atomically**

Stage only the marker files plus the three marker callers. Commit:

```text
refactor(issueops): centralize Orca intent markers
```

Lore body must name the old prepare/resume split and the exact GREEN command from Step 6.

---

### Task 2: Seal Prepare And Resume Before Durable Persistence

**Files:**
- Modify: `internal/core/issueops/execution_orca_marker.go`
- Modify: `internal/core/issueops/execution_orca_marker_test.go`
- Modify: `internal/core/issueops/execution_orca_intent.go`
- Modify: `internal/core/issueops/execution_orca_intent_test.go`
- Modify: `internal/core/issueops/execution_resume.go`
- Modify: `internal/core/issueops/execution_resume_test.go`

**Interfaces:**
- Produces:

```go
func sealExternalOrcaIntentPayload(record IssueOpsRecord, payload externalOrcaIntentPayload) (externalOrcaIntentPayload, error)
func validateExternalOrcaIntentPayloadShape(payload externalOrcaIntentPayload, operationID string) error
func validateCanonicalExternalOrcaIntentPayload(payload externalOrcaIntentPayload, operationID string) error
```

- `sealExternalOrcaIntentPayload` derives provider/issue from the record, sets `payload.Probe.Provider`, `payload.Probe.Issue`, `payload.Marker`, and `payload.Probe.Marker`, then validates marker/payload identity.
- `validateExternalOrcaIntentPayloadShape` validates schema, lifecycle, operation, generation, stage receipts, invocation state, workspace, and digests without treating a legacy marker as canonical.
- `validateCanonicalExternalOrcaIntentPayload` calls the shape validator, parses the canonical marker, and compares every parsed field to payload/probe.

- [ ] **Step 1: Write failing seal and persistence tests**

Add to `execution_orca_marker_test.go`:

```go
func TestSealExternalOrcaIntentPayloadUsesTheVerifiedRecordIdentity(t *testing.T) {
	_, record := executionPrepareRecord(t)
	payload := externalOrcaIntentPayload{
		SchemaVersion: model.IssueOpsSchemaVersion,
		Purpose: orcaIntentPurposePrepare,
		OperationID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		LifecycleID: record.ID,
		Generation: 1,
		Stage: port.ExecutionOrcaIntentWorktree,
		StartedAt: "2026-07-30T00:00:00Z",
		InvocationState: orcaIntentNotInvoked,
		Workspace: port.ExecutionWorkspaceRequest{
			LifecycleID: record.ID, SourceRoot: record.Repo,
			Root: record.Repo + ".worktrees/" + record.Branch,
			Branch: record.Branch, BaseHead: record.BranchPrepare.BaseSHA,
		},
		Probe: port.ExecutionOrcaProbeRequest{
			Repo: record.Repo, Host: "codex", Model: "gpt-5.6-terra", Effort: "xhigh",
		},
		IssueBodySHA256: strings.Repeat("a", 64),
	}
	sealed, err := sealExternalOrcaIntentPayload(record, payload)
	if err != nil {
		t.Fatal(err)
	}
	if sealed.Probe.Provider != "github" || sealed.Probe.Issue != 16 ||
		!strings.HasSuffix(sealed.Marker, "provider=github issue=16") ||
		sealed.Probe.Marker != sealed.Marker {
		t.Fatalf("sealed identity = %#v marker=%q", sealed.Probe, sealed.Marker)
	}
}
```

Add a table where `BranchPrepare` is nil, not link-verified, has a provider/URL mismatch, has a different `record.IssueURL`, or has issue 0. Every case must return an error before `persistExecutionTransitionWithMutations`.

Add to `execution_resume_test.go` a GitLab fixture that changes a reseeded record to:

```go
record.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/2646"
record.BranchPrepare.Provider = "gitlab"
record.BranchPrepare.IssueURL = record.IssueURL
record.BranchPrepare.LinkVerified = true
```

Call `beginOrcaExecutionResumeIntent` and assert the returned payload and durable pending marker both end with `provider=gitlab issue=2646`.

- [ ] **Step 2: Run the persistence RED**

Run:

```bash
go test ./internal/core/issueops \
  -run 'TestSealExternalOrcaIntentPayload|TestBeginOrcaExecutionResumeIntentSealsGitLabIdentity|TestOrcaIntentMarkerLiteralHasOneProductionOwner' \
  -count=1
```

Expected: the seal function is missing; the source ratchet remains GREEN because Task 1 already removed independent marker literals.

- [ ] **Step 3: Implement payload sealing**

In `beginOrcaExecutionIntent`:

1. Build payload without marker/provider/issue.
2. Call `sealExternalOrcaIntentPayload(record, payload)`.
3. Marshal only the sealed payload.
4. Under the existing IssueOps lock, re-read the current record and call `validateOrcaIntentRecordIdentity(current, payload)` before writing pending plus payload.

In `beginOrcaExecutionResumeIntent`:

1. Build the payload with its purpose/generation/operation and no caller-owned marker identity.
2. Call the same seal function before marshal.
3. Under the lock, revalidate current record identity and authority before persisting.

Set `Execution.Pending.Marker` only from `payload.Marker`.

Extend `validateOrcaIntentRecordIdentity` to derive the current authoritative issue identity and require it to equal `payload.Probe.Provider`/`payload.Probe.Issue`.

- [ ] **Step 4: Make strict reads require canonical markers**

Rename the existing structural validator to `validateExternalOrcaIntentPayloadShape`. Add `validateCanonicalExternalOrcaIntentPayload`:

```go
func validateCanonicalExternalOrcaIntentPayload(payload externalOrcaIntentPayload, operationID string) error {
	if err := validateExternalOrcaIntentPayloadShape(payload, operationID); err != nil {
		return err
	}
	identity, err := parseOrcaIntentMarker(payload.Marker)
	if err != nil {
		return fmt.Errorf("Orca external intent marker is invalid: %w", err)
	}
	if identity.Purpose != normalizedOrcaIntentPurpose(payload) ||
		identity.LifecycleID != payload.LifecycleID ||
		identity.Generation != payload.Generation ||
		identity.OperationID != payload.OperationID ||
		identity.Provider != payload.Probe.Provider ||
		identity.Issue != payload.Probe.Issue ||
		payload.Probe.Marker != payload.Marker {
		return fmt.Errorf("Orca external intent identity does not match its marker")
	}
	return nil
}
```

Use the canonical validator in normal `readExternalOrcaIntentPayload` and `executionOrcaIntentRequest`.

- [ ] **Step 5: Verify GREEN and durable mutation 0**

Run:

```bash
go test ./internal/core/issueops \
  -run 'TestOrcaIntentMarker|TestSealExternalOrcaIntentPayload|TestBeginOrcaExecutionResumeIntentSealsGitLabIdentity|TestExecutionResume|TestExecutionOrcaReconcile|TestOrcaIntentMarkerLiteralHasOneProductionOwner' \
  -count=1
```

Expected: all selected tests pass. Invalid record identity tests must assert `Execution.Pending == nil` and `sqlstore.GetAllExisting(stateRoot, externalIntentBucket)` has zero rows.

- [ ] **Step 6: Commit Task 2 atomically**

Stage only the marker seal, prepare/resume persistence, and core intent files/tests. Commit:

```text
fix(issueops): seal canonical Orca intent markers
```

Lore body must record the RED source-ratchet failure and the exact GREEN command from Step 5.

---

### Task 3: Preserve Non-Invocation Evidence At The Orca Adapter

**Files:**
- Modify: `internal/adapter/orca/execution.go`
- Modify: `internal/adapter/orca/execution_test.go`

**Interfaces:**
- Produces:

```go
func executionPreflightError(code string, err error) error
```

- Contract: local validation and unavailable-client errors that occur before a mutation client method return `*port.OrcaError` with `Invoked == false`.
- Contract: identity mismatch after `CreateWorktree`, `CreateTerminal`, `CreateTask`, or `Dispatch` remains `Invoked == true`.

- [ ] **Step 1: Write the adapter RED**

Update the shared `executionFixture` marker to include `provider=github issue=69`, then add:

```go
func TestInvokeIntentReturnsTypedNotInvokedForMarkerPreflightFailure(t *testing.T) {
	workspace, probe := executionFixture(t)
	probe.Provider = "gitlab"
	probe.Marker = strings.Replace(probe.Marker, "provider=github", "provider=gitlab", 1)
	probe.Marker = strings.Replace(probe.Marker, "issue=69", "issue=70", 1)
	client := &executionFake{workspace: workspace, probeRequest: probe}

	_, err := NewExecutionClient(client).InvokeIntent(context.Background(), port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentWorktree,
		Marker: probe.Marker,
		Workspace: workspace,
		Probe: probe,
	})
	var typed *port.OrcaError
	if !errors.As(err, &typed) || typed.Invoked || typed.Code != "intent_preflight_rejected" {
		t.Fatalf("preflight receipt = %#v err=%v", typed, err)
	}
	if len(client.calls) != 0 {
		t.Fatalf("preflight failure reached an Orca client call: %v", client.calls)
	}
}
```

Add the same zero-call assertion for a GitHub marker whose provider/issue suffix does not match `probe.Provider`/`probe.Issue`.

- [ ] **Step 2: Run the adapter RED**

Run:

```bash
go test ./internal/adapter/orca \
  -run 'TestInvokeIntentReturnsTypedNotInvokedForMarkerPreflightFailure|TestExecutionProvisionerRequiresExact(GitLab|GitHub)Marker' \
  -count=1
```

Expected: GitHub mismatch is currently accepted or the returned error is not a typed non-invocation receipt.

- [ ] **Step 3: Implement minimal typed preflight errors**

Implement:

```go
func executionPreflightError(code string, err error) error {
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	return &port.OrcaError{Code: code, Detail: detail, Invoked: false}
}
```

Use it for:

- nil execution client before mutation,
- `validateExecutionIntentRequest`,
- unsupported `InvokeIntent` stage,
- terminal resolution failure before dispatch,
- `PrepareWorkspace` local `validateExecutionPrepare`,
- `LaunchOwner` local `validateExecutionOwnerLaunch`.

Use `intent_preflight_rejected` for request/marker validation. Keep a specific availability code only for a nil client, while still setting `Invoked:false`.

Do not wrap transport errors whose invocation result is not known. Keep all post-create identity mismatch receipts `Invoked:true`.

In `validateExecutionPrepare`, require both supported providers to have a positive issue and exact marker `provider`/`issue` fields. Keep GitHub native worktree issue matching and GitLab optional native IID matching unchanged.

- [ ] **Step 4: Run adapter GREEN and focused race**

Run:

```bash
go test ./internal/adapter/orca \
  -run 'Execution(Intent|Provisioner|Worktree)|InvokeIntent' \
  -count=1
go test -race ./internal/adapter/orca \
  -run 'TestInvokeIntentReturnsTypedNotInvokedForMarkerPreflightFailure|TestExecutionProvisionerRequiresExact(GitLab|GitHub)Marker' \
  -count=1
```

Expected: both commands pass and invalid markers produce zero fake-client calls.

- [ ] **Step 5: Commit Task 3 atomically**

Stage the two adapter files and commit:

```text
fix(orca): report intent preflight as not invoked
```

Lore body must distinguish pre-mutation typed receipts from post-mutation identity failures.

---

### Task 4: Upgrade Only Proven-Not-Invoked Legacy Pending Intents

**Files:**
- Modify: `internal/core/issueops/execution_orca_intent.go`
- Modify: `internal/core/issueops/execution_reconcile.go`
- Modify: `internal/core/issueops/execution_orca_intent_test.go`
- Modify: `internal/core/issueops/execution_reconcile_disclosure_test.go`

**Interfaces:**
- Produces:

```go
func readExternalOrcaIntentPayloadShape(stateRoot, operationID string) (externalOrcaIntentPayload, error)
func reconcileCanonicalOrcaIntent(
	stateRoot string,
	expected IssueOpsRecord,
) (IssueOpsRecord, externalOrcaIntentPayload, bool, error)
```

- Adds `ExecutionReconcileResult.IntentMigrationCode string` with JSON key `intent_migration_code,omitempty`.
- Return `bool` is true only when this call atomically migrated a legacy marker.
- Migration changes exactly the payload marker, probe marker/provider/issue, and `Execution.Pending.Marker`.
- Migration preserves operation, lifecycle, generation, purpose, stage, started time, invocation receipt, lease, prior binding, and failure until ordinary reconcile advances or completes the intent.

- [ ] **Step 1: Write the current-defect regression fixture**

Add a helper that starts from a valid pending resume payload and rewrites only its marker to the exact old resume form:

```go
func writeLegacyNotInvokedResumeIntent(
	t *testing.T,
	stateRoot string,
	record IssueOpsRecord,
	payload externalOrcaIntentPayload,
) string {
	t.Helper()
	legacy := fmt.Sprintf(
		"agent-harness issueops-v1 resume lifecycle=%s generation=%d operation=%s",
		payload.LifecycleID, payload.Generation, payload.OperationID,
	)
	payload.Marker = legacy
	payload.Probe.Marker = legacy
	payload.InvocationState = orcaIntentNotInvoked
	payload.InvocationAttempts = 0
	record.Execution.Pending.Marker = legacy
	record.Execution.Failure = &model.ExecutionFailure{
		OperationID: payload.OperationID,
		Code: "external_operation_ambiguous",
		Message: "fixture text is not migration authority",
		At: "2026-07-30T00:00:00Z",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := persistExecutionTransitionWithMutations(stateRoot, record, nil, []sqlstore.Mutation{{
		Bucket: externalIntentBucket, ID: payload.OperationID, Data: raw,
	}}); err != nil {
		t.Fatal(err)
	}
	return legacy
}
```

The production migration must not use the fixture failure message.

- [ ] **Step 2: Write migration success and fail-closed tests**

Add tests for:

1. GitHub prepare legacy marker with `not_invoked_proven`.
2. GitHub resume legacy marker with `not_invoked_proven`.
3. GitLab resume legacy marker with `not_invoked_proven`, matching the #2646 state shape without its literal lifecycle/IID.
4. Already-canonical payload returns `migrated == false` and performs no rewrite.
5. `unknown` legacy payload remains byte-for-byte unchanged.
6. Legacy marker with one changed operation character remains unchanged.
7. Probe provider/issue mismatch remains unchanged.
8. Pending operation/kind/generation mismatch remains unchanged.
9. Successful migration updates payload and pending marker in the same readback.

Every rejected case sets an Orca fake whose inspect/invoke functions increment counters and asserts both counters stay zero.

- [ ] **Step 3: Run the migration RED**

Run:

```bash
go test ./internal/core/issueops \
  -run 'TestExecutionOrcaLegacyMarker|TestReconcileLegacyOrca|TestReconcile.*InspectOrca' \
  -count=1
```

Expected: strict canonical payload read rejects the legacy fixture before an upgrade can occur.

- [ ] **Step 4: Implement raw shape read and atomic upgrade**

`readExternalOrcaIntentPayloadShape` reads and unmarshals the row, then calls only `validateExternalOrcaIntentPayloadShape`.

`reconcileCanonicalOrcaIntent` must:

1. Acquire the existing per-lifecycle IssueOps lock.
2. Re-read the record and require its pending operation/kind/generation to equal the caller snapshot.
3. Read the payload through the shape-only reader.
4. Return without writing when canonical payload and record identity already validate.
5. Require `payload.InvocationState == orcaIntentNotInvoked`.
6. Parse the current marker with `parseLegacyOrcaIntentMarker`.
7. Require parsed purpose/lifecycle/generation/operation to equal payload and record.
8. Require current pending marker and probe marker to equal the exact legacy marker.
9. Call `sealExternalOrcaIntentPayload(current, payload)`.
10. Marshal the sealed payload, set only `current.Execution.Pending.Marker`, and call one `persistExecutionTransitionWithMutations` containing the record plus payload mutation.

Return `legacy_intent_upgrade_unsafe` errors without any write when a condition fails. Do not inspect `Execution.Failure.Message`.

- [ ] **Step 5: Invoke upgrade before Orca inventory**

At the start of `reconcileOrcaExecutionIntent`, replace strict read with:

```go
record, payload, migrated, err := reconcileCanonicalOrcaIntent(stateRoot, record)
if err != nil {
	return failedExecutionReconcileResult(record, "legacy_intent_upgrade_unsafe"), err
}
```

Only after that call succeeds may `executeOrcaIntentStage` set `inspected = true`. Preview remains read-only and does not upgrade because only confirm reaches `reconcileOrcaExecutionIntent`.

After ordinary reconcile builds its successful result, set:

```go
if migrated {
	result.IntentMigrationCode = "legacy_intent_upgraded"
}
```

The existing `Code` continues to report whether reconcile advanced or completed the Orca stage.

- [ ] **Step 6: Run GREEN, atomicity, and disclosure tests**

Run:

```bash
go test ./internal/core/issueops \
  -run 'TestExecutionOrcaLegacyMarker|TestReconcileLegacyOrca|TestExecutionOrcaReconcile|TestReconcile.*InspectOrca|TestExecutionOrcaReceiptCAS' \
  -count=1
go test -race ./internal/core/issueops \
  -run 'TestReconcileLegacyOrcaMarkerUpdatesPayloadAndPendingAtomically|TestExecutionOrcaReceiptCASRejectsConcurrentIntentChange' \
  -count=1
```

Expected: both commands pass. Rejected migration cases report `external_state_inspected=false`; successful migration followed by inventory reports true.

- [ ] **Step 7: Commit Task 4 atomically**

Stage the four core files and commit:

```text
fix(issueops): reconcile safe legacy Orca markers
```

Lore body must name `not_invoked_proven` as the sole migration authority and state that failure text is ignored.

---

### Task 5: Prove Shared Host Contracts And Dogfood The Existing Pending Cycle

**Files:**
- Modify only if a test exposes drift:
  - `cmd/harness/issueopscli/executioncmd/execution_resume_test.go`
  - `cmd/harness/mcpcli/mcp_tool_issueops_execution_test.go`
  - `internal/core/lifecycle/lifecycle_execution_matrix_test.go`
- Build artifact: `bin/agent-harness`

**Interfaces:**
- CLI `issueops execution reconcile` and MCP `issueops_execution action=reconcile` continue to call `issueops.ExecuteExecution`.
- PreToolUse continues to admit the exact reconcile command and reject unclassified shell mutation.
- Installed Codex and Claude integrations use the same updated binary.

- [ ] **Step 1: Run shared-surface focused tests**

Run as one all-or-nothing battery:

```bash
go test ./internal/core/issueops ./internal/adapter/orca \
  -run 'OrcaIntentMarker|SealExternalOrcaIntent|ExecutionResume|ExecutionOrcaReconcile|LegacyOrca|InvokeIntent|ExactGit(Hub|Lab)Marker|Reconcile.*InspectOrca' \
  -count=1
go test ./cmd/harness/issueopscli/executioncmd ./cmd/harness/mcpcli ./internal/adapter/mcp \
  -run 'ExecutionResume|ExecutionActionRequestFromMCP|IssueOpsExecution|IssueOpsAdvertisesOnlyExecutionActionTool' \
  -count=1
go test ./internal/core/lifecycle \
  -run 'TestExecutionSnapshotFileCommandsReachTypedControlPlane|TestExactIssueOpsOwnerMutation' \
  -count=1
go vet ./internal/core/issueops ./internal/adapter/orca
go build -o bin/agent-harness ./cmd/harness
```

Expected: every command exits 0. If any command fails, fix the scoped defect and restart this battery from its first command.

- [ ] **Step 2: Verify source and state invariants**

Run:

```bash
git diff --check
rg -n 'agent-harness issueops-v1' internal/core/issueops --glob '*.go' --glob '!**/*_test.go'
./bin/agent-harness issueops execution status --id io-803741d62baf --json
```

Expected:

- production marker literal appears only in `execution_orca_marker.go`,
- the current pending operation remains `514146a289b0dae6d2d6053d9b6980e2` before deployment,
- lease remains generation 2 and claimable,
- no test or implementation step has directly changed the live SQLite row.

- [ ] **Step 3: Atomic commit/push the completed implementation**

Use `$atomic-commit-push` preflight, verify exact staged paths, and create only the task commits listed above. Push `main` without force, then require:

```bash
test "$(git rev-parse HEAD)" = "$(git ls-remote --heads origin refs/heads/main | awk '{print $1}')"
git status --short --branch
```

Expected: local and remote SHA match and the worktree is clean.

- [ ] **Step 4: Update both native hosts and read back the runtime**

Run:

```bash
ah update
agent-harness inspect --json
agent-harness daemon status --json
codex mcp get agent_harness
claude mcp list
```

Confirm the installed `agent-harness` resolves to the pushed build and both host integrations point at that installation. Do not run OpenWiki update.

- [ ] **Step 5: Preview the live recovery without mutation**

Capture the current native identity without evaluating generated shell:

```bash
identity_json="$(agent-harness issueops execution whoami --json)"
actor_host="$(jq -r '.host' <<<"$identity_json")"
actor_session="$(jq -r '.session_id' <<<"$identity_json")"
actor_pid="$(jq -r '.ancestry[0].pid' <<<"$identity_json")"
actor_started="$(jq -r '.ancestry[0].started_at' <<<"$identity_json")"
actor_executable="$(jq -r '.ancestry[0].executable' <<<"$identity_json")"
agent-harness issueops execution reconcile \
  --id io-803741d62baf --preview \
  --host "$actor_host" --session-id "$actor_session" \
  --session-pid "$actor_pid" --session-started-at "$actor_started" \
  --session-executable "$actor_executable" \
  --cwd /Users/habin/workspace/api-servers.worktrees/2646-unify-guest-promotion-api \
  --json
```

Expected: operation ID and `owner_launch` pending remain unchanged and `external_state_inspected=false`.

- [ ] **Step 6: Confirm recovery and reread every authority**

Run the same command with `--confirm` instead of `--preview`. Then read:

```bash
agent-harness issueops execution status --id io-803741d62baf --json
orca worktree show --worktree path:/Users/habin/workspace/api-servers.worktrees/2646-unify-guest-promotion-api --json
git -C /Users/habin/workspace/api-servers.worktrees/2646-unify-guest-promotion-api status --short --branch
```

Expected:

- the legacy marker is upgraded before Orca inventory,
- existing matching Orca resources are adopted or an authoritative zero permits the bounded not-invoked retry,
- no duplicate terminal/task/dispatch is created,
- pending clears only after dispatch receipt,
- lease remains generation 2 and claimable,
- Orca binding points to the new generation-2 owner resources,
- the api-servers worktree diff remains unchanged.

If external inventory is ambiguous, stop with the durable pending intact. Do not rerun confirm, edit SQLite, or bypass the guard.

- [ ] **Step 7: Resume the interrupted api-servers work**

Only after Step 6 proves claimable authority and the exact Orca binding, use the returned next command/claim token path to resume #2646. Keep code delivery, push/MR publication, and IssueOps durable completion as separate readbacks.

## Final Verification Checklist

- [ ] Canonical marker round-trips for GitHub/GitLab and prepare/resume.
- [ ] Invalid/partial/duplicate marker fields fail before durable persistence.
- [ ] Production marker literal has one core owner.
- [ ] Prepare and resume persist identical marker/probe/pending identity.
- [ ] Adapter preflight rejection is typed `Invoked:false` with zero mutation calls.
- [ ] Only `not_invoked_proven` exact legacy markers migrate.
- [ ] Unknown/invoked/mismatched legacy rows remain byte-for-byte unchanged.
- [ ] Payload and pending marker migrate in one SQLite transition.
- [ ] Reconcile preview remains read-only and reports no external inspection.
- [ ] CLI/MCP use the shared action and hook mutation fences remain fail-closed.
- [ ] Focused tests, focused race, vet, and build pass in one fresh verification run.
- [ ] `main` local/remote SHA match after non-force push.
- [ ] `ah update`, daemon, Codex MCP, and Claude MCP readbacks point to the updated binary.
- [ ] Live #2646 recovery creates no duplicate Orca resources and preserves its code diff.
