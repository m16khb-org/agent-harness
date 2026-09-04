# 이슈 #111 — usage parity 방향 강화 (adapter ⊆ issueops)

이슈: https://github.com/m16khb-org/issueops/issues/111

## 문제 (design-review devil's-advocate revise 반영)

`TestIssueOpsUsageMatchesAdapterUsage`가 한쪽 단독 명령을 무음 skip. design-review 실측으로 계획 전제가 붕괴: 현 표면에서 adapter-단독 키가 **3개 실존** — `remote render-template`·`remote create-issue`·`remote create-child`(`internal/adapter/cli/usage.go:112-114`, 셋 다 실제 디스패치됨·remote 서브-usage에는 존재·최상위 issueops usage에만 누락). #93 devils-advocate와 동일 구멍 3개가 살아 있다. 실측 수치: adapter issueops 라인 44개, issueops usage 명령 라인 57개, commandKey 충돌 0.

## 변경

1. `issueops_usage_parity_test.go`에 adapter→issueops 방향 검사 추가: adapter의 issueops 라인 commandKey가 issueops usage 키 집합에 없으면 누락 명령을 나열하며 실패. 기존 공존-라인 byte 일치 검사·issueops superset 허용 유지.
2. `issueOpsUsageText()`에 누락 3줄 추가 — **adapter 라인과 byte 동일 복사**(공존-라인 일치 검사 때문에 재타이핑 금지).
3. 합성 RED 단위 케이스는 **불채택**(design-review: 진짜 RED 3건이 존재하므로 우발 복잡성 — gold-plating 씨앗).

## TDD 순서

1. RED: 방향 검사 추가 → 실측 3개 명령 누락으로 실패.
2. GREEN: issueOpsUsageText에 3줄 byte 복사 추가 → 통과(공존-라인 일치 검사도 함께 green이어야).
3. 회귀: issueopscli·전체 green.

## 비범위

- 전체 집합 동등 강제(축약 카탈로그 계약 유지). ~~usage 내용 변경~~ → **정정**: 방향 검사가 GREEN이 되려면 누락 3줄 추가가 필수라 범위에 편입(이슈 본문 동기화).
- commandKey의 `[`·`(` 접두 절단 보강은 design-review 권고(현재 무해) — 선택 반영.

## 역할 분담

- 계획·리뷰: Fable 5. 구현안: Opus 5 서브에이전트(holder 적용).
