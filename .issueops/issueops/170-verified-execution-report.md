# #170 검증 보고서 — lease 막다른 골목

lifecycle: `io-6184f20c2d66`
issue: https://github.com/m16khb-org/issueops/issues/170
branch: `170-lease-deadend` (base `d18127cf6ec2fba9a3ae9121a98bc97246313b5e`)

## 판정

| AC | 판정 | 증거 |
|---|---|---|
| AC-01 살아 있는 홀더의 자기-revoke 거부 + `release` 안내 | 충족 | `TestExecutionRevokeRefusesTheLiveHolderItself` |
| AC-02 죽은 홀더 뺏기는 그대로 | 충족 | `TestExecutionRevokeStillTakesOverADeadHolder` |
| AC-03 `revoking`에 나가는 문이 있다 | 충족 | AC-01이 그 상태 진입을 막는다 — 아래 |
| AC-04 `reset-legacy` 가드 분류 | 충족 | `TestResetLegacyObservationIsAdmittedWhileAuthorityIsActive` 외 1 |
| AC-05 writer 없는 lease에서 거짓 성공 없음 | 충족 | `TestPrepareDoesNotReportSuccessWithoutAWriter` (2) |
| AC-06 멱등·거부 경로 유지 | **정정** | 아래 |
| AC-07 RED 선행 | 충족 | 아래 |

## RED

```
--- FAIL: TestExecutionRevokeRefusesTheLiveHolderItself
    살아 있는 홀더의 자기-revoke는 나갈 문이 없는 상태를 만든다
--- FAIL: TestPrepareDoesNotReportSuccessWithoutAWriter/released
    writer 없는 lease에 ok:true를 주면 거짓 성공이다: {OK:true ...}
--- FAIL: TestPrepareDoesNotReportSuccessWithoutAWriter/claimable
```

가드 결함은 `TestResetLegacyObservationIsAdmittedWhileAuthorityIsActive`가 RED였다.

## 변경

### ① 자기-revoke 거부 (`execution_lease.go`)

`refuseSelfRevoke`를 `sameNativeActor` 옆에 추가하고 `ExecutionReplaceRevoke` 분기에서
호출한다. 요청자와 홀더의 identity가 같고 그 프로세스가 `live`면 거부하고 `release`를
안내한다.

생존 판정은 `finalize`가 쓰는 `inspectNativeProcessReceipt`와 같은 함수다. 두 곳이 같은
기준을 봐야 한쪽은 revoke를 막는데 다른 쪽은 finalize를 막는 교착이 안 생긴다. 판정이
실패하거나 `live`가 아니면 통과시킨다 — 그것이 지금 동작이고, 죽은 홀더 뺏기와 제3자
revoke를 막지 않는다.

### ② `reset-legacy` 가드 분류 (`lifecycle_execution_guard.go`)

`--preview`/`--status`만 읽기 허용 목록에 넣었다. mutation 경로는 넣지 않았다 — 아래 정정
참조.

### ③ writer-없음 진단 (`execution_prepare.go`)

`executionWriterAbsentNextCommand`가 lease 상태별로 다른 해소 명령을 준다.

| 상태 | 안내 |
|---|---|
| `claimable` | `execution claim --generation N --claim-token-file <token>` |
| `released` | `execution replace --expected-generation N --reseed --confirm` |
| `revoking` | `execution replace --expected-generation N --finalize-preview` |

`prepare`가 lease를 잡아 주지는 않는다. 그것은 `claim`의 일이고, 두 책임을 섞으면 claim 토큰
계약이 흐려진다.

## 이슈 본문에서 정정한 것 둘

### `reset-legacy`는 갇힘을 풀지 못한다

이슈 본문 ②가 그것을 "①의 유일한 탈출구가 될 수 있는 명령"이라고 썼다(단서는 달았다).
확인 결과 `drain-cycle`은 schema v0 사이클 전용이고 — `legacy cycle`, `manifest`를 다룬다 —
v1 lease가 갇힌 상태에 적용되지 않는다.

그래서 가드 분류의 근거를 바꿨다. **탈출구가 아니라 진단 명령이기 때문**이다.
`--preview`/`--status`는 schema 상태를 읽기만 하는데 그것이 authority 활성 중 막혀 있었다.
mutation 경로는 열지 않았다 — 갇힘을 풀지도 못하는데 파괴 경로를 여는 것은 근거가 없다.

### AC-06 후반부가 틀렸다

"`active`이고 홀더가 다르면 지금처럼 거부한다"고 썼는데, **`prepare`는 지금도 거부하지
않는다.** 그것은 워크스페이스 준비 명령이고 lease를 잡지 않으므로 홀더가 누구인지가 결과를
바꾸지 않는다. 실제 mutation은 가드와 core의 lease 검사가 막는다.

테스트를 실제 계약에 맞게 고쳤다 — `TestPrepareStaysAvailableWhileAnotherSessionHolds`.

또 하나: writer 검사를 `--confirm`일 때만 한다. `preview`는 상태 조회이고, 거기서 막으면
갇힌 상태를 진단할 수단이 사라진다(`TestPreparePreviewStaysAvailableWithoutAWriter`).

### AC-03을 충족하는 방식

`revoking`에서 나가는 문을 새로 만들지 않았다. 대신 **살아 있는 홀더가 그 상태에 들어가지
못하게** 했다. 죽은 홀더를 뺏어 들어간 `revoking`은 `finalize`로 나갈 수 있다 — 홀더가
죽었으므로 그 게이트가 통과한다.

즉 나갈 문이 없던 조합(`revoking` + 홀더 live)이 만들어지지 않는다.

## 검증

```
go build ./...                                     성공
go test ./internal/core/issueops/... -count=1      PASS
go test ./internal/core/lifecycle/... -count=1     PASS
go test ./... -count=1                             PASS (전 패키지)
```

## 남긴 것

- `previewExecutionReplacement`가 `revoking`을 허용하지 않아 그 상태에서 preview가 안 된다.
  같은 성질이나 AC에 없고, 갇힘 조합이 사라지면 필요성도 준다. 후속으로 남긴다
