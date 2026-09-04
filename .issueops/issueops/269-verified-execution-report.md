# #269 검증 보고서 — exact `command -v` availability probe

이슈: https://github.com/m16khb-org/issueops/issues/269

## 목표와 경계

active IssueOps write lease가 있어도 비-holder 관찰자가 정확한
`command -v NAME` 한 명령으로 선택 도구의 설치 여부를 확인할 수 있게 한다.
조회 결과를 실행하거나 설치하지 않으며, 기존 read-only sequence에 이 명령을
합성하지 않는다.

허용 name은 첫 바이트가 ASCII 영문자·숫자·underscore이고, 나머지는 같은
문자와 dot·plus·hyphen만 가능하다. quote, backslash, Unicode, path, glob,
expansion, 여러 name, option, assignment, comment, redirect, control operator,
pipeline은 계속 `unsafe_mutation`으로 닫힌다.

## RED

production code 변경 전에 다음 두 테스트가 실패했다.

```text
go test ./internal/core/commandparse -run TestExactReadOnlyShellCommandCorpus -count=1
expected read-only allow: "command -v gocyclo"

go test ./internal/core/lifecycle -run TestExecutionAllowsOnlyStandaloneCommandVObservation -count=1
exact command availability observation was blocked: "command -v go"
```

## 구현

- `ExactReadOnlyShellCommand`의 전역 위험 구문 검사 직후에 standalone raw
  command helper를 호출한다.
- `exactReadOnlySimpleShellCommand`에는 `command` case를 추가하지 않았다.
  따라서 semicolon, newline, `&&`, bounded pipeline 합성 경로로 권한이
  확장되지 않는다.
- parser corpus에 허용 6개와 deny matrix 24개를 추가했다.
- lifecycle에서 current holder가 아닌 `observer-session`의 exact probe만
  allow하고 malformed/composed 형태는 `unsafe_mutation`인지 고정했다.
- 허용 결정 뒤 실제 `sh`가 found를 성공, not-found를 nonzero로 반환하는지
  실행해 hook classification이 shell 결과를 덮어쓰지 않음을 검증했다.

## Brooks 검토

첫 검토는 simple reader 합성 우회를 찾아 `REVISE`였다. 계획을 top-level
raw helper로 바꾸고 quote/backslash/Unicode/sequence/pipeline deny 및
non-holder observer 검증을 추가한 뒤 재검토는 `PASS`였다.

## fresh implementation review

첫 구현 검토는 sequence에서 lookup이 반대편에 놓이는 네 배치의 테스트 누락을
찾아 `REVISE`였다. semicolon, newline, `&&`, pipeline의 좌·우 배치를 parser와
lifecycle 양쪽에 추가하고 focused/race 검증을 통과한 뒤 재검토는 findings 0건
`PASS`였다. reviewer는 code publication gate 충족과 Orca external dogfood AC를
분리해 판정했다.

## hook dogfood

변경 head로 빌드한 hook에 실제 Codex `PreToolUse` JSON을 전달했다.

```text
command -v go
decision: allow

command -v definitely_missing_issueops_tool
decision: allow

command -v go && pwd
decision: block
deny.code: unsafe_mutation
```

found/not-found는 둘 다 guard를 통과하고 실제 shell exit result로 구분된다.
Orca CLI는 설치돼 있고 직접 호출했지만 현재 relay에 owning Orca client가
연결되지 않아 #261 owner 품질 게이트 재개는 외부 연결 복구 뒤 수행한다.
CLI 미설치나 fallback으로 판정하지 않는다.

## 검증

다음 검증은 변경 worktree에서 통과했다.

```text
go test ./internal/core/commandparse ./internal/core/lifecycle -count=1
go test -race ./internal/core/commandparse ./internal/core/lifecycle -count=1
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o /tmp/issueops-269 ./cmd/issueops
git diff --check
```

## 남은 publication 단계

fresh code review, atomic commit/push, draft PR CI, merge, stable native hook
재설치와 cleanup을 수행한다. Orca owning client가 재연결되면 #261의 exact
owner 품질 게이트에서 같은 probe와 fallback을 다시 실행해 이 이슈에 증거를
추가한다.
