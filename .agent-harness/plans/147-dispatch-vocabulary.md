# #147 dispatch 상태 어휘 의존

이슈: https://github.com/m16khb/agent-harness/issues/147
lifecycle: io-b2d0c0f1daf2
branch: 147-dispatch-vocabulary (base e0f2b8880b6911039a50dad43a762951978fa000)
mode: **orca** — 이 사이클은 orca 소유자가 구현했다

## 이 사이클의 특이점

**orca 에이전트가 구현했고 planner가 이어받아 완결했다.** 이 저장소에서 orca 모드 소유권
이전이 실제로 작동한 첫 기록이다.

| 시각 | 사건 |
|---|---|
| 08:32:59 | orca가 워크트리·터미널 생성 (`prepare --mode orca --confirm`) |
| 08:36:50 | orca 에이전트가 gen 1 claim |
| 09:14:29 | 구현을 남기고 release |
| 10:35:14 | planner가 `reseed` → `claim`으로 gen 2 이어받음 |

그 에이전트의 Turing 리포트는 `.agent-harness/turing/147-acceptance.json`에 있다. 자기가 막힌
게이트 셋(`branch_link_verified`, `compatibility_review`, `plan_path`)을 "planner 소유 게이트라
자기가 해결하면 범위 확장"이라고 판단해 publication을 멈춘 것까지 기록했다 — 정확한 판단이다.

## 결함

`internal/adapter/orca/execution.go`의 `InspectOwner`가 dispatch 상태를 `executionTerminalTaskStatus`
로 판정했다. 그 함수는 task 어휘(`completed`, `failed`)만 알아서 dispatch의 `circuit_broken`을
모르고, 모르는 값은 미종결로 떨어졌다.

```go
if !executionTerminalTaskStatus(result.DispatchStatus) {
    result.TaskLive = true
}
```

`db.ts`의 `failDispatch`가 그 상태의 의미를 확정한다 — `circuit_broken`이면 task는 `failed`다.
즉 `TaskLive`가 `false`로 계산된 것을 이 분기가 `true`로 덮어썼다. **orca가 종결로 선언한
소유자를 우리 코드가 살아 있다고 판정했다.**

## 결정: 후보 A (dispatch를 판정에서 제외)

원래 design review는 어휘 확장(`executionTerminalDispatchStatus` 신설)을 승인했다. orca
에이전트는 후보 A를 구현했고, 그 손실 분석이 더 정확했다.

리포트의 `impact_analysis`:

> 두 설계는 `circuit_broken`에서 같은 결과를 낸다. 갈리는 지점은 **task가 종결이고 dispatch가
> pending 또는 dispatched인 조합 하나뿐**이다 — planner 설계는 막고 내 설계는 통과시킨다.

그 조합을 소스로 검증했다. `worker_done`이 task·dispatch를 함께 completed로 만들고
`failDispatch`가 둘을 함께 움직이므로 **정상 흐름에서 나오지 않는다.** 그리고 그때 task가
종결이면 worker는 이미 없으므로 막는 것이 오탐이다.

**결론**: dispatch가 살아 있음을 말하는 모든 경우에 task도 비종결이다. 판정에 넣는 것은 같은
사실을 두 번 묻는 것이었다.

## planner가 정정한 것

주석이 "어휘를 확인할 방법이 없다"를 근거로 삼았는데, 이 세션이 #171에서 orca 소스로 어휘를
확정했다(`types.ts` `DispatchStatus` 5개, `db.ts` SQLite `CHECK` 제약).

그 에이전트는 그 사실을 알았지만 검증할 수 없었다 — 리포트가 정확히 기록한다:

> 이 세션에서 그 소스를 직접 검증하지 못했다 — lifecycle guard가 worktree 밖 탐색 명령을
> 차단한다.

주석을 정정하면서 손실 분석을 **강화**했다. 이제 근거가 "어휘를 모른다"가 아니라 "어휘를 알고
보니 dispatch가 task와 함께 움직여 정보를 더하지 않는다"이고, `db.ts` 인용으로 증명된다.

## 세 곳의 dispatch 어휘 관계 (AC-05)

| 위치 | 판정 | 근거 |
|---|---|---|
| `core/issueops/reset_legacy_drain.go` | `{completed, failed, circuit_broken}` 종결 | legacy v0 드레인 |
| `core/operationalhealth/classifier.go` | 같은 집합 (#171) | 인벤토리 분류 — "이 자원이 잔여물인가" |
| `adapter/orca/execution.go` | **판정에서 제외** (#147) | 소유자 생존 — "이 소유자가 살아 있는가" |

앞 둘은 같은 질문을 하고 같은 집합을 쓴다. 셋째는 다른 질문이고, task 축이 이미 그 정보를
담으므로 dispatch 어휘가 필요 없다. 그 차이를 `InspectOwner` 주석이 설명한다.

## 수용 기준

- AC-01 dispatch 의존의 처분 결정 — 제거
- AC-02 근거가 코드에 남음 — `InspectOwner` 주석
- AC-03 `orca_resources_absent` fail-closed 유지 — 테스트 2개
- AC-04 RED가 계약을 실증 — `TestExecutionOwnerInventoryDoesNotDeriveLivenessFromDispatchStatus`
- AC-05 세 곳의 어휘 관계 설명 — 위 표와 주석

## 검증

```
go test ./internal/adapter/orca/... -count=1
go test ./... -count=1
```

## 남긴 것

- `gh issue develop`이 lease 활성 중 워크트리에서 가드에 막힌다. source root에서만 된다.
  #163이 정한 orca 순서가 그 제약을 반영하지 않는다 — 후속 이슈로 낸다
- base가 `e0f2b88`이라 네 PR 뒤처져 있다. 봉인 가드가 `merge`를 막으므로 GitHub 병합에 맡긴다
