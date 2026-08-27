# 152 — auto가 실행 불가능한 orca를 고르지 않게 한다

이슈: https://github.com/m16khb/agent-harness/issues/152
사이클: io-d724112b669b
브랜치: `152-orca-auto-fallback` (base `main` @ f9269cf)

## 문제

`--mode auto`가 orca를 고른 뒤 브랜치 이름 충돌 사전 확인(#149·#154)에 막힌다.
IssueOps 정식 순서(`gh issue develop` → `branch prepare`)를 따르면 **항상** 그렇게
되므로, orca CLI가 설치된 환경에서 auto가 사실상 쓸 수 없다.

auto의 목적은 실행 가능한 모드를 고르는 것이다. 지금은 실행 불가능한 모드를 골라놓고
사용자가 손으로 `--mode direct`를 다시 주게 한다.

## 범위: 후보 B

#152 이슈에 후보 A(순서 뒤집기)와 B(auto 폴백)를 적었다. **B로 좁힌다.**

후보 A의 비용을 실측했다. `BranchPrepare`에 의존하는 프로덕션 파일이 14개다.

```
issueops_umbrella_topology.go   execution_prepare.go       execution_remote.go
execution_complete.go            execution_owner_context.go execution_sync_base.go
issueops_cleanup_finish.go       issueops_cleanup_remote_branch.go
issueops_phase_ledger.go         issueops_remote_sync.go    issueops_pr_readiness.go
implementation/evidence.go       branchprepare/             model/types.go
```

`executionWorkspaceRequest`(execution_prepare.go:360)가 `BranchPrepare.BaseSHA`를 필수로
요구하고, 우산 브랜치 위상 게이트(#129)도 같은 필드를 읽는다. 계약 근간 변경이므로 별도
결정으로 남긴다 — 이슈 #152는 열린 채 후보 A를 담는다.

B는 이미 세 번 쓰인 폴백 패턴에 네 번째 사유를 더하는 것이고, 이슈의 **AC-04(auto 동작을
계약으로 정한다)를 직접 충족한다.** #149에서 그 AC를 정하지 않은 채 남겼다.

## 변경

`resolveExecutionPrepareMode`가 orca를 확정하기 전에 `ensureOrcaBranchIsFree`를 부른다.

- **충돌 + auto** → direct와 폴백 코드를 돌려준다
- **충돌 + 명시 orca** → 종전대로 실패한다. 사용자가 지정한 모드를 대신 바꾸지 않는다
- **충돌 없음** → 종전대로 orca

기존 폴백 세 사유와 같은 형태다.

```go
// execution_prepare.go 326-355행에 이미 있는 패턴
if requested == ExecutionModeAuto {
    return string(model.ExecutionModeDirect), code, probeReq, nil
}
return "", "", probeReq, fmt.Errorf(...)
```

## 사전 확인 함수를 재사용한다

폴백 판정과 실제 차단이 **같은 함수**를 써야 한다. 두 곳에 같은 조건을 따로 쓰면 한쪽만
고쳐져 auto가 direct로 갔는데 direct도 막히거나 그 반대가 된다.

`ensureOrcaBranchIsFree`는 #154에서 로컬과 원격 refs를 모두 보게 됐다. 그 기준을 그대로
쓴다.

## 폴백은 조용해서는 안 된다

`fallback_code`가 왜 direct가 됐는지 말해야 사용자가 orca를 쓰려면 무엇을 바꿔야 하는지
안다. 기존 세 사유도 같은 방식이다(`gitlab_issue_metadata_unsupported`,
`orca_adapter_unavailable`, `orca_probe_failed`).

## 비범위

- linked-branch 순서 계약 자체 변경(후보 A) — 14개 프로덕션 파일과 여러 게이트
- orca가 기존 브랜치를 채택하게 만드는 것 — orca CLI에 수단이 없다(#149에서 확인)
- 사전 확인의 판정 기준 변경 — 같은 검사를 앞당겨 쓸 뿐이다

## 검증

```bash
go test ./internal/core/issueops/... -count=1
go test ./... -count=1
```

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
