# 483 — `gates_incomplete` 판정을 현재 사이클의 이슈 원장으로 한정 (v2)

v2: brooks `revise` 반영 — 폴더 비교는 문자열 완전일치(`021`≠`21`), legacy 동번호 문구 정정, T4 삭제, `gates_skipped` 한 줄 집계.

이슈: https://github.com/m16khb/agent-harness/issues/483
브랜치: `483-gates-readiness-own-issue` (base: `origin/main`)

## 결함

`gatesgate.withGatesGate`(`internal/adapter/issueops/gatesgate/gates_gate.go`)는 `DiscoverGateFiles(root)`가 돌려준 원장 전부를 현재 사이클 readiness에 합산한다. #480 이후 원장이 `issues/<n>/gates.md`로 이슈마다 누적되므로 다른 이슈의 미완 원장이 내 `phase --to pr`을 막는다.

## 규칙

linked issue 번호 `n`이 있을 때 판정 대상:

| 원장 | 판정 |
|---|---|
| `.agent-harness/issues/<n>/gates.md` (n 일치) | 판정 |
| `.agent-harness/issues/<다른 숫자>/gates.md` | 제외 → 집계 warning `gates_skipped:<count> (<rel>, …)` 한 줄 |
| `.agent-harness/issues/<비숫자>/gates.md` (예: `_unnumbered`) | 판정 (소유자 불명 = fail-closed) |
| `.agent-harness/gates/<파일>.md` — `legacyLedgerIssueNumber`가 n | 판정(pre-#480 사이클의 유일한 원장일 수 있음; canonical이 함께 있을 때만 duplicate 게이트가 fail-closed) |
| `.agent-harness/gates/<파일>.md` — 다른 번호 | 제외 |
| `.agent-harness/gates/<파일>.md` — 번호 없음, `GATES.md`, `gates/*.md` | 판정 |

`n`이 없으면 전부 판정(현행 유지).

## 작업

| # | 작업 | 파일 | 검증 |
|---|---|---|---|
| T1 | 순수 함수 `scopeLedgers(root string, files []string, issueNumber string) (judged, skipped []string)` — 위 표. 폴더명 비교는 **문자열 완전일치**(`folder == n` → 판정, 전부 숫자면 skip, 그 외 → 판정; int 변환 없음, `021`·`210`은 `21`이 아님), 파일명은 `legacyLedgerIssueNumber` | gates_gate.go | `go test ./internal/adapter/issueops/gatesgate -run Scope` |
| T2 | `withGatesGate`가 `scopeLedgers`로 거른 `judged`만 순회하고 skipped가 있으면 warning 한 줄 `gates_skipped:<count> (<rel1>, <rel2>, …)` 추가 | gates_gate.go, gates_gate_test.go | 다른 이슈 미완 원장(248 unmet) → ready 유지 + 집계 warning; 자기 미완 원장 → 차단; 번호 없는 레코드 → 전부 판정; 폴더 `021`·`210`·`21` 경계 |
| T3 | 문서: `skills/issueops/SKILL.md` gate 문단 한 줄("readiness는 내 이슈 원장과 번호 없는 원장만 본다"), `.agent-harness/CONVENTIONS.md` 레이아웃 절 한 줄 | 문서 | validate-skill, docs checker |
| T5 | 배터리·골든 | | AGENTS.md §9 |

## 비목표

`DiscoverGateFiles`·`gates check` CLI 출력 변경, 원장 형식 변경.

## 롤백

gatesgate 단일 파일 변경의 revert.
