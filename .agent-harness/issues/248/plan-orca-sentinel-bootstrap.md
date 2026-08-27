# #248 Orca sentinel bootstrap

이슈: https://github.com/m16khb/agent-harness/issues/248
lifecycle: io-d37060eb87f6
branch: `248-orca-sentinel-bootstrap`

## 문제

`issueops execution prepare --mode auto`가 설치·준비된 Orca를 선택하지 못한다. 현재 Orca의
빈 runtime projection에는 `run_legacy_local` Run이 포함되는데, 하네스 production validation이
이 값만 별도로 거부해 전체 Run inventory probe를 `orchestration_unready`로 판정한다.

## 구현

1. adapter의 Run inventory ID 검증에서 sentinel 전용 거부를 제거한다.
2. IssueOps Orca binding contract에서도 같은 전용 거부를 제거한다.
3. 모든 문법상 유효한 Run ID를 opaque identity로 처리하고, malformed ID 거부는 유지한다.
4. exact IssueOps lifecycle marker와 durable Run ID가 mutable execution identity를 선택하는 기존
   규칙은 바꾸지 않는다.

## 수용 기준

- production 코드가 Orca의 `legacy` 필드를 파싱하거나 sentinel을 특별 취급하지 않는다.
- sentinel이 포함된 실제 형태의 Run inventory를 probe해도 Orca readiness가 통과한다.
- sentinel 자체는 unrelated Run으로 남고 exact lifecycle marker 선택 대상이 되지 않는다.
- malformed Run ID validation과 기존 task inventory 계약은 유지된다.
- focused tests, full tests, race, vet, contract goldens, build가 통과한다.
- 이 bootstrap 변경을 parent에 병합한 뒤 새 #248 child를 `--mode auto`로 준비해 실제 Orca
  worktree, Run, task, dispatch, owner claim 증거를 남긴다.

## 비범위

- Orca가 반환하는 `legacy` projection의 의미 해석 또는 제거
- `run_legacy_local` 이름을 하네스 production 코드에 다시 고착
- direct 실행 경로 제거
