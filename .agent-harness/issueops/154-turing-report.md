# #154 Turing 수용 리포트

사이클: `io-88ff9d63699d`
이슈: https://github.com/m16khb/agent-harness/issues/154
브랜치: `154-blocking-diagnostics` (base `main` @ eb94899)

이 리포트는 **PR 생성 이전에** 커밋한다. #153에서 실측한 대로, `execution complete`가
리포트를 요구하는 시점이 PR 머지 이후면 그 커밋이 어디에도 실리지 못한다.

## 무엇을 바꿨나

차단이 원인과 다음 행동을 말하게 했다. **새 판정 로직을 만들지 않았고, 게이트 조건을
한 줄도 바꾸지 않았다.** 이미 계산되던 값의 전달 경로만 이었다.

| 지점 | 변경 |
|---|---|
| `IssueOpsDenyReason` | `Reason` 필드 추가 — `hookDenyReason`이 버리던 사유가 전달된다 |
| `cleanup finish` | `WorkspaceProcesses`로 점유 PID·명령명을 담고, 관측 불가를 `workspace_processes_observable`로 분리 |
| `cleanup finish` | `completion_reflected` 차단 시 `remote reflect-completion` 안내 |
| `execution reconcile` | `ExternalStateInspected`로 외부 조회 여부를 밝힌다 |
| `ensureOrcaBranchIsFree` | 로컬에 더해 remote-tracking ref도 조회 |

## 수용 기준 판정

| AC | 판정 | 근거 |
|---|---|---|
| AC-01 프로세스 PID·명령명 | **달성** | `TestCleanupFinishNamesTheProcessesHoldingTheWorktree` |
| AC-02 관측불가 슬러그 분리 | **달성** | `TestCleanupFinishSeparatesUnobservableFromOccupied` |
| AC-03 확정적 해소 명령 | **달성** | `TestCleanupFinishGuidesTheDeterministicRemedy`. 상황에 따라 갈리는 missing에는 붙이지 않는다 — 틀린 안내는 안내가 없는 것보다 나쁘다 |
| AC-04 거부 사유 전달 | **달성** | `TestUnsafeMutationDenyCarriesTheReasonItAlreadyComputed`(3케이스), `TestHookDenyReasonCarriesTheBlockingCause` |
| AC-05 비밀 미노출 | **달성** | `TestDenyReasonDoesNotEchoTheRawCommand` — 인자의 토큰이 사유에 나타나지 않고, 그렇다고 사유가 비지도 않음을 함께 고정 |
| AC-06 RED 선행 | **달성** | 아래 참조 |
| AC-07 preview 조회 여부 | **달성** | `TestReconcilePreviewDeclaresItDidNotInspectOrca`, `TestReconcileConfirmDeclaresItInspectedOrca` |
| AC-08 원격 refs 검사 | **달성** | `TestOrcaPrepareRejectsRemoteOnlyBranchBeforeMutation`, `TestOrcaRemoteOnlyConflictExplainsWhereTheBranchLives` |

## RED → GREEN

- **AC-04/05**: `got.Deny.Reason undefined` — 필드 자체가 없어 컴파일이 실패했다. 구현 후 3케이스 전부 GREEN.
- **AC-01/02/03**: `result.WorkspaceProcesses undefined` — 같은 형태의 RED. 구현 후 4건 GREEN.
- **AC-07**: `result.ExternalStateInspected undefined`. 구현 후 2건 GREEN.
- **AC-08**: 신규 2건만 실패하고 기존 5건은 통과 — 이 검사가 막지 말아야 할 것들이다. 구현 후 7건 전부 GREEN.
- `go vet` 통과, `go test ./... -count=1` 전체 통과.

## 이 사이클이 스스로 증명한 것

구현 중 `TestDenyReasonStaysEmptyWhenNothingIsBlocked`가 예상 밖으로 실패했다. 원인은
픽스처의 AgentID 누락이었는데, **#90이 넣은 `mismatch axis: agent_id`가 그것을 즉시
알려줬다.** 진단 정보가 있으면 원인을 찾는 데 한 번의 확인으로 끝난다는 것을 이 사이클이
스스로 겪었다.

## 드러난 것: 기존 테스트가 실환경과 다른 전제 위에 있었다

`executionPrepareRecord` 픽스처는 `refs/remotes/origin/<branch>`를 만든다 — 그것이
IssueOps 정식 순서(`gh issue develop` → `branch prepare`)의 상태이기 때문이다. 그런데
orca 테스트들이 **그 상태에서 orca가 성공한다고 검증**하고 있었다. 실환경에서는 orca가
접미사를 붙여 실패한다.

`orcaPrepareRecord` 헬퍼를 만들어 orca가 작동하는 조건(이름이 로컬·원격 어디에도 없음)을
명시했고, 그 조건이 정식 순서와 양립하지 않는다는 사실을 주석으로 #152에 연결했다.

#149의 사전 확인이 실환경에서 뚫린 이유가 이것이다. 그 사이클의 테스트가 로컬 브랜치를
만드는 픽스처를 써서 정식 순서를 재현하지 못했다. **테스트가 통과했지만 실환경은 달랐다.**

## 의도한 계약 변경

`assertIssueOpsDenyFields`가 "non-identity deny는 필드 5개"를 고정하고 있었다. 6개로
갱신했다. 그 테스트가 지키려던 것(진단 필드 남발 금지)은 identity 필드의 조건부 노출로
유지하고, `reason`은 모든 deny에 필수로 검증하게 했다.

`ExternalStateInspected`에는 `omitempty`를 쓰지 않았다. 설계에서 "omitempty로만 더한다"고
했으나, 이 필드는 "조회하지 않았다"가 핵심 정보이므로 false가 사라지면 목적이 무너진다.
그 근거를 주석에 남겼다.

## 검증

```
go test ./internal/core/lifecycle/ -run "DenyReason|UnsafeMutationDeny" -count=1
go test ./cmd/harness/hookcli/ -run "HookDenyReason" -count=1
go test ./internal/core/issueops/ -run "CleanupFinish|Reconcile.*Declares|OrcaPrepare|OrcaRemoteOnly" -count=1
go vet ./internal/core/issueops/ ./internal/core/lifecycle/
go test ./... -count=1
```

## 비범위

- `remote_tip_equals_merged_head`의 단일 아티팩트 한계 — #153
- orca 모드와 linked-branch 순서 충돌 자체 — #152. 이 사이클은 사전 확인의 검사 범위만
  바로잡았고, orca 모드로 정식 순서를 쓰는 길은 여전히 막혀 있다.

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
메인 에이전트가 직접 반론을 수행했고 저자와 검토자가 분리되지 않았다.
