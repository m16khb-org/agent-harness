# 164 — codex·gitlab 경로를 테스트로 고정한다

이슈: https://github.com/m16khb/agent-harness/issues/164
사이클: io-e0c0b45f2700
브랜치: `164-codex-gitlab-coverage` (base `main` @ d5249fb)

## 문제

이 저장소는 first-party로 **codex·claude** 호스트와 **github·gitlab** provider를 지원한다.
그런데 #149·#152·#153·#154·#158 다섯 사이클이 추가한 테스트 17건이 전부 `claude` +
`github`만 검증했다.

제품 코드는 양쪽을 다룬다 — 확인했다. 문제는 **그 대등함이 회귀에서 지켜진다는 보장이
없다**는 것이다.

## 이 사이클은 버그를 고치지 않는다

**RED가 없었다.** 새로 쓴 테스트 7건이 처음부터 GREEN이다. 제품 코드가 이미 옳게
동작하기 때문이며, 그것이 정직한 결과다.

따라서 이 사이클의 성격은 **회귀 방지**다. AC-05(RED 선행)는 달성 불가이고, 그 사실을
Turing 리포트에 명시한다.

## 이슈 본문의 전제를 정정했다

본문에 "provider 우선순위를 지키는 테스트가 없다"고 썼는데 틀렸다.
`TestExecutionGitLabOrcaCapabilityFailsBeforeProbeOrMutation`이 이미 고정한다 — 실행해
확인했다. 확인 없이 단정한 오류이며 이슈에 코멘트로 정정했다.

**실제 공백은 더 좁았다.** 그 테스트는 `orcaPrepareRecord`를 쓰는데 그 헬퍼는 원격 브랜치
ref를 지운다(#154에서 만들었다). 즉 "브랜치 이름이 어디에도 없는 GitLab 사이클"만
검증한다. IssueOps 정식 순서를 따른 GitLab 사이클 — 브랜치가 원격에 이미 있는 상태 — 는
미검증이었다.

## 무엇을 고정했나

| 테스트 | 고정하는 계약 |
|---|---|
| `TestGitLabCapabilityOutranksBranchConflictRegardlessOfBranchState` | 브랜치가 원격에 있어도 GitLab 사유가 먼저다. `orca_branch_name_taken`이 나오면 사용자가 엉뚱한 조치를 한다 |
| `TestAutoBranchFallbackIsIdenticalAcrossFirstPartyHosts` | auto 폴백이 codex·claude에서 같다 |
| `TestExplicitOrcaBranchConflictIsIdenticalAcrossFirstPartyHosts` | 명시적 orca 차단이 두 host에서 같다 |
| `TestRemoteBranchAncestryIsProviderNeutral` | #153의 ancestry 경로가 github·gitlab에서 같다 |
| `TestOrcaBranchPrecheckSeesRemoteRefRegardlessOfProvider` | 원격 ref 검사가 provider와 무관하다 |
| `TestDecisionAddReachesHolderFenceForCodexHost` | #158 allowlist가 codex holder에서 동작한다 |
| `TestDecisionAddFromNonHolderStaysBlockedForCodexHost` | codex 비홀더도 거부된다 |

가장 중요한 것은 첫 항목이다. **어느 코드로 폴백하는지가 사용자가 할 일을 정한다** —
`gitlab_issue_metadata_unsupported`는 "orca가 GitLab 메타데이터를 봉인하지 못한다"이고
`orca_branch_name_taken`은 "브랜치 이름을 비우면 된다"다. 순서가 뒤바뀌면 #154가 세운
진단 계약이 거짓 원인을 말한다.

## 문서

`OPERATIONS.md`의 Turing 리포트 타이밍 항목이 `PR`만 언급했다. `PR/MR`로 바꾸고 GitLab
실환경 미확인을 명시했다 — 원인이 provider가 아니라 `execution complete`의 요구 시점이므로
같을 것으로 보이지만 추측을 단정으로 쓰지 않는다.

## 비범위

- 제품 코드의 host·provider 분기 변경. **이미 동작하며 건드리면 깨뜨릴 위험만 생긴다.**
- GitLab 실환경 dogfood. 이 저장소는 GitHub에 있다.
- host·provider 조합 전수 테스트. 복사본은 유지 비용만 늘린다. host가 실제로 읽히는
  경로만 골랐다.

## 검증

```bash
go test ./internal/core/issueops/ -run "GitLabCapabilityOutranks|IsIdenticalAcrossFirstPartyHosts|IsProviderNeutral|SeesRemoteRefRegardlessOfProvider" -count=1
go test ./internal/core/lifecycle/ -run "DecisionAdd" -count=1
go test ./... -count=1
```

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
