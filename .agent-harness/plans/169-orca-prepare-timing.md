# #169 orca 준비 타이밍

이슈: https://github.com/m16khb/agent-harness/issues/169
lifecycle: io-89ae4380c6a3
branch: 169-orca-prepare-timing (base 479f788fdfede97b0252618219c1a75c3c9875a7)

## 결함

`execution prepare --mode orca --confirm`이 실환경에서 한 번에 성공하지 못한다. orca는
워크트리·터미널을 정상 생성하는데 우리 검증이 그보다 빨라 `terminal_identity_mismatch`로
떨어진다.

### 실측

lifecycle `io-b2d0c0f1daf2`, 2026-07-26.

```
08:32:59.706  orca가 워크트리·터미널 생성
08:33:02.607  execution prepare 실패 — terminal_identity_mismatch
```

실패 직후 orca 상태를 직접 읽으면 **모든 것이 정상으로 존재한다**. 마커는 **탭** 제목에 있고
터미널 제목은 에이전트가 `✳ Claude Code`로 덮어쓴다.

`validateExecutionIntentTerminal`(`internal/adapter/orca/execution.go`)은 둘 다 본다.

```go
(strings.TrimSpace(terminal.Title) != marker && strings.TrimSpace(terminal.StableTabTitle) != marker)
```

`StableTabTitle`은 `client.go`의 `stableVisualTabTitles`가 `visualLayouts`에서 채운다. orca가
그 레이아웃을 갱신하기 전에 읽으면 빈 값이다.

### 수정 지점이 하나인 이유

`LaunchOwner`(`execution.go:249-287`)는 순차 실행이다.

```go
terminal, err = p.reconcileCreatedTerminal(...)   // ← 여기서 실패
if err != nil {
    return ..., &port.OrcaError{Code: "terminal_identity_mismatch", Invoked: true}
}
task, err := p.client.CreateTask(...)             // 도달 못 함
dispatch, err := p.client.Dispatch(...)           // 도달 못 함
```

**이슈 AC-04의 답**: task·dispatch 단계에는 타이밍 문제가 없다. 실측에서 `reconcile`이 세 번
필요했던 것은 pending이 `owner_launch` → `task_create` → `dispatch`로 순차 진행되기 때문이지
각 단계에 지연이 있어서가 아니다.

`reconcileCreatedTerminal`(`:429-459`)이 재조회 경로지만 **즉시 한 번**이라 같은 3초 창 안이다.

## 설계

재조회를 상한까지 짧은 간격으로 반복한다. 첫 조회는 지금처럼 즉시 하고, 실패하면 간격을 두고
다시 읽는다.

- **mutation은 반복하지 않는다.** `CreateTerminal`은 한 번이고, 반복하는 것은
  `listTerminalsInventory` 조회다. #90의 교훈("실패한 mutation을 재시도하지 말라")은 이 경우가
  아니다.
- **검증 기준을 바꾸지 않는다.** `validateExecutionIntentTerminal`은 그대로다 — 언제 묻는지만
  바뀐다. 마커가 끝내 안 나타나면 여전히 거부한다.
- **상한을 넘으면 지금과 같은 오류**로 떨어지고, 메시지에 시도 횟수와 총 대기 시간을 담는다.
  조용한 재시도는 다음 사람이 타이밍 문제를 다시 발견하게 만든다.

상한은 실측 3초보다 넉넉하게 잡는다. 넘으면 기존 경로로 떨어지므로 과하게 잡아도 손해가 없고,
부족하면 지금과 같다 — 비대칭이다.

## 수용 기준

- AC-01 정상 환경에서 `reconcile` 없이 준비가 끝난다
- AC-02 대기는 무한하지 않다. 상한을 넘으면 pending을 남기고 `reconcile`을 안내한다
- AC-03 재조회 횟수와 대기 시간이 결과에 남는다
- AC-04 task·dispatch 단계 확인 — **지연 없음. 순차 실행이라 도달하지 못했을 뿐이다**
- AC-05 봉인 검증이 약해지지 않는다
- AC-06 RED가 즉시-실패를 실증한다

## 검증

```
go test ./internal/adapter/orca/... -count=1
go test ./... -count=1
```

## 비범위

- orca에 탭 제목 동기화 요청. 통제 밖이다
- `reconcile` 경로 제거. 외부 mutation 애매성의 정식 복구 경로이고 남는다
