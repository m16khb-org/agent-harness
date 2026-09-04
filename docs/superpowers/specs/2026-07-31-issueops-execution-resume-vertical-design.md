# IssueOps Execution Resume Vertical 설계

## 상태

- Parent: GitHub #117
- Child: GitHub #193
- 분류: `[s] sequential`, Wave 6
- 선행 작업: #192 execution lease reseed vertical
- 현재 단계: 설계 승인, 구현 계획 작성 전 명세 검토

이 문서는 `execution resume` 한 capability를 contract, domain, application,
inbound, outbound 경계까지 수직 이전하는 설계 계약이다. 구현 작업 목록이나
파일별 순서는 이 명세가 승인된 뒤 별도 계획에서 확정한다.

## 배경

현재 `execution resume`의 동작 자체는 안전한 외부 intent와 reconcile 계약을
갖고 있다. 그러나 production entry인
`internal/core/issueops/execution_api.go`는 resume 요청을
`ResumeExecutionWithDependencies`로 직접 보내고, 다음 책임이
`internal/core/issueops/execution_resume.go`와
`internal/core/issueops/execution_orca_intent.go`에 함께 남아 있다.

- 공개 request/result 조립
- mutation authority, actor, CWD, generation과 lease 상태 검증
- claim token, context packet, owner prompt 읽기와 digest 검증
- Orca owner inventory 판정
- resume external intent 생성과 SQLite 원자 저장
- terminal, task, dispatch 단계 실행과 receipt CAS
- ambiguous outcome 보존과 legacy reconcile 연결

#196 release, #197 claim, #192 reseed는 같은 IssueOps lease capability를
domain/application/contract/inbound/outbound 경계로 이전했다. resume만 legacy
core가 직접 orchestration하면 dependency direction과 composition root 계약이
다시 갈라진다.

#192가 만든 holderless claimable generation, sealed claim token, context packet,
owner prompt와 exact resume next-command가 resume의 hard input이다. 그러므로
#193은 #192 뒤에 실행하는 sequential child다.

## 문제 정의

이번 작업의 문제는 resume 기능 부족이 아니라 ownership 불일치다.

현재 구현은 다음 질문에 답하는 순수 정책과 외부 효과를 한 함수에서 처리한다.

1. 현재 lease가 resume 가능한가?
2. 이전 Orca owner inventory를 보고 멱등 성공, terminal 재사용, 새 owner
   생성, 안전 거부 중 무엇을 선택해야 하는가?
3. 선택한 외부 효과 전에 어떤 intent bytes를 원자적으로 저장해야 하는가?
4. 각 receipt를 어떤 CAS 조건으로 반영해야 하는가?
5. 모호한 결과를 legacy reconcile이 어떻게 이어받아야 하는가?

이 결합을 유지하면 resume 정책을 SQLite, filesystem, Orca adapter 없이
검증하기 어렵고, 다른 inbound surface가 같은 정책을 우회할 가능성이 남는다.

## 설계 목표

- resume request/result의 공개 의미와 CLI/MCP JSON을 유지한다.
- holderless claimable lease와 Orca owner inventory에 대한 판단을 순수
  domain transition으로 분리한다.
- application service가 use case 순서와 consumer-owned port를 소유한다.
- record/CAS, sealed artifact, Orca inventory/mutation, pending intent 저장을
  outbound adapter 뒤에 둔다.
- `ExecutionActionResume` production entry를 새 inbound handler에 연결한다.
- handler가 없으면 legacy path로 조용히 후퇴하지 않고 fail-closed한다.
- 현재 external intent payload, marker, pending kind와 stage bytes를 바꾸지
  않아 legacy `execution reconcile`이 진행 중 intent를 계속 처리하게 한다.
- current/new differential로 request, result, persisted bytes, error
  classification의 동등성을 증명한다.
- production caller가 0으로 확인된 resume 전용 legacy orchestration만 제거한다.

## 비목표

- `execution reconcile` vertical migration
- `execution prepare` vertical migration
- replace preview, revoke, finalize migration
- 새 CLI/MCP flag, action 또는 response field 추가
- Orca CLI나 Orca runtime 동작 변경
- SQLite/state 전체 horizontal refactor
- external intent schema 또는 IssueOps schema version 변경
- 완료된 이전 Orca terminal/task 자동 삭제
- caller-zero evidence 없는 core, port, package 제거

## 보존할 공개 계약

### 요청

다음 입력 의미를 그대로 유지한다.

```text
id
expected_generation
actor
cwd
confirm
```

- `confirm`이 없으면 외부 mutation 전에 거부한다.
- actor는 현재 native identity와 process ancestry 계약을 사용한다.
- `expected_generation`은 exact generation CAS다.
- CWD는 canonical worktree와 같아야 한다.
- CLI와 MCP는 기존 `ExecutionActionRequest`를 계속 사용한다.

### 결과

성공 결과의 다음 필드와 값 의미를 유지한다.

```text
ok
id
execution
claim_token_path
issue_body_sha256
context_packet_path
context_packet_sha256
owner_prompt_path
owner_prompt_sha256
next_command
```

claim token 원문은 어떤 결과에도 포함하지 않는다. `next_command`는 현재
generation의 token path와 sealed digest를 포함하는 exact claim 명령이다.

### 오류

외부 호출 전 deny, persistence failure, inventory ambiguity, invoked 여부가
불명확한 mutation failure를 구분한다. inbound adapter는 typed domain 또는
application failure를 현재 공개 오류 문자열과 분류로 변환한다.

## 목표 아키텍처

| 경계 | 책임 | 의존하면 안 되는 것 |
| --- | --- | --- |
| contract | transport-neutral resume request/receipt, stable execution projection | SQLite, filesystem, Orca, CLI, MCP |
| domain | lease와 owner inventory를 입력으로 멱등 성공·재사용·새 생성·거부 결정 | context, JSON, file path, process 실행, database |
| application | fence 안에서 snapshot, artifact, inventory를 읽고 intent/stage 순서를 조정 | concrete SQLite, concrete Orca CLI |
| inbound adapter | legacy core request를 application request로, application receipt/error를 legacy result/error로 변환 | 정책 재구현 |
| outbound adapter | record/CAS, sealed file read, owner inventory, intent persistence, Orca mutation | CLI/MCP presentation |
| composition root | production service와 adapter를 한 번 조립해 CLI/MCP에 같은 handler 제공 | package-global mutable dependency cache |

## Contract 경계

`internal/contract/issueopslease`가 resume의 transport-neutral request와 receipt를
소유한다. 기존 stable execution projection과 actor/process receipt는 재사용한다.

contract는 다음을 포함한다.

- resume ID와 expected generation
- canonical CWD 입력
- sealed artifact 경로와 digest receipt
- 최종 execution projection
- exact claim next-command를 만들 수 있는 비밀이 아닌 값

contract는 external intent 내부 JSON 구조를 새 공개 DTO로 승격하지 않는다.
pending intent bytes는 persistence compatibility contract이며 CLI/MCP 공개
schema와는 별개다.

기존 `internal/core/issueops` request/result는 호스트 표면 호환성을 위한 facade로
남길 수 있다. production 정책의 source of truth가 되어서는 안 된다.

## Domain 결정

domain은 검증된 lease snapshot과 정규화된 owner inventory만 입력으로 받는다.
filesystem digest 검증과 Orca 조회는 application/outbound 책임이다.

### 입력 상태

resume 가능한 기본 상태는 다음과 같다.

```text
mode == orca
pending == nil
lease.status == claimable
lease.holder == nil
lease.generation == expected_generation
claim_token_sha256 is valid
canonical_cwd == true
binding exists
```

### owner inventory 결정표

| 관측 | generation 관계 | 결정 |
| --- | --- | --- |
| task live, terminal live | same | 기존 binding을 반환하는 멱등 성공 |
| task live, terminal absent | any | 모순 상태로 거부 |
| task live, terminal live | other/legacy | 이전 owner가 live이므로 거부 |
| task absent, terminal live | terminal identity same | terminal 재사용, task → dispatch 계획 |
| task absent, terminal live | terminal identity changed/empty | identity drift로 거부 |
| task absent, terminal absent | settled current/previous/legacy binding | terminal → task → dispatch 계획 |
| runtime identity incompatible | any | runtime rollover fence로 거부 |

domain 결과는 외부 도구 명령이 아니라 다음 의미만 표현한다.

- existing binding을 멱등 반환
- 기존 terminal을 재사용해 task 단계부터 시작
- 새 terminal부터 시작
- typed deny code와 원인

## Application 흐름

### 1. fence와 snapshot

application service는 lifecycle 단위 fence 안에서 실행한다.

1. native actor와 process ancestry를 검증한다.
2. `confirm`을 검증한다.
3. expected generation의 exact record snapshot을 읽는다.
4. mode, pending, holderless claimable lease, canonical CWD를 검증한다.

외부 inventory나 mutation은 이 단계가 성공하기 전에 호출하지 않는다.

### 2. sealed artifact 검증

sealed artifact port를 통해 다음을 읽고 검증한다.

- deterministic current-generation claim token path와 token hash
- context packet regular-file, size, permission과 digest
- lifecycle, source/worktree root, branch, base, generation, issue identity
- artifact manifest의 각 file digest
- owner host/model/effort와 Orca binding 일치
- owner prompt bytes와 context packet에서 재렌더한 expected prompt 일치

이 단계는 read-only다. token을 소비하거나 새 token/artifact를 만들지 않는다.

### 3. owner inventory와 순수 결정

owner inventory port는 현재 binding identity로 Orca 상태를 한 번 관측한다.
application은 관측값을 domain 입력으로 정규화하고 domain 결정을 받는다.

- 멱등 성공이면 mutation과 pending 저장 없이 현재 binding을 반환한다.
- 거부면 mutation과 pending 저장 없이 typed error를 반환한다.
- terminal 재사용 또는 새 terminal 계획이면 resume intent 시작으로 진행한다.

### 4. intent 선저장

외부 mutation 전에 resume intent payload와 `Execution.Pending`을 하나의
transaction/CAS로 저장한다.

payload는 기존 bytes 계약을 유지한다.

- schema version과 purpose=`resume`
- lifecycle, generation, operation ID와 canonical marker
- workspace/probe/prepared worktree receipt
- claim token, issue body, context packet, owner prompt digest
- prior Orca binding과 exact resume lease
- terminal 재사용 시 기존 terminal receipt
- 최초 stage: 새 terminal이면 terminal, 재사용이면 task

snapshot의 raw bytes, generation, lease, prior binding, pending nil 조건 중 하나라도
달라지면 intent를 저장하지 않는다.

### 5. terminal → task → dispatch

application은 최대 세 stage를 순서대로 조정한다.

```text
terminal
  → task
  → dispatch
  → current Orca binding 교체 및 pending 제거
```

각 stage는 다음 규칙을 공유한다.

1. marker 기반 inventory를 먼저 조회한다.
2. candidate가 하나면 receipt를 검증하고 CAS로 채택한다.
3. authoritative zero이고 `not_invoked_proven`일 때만 mutation을 호출한다.
4. mutation 직전 invocation state를 `unknown`으로 CAS한다.
5. typed `Invoked=false` failure만 absence가 증명된 상태로 기록한다.
6. candidate 복수, non-authoritative zero, transport ambiguity, receipt CAS 실패는
   pending을 보존하고 reconcile을 안내한다.
7. retry 상한 이후에는 자동 재실행하지 않는다.

dispatch receipt 반영 시 시작 때 봉인한 resume lease와 현재 lease가
byte-equivalent여야 한다. 성공하면 current Orca binding만 새
terminal/task/dispatch와 current generation으로 교체하고 pending/failure를
원자적으로 제거한다.

## Outbound port와 adapter

application capability가 필요한 최소 port를 소유한다.

### record와 intent

- exact record snapshot 읽기
- pending nil과 raw snapshot을 조건으로 resume intent 시작
- invocation state와 stage receipt CAS
- final binding 교체와 intent payload 삭제
- 실패 receipt 기록

SQLite adapter는 기존 `record`와 external-intent bucket, JSON encoder,
`TransactionalRecordStore`를 재사용한다. 새 database schema나 generic repository를
도입하지 않는다.

### sealed artifact

- current generation token과 artifact bounded read
- path containment, symlink, mode, size 검증
- digest와 packet identity 검증

filesystem adapter는 기존 안전 helper를 재사용하되 application port 뒤에서만
호출한다.

### Orca

- previous owner inventory 조회
- staged intent inventory 조회
- terminal/task/dispatch mutation 호출

기존 Orca adapter의 전문 동작과 marker 검증을 재사용한다. resume 정책이나
retry 판단을 Orca adapter에 옮기지 않는다.

## Inbound와 composition

`ExecutionActionResume`은 다른 migrated lease action과 같은 방식으로 injected
handler만 호출한다.

- mutation authority를 먼저 fail-closed 검증한다.
- resume handler가 nil이면 전용 `handler unavailable` 오류를 반환한다.
- nil handler에서 `ResumeExecutionWithDependencies`로 fallback하지 않는다.
- inbound adapter는 legacy request/result와 새 application contract만 매핑한다.
- CLI와 MCP는 같은 production resume handler를 composition root에서 받는다.
- 기존 CLI flag parser, MCP action enum과 response fields는 변경하지 않는다.

CLI의 기존 compatibility entry point는 wrapper로 유지할 수 있지만, production
composition은 positional handler 이름을 계속 늘리지 않고 immutable dependency
bundle로 claim, release, reseed, resume handler를 함께 전달한다. package-global
handler cache는 만들지 않는다.

## Reconcile 호환성

reconcile migration은 이번 범위가 아니다. 따라서 새 resume path는 현재
reconcile이 읽는 bytes와 stage vocabulary를 그대로 생성해야 한다.

- pending kind: `owner_launch` 또는 `dispatch`
- purpose: `resume`
- marker identity와 provider/issue metadata
- invocation state와 attempts
- prior binding과 resume lease
- prepared receipt, launch identity, terminal/task receipt

새 path에서 ambiguous outcome이 발생하면 기존
`ReconcileExecutionWithDependencies`가 payload migration 없이 바로 읽고 다음
stage를 진행할 수 있어야 한다.

기존에 이미 pending인 resume intent도 그대로 reconcile할 수 있어야 하며,
#193 배포가 pending bytes를 일괄 변환하거나 직접 수정해서는 안 된다.

## 원자성과 실패 의미

원자성 경계는 SQLite transition과 외부 Orca mutation 사이에 있다. 둘을 하나의
분산 transaction으로 만들 수 없으므로 durable intent와 receipt CAS로
eventual reconciliation을 보장한다.

- intent 저장 실패: 외부 호출 0회
- invocation 직전 CAS 실패: 외부 호출 0회
- typed not-invoked failure: pending 보존, absence 증거 유지
- invoked 여부 불명: pending 보존, 자동 재호출 금지
- receipt CAS 실패: 외부 결과를 추측하지 않고 pending 보존
- final dispatch CAS 성공: binding 교체, pending/failure 제거, intent payload
  삭제가 한 transaction

오류 메시지에 claim token, prompt 본문, context packet 본문을 포함하지 않는다.

## Compatibility와 differential

production 전환 전에 legacy path와 새 path를 같은 fixture로 비교한다.

### 비교 대상

- request mapping
- success result와 exact next-command
- execution projection
- pending/external-intent persisted JSON bytes
- stage별 receipt 이후 persisted bytes
- 공개 오류 문자열과 typed classification
- 외부 inventory/mutation 호출 순서와 횟수

### 필수 시나리오

- same-generation live task 멱등 성공
- live terminal 재사용
- 새 terminal/task/dispatch 성공
- task without terminal 모순 거부
- other-generation live task 거부
- runtime rollover/terminal identity drift 거부
- confirm, actor, CWD, generation, lease 상태 거부
- token, packet, prompt, manifest 변조 거부
- terminal/task/dispatch 각 stage의 not-invoked failure
- terminal/task/dispatch 각 stage의 ambiguous failure
- receipt CAS drift
- legacy reconcile이 새 pending intent를 이어서 완료

differential이 다르면 차이를 호환성 계약 변경으로 승인하지 않는다. 먼저
adapter mapping 또는 persistence encoding 결함으로 취급하고 수정한다.

## 테스트와 검증

로컬은 머신 자원을 보호하기 위해 focused gate만 실행한다.

- domain table test: owner inventory 결정표 전체
- application test: success, deny, ambiguous stage, invocation 횟수
- outbound adapter test: persisted byte parity, CAS, artifact fence
- inbound test: request/result/error mapping과 nil-handler fail-closed
- shared execution API test: resume handler route와 legacy fallback 부재
- CLI/MCP contract golden: 기존 flag, action, response field 불변
- architecture fitness: domain/application의 adapter/core 역방향 import 0
- focused race: resume application과 SQLite adapter의 CAS 경로
- scoped `go vet`과 `go build ./cmd/issueops`

전체 `go test ./...`와 전체 race는 로컬에서 실행하지 않고 원격 CI에 맡긴다.

## Rollback

#193 child PR을 revert한다.

- 공개 CLI/MCP schema가 바뀌지 않으므로 caller rollback이 필요 없다.
- IssueOps schema version과 pending bytes가 바뀌지 않으므로 기존 reconcile이
  계속 동작한다.
- 진행 중 pending intent는 revert 전후 같은 payload로 읽힌다.
- rollback을 위해 SQLite를 직접 수정하거나 Orca owner를 추측해 재생성하지
  않는다.

## 완료 조건

- #193 scope와 이 명세가 GitHub #117의 승인된 다음 child 계약과 일치한다.
- `ExecutionActionResume` production 경로가 새 handler만 사용한다.
- domain owner-inventory 결정이 pure test로 고정된다.
- application/outbound 경계가 기존 durable intent와 CAS semantics를 보존한다.
- CLI/MCP 공개 계약과 persisted bytes differential이 통과한다.
- 새 pending intent와 기존 reconcile의 compatibility test가 통과한다.
- production caller가 0인 resume 전용 legacy orchestration만 제거된다.
- focused 검증과 원격 CI가 통과한다.
- child PR은 parent integration branch만 대상으로 한다.
