# 이슈 #104 — golden docs 계수 97 동기화

이슈: https://github.com/m16khb/agent-harness/issues/104

## 문제

PR #101·#103이 각각 plan 문서 1건을 커밋하며 golden docs 계수를 동일하게 95→96으로 올렸다. 동일 내용이라 무충돌 머지됐고 실측은 97 — main `TestResponseContractsGolden` red(got 97, want 96). PR #103 본문에 사전 경고된 상호작용.

## 변경 (brooks 약식 리뷰 revise 반영)

`response_contracts.golden.json` docs_count 2곳(661·6820행)·docs_indexed 2곳(4946·7946행)을 **98**로 동기화(단일 확정값 — 본 plan 문서 +1 포함, brooks 실측 검증: HEAD tracked 97 + plan 1건 = 98, 반대 방향 누락 0건, 열린 PR 없음). 코드 변경 없음.

절차 제약(brooks 지적):
1. docs_index는 git INDEX(`git ls-files`) 기준이므로 **plan 문서를 먼저 `git add`** 한 뒤 계측·재생성한다.
2. 갱신은 손편집이 아니라 `go test ./cmd/harness/harnessapp -run Golden -update` 재생성으로 수행(같은 파일의 `"feasibility": 96` 치환 지뢰 회피). diff가 계수 4곳만 건드리는지 검수.
3. 검증은 반드시 워크트리 내부에서(HarnessRoot는 cwd 상향 탐색, `HARNESS_ROOT` env 미설정 확인).
4. 98 성립 조건: 이 사이클이 커밋하는 md는 plan 1건뿐(`.agent-harness/turing/`·`research/` 등 추가 md 커밋 금지 — 각 +1).

## 검증

golden 테스트 green(워크트리 내), harnessapp·contractgolden 패키지 green, diff 4곳 한정 검수.

## 후속

docs_count 스냅샷 방식 재검토(동적 계수의 golden 고정이 3회 연속 함정) — #99 이관.
