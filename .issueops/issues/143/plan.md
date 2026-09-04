# 143 — abandon 삭제 CAS의 lease 조건 정합

이슈: https://github.com/m16khb-org/issueops/issues/143
사이클: io-3ff9671412bd
브랜치: `143-abandon-deletion-cas` (base `main` @ a752119)

## 문제

#140이 게이트 ③의 lease 조건을 홀더 유무로 바꿨는데, 삭제 직전 임계구역 재검사는
여전히 `released`만 허용한다. `claimable` 레코드가 **모든 게이트를 통과한 뒤 삭제
단계에서 거부된다.**

실측(#142, 사이클 `io-74114423db59`): preview가 `missing` 없이 fingerprint를 발급하고
apply 명령까지 알려준 뒤, 그 명령이 `abandon authority changed before deletion CAS`로
거부됐다. 실제로는 아무것도 바뀌지 않았다.

## 원인

같은 조건을 두 곳에서 각각 표현하고 있었다. #140이 한쪽만 고쳤다.

## 변경

삭제 CAS가 `cleanupAbandonLeaseHoldsWriter`를 쓰게 한다. #140이 그 함수를 이미
만들었으므로 조건식 하나를 교체한다.

`phase`·`RemoteArtifact`·children 검사는 그대로 둔다 — 그것들은 fingerprint 계산
이후 실제로 바뀔 수 있는 권위 필드다.

## 검증

```bash
go test ./internal/core/issueops/... -count=1
go test ./... -count=1
```

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
