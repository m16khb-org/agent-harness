# #170 lease 막다른 골목

이슈: https://github.com/m16khb-org/issueops/issues/170
lifecycle: io-6184f20c2d66
branch: 170-lease-deadend (base d18127cf6ec2fba9a3ae9121a98bc97246313b5e)

## 결함 셋

### ① 홀더가 자기 자신을 revoke하면 갇힌다

`execution_lease.go:307`

```go
case ExecutionReplaceRevoke:
    if lease.Status != model.LeaseStatusActive || strings.TrimSpace(req.Reason) == "" {
        return fmt.Errorf("revoke requires an active lease and a reason")
    }
```

**요청자가 홀더인지 보지 않는다.** 그것은 설계다 — 죽은 홀더를 제3자가 뺏는 경로이기
때문이다. 그런데 홀더 자신도 호출할 수 있고, 그러면 나갈 문이 전부 막힌다.

| 문 | 조건 | 결과 |
|---|---|---|
| `release` | `active` (`:202`) | 거부 — 지금은 `revoking` |
| `replace --reseed` | `released`/`claimable` (`:349`) | 거부 |
| `replace --finalize` | 이전 홀더 `dead` (`:571`) | 거부 — 홀더가 나 자신이고 살아 있다 |
| `claim` | `claimable` (`:145`) | 거부 |

`previewExecutionReplacement`(`:257`)도 `revoking`을 허용하지 않아 진단조차 어렵다.

**이 세션에서 실제로 겪었다.** Claude Code 세션을 재시작해야 풀렸다.

### ② `reset-legacy`가 가드 어디에도 없다

`lifecycle_execution_guard.go`의 세 목록 — 읽기 허용, typed control plane, owner mutation —
어디에도 없다. mutation authority가 활성인 동안 `unclassified shell command`로 거부되고,
갇힌 상태에서 authority는 항상 활성이다.

### ③ `released` lease를 `prepare`가 재claim하지 않는다

`execution_prepare.go:82-90`이 `record.Execution`이 있으면 lease 상태를 보지 않는다. #167이
같은 분기의 **모드 불일치**를 고쳤고 **lease 상태**는 별개 계약이라 남겼다.

```
$ execution release --generation 1 ...     → status: released
$ execution prepare --mode direct --confirm ...
{ "ok": true, "execution": { "lease": { "status": "released" } } }   ← 거짓 성공
```

## 설계

| # | 변경 |
|---|---|
| ① | `revoke`가 요청자와 홀더가 같고 그 프로세스가 살아 있으면 거부하고 `release`를 안내한다. 홀더가 죽었거나 요청자가 제3자면 지금처럼 동작한다 |
| ② | `reset-legacy`를 `sync-base`와 같은 typed control plane으로 분류한다. 가드는 분류만 하고 안전은 core가 본다 |
| ③ | `prepare`가 lease 상태를 보고 `released`/`claimable`이면 거짓 성공 대신 해소 명령을 안내한다 |

①의 생존 판정은 `inspectNativeProcessReceipt`를 재사용한다 — `finalize`(`:567`)가 이미 쓰는
함수라 두 곳이 같은 기준을 본다.

### `prepare`가 lease를 잡지 않는 이유

`prepare`는 워크스페이스 준비 명령이고 lease 획득은 `claim`의 일이다. 두 책임을 섞으면 claim
토큰 계약이 흐려진다. 그래서 ③은 "새 lease를 잡는다"가 아니라 "거짓 성공을 없애고 다음
명령을 안내한다"이다.

## 수용 기준

- AC-01 살아 있는 홀더의 자기-revoke를 거부하고 `release`를 안내한다
- AC-02 죽은 홀더를 뺏는 revoke는 그대로 동작한다
- AC-03 `revoking`에서 나가는 문이 최소 하나 존재함을 테스트가 고정한다
- AC-04 `reset-legacy`가 가드에서 분류되고 근거가 주석에 남는다
- AC-05 `released`/`claimable`에서 `prepare`가 거짓 성공을 주지 않는다
- AC-06 `active`이고 홀더가 같으면 멱등 성공, 다르면 거부
- AC-07 RED가 세 결함을 각각 실증한다

## 검증

```
go test ./internal/core/issueops/... -count=1
go test ./internal/core/lifecycle/... -count=1
go test ./... -count=1
```

## 비범위

- 모드 불일치 처리 — #167이 다뤘다
- `reset-legacy`의 기능 변경 — 가드 통과 여부만 다룬다
- `revoking`에서 preview가 막히는 것 — 같은 성질이나 AC에 없다. 갇힘이 풀리면 필요성도
  줄어든다. 후속으로 남긴다
