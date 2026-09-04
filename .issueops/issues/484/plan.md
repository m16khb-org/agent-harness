# 484 — `gates check`의 셸 연산자 체인 거짓 met 차단 (v2)

v2: design-review `revise` 반영 — 새 판별 함수 대신 기존 `shelltoken.HasUnquotedControlOperator`(원문·따옴표 인식) 재사용, `-c "…;…"` 원장 회귀 확인 명시, `EXPECT` 존재 시 종료코드 무시 잔여 위험 기록.

이슈: https://github.com/m16khb-org/issueops/issues/484
브랜치: `484-gates-check-shell-chain` (base: `origin/main`)

## 결함

`runGateCheck`(`internal/adapter/gates/check.go:156`)는 CHECK를 `shelltoken.SplitCommandTokens`로 argv 분해해 정책 평가 후 실행한다. `&&`·`||`·`;`·`|`는 토큰으로 남아 첫 명령의 인자가 되고, `ExpectMatches`(`internal/domain/gates/evaluate.go:24`)는 stdout+stderr 전체 부분일치라 `docs-ok: error: …` 같은 에러 줄로 게이트가 `met`이 된다.

## 작업

| # | 작업 | 파일 | 검증 |
|---|---|---|---|
| T1 | `runGateCheck`가 `SplitCommandTokens` 전에 원문 `gate.CheckCmd`에 `shelltoken.HasUnquotedControlOperator`(따옴표 밖 `; & \| 개행`; lifecycle 가드와 같은 함수)를 적용해 참이면 `check_error: "CHECK contains shell syntax that argv execution does not honor"`로 unchecked 반환(정책 평가·실행 없음). 새 함수 없음. `a&&b`·`2>&1`은 거부, `python3 -c "x; y"`는 통과 | check.go, check_test.go | `go test ./internal/domain/gates ./internal/adapter/gates` |
| T2 | `ExpectMatches` 리터럴 규칙을 줄 단위로: 어느 한 줄(trim)이 EXPECT와 같거나 EXPECT 뒤에 공백/탭이 오면 일치. 정규식 `/…/` 형식은 불변 | evaluate.go, evaluate_test.go | 기존 EXPECT(`ok`, `docs-ok`, `hello-gates`, `3/3 tiers ok`) 통과, `docs-ok: error` 불일치 |
| T3 | `gates init` `renderGateSpec`: CHECK 세그먼트에 같은 함수가 참이면 오류(`CHECK must be one argv command; shell syntax is not executed`). `\|`는 세그먼트 구분자라 이미 거부되며 따옴표 안 `\|`도 init에서는 쓸 수 없음을 usage에만 명시 | init.go, init_test.go | `go test ./internal/adapter/gates -run Init` |
| T4 | 문서: `skills/issueops/SKILL.md` 게이트 규칙 한 줄(단일 명령·복합 검사는 스크립트/python -c 래퍼·EXPECT는 줄 앞 토큰), `.issueops/CAUTIONS.md` 한 줄 | 문서 | validate-skill, docs checker |
| T5 | 실환경: 임시 원장(체인 CHECK, `docs-ok: error` 출력)으로 `./bin/issueops gates check` → unchecked 확인; 저장소 원장 `issues/477`(8 met), `issues/480`(6 met — G3·G4·G6은 `python3 -c "…;…"` 형태라 따옴표 안 `;`가 거부되지 않아야 함) 판정 불변 | 수동 | 게이트 원장 G4 |
| T6 | 골든·배터리 | | AGENTS.md §9 |

## 잔여 위험(범위 밖, 기록)

EXPECT가 있으면 `runGateCheck`(check.go:189-201)는 종료코드를 보지 않는다. `EXPECT: ok`인 `go test ./...`는 실패 패키지가 있어도 다른 줄의 `ok  \tpkg`로 met이 된다(`issues/477` 8개 게이트 전부 해당). 줄 앵커로는 막히지 않으며, 별도 이슈(EXPECT + 종료코드 0 요구)로 다룬다.

## 비목표

`sh -c` 실행 지원, 명령 정책 카탈로그 변경, 원장 형식 변경.

## 롤백

두 순수 함수 변경과 한 가드뿐이라 커밋 revert로 즉시 원상복구.
