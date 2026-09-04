# 이슈 #93 — issueops list dispatch 등록 누락 수정

이슈: https://github.com/m16khb-org/issueops/issues/93

## 문제

`issueops list`가 usage(`issueOpsUsage`)·commandparse(`internal/core/commandparse/issueops.go:100,713`)·핸들러(`runIssueOpsList`, `cmd/issueops/issueopscli/issueops_subcommands.go:450`) 3면에 존재하지만, dispatch registry `issueOpsSubcommands`(`cmd/issueops/issueopscli/issueops.go:23`)에 `"list"` 항목이 없어 항상 `unknown issueops subcommand "list"`로 실패한다. PR #87의 AC-06 검증이 core 함수(`ListIssueOpsCycles`) 테스트에 그쳐 CLI dispatch 경로 공백이 검출되지 않았다.

## 변경 (design-review devil's-advocate revise 반영)

1. `cmd/issueops/issueopscli/issueops.go` — `issueOpsSubcommands` map에 `"list": runIssueOpsList` 1항목 추가.
2. `cmd/issueops/issueopscli/issueops_cli_support.go` — `issueOpsUsageText()`에 `devils-advocate review` usage 라인 추가 (design-review 스캔에서 발견된 동일 드리프트 클래스의 역방향 실사례: registry·adapter usage에는 있으나 usage 텍스트에 누락).
3. usage↔registry **양방향 집합 정합성 테스트** 추가(필수로 승격, design-review 권고): `issueOpsUsageText()`에서 `issueops ` 다음 첫 토큰을 파싱해 registry 키와 집합 동등성을 단언. 기존 두 테스트(`TestIssueOpsUsageListsNewlyAddedSubcommands`의 하드코딩 fragment, `TestIssueOpsUsageMatchesAdapterUsage`의 `if !ok { continue }` 교집합 검사)가 놓친 클래스를 파생 기반으로 봉쇄.
4. dispatch 경로 테스트 — `runIssueOps(["list","--json"])`가 unknown subcommand 오류 없이 성공(이번 인스턴스의 RED/GREEN 증거).

## TDD 순서

1. RED: dispatch 경로 테스트 + 양방향 정합성 테스트 작성 → 현재 코드에서 둘 다 실패 확인(list dispatch 실패, devils-advocate usage 누락 검출).
2. GREEN: map 항목 1줄 + usage 라인 1줄 추가 → 테스트 통과.
3. 회귀: `go test ./cmd/issueops/issueopscli/... -count=1` 전체 green.

## 수용 기준 매핑

- AC-01: `issueops list --json`/텍스트 dispatch 통과 — 수동 실행 + 테스트.
- AC-02: dispatch 테스트가 registry 누락 재발 시 실패 — RED 단계에서 실패 증거 확보.
- AC-03: 기존 테스트 green — 패키지 전체 테스트.

## 비범위

- `ListIssueOpsCycles` core 로직 변경, usage/commandparse 변경.

## 위험

- dispatch 테스트의 state root 오염 — 기존 CLI 테스트의 임시 state root 패턴(`t.Setenv` 기반)을 재사용해 회피.
