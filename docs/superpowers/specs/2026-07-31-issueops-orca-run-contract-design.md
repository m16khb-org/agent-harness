# IssueOps Orca Run 계약 호환성 설계

## 배경

Orca 1.4.162는 orchestration task를 명시적인 Run namespace 안에서 관리한다.
`task-create`, `task-list`, `task-update`, `dispatch`, `send`는 `--run`을
지원하며, Run을 지정하지 않은 호출은 현재 coordinator terminal의 바인딩 또는
legacy coordinator에 의존한다.

현재 agent-harness Orca adapter는 Run 개념을 모델링하지 않는다. Probe는
`orca orchestration task-list --ready --json`을 호출하고, task 생성·조회·갱신과
dispatch도 `--run` 없이 실행한다. Orca가 보존한 legacy coordinator의 원래
process identity를 증명하지 못하면 이 호출은 `legacy_read_only`로 거부된다.
IssueOps에서는 이 typed 원인이 `orchestration_unready`로 축약되어 정상적인
Orca 실행 전체가 중단된다.

실제 설치된 Orca 1.4.162에서 다음을 확인했다.

- runtime과 graph는 모두 ready/reachable이다.
- `run-list`는 정상 동작하며 일반 Run과 `run_legacy_local`을 구분한다.
- 전역 `task-list`는 `legacy_read_only`로 실패한다.
- 같은 명령에 `--run run_legacy_local` 또는 일반 Run ID를 명시하면 정상
  inventory를 반환한다.
- `run-create`는 lightweight namespace를 만들며 worker를 배치하지 않는다.
- `task-create`, `task-list`, `task-update`, `dispatch`, `send`는 `--run`을
  공식 지원한다.
- 설치 번들의 `resolveRunScope(... requireCurrentConsumer: true)`는
  `task-create`, `task-update`, `dispatch`가 explicit Run ID뿐 아니라 호출
  terminal의 current coordinator binding도 요구하고, 불일치하면
  `consumer_fenced`로 거부한다.
- `run-create`는 Run을 만들면서 호출 coordinator terminal에 bind하고,
  `run-use`는 ordinary Run을 다른 current coordinator terminal에 다시 bind할
  수 있다.
- `dispatch-show`, `gate-list`, `inbox`는 현재 버전에서 Run 플래그를 제공하지
  않는다.

따라서 readiness check만 완화해서는 안 된다. Probe를 통과한 뒤 실제
task/dispatch가 다시 legacy coordinator에 도달하는 부분 통합이 되기 때문이다.

## 목표

- 신규 IssueOps Orca lifecycle은 각각 고유한 Orca Run을 소유한다.
- Run 생성도 worktree, terminal, task, dispatch와 같은 durable external
  intent로 기록하고 unknown-result reconcile을 지원한다.
- Run bind를 Run 생성과 분리된 durable external intent로 기록해 다른
  coordinator session에서도 reconcile을 계속할 수 있게 한다.
- task 생성·조회·갱신과 dispatch는 봉인된 exact Run ID를 사용한다.
- coordinator mutation은 현재 process에 주입된 exact
  `ORCA_TERMINAL_HANDLE`을 fail-closed gate로 검증하되 `--from`은 생략해
  Orca가 process authority를 직접 인증하게 한다.
- Probe는 legacy coordinator의 상태가 아니라 Run API의 readiness와 필요한
  `--run` capability를 검증한다.
- resume, replace, reconcile, completion, cleanup, operational inventory가
  암묵적 current/shared Run에 의존하지 않는다.
- Run ID가 없는 기존 record와 Orca 1.4.162의 recovered Run을 읽기 전용
  inventory로 안전하게 해석한다.
- CLI와 MCP는 같은 core/adapter 계약을 사용한다.

## 비목표

- Orca의 scheduler, worker placement 또는 Run 저장소를 agent-harness에 복제
- 기존 recovered Run을 신규 lifecycle의 공유 namespace로 재사용
- `run-use --takeover-legacy`로 사용자의 coordinator terminal 소유권 변경
- owner worker terminal을 coordinator로 bind해 두 역할의 authority를 합침
- SQLite 직접 수정이나 현재 process memory 기반 Run 선택
- Orca에 존재하지 않는 Run 삭제 기능을 추측해 cleanup에 추가
- OpenWiki 자동 갱신

## 검토한 접근

### A. Probe만 `run-list`로 변경

Preview는 통과하지만 task 생성과 dispatch가 여전히 전역 legacy coordinator에
의존한다. 실제 mutation 직전에 같은 오류가 재발하므로 제외한다.

### B. 기존 recovered Run 또는 current Run 재사용

코드 변경은 작지만 여러 lifecycle의 task와 메시지가 같은 namespace에 섞인다.
현재 terminal binding과 로컬 migration 상태에 따라 결과도 달라진다. 다른
머신과 다른 사용자가 재현할 수 있는 durable 계약이 아니므로 제외한다.

### C. lifecycle별 전용 Run을 durable intent로 생성

외부 mutation 하나당 intent stage 하나를 유지하고, Run ID를 이후 모든
orchestration mutation의 authority로 사용한다. unknown result도 marker로
reconcile할 수 있고 lifecycle 간 격리가 명확하다. 이 방안을 선택한다.

## 핵심 불변식

신규 Orca 실행은 다음 관계를 만족해야 한다.

```text
IssueOps lifecycle + generation + operation marker
    -> exact Orca Run objective
    -> exact Run ID
    -> exact task ID
    -> exact dispatch ID
```

- Run ID는 Orca runtime ID와 별개의 namespace identity다.
- task/dispatch mutation은 durable payload에 봉인된 Run ID가 없으면
  호출하지 않는다.
- Run 생성과 task 생성을 한 intent stage에서 연속 실행하지 않는다.
- Run 생성과 coordinator bind도 서로 다른 intent stage로 둔다.
- current Run, recovered Run, terminal-local binding은 authority가 아니다.
- coordinator handle은 현재 호출을 인증하는 transient identity이며 durable
  record에 저장하지 않는다. 호출마다 `ORCA_TERMINAL_HANDLE`에서 다시 읽는다.
- coordinator handle이 없거나 concrete terminal 형식이 아니면 focus/cwd
  fallback 없이 mutation 전에 거부한다.
- Run inventory에서 marker가 같은 후보가 0개면 authoritative zero, 1개면
  reconcile 후보, 2개 이상이면 ambiguous로 거부한다.
- Run ID가 있는 record는 task ID가 같은 다른 Run을 절대 채택하지 않는다.
- legacy record의 Run 추론은 모든 명시적 Run inventory에서 exact task ID가
  하나만 발견될 때만 허용한다.

## 데이터 모델

다음 port/model에 `run_id`를 추가한다.

- `ExecutionOrcaIntentRequest`
- `ExecutionOrcaIntentReceipt`
- external Orca intent payload
- `ExecutionOrcaReceipt`
- `ExecutionOrcaOwnerInventoryRequest`
- `model.OrcaBinding`
- cleanup과 operational-health에서 사용하는 Orca authority projection

신규 intent stage `run_create`와 `run_bind`를 추가한다. 순서는 다음과 같다.

```text
worktree_create
terminal_create
run_create
run_bind
task_create
dispatch
```

Run objective는 사람이 읽을 수 있으면서 exact match가 가능한 canonical
IssueOps marker를 사용한다. 별도 랜덤 문자열이나 현재 시각만으로 identity를
만들지 않는다.

`OrcaBinding.RunID`는 JSON에서 optional로 읽어 기존 record의 역호환성을
유지한다. 다만 신규 prepare/resume의 최종 binding과 새 receipt 검증에서는
필수다. 기존 binding의 빈 Run ID를 현재 Run으로 자동 채우지 않는다.

## Adapter 계약

### Run API

Orca port에 다음 최소 기능을 둔다.

- `ListRuns`: complete Run inventory를 읽는다.
- `CreateRun(objective)`: 고유 objective로 Run을 생성하고 ID/objective를
  검증한다.
- `CurrentRun(from)`: 현재 coordinator terminal에 bind된 Run을 읽는다.
- `UseRun(runID, from)`: current coordinator terminal을 exact ordinary Run에
  bind한다.
- `ListTasks(runID, filter)`: 항상 `--run`을 전달한다.

Run list 응답은 ID, objective, legacy 여부를 보존한다. 빈 ID/objective,
중복 ID, completeness를 증명할 수 없는 응답은 거부한다.

### Probe

Probe는 다음을 확인한다.

1. runtime과 graph ready
2. repository identity
3. `run-create`, `run-list` capability
4. `run-use`, `run-current` capability
5. task create/list/update, dispatch, send의 `--run` capability
6. `ORCA_TERMINAL_HANDLE`의 concrete identity와 현재 process에서 실행한
   `run-current --json` readback
7. `run-list --json`의 정상 응답과 runtime identity

Probe는 전역 `task-list`를 호출하지 않는다. 사용자의 coordinator terminal에
Run을 bind하거나 Run을 생성하지도 않는다.

### Mutation

- `CreateTask` request는 non-empty Run ID를 요구하고 `--run`을 전달한다.
- `Dispatch` request도 같은 Run ID를 요구한다.
- `UpdateTask`와 worker-done `send`는 binding의 Run ID를 전달한다.
- `CreateRun`, `UseRun`, `CreateTask`, `UpdateTask`, `Dispatch`는 adapter가
  현재 environment의 coordinator handle 형식을 검증하되 `--from`은 생략한다.
  Orca가 호출 process의 terminal authority를 직접 인증하며, focus/cwd를
  권한으로 사용하지 않는다.
- Run ID가 없는 mutation request는 runner 호출 전에 typed
  `Invoked=false` 오류로 거부한다.
- `dispatch-show`, `gate-list`, `inbox`는 설치된 CLI가 Run 플래그를 제공하지
  않으므로 현재 exact task/dispatch identity 계약을 유지한다.

## Durable intent 및 reconcile

terminal receipt가 저장되면 다음 pending stage는 `run_create`다.

1. core가 lifecycle/generation/operation marker를 Run objective로 봉인한다.
2. adapter가 현재 coordinator process에서
   `run-create --objective <marker> --json`을 한 번 호출한다.
3. 정상 응답의 Run ID/objective/runtime을 검증한다.
4. receipt CAS에서 Run ID를 payload에 기록하고 stage를 `run_bind`로
   전진한다.
5. `run_bind`는 현재 process의 `run-current`가 같은 Run을 가리키는지 읽고,
   아니면 `run-use --id <run_id> --json`을 한 번 호출한다.
6. bind receipt CAS 뒤 stage를 `task_create`로 전진한다.
7. task와 dispatch stage는 같은 Run ID와 현재 coordinator process
   authority만 사용한다.
8. dispatch receipt를 저장할 때 최종 `OrcaBinding.RunID`도 함께 기록한다.

Run 생성 결과가 timeout 또는 unknown이면 재호출하지 않는다. reconcile은
`run-list`에서 exact objective 후보를 조회한다.

- 0개: authoritative zero일 때만 기존 재시도 정책 적용
- 1개: receipt로 채택하고 다음 stage로 전진
- 2개 이상: ambiguous external mutation으로 중단

`run_bind` 결과가 unknown이면 fresh reconciler는 자신의 exact coordinator
handle로 `run-current`를 조회한다. 이미 target Run이면 receipt로 채택한다.
아니면 `run_bind`에 한해 exact target으로 수렴하는 bounded `run-use` 재bind를
허용한다. Run은 추가 생성되지 않고 current consumer만 target Run으로
교체되므로 resource-creating stage의 unknown-result 재실행 금지와 구분한다.
기존 intent attempt 상한을 넘기지 않으며, ordinary Run의 정상 재bind만
사용하고 `--takeover-legacy`는 사용하지 않는다.

payload validation은 stage별 later-stage receipt를 제한한다. `run_create`
진입에는 terminal receipt가 필요하고 Run ID/task ID는 없어야 한다.
`run_bind`에는 Run ID가 필요하고 bind receipt/task ID는 없어야 한다.
`task_create`에는 bind receipt와 Run ID가 필요하고 task ID는 없어야 한다.
`dispatch`에는 Run ID와 task ID가 모두 필요하다.

## Legacy record 호환성

Orca 1.4.162는 이전 task를 일반 recovered Run으로 이동해 보존할 수 있다.
기존 `OrcaBinding`에는 Run ID가 없으므로 다음 read-only resolver를 사용한다.

1. `run-list`로 모든 Run을 가져온다.
2. 각 Run에 대해 `task-list --run <id> --brief --json`을 호출한다.
3. binding의 exact task ID를 찾는다.
4. 후보가 정확히 하나이고 runtime/task/dispatch identity가 일치할 때만 그
   Run을 이번 operation의 resolved authority로 사용한다.
5. 0개 또는 2개 이상이면 fail-closed한다.

이 resolver는 신규 task/dispatch 생성에는 사용하지 않는다. 기존 binding의
inspection, completion settle, cleanup/recovery처럼 이미 존재하는 exact task를
다루는 경로에만 사용한다. durable record에 Run ID를 backfill하려면 별도의
CAS transition이 필요하며, 이번 변경에서는 읽기 결과를 임의 저장하지 않는다.

전역 operational inventory는 모든 명시적 Run의 task 목록을 합치고
`run_id + task_id`로 중복을 검사한다. legacy 전역 `task-list`는 호출하지 않는다.

## Completion 및 cleanup

- owner inspection은 binding Run ID가 있으면 해당 Run 하나만 조회한다.
- task settle은 exact Run ID와 task ID를 함께 전달하되, 현재 caller가 Run
  coordinator임이 증명될 때만 `task-update`를 호출한다.
- owner worker가 보내는 valid `worker_done`은 Orca가 task/dispatch를
  자동 종결하므로 정상 완료의 우선 경로다. owner terminal에서 coordinator
  권한을 얻기 위해 `run-use`를 호출하지 않는다.
- completion의 기존 best-effort settle이 coordinator가 아닌 caller에서
  거부되면 완료 state를 되돌리지 않고 typed 오류를 계속 표면화한다.
- legacy binding은 위 resolver가 유일한 Run을 증명했을 때만 settle한다.
- Run 자체는 Orca 1.4.162에 delete 명령이 없고 lightweight namespace이므로
  cleanup에서 추측성 삭제를 하지 않는다.
- task terminal 상태, dispatch 상태, terminal/worktree 제거, lease release,
  branch/ref cleanup은 기존처럼 별도 증거로 검증한다.
- orphan Run 판단은 Orca가 Run terminal lifecycle/delete 계약을 제공하기
  전까지 operational residue 기준에 추가하지 않는다.

## 오류와 관측성

다음 typed 오류를 구분한다.

- `run_capability_missing`: 필요한 Run CLI 계약 부재
- `run_identity_invalid`: Run 응답의 ID/objective가 불완전
- `run_identity_mismatch`: 응답 objective가 sealed marker와 다름
- `coordinator_identity_unavailable`: exact `ORCA_TERMINAL_HANDLE` 부재
- `run_binding_mismatch`: current coordinator가 target Run에 bind되지 않음
- `run_required`: 신규 task/dispatch mutation에 Run ID 누락
- `run_inventory_ambiguous`: marker 또는 legacy task가 여러 Run에 존재
- `run_inventory_incomplete`: 모든 Run/task 후보를 증명하지 못함

public 결과에는 Run ID, lifecycle, generation, operation ID만 노출한다.
prompt, claim token, 사용자 대화 원문은 오류나 inventory log에 넣지 않는다.

## 테스트 전략

전체 suite와 전체 race는 실행하지 않는다. 다음 묶음을 독립 실행한다.

### Orca adapter

- Probe가 `run-list`를 사용하고 전역 `task-list`를 호출하지 않음
- 필요한 help에 `--run`이 없으면 capability failure
- coordinator handle 부재/invalid/stale에서 mutation 0회
- CreateRun 응답의 ID/objective/runtime 검증
- CurrentRun과 UseRun의 current-process authority, Run ID, runtime 검증
- task-create/task-list/task-update/dispatch/send의 exact `--run`
- coordinator mutation의 `--from` 생략과 환경 handle gate
- Run ID 누락 시 runner 호출 0회
- 여러 Run task inventory의 completeness와 deduplication
- legacy task resolver의 0/1/N 후보

### IssueOps intent

- stage 순서가 worktree/terminal/run-create/run-bind/task/dispatch임
- Run 생성 unknown-result에서 재호출 금지
- Run bind unknown-result를 current Run readback으로 reconcile
- fresh coordinator session이 ordinary Run을 재bind한 뒤 task stage로 진행
- exact objective 1개만 reconcile
- 0개 authoritative zero와 2개 ambiguous 분기
- stage별 RunID/TaskID receipt validation
- final binding에 RunID 봉인
- prepare와 resume 모두 같은 stage 계약 사용

### Lifecycle 소비 경로

- owner inspection과 settle이 binding Run ID 사용
- Run ID 없는 legacy binding의 unique resolver
- runtime rollover에서 이전 Run을 신규 generation에 재사용하지 않음
- cleanup/abandon/operational health가 전역 legacy task-list에 의존하지 않음
- CLI/MCP response contract와 hook command policy 불변

### Dogfood

1. 관련 adapter/core/model/CLI/MCP 패키지 테스트를 작은 묶음으로 실행한다.
2. `go build`로 바이너리를 만든다.
3. `ah update` 후 daemon/MCP binary identity를 readback한다.
4. 실제 `#194` prepare preview가 mutation 없이 Orca mode로 resolve되는지
   확인한다.
5. confirm에서 child worktree, Terra/xhigh terminal, lifecycle 전용 Run,
   explicit coordinator bind, task, dispatch를 순서대로 생성한다.
6. binding의 Run ID와 각 Orca inventory의 Run ID를 대조한다.
7. claim, resume/reconcile preview, task settle, cleanup의 Run 경로를
   단계별로 확인한다.

## 완료 기준

- 신규 Orca 실행마다 lifecycle 전용 Run ID가 binding에 봉인된다.
- Probe와 모든 task/dispatch mutation이 legacy/current Run에 의존하지 않는다.
- fresh reconcile coordinator가 durable `run_bind` stage로 target Run의 current
  consumer가 된다.
- external mutation 하나당 durable intent stage 하나가 유지된다.
- Run 생성 unknown-result를 0/1/N inventory로 안전하게 reconcile한다.
- 기존 Run ID 없는 binding은 exact unique task 증거가 있을 때만 읽기/종결된다.
- #194 prepare preview와 confirm이 Orca 1.4.162에서 정상 동작한다.
- focused tests, build, 설치 readback, 실제 dogfood 증거가 모두 남는다.
