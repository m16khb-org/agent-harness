# #177 검증 보고서 — 가드 allowlist 누락 2건

lifecycle: `io-e538dec0f98d`
issue: https://github.com/m16khb-org/issueops/issues/177
branch: `177-guard-allowlist-gaps` (base `2a56f2cc4d2e6b7b4fa99e3cdd71e3673ae060d2`)

## 판정

| AC | 판정 | 증거 |
|---|---|---|
| AC-01 `cleanup orphan` 분류 | 충족 | `TestCleanupOrphanTypedControlPlaneIsAdmitted` (preview·apply) |
| AC-02 `gh issue develop` 분류 | 충족 | `TestGHIssueDevelopIsAdmittedFromTheCanonicalWorktree` (develop·list) |
| AC-03 안내와 실행 가능성 일치 | 충족 | AC-01·AC-02로 충족 |
| AC-04 다른 mutation 미개방 | 충족 | `TestOtherGHIssueMutationsStayBlocked` (5), `TestCleanupOrphanUnregisteredShapeStaysUnclassified` (2), 기존 가드·파서 테스트 전체 통과 |
| AC-05 RED 선행 | 충족 | 아래 |

## RED

```
--- FAIL: TestCleanupOrphanTypedControlPlaneIsAdmitted/preview
    Reason: unclassified shell command is blocked while IssueOps mutation authority is active
--- FAIL: TestCleanupOrphanTypedControlPlaneIsAdmitted/apply
    Reason: mutation requires the current write lease for IssueOps execution ...
--- FAIL: TestGHIssueDevelopIsAdmittedFromTheCanonicalWorktree/develop
    Reason: unclassified shell command is blocked ...
--- FAIL: .../develop_list
```

`TestOtherGHIssueMutationsStayBlocked`와 `TestCleanupOrphanUnregisteredShapeStaysUnclassified`는
처음부터 통과했다 — 변경이 표면을 넓히지 않았는지 고정하는 테스트다.

## 변경

`internal/core/lifecycle/lifecycle_execution_guard.go`

| # | 내용 |
|---|---|
| ① | `cleanup orphan`을 typed control plane case 목록에 추가 |
| ② | `exactProviderBranchLink`를 새로 두고 `executionObservation`에서 호출 |

### ②를 `ExactReadOnlyShellCommand`에 넣지 않은 이유

**그 이름이 계약이다.** `gh issue develop`은 provider에 브랜치를 만들므로 읽기가 아니다. 별도
판정으로 두고 통과 근거를 주석에 남겼다 — "읽기라서"가 아니라 "IssueOps가 그 명령을 지시하고
형태를 정확히 고정할 수 있어서"다.

원래 design review는 `exactReadOnlyGHCommand`에 `issue` 분기를 추가하는 것이었다. 구현에서
그 함수군의 이름이 다른 곳의 판단 기준이 될 수 있다고 보아 분리했다.

### 형태를 좁히는 방식

두 형태만 인정한다:

```
gh issue develop <number> --repo <slug> --base <branch> --name <branch>
gh issue develop --list <number> --repo <slug>
```

토큰 수와 위치를 고정하므로 `create`·`close`·`edit`·`comment`가 통과하지 않고, 열거 밖 플래그가
하나라도 붙으면 거부한다. `exactReadOnlyGHPRCommand`가 같은 방식으로 좁힌 선례다.

## 이 결함이 실제로 막았던 것

`#147`의 cleanup이 `cleanup orphan` 차단 때문에 완결되지 못했다. `cleanup abandon`이 그 명령을
`orca_residue_error`에서 안내하는데, 안내받은 명령이 실행되지 않았다.

`#163`이 정한 orca 순서는 `gh issue develop`을 orca 준비 뒤에 실행하라고 하는데, 그 시점에
lease는 활성이고 사용자는 워크트리에 있다. `#147`에서 그 명령을 source root로 나가서 실행해야
했다.

## 검증

```
go build ./...                                     성공
go test ./internal/core/lifecycle/... -count=1     PASS
go test ./internal/core/commandparse/... -count=1  PASS
go test ./... -count=1                             PASS (전 패키지)
```

## 비범위

- `cleanup finish`·`remote-branch`·`abandon` — lease writer가 없을 때만 동작하므로 authority
  활성 중 막히는 것이 계약과 일치한다. 이 세션에서 네 사이클을 그 경로로 정리했다
- `reset-legacy` mutation 경로 — `#170`이 제외한 근거가 유효하다
