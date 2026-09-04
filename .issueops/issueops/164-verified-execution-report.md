# 164 Turing 수용 리포트

사이클: `io-e0c0b45f2700`
이슈: https://github.com/m16khb-org/issueops/issues/164
브랜치: `164-codex-gitlab-coverage` (base `main` @ d5249fb)

## RED가 없었다

새로 쓴 테스트 7건이 **처음부터 GREEN**이다. 제품 코드가 이미 옳게 동작하기 때문이다.

따라서 **AC-05(RED 선행)는 달성하지 못했다.** 이 사이클은 버그를 고치지 않았고 회귀
방지 테스트를 추가했다. 그것이 정직한 성격 규정이다.

제품 코드 변경은 **0줄**이다(`git status`로 확인). 문서 한 줄과 테스트 두 파일뿐이다.

## 수용 기준 판정

| AC | 판정 | 근거 |
|---|---|---|
| AC-01 브랜치 있는 GitLab도 provider 사유가 먼저 | **달성** | `TestGitLabCapabilityOutranksBranchConflictRegardlessOfBranchState` (auto·명시 orca 두 조합) |
| AC-02 codex host 커버리지 | **달성** | `TestAutoBranchFallbackIsIdenticalAcrossFirstPartyHosts`, `TestExplicitOrcaBranchConflictIsIdenticalAcrossFirstPartyHosts` |
| AC-03 `decision add` codex actor | **달성** | `TestDecisionAddReachesHolderFenceForCodexHost`, `...StaysBlockedForCodexHost` |
| AC-04 `OPERATIONS.md` provider 중립 | **달성** | `PR` → `PR/MR`, GitLab 실환경 미확인 명시 |
| AC-05 RED 선행 | **달성 불가** | 위 참조 — 고칠 버그가 없었다 |

추가로 #153의 ancestry 경로와 `ensureOrcaBranchIsFree`의 provider 중립성도 고정했다
(`TestRemoteBranchAncestryIsProviderNeutral`,
`TestOrcaBranchPrecheckSeesRemoteRefRegardlessOfProvider`).

## 이슈 본문의 전제를 정정했다

본문에 "provider 우선순위를 지키는 테스트가 없다"고 썼는데 **틀렸다.**
`TestExecutionGitLabOrcaCapabilityFailsBeforeProbeOrMutation`(`execution_orca_test.go:93`)이
이미 고정한다 — auto·명시 orca × preview·confirm 네 조합에서 `gitlab_issue_metadata_
unsupported`를 확인하고 `probeCalls == 0`까지 검증한다. 실행해 통과를 확인했다.

**확인 없이 "없다"고 단정한 오류다.** 이슈에 코멘트로 정정했다.

실제 공백은 더 좁았다. 그 테스트는 `orcaPrepareRecord`를 쓰고 그 헬퍼는 원격 브랜치 ref를
지운다(#154에서 만들었다). 즉 "브랜치 이름이 어디에도 없는 GitLab 사이클"만 검증한다.
**IssueOps 정식 순서를 따른 GitLab 사이클 — 브랜치가 원격에 이미 있는 상태 — 가
미검증이었다.**

## 가장 중요한 계약

`AC-01`이 고정하는 것은 **폴백 코드가 사용자가 할 일을 정한다**는 계약이다.

- `gitlab_issue_metadata_unsupported` → Orca가 GitLab 메타데이터를 봉인하지 못한다. 사용자가
  할 수 있는 것이 없다
- `orca_branch_name_taken` → 브랜치 이름을 비우면 Orca를 쓸 수 있다

`execution_prepare.go`에서 GitLab 분기(327행)가 브랜치 검사(366행)보다 위에 있어 GitLab
사이클은 전자를 받는다. 그 순서가 뒤바뀌면 **#154가 세운 진단 계약이 거짓 원인을 말하고**
사용자는 브랜치를 지우려 하다 실패한다.

테스트가 `FallbackCode`를 정확히 비교하므로 순서가 바뀌면 걸린다.

## 중복을 만들지 않았다

host별 테스트가 claude 테스트의 복사본이 되지 않도록 **host가 실제로 읽히는 경로만**
골랐다. `IssueOpsImplementerDefaults`의 host별 모델·effort 차이는
`TestExecutionOrcaPrepareAppliesHostImplementerDefaults`가 이미 두 host를 다루므로
중복하지 않았다.

## 문서

`OPERATIONS.md`의 리포트 타이밍 항목이 `PR`만 언급했다. `PR/MR`로 넓히고 **GitLab
실환경 미확인을 명시했다** — 원인이 provider가 아니라 `execution complete`의 리포트 요구
시점이므로 같을 것으로 보이지만 추측을 단정으로 쓰지 않는다.

## 검증

```
go test ./internal/core/issueops/ -run "GitLabCapabilityOutranks|IsIdenticalAcrossFirstPartyHosts|IsProviderNeutral|SeesRemoteRefRegardlessOfProvider" -count=1
go test ./internal/core/lifecycle/ -run "DecisionAdd" -count=1
go test ./... -count=1
```

신규 7건 GREEN(RED 없이), 전체 회귀 통과.

## 비범위

- 제품 코드의 host·provider 분기 변경 — 이미 동작하며 건드리면 깨뜨릴 위험만 생긴다
- GitLab 실환경 dogfood — 이 저장소는 GitHub에 있다
- host·provider 조합 전수 테스트 — 복사본은 유지 비용만 늘린다

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
메인 에이전트가 직접 반론을 수행했고 저자와 검토자가 분리되지 않았다.
