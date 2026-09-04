# 136 — abandon의 잔여물 게이트를 orca 자원까지 넓힌다

이슈: https://github.com/m16khb-org/issueops/issues/136
사이클: io-af3e4d3592e6
브랜치: `136-abandon-orca-residue-gate` (base `main` @ 5676f80)

## 문제

`cleanup abandon`은 "아무것도 지우지 않는 경로"다. 잔여물이 있으면 그것을 지울 수 있는
경로(`finish`/`orphan`)로 보낸다. 게이트 ⑥이 그 원칙을 강제하는데, **로컬 디렉터리의
존재만** 본다.

orca 워크트리가 외부에서 사라진 orca 사이클을 abandon하면 task가 `dispatched`로 남아
영구 residue가 된다. #130이 정상 완료 경로를 봉합했지만 abandon은 complete를 거치지
않는다.

pending intent 경로는 이미 안전하다 — 게이트 ⑤가 `worktree_create` 단계만 허용하고
그 단계의 payload는 `TaskID` 공백을 강제한다. 문제는 prepare가 완료된 사이클이다.

## 착수 중 발견: 종결 상태 집합이 어긋난다

| 정의 | 집합 |
|---|---|
| `executionTerminalTaskStatus` | completed, complete, failed, cancelled, canceled, closed |
| `settledTaskStatus` (분류기) | completed, failed |

**[정정]** 이슈 코멘트에 "cancelled task가 residue로 보고된다"고 썼는데 부정확했다.
`knownTaskStatus`가 **ready, dispatched, completed, failed** 넷만 알기 때문에, `cancelled`는
`settledTaskStatus` 검사에 도달하기도 전에 `FindingInventoryUnknown`으로 분기한다. 증상
이름은 `operational_task_residue`가 아니라 `inventory_unknown`이다.

결론은 바뀌지 않는다 — 게이트가 통과시킨 task를 분류기가 계속 finding으로 보고한다.
다만 수정 범위가 넓어진다: `settledTaskStatus` 하나가 아니라 `knownTaskStatus`까지
adapter 집합과 맞춰야 한다.

분류기를 넓은 쪽에 맞춘다. cancelled와 closed도 dispatch될 수 없으므로 논리적으로
종결이며, #121이 completed/failed에 대해 세운 논리가 그대로 적용된다.

## 설계

**게이트를 더한다. mutation을 더하지 않는다.**

`CleanupAbandonDeps`에 `ExecutionOrcaOwnerInspector`를 주입하고, orca 모드이고 `TaskID`가
있으면 `InspectOwner`로 실조회한다. 자원이 살아 있으면 거부하고 `finish`나 `orphan`을
지시한다.

- 조회 실패나 inspector 부재는 **거부**다(#106 계약: 어댑터 부재는 통과가 아니다).
- 레코드에 바인딩이 있다는 것만으로 막지 않는다. 그러면 orca에서 이미 정리된 사이클까지
  차단해 중도 포기 경로가 사라지고 #129와 같은 데드락을 새로 만든다.

`InspectOwner`는 `execution replace --preview`가 이미 쓰는 quiescence 조회다. 새 표면이
필요 없다.

## 비범위

- abandon이 orca 자원을 회수하게 만드는 것. "아무것도 지우지 않는 경로"라는 설계 의도와
  충돌하고 #106이 이 명령을 조회 전용으로 못박은 근거를 무너뜨린다.
- pending intent 경로 변경. 이미 안전하다.

## 검증

```bash
go test ./internal/core/issueops/... -count=1
go test ./internal/core/operationalhealth/... -count=1
go test ./... -count=1
```

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
활성 orca 사이클이 0건이므로 실환경 도그푸드는 불가능하다. fake inspector로 계약을 고정한다.
