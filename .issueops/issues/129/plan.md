# 129 — 우산 사이클의 브랜치 위상 규약

이슈: https://github.com/m16khb-org/issueops/issues/129
사이클: io-c328698d8e39
브랜치: `129-umbrella-branch-topology` (base `main` @ 8401950)

## 확립하려는 위상

```
main
 └─ 78-umbrella                          → PR base: main
     ├─ 79-child (78-umbrella에서 분기)   → PR base: 78-umbrella
     └─ 80-child (78-umbrella에서 분기)   → PR base: 78-umbrella
```

가드(`issueOpsPRTargetBranchBlockReason`)는 이미 이 위상을 전제하고 자식 PR의 base를
`branch_prepare.base_branch`와 대조한다. 없는 것은 **우산이 자체 브랜치를 갖도록 요구하는
게이트**와 **자식의 base가 우산 브랜치를 가리키도록 요구하는 게이트**다.

## 변경 단위

### A. create-child가 우산 브랜치를 요구한다 (AC-01)

`internal/core/issueops`에 순수 판정 함수를 둔다.

```go
// UmbrellaBranchGateReason은 우산 사이클이 자식 작업 항목을 만들 준비가
// 되지 않은 이유를 돌려준다. 빈 문자열이면 통과다.
func UmbrellaBranchGateReason(record IssueOpsRecord) string
```

`runRemoteCreateChild`가 provider 호출 **이전에** 호출한다. 판정을 CLI에 두지 않는 이유는
core 테스트로 계약을 고정하기 위해서다.

거부 메시지는 다음 명령을 정확히 지시한다: `issueops branch prepare`.

### B. 자식 branch prepare가 우산 브랜치를 base로 요구한다 (AC-02)

`branchprepare.Store`에 부모 역조회를 주입한다.

```go
UmbrellaForChildIssue func(repo, childIssueURL string) (model.IssueOpsRecord, bool)
```

`active` 패키지에 스캔을 추가한다(기존 `ListIDs` 패턴 재사용). 부모를 찾으면
`base_branch != 부모.Branch`를 거부한다. **못 찾으면 통과한다** — 자식이 아니거나 우산이
이미 정리된 경우이고, 근거를 잃은 검증이 일상 작업을 막아서는 안 된다.

### C. close-children의 레거시 탈출구 (AC-05, AC-06)

두 층을 함께 바꾼다.

1. **provider**: `CloseChild`의 preview 경로가 원격 hierarchy와 자식 상태를 **best-effort**로
   읽어 `HierarchyVerified`/`AlreadyClosed`/`State`를 채운다. 읽기 실패는 오류가 아니라
   상태 미상이다(기존 dry-run 계약 유지 — gh 부재 환경에서도 성공해야 한다).
2. **core**: `cleanupchildren.ByID`가 `!req.Merged`일 때 즉시 거부하는 대신, 모든 자식을
   preview로 readback해 **전부 확인된 closed**이면 통과시킨다. 하나라도 열려 있거나 상태가
   미상이면 기존 `merge_evidence` 거부를 유지한다(fail-closed).

통과 근거는 결과에 명시한다 — 운영자가 무엇을 근거로 통과했는지 알아야 한다.

### D. 회귀 고정 (AC-03, AC-04)

- 자식 PR의 base가 우산 브랜치가 아니면 기존 가드가 차단한다.
- 우산이 자체 브랜치를 가지면 done 도달 경로가 열린다.

## 비범위

- `doctor` finding 추가 — 이 레코드는 자원을 점유하지 않는다.
- 이미 머지된 #78 계보의 소급 수정 — 불가능하며 필요하지도 않다.

## 검증

```bash
go test ./internal/core/issueops/... -count=1
go test ./internal/core/lifecycle/... -count=1
go test ./internal/adapter/provider/... -count=1
go test ./cmd/issueops/issueopscli/... -count=1
go test ./... -count=1
```

각 AC마다 RED 테스트를 선행한다.

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
메인 에이전트가 직접 반론을 수행했으므로 저자와 검토자가 분리되지 않았다.
