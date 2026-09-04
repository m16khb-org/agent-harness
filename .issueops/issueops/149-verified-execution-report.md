# #149 Turing 수용 리포트

사이클: `io-aa325be0c36b`
이슈: https://github.com/m16khb-org/issueops/issues/149
PR: https://github.com/m16khb-org/issueops/pull/150
최종 HEAD: `ebbb5d5d39c9010760622e6a16251fc7d6e78201`

## 무엇을 바꿨나

`ensureOrcaBranchIsFree`를 orca 경로의 confirm 직후, `beginOrcaExecutionIntent` 이전에 두었다.
대상 브랜치가 로컬에 있으면 외부 mutation 없이 실패하고 원인과 다음 행동을 알린다.

## 수용 기준 판정

| AC | 판정 | 근거 |
|---|---|---|
| AC-01 브랜치가 있으면 채택한다 | **달성 불가** | orca CLI에 채택 수단이 없다. `orca worktree` 하위 명령 전수 확인(`list show current create set rm ps`)에 `adopt`가 없고, `create`에 기존 브랜치 체크아웃 옵션이 없다. `AdoptWorktree`는 `orca worktree set`으로 메타데이터 갱신이다. 사전 확인으로 대체했다 |
| AC-02 채택 브랜치 일치 | **범위 밖** | AC-01에 종속 |
| AC-03 채택 불가 시 명확히 실패 | **달성** | `TestOrcaPrepareRejectsExistingBranchBeforeMutation`이 차단·`prepareCalls == 0`·pending 부재를 고정. `TestOrcaPrepareBranchConflictExplainsTheCause`가 메시지의 원인·다음 행동을 고정 |
| AC-04 `--mode auto` 계약 | **부분** | auto가 orca로 해소된 뒤 이 검사가 막는다. 이 상황에서 direct 폴백을 할지는 정하지 않았다 — 사용자가 지정하지 않은 모드로 조용히 넘어가는 것이라 별도 결정이 필요하다 |
| AC-05 RED 선행 | **달성** | 아래 참조 |

## RED → GREEN

RED 시점 실패는 차단 두 건뿐이었고 `prepareCalls=1`로 외부 mutation이 실제 일어남을 확인했다.

```
--- FAIL: TestOrcaPrepareRejectsExistingBranchBeforeMutation
    the precheck must run before any Orca mutation: prepareCalls=1
--- FAIL: TestOrcaPrepareBranchConflictExplainsTheCause
    expected the branch conflict to block prepare
```

같은 시점에 preview·정상경로·direct 세 건은 통과했다 — 이 검사가 막지 말아야 할 것들이다.
구현 후 다섯 건 전부 GREEN, `go test ./... -count=1` 전체 회귀 통과, CI `verify: pass`.

## 검증

```
go test ./internal/core/issueops/ -run "OrcaPrepare|DirectPrepareStillAllows" -count=1
go test ./... -count=1
gh pr checks 150   # verify: pass
```

## 이 변경이 하지 않는 것

orca 모드로 IssueOps 정식 순서를 쓰는 길을 열지 않는다. 실패가 잔여물 없이 명확해질 뿐이다.
근본 해결은 orca 모드에서 linked branch를 나중에 만드는 순서 변경이며 별도 이슈로 남긴다.

로컬 refs만 검사한다. 원격 전용 브랜치에 대한 orca 동작을 확인하지 못했고, 추측으로 정상
경로를 막지 않기 위해 범위를 주석에 명시했다.

## 사이클 중 정정한 오류

1. **이슈 본문의 설계 방향이 틀렸다.** "`AdoptWorktree`가 있으니 채택하면 된다"고 썼으나 그
   함수는 `orca worktree set`이다. 이름만 보고 기능을 추정한 오류였고, CLI를 실제로 확인해
   정정한 뒤 범위를 좁혔다.
2. **테스트 픽스처를 잘못 골랐다.** 처음 쓴 RED가 `ResolvedMode: direct`로 실패했는데, 이미
   direct execution을 가진 레코드라 orca 경로에 닿지 않았기 때문이다. 코드가 아니라 픽스처
   문제였다. 이 확인 과정에서 `PrepareExecution`이 `record.Execution != nil`이면 먼저
   반환한다는 것도 드러났고, 덕분에 재진입이 이 검사에 닿지 않음을 근거로 말할 수 있게 됐다.

## 게이트 한계

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
메인 에이전트가 직접 반론을 수행했고 저자와 검토자가 분리되지 않았다.
