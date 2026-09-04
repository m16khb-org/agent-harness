# 154 — 차단이 원인과 다음 행동을 알려주게 한다

이슈: https://github.com/m16khb-org/issueops/issues/154
사이클: io-88ff9d63699d
브랜치: `154-blocking-diagnostics` (base `main` @ eb94899)

## 문제

차단은 정확한데 **차단 사유와 다음 행동을 알려주지 않는다.** 그래서 매번 소스를 읽어야
통과 방법을 안다. 이 저장소는 라이브러리이므로 그 지식이 개인 메모리에 쌓이면 다른
사용자는 같은 시행착오를 반복한다.

가장 뚜렷한 증거: **설명이 이미 만들어져 있는데 마지막 한 줄에서 버려진다.**

```go
// cmd/issueops/hookcli/hook_pre_tool_use.go
func hookDenyReason(result core.HookPreToolUseDecisionResult) string {
	if result.Deny == nil {
		return result.Reason      // Deny 없으면 설명을 준다
	}
	encoded, err := json.Marshal(result.Deny)
	return string(encoded)        // Deny 있으면 설명을 버린다
}
```

`executionUnsafeMutationReason`(lifecycle 가드 소스 317-338행)이 여섯 종류의 사람이
읽을 사유를 생성하지만 `IssueOpsDenyReason`에 담을 필드가 없어 사라진다.

## 선례

이 저장소는 같은 병을 한 번 진단했다. `IssueOpsDenyReason` 주석 117-121행:

> IdentityMismatch/ObservedActor는 active holder identity 불일치 deny에서만 채워진다.
> **훅이 관측한 값을 에코하지 않으면 owner는 어떤 identity 축을 고쳐야 하는지 알 수
> 없어 추측 재시도만 반복한다**(이슈 #90 발견 4).

그때는 identity 축 하나에만 적용했다. 이번에 사유 자체에 적용한다.

## 변경 대상

1. **`IssueOpsDenyReason`에 `Reason` 추가** — `hookDenyReason`이 버리지 않는다.
2. **`cleanup finish`가 점유 프로세스를 담는다** — `InspectProcesses`가 이미 PID와
   명령명을 수집하는데 개수만 쓰고 버린다. lsof 실패(관측 불가)와 프로세스 존재를
   별도 슬러그로 나눈다.
3. **확정적 해소 명령 안내** — `completion_reflected`는 `remote reflect-completion`
   하나로 정해진다. 상황에 따라 갈리는 missing에는 붙이지 않는다.
4. **`reconcile --preview`가 조회 여부를 밝힌다** — preview는 상수 코드만 반환하고
   orca를 조회하지 않는데 출력에 그 구분이 없다. 그래서 preview 결과를 관측 증거로
   읽게 된다(#99 오진단의 원인).
5. **`ensureOrcaBranchIsFree`가 원격 refs도 본다** — 아래 참조.

## AC-08: #149 수정의 구멍

#149에 사전 확인을 넣으면서 계획서에 이렇게 적었다.

> **원격 전용 브랜치**: orca 동작을 확인하지 못했다. 로컬 기준으로 검사하고 범위를
> 주석에 남긴다.

이번 사이클의 dogfood가 그 미확인 경우를 실측했다. `gh issue develop`은 **원격
브랜치만** 만들고 로컬에는 만들지 않는다. 즉 IssueOps 정식 순서를 따르면 **항상**
그 경우이고, 로컬만 보는 검사는 언제나 통과한다.

```
$ issueops execution prepare --id io-88ff9d63699d --mode auto --confirm
worktree_branch_mismatch: created branch "154-blocking-diagnostics-2"
does not match requested branch "154-blocking-diagnostics"
```

orca는 원격 브랜치를 본다. #149에서 "확인하지 못했다"고 남긴 것의 답이다.

#149 사이클의 테스트가 로컬 브랜치를 만드는 픽스처를 써서 정식 순서를 재현하지
못했고, 그래서 GREEN이 나왔지만 실환경에서 뚫렸다.

**remote-tracking ref로 확인한다.** `git ls-remote`는 prepare를 네트워크에 묶어
오프라인에서 정상 경로를 막는다. 낡은 ref가 오탐을 만들 수 있으므로 메시지가 fetch를
안내한다.

## 제약

- **게이트가 막는 조건 자체를 완화하지 않는다.** 어떤 게이트도 더 통과하기 쉬워지지
  않는다. 왜 막혔는지만 말한다.
- **비밀을 노출하지 않는다.** 원문 명령을 담지 않는다. 분류 결과와 이미 추출된 경로만
  담는다. 토큰이 인자에 있어도 되비추지 않는다.
- **기존 필드를 깨지 않는다.** omitempty로만 더한다.

## 비범위

- `remote_tip_equals_merged_head`의 단일 아티팩트 한계 — #153
- orca 모드와 linked-branch 순서 충돌 자체 — #152. 여기서는 사전 확인의 검사 범위만
  바로잡는다.

## 검증

```bash
go test ./internal/core/issueops/... -count=1
go test ./internal/core/lifecycle/... -count=1
go test ./... -count=1
```

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
