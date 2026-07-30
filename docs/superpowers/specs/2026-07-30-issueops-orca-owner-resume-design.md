# IssueOps Orca Owner Resume 설계

## 배경

GitHub #197 구현을 재개하려고 released lease를 `execution replace --reseed`로
재봉인하는 과정에서 Orca binding의 세대 정체가 끊기는 결함이 확인됐다.

현재 reseed는 새 generation의 claim token, context packet, owner prompt를
생성하지만 `Execution.Orca`의 `task_id`, `dispatch_id`,
`terminal_pty_id`는 최초 prepare가 만든 완료된 dispatch를 계속 가리킨다.
새 owner에게 prompt를 수동 전송하면 코드는 실행할 수 있지만 IssueOps 상태에는
실제 owner provenance가 남지 않는다. 이후 replacement, cleanup, completion이
이전 task를 관찰하는 것도 잘못이다.

이 설계는 메모리나 수동 prompt 우회 없이, 기존 durable external-intent와
reconcile 경계를 재사용해 holderless generation에 새 Orca owner를 연결한다.

## 목표

- released/claimable generation을 재봉인한 뒤 같은 canonical worktree에 새
  Orca terminal, task, injected dispatch를 생성한다.
- 각 Orca 외부 mutation 전에 durable intent를 기록하고, 결과가 모호하면
  재실행하지 않은 채 `execution reconcile`로 복구한다.
- durable `OrcaBinding`이 현재 lease generation의 owner만 가리키게 한다.
- 동일 generation의 live binding에 대한 재요청은 새 owner를 중복 생성하지
  않고 멱등 성공한다.
- CLI, MCP, PreToolUse command parser와 response contract가 같은 기능을
  노출한다.
- Codex owner 기본값 `gpt-5.6-terra/xhigh`와 Claude owner 기본값
  `claude-sonnet-5/high`를 그대로 사용한다.

## 비목표

- direct mode의 실행 계약 변경
- active lease 강제 교체 또는 replacement 안전 게이트 완화
- 기존 worktree 재생성, branch/base/parent lineage 변경
- claim token 또는 sealed packet 공개
- 완료된 기존 Orca terminal/task 자동 삭제
- claim vertical의 domain/application migration

마지막 항목은 #197의 범위다. 이 복구 기능은 #197을 실행할 control-plane을
정상화하지만 #197의 production vertical을 선행 구현하지 않는다.

## 검토한 접근

### A. 명시적 `execution resume` 명령

reseed와 owner dispatch를 분리한다. reseed는 세대와 sealed artifact를
회전하고, resume은 현재 generation에 새 Orca owner를 연결한다.

장점:

- lease replacement와 외부 Orca mutation의 실패 경계가 분명하다.
- 기존 pending intent와 reconcile 흐름을 그대로 재사용할 수 있다.
- 재개가 필요한지 status와 binding generation으로 판단할 수 있다.

단점:

- CLI/MCP/hook에 새 공개 명령을 추가해야 한다.

### B. `replace --reseed`가 owner까지 자동 생성

사용자 명령은 하나지만 reseed의 SQLite transition과 여러 Orca mutation이
한 요청에 섞인다. 외부 결과가 모호할 때 lease rotation 성공과 owner launch
실패를 한 응답에서 구분해야 하고, 기존 replace 함수의 transaction 책임도
커진다.

### C. owner prompt 수동 전송

구현량은 없지만 새 task/dispatch provenance가 durable state에 기록되지 않는다.
IssueOps status와 실제 owner가 달라지는 현재 결함을 그대로 유지하므로 제외한다.

선택은 A다.

## 상태 모델

`model.OrcaBinding`에 additive 필드가 추가된다.

```text
lease_generation: uint64
```

- 새 prepare와 resume은 현재 `WriteLease.Generation`을 기록한다.
- 기존 schema v1 레코드의 missing/zero 값은 legacy unknown으로 읽는다.
- zero를 현재 세대라고 추론하지 않는다. 다음 resume 성공 시 현재 generation으로
  갱신한다.
- public schema version은 1을 유지한다. JSON additive field이며 기존 reader는
  알 수 없는 필드를 무시한다.

resume의 입력 상태는 다음과 같다.

```text
mode == orca
pending == nil
lease.status == claimable
lease.generation == expected_generation
claim_token_sha256 != ""
canonical cwd 일치
generation별 packet/prompt가 private bounded regular file이며 digest 검증 성공
```

`released` 상태는 resume하지 않는다. 먼저 `replace --reseed`를 실행해야 한다.
세대 회전과 owner 재연결을 분리해 resume이 token을 암묵적으로 발급하지 않게
한다.

## 명령 계약

```bash
agent-harness issueops execution resume \
  --id ID \
  --expected-generation N \
  ACTOR_FLAGS \
  --confirm \
  --json
```

- `--confirm` 없는 요청은 외부 mutation을 수행하지 않는다.
- `--expected-generation`은 필수 CAS다.
- mode가 direct이거나 lease가 active/released/revoking이면 fail-closed다.
- 현재 binding이 같은 generation이고 terminal과 task가 모두 live이면 현재
  binding과 sealed artifact identity를 반환하는 멱등 성공이다.
- task만 live이고 terminal이 사라진 모순 상태는 새 owner를 중복 생성하지 않고
  fail-closed로 거부한다.
- 현재 binding의 task가 live이면서 다른 generation이면 새 owner를 만들지 않고
  `previous Orca owner task is still live`로 거부한다.
- 같은 generation binding이 settled됐으면 새 terminal/task/dispatch를 만든다.
- 이전 generation binding이 settled됐거나 legacy zero이면 새 owner를 만든다.
- 성공 응답은 execution, artifact 경로/digest, claim token 경로와 exact claim
  next command를 포함한다. token 원문은 포함하지 않는다.

CLI와 MCP는 하나의 core request/result DTO를 사용한다. MCP tool 이름은
기존 규칙에 맞춰 `issueops_execution_resume`으로 둔다.

## 실행 흐름

### 1. 읽기·검증

1. current record와 expected generation을 읽는다.
2. canonical CWD, claimable lease, token hash, sealed artifact를 검증한다.
3. 현재 Orca binding inventory를 읽는다.
4. 같은 generation의 terminal과 task가 모두 live이면 멱등 성공한다.
5. task만 live인 모순 상태면 거부한다.
6. 다른 generation의 live task면 거부한다.

### 2. resume intent 시작

새 `beginOrcaExecutionResumeIntent`는 기존 worktree를 다시 만들지 않는다.

- 기존 workspace와 Orca worktree receipt를 payload에 봉인한다.
- 현재 generation의 owner prompt/context packet digest를 launch identity로
  봉인한다.
- 현재 binding의 owner host/model/effort를 재사용한다.
- terminal inventory가 이전 generation의 idle terminal을 채택하지 않도록
  `lifecycle + generation + operation_id`로 구성한 resume 전용 marker를 봉인한다.
- payload purpose를 `resume`으로 기록한다.
- 첫 stage를 `terminal_create`로 두고 `Execution.Pending`과 payload를 하나의
  SQLite transition으로 기록한다.

### 3. terminal → task → dispatch

기존 `executeOrcaIntentStage`와 adapter를 재사용한다.

```text
terminal_create
  → task_create
  → dispatch
  → OrcaBinding 교체
```

각 stage는 기존처럼 inventory를 먼저 조회한다.

- authoritative zero이고 `not_invoked_proven`일 때만 외부 mutation을 호출한다.
- 후보가 하나면 receipt를 채택한다.
- 후보가 둘 이상이거나 결과가 모호하면 pending intent를 보존한다.
- retry 상한과 runtime identity 검증을 유지한다.

dispatch receipt를 적용할 때 payload purpose가 `resume`이면 lease 전체를 새
구조체로 덮어쓰지 않는다. generation, status, token hash와 replacement audit
필드가 시작 시점과 동일한지 CAS로 확인하고 그대로 보존한다. 바뀌는 값은
`Execution.Orca`, `Pending`, `Failure`뿐이다.

새 binding은 다음을 기록한다.

```text
runtime_id
repo_id
worktree_id
worktree_instance_id
owner_host
owner_model
owner_effort
lease_generation
task_id
dispatch_id
terminal_pty_id
```

### 4. reconciliation

resume은 prepare와 같은 external-intent payload를 사용하므로 기존
`execution reconcile --preview|--confirm`이 별도 우회 없이 이어받는다.

- pre-invocation failure는 exact intent만 정리하고 record를 claimable로 둔다.
- ambiguous invocation은 pending을 보존한다.
- reconcile은 현재 stage를 한 단계씩 검증·진행한다.
- 최종 dispatch receipt가 durable binding에 반영된 뒤에만 pending을 지운다.

## 구성 요소 변경

### Core/model

- `OrcaBinding.LeaseGeneration`
- `ExecutionResumeRequest`, `ExecutionResumeResult`
- `ResumeExecutionWithDependencies`
- resume용 external-intent begin과 purpose-aware dispatch receipt 적용

### Port/adapter

- 기존 `ExecutionOrcaProvisioner`와 owner inventory port를 재사용한다.
- Orca adapter에 새 전문 기능을 복제하지 않는다.

### CLI/MCP/composition

- `issueops execution resume` flag parser와 harnessapp facade
- `issueops_execution_resume` MCP schema/handler
- response contract, usage, MCP golden 갱신

### Hook/guard

- exact command parser에 resume flag set을 등록한다.
- PreToolUse execution guard가 preview가 아닌 confirmed resume만 mutation으로
  분류한다.
- `--issue-snapshot-file`은 resume에 필요하지 않다. resume은 reseed가 이미
  봉인한 공개 이슈 snapshot을 다시 읽지 않는다.

## 오류 처리

- 모든 generation/CWD/status mismatch는 외부 호출 전에 거부한다.
- pending intent가 있으면 `execution reconcile`을 안내하고 새 resume을
  시작하지 않는다.
- artifact path의 symlink, mode, size, digest가 다르면 read-only 실패한다.
- Orca runtime rollover는 resume에서 암묵 채택하지 않는다. holderless reseed가
  inventory fingerprint로 runtime을 먼저 갱신해야 한다.
- 최종 binding CAS 실패 시 외부 dispatch 결과를 추측하지 않고 pending을
  유지해 reconcile 대상으로 남긴다.
- 기존 완료 terminal/task는 자동 삭제하지 않는다. cleanup은 별도 권한과
  lifecycle 단계가 소유한다.

## 테스트 전략

구현은 TDD로 진행한다.

1. Core RED
   - released lease는 resume 거부
   - claimable + previous settled binding은 terminal stage에서 시작
   - 다른 generation live task는 외부 호출 0회
   - 같은 generation live binding은 외부 호출 0회 멱등 성공
   - dispatch 완료 시 lease audit와 token hash를 보존하고 binding generation만
     현재 값으로 교체
   - ambiguous terminal/task/dispatch 결과는 pending 보존 후 reconcile 가능
   - legacy zero binding은 settled 상태에서 새 binding으로 승격
2. Adapter/CLI/MCP RED
   - CLI와 MCP가 같은 request/result를 composition root에 전달
   - MCP schema, usage, response contract에 resume이 정확히 한 번 존재
3. Guard RED
   - exact confirmed resume 허용
   - unknown flag, shell expansion, foreign CWD, missing confirm 차단
4. Focused GREEN
   - `go test ./internal/core/issueops -run 'Resume|Reconcile' -count=1`
   - `go test ./internal/adapter/orca -run 'Intent|Owner' -count=1`
   - `go test ./cmd/harness/issueopscli/executioncmd ./cmd/harness/harnessapp -run 'Resume|Golden' -count=1`
   - `go test ./internal/core/commandparse ./internal/core/lifecycle -run 'Resume|Execution' -count=1`
   - `go test ./internal/adapter/mcp ./cmd/harness/mcpcli -run 'Resume|IssueOps' -count=1`
   - `go test -race ./internal/core/issueops -run 'Resume|Reconcile' -count=1`
   - `go vet`은 변경된 패키지로 제한한다.
   - `go build -o bin/agent-harness ./cmd/harness`

로컬 `go test ./...`와 전체 race는 실행하지 않는다. full suite는 push 이후 CI에
맡긴다.

## Dogfood 완료 기준

main 구현·focused 검증·커밋·푸시 후 다음 순서가 실제 #197 lifecycle에서
성공해야 한다.

1. `ah update --json`
2. daemon build SHA와 installed binary SHA 일치
3. MCP contract check 성공
4. generation 2 released 상태에서 replace preview/reseed로 generation 3 발급
5. `execution resume --expected-generation 3 --confirm`
6. status의 `orca.lease_generation == 3`
7. 새 task/dispatch가 live이고 owner model/effort가 Terra/xhigh
8. 새 owner가 generation 3을 claim하고 implement phase에 진입

이 검증 전에는 #197 구현 재개를 완료로 보고하지 않는다.
