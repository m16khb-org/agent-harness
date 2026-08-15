# IssueOps Orca Owner Resume Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 재봉인된 holderless IssueOps generation에 새 Orca terminal/task/dispatch를 fail-closed로 연결하고 durable binding이 현재 세대 owner를 가리키게 한다.

**Architecture:** `execution resume`은 기존 worktree와 generation별 sealed artifact를 입력으로 기존 Orca external-intent state machine을 `terminal_create`부터 재사용한다. `OrcaBinding.lease_generation`과 intent purpose/lease/binding snapshot으로 prepare와 resume의 CAS를 분리하며, ambiguous 외부 결과는 기존 `execution reconcile`이 이어받는다.

**Tech Stack:** Go 1.26, SQLite state store, Orca CLI adapter, stdlib `flag`, JSON MCP schema/golden tests.

## Global Constraints

- 현재 사용자가 명시한 대로 support-plane 수정은 `/Users/sample/workspace/agent-harness`의 `main`에서 수행한다.
- 모든 production edit는 먼저 예상 이유의 RED를 확인한 뒤 최소 구현으로 GREEN을 만든다.
- 새 코드 주석은 비자명한 authority/CAS 이유만 한국어로 작성한다.
- direct mode, branch/base/parent lineage, active lease replacement, cleanup 동작은 변경하지 않는다.
- Codex owner 기본값은 `gpt-5.6-terra/xhigh`, Claude owner 기본값은 `claude-sonnet-5/high`를 유지한다.
- MCP는 새 tool을 만들지 않고 단일 `issueops_execution`의 `action=resume`만 추가한다.
- 로컬 `go test ./...`, 전체 race, OpenWiki 자동 update는 실행하지 않는다.
- 변경된 패키지의 focused test/race/vet/build만 로컬에서 실행하고 full suite는 원격 CI에 맡긴다.
- claim token 원문은 응답, 로그, fixture, commit message에 기록하지 않는다.

## File Structure

- `internal/core/issueops/model/execution.go`: Orca binding의 generation 정체와 backward-compatible validation.
- `internal/core/issueops/execution_resume.go`: resume request/result, 상태 검증, 기존 owner 판정, resume intent 실행, claim handoff 결과.
- `internal/core/issueops/execution_resume_test.go`: resume 상태 행렬, 멱등성, CAS 보존, ambiguous recovery.
- `internal/core/issueops/execution_orca_intent.go`: prepare/resume purpose를 구분하는 durable payload와 purpose-aware receipt CAS.
- `internal/core/issueops/execution_orca_intent_test.go`: prepare regression과 legacy payload compatibility.
- `internal/core/issueops/execution_api.go`: shared CLI/MCP action DTO와 resume dispatch.
- `cmd/harness/issueopscli/executioncmd/execution.go`: `execution resume` CLI parser.
- `cmd/harness/issueopscli/executioncmd/execution_resume_test.go`: CLI request mapping과 required generation/confirm.
- `internal/adapter/mcp/issueops_catalog.go`: 단일 MCP tool의 action enum/description.
- `internal/adapter/mcp/issueops_catalog_test.go`: 단일-tool invariant와 resume enum.
- `cmd/harness/mcpcli/mcp_tool_issueops_execution_test.go`: MCP request mapping.
- `internal/core/commandparse/issueops.go`: exact resume flag catalog.
- `internal/core/commandparse/issueops_test.go`: exact/near-miss parser cases.
- `internal/core/lifecycle/lifecycle_execution_guard.go`: confirmed resume control-plane admission.
- `internal/core/lifecycle/lifecycle_execution_matrix_test.go`: active-authority allow/deny matrix.
- `cmd/harness/contractcli/contract.go`: resume CLI response shape.
- `cmd/harness/testdata/usage.golden.txt`, `cmd/harness/testdata/mcp_tools.golden.json`, `cmd/harness/testdata/response_contracts.golden.json`: public contract goldens.
- `skills/issueops/references/execution.md`: reseed → resume → claim 운영 순서.
- `docs/superpowers/specs/2026-07-30-issueops-orca-owner-resume-design.md`: code-map self-review에서 확인한 finite next-command 규칙 보강.

---

### Task 1: Bind Every New Orca Owner To Its Lease Generation

**Files:**
- Modify: `internal/core/issueops/model/execution.go`
- Create: `internal/core/issueops/model/execution_test.go`
- Modify: `internal/core/issueops/execution_orca_intent.go`
- Modify: `internal/core/issueops/execution_owner_context_test.go`

**Interfaces:**
- Produces: `model.OrcaBinding.LeaseGeneration uint64`
- Invariant: `LeaseGeneration == 0` is readable legacy state; non-zero binding generation may not exceed `WriteLease.Generation`.
- Consumes: existing `advanceOrcaIntentReceipt` dispatch receipt.

- [ ] **Step 1: Write the failing model compatibility tests**

Add exact cases to `internal/core/issueops/model/execution_test.go`:

```go
import (
	"strings"
	"testing"
)

func validOrcaExecutionForTest() Execution {
	return Execution{
		Mode: ExecutionModeOrca,
		Workspace: Workspace{
			SourceRoot: "/repo",
			Root: "/repo.worktrees/resume",
			Branch: "resume",
			BaseHead: strings.Repeat("a", 40),
			Driver: "orca",
			LinkedAt: "2026-07-30T00:00:00Z",
		},
		Lease: WriteLease{Generation: 2, Status: LeaseStatusReleased},
		Orca: &OrcaBinding{
			RuntimeID: "runtime-1",
			RepoID: "repo-1",
			WorktreeID: "worktree-1",
			OwnerHost: "codex",
			OwnerModel: "gpt-5.6-terra",
			TaskID: "task-1",
			DispatchID: "dispatch-1",
		},
	}
}

func TestValidateExecutionAcceptsLegacyOrcaBindingWithoutLeaseGeneration(t *testing.T) {
	execution := validOrcaExecutionForTest()
	execution.Orca.LeaseGeneration = 0
	if err := ValidateExecution(execution); err != nil {
		t.Fatalf("legacy Orca binding must remain readable: %v", err)
	}
}

func TestValidateExecutionRejectsBindingFromFutureLeaseGeneration(t *testing.T) {
	execution := validOrcaExecutionForTest()
	execution.Lease.Generation = 2
	execution.Orca.LeaseGeneration = 3
	if err := ValidateExecution(execution); err == nil ||
		!strings.Contains(err.Error(), "Orca binding lease_generation exceeds the lease generation") {
		t.Fatalf("future binding generation must fail closed: %v", err)
	}
}
```

- [ ] **Step 2: Write the failing prepare binding test**

Add to `internal/core/issueops/execution_owner_context_test.go`:

```go
func TestExecutionOrcaPrepareRecordsBindingLeaseGeneration(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n\n## 검증 명령\n\n```bash\ngo test ./internal/core/issueops -count=1\n```\n"
	_, _, prepared, _ := sealedOrcaCycle(t, issueBody)
	if prepared.Execution.Orca == nil ||
		prepared.Execution.Orca.LeaseGeneration != prepared.Execution.Lease.Generation {
		t.Fatalf("prepare binding generation = %#v lease=%d", prepared.Execution.Orca, prepared.Execution.Lease.Generation)
	}
}
```

- [ ] **Step 3: Run the focused RED**

Run:

```bash
go test ./internal/core/issueops/model ./internal/core/issueops \
  -run 'TestValidateExecution(AcceptsLegacyOrcaBindingWithoutLeaseGeneration|RejectsBindingFromFutureLeaseGeneration)|TestExecutionOrcaPrepareRecordsBindingLeaseGeneration' \
  -count=1
```

Expected: compile failure because `OrcaBinding.LeaseGeneration` does not exist.

- [ ] **Step 4: Add the minimal model field and validation**

In `model.OrcaBinding`:

```go
LeaseGeneration uint64 `json:"lease_generation,omitempty"`
```

In `ValidateExecution`, after validating the Orca binding:

```go
if execution.Orca.LeaseGeneration > execution.Lease.Generation {
	return fmt.Errorf("Orca binding lease_generation exceeds the lease generation")
}
```

In the dispatch branch of `advanceOrcaIntentReceipt`, populate:

```go
LeaseGeneration: expected.Generation,
```

- [ ] **Step 5: Run GREEN and focused regression**

Run:

```bash
go test ./internal/core/issueops/model ./internal/core/issueops \
  -run 'ValidateExecution|ExecutionOrcaPrepare|OwnerPacket' \
  -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 1 atomically**

Stage only the four Task 1 files and commit:

```text
fix(issueops): bind Orca owners to lease generations
```

Lore verification must name the exact focused command from Step 5.

---

### Task 2: Add The Fail-Closed Core Resume Vertical

**Files:**
- Create: `internal/core/issueops/execution_resume.go`
- Create: `internal/core/issueops/execution_resume_test.go`
- Modify: `internal/core/issueops/execution_orca_intent.go`
- Modify: `internal/core/issueops/execution_orca_intent_test.go`
- Modify: `internal/core/issueops/execution_lease.go`
- Modify: `internal/core/issueops/execution_lease_deadend_test.go`

**Interfaces:**
- Produces:

```go
type ExecutionResumeRequest struct {
	ID                 string            `json:"id"`
	ExpectedGeneration uint64            `json:"expected_generation"`
	Actor              model.NativeActor `json:"actor"`
	CWD                string            `json:"cwd"`
	Confirm            bool              `json:"confirm"`
}

type ExecutionResumeDependencies struct {
	Orca      port.ExecutionOrcaProvisioner
	OrcaOwner port.ExecutionOrcaOwnerInspector
	Now       func() time.Time
}

type ExecutionResumeResult struct {
	OK                 bool            `json:"ok"`
	ID                 string          `json:"id"`
	Execution          model.Execution `json:"execution"`
	ClaimTokenPath     string          `json:"claim_token_path"`
	IssueBodySHA256    string          `json:"issue_body_sha256"`
	ContextPacketPath string          `json:"context_packet_path"`
	ContextPacketSHA256 string        `json:"context_packet_sha256"`
	OwnerPromptPath    string          `json:"owner_prompt_path"`
	OwnerPromptSHA256  string          `json:"owner_prompt_sha256"`
	NextCommand        string          `json:"next_command"`
}

func ResumeExecutionWithDependencies(context.Context, string, ExecutionResumeRequest, ExecutionResumeDependencies) (ExecutionResumeResult, error)
```

- Adds durable `externalOrcaIntentPayload.Purpose`, `PriorBinding`, and `ResumeLease`.
- Purpose values are `prepare` and `resume`; missing purpose is accepted only as legacy prepare.
- Consumes: `executionOwnerArtifactPaths`, `readExecutionOwnerArtifact`, `validateExecutionClaimPacket`, `executeOrcaIntentStage`, `executionOwnerInventory`.

- [ ] **Step 1: Write the resume state-matrix tests**

Create `execution_resume_test.go` using `sealedOrcaCycle`, the formal replace preview/reseed flow, `executionOrcaFake`, and `executionOrcaOwnerInspectorFake`.

Required test names:

```go
func TestExecutionResumeRejectsReleasedLeaseBeforeExternalMutation(t *testing.T)
func TestExecutionResumeRejectsPreviousLiveTaskFromAnotherGeneration(t *testing.T)
func TestExecutionResumeRejectsLiveTaskWithoutLiveTerminal(t *testing.T)
func TestExecutionResumeReturnsExistingLiveBindingForTheSameGeneration(t *testing.T)
func TestExecutionResumeCreatesFreshBindingAndPreservesLeaseAudit(t *testing.T)
func TestExecutionResumePromotesLegacyZeroGenerationBinding(t *testing.T)
```

The main GREEN assertion must preserve the exact pre-resume lease:

```go
before := reseeded.Execution.Lease
resumed, err := ResumeExecutionWithDependencies(ctx, stateRoot, request, deps)
if err != nil {
	t.Fatal(err)
}
if resumed.Execution.Lease != before {
	t.Fatalf("resume changed the sealed lease: before=%#v after=%#v", before, resumed.Execution.Lease)
}
if resumed.Execution.Orca.LeaseGeneration != before.Generation ||
	resumed.Execution.Orca.TaskID != "task-resume" ||
	resumed.Execution.Orca.DispatchID != "dispatch-resume" ||
	resumed.Execution.Orca.TerminalPTYID != "pty-resume" {
	t.Fatalf("resume binding = %#v", resumed.Execution.Orca)
}
```

The custom fake must capture every stage and return these literal receipts:

```go
terminal_create -> TerminalPTYID: "pty-resume"
task_create     -> TaskID: "task-resume"
dispatch        -> TaskID: "task-resume", DispatchID: "dispatch-resume"
```

Assert the first observed stage is `port.ExecutionOrcaIntentTerminal`, never `Worktree`.

- [ ] **Step 2: Write the ambiguous-result reconciliation test**

Add:

```go
func TestExecutionResumeAmbiguousDispatchRemainsReconcileable(t *testing.T)
```

Make the dispatch fake return:

```go
&port.OrcaError{Code: "transport", Invoked: true}
```

Assertions:

- resume returns an error containing `requires execution reconcile`;
- persisted `Execution.Pending.Kind == "dispatch"`;
- persisted lease remains claimable and byte-equivalent to the reseeded lease;
- `ReconcileExecutionWithDependencies(... Confirm: true ...)` adopts the single matching dispatch receipt;
- final binding uses `task-resume`, `dispatch-resume`, current lease generation.

- [ ] **Step 3: Run the core RED**

Run:

```bash
go test ./internal/core/issueops \
  -run 'TestExecutionResume' \
  -count=1
```

Expected: compile failure because resume types/functions do not exist.

- [ ] **Step 4: Add purpose-aware external intent state**

Extend `externalOrcaIntentPayload`:

```go
Purpose      string             `json:"purpose,omitempty"`
PriorBinding *model.OrcaBinding `json:"prior_binding,omitempty"`
ResumeLease  *model.WriteLease  `json:"resume_lease,omitempty"`
```

Add:

```go
const (
	orcaIntentPurposePrepare = "prepare"
	orcaIntentPurposeResume  = "resume"
)

func normalizedOrcaIntentPurpose(payload externalOrcaIntentPayload) string {
	if strings.TrimSpace(payload.Purpose) == "" {
		return orcaIntentPurposePrepare
	}
	return strings.TrimSpace(payload.Purpose)
}
```

New prepare payloads set `Purpose: orcaIntentPurposePrepare`. Validation accepts
missing purpose only through `normalizedOrcaIntentPurpose` so an installed new
binary can reconcile an old pending prepare.

For resume payload validation require:

```go
payload.Generation > 0
payload.Stage != port.ExecutionOrcaIntentWorktree
payload.PriorBinding != nil
payload.ResumeLease != nil
payload.ResumeLease.Generation == payload.Generation
payload.ResumeLease.Status == model.LeaseStatusClaimable
payload.ResumeLease.Holder == nil
payload.ResumeLease.ClaimTokenSHA256 == payload.ClaimTokenSHA256
```

- [ ] **Step 5: Make intent CAS purpose-aware**

Refactor `readAndMatchOrcaIntent` into shared identity checks plus:

```go
switch normalizedOrcaIntentPurpose(expected) {
case orcaIntentPurposePrepare:
	require lease generation 1, released status, and nil Orca binding
case orcaIntentPurposeResume:
	require reflect.DeepEqual(current.Execution.Lease, *expected.ResumeLease)
	require reflect.DeepEqual(current.Execution.Orca, expected.PriorBinding)
default:
	return "unsupported Orca intent purpose"
}
```

In the dispatch receipt branch:

```go
if normalizedOrcaIntentPurpose(expected) == orcaIntentPurposeResume {
	if !reflect.DeepEqual(current.Execution.Lease, *expected.ResumeLease) {
		return fmt.Errorf("resume lease changed before dispatch receipt CAS")
	}
} else {
	current.Execution.Lease = model.WriteLease{
		Generation: expected.Generation,
		Status: model.LeaseStatusClaimable,
		ClaimTokenSHA256: expected.ClaimTokenSHA256,
	}
}
```

Then replace `current.Execution.Orca` for both purposes and set
`LeaseGeneration: expected.Generation`.

- [ ] **Step 6: Implement resume artifact validation and intent start**

`readExecutionResumeArtifacts` must:

1. read the generation token and match `lease.ClaimTokenSHA256`;
2. read packet/prompt with `readExecutionOwnerArtifact`;
3. decode the packet and derive its public issue body digest;
4. call `validateExecutionClaimPacket` with derived issue/packet digests;
5. return only paths/digests, never token bytes.

`beginOrcaExecutionResumeIntent` must:

- create a new operation ID;
- build marker with
  `fmt.Sprintf("agent-harness issueops-v1 resume lifecycle=%s generation=%d operation=%s", record.ID, generation, operationID)`;
- copy the snapshots before persistence (`prior := *record.Execution.Orca`,
  `lease := record.Execution.Lease`) so later receipt handling never aliases the
  mutable record;
- persist the exact current lease and binding snapshots;
- derive `Prepared` from the existing workspace/binding;
- start at `ExecutionOrcaIntentTerminal`;
- write payload and `Execution.Pending` in one `persistExecutionTransitionWithMutations`.

- [ ] **Step 7: Implement the public resume service**

`ResumeExecutionWithDependencies` order:

```text
require confirm
RequireIssueOpsMutationAllowed
normalize actor
read current generation
require confirm
require Orca mode, nil pending, claimable lease, canonical worktree cwd
validate sealed token/packet/prompt
InspectOwner for the current binding
same generation + terminal live + task live => idempotent result
any task live without a valid same-generation live terminal => fail closed
begin resume intent
execute exactly terminal/task/dispatch stages
return claim next command with exact generation/path/digests
```

The claim next command must use shell-quoted literal values and omit token
contents:

```text
agent-harness issueops execution claim --id 'io-id' --generation 3 --claim-token-file '/abs/lease-3.token' --issue-body-sha256 64hex --context-packet-sha256 64hex
```

- [ ] **Step 8: Route replacement and writer-absent next commands through resume**

For Orca `replace --finalize|--reseed` results that are claimable, set:

```go
result.NextCommand = executionResumeCommand(record.ID, lease.Generation)
```

In `executionWriterAbsentNextCommand`, an Orca claimable lease whose binding has
`LeaseGeneration != lease.Generation` returns the same resume command instead
of a claim command. Direct mode behavior stays unchanged.

- [ ] **Step 9: Run core GREEN, race, and regressions**

Run:

```bash
go test ./internal/core/issueops \
  -run 'Resume|Reconcile|Reseed|Finalize|ExecutionWriterAbsent|OrcaIntent' \
  -count=1
go test -race ./internal/core/issueops \
  -run 'Resume|Reconcile' \
  -count=1
```

Expected: PASS.

- [ ] **Step 10: Commit Task 2 atomically**

Stage Task 2 production files and their direct tests. Commit:

```text
feat(issueops): resume Orca owners after reseed
```

---

### Task 3: Expose Resume Through The Shared CLI/MCP Action

**Files:**
- Modify: `internal/core/issueops/execution_api.go`
- Modify: `internal/core/issueops/execution_issue_snapshot_test.go`
- Modify: `cmd/harness/issueopscli/executioncmd/execution.go`
- Create: `cmd/harness/issueopscli/executioncmd/execution_resume_test.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops_execution_test.go`
- Modify: `internal/adapter/mcp/issueops_catalog.go`
- Modify: `internal/adapter/mcp/issueops_catalog_test.go`

**Interfaces:**
- Adds `ExecutionActionResume = "resume"`.
- Reuses `ExecutionActionRequest.ExpectedGeneration`, `Actor`, `CWD`, `Confirm`.
- Reuses `ExecutionActionDependencies.Orca` and `OrcaOwner`.
- Extends the existing `issueops_execution` MCP action enum; no new tool.

- [ ] **Step 1: Write failing API routing and snapshot-reader tests**

Add a core test that calls `ExecuteExecution` with the Task 2 sealed lifecycle
and fake Orca dependencies:

```go
ExecutionActionRequest{
	Action: ExecutionActionResume,
	ID: "io-resume",
	ExpectedGeneration: 3,
	Actor: actor,
	CWD: worktree,
	Confirm: true,
}
```

Assert the returned binding has generation 3 and the fake sees the
terminal/task/dispatch stages. Do not add a test-only production hook.

In `execution_issue_snapshot_test.go`, assert `ExecutionActionResume` does not
require or report an issue snapshot reader.

- [ ] **Step 2: Write failing CLI and MCP mapping tests**

CLI tests call `Run` for the exact flag surface:

```text
issueops execution resume --id io-resume --expected-generation 3 --host codex --session-id session-resume --session-pid 42 --session-started-at 2026-07-30T00:00:00Z --session-executable /usr/local/bin/codex --cwd /repo.worktrees/resume --confirm --json
```

Assert help/usage includes the command and that an added
`--issue-snapshot-file /tmp/issue.json` fails with
`flag provided but not defined`. The real Task 2 fixture through
`ExecuteExecution` covers action/generation/confirm mapping; do not add a
production callback solely to capture the CLI request.

MCP mapping test uses:

```go
map[string]any{
	"action": "resume",
	"id": "io-resume",
	"expected_generation": float64(3),
	"host": "codex",
	"session_id": "session-resume",
	"session_pid": float64(42),
	"session_started_at": "2026-07-30T00:00:00Z",
	"session_executable": "/usr/local/bin/codex",
	"cwd": "/repo.worktrees/resume",
	"confirm": true,
}
```

Assert the same DTO fields.

- [ ] **Step 3: Write the failing single-tool catalog test**

Extend `TestIssueOpsAdvertisesOnlyExecutionActionTool`:

```go
if len(tools) != 1 || tools[0].Name != "issueops_execution" {
	t.Fatalf("IssueOps MCP tools = %#v", tools)
}
actions := tools[0].InputSchema["properties"].(map[string]any)["action"].(map[string]any)["enum"].([]string)
if !slices.Contains(actions, "resume") {
	t.Fatalf("execution actions = %#v", actions)
}
```

- [ ] **Step 4: Run CLI/MCP RED**

Run:

```bash
go test ./cmd/harness/issueopscli/executioncmd ./cmd/harness/mcpcli ./internal/adapter/mcp \
  -run 'Resume|IssueOpsAdvertisesOnlyExecutionActionTool|ExecutionActionRequest' \
  -count=1
```

Expected: missing resume action/subcommand assertions fail.

- [ ] **Step 5: Implement shared action routing**

Add to `execution_api.go`:

```go
const ExecutionActionResume = "resume"
```

Route:

```go
case ExecutionActionResume:
	return ResumeExecutionWithDependencies(ctx, stateRoot, ExecutionResumeRequest{
		ID: req.ID,
		ExpectedGeneration: req.ExpectedGeneration,
		Actor: req.Actor,
		CWD: req.CWD,
		Confirm: req.Confirm,
	}, ExecutionResumeDependencies{
		Orca: deps.Orca,
		OrcaOwner: deps.OrcaOwner,
	})
```

- [ ] **Step 6: Implement CLI resume**

Add usage and Run switch entry. `runResume` has exactly these flags:

```go
id := fs.String("id", "", "IssueOps id")
generation := fs.Uint64("expected-generation", 0, "expected lease generation")
actor := addActorFlags(fs)
confirm := fs.Bool("confirm", false, "confirm owner resume")
jsonOut := fs.Bool("json", false, "print JSON")
```

It maps to `ExecutionActionRequest{Action: ExecutionActionResume, ...}` and does
not accept `--issue-snapshot-file`.

- [ ] **Step 7: Extend the single MCP action enum**

Change only the existing tool:

```go
"action": map[string]any{
	"type": "string",
	"enum": []string{"prepare", "status", "claim", "release", "replace", "resume", "reconcile", "complete"},
},
```

Update its description to include resume. Do not add a dispatch entry or new
tool name.

- [ ] **Step 8: Run CLI/MCP GREEN**

Run:

```bash
go test ./cmd/harness/issueopscli/executioncmd ./cmd/harness/mcpcli ./internal/adapter/mcp \
  -run 'Resume|IssueOpsAdvertisesOnlyExecutionActionTool|ExecutionActionRequest|Usage' \
  -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit Task 3 atomically**

Commit production mappings and direct tests:

```text
feat(cli): expose IssueOps owner resume
```

---

### Task 4: Admit Only The Exact Resume Control Plane And Pin Contracts

**Files:**
- Modify: `internal/core/commandparse/issueops.go`
- Modify: `internal/core/commandparse/issueops_test.go`
- Modify: `internal/core/lifecycle/lifecycle_execution_guard.go`
- Modify: `internal/core/lifecycle/lifecycle_execution_matrix_test.go`
- Modify: `cmd/harness/contractcli/contract.go`
- Modify: `cmd/harness/testdata/usage.golden.txt`
- Modify: `cmd/harness/testdata/mcp_tools.golden.json`
- Modify: `cmd/harness/testdata/response_contracts.golden.json`
- Modify: `skills/issueops/references/execution.md`
- Modify: `docs/superpowers/specs/2026-07-30-issueops-orca-owner-resume-design.md`

**Interfaces:**
- Exact shell path: `execution resume`.
- Values: `--id`, `--expected-generation`, actor receipt flags, `--cwd`.
- Booleans: `--confirm`, `--json`.
- Response contract key: `issueops_execution_resume`.
- MCP tool count remains unchanged.

- [ ] **Step 1: Write parser RED cases**

Add exact accepted case and rejected near misses:

```text
accepted: execution resume + id + generation + full actor + cwd + confirm + json
rejected: unknown --issue-snapshot-file
rejected: duplicate --expected-generation
rejected: missing value
rejected: active command substitution
```

`IssueOpsCommandSpec("execution resume")` must expose only:

```go
values := []string{
	"--id", "--expected-generation", "--host", "--session-id", "--agent-id",
	"--session-pid", "--session-started-at", "--session-executable", "--cwd",
}
booleans := []string{"--confirm", "--json"}
```

- [ ] **Step 2: Write active-authority guard RED cases**

In the execution matrix, use a lifecycle with active mutation authority.

Allow:

```text
agent-harness issueops execution resume --id io-1 --expected-generation 3 --host codex --session-id session-1 --session-pid 42 --session-started-at 2026-07-30T00:00:00Z --session-executable /usr/local/bin/codex --cwd /repo.worktrees/resume --confirm --json
```

Block:

- the same command without `--confirm`;
- missing generation;
- `--issue-snapshot-file`;
- `--expected-generation $(date +%s)`;
- foreign/empty ID.

Expected deny code for unclassified near misses remains `unsafe_mutation`.

- [ ] **Step 3: Run guard RED**

Run:

```bash
go test ./internal/core/commandparse ./internal/core/lifecycle \
  -run 'Resume|ExecutionAdmitsExactOrcaOwnerControlPlaneCommands|ExecutionKeepsNearMissOrcaOwnerControlPlaneCommandsBlocked' \
  -count=1
```

Expected: exact resume is blocked before implementation.

- [ ] **Step 4: Register exact resume parsing and mutation admission**

Add command spec:

```go
case "execution resume":
	return v(
		"--id", "--expected-generation", "--host", "--session-id", "--agent-id",
		"--session-pid", "--session-started-at", "--session-executable", "--cwd",
	), b("--confirm", "--json"), r, true
```

In `exactIssueOpsOwnerMutation`, give resume its own strict branch before the
generic execution mutation list:

```go
case "execution resume":
	id, idOK := oneFlag(flags, "--id")
	generation, generationOK := oneFlag(flags, "--expected-generation")
	_, confirm := flags["--confirm"]
	return idOK && strings.TrimSpace(id) != "" &&
		generationOK && strings.TrimSpace(generation) != "" && confirm
```

Do not classify resume as a read-only observation.

- [ ] **Step 5: Add the CLI response contract**

`issueops_execution_resume` exact fields:

```go
{
	"ok", "id", "execution", "claim_token_path", "issue_body_sha256",
	"context_packet_path", "context_packet_sha256",
	"owner_prompt_path", "owner_prompt_sha256", "next_command",
}
```

- [ ] **Step 6: Update user-facing execution docs**

In `skills/issueops/references/execution.md`, replace the holderless recovery
sequence with:

```text
replace --preview
replace --reseed --confirm
execution resume --expected-generation N --confirm
claim using resume result token path and issue/packet digests
```

State that resume creates a fresh Orca owner in the existing canonical
worktree, records binding `lease_generation`, and leaves ambiguous mutations to
`execution reconcile`.

- [ ] **Step 7: Regenerate only intentional goldens**

Run:

```bash
go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp \
  -run Golden -update -count=1
```

Inspect every changed golden. Required facts:

- usage contains `execution resume` exactly once;
- MCP tool count is unchanged;
- `issueops_execution` action enum includes `resume` exactly once;
- response contract adds only `issueops_execution_resume`.

- [ ] **Step 8: Run guard/contract GREEN**

Run:

```bash
go test ./internal/core/commandparse ./internal/core/lifecycle \
  -run 'Resume|ExecutionAdmitsExactOrcaOwnerControlPlaneCommands|ExecutionKeepsNearMissOrcaOwnerControlPlaneCommandsBlocked' \
  -count=1
go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp \
  -run Golden -count=1
python3 scripts/validate-skill.py skills/issueops
```

Expected: PASS.

- [ ] **Step 9: Commit Task 4 atomically**

Commit exact guard, public contracts, generated goldens, and docs together:

```text
fix(issueops): admit generation-bound owner resume
```

---

### Task 5: Focused Verification, Publication, Install Refresh, And #197 Dogfood

**Files:**
- Verify all files from Tasks 1-4.
- Do not edit #197 worktree until the installed resume control-plane passes.

**Interfaces:**
- Installed CLI: `agent-harness issueops execution resume`.
- Dogfood lifecycle: `io-6e932f2e6c54`.
- Canonical worktree: `/Users/sample/workspace/agent-harness.worktrees/197-issueops-lease-claim-vertical`.
- Parent worktree: `/Users/sample/workspace/agent-harness.worktrees/117-hexagonal-architecture-migration`.
- Current released generation before dogfood: 2.
- Owner: `codex / gpt-5.6-terra / xhigh`.

- [ ] **Step 1: Run the final all-or-nothing focused verification**

Run this complete sequence from the first command whenever any step fails and a
fix is made:

```bash
go test ./internal/core/issueops/model ./internal/core/issueops -count=1
go test ./internal/adapter/orca -run 'Intent|Owner' -count=1
go test ./cmd/harness/issueopscli/executioncmd ./cmd/harness/mcpcli ./internal/adapter/mcp -count=1
go test ./internal/core/commandparse ./internal/core/lifecycle -run 'Resume|Execution' -count=1
go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1
go test -race ./internal/core/issueops -run 'Resume|Reconcile' -count=1
go vet ./internal/core/issueops/model ./internal/core/issueops ./internal/adapter/orca ./cmd/harness/issueopscli/executioncmd ./cmd/harness/mcpcli ./internal/adapter/mcp ./internal/core/commandparse ./internal/core/lifecycle
python3 scripts/validate-skill.py skills/issueops
go build -o /tmp/agent-harness-owner-resume ./cmd/harness
```

Do not substitute a local full suite.

- [ ] **Step 2: Run atomic commit preflight and inspect commit boundaries**

Run:

```bash
python3 skills/atomic-commit-push/scripts/git_preflight.py /Users/sample/workspace/agent-harness
python3 skills/atomic-commit-push/scripts/api_doc_gate.py /Users/sample/workspace/agent-harness
git status --short --branch
git log --oneline origin/main..HEAD
git diff origin/main...HEAD --stat
```

There must be no secret-like path and no unrelated working-tree change.

- [ ] **Step 3: Push main and prove remote equality**

Run:

```bash
git push origin main
git rev-parse HEAD
git ls-remote --heads origin main
```

The two full SHAs must match.

- [ ] **Step 4: Refresh installed hosts and daemon/MCP**

Run:

```bash
ah update --json
agent-harness daemon status --json
agent-harness contract check --json
codex mcp get agent_harness
claude mcp list
```

Verify:

- Codex and Claude updates succeed;
- daemon is healthy;
- daemon build SHA matches installed binary SHA;
- contract check has no warnings;
- OpenWiki automatic update did not run.

- [ ] **Step 5: Reseed #197 from released generation 2**

From the canonical #197 worktree, obtain exact actor receipt with
`execution whoami`, then run the preview command with the literal actor receipt:

```text
execution replace --preview --expected-generation 2
```

Copy the 64-hex `.inventory_fingerprint` printed by that preview directly into
`--inventory-fingerprint`, and run `execution replace --reseed` with
`--expected-generation 2`, reason `#197 구현 재개를 위한 owner 재연결`, the
same literal actor receipt, and `--confirm --json`. Do not use a shell variable,
substitution, or pipeline.

Expected durable result:

```text
lease.generation = 3
lease.status = claimable
next_command = execution resume for generation 3
```

- [ ] **Step 6: Resume the #197 owner with the installed command**

Run the exact `.next_command` returned by reseed after appending the first
literal `claim_actor_flags` entry from `execution whoami`, canonical
`--cwd /Users/sample/workspace/agent-harness.worktrees/197-issueops-lease-claim-vertical`,
and `--json`. Do not use shell expansion.

Verify:

- `execution.orca.lease_generation == 3`;
- task/dispatch IDs differ from historical `task_e8a1404f4a37` /
  `ctx_fa868af21356`;
- terminal PTY differs from the historical `@@66ce05b0`;
- owner host/model/effort is `codex / gpt-5.6-terra / xhigh`;
- `orca orchestration task-list` shows the new task live;
- `orca orchestration dispatch-show` binds it to the new terminal.

- [ ] **Step 7: Supervise the owner claim and implementation phase entry**

The new owner uses the resume result’s exact `.next_command` to claim generation
3, verifies pwd/branch/HEAD, then executes the exact `phase_implement` command
sealed in generation 3 `context.json`, replacing only its documented native
session receipt values with the owner’s literal `execution whoami` output.

Before #197 source edits, prove:

```text
lease.status = active
lease.generation = 3
phase = implement
holder session/process = new Terra/xhigh terminal
worktree branch = 197-issueops-lease-claim-vertical
HEAD = ba6f7eab82dfe51f8ebb0eaca5e3979cc904d0a1
```

At that point resume control-plane dogfood is complete and the recorded #197
implementation plan resumes under the supervised Orca owner.
