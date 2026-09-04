# #188 구현 계획 — usage 카탈로그 단일화

이슈: https://github.com/m16khb-org/issueops/issues/188

## 문제

`issueops` 명령 목록이 두 곳에 손으로 유지된다. 한 곳만 고치면 parity 테스트가 잡지만, **양쪽에
아예 없으면 검사할 대상이 없다.** `execution switch-mode`(`#167`)가 그 구멍으로 살아남았고
`#184`가 손으로 찾아 넣을 때까지 두 텍스트 모두에 없었다.

## 설계

`internal/adapter/cli`가 순서 있는 전체 카탈로그를 소유한다.

- `IssueOpsUsageLines()` — 전체 목록(선행 두 칸 포함)
- 축약 키 집합 — 최상위 help에 노출할 명령 경로
- `Usage()` — 축약 키로 걸러 렌더
- `issueOpsUsageText()` — 전체를 렌더

`internal/adapter/cli`는 `fmt`만 import하므로 `cmd/issueops/issueopscli`가 그것을 import해도 순환이
없다. `issueops_usage_parity_test.go`가 이미 그 방향으로 import한다.

레이어도 맞다 — `CONVENTIONS.md`의 표가 `adapter/cli`를 "flag/stdout/stderr/exit code" 책임으로
정의하고, usage 텍스트가 그것이다.

## 렌더 동등성

**결과를 바꾸지 않는다.** `usage.golden.txt`가 그대로여야 한다 — 그것이 이 리팩터의 검증이다.

`devils-advocate review`와 `implementation-review record`가 문자열 결합 상수
(`issueOpsDevilsAdvocateUsage`)로 들어 있어 렌더 위치가 목록 끝이다. 카탈로그로 옮기면서 그 위치를
유지해야 golden이 같다.

## 새로 생기는 위험과 대책

중복을 없애면 "한쪽에만 있다"는 불가능해진다. 대신 **축약 키가 어느 카탈로그 줄과도 맞지 않으면**
그 명령이 최상위 help에서 조용히 사라진다.

키와 줄의 일대일 대응을 테스트로 고정한다 — 맞지 않는 키도, 두 줄에 맞는 키도 실패다.

## RED

현재 구조에서는 "한 곳에서만 줄을 지워도 다른 곳이 남는다"를 실증할 수 있다. 카탈로그가 하나면
그것이 불가능해진다. 그리고 축약 키 대응 테스트가 새 위험을 막는다.

## 축약이 생략하는 명령 (실측)

```
ai-slop-clean record   decision add        domain-review record   feedback resolve
link-related           phase               plan-prep record       prune
record-routing         regress             remote close-issue     remote reflect-completion
remote verify-artifact remote-score        routing-score
```

`devils-advocate review`와 `implementation-review record`는 상수에 문자열 결합으로 들어 있어
정규식 실측에서 오탐이었다 — 실제로는 축약에 **포함된다.**

## 검증

```
go test ./cmd/issueops/issueopscli/... ./internal/adapter/cli/... -count=1
go test ./... -count=1
```

`usage.golden.txt`가 바뀌지 않는 것이 렌더 동등성의 증거다.

## 비범위

- usage 문자열을 flag 등록에서 **생성**하는 것. 명령마다 형식이 다르다.
- 축약 카탈로그의 선정 기준 변경. 현재 목록을 그대로 옮긴다.
- 최상위 usage의 비-`issueops` 줄.
