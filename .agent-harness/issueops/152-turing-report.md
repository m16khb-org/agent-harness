# #152 Turing 수용 리포트

사이클: `io-d724112b669b`
이슈: https://github.com/m16khb/agent-harness/issues/152
브랜치: `152-orca-auto-fallback` (base `main` @ f9269cf)

## 범위: 후보 B

#152는 후보 A(linked-branch 순서 뒤집기)와 B(auto 폴백)를 담고 있다. **B로 좁혔고
이슈는 열린 채 A를 남긴다.**

후보 A의 비용을 실측했다. `BranchPrepare`에 의존하는 프로덕션 파일이 14개다.

```
issueops_umbrella_topology.go   execution_prepare.go       execution_remote.go
execution_complete.go            execution_owner_context.go execution_sync_base.go
issueops_cleanup_finish.go       issueops_cleanup_remote_branch.go
issueops_phase_ledger.go         issueops_remote_sync.go    issueops_pr_readiness.go
implementation/evidence.go       branchprepare/             model/types.go
```

`executionWorkspaceRequest`가 `BranchPrepare.BaseSHA`를 필수로 요구하고 우산 브랜치 위상
게이트(#129)도 같은 필드를 읽는다. 계약 근간 변경이다.

B는 이슈의 **AC-04(auto 동작을 계약으로 정한다)를 직접 충족한다.** #149에서 그 AC를
정하지 않은 채 남겼다.

## 무엇을 바꿨나

`resolveExecutionPrepareMode`가 Orca를 확정하기 전에 `ensureOrcaBranchIsFree`를 부른다.

- **충돌 + auto** → direct와 `orca_branch_name_taken`
- **충돌 + 명시 orca** → 종전대로 실패. 사용자 의도를 대신 바꾸지 않는다
- **충돌 없음** → 종전대로 orca

기존 폴백 세 사유(`gitlab_issue_metadata_unsupported`, `orca_adapter_unavailable`,
`orca_probe_failed`)와 같은 형태다.

`prepareOrcaExecution` 안의 중복 호출을 제거해 **판정 지점을 하나로** 두었다. 두 곳에
같은 검사가 있으면 "어디서 막혔는지"가 갈리고 한쪽만 고쳐질 수 있다.

## 수용 기준 판정

| AC | 판정 | 근거 |
|---|---|---|
| AC-01 auto가 충돌을 미리 알고 direct로 해소 | **달성** | `TestAutoFallsBackToDirectWhenBranchNameIsTaken` — `prepareCalls == 0` 확인 |
| AC-02 폴백 사유가 드러남 | **달성** | `TestAutoFallbackNamesTheBranchConflict` — 코드가 "branch"를 포함하고 `RequestedMode`가 auto로 남는다 |
| AC-03 명시 orca는 여전히 실패 | **달성** | `TestExplicitOrcaStillFailsOnBranchConflict` |
| AC-04 이름이 비면 orca 선택 | **달성** | `TestAutoStillChoosesOrcaWhenTheNameIsFree` — 폴백 코드가 비어 있음까지 확인 |
| AC-05 RED 선행 | **달성** | 폴백이 필요한 3건만 실패, 나머지 2건은 그때도 통과 |

## 의도한 계약 변경: preview도 충돌을 알린다

#149는 "preview는 mutation이 아니므로 막지 않는다"는 계약을 세웠다. mutation 관점에서는
옳았지만 **실행 가능성 관점에서는 아니다.**

실행할 수 없는 워크스페이스를 preview가 설명하면 운영자는 confirm에서 처음 막힌다. 그리고
auto의 경우 preview가 보여준 모드와 실제 모드가 달라진다 — 그것이 더 나쁘다.

preview의 목적은 무엇이 일어날지 보여주는 것이고, **"이 모드로는 아무것도 일어날 수
없다"도 그 답이다.**

`TestOrcaPreparePreviewSurvivesExistingBranch`를 `...PreviewReportsTheBranchConflict`로
바꾸고 근거를 주석에 남겼다. `TestAutoPreviewShowsTheSameResolvedModeAsConfirm`이 새
계약을 고정한다.

## 이번 사이클에서 진단이 작동하는 것을 확인했다

`decision add`를 실행하려다 두 번 막혔고, **#154가 넣은 `reason` 필드가 두 단계를 정확히
구분해 알려줬다.**

```
1차: "shell substitution or wrapper target is not statically resolvable" → 파이프 제거
2차: "unclassified shell command is blocked ..." → owner mutation 분류 실패
```

이전에는 `unsafe_mutation` 코드만 보고 명령을 바꿔가며 재시도했다. 이번엔 각 단계에서
무엇을 고칠지 바로 알았다.

## 발견한 갭: `decision add`가 lease active 중 실행 불가

`exactIssueOpsOwnerMutation` allowlist에 `decision add`가 없다. 그래서 이 사이클의 계약
변경 결정을 durable state에 기록할 수 없었고 이 리포트와 PR 본문에 남겼다.

**구현 중 설계 결정이 바뀌는 것은 정상이다.** 그 기록 경로가 implement 단계에서 막혀 있는
것은 갭이다. 별도 이슈로 등록한다.

## 검증

```
go test ./internal/core/issueops/ -run "Auto|ExplicitOrca|OrcaPrepare" -count=1
go test ./... -count=1
```

RED에서 폴백이 필요한 3건만 실패했고 명시 orca 실패와 정상 orca 경로 2건은 그때도
통과했다 — 이 변경이 건드리지 말아야 할 것들이다. 구현 후 전부 GREEN, 전체 회귀 통과.

## 비범위

- linked-branch 순서 계약 자체 변경(후보 A) — 14개 프로덕션 파일. #152에 남긴다
- orca가 기존 브랜치를 채택하게 만드는 것 — orca CLI에 수단이 없다(#149에서 확인)
- 사전 확인의 판정 기준 변경 — 같은 함수를 앞당겨 쓸 뿐이다

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
메인 에이전트가 직접 반론을 수행했고 저자와 검토자가 분리되지 않았다.
