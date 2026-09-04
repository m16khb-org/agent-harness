# #188 검증 보고서 — usage 카탈로그 단일화

이슈: https://github.com/m16khb-org/issueops/issues/188

## 무엇이 문제였나

`issueops` 명령 목록이 두 곳에 손으로 유지됐다. 한쪽 누락은 parity 테스트가 잡지만 **양쪽에 아예
없으면 검사할 대상이 없다.** `execution switch-mode`(`#167`)가 그 구멍으로 살아남았고 `#184`가 손으로
찾아 넣을 때까지 두 텍스트 모두에 없었다.

`#184`는 `TestExecutionSubcommandsAppearInUsageTexts`로 execution 하위 명령만 막았다. 나머지 명령에는
같은 구멍이 그대로였다.

## 무엇을 바꿨나

`internal/adapter/cli`가 순서 있는 전체 카탈로그를 소유한다.

| 함수 | 역할 |
|---|---|
| `IssueOpsUsageLines()` | 카탈로그를 줄 단위로 |
| `IssueOpsUsageKey(line)` | 줄에서 명령 경로 추출. 파싱 규칙의 유일한 소유자 |
| `IssueOpsActorFlagLegend` | 두 출력이 공유하는 축약 정의 |
| `renderIssueOpsUsage(keys)` | 키에 해당하는 줄을 카탈로그 순서로 렌더 |

두 표면은 같은 카탈로그의 서로 다른 투영이 됐다 — `Usage()`는 축약 키로 걸러 렌더하고,
`issueOpsUsageText()`는 전체를 렌더한다.

**레이어가 맞다.** `internal/adapter/cli`는 `fmt`만 import하므로 `cmd/issueops/issueopscli`가 그것을
import해도 순환이 없고, cmd → adapter는 정상 방향이다. `CONVENTIONS.md`의 표가 `adapter/cli`를
"flag/stdout/stderr/exit code" 책임으로 정의하며 usage 텍스트가 그것이다.

## 렌더 위치를 유지해야 했다

`devils-advocate review`와 `implementation-review record`는 문자열 결합 상수로 들어 있어 최상위
usage의 **비-issueops 목록 뒤에** 렌더됐다. 카탈로그 안에서는 중간에 있다.

그래서 축약 키를 두 벌로 나눴다 — `abridgedIssueOpsMainKeys`(본문)와
`abridgedIssueOpsTrailingKeys`(꼬리). `Usage()`가 두 번 렌더한다. 이 분리가 없으면 golden이 바뀐다.

## 새로 생긴 위험과 대책

중복을 없애면 "한쪽에만 있다"는 불가능해진다. 대신 **축약 키가 어느 줄과도 맞지 않으면** 그 명령이
최상위 help에서 조용히 사라진다. 기존 구조에는 이 위험이 없었다 — 줄이 그냥 거기 있었으니까.

`internal/adapter/cli/issueops_catalog_test.go`가 세 가지를 막는다.

1. 축약 키가 카탈로그 줄과 **정확히 하나씩** 대응한다. 0개(오타)도 2개(모호)도 실패다.
2. 최상위 usage가 그 키들만, 그리고 그 키들을 **모두** 렌더한다. 필터가 깨지면 실패다.
3. 축약은 카탈로그의 부분집합이다. 최상위에만 있는 명령은 새로운 중복이다.

## 파싱 규칙도 하나로

`issueopscli`의 테스트 헬퍼 `usageCommandKey`가 자기 파싱을 갖고 있었다. `#184`에서 그 규칙의 결함을
고쳤는데(대괄호를 끊지 않아 검사가 명령을 실행했다), 규칙이 두 곳에 있으면 그 둘이 다시 어긋난다.
이제 카탈로그의 `IssueOpsUsageKey`에 위임한다.

## 검증

**`usage.golden.txt`가 바뀌지 않았다** — 이것이 렌더 동등성의 증거이자 이 리팩터의 핵심 검증이다.
사용자가 보는 help 출력이 문자 단위로 같다.

`go test ./... -count=1` 전 패키지 PASS. `#184`가 추가한 flag parity 테스트와 기존 두 usage parity
테스트가 모두 통과한다 — 후자는 이제 구조로 보장되지만 계약 문서로 남겼다.

## 남는 것

- **축약 카탈로그의 선정 기준이 코드에 명시되지 않는다.** 어느 명령을 최상위에 노출할지는 현재 키
  목록이 유일한 근거다. 이 이슈는 그 선정을 그대로 옮겼고 기준을 다시 정하는 것은 별건이다.
- **`issueopscli`가 `internal/adapter/cli`를 프로덕션에서 import하게 됐다.** 정상 방향이고 adapter가
  `fmt`만 import하므로 비용은 작지만 의존이 하나 늘었다.
