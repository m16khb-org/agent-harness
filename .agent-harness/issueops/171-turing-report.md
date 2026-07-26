# #171 Turing 리포트 — orca dispatch·gate 어휘

lifecycle: `io-4584e4c16409`
issue: https://github.com/m16khb/agent-harness/issues/171
branch: `171-orca-status-vocabulary` (base `0e14ffb403a9194dda04964e8f70e1359da48085`)

## 판정

| AC | 판정 | 증거 |
|---|---|---|
| AC-01 dispatch 5개 어휘를 안다 | 충족 | `TestClassifyKnowsEveryOrcaDispatchStatus` (5 서브테스트) |
| AC-02 미지 값은 여전히 unknown | 충족 | `TestClassifyStillFlagsUnknownDispatchStatus` |
| AC-03 종결 dispatch는 잔여물 판정 skip | 충족 | `TestClassifySkipsResidueForSettledDispatch` (3) + `TestClassifyStillFlagsLiveDispatchWithoutOwner` (2) |
| AC-04 gate `timeout` | 충족 | `TestClassifyKnowsEveryOrcaGateStatus` (3), `TestClassifyStillFlagsUnknownGateStatus` |
| AC-05 어휘 출처 인용 | 충족 | `knownDispatchStatus`·`settledDispatchStatus`·`knownGateStatus` 주석 |
| AC-06 RED 선행 | 충족 | 아래 |

## RED

```
--- FAIL: TestClassifyKnowsEveryOrcaDispatchStatus/pending
    유효 어휘 "pending"를 unsupported로 분류하면 진짜 미지 값이 묻힌다
--- FAIL: .../completed
--- FAIL: .../failed
--- FAIL: .../circuit_broken
--- FAIL: TestClassifyStillFlagsLiveDispatchWithoutOwner/pending
--- FAIL: TestClassifyKnowsEveryOrcaGateStatus/timeout
```

`dispatched`만 통과하던 것이 그대로 드러났다.

## 변경

`internal/core/operationalhealth/classifier.go`

dispatch 처리를 task와 같은 3단으로 맞췄다.

```go
if !knownDispatchStatus(status) {
    builder.add(FindingInventoryUnknown, ...)
    continue
}
if settledDispatchStatus(status) {
    continue
}
if len(dispatchOwners[id]) != 1 {
    builder.add(FindingTaskResidue, ...)
}
```

| 함수 | 집합 | 출처 |
|---|---|---|
| `knownDispatchStatus` | `pending`, `dispatched`, `completed`, `failed`, `circuit_broken` | `types.ts` `DispatchStatus`, `db.ts` `CHECK` 제약 |
| `settledDispatchStatus` | `completed`, `failed`, `circuit_broken` | `db.ts` `failDispatch`, `coordinator.ts` |
| `knownGateStatus` | `timeout` 추가 | `types.ts` `GateStatus` |

## 설계 근거

### `failed`가 종결인 이유

`db.ts`의 `failDispatch`:

```ts
const newStatus: DispatchStatus = newFailureCount >= 3 ? 'circuit_broken' : 'failed'
const taskStatus: TaskStatus = newStatus === 'circuit_broken' ? 'failed' : 'ready'
```

dispatch가 `failed`면 그 **시도**가 끝났다. 작업은 task가 `ready`로 돌아가 재dispatch를
기다리고, 그 대기는 task 축(`settledTaskStatus`)이 본다. 재dispatch는 새 dispatch ID를
만들므로 이 dispatch로 돌아오지 않는다.

### `circuit_broken`이 종결인 이유

`coordinator.ts`가 그 상태에서 재dispatch하지 않고 `failedTasks`에 넣는다. 워커를 다시 붙일
경로가 없다.

### 종결 dispatch를 skip하는 이유

task 처리가 같은 판단을 이미 한다 — "A settled task holds no resource: it is orchestration
history". orca에 per-dispatch 삭제 명령이 없으므로(전역 `reset`뿐) owner를 요구하면 끝난
dispatch가 영원히 residue로 보고된다.

## 테스트 설계에서 정정한 것

첫 RED에서 사이클이 소유한 dispatch로 검증했더니 `cycle dispatch identity does not match`가
함께 떴다. 그것은 어휘가 아니라 **살아 있는 홀더의 dispatch는 `dispatched`여야 한다**는 별개
계약이다(`classifier.go`의 cycle-dispatch 일치 검사).

사이클이 참조하지 않는 dispatch(`unowned-dispatch`)로 바꿔 인벤토리 루프만 검증하도록 했고,
`hasFindingID`로 리소스 ID까지 대조해 다른 finding에 섞이지 않게 했다.

## 검증

```
go build ./...                                        성공
go test ./internal/core/operationalhealth/ -count=1   PASS
go test ./... -count=1                                PASS (전 패키지)
```

## 비범위

- `internal/adapter/orca/execution.go`의 dispatch 판정 — #147이 다룬다. 두 곳이 같은 어휘를
  쓰게 되는 시점에 주석이 서로를 참조하게 한다
- `CoordinatorStatus` — 이 저장소에서 쓰지 않는다(확인함)
