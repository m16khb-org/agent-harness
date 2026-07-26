# 158 Turing 수용 리포트

사이클: `io-7161b62e2534`
이슈: https://github.com/m16khb/agent-harness/issues/158
브랜치: `158-decision-add-allowlist` (base `main` @ b785c32)

## 차단 경로가 세 곳이었다

이슈 본문은 `exactIssueOpsOwnerMutation` allowlist만 언급했다. 실제로는 세 곳이다.

| 위치 | 문제 |
|---|---|
| `commandparse/issueops.go` `ParseExactIssueOpsCommand` 33행 | 두 단어 명령 목록에 `decision`이 없어 `decision add`가 한 단어 `decision`으로 파싱된다 |
| `commandparse/issueops.go` `IssueOpsCommandSpec` | `decision add`의 플래그 spec이 등록되지 않았다 |
| `lifecycle_execution_guard.go` `exactIssueOpsOwnerMutation` | case 목록에 `decision add`가 없다 |

세 곳이 순차적 관문이므로 **하나만 고치면 증상이 남는다.** 조사에서 두 번째를, 구현
중 RED가 세 번째를 드러냈다.

## 수용 기준 판정

| AC | 판정 | 근거 |
|---|---|---|
| AC-01 implement 단계 lease active 중 실행 | **달성** | `TestDecisionAddReachesCanonicalHolderFence` |
| AC-02 actor 플래그 규율 동일 | **달성** | `TestDecisionAddWithoutActorFlagsIsNotOwnerMutation` — 플래그 없는 호출은 분류되지 않아 차단 |
| AC-03 비홀더 거부 유지 | **달성** | `TestDecisionAddFromNonHolderStaysBehindTheLease` — `holder_identity_mismatch` |
| AC-04 append-only 고정 | **달성** | `TestDecisionAddTouchesOnlyTheDecisionList`, `TestDecisionAddKeepsActiveLeaseUntouched` |
| AC-05 RED 선행 | **달성** | 홀더 통과와 비홀더 거부 두 건이 실패, actor 플래그 없는 케이스는 그때도 통과 |

## AC-04가 이 변경의 안전 근거다

`addIssueOpsDecisionLocked`는 `record.Decisions`에 append만 하고 `phase`·`lease`·
`execution`을 건드리지 않는다. allowlist에 넣는 것이 안전한 이유가 그것이므로 **테스트로
고정했다** — 나중에 이 함수가 다른 필드를 만지면 거기서 걸린다.

`AddIssueOpsDecision`은 `validateWorkspacePreparationMutation`을 부르고, 그것은
`validateExecutionMutation`으로 위임되어 다른 owner mutation과 같은 홀더 검증을 쓴다. 새
권한을 만드는 것이 아니라 이미 있어야 했던 것을 채운다.

`TestDecisionAddKeepsActiveLeaseUntouched`를 쓰다 확인한 것: **lease가 active면 actor가
필수다.** actor 없는 `AddIssueOpsDecision`은 `IssueOps execution mutation requires the
current write lease holder`로 거부된다. allowlist가 요구하는 규율과 일치한다.

## 실환경 실증은 머지 후에만 가능하다

이 사이클에서 고친 기능으로 결정을 기록해보려 했으나 막혔다. 원인이 둘이다.

1. `ParseExactIssueOpsCommand`는 `agent-harness`·`bin/agent-harness`·`./bin/agent-harness`만
   허용한다. worktree 바이너리의 절대 경로는 unclassified가 된다.
2. **가드를 실행하는 것은 훅이고 훅은 PATH의 설치본이다.** worktree에서 빌드한 바이너리로
   명령을 실행해도 그 명령을 검사하는 훅은 낡은 설치본이다.

즉 가드 변경은 **머지 후 설치본을 재빌드해야** 실환경에서 확인된다. #160이 `OPERATIONS.md`에
넣은 "설치본 재빌드" 항목이 여기에도 적용된다. 다음 사이클(#159)에서 이 기능이 실제로
동작하는지 드러날 것이다.

## 검증

```
go test ./internal/core/lifecycle/ -run "DecisionAdd" -count=1
go test ./internal/core/issueops/ -run "DecisionAddTouches|DecisionAddKeeps" -count=1
go test ./... -count=1
```

신규 5건 GREEN, 전체 회귀 통과.

## 비범위

- allowlist에 다른 명령을 함께 넣는 것. 각 명령의 안전성은 따로 판단해야 한다.
- 결정 스키마 변경.
- `ParseExactIssueOpsCommand`가 절대 경로를 받게 하는 것. 별개 판단이며 이 사이클의 범위가
  아니다.

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
메인 에이전트가 직접 반론을 수행했고 저자와 검토자가 분리되지 않았다.

구현 중 발견한 "차단 경로가 세 곳"이라는 결정을 `decision add`로 기록하려 했으나 위
이유로 실패했다 — **이 이슈가 고치는 문제를 이 사이클에서 다시 겪었다.** 그 결정을 이
리포트에 남긴다.
