# #153 Turing 수용 리포트

사이클: `io-66c28415cf2b`
이슈: https://github.com/m16khb/agent-harness/issues/153
브랜치: `153-cleanup-multi-artifact` (base `main` @ d3dd2d2)

## 무엇을 바꿨나

게이트 ⑩(`remote_tip_equals_merged_head`)에 **두 번째 통과 경로**를 더했다.

1. OID 일치 — 기존 경로, squash 머지 포함해 불변
2. 원격 tip이 준비된 base의 remote-tracking ref의 조상 — 신규
3. 둘 다 아니면 종전대로 차단

게이트 조건을 완화하지 않았다. 새로 통과하는 것은 **잃을 커밋이 없음이 증명된
경우**뿐이다.

## 수용 기준 판정

| AC | 판정 | 근거 |
|---|---|---|
| AC-01 두 PR 상황에서 완결 | **달성** | `TestRemoteBranchDeleteAcceptsTipAlreadyInBase` |
| AC-02 미머지 커밋은 차단 | **달성** | `TestRemoteBranchDeleteStillBlocksTipOutsideBase` |
| AC-03 squash 동작 유지 | **달성** | `TestRemoteBranchDeleteKeepsOIDPathIndependentOfAncestry` — OID 일치 시 `merge-base`를 아예 묻지 않음을 고정 |
| AC-04 차단 진단 | **달성** | `RemoteTipReachedBase`로 어느 근거로 통과했는지 밝힌다 |
| AC-05 RED 선행 | **달성** | `result.RemoteTipReachedBase undefined` 컴파일 실패에서 출발, 구현 후 6건 GREEN |

fail-closed 세 경로도 함께 고정했다.

- `TestRemoteBranchDeleteFailsClosedWhenAncestryIsUnobservable` — 조회 실패는 통과가 아니다
- `TestRemoteBranchDeleteSkipsAncestryWithoutPreparedBase` — base를 모르면 묻지 않는다
- `TestRemoteBranchAncestryComparesAgainstRemoteTrackingBase` — 로컬 ref가 아니라 `refs/remotes/origin/<base>`와 비교한다

## 주석의 ancestry 기각을 다시 읽었다

> ancestry 검사는 squash 머지에서 부적합하므로 OID CAS만 쓴다(brooks B3).

이 기각은 **ancestry를 OID CAS 대신 쓰는 경우**에 옳다. squash에서는 원본 커밋이 base의
조상이 아니므로 ancestry만 쓰면 squash된 브랜치를 영구히 못 지운다.

OID 경로를 유지하고 추가 경로로 쓰면 그 문제가 생기지 않는다. `TestRemoteBranchDelete
KeepsOIDPathIndependentOfAncestry`가 그것을 테스트로 고정한다 — OID가 일치하면
`merge-base`를 호출조차 하지 않는다.

#149에서 원격 브랜치를 하네스 밖에서 지우기 전에 손으로 확인한 것이 정확히 이 검사였다.

```
$ git merge-base --is-ancestor 4a9d083 origin/main && echo ANCESTOR_OK
ANCESTOR_OK
```

그 판단을 코드가 하게 했다.

## 구현 중 드러난 위험: fake가 게이트를 무력화하고 있었다

기존 테스트 `TestCleanupRemoteBranchFailsClosed/tip_pushed_after_merge`가 깨졌다.
원인을 추적하니 `fakeRemoteBranchGit.run`의 default가 **exit 0을 반환**했다.

```go
// 고치기 전
return 0, ""    // 모르는 명령도 성공
```

그래서 새 ancestry 검사가 조용히 통과했다. 게이트는 fail-closed인데 fake는 fail-open이었던
것이다. **이 fake에 새 git 검사를 추가하는 누구든 같은 함정을 밟는다.**

fake의 규율을 게이트와 같게 맞췄다 — 모르는 명령은 exit 128, `merge-base`는 명시적으로
"조상 아님"(exit 1)을 반환한다. 후자는 그 fake의 시나리오(머지 후 push된 커밋)와 정확히
일치한다.

## 검증

```
go test ./internal/core/issueops/ -run "RemoteBranch|CleanupRemoteBranch" -count=1
go test ./... -count=1
```

RED는 `result.RemoteTipReachedBase undefined`로 컴파일 실패에서 출발했다. 구현 후 신규
6건과 기존 remote-branch 테스트 전부 GREEN, 전체 회귀 통과.

## 비범위

- `execution complete`가 리포트를 요구하는 시점 변경. #154가 운영으로 회피했고, 리포트에
  CI 결과와 머지 사실이 들어가는 문제가 남는다.
- 레코드가 아티팩트를 여러 개 담는 스키마 변경.
- `lease_released` 이후 아티팩트 갱신 허용. `execution complete`의 봉인이 약해진다.

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
메인 에이전트가 직접 반론을 수행했고 저자와 검토자가 분리되지 않았다.
