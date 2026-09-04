# IssueOps Execution Prepare Vertical Design

## 목적

`issueops execution prepare` 하나를 capability-local hexagonal vertical로
이전한다. 기존 CLI/MCP request와 result, schema v1 persisted bytes, direct와
Orca mode 선택, external-intent recovery, Codex/Claude hook의 authority 관측은
바꾸지 않는다.

이 작업은 기존 #199의 MCP/hook/daemon 수평 이동이 아니다. preparation이라는
하나의 use case만 contract, domain, application, inbound, outbound와 composition
root까지 완결한다.

## 현재 문제

production `ExecuteExecution`의 prepare 분기는 injected handler 없이
`internal/core/issueops.PrepareExecution`을 직접 호출한다. 그 함수와
`execution_orca_intent.go`는 다음 책임을 한 경계에서 소유한다.

- mode 기본값, normalize, probe와 auto fallback
- existing execution의 pending, idempotent, mode mismatch, writerless 판정
- sealed branch/base, canonical root collision, actor와 CWD 검증
- direct worktree access probe와 provision
- staged artifact materialization
- generation-1 active lease와 holder reverse index persistence
- Orca external-intent 시작, invocation, receipt CAS와 bounded stage loop
- legacy request/result와 next-command rendering

이 구조는 CLI와 daemon-backed MCP가 같은 switch에 도달하게 만들지만,
application contract나 consumer-owned port를 제공하지 않는다. completion을 포함한
다른 migrated action과 달리 prepare만 handler 누락 시 fail-closed할 수도 없다.

## 범위 결정

direct와 Orca는 하나의 공개 action인 `prepare --mode auto|direct|orca`의 mode다.
둘을 별도 child로 나누면 router가 mode를 먼저 해석하거나 한 mode를 legacy로
fallback해야 한다. 따라서 handler cutover와 preparation contract는 둘을 함께
소유한다.

다음 항목은 독립 capability이므로 제외한다.

- execution status
- replacement preview, revoke, finalize와 reseed
- resume, reconcile, complete
- sync-base, switch-mode와 cleanup
- 일반 lifecycle hook policy와 command parser 재설계
- Orca/provider protocol 확장

## 선택한 아키텍처

### Contract

`internal/contract/issueopspreparation`은 transport-neutral command/result와
Orca preparation intent의 유일한 stable codec을 소유한다. schema v1 record,
workspace, execution, lease와 actor projection은 이미 이 역할을 맡은
`internal/contract/issueopslease`를 재사용한다. preparation package가 같은 JSON
shape를 다시 선언하지 않는다. 두 contract package 모두 production
`internal/core/issueops`와 `model`을 import하지 않는다.

주요 타입은 다음과 같다.

- `Command`: lifecycle ID, requested mode, actor/process ancestry, CWD, owner
  host/model/effort, confirm
- `Result`: requested/resolved mode, fallback code, workspace, execution,
  claim/context/prompt artifact와 next command
- `Snapshot`: `issueopslease.Record`를 중심으로 sealed branch/base/parent
  worktree, root claims와 staged artifact metadata를 더한 application input
- `Intent`: Orca preparation operation, stage, invocation state와 sealed payload
- `IntentCodec`: canonical decode/validate/encode와 marker validation

`IntentCodec`은 기존 `externalOrcaIntentPayload`의 raw shape와 marker rule을
이동한 단일 source of truth다. 새 prepare와 기존 reconcile compatibility path가
같은 codec을 호출한다. wrapper로 core prepare orchestration을 부르거나 별도
intent DTO를 복제하지 않는다. JSON tags와 field order는 legacy canonical
re-marshal 순서를 그대로 고정하며 새 persisted field나 schema version은
추가하지 않는다.

### Domain

`internal/domain/issueopspreparation`은 I/O 없는 decision만 제공한다.

1. requested mode를 `auto|direct|orca`로 normalize한다.
2. existing execution을 pending, idempotent, mode mismatch, writerless로
   분류한다.
3. sealed snapshot에서 preview/deny/direct/orca decision을 만든다.
4. direct receipt와 actor를 generation-1 active execution으로 전환한다.
5. Orca intent stage receipt가 다음 stage 또는 terminal prepared state로
   전이 가능한지 판정한다.

시간, UUID, path resolution, Git, filesystem, SQLite, process와 Orca는 domain에
들어오지 않는다. error text는 inbound compatibility adapter가 stable reason을
legacy error로 mapping한다.

### Application

`internal/application/issueopspreparation.Service`는 codec/reconcile spike가
통과한 뒤에만 다음 최소 consumer-owned port를 조율한다.

- `Repository`: shared `issueopslease.Record` snapshot load, root-claim read,
  direct atomic apply와 codec-backed intent CAS
- `Clock`: legacy가 읽는 위치와 횟수를 보존한 UTC timestamp
- `OperationID`: deterministic test seam과 production CSPRNG
- `DirectWorkspace`: access probe와 worktree provision
- `OrcaGateway`: readiness/branch precheck와 stage별 external invocation
- `PreparationEvidence`: staged artifact materialization과 remote owner snapshot

한 production call site만 있는 기능을 더 작은 인터페이스로 다시 쪼개지 않는다.
clock과 operation ID는 deterministic seam이고, 나머지 port는 direct workspace,
Orca gateway, preparation evidence의 실제 세 외부 경계만 반영한다. application
흐름은 snapshot load와 domain decision으로 시작한다. preview와 idempotent
결과는 외부 mutation 없이 즉시 반환하고 새 preparation만 mode별 workflow로
진행한다.

### Inbound adapter

`internal/adapter/inbound/issueopspreparation`은 기존 core request/result와 새
contract를 양방향 mapping한다. CLI와 MCP schema는 이 adapter 밖에서 바뀌지
않는다. `ExecutionPrepareHandler`가 이 adapter의 유일한 production 호출 표면이다.

handler가 없으면 `ExecuteExecution`은 `ErrPrepareHandlerUnavailable`을 반환한다.
`PrepareExecution`으로 fallback하지 않는다.

### Outbound adapter

`internal/adapter/outbound/issueopspreparation`은 기존 SQLite bucket과 schema
v1 raw record를 shared `issueopslease` projection으로 읽는다. record와 holder
index 갱신은 한 sqlstore apply에 둔다. Intent row는 preparation contract의
공용 codec만 사용해 기존 bucket/key와 raw shape를 유지한다.

Git worktree, staged artifact, issue snapshot과 Orca client는 기존 authoritative
adapter를 조합하되 application에는 consumer-owned interface만 노출한다. 새
adapter가 `internal/core/issueops` orchestration을 호출하는 wrapper가 되어서는
안 된다.

### Composition root

`cmd/issueops/issueopsapp`만 production service를 조립한다. 현재
`cmd/issueops/issueopscli`와 `cmd/issueops/mcpcli`가 직접 만드는 Git worktree,
Orca와 provider reader를 preparation wiring으로 옮기고, 두 adapter의 dependency
struct에는 같은 typed prepare handler만 전달한다. daemon은 SDK MCP server에
동일 dependencies를 전달하므로 별도 preparation 구현을 갖지 않는다.

이 relocation은 handler cutover와 별도 순차 단계다. 먼저 dependency struct와
composition test로 concrete adapter caller를 0으로 만들고, 그 뒤 production
router를 handler-only로 전환한다.

## 데이터 흐름

### Preview와 existing execution

1. inbound가 legacy request를 command로 변환한다.
2. repository가 current snapshot과 canonical-root claim inventory를 읽는다.
3. domain이 mode와 existing execution을 판정한다.
4. pending은 reconcile next command, mode mismatch는 switch-mode next command,
   writerless confirm은 recovery next command를 반환한다.
5. preview와 idempotent 결과는 clock, provisioner, materializer, Orca와
   persistence를 호출하지 않는다.

### Direct confirm

1. mutation policy, actor/process, source/canonical CWD, sealed base와 root
   collision을 검증한다.
2. worktree access를 probe한다. denied면 relaunch command를 legacy와 같은
   error/result에 싣는다.
3. Git worktree를 provision한다. 이 외부 호출은 SQLite span 밖이다.
4. staged artifact를 canonical root에 materialize한다.
5. repository atomic apply가 current record와 root claim을 다시 확인하고
   worktree, generation-1 active lease와 holder index를 함께 저장한다.
6. clock read 위치와 direct persistence 실패 뒤 workspace residue는 legacy
   semantics를 그대로 둔다.

### Orca confirm

1. owner defaults, readiness probe, branch-free precheck, owner snapshot과 CWD를
   검증한다.
2. operation ID와 sealed payload를 만들고 external mutation 전에 intent를
   persist한다.
3. 각 stage에서 repository CAS로 `invoking`을 기록한다.
4. SQLite span 밖에서 Orca external call을 실행한다.
5. error/unknown outcome 또는 receipt를 CAS로 기록한다.
6. receipt decision이 가리키는 다음 stage를 최대 여섯 번 진행한다.
7. terminal receipt가 workspace, Orca binding, claim token과 owner artifacts를
   완성하면 record/index/intent를 legacy 순서로 반영한다.

기존 reconcile vertical은 새 prepare와 동일한 `IntentCodec`을 사용해 같은
intent bytes를 읽고 crash/unknown outcome을 복구한다. production service를
작성하기 전에 legacy intent → shared codec decode/encode → legacy reconcile
성공을 증명하는 compatibility spike가 반드시 통과해야 한다. 실패하면 contract
설계를 수정하고 handler cutover를 시작하지 않는다.

## 오류와 원자성

- validation error는 record, holder index, intent와 외부 gateway를 변경하지
  않는다.
- root collision은 provisioner 이전과 direct atomic apply 안에서 다시 확인해
  TOCTOU를 닫는다.
- record와 holder index는 분리 commit하지 않는다.
- intent는 external call 전에 durable해야 한다.
- Git, filesystem, issue provider와 Orca call은 SQLite span 안에서 실행하지
  않는다.
- external outcome이 불명확하면 absence로 추정하지 않고 pending intent를
  남겨 reconcile을 요구한다.
- 동일 preparation evidence 재시도는 legacy와 같은 idempotent result를
  반환하고, mode drift는 mutation 없이 거부한다.
- error priority와 exact text는 deterministic legacy/new differential로
  고정한다.

## Hook 활성 환경 계약

Hook은 preparation을 실행하거나 별도 policy를 복제하지 않는다. 새 prepare가
기존과 동일한 record, holder reverse index, native process receipt와 canonical
root를 쓰는지가 hook compatibility의 핵심이다.

focused test는 실제 Codex/Claude PreToolUse-shaped request로 다음을 비교한다.

- exact holder가 canonical root에서 mutation하면 허용
- source CWD라도 explicit target이 canonical root 안이면 기존 structured
  workdir 규칙대로 허용
- foreign session, agent, PID start identity와 executable mismatch는 거부
- canonical subdirectory와 symlink containment semantics 보존
- released/claimable/revoking lease의 deny code, generation, root와 next command
  보존

설치된 hook을 끄는 flag나 host별 bypass를 product behavior로 추가하지 않는다.

## 테스트 전략

첫 RED는 production service가 아니라 shared intent codec compatibility
spike다. legacy `beginOrcaExecutionIntent`가 만든 prepare-shaped payload와
기존 resume authority를 담은 resume-shaped payload를 새 codec으로
decode/encode한 뒤 다음을 증명한다.

- record와 external intent bucket bytes가 byte-for-byte 동일하다.
- marker validation과 error classification이 동일하다.
- 기존 `reconcileCanonicalOrcaIntent`가 두 payload shape의 새 codec bytes를
  canonicalize하고 receipt까지 성공적으로 복구한다.
- spike가 core prepare orchestration wrapper를 호출하지 않는다.

이 gate가 green인 뒤 injected clock과 operation ID를 사용한 deterministic
service oracle을 추가한다. legacy와 new vertical을 격리 state root에서 같은
scripted dependencies로 실행해 다음을 비교한다.

- public result JSON과 exact error
- schema v1 record bytes와 holder index bytes
- external intent bytes와 stage별 ordered gateway trace
- preview/idempotency/mode mismatch/writerless/root collision
- direct success, access denial, provision/persistence failure와 artifact failure
- Orca auto fallback, explicit unavailable, branch collision, parent worktree,
  stage success/failure/unknown/reconcile compatibility

production cutover는 다음 순서를 지킨다.

1. `ExecutionPrepareHandler`와 handler-missing fail-closed test를 추가한다.
2. prepare-shaped와 resume-shaped payload를 모두 포함한 shared intent
   codec/reconcile canonicalize·receipt recovery spike를 통과시킨다.
3. direct prepare를 service 뒤로 옮기고 record/index/hook parity를 통과시킨다.
4. Orca stage를 shared codec과 service 뒤로 옮기고 existing reconcile recovery를
   다시 통과시킨다.
5. CLI/MCP concrete dependencies를 issueopsapp으로 옮긴다.
6. router를 handler-only로 전환하고 legacy production caller를 0으로 만든다.

추가 gate는 다음과 같다.

- domain/application/inbound/outbound unit와 focused race
- production prepare handler-only/caller-zero architecture fitness
- CLI/MCP request/result와 handler-missing fail-closed
- Codex/Claude hook canonical workdir/owner mutation tests
- CLI/MCP response golden과 contract golden
- scoped vet, build, full unit/race, deterministic self-verify와 final GitHub CI

## Rollback

schema와 data migration이 없으므로 child merge commit을 revert한다. legacy
compatibility facade와 stable raw bytes는 revert 후에도 현재 records를 읽을 수
있다. mode별 fallback, silent data repair와 cleanup side effect는 rollback 경로로
사용하지 않는다.
