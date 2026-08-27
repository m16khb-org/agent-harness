# 140 — dispatch 단계 사이클의 정리 경로

이슈: https://github.com/m16khb/agent-harness/issues/140
사이클: io-f5e86f3d7a52
브랜치: `140-abandon-claimable-lease` (base `main` @ e70bdad)

## 진단

실측한 다섯 단계 중 **둘은 게이트 ③이 만든 우회**다.

| 단계 | 원인 | 정당한가 |
|---|---|---|
| 1. `execution reconcile` 완주 | 게이트 ⑤(pending kind) | 예 |
| 2. `execution claim` | 게이트 ③이 `claimable` 거부 | **아니다** |
| 3. `execution release` | 2를 되돌리려고 | **아니다** |
| 4. `orca worktree remove` | 게이트 ⑥·⑨ | 예 |
| 5. `cleanup abandon` | 목적 | 예 |

2·3은 lease를 `claimable → active → released`로 한 바퀴 돌릴 뿐 아무것도 정리하지 않는다.

## 왜 `claimable`을 거부할 근거가 없나

게이트 ③의 주석은 `revoking`만 설명한다. 모델을 보면 `claimable`은 홀더 부재가 강제된다.

```go
case LeaseStatusClaimable:
    if lease.Holder != nil || !validSHA256(lease.ClaimTokenSHA256) {
        return fmt.Errorf("claimable lease requires no holder and one token hash")
    }
```

`claimable`과 `released`는 홀더 부재를 공유한다. 거부 대상은 홀더를 가진 `active`와 fenced holder를 가진 `revoking`이다.

## 안전판은 다른 게이트가 맡는다

`claimable`을 허용해도 abandon이 통과하려면 나머지를 모두 지나야 한다.

- ⑤ pending external intent
- ⑥·⑦ 로컬 워크트리·브랜치 잔여
- ⑨ orca 자원 잔여(#136)

실제로 열리는 것은 **홀더 없고 pending 없고 워크트리·브랜치 없고 orca 자원도 없는** 레코드뿐이다. claim token 파일은 canonical worktree 안에 있고 게이트 ⑥이 그 부재를 이미 요구하므로 함께 사라진 뒤다.

## 변경

1. 게이트 ③이 홀더 부재를 기준으로 판정한다. `claimable`·`released` 허용, `active`·`revoking` 거부. 판단 근거를 주석에 남긴다.
2. 게이트 ⑤가 차단할 때 kind별 다음 명령을 지시한다.
3. `claimable` 허용이 여는 조합을 회귀 테스트로 고정한다.

## 비범위

- 게이트 ⑤의 kind allowlist 확장. 기존 주석이 명시적으로 경고한 방향이며, `worktree_create` 단계의 payload 불변식에 기대는 안전성 분석을 무너뜨린다.
- `execution prepare --mode orca`의 실패 원인 조사.
- `execution reconcile`의 단계 전진 로직.

## 검증

```bash
go test ./internal/core/issueops/... -count=1
go test ./... -count=1
```

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
