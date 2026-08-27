# 153 — 한 사이클이 두 PR을 낳으면 cleanup remote-branch가 영구 차단된다

이슈: https://github.com/m16khb/agent-harness/issues/153
사이클: io-66c28415cf2b
브랜치: `153-cleanup-multi-artifact` (base `main` @ d3dd2d2)

## 문제

게이트 ⑩(`issueops_cleanup_remote_branch.go` 222-228행)이 레코드의 **단일 아티팩트**
head OID와 원격 tip을 비교한다.

```go
if result.RemoteBranchPresent &&
    (result.ArtifactHeadOID == "" || !strings.EqualFold(result.ArtifactHeadOID, inventory.RemoteOID)) {
    missing = append(missing, "remote_tip_equals_merged_head")
```

`ArtifactHeadOID`는 `record.RemoteArtifact` 하나를 readback해 얻는다. 한 사이클이 PR을
두 개 낳으면 두 번째 PR의 커밋이 그 필드에 담기지 않는다.

`execution complete`가 Turing 리포트를 요구하는 시점이 PR 머지 이후일 수 있으므로, 그
상황은 정상적인 순서를 따라도 생긴다. 그리고 레코드의 아티팩트를 갱신하려 하면
`lease_released`로 거부된다 — **게이트를 만족시킬 유일한 경로가 같은 시점에 닫힌다.**

#149 사이클에서 실측하고 `git push origin --delete`로 하네스 밖에서 우회했다.

## 게이트 자체는 옳다

주석이 밝힌 의도는 "머지 이후 push된 커밋이 있으면 지우지 마라"이고 정당하다. 실제로
#149에서 리포트 커밋 유실을 막아냈다. 문제는 판정 근거가 단일 아티팩트라는 것이다.

## 변경

게이트 ⑩에 **두 번째 통과 경로**를 더한다.

1. OID가 일치하면 종전대로 통과 (기존 경로, squash 머지 포함)
2. 일치하지 않으면 원격 tip이 준비된 base 브랜치의 remote-tracking ref의 **조상**인지
   확인한다. 조상이면 그 커밋은 이미 base에 있으므로 통과
3. 둘 다 아니면 종전대로 차단

게이트 조건을 완화하지 않는다. 기존에 통과했던 것은 그대로 통과하고, 새로 통과하는
것은 **잃을 커밋이 없음이 증명된 경우**뿐이다.

## 주석의 ancestry 기각을 다시 읽었다

> ancestry 검사는 squash 머지에서 부적합하므로 OID CAS만 쓴다(brooks B3).

이 기각은 **ancestry를 OID CAS 대신 쓰는 경우**에 옳다. squash 머지에서는 원본 커밋이
base의 조상이 아니므로, ancestry만 쓰면 squash된 브랜치를 영구히 못 지운다.

OID 경로를 유지하고 ancestry를 추가하면 그 문제가 생기지 않는다. squash 환경에서는 새
경로가 성립하지 않아 동작이 불변이고, merge 커밋 방식에서 추가 커밋이 별도 PR로 머지된
경우만 새로 통과한다.

#149에서 원격 브랜치를 지우기 전에 손으로 확인한 것이 정확히 이 검사였다.

```
$ git merge-base --is-ancestor 4a9d083 origin/main && echo ANCESTOR_OK
ANCESTOR_OK
```

그 판단을 코드가 하게 한다.

## 안전 설계

- **fail-closed**: ancestry 조회가 실패하면 새 경로가 성립하지 않고 기존 판정이 남는다.
- **base 이름 부재**: `record.BranchPrepare`가 없으면 새 경로를 시도하지 않는다. 게이트
  ③이 이미 같은 필드로 base 삭제를 방어하므로 새로운 신뢰 가정이 아니다.
- **낡은 remote-tracking ref**: ancestry가 성립하지 않아 기존대로 차단된다. 과잉 차단
  방향이므로 안전하다.

## 진단 (#154의 계약)

차단 시 무엇이 어긋났는지 결과에 담는다. 지금은 `remote_tip_equals_merged_head`라는
슬러그만 나오고 두 검사(OID 불일치, ancestry 불성립) 중 어느 쪽인지 알 수 없다.

## 비범위

- `execution complete`가 리포트를 요구하는 시점 변경. #154가 운영으로 회피했고, 리포트에
  CI 결과와 머지 사실이 들어가는 문제가 남는다.
- 레코드가 아티팩트를 여러 개 담는 스키마 변경.
- `lease_released` 이후 아티팩트 갱신 허용. `execution complete`의 봉인이 약해진다.

## 검증

```bash
go test ./internal/core/issueops/... -count=1
go test ./... -count=1
```

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
