# #171 orca dispatch·gate 어휘

이슈: https://github.com/m16khb/agent-harness/issues/171
lifecycle: io-4584e4c16409
branch: 171-orca-status-vocabulary (base 0e14ffb403a9194dda04964e8f70e1359da48085)

## 결함

`internal/core/operationalhealth/classifier.go:271`

```go
if strings.TrimSpace(dispatch.Status) != "dispatched" {
    builder.add(FindingInventoryUnknown, "dispatch", id, "dispatch status is unsupported: "+..., "")
    continue
}
```

orca의 실제 어휘는 다섯이다 — `stablyai/orca` `src/main/runtime/orchestration/types.ts:15`

```ts
export type DispatchStatus = 'pending' | 'dispatched' | 'completed' | 'failed' | 'circuit_broken'
```

`db.ts:164`의 SQLite `CHECK` 제약도 같은 집합을 박는다. **유효 어휘 5개 중 4개가
오분류된다.** 이 저장소 orca 런타임에 `failed` dispatch가 3건 있어 지금 오탐이 뜬다.

`:486-493`의 `knownGateStatus`도 `pending`, `resolved`만 안다. `GateStatus`는
`'pending' | 'resolved' | 'timeout'`(`types.ts:17`)이다.

## 모범 패턴이 이미 있다

task 처리(`:250-266`)는 세 질문을 순서대로 한다.

1. `knownTaskStatus` — 어휘를 아는가. 모르면 `FindingInventoryUnknown`
2. `settledTaskStatus` — 종결됐는가. 그러면 자원을 잡고 있지 않으므로 skip
3. owner가 정확히 하나인가. 아니면 `FindingTaskResidue`

dispatch는 이 셋이 `!= "dispatched"` 하나로 뭉개져 있다.

## 변경

`knownDispatchStatus`와 `settledDispatchStatus`를 `knownTaskStatus`·`settledTaskStatus` 옆에
둔다.

| 함수 | 집합 | 근거 |
|---|---|---|
| `knownDispatchStatus` | `pending`, `dispatched`, `completed`, `failed`, `circuit_broken` | `types.ts:15`, `db.ts:164` |
| `settledDispatchStatus` | `completed`, `failed`, `circuit_broken` | 아래 |
| `knownGateStatus` | `pending`, `resolved`, `timeout` 추가 | `types.ts:17` |

### `failed`가 종결인 이유

`db.ts:818,827`

```ts
const newStatus: DispatchStatus = newFailureCount >= 3 ? 'circuit_broken' : 'failed'
const taskStatus: TaskStatus = newStatus === 'circuit_broken' ? 'failed' : 'ready'
```

dispatch가 `failed`면 그 **시도**가 끝났다. task는 `ready`로 돌아가 재dispatch를 기다리고,
그 대기는 task 축(`:250-266`)이 본다. 재dispatch는 새 dispatch ID를 만든다.

### `circuit_broken`이 종결인 이유

`coordinator.ts:291-294`가 그 상태에서 재dispatch하지 않고 `failedTasks`에 넣는다. 워커를
다시 붙일 경로가 없다.

## 수용 기준

- AC-01 dispatch 판정이 orca의 5개 어휘를 안다
- AC-02 어휘에 없는 값은 여전히 `FindingInventoryUnknown`이다
- AC-03 종결된 dispatch는 잔여물 판정을 건너뛴다
- AC-04 `knownGateStatus`가 `timeout`을 안다
- AC-05 어휘 출처가 `types.ts`·`db.ts` 인용으로 주석에 남는다
- AC-06 RED가 현재의 오분류를 실증한다

## 검증

```
go test ./internal/core/operationalhealth/... -count=1
go test ./... -count=1
```

## 비범위

- `internal/adapter/orca/execution.go`의 dispatch 판정. #147이 다룬다
- orca CLI에 어휘 노출 요청. 소스가 공개라 불필요하다
