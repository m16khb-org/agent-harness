# 486 — `gates check`는 EXPECT가 있어도 종료코드 0을 요구한다 (v2)

v2: brooks `revise` 반영 — `!run.Executed` 분기 삭제(정책 거부에서 이미 반환, 실행 후 항상 true), #485(`484-gates-check-shell-chain`) 위에 스택(같은 함수·같은 문서 절), 실환경 검증에 clean tree 선행조건.

이슈: https://github.com/m16khb/agent-harness/issues/486
브랜치: `486-gates-expect-exit-code` (base: `484-gates-check-shell-chain` @ f10fa175 — PR #485 머지 후 main으로 자동 재타깃)

## 결함

`runGateCheck`(`internal/adapter/gates/check.go:189-205`)는 `EXPECT:`가 있으면 `ExpectMatches`만으로 `passed`를 정하고 `run.ExitCode`를 보지 않는다. `CHECK: go test ./... -count=1` + `EXPECT: ok`는 실패 패키지가 있어도 다른 줄의 `ok  \tpkg`로 met이 된다(`issues/477/gates.md` 8개 전부 이 형태).

## 규칙

`met` = CHECK가 exit 0으로 끝났고, EXPECT가 있으면 출력 줄에 앵커됨. 순서: 타임아웃 → 비영 종료코드 → EXPECT.

## 작업

| # | 작업 | 파일 | 검증 |
|---|---|---|---|
| T1 | `runGateCheck`: 실행 후 `run.TimedOut` → `check timed out: …`; `run.ExitCode != 0` → `exit code N: <evidence>`(실행 시작 실패도 ExitCode=1로 여기 잡힘); 그 뒤에만 EXPECT 매칭(있으면) / 통과(없으면). `!Executed` 분기 없음, 새 문법 없음 | check.go, check_test.go | `go test ./internal/adapter/gates -run ExitCode` |
| T2 | 문서: `skills/issueops/SKILL.md` 게이트 규칙에 "CHECK는 exit 0이어야 한다; 비영 종료가 정상인 도구(grep no-match 등)는 `python3 -c`로 감싸 0으로 끝낸다" 한 줄, `.agent-harness/CAUTIONS.md` 한 줄 | 문서 | validate-skill |
| T3 | 실환경(선행: `git status --short` 비어 있음 — 오염 트리는 477 G8 `go test ./...`를 정당하게 unmet으로 만든다): 임시 원장 `CHECK: python3 -c "print('ok'); raise SystemExit(1)"` + `EXPECT: ok` → unmet(`exit code 1`); 저장소 원장 477(8/8)·480(6/6)·484(6/6) 재실행 met 수 불변 | 수동 | 원장 G3 |
| T4 | 배터리·골든 | | AGENTS.md §9 |

## 비목표

EXPECT 문법·`gates init` 규칙 변경, 실패 기대 게이트용 새 문법.

## 롤백

단일 함수 분기 순서 변경의 revert.
