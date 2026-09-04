# 145 — task status 어휘를 orca 실제 집합에 맞춘다

이슈: https://github.com/m16khb-org/issueops/issues/145
사이클: io-2021361a3160
브랜치: `145-task-status-vocabulary` (base `main` @ 7b2538d)

## 어휘의 출처

`orca orchestration task-update --help`가 명시한다.

```
Notes:
  Valid --status values: pending, ready, dispatched, completed, failed, blocked.
```

잘못된 값을 보내면 오류 응답도 같은 여섯을 나열한다. 이것이 유일한 근거이며,
코드 대조가 아니라 실제 CLI 응답이다.

## 현재 상태

| 정의 | 집합 |
|---|---|
| **orca 실제** | pending, ready, dispatched, completed, failed, blocked |
| `executionTerminalTaskStatus` (adapter) | completed, complete, failed, cancelled, canceled, closed |
| `knownTaskStatus` (분류기, #136 이후) | ready, dispatched + settled 전부 |

두 방향으로 어긋난다.

- **분류기가 모르는 실제 상태**: `pending`, `blocked` → `inventory_unknown`으로 잘못 보고된다
- **어느 쪽도 쓰지 않는 값**: `complete`, `cancelled`, `canceled`, `closed` → orca가 거부한다

## 변경

1. **종결은 `completed`·`failed` 둘로 되돌린다.** #121이 처음 정한 것이 옳았다. `pending`(dispatch 대기)·`blocked`(의존성 대기)·`ready`(dispatch 가능)·`dispatched`(실행 중)는 넷 다 worker를 붙들거나 붙들 수 있다.
2. **orca가 거부하는 네 값을 제거한다.** 관측될 수 없는 값을 종결로 인정하면 어휘 출처가 흐려지고, 실제 방어도 되지 않는다 — 모르는 상태는 이미 `unknown`으로 fail-closed다.
3. **알려진 어휘를 여섯으로 맞춘다.** `pending`과 `blocked`가 종결이 아니므로 소유자를 요구한다.
4. **adapter의 `executionTerminalTaskStatus`도 같은 기준으로 맞춘다.**
5. **어휘의 출처를 양쪽 주석에 남긴다.** 다음 사람이 CLI를 다시 호출하지 않아도 근거를 알 수 있어야 하고, 한쪽만 바뀌면 드러나야 한다.

## 비범위

- **dispatch 상태 어휘.** CLI에 설정 명령이 없어 근거가 관측뿐이다(`dispatched`·`failed`만 봤다). 추측으로 정하면 이 이슈가 고치려는 오류를 반복한다. 발견 사항으로 남긴다.
- adapter와 core가 어휘를 공유 패키지로 참조하게 만드는 것. 의존 방향이 꼬인다.

## 검증

```bash
go test ./internal/core/operationalhealth/... -count=1
go test ./internal/adapter/orca/... -count=1
go test ./... -count=1
```

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
