# IssueOps Orca Run Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Orca 1.4.162에서 각 IssueOps execution generation이 고유한 durable Run을 만들고 모든 task/dispatch 경로가 봉인된 Run ID를 사용하게 한다.

**Architecture:** `run_create`와 `run_bind`를 기존 external-intent state machine의 독립 stage로 추가하고, 생성·bind된 Run ID를 task·dispatch receipt와 최종 `OrcaBinding`까지 전달한다. Orca adapter는 현재 process의 exact `ORCA_TERMINAL_HANDLE`을 fail-closed gate로 검증하되 현재 coordinator mutation에서는 `--from`을 생략해 Orca의 process authority를 보존하며, Run ID가 없는 기존 binding은 모든 명시적 Run의 read-only task inventory에서 exact task가 하나일 때만 해석한다.

**Tech Stack:** Go 1.26.3, 표준 `context`/`encoding/json`/`sort`/`strings`, Orca 1.4.162 CLI JSON 계약, IssueOps SQLite external-intent CAS

## Global Constraints

- 핵심 동작은 Go core/port에 두고 Codex/Claude host adapter에는 복제하지 않는다.
- 신규 IssueOps 실행은 `worktree_create → terminal_create → run_create → run_bind → task_create → dispatch` 순서를 사용한다.
- Run 생성과 task 생성은 서로 다른 durable external intent stage여야 한다.
- Run 생성과 coordinator bind도 서로 다른 durable external intent stage여야 한다.
- 신규 task/dispatch mutation은 non-empty sealed Run ID가 없으면 runner 호출 전에 fail-closed한다.
- coordinator mutation은 concrete `ORCA_TERMINAL_HANDLE`이 없으면 focus/cwd fallback 없이 fail-closed한다.
- 기존 Run ID 없는 binding은 exact task가 하나의 Run에서만 발견될 때만 읽기/종결한다.
- 주석은 한글로 작성한다.
- 전체 `go test ./...`와 전체 race는 로컬에서 실행하지 않는다. 각 Task의 focused 명령과 마지막 build만 실행한다.
- OpenWiki 자동 update는 실행하지 않는다.
- resource-creating external mutation의 unknown result는 재호출하지 않고
  0/1/N inventory로 reconcile한다.
- `run_bind`는 current Run readback 뒤 exact target으로 수렴하는 bounded
  `run-use` 재bind만 기존 intent attempt 상한 안에서 허용한다.
- `gpt-5.6-terra/xhigh` owner와 명시적 parent-worktree 계약은 #194 dogfood에서 유지한다.

---

### Task 1: Run identity port와 durable model

**Files:**
- Modify: `internal/port/orca.go`
- Modify: `internal/port/execution_workspace.go`
- Modify: `internal/core/issueops/model/execution.go`
- Modify: `internal/core/issueops/model/execution_test.go`

**Interfaces:**
- Produces: `port.OrcaRun`, `port.OrcaCreateRunRequest`
- Produces: `RunID string` on task/dispatch requests, intent requests/receipts, owner inventory, and final Orca receipts
- Produces: transient stage receipt `RunBound bool` on intent requests/receipts/payloads
- Produces: optional-on-read `model.OrcaBinding.RunID`
- Consumes: existing runtime/worktree/task/dispatch identity fields without changing their JSON names

- [ ] **Step 1: 기존 binding 역호환성과 신규 Run identity 테스트를 작성한다**

`internal/core/issueops/model/execution_test.go`에 다음 계약을 추가한다.

```go
func TestValidateExecutionAcceptsLegacyOrcaBindingWithoutRunID(t *testing.T) {
	execution := validOrcaExecutionForTest()
	execution.Orca.RunID = ""
	if err := ValidateExecution(execution); err != nil {
		t.Fatalf("legacy Orca binding must remain readable: %v", err)
	}
}

func TestValidateExecutionAcceptsSealedOrcaRunID(t *testing.T) {
	execution := validOrcaExecutionForTest()
	execution.Orca.RunID = "run_issueops_1"
	if err := ValidateExecution(execution); err != nil {
		t.Fatalf("sealed Orca run id must be valid: %v", err)
	}
}
```

- [ ] **Step 2: model 테스트가 새 필드 부재로 실패하는지 확인한다**

Run:

```bash
go test ./internal/core/issueops/model -run 'TestValidateExecution(AcceptsLegacyOrcaBindingWithoutRunID|AcceptsSealedOrcaRunID)' -count=1
```

Expected: `OrcaBinding.RunID undefined` compile failure.

- [ ] **Step 3: port와 model에 최소 Run identity를 추가한다**

`internal/port/orca.go`의 새 타입과 request shape:

```go
type OrcaRun struct {
	RuntimeID string `json:"-"`
	ID        string `json:"id"`
	Objective string `json:"objective"`
	Legacy    bool   `json:"legacy,omitempty"`
}

type OrcaCreateRunRequest struct {
	Objective string `json:"objective"`
}

type OrcaCreateTaskRequest struct {
	RunID       string `json:"run_id"`
	Spec        string `json:"spec"`
	Title       string `json:"title"`
	DisplayName string `json:"display_name"`
}

type OrcaDispatchRequest struct {
	RunID          string `json:"run_id"`
	TaskID         string `json:"task_id"`
	ToHandle       string `json:"to_handle"`
	FromHandle     string `json:"from_handle,omitempty"`
	Inject         bool   `json:"inject"`
	ReturnPreamble bool   `json:"return_preamble"`
}
```

`OrcaTask`에도 `RunID string`을 추가한다. `internal/port/execution_workspace.go`의
다음 DTO에 아래 field를 추가한다.

```go
RunID string `json:"run_id,omitempty"`
```

- `ExecutionOrcaReceipt`
- `ExecutionOrcaIntentRequest`
- `ExecutionOrcaIntentReceipt`
- `ExecutionOrcaOwnerInventoryRequest`

`ExecutionOrcaIntentRequest`와 `ExecutionOrcaIntentReceipt`에는 다음 field도
추가한다.

```go
RunBound bool `json:"run_bound,omitempty"`
```

`port.OrcaClient` interface에는 `ListRuns`, `CreateRun`, `CurrentRun`,
`UseRun`과 Run-scoped `UpdateTask` signature를 반영한다.

새 stage 상수는 exact literal을 사용한다.

```go
ExecutionOrcaIntentRun ExecutionOrcaIntentStage = "run_create"
ExecutionOrcaIntentRunBind ExecutionOrcaIntentStage = "run_bind"
```

`model.OrcaBinding`에는 다음 필드를 추가하되 legacy validation에서는 필수로
만들지 않는다.

```go
RunID string `json:"run_id,omitempty"`
```

- [ ] **Step 4: model focused 테스트를 통과시킨다**

Run:

```bash
go test ./internal/core/issueops/model -run 'TestValidateExecution(AcceptsLegacyOrcaBindingWithoutRunID|AcceptsSealedOrcaRunID|RejectsBindingFromFutureLeaseGeneration)' -count=1
```

Expected: PASS.

- [ ] **Step 5: Task 1을 커밋한다**

```bash
git add internal/port/orca.go internal/port/execution_workspace.go internal/core/issueops/model/execution.go internal/core/issueops/model/execution_test.go
git commit -m "feat(issueops): model Orca Run identity" \
  -m "Context: Orca 1.4.162 requires explicit Run namespaces for orchestration tasks." \
  -m "Decision: carry an optional legacy-compatible Run ID through port DTOs and durable Orca bindings." \
  -m "Consequences: later stages can seal exact Run authority without making historical records unreadable."
```

### Task 2: Orca Run client와 명시적 task scope

**Files:**
- Create: `internal/adapter/orca/testdata/run_list.json`
- Create: `internal/adapter/orca/testdata/run_create.json`
- Create: `internal/adapter/orca/testdata/run_current.json`
- Create: `internal/adapter/orca/testdata/run_use.json`
- Modify: `internal/adapter/orca/client.go`
- Modify: `internal/adapter/orca/client_test.go`
- Modify: `internal/adapter/orca/client_settle_task_test.go`

**Interfaces:**
- Consumes: `port.OrcaRun`, `port.OrcaCreateRunRequest`, request `RunID` fields from Task 1
- Produces: `Client.ListRuns`, `Client.CreateRun`, `Client.CurrentRun`, `Client.UseRun`, run-scoped task inventory, `Client.SettleTask(ctx, runID, taskID)`
- Preserves: `ListAllTasks` and `ListDispatchedTasks` as complete cross-Run inventory for operational callers

- [ ] **Step 1: Run JSON fixtures를 실제 Orca 1.4.162 shape로 작성한다**

`run_list.json`:

```json
{
  "id": "request-run-list",
  "ok": true,
  "result": {
    "runs": [
      {
        "id": "run_issueops_1",
        "objective": "issueops-v1 lifecycle=io-test generation=1 intent=op-test",
        "legacy": 0
      }
    ]
  },
  "_meta": {"runtimeId": "runtime-1"}
}
```

`run_create.json`:

```json
{
  "id": "request-run-create",
  "ok": true,
  "result": {
    "run": {
      "id": "run_issueops_1",
      "objective": "issueops-v1 lifecycle=io-test generation=1 intent=op-test",
      "legacy": 0
    }
  },
  "_meta": {"runtimeId": "runtime-1"}
}
```

`run_current.json`과 `run_use.json`도 같은 `result.run` shape와
`_meta.runtimeId`를 사용한다. `run-current`의 unbound fixture는
`{"run": null}`을 사용한다.

- [ ] **Step 2: Run CRUD와 exact argv 실패 테스트를 작성한다**

`client_test.go`에 다음 경우를 독립 test로 추가한다.

- `ListRuns`가 `orca orchestration run-list --json`을 호출하고 runtime/ID/objective를 투영
- `CreateRun`이
  `run-create --objective <exact> --json`을 호출
- `CurrentRun`이
  `run-current --json`을 호출하고 null을 보존
- `UseRun`이
  `run-use --id run_issueops_1 --json`을 호출
- create 응답 objective가 요청과 다르면 `run_identity_mismatch`
- 빈 Run ID/objective 응답은 `run_identity_invalid`
- `CreateTask`, `Dispatch`, `UpdateTask`가 exact `--run run_issueops_1`을
  포함하고 현재 terminal을 대리하는 `--from`은 생략
- worker-done `send`는 sealed Run ID와 worker의 exact FromHandle을 사용
- Run ID가 비면 runner call count 0과 `run_required`
- `ORCA_TERMINAL_HANDLE`이 empty/invalid면 coordinator mutation call count 0과
  `coordinator_identity_unavailable`
- `ListAllTasks`가 각 Run마다 `task-list --run <id> --brief --json`을 호출
- task row의 `run_id`가 조회 Run과 다르면 identity mismatch
- 같은 `run_id + task_id` 중복과 count 누락을 거부

`client_settle_task_test.go`의 호출은 다음처럼 변경한다.

```go
if err := NewClient(runner).SettleTask(context.Background(), "run_issueops_1", "task-130"); err != nil {
	t.Fatal(err)
}
```

- [ ] **Step 3: 새 client 테스트가 구현 부재와 unscoped argv로 실패하는지 확인한다**

Run:

```bash
go test ./internal/adapter/orca -run 'TestClient(ListRuns|CreateRun|CurrentRun|UseRun|CreateTask.*Run|Dispatch.*Run|UpdateTask.*Run|WorkerDone.*Run|ListAllTasks.*Run|SettleTask|CoordinatorIdentity)' -count=1
```

Expected: missing methods/fields 또는 기존 unscoped argv mismatch로 FAIL.

- [ ] **Step 4: Run payload와 검증 helper를 구현한다**

`client.go`에 private JSON payload와 다음 public methods를 추가한다.

```go
func (c *Client) ListRuns(ctx context.Context) ([]port.OrcaRun, error)
func (c *Client) CreateRun(ctx context.Context, req port.OrcaCreateRunRequest) (port.OrcaRun, error)
func (c *Client) CurrentRun(ctx context.Context) (*port.OrcaRun, error)
func (c *Client) UseRun(ctx context.Context, runID string) (port.OrcaRun, error)
```

`ListRuns`는 ID를 기준으로 정렬해 cross-Run inventory 순서를 고정한다.
각 row의 ID/objective가 비거나 ID가 중복되면 `run_inventory_incomplete`로
거부한다. `CreateRun`은 objective를 trim한 값이 아니라 요청의 canonical exact
문자열과 응답을 비교하고, mismatch는 `Invoked:true`로 반환한다.

- [ ] **Step 5: coordinator identity와 explicit bind를 구현한다**

```go
func currentCoordinatorHandle() (string, error) {
	handle := strings.TrimSpace(os.Getenv("ORCA_TERMINAL_HANDLE"))
	if !concreteTerminalHandlePattern.MatchString(handle) {
		return "", &port.OrcaError{
			Code:    "coordinator_identity_unavailable",
			Invoked: false,
		}
	}
	return handle, nil
}
```

`CreateRun`, `CurrentRun`, `UseRun`, `CreateTask`, `UpdateTask`, `Dispatch`는
이 helper로 현재 Orca terminal 안에서 호출됐는지 검증하되 `--from`은 전달하지
않는다. Orca가 호출 process의 terminal authority를 직접 인증해야 하며,
`--from`은 다른 terminal을 명시적으로 대리할 때만 쓴다. `CurrentRun`은
`result.run:null`을 오류가 아닌 unbound observation으로 반환한다.
`UseRun`은 ordinary Run에만 exact ID로 bind하며 `--takeover-legacy`를 절대
추가하지 않는다.

- [ ] **Step 6: 모든 task mutation과 inventory를 Run-scoped로 바꾼다**

private helper shape:

```go
func (c *Client) listTasksInventory(
	ctx context.Context,
	runID string,
	flags ...string,
) (executionTaskInventory, error)
```

helper는 항상 argv 끝의 `--json` 앞에 `--run <runID>`를 넣고, 각 task row의
`run_id`가 요청 Run과 같아야 한다. `ListAllTasks`와 `ListDispatchedTasks`는
`ListRuns` 결과를 순회해 모든 row를 합치고 `RunID + "\x00" + ID`로 중복을
검사한다.

mutation signature:

```go
func (c *Client) UpdateTask(ctx context.Context, runID, id, status, result string) error
func (c *Client) SettleTask(ctx context.Context, runID, id string) error
```

`CreateTask`, `Dispatch`, `UpdateTask`, `SendWorkerDone`은 non-empty Run ID를
runner 호출 전에 검증하고 exact `--run`을 전달한다. coordinator mutation은
위 helper를 fail-closed gate로만 사용하고 현재 process RPC에서는 `--from`을
생략한다. worker-done은 request의 exact worker FromHandle을 사용한다. empty
Run ID legacy settle은 unique read-only resolver로
Run을 찾더라도 current caller가 그 Run의 coordinator가 아니면 `task-update`
오류를 그대로 반환하며 implicit `run-use`로 authority를 빼앗지 않는다.

- [ ] **Step 7: Probe를 Run readiness로 바꾼다**

capability matrix에 다음을 추가한다.

- `run-create`: `--objective`, `--from`, `--json`
- `run-list`: `--json`
- `run-current`: `--from`, `--json`
- `run-use`: `--id`, `--from`, `--json`
- task create/list/update, dispatch, send: `--run`

Probe 마지막 read는 `ListTasks`가 아니라 `CurrentRun`과 `ListRuns`를
호출한다. 전역 `task-list --ready --json` response fixture는 probe setup에서
제거한다. preview는 Run을 생성하거나 bind하지 않는다.

- [ ] **Step 8: adapter focused 테스트를 통과시킨다**

Run:

```bash
go test ./internal/adapter/orca -run 'Test(Client|Probe).*(Run|Bind|Coordinator|Task|Dispatch|WorkerDone|OrchestrationReadiness|Capability)' -count=1
go test ./internal/adapter/orca -run 'TestClient(SettleTask|ListAllTasks|ListDispatchedTasks|ListFailedTasks)' -count=1
```

Expected: PASS.

- [ ] **Step 9: Task 2를 커밋한다**

```bash
git add internal/adapter/orca/client.go internal/adapter/orca/client_test.go internal/adapter/orca/client_settle_task_test.go internal/adapter/orca/testdata/run_list.json internal/adapter/orca/testdata/run_create.json internal/adapter/orca/testdata/run_current.json internal/adapter/orca/testdata/run_use.json
git commit -m "feat(orca): scope orchestration to explicit Runs" \
  -m "Context: unscoped task commands bind to Orca's retained legacy coordinator and fail as legacy_read_only." \
  -m "Decision: probe Run capabilities, bind an explicit coordinator terminal, and require Run IDs on every task mutation." \
  -m "Consequences: operational inventory spans explicit Runs while new mutations cannot depend on terminal-local state."
```

### Task 3: Execution adapter의 Run external intent

**Files:**
- Modify: `internal/adapter/orca/execution.go`
- Modify: `internal/adapter/orca/execution_test.go`
- Modify: `internal/adapter/orca/execution_launch_timing_fixtures_test.go`
- Modify: `internal/adapter/orca/execution_dispatch_vocabulary_test.go`

**Interfaces:**
- Consumes: Task 2 `ListRuns`, `CreateRun`, `CurrentRun`, `UseRun`, run-scoped task/dispatch calls
- Produces: `InspectIntent`와 `InvokeIntent`의 `run_create`, `run_bind` stages
- Produces: exact Run-scoped owner inspection

- [ ] **Step 1: fake client에 Run methods와 Run별 inventory를 추가한다**

`executionFake`와 timing fake에 다음 fields/methods를 둔다.

```go
runRequest  port.OrcaCreateRunRequest
runs        []port.OrcaRun
tasksByRun  map[string][]port.OrcaTask

func (f *executionFake) ListRuns(context.Context) ([]port.OrcaRun, error)
func (f *executionFake) CreateRun(_ context.Context, req port.OrcaCreateRunRequest) (port.OrcaRun, error)
func (f *executionFake) CurrentRun(context.Context) (*port.OrcaRun, error)
func (f *executionFake) UseRun(_ context.Context, runID string) (port.OrcaRun, error)
```

- [ ] **Step 2: Run create/bind stage와 run-scoped task/dispatch 실패 테스트를 작성한다**

다음을 검증한다.

- `InspectIntent(run_create)`는 exact objective 후보만 반환
- 0개는 `AuthoritativeZero:true`, 2개는 후보 2개를 보존
- `InvokeIntent(run_create)`는 exact marker objective로 `CreateRun` 호출
- `InspectIntent(run_bind)`는 current Run이 request Run과 같을 때만
  `{RunID, RunBound:true}` 후보 반환
- `InspectIntent(run_bind)`의 unbound/different Run은 authoritative zero
- `InvokeIntent(run_bind)`는 current Run이 target이면 mutation 없이 receipt
- current Run이 target이 아니면 exact Run ID로 `UseRun` 호출
- bind 응답 runtime/Run ID/objective mismatch는 invoked identity 오류
- 응답 runtime/objective mismatch는 invoked identity 오류
- task inspect는 request Run 하나만 조회
- task create와 dispatch request에 같은 Run ID 전달
- owner inspection은 non-empty request Run ID 하나만 조회
- legacy empty Run ID는 모든 Run에서 exact task 0/1/N resolver 사용
- direct compatibility `LaunchOwner`도 create Run 후 같은 Run으로 task/dispatch

- [ ] **Step 3: 새 execution 테스트의 실패를 확인한다**

Run:

```bash
go test ./internal/adapter/orca -run 'TestExecution.*(Run|Intent|OwnerInventory|Launch)' -count=1
```

Expected: `run_create`/`run_bind` unsupported 또는 RunID 누락으로 FAIL.

- [ ] **Step 4: execution client interface와 stage switch를 구현한다**

`executionClient`:

```go
ListRuns(context.Context) ([]port.OrcaRun, error)
CreateRun(context.Context, port.OrcaCreateRunRequest) (port.OrcaRun, error)
CurrentRun(context.Context) (*port.OrcaRun, error)
UseRun(context.Context, string) (port.OrcaRun, error)
CreateTask(context.Context, port.OrcaCreateTaskRequest) (port.OrcaTask, error)
Dispatch(context.Context, port.OrcaDispatchRequest) (port.OrcaDispatch, error)
```

`InspectIntent`의 Run stage는 `req.Marker == run.Objective`인 row만 receipt
`{RunID: run.ID}`로 반환한다. Run bind stage는 `CurrentRun`이 target과 같을
때만 `{RunID: run.ID, RunBound:true}`를 반환한다. Task stage는
`req.RunID`의 task inventory만 읽는다. `InvokeIntent`는 Run create stage에서
`CreateRun`, Run bind stage에서 `CurrentRun` read-first 후 필요한 경우에만
`UseRun`, Task/Dispatch stage에서 `req.RunID`을 전달한다.

- [ ] **Step 5: owner inspection의 exact/legacy resolver를 구현한다**

```go
func (p *ExecutionProvisioner) resolveTaskRun(
	ctx context.Context,
	sealedRunID string,
	taskID string,
) (string, executionTaskInventory, error)
```

- sealed Run ID가 있으면 그 Run 하나만 조회한다.
- 비어 있으면 `ListRuns` 전체를 읽고 exact task ID가 있는 Run만 모은다.
- 후보 1개만 반환하고 0/N은 `run_inventory_ambiguous`로 거부한다.
- resolved Run은 task/dispatch 조회와 task settle에만 쓰며 record에 쓰지 않는다.

- [ ] **Step 6: execution adapter focused 테스트를 통과시킨다**

Run:

```bash
go test ./internal/adapter/orca -run 'TestExecution.*(Run|Intent|OwnerInventory|Launch|Runtime)' -count=1
```

Expected: PASS.

- [ ] **Step 7: Task 3을 커밋한다**

```bash
git add internal/adapter/orca/execution.go internal/adapter/orca/execution_test.go internal/adapter/orca/execution_launch_timing_fixtures_test.go internal/adapter/orca/execution_dispatch_vocabulary_test.go
git commit -m "feat(issueops): provision lifecycle Orca Runs" \
  -m "Context: the execution adapter needs a recoverable Run mutation before creating owner tasks." \
  -m "Decision: add Run create/bind inspect and invoke stages, then resolve legacy task ownership only from unique explicit Run inventory." \
  -m "Consequences: task and dispatch receipts remain scoped across prepare, resume, and owner inspection."
```

### Task 4: Core durable intent state machine과 final binding

**Files:**
- Modify: `internal/core/issueops/execution_orca_intent.go`
- Modify: `internal/core/issueops/execution_orca_intent_test.go`
- Modify: `internal/core/issueops/execution_prepare.go`
- Modify: `internal/core/issueops/execution_orca_test.go`
- Modify: `internal/core/issueops/execution_resume.go`
- Modify: `internal/core/issueops/execution_resume_test.go`
- Modify: `internal/core/issueops/execution_reconcile_disclosure_test.go`

**Interfaces:**
- Consumes: Run create/bind request/receipt from Task 3
- Produces: durable payload `RunID`/`RunBound`, six-stage prepare/resume progression, final `OrcaBinding.RunID`
- Preserves: one external mutation attempt per stage and current CAS/reconcile retry limits

- [ ] **Step 1: intent crash/reconcile matrix에 Run stage를 추가한다**

기존 matrix를 다음 exact 순서로 확장한다.

```go
tests := []struct {
	name      string
	stage     port.ExecutionOrcaIntentStage
	nextKind  string
	completed bool
}{
	{name: "worktree", stage: port.ExecutionOrcaIntentWorktree, nextKind: "owner_launch"},
	{name: "terminal", stage: port.ExecutionOrcaIntentTerminal, nextKind: "owner_launch"},
	{name: "run-create", stage: port.ExecutionOrcaIntentRun, nextKind: "owner_launch"},
	{name: "run-bind", stage: port.ExecutionOrcaIntentRunBind, nextKind: "owner_launch"},
	{name: "task", stage: port.ExecutionOrcaIntentTask, nextKind: "dispatch"},
	{name: "dispatch", stage: port.ExecutionOrcaIntentDispatch, completed: true},
}
```

`successfulExecutionOrcaIntentReceipt`의 Run case:

```go
case port.ExecutionOrcaIntentRun:
	return port.ExecutionOrcaIntentReceipt{RunID: "run-1"}
case port.ExecutionOrcaIntentRunBind:
	return port.ExecutionOrcaIntentReceipt{RunID: request.RunID, RunBound: true}
```

final binding assertion은 `RunID == "run-1"`을 요구한다.

- [ ] **Step 2: stage validation edge-case 테스트를 작성한다**

다음을 table test로 고정한다.

- terminal stage에 RunID가 있으면 later-stage receipt 오류
- run stage에 TerminalPTYID가 없으면 오류
- run stage에 기존 RunID/TaskID가 있으면 오류
- run-bind stage에 RunID가 없거나 RunBound가 이미 true면 오류
- task stage에 RunID가 없거나 RunBound가 false면 오류
- dispatch stage에 RunID 또는 TaskID가 없으면 오류
- Run receipt가 빈 값이면 CAS 전진 없음
- Run bind receipt의 RunID가 request와 다르거나 RunBound가 false면 CAS 전진 없음
- Run bind unknown 뒤 fresh coordinator가 unbound이면 attempt 상한 안에서
  exact target `UseRun` 재bind를 허용
- Run bind 재bind도 연속 두 번 unknown이면 추가 호출 없이 fail-closed
- resume이 live terminal을 재사용해도 다음 stage가 Run임
- 새 generation resume은 prior binding RunID를 새 payload authority로 재사용하지 않음

- [ ] **Step 3: core focused 테스트가 4-stage 가정으로 실패하는지 확인한다**

Run:

```bash
go test ./internal/core/issueops -run 'TestExecution(Orca.*(Crash|Reconcile|Receipt|Stage|Run)|Resume.*Run)' -count=1
```

Expected: fixed stage count 또는 unsupported Run stage로 FAIL.

- [ ] **Step 4: external payload와 stage 전이를 구현한다**

`externalOrcaIntentPayload`:

```go
RunID    string `json:"run_id,omitempty"`
RunBound bool   `json:"run_bound,omitempty"`
```

전이:

```text
terminal receipt -> run_create
run receipt      -> payload.RunID + run_bind
bind receipt     -> payload.RunBound + task_create
task receipt     -> payload.TaskID + dispatch
dispatch receipt -> OrcaBinding{RunID, TaskID, DispatchID}
```

`orcaIntentRequest`는 payload RunID를 request에 복사한다. validation은 Global
Constraints의 stage별 receipt 조건을 exact하게 적용한다. prepare loop limit은
4에서 6으로 올리되 무제한 loop로 바꾸지 않는다.

- [ ] **Step 5: resume payload도 전용 Run stage에서 시작하게 한다**

이전 task가 종결되어 새 owner launch가 필요하면 reused terminal 여부와 무관하게
RunID를 비우고 `run_create`를 먼저 기록한다. `PriorBinding.RunID`는 이전 owner
inspection에만 사용하고 신규 task mutation authority로 복사하지 않는다.

- [ ] **Step 6: core intent와 resume focused 테스트를 통과시킨다**

Run:

```bash
go test ./internal/core/issueops -run 'TestExecution(Orca|Resume|Reconcile).*(Intent|Run|Crash|Receipt|Owner|Runtime)' -count=1
go test ./internal/core/issueops -run 'TestExecutionOrcaReconcile(ZeroMultipleAndTransportAmbiguityNeverMutate|RetriesOnlyProvenNotInvokedAndOnlyOnce)' -count=1
```

Expected: PASS.

- [ ] **Step 7: Task 4를 커밋한다**

```bash
git add internal/core/issueops/execution_orca_intent.go internal/core/issueops/execution_orca_intent_test.go internal/core/issueops/execution_prepare.go internal/core/issueops/execution_orca_test.go internal/core/issueops/execution_resume.go internal/core/issueops/execution_resume_test.go internal/core/issueops/execution_reconcile_disclosure_test.go
git commit -m "feat(issueops): seal Orca Runs in external intents" \
  -m "Context: Run creation must survive crashes without sharing mutation authority with task creation." \
  -m "Decision: add separate Run create and bind stages and persist their receipts through resume, reconcile, and final binding CAS." \
  -m "Consequences: every generation owns a recoverable Run before task dispatch."
```

### Task 5: Completion, cleanup, owner inspection 소비 경로

**Files:**
- Modify: `internal/core/issueops/execution_api.go`
- Modify: `internal/core/issueops/execution_complete.go`
- Modify: `internal/core/issueops/execution_complete_orca_settle_test.go`
- Modify: `internal/core/issueops/execution_lease.go`
- Modify: `internal/core/issueops/execution_lease_test.go`
- Modify: `internal/core/issueops/issueops_cleanup_abandon.go`
- Modify: `internal/core/issueops/issueops_cleanup_abandon_orca_test.go`
- Modify: `cmd/issueops/issueopscli/executioncmd/execution.go`
- Modify: `cmd/issueops/issueopscli/executioncmd/execution_settle_wiring_test.go`
- Modify: `cmd/issueops/issueopscli/issueops_execution_cli.go`
- Modify: `cmd/issueops/mcpcli/mcp_tool_issueops_execution.go`

**Interfaces:**
- Consumes: `OrcaBinding.RunID`, `ExecutionOrcaOwnerInventoryRequest.RunID`
- Produces: `SettleOrcaTask(ctx, runID, taskID)` through core, CLI, and MCP wiring
- Preserves: completion commit-before-best-effort-settle semantics

- [ ] **Step 1: Run ID 전달 회귀 테스트를 작성한다**

fake settler:

```go
type fakeTaskSettler struct {
	runID  string
	taskID string
}

func (f *fakeTaskSettler) settle(_ context.Context, runID, taskID string) error {
	f.runID = runID
	f.taskID = taskID
	return nil
}
```

completion test는 `runID == record.Execution.Orca.RunID`와 exact task ID를
검증한다. resume/replace/abandon owner inspector fake도 request RunID가 binding
값과 같은지 확인한다. CLI action wiring test는 두 인자를 그대로 전달하는지
검증한다.

- [ ] **Step 2: 소비 경로 테스트가 기존 단일 taskID signature로 실패하는지 확인한다**

Run:

```bash
go test ./internal/core/issueops -run 'TestExecution(CompleteSettlesOrcaTask|OwnerInventory.*Run)|TestAbandon.*Orca.*Run' -count=1
go test ./cmd/issueops/issueopscli/executioncmd -run 'TestActionDepsCarriesTheOrcaTaskSettler' -count=1
```

Expected: function signature 또는 missing RunID assertion으로 FAIL.

- [ ] **Step 3: settle dependency와 모든 owner inventory request에 Run ID를 전달한다**

다음 signature를 core/CLI/MCP 전체에서 일치시킨다.

```go
SettleOrcaTask func(ctx context.Context, runID, taskID string) error
```

`settleExecutionOrcaTask`는 binding의 RunID와 TaskID를 함께 넘긴다. legacy
binding의 RunID가 비어 있어도 adapter의 unique resolver가 처리하도록 빈 값을
그대로 넘기며 current Run을 추론하지 않는다.

`execution_resume.go`, `execution_lease.go`, `issueops_cleanup_abandon.go`의
`ExecutionOrcaOwnerInventoryRequest`는 모두 `RunID: binding.RunID`를 포함한다.

- [ ] **Step 4: lifecycle 소비 경로 focused 테스트를 통과시킨다**

Run:

```bash
go test ./internal/core/issueops -run 'TestExecution(Complete|Resume|Replace|Lease).*(Orca|Run)|TestAbandon.*Orca' -count=1
go test ./cmd/issueops/issueopscli/executioncmd -run 'TestActionDeps.*Settler' -count=1
go test ./cmd/issueops/issueopsapp -run 'Test.*Execution.*Wiring' -count=1
go test ./cmd/issueops/mcpcli -run 'Test.*IssueOps.*Execution' -count=1
```

Expected: PASS.

- [ ] **Step 5: Task 5를 커밋한다**

```bash
git add internal/core/issueops/execution_api.go internal/core/issueops/execution_complete.go internal/core/issueops/execution_complete_orca_settle_test.go internal/core/issueops/execution_lease.go internal/core/issueops/execution_lease_test.go internal/core/issueops/issueops_cleanup_abandon.go internal/core/issueops/issueops_cleanup_abandon_orca_test.go cmd/issueops/issueopscli/executioncmd/execution.go cmd/issueops/issueopscli/executioncmd/execution_settle_wiring_test.go cmd/issueops/issueopscli/issueops_execution_cli.go cmd/issueops/mcpcli/mcp_tool_issueops_execution.go
git commit -m "fix(issueops): retain Run authority through cleanup" \
  -m "Context: completion and owner inspection previously projected only task and dispatch IDs." \
  -m "Decision: pass the sealed Run ID through settle, resume, replace, abandon, CLI, and MCP wiring." \
  -m "Consequences: lifecycle consumers can inspect exact Runs and legacy resolution remains adapter-controlled."
```

### Task 6: Operational inventory, contract 검증, 설치와 #194 dogfood

**Files:**
- Modify: `internal/core/operationalhealth/types.go`
- Modify: `internal/core/operationalhealth/classifier.go`
- Modify: `internal/core/operationalhealth/*_test.go`
- Modify: `internal/adapter/operationalhealth/collector.go`
- Modify: `internal/adapter/operationalhealth/collector_test.go`
- Modify if golden drift proves necessary: `cmd/issueops/contractgolden/testdata/*`
- Modify if response projection proves necessary: `cmd/issueops/issueopsapp/testdata/*`

**Interfaces:**
- Consumes: cross-Run `ListAllTasks`/`ListDispatchedTasks` rows with `RunID`
- Produces: cycle/task Run identity in operational snapshot and run-aware ownership matching
- Verifies: CLI/MCP contract, build/install identity, live #194 preview/confirm

- [ ] **Step 1: operational snapshot Run identity 테스트를 작성한다**

`corehealth.Cycle`과 `corehealth.OrcaTask`에 `RunID`를 추가하고 다음 경우를
먼저 테스트로 표현한다.

- binding RunID가 cycle snapshot에 투영됨
- 같은 task ID가 다른 Run에 있으면 `run_id + task_id`로 별도 행
- RunID가 있는 cycle은 같은 Run/task만 소유
- RunID 없는 legacy cycle은 task ID가 전체 inventory에서 유일할 때만 소유
- legacy task가 두 Run에 있으면 inventory ambiguity finding
- dispatched task set 비교도 Run/task key를 사용

- [ ] **Step 2: operational focused 테스트가 taskID-only matching으로 실패하는지 확인한다**

Run:

```bash
go test ./internal/adapter/operationalhealth -run 'Test.*(Run|Task|Dispatch|Inventory)' -count=1
go test ./internal/core/operationalhealth -run 'Test.*(Run|Task|Dispatch|Residue)' -count=1
```

Expected: missing RunID projection 또는 taskID-only ownership mismatch로 FAIL.

- [ ] **Step 3: collector와 classifier를 Run-aware로 구현한다**

공통 key helper는 다음 semantics를 갖는다.

```go
func orcaTaskKey(runID, taskID string) string {
	return strings.TrimSpace(runID) + "\x00" + strings.TrimSpace(taskID)
}
```

신규 cycle은 exact Run/task key로 매칭한다. legacy cycle은 같은 task ID의
inventory row가 정확히 하나일 때만 그 row를 소유한다. 여러 Run 후보는 소유로
간주하지 않고 명시적 ambiguity problem/finding을 남긴다.

- [ ] **Step 4: operational focused 테스트를 통과시킨다**

Run:

```bash
go test ./internal/adapter/operationalhealth -run 'Test.*(Run|Task|Dispatch|Inventory|Collector)' -count=1
go test ./internal/core/operationalhealth -run 'Test.*(Run|Task|Dispatch|Residue|Owner)' -count=1
```

Expected: PASS.

- [ ] **Step 5: compile surface와 contract golden을 작은 묶음으로 검증한다**

Run:

```bash
go test ./internal/port ./internal/core/issueops/model -count=1
go test ./internal/adapter/orca -count=1
go test ./internal/core/issueops -run 'TestExecution(Orca|Resume|Complete|Lease|Reconcile)|TestAbandon.*Orca' -count=1
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
go test ./cmd/issueops/issueopscli/executioncmd ./cmd/issueops/mcpcli -run 'Test.*(Execution|IssueOps)' -count=1
```

Expected: PASS. Golden mismatch가 실제 public `run_id` 추가 때문일 때만 해당
fixture를 갱신하고, 무관한 golden churn은 되돌린다.

- [ ] **Step 6: build와 source-level 잔여 unscoped command를 검증한다**

Run:

```bash
go build -o bin/issueops ./cmd/issueops
rg -n 'orchestration", "(task-create|task-list|task-update|dispatch|send)' internal/adapter/orca
git diff --check
```

Expected:

- build PASS
- `task-create`, `task-list`, `task-update`, `dispatch`, `send` production argv는
  Run ID를 추가하는 경로를 통과
- whitespace error 없음

- [ ] **Step 7: Task 6 코드 변경을 커밋하고 main을 푸시한다**

```bash
git add internal/core/operationalhealth internal/adapter/operationalhealth cmd/issueops/contractgolden/testdata cmd/issueops/issueopsapp/testdata
git commit -m "fix(health): classify Orca tasks by Run" \
  -m "Context: task IDs alone no longer identify ownership once orchestration is partitioned into explicit Runs." \
  -m "Decision: project Run IDs into health snapshots and match ownership with exact Run-task keys." \
  -m "Consequences: legacy cycles remain inspectable only when their task is globally unique."
git push origin main
```

- [ ] **Step 8: native 설치와 daemon identity를 갱신한다**

Run:

```bash
io update
io daemon status --json
io inspect --json
```

Expected:

- update가 daemon을 verified stop/start
- daemon ready/reachable
- installed binary와 current checkout build identity 일치
- Codex/Claude native activation receipt 정상

- [ ] **Step 9: #194 parent base를 최신 main으로 다시 봉인한다**

Parent worktree:

```text
/Users/sample/workspace/issueops.worktrees/117-hexagonal-architecture-migration
```

순서:

1. parent worktree가 clean인지 확인한다.
2. `origin/main`을 merge한다.
3. 충돌이 있으면 parent의 hexagonal vertical 변경과 Run contract를 모두
   보존하는 semantic resolution을 한다.
4. Task 1~6의 focused tests 중 충돌 파일 관련 묶음만 재실행한다.
5. parent branch를 push한다.
6. #194 intent constraint와 branch prepare의 `base_sha`를 새 parent HEAD로
   다시 기록한다.

- [ ] **Step 10: #194 preview를 mutation 없이 dogfood한다**

현재 coordinator actor receipt와 exact parent cwd로 다음 profile을 사용한다.

```text
mode=orca
owner_host=codex
owner_model=gpt-5.6-terra
owner_effort=xhigh
parent_worktree=/Users/sample/workspace/issueops.worktrees/117-hexagonal-architecture-migration
```

Expected preview:

- `resolved_mode=orca`
- `orchestration_unready` 없음
- Run capability와 repo/worktree identity ready
- Orca worktree/terminal/Run/task/dispatch mutation 0개

- [ ] **Step 11: #194 confirm과 단계별 readback을 실행한다**

confirm 후 각 단계에서 다음을 확인한다.

1. `run-list`: canonical marker objective의 새 Run 정확히 1개
2. `task-list --run <run_id>`: #194 task 정확히 1개
3. `dispatch-show --task <task_id>`: sealed dispatch/terminal 일치
4. IssueOps binding: runtime/repo/worktree/Run/task/dispatch/PTY 모두 non-empty
5. Orca lineage: child의 parentWorktreeId가 #117 parent worktree
6. terminal agent: Codex `gpt-5.6-terra/xhigh`
7. claim: child worktree에서 generation/token/snapshot digest 일치
8. resume/reconcile preview: active 정상 상태에서 mutation 0개

어느 readback도 불일치하면 후속 mutation을 중단하고 해당 일반 결함을 main에서
수정한 뒤 이 Task의 focused cycle을 반복한다.

- [ ] **Step 12: #194 vertical 구현으로 복귀한다**

Run contract dogfood가 모두 통과하면 #194 staged plan을 link하고 stale inherited
compatibility review를 현재 evidence로 교체한다. 이후 기존 #194 구현 계획의
implement phase를 Terra/xhigh owner에게 dispatch하고, independent Sol/xhigh
review와 분할 테스트를 거쳐 parent branch 대상 PR/MR lifecycle을 계속한다.
