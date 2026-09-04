# 이슈 #97 — orca 비영 종료 시 typed 오류 코드 유실 복원 (cleanup finish 수렴 회복)

이슈: https://github.com/m16khb-org/issueops/issues/97

## 문제 (design-review devil's-advocate 진단 반영)

`cleanup finish`의 orca_remove 무한 실패의 근본 원인은 core가 아니라 **orca 어댑터의 오류 형상 유실**이다:

- 배선층 `issueOpsFeedbackCleanupDeps().RemoveOrcaWorktree`(`cmd/issueops/issueopscli/issueops.go:140-155`)에는 이미 "이미 없음 = 성공" 멱등 정규화가 존재한다(typed `OrcaError.Code`에 `not_found` 포함 시 nil).
- 그러나 orca CLI가 **비영 종료**하면 `runner.Run`이 `OrcaError{Code:"command_failed", Detail:"stdout: {...selector_not_found...}"}`를 반환하고(`internal/adapter/orca/runner.go:89`), envelope의 typed 코드를 복원하는 `decodeResult`(`decode.go`)는 실행되지 않는다(`client.go runJSON:729-731`이 즉시 반환).
- 결과: 배선층 정규화가 `command_failed`와 불일치 → finish가 영원히 수렴 불가. 실측 io-4bd36030750e 2회 재현, 오류 전문이 이 경로와 정확히 일치.

## 변경

1. `internal/adapter/orca/client.go` `runJSON`: runner 오류가 `command_failed`이고 stdout에 정상 `ok:false` envelope이 있으면 `decodeResult`로 디코드해 **typed `OrcaError`(예: `selector_not_found`)를 복원**해 반환한다. envelope이 없거나 ok:true(모순)면 원래 오류 유지. core(`issueops_cleanup_finish.go`)는 0줄 변경 — 멱등 정규화 책임은 dep 계약 주석(:34-35)대로 배선층에 유지.
2. `cmd/issueops/issueopscli/issueops.go`: RemoveOrcaWorktree 정규화 클로저를 테스트 가능한 명명 함수로 추출(동작 불변), selector_not_found·기타 오류 두 경로 단위 테스트 추가(design-review 지적: 이 정규화는 테스트 0건이었다).

## TDD 순서

1. RED(어댑터): 비영 종료 + `ok:false` envelope(selector_not_found)을 내는 fake runner로 `RemoveWorktree`가 typed `OrcaError{Code:"selector_not_found"}`를 반환함을 단언 → 현재 `command_failed`라 실패.
2. RED(배선): 추출한 정규화 함수가 typed selector_not_found를 nil로, `command_failed`(envelope 복원 전 형상)·기타 코드를 오류로 유지함을 단언.
3. GREEN: runJSON envelope 복원 + 함수 추출.
4. 회귀: orca·issueopscli·issueops 패키지와 전체 모듈 green.
5. 실환경 수렴: 머지 후 io-4bd36030750e finish preview→apply 재실행으로 레코드 삭제 확인(AC-01 실증).

## 비범위

- core 판별 로직 추가(design-review 기각: 계층 위반·책임 이원화), force 경로, orca 재시도 정책.

## 위험

- ok:true envelope + 비영 종료의 모순 케이스 — 원래 command_failed 오류를 유지해 실패 신호를 삼키지 않는다.
- Detail 1024바이트 절단은 envelope 디코드에는 영향 없음(원본 output.Stdout 사용, MaxEnvelopeBytes 2MiB 한도).
