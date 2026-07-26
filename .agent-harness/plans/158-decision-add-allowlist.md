# 158 — `decision add`를 lease active 중 실행할 수 있게 한다

이슈: https://github.com/m16khb/agent-harness/issues/158
사이클: io-7161b62e2534
브랜치: `158-decision-add-allowlist` (base `main` @ b785c32)

## 문제

`decision add`가 lease active 중 `unsafe_mutation`으로 거부된다. #152에서 실측했고, #154가
넣은 `reason` 필드가 원인을 밝혔다.

```
"reason": "unclassified shell command is blocked while IssueOps mutation authority is active"
```

**구현 중 설계 결정이 바뀌는 것은 정상이다.** #152에서 `resolveExecutionPrepareMode`에
브랜치 검사를 넣으니 #149가 세운 preview 계약과 충돌했고, 그 계약을 바꾸는 판단이 필요했다.
그 시점은 이미 `implement` 단계였고 lease가 active였다. 결정을 기록할 방법이 없어 Turing
리포트와 PR 본문에만 남겼다.

IssueOps는 결정을 durable state에 담아 나중 사이클이 `plan-prep`의 prior-decisions로
조회하게 설계됐다. **문서에만 남은 결정은 그 경로에 들어오지 않는다.**

## 차단 경로가 두 단계다

이슈 본문은 allowlist만 언급했으나 조사 결과 두 곳에 없다.

1. `commandparse.IssueOpsCommandSpec` — `decision add`의 플래그 spec이 등록되지 않았다
2. `exactIssueOpsOwnerMutation`의 case 목록 — `decision add`가 없다

`exactIssueOpsOwnerMutation`은 case를 통과한 뒤 spec을 조회하므로, **allowlist만 고치면 spec
조회에서 실패해 여전히 거부된다.** 두 곳을 함께 고친다.

## 왜 안전한가

`addIssueOpsDecisionLocked`(issueops_decision.go:117-127)는 **append-only**다.

```go
record.Decisions = append(record.Decisions, IssueOpsDecision{...})
return touchAndWriteIssueOps(stateRoot, record)
```

`phase`·`lease`·`execution`을 건드리지 않는다. 그리고 `addIssueOpsDecision`은
`validateWorkspacePreparationMutation`을 부르는데, 그것은 `validateExecutionMutation`으로
위임되어 **다른 owner mutation과 같은 홀더 검증**을 쓴다.

즉 새 권한을 만드는 것이 아니라 이미 있어야 했던 것을 채우는 것이다.

## 변경

- `IssueOpsCommandSpec`에 `decision add` 등록. CLI가 받는 플래그 전체를 담고
  `--alternative`·`--affected-link`·`--affected-artifact`를 repeatable로 표시한다
- `exactIssueOpsOwnerMutation`의 case 목록에 `"decision add"` 추가

actor 플래그 규율(`--id --host --session-id --cwd`)은 다른 owner mutation과 같다.

## 비범위

- allowlist에 다른 명령을 함께 넣는 것. 각 명령의 안전성은 따로 판단해야 한다.
- 결정 스키마 변경.

## 검증

```bash
go test ./internal/core/lifecycle/... -count=1
go test ./internal/core/commandparse/... -count=1
go test ./... -count=1
```

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
