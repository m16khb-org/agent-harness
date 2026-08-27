# 이슈 #109 — golden docs 계수 placeholder 정규화

이슈: https://github.com/m16khb/agent-harness/issues/109

## 문제

`response_contracts.golden.json`이 `docs_count`/`docs_indexed` 절대값을 고정해 tracked md 증감마다 수동 동기화 필요 — 하루 4회 함정 실증(#94 회귀, #95·#97·#100·#102·#107 선반영, #104 두-PR 상호작용). 절대 계수는 계약이 아니다.

## 변경

1. `response_contract_golden_test.go`에 스냅샷 후처리 정규화 함수 추가: `map[string]any`/`[]any` 트리를 재귀로 걸어 키가 `docs_count` 또는 `docs_indexed`인 수치 값을 문자열 `"$DOCS_COUNT"`로 치환. 경로 치환(replacements)과 같은 계층에서 `assertJSONGolden` 직전에 적용.
2. golden `-update` 1회 재생성 — diff는 계수 4곳의 placeholder 전환뿐이어야 함(검수).
3. 테스트: 정규화 함수 단위 테스트(두 키 치환·무관 키 보존·중첩/배열 통과) + 실측 golden에 수치 계수가 남아 있지 않음을 단언.

## TDD 순서

1. RED: 정규화 함수 단위 테스트 작성(함수 미존재로 컴파일 실패 → 스텁 후 동작 실패) 및 "golden 내 docs 계수는 placeholder여야 한다" 단언이 현 golden(99)에서 실패.
2. GREEN: 함수 구현·배선·golden 재생성.
3. 회귀: harnessapp·contractgolden·전체 green. **이 사이클의 plan 문서 커밋이 golden에 영향을 주지 않음을 확인하는 것 자체가 AC-01의 실증**(선반영 커밋 불필요해짐).

## 비범위

- docs_index 기능, 다른 필드 정규화 확대.

## 역할 분담

- 계획·리뷰: Fable 5. 구현안: Opus 5 서브에이전트(holder 적용).
