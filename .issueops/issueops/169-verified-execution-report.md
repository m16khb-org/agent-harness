# #169 검증 보고서 — orca 준비 타이밍

lifecycle: `io-89ae4380c6a3`
issue: https://github.com/m16khb-org/issueops/issues/169
branch: `169-orca-prepare-timing` (base `479f788fdfede97b0252618219c1a75c3c9875a7`)

## 판정

| AC | 판정 | 증거 |
|---|---|---|
| AC-01 `reconcile` 없이 준비 완주 | 충족 | `TestLaunchOwnerWaitsForTheDelayedTabTitle` |
| AC-02 대기 상한 존재 | 충족 | `TestLaunchOwnerStillRefusesWhenTheMarkerNeverAppears` |
| AC-03 재조회 횟수·대기 시간 기록 | 충족 | 오류 메시지의 `attempt N over 12s` |
| AC-04 task·dispatch 지연 확인 | **결론: 지연 없음** | 아래 |
| AC-05 봉인 검증 유지 | 충족 | AC-02와 같은 테스트 |
| AC-06 RED 선행 | 충족 | 아래 |

## RED

```
--- FAIL: TestLaunchOwnerWaitsForTheDelayedTabTitle
    탭 제목 갱신 지연 때문에 정상 준비가 실패하면 orca 모드를 쓸 수 없다:
    terminal_identity_mismatch: Orca owner terminal does not match the sealed intent
--- FAIL: TestLaunchOwnerStillRefusesWhenTheMarkerNeverAppears
    몇 번 다시 읽었는지가 진단에 남아야 한다: "Orca owner terminal does not match the sealed intent"
```

세 번째(`TestLaunchOwnerDoesNotWaitWhenTheMarkerIsAlreadyThere`)는 처음부터 통과했다 — 정상
경로를 느리게 만들지 않는지 고정하는 테스트다.

## 변경

`internal/adapter/orca/execution.go`의 `reconcileCreatedTerminal`이 재조회를 상한까지 짧은
간격으로 반복한다.

| 상수 | 값 | 근거 |
|---|---|---|
| `executionTerminalSettleBudget` | 12s | 실측 3초보다 넉넉하게. 넘으면 종전 실패로 떨어지므로 비대칭 |
| `executionTerminalSettleInterval` | 400ms | orca 조회 부하 제한 |

- **mutation을 반복하지 않는다.** `CreateTerminal`은 한 번이고 반복하는 것은
  `listTerminalsInventory` 조회다. #90의 교훈("실패한 mutation을 재시도하지 말라")은 이 경우가
  아니다. `TestLaunchOwnerWaitsForTheDelayedTabTitle`이 `createCalls == 1`을 단언한다.
- **검증 기준을 바꾸지 않았다.** `validateExecutionIntentTerminal`은 그대로다 — 언제 묻는지만
  바뀐다.
- **모호함과 부재는 대기로 해소되지 않는다.** `executionSoleTerminalByPTY`가 그 둘을 즉시
  반환한다.
- 실패 메시지에 `attempt N over 12s`를 담는다.

`ExecutionProvisioner`에 `terminalSettleBudget`·`terminalSettleInterval` 필드를 두어 테스트가
상한을 밀리초로 줄인다. 0이면 기본 상수를 쓴다 — 그러지 않으면 상한 테스트가 12초를 실제로
기다려 전체 테스트를 느리게 만든다(0.9초로 줄었다).

## AC-04의 결론: task·dispatch에는 지연이 없다

`LaunchOwner`는 순차 실행이다.

```go
terminal, err = p.reconcileCreatedTerminal(...)   // ← 여기서 실패
if err != nil {
    return ..., &port.OrcaError{Code: "terminal_identity_mismatch", Invoked: true}
}
task, err := p.client.CreateTask(...)             // 도달 못 함
dispatch, err := p.client.Dispatch(...)           // 도달 못 함
```

실측에서 `reconcile`이 세 번 필요했던 것은 pending이 `owner_launch` → `task_create` →
`dispatch`로 순차 진행되기 때문이지 각 단계에 타이밍 문제가 있어서가 아니다. 터미널 검증을
통과하면 `CreateTask`·`Dispatch`가 같은 호출 안에서 이어진다 —
`TestLaunchOwnerWaitsForTheDelayedTabTitle`이 `TaskID`·`DispatchID`가 채워지는 것으로 그것을
고정한다.

## 관측한 결함의 출처

orca는 터미널 제목을 에이전트 상태(`✳ Claude Code`)로 덮어쓰고 마커는 **탭** 제목에 둔다.
`StableTabTitle`은 `client.go`의 `stableVisualTabTitles`가 `visualLayouts`에서 채우는데, orca가
그 레이아웃을 갱신하기 전에 읽으면 빈 값이다.

실측(lifecycle `io-b2d0c0f1daf2`, 2026-07-26):

```
08:32:59.706  orca가 워크트리·터미널 생성
08:33:02.607  execution prepare 실패 — terminal_identity_mismatch
```

실패 직후 `orca terminal list --json`을 직접 읽으면 마커가 탭 제목에 정상으로 있다.

`#70`·`#71`이 "mutating Orca E2E 미검증"으로 남긴 공백이 이것이었다.

## 검증

```
go build ./...                                     성공
go test ./internal/adapter/orca/... -count=1       PASS (0.66s)
go test ./internal/core/issueops/... -count=1      PASS
go test ./... -count=1                             PASS (전 패키지)
```

## 남긴 것

- 실환경 재검증. 이 세션에서 #147을 orca로 준비할 때 그 경로를 밟았으므로, 설치본 갱신 후
  같은 명령이 `reconcile` 없이 완주하는지 확인할 수 있다
