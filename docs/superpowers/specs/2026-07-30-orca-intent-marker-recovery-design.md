# IssueOps Orca Intent Marker 및 복구 설계

## 배경

`execution resume`이 GitLab IssueOps 실행을 재개할 때 durable
`BranchPrepare`에는 provider와 issue IID가 있었지만, resume 전용 marker는
`lifecycle`, `generation`, `operation`만 직접 조합했다. Orca adapter는 GitLab
외부 mutation 전에 marker의 `provider=gitlab`과 exact issue IID를 검증하므로
요청을 정상적으로 거부했다. 그 결과 lease는 holderless claimable 상태를
유지했지만 `owner_launch` pending intent가 남았다.

이 결함은 특정 lifecycle이나 GitLab issue의 문제가 아니다. prepare와 resume이
각자 marker 문자열을 만들고, durable intent가 marker 계약을 만족하는지 저장
전에 검증하지 않으며, 이미 저장된 `not_invoked` intent를 안전하게 승격하는
공통 경로가 없어서 생긴 구조적 결함이다.

이 설계는
`2026-07-30-issueops-orca-owner-resume-design.md`의 외부 intent/reconcile 계약을
보강한다. 현재 pending record를 직접 수정하거나 guard를 완화하지 않는다.

## 확인된 구현 근거

- `internal/core/issueops/execution_resume.go`의 resume 경로는 marker를 직접
  조합한 뒤 probe의 provider/issue를 나중에 채운다.
- `internal/core/issueops/execution_prepare.go`에는 GitLab provider/IID suffix를
  붙이는 별도 helper가 있어 prepare와 resume의 생성 규칙이 갈라져 있다.
- `internal/adapter/orca/execution.go`는 외부 mutation 전에 GitLab marker의
  exact provider/IID를 검증한다.
- `internal/core/issueops/execution_resume_test.go`에는 GitLab resume marker를
  고정하는 회귀 테스트가 없다.
- 실제 generation 2 pending payload는 record/probe에 GitLab identity를
  유지하면서 marker에서만 그 suffix가 빠졌고, adapter가
  `external_operation_ambiguous`로 거부했다.
- 같은 payload를 SQLite read-only로 확인한 결과 invocation state는
  `not_invoked_proven`, invocation attempts는 0이다. 따라서 현재 결함은
  오류 문자열을 해석하지 않고 durable non-invocation receipt만으로 복구할 수
  있다.

따라서 guard나 Orca의 문제로 분류하지 않는다. 생성 규칙의 분산, 저장 전 봉인
검증 부재, preflight invocation receipt의 비정형성이 함께 드러난 core/adapter
계약 결함으로 분류한다.

## 목표

- 모든 IssueOps Orca intent marker를 하나의 typed renderer가 생성한다.
- provider와 issue identity를 durable IssueOps record에서 한 번만 해석한다.
- marker, probe, payload, pending record가 같은 identity를 봉인한 경우에만
  external intent를 저장한다.
- 외부 mutation이 없었다고 증명된 legacy pending intent는 일반
  `execution reconcile`이 원자적으로 canonical marker로 승격한다.
- adapter의 모든 mutation 전 검증 오류는 typed `Invoked=false`로 core에
  전달해 다시 ambiguous state를 만들지 않는다.
- 외부 mutation 가능성이 있거나 identity가 모호한 intent는 자동으로 고치지
  않고 그대로 fail-closed한다.
- 새 action이 marker 문자열을 별도로 조합하면 source ratchet이 실패한다.
- CLI와 MCP가 같은 core 생성·검증·복구 경로를 사용한다.

## 비목표

- Orca adapter의 GitLab marker 검증 완화
- 특정 lifecycle ID, issue IID, repository 경로 하드코딩
- SQLite 직접 편집이나 runtime memory 기반 우회
- invoked 또는 invocation 결과가 모호한 외부 intent의 추측성 재실행
- 알려지지 않은 legacy 오류 문자열을 근거로 한 invocation state 추론
- remote PR처럼 Orca execution intent가 아닌 marker 형식의 통합
- GitHub/GitLab API나 Orca CLI의 전문 기능 복제

## 검토한 접근

### A. resume marker에 GitLab 필드만 추가

현재 실패는 고치지만 다음 action이 marker를 직접 조합하면 같은 결함이
재발한다. 이미 남은 pending intent도 복구하지 못하므로 제외한다.

### B. Orca adapter가 probe 필드만 신뢰하도록 완화

marker는 외부 inventory와 mutation을 durable operation에 묶는 봉인이다.
probe와 marker가 달라도 통과시키면 다른 issue의 terminal/task를 채택할 수
있으므로 제외한다.

### C. typed marker와 저장 전 검증, 안전한 reconcile migration

모든 생성자를 공통 경계로 모으고 잘못된 intent가 durable state에 들어가기
전에 차단한다. 이미 저장된 intent는 `not_invoked` receipt가 있을 때만 같은
경계로 승격한다. 이 방안을 선택한다.

## 핵심 불변식

하나의 Orca intent에 대해 다음 값은 모두 같아야 한다.

```text
record identity
  == typed marker identity
  == payload lifecycle/generation/operation/purpose
  == payload probe provider/issue
  == Execution.Pending operation/marker
```

canonical marker는 지원되는 모든 provider의 provider와 양의 issue number/IID를
봉인한다. GitLab adapter는 `provider=gitlab`과 exact IID를 marker에서 계속
필수로 검증하고, GitHub adapter는 native linked issue metadata와 canonical
marker identity를 함께 대조한다.

외부 mutation 전에는 canonical marker가 아니면 pending intent 자체를 저장하지
않는다. 저장된 intent의 marker를 바꾸는 것은 외부 호출이 없었다고 durable
payload로 증명되는 경우에만 허용한다. 오류 문자열이나 producer version으로
non-invocation을 추론하지 않는다.

## Typed marker

core에 Orca intent 전용 identity를 둔다.

```text
schema version
purpose
lifecycle ID
generation
operation ID
provider
issue number
```

renderer는 기존 marker의 호환 가능한 field 순서를 책임진다. prepare와 resume
같은 caller는 문자열을 조합하지 않고 typed identity만 전달한다. provider/issue
suffix 생성 규칙도 renderer 한 곳에만 존재한다.

parser는 marker를 다시 typed identity로 읽고 중복 field, unknown duplicate,
잘못된 숫자, provider/issue의 부분 존재를 거부한다. renderer 결과와 parser
결과의 round-trip이 일치해야 canonical marker다.

IssueOps production Go 파일에서 `issueops-v1` marker literal은
renderer 파일 하나에만 존재할 수 있다. AST 기반 source ratchet이 다른
production 파일의 새 literal을 거부한다. 테스트 fixture와 문서는 대상에서
제외한다.

## Authoritative issue identity

provider와 issue number를 action마다 다시 추론하지 않는다. 하나의 helper가
현재 record에서 identity를 만든다.

1. verified `BranchPrepare.Provider`와 `BranchPrepare.IssueURL`을 우선 사용한다.
2. provider는 지원되는 canonical 값으로 normalize한다.
3. issue URL에서 양의 issue number/IID를 추출한다.
4. `record.IssueURL`과 verified branch identity가 충돌하면 거부한다.
5. identity가 필요한 provider인데 값이 완전하지 않으면 외부 intent 저장 전에
   실패한다.

이 helper의 결과가 probe와 marker renderer 양쪽의 입력이다. caller가 probe를
채운 뒤 별도로 marker를 보정하는 순서를 허용하지 않는다.

## 저장 전 봉인 검증

prepare, resume과 이후 추가되는 Orca intent begin 함수는 공통 seal 함수를
호출한다.

1. record에서 authoritative issue identity를 읽는다.
2. payload의 purpose/lifecycle/generation/operation을 typed marker identity에
   복사한다.
3. probe provider/issue를 authoritative identity로 설정한다.
4. canonical marker를 렌더링해 payload marker와 probe marker에 함께 설정한다.
5. payload 구조, marker parse 결과, probe, record identity를 교차 검증한다.
6. 검증이 끝난 payload만 `Execution.Pending`과 같은 SQLite transition으로
   저장한다.

검증 실패 시 external-intent bucket과 `Execution.Pending`은 모두 생성되지
않는다. adapter까지 잘못된 marker가 도달한 뒤 pending을 남기는 현재 실패
형태를 생성 단계에서 차단한다.

## Legacy pending migration

`execution reconcile --confirm`은 Orca inventory를 조회하기 전에 pending
payload를 검사한다.

### 자동 승격 조건

다음 조건을 모두 만족해야 한다.

- payload schema와 record schema가 지원되는 버전이다.
- pending operation ID와 payload operation ID가 같다.
- payload lifecycle, generation, purpose가 현재 record와 일치한다.
- authoritative provider/issue identity를 현재 record에서 완전하게 복원할 수
  있다.
- 현재 marker가 parser가 인정하는 exact legacy 형식이다.
- payload invocation state가 `not_invoked`다.
- failure가 없거나 현재 pending과 같은 operation을 가리킨다.
- record와 payload가 읽은 뒤 변경되지 않았다는 CAS가 성립한다.

조건이 성립하면 canonical marker를 생성하고 다음 값을 한 SQLite transition에서
갱신한다.

- external-intent payload marker
- payload probe marker와 provider/issue
- `Execution.Pending.Marker`

승격 후 일반 reconcile stage를 그대로 실행한다. lifecycle, issue 번호,
provider별 특별 분기는 두지 않는다.

### 자동 승격 금지

다음 상태는 원문을 보존하고 명시적 blocker를 반환한다.

- `invoking` 또는 `invoked` invocation state
- unknown 또는 ambiguous invocation state
- marker가 exact legacy 형식이 아니라 임의로 변조된 경우
- provider/issue identity가 없거나 record와 충돌하는 경우
- operation, generation, purpose 또는 pending kind가 다른 경우
- CAS 실패

이 경우 reconcile은 marker를 고치거나 Orca mutation을 재실행하지 않는다.

## Hook 및 adapter 계약

PreToolUse guard와 Orca adapter의 fail-closed 검증은 유지한다. 이번 결함은
guard가 잘못 막은 것이 아니라 core가 잘못 봉인한 marker를 정확히 차단한
사례다.

- hook은 canonical command와 lifecycle authority만 허용한다.
- adapter는 GitLab marker의 exact provider/IID를 계속 요구한다.
- adapter가 client/runner 호출 전에 거부하는 모든 검증 오류는
  `OrcaError{Invoked:false}` 같은 typed receipt로 반환한다.
- core 저장 전 검증과 source ratchet이 잘못된 intent가 adapter에 도달하는 것을
  예방한다.
- core는 typed preflight rejection을 `not_invoked`로 저장하고, unknown
  transport 결과와 구분한다.

기존 payload가 generic `external_operation_ambiguous` failure를 함께 기록했더라도
failure 문자열은 승격 근거로 사용하지 않는다. durable invocation state가
`not_invoked`가 아니면 exact legacy marker여도 자동 승격하지 않는다.

## 오류 및 관측성

오류는 다음 경계를 구분한다.

- `intent_marker_invalid`: 신규 intent를 저장하기 전 marker 계약 실패
- `intent_identity_mismatch`: record, payload, probe identity 충돌
- `intent_preflight_rejected`: adapter가 외부 호출 전에 거부해 재시도 가능한 상태
- `legacy_intent_upgraded`: 외부 조회 전에 legacy pending을 원자적으로 승격
- `legacy_intent_upgrade_unsafe`: invocation 또는 identity 증거가 부족해 보존

public 결과에는 secret이나 token 원문을 포함하지 않는다. lifecycle,
generation, operation ID, provider, issue number와 migration 여부만 진단
필드로 노출할 수 있다.

## 테스트 전략

전체 suite 대신 다음 집중 gate를 사용한다.

### Marker contract

- GitHub/GitLab provider와 prepare/resume purpose matrix
- render/parse round-trip
- duplicate/partial/invalid identity 거부
- production marker literal 단일 위치 source ratchet

### Intent persistence

- canonical marker만 pending/payload에 저장
- record/probe/marker mismatch에서 durable mutation 0회
- GitLab provider/IID 누락이 adapter 호출 전에 실패
- adapter preflight rejection이 typed `Invoked=false`이고 client/runner
  호출이 0회

### Reconcile migration

- 임의의 lifecycle/issue fixture에서 `not_invoked` legacy resume marker를 승격
- 현재 producer defect와 같은 `not_invoked` payload를 오류 문자열과 무관하게
  승격
- failure message나 marker가 한 글자라도 다른 unknown payload는 승격 거부
- 승격과 payload/pending marker 갱신의 atomicity
- 승격 후 기존 reconcile stage 실행
- already-canonical intent의 멱등성
- invoked/ambiguous, identity mismatch, CAS drift에서 mutation 0회
- adapter preflight rejection의 mutation 0회 회귀 증거

### Adapter 및 host parity

- GitLab exact marker 허용과 wrong provider/IID 거부
- GitHub native linked metadata 계약 유지
- CLI/MCP가 동일한 reconcile result와 error code 반환
- PreToolUse가 unsafe mutation을 계속 차단

## 배포 및 현재 pending 복구

1. focused core/adapter/CLI/MCP/guard tests와 build를 통과한다.
2. atomic commit/push 후 `io update`로 Codex와 Claude native 설치를 함께
   갱신한다.
3. daemon과 MCP가 새 binary를 사용함을 readback한다.
4. 현재 pending lifecycle에서 `execution reconcile --preview`로 operation과
   invocation state를 다시 읽는다.
5. 자동 승격 조건이 성립할 때만 `execution reconcile --confirm`을 실행한다.
6. 새 marker, pending 해소, Orca task/dispatch, claim 가능 상태를 모두
   readback한 뒤 원래 작업을 재개한다.

현재 record가 자동 승격 조건을 만족하지 않으면 DB를 직접 수정하지 않는다.
외부 inventory와 durable invocation 증거에 맞는 별도 recovery action을
설계할 때까지 fail-closed 상태를 유지한다.

## 완료 기준

- production Orca intent marker 생성 경로가 typed renderer 하나뿐이다.
- 신규 invalid marker는 durable pending을 만들지 않는다.
- 모든 `not_invoked` exact legacy intent가 provider나 issue 번호 하드코딩 없이
  reconcile로 복구된다.
- invoked 또는 ambiguous intent는 자동으로 변경되지 않는다.
- GitHub/GitLab과 CLI/MCP/hook의 focused contract가 모두 통과한다.
- 설치 갱신 후 현재 pending lifecycle이 안전한 reconcile 경로로 복구되거나,
  자동 복구 불가 이유가 durable 증거로 정확히 보고된다.
