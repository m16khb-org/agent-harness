# #177 가드 allowlist 누락 2건

이슈: https://github.com/m16khb/agent-harness/issues/177
lifecycle: io-e538dec0f98d
branch: 177-guard-allowlist-gaps (base 2a56f2cc4d2e6b7b4fa99e3cdd71e3673ae060d2)

## 결함

정리·준비 명령 둘이 가드 allowlist에 없어 mutation authority가 활성인 동안 실행할 수 없다.
그 상태에서 authority는 항상 활성이므로 **그 명령이 필요한 순간에 정확히 막힌다.**

### ① `cleanup orphan`

`commandparse`의 spec에는 있는데(`issueops.go:182-183`) 가드 세 목록 어디에도 없다. 그 명령의
대상은 "정식 phase를 밟지 못한 사이클의 자원"이고, 그런 사이클은 정의상 authority가 활성인 채로
남는다.

`cleanup abandon`이 그것을 안내하기까지 한다:

```
"orca_residue_error": "... abandon leaves them without an owner, so use
 `agent-harness issueops cleanup finish` or `agent-harness issueops cleanup orphan`"
```

안내받은 명령이 실행되지 않는다. **#147의 cleanup이 이것 때문에 막혔다.**

### ② `gh issue develop`

`exactReadOnlyGHCommand`가 `pr`·`run`만 분기한다(`:541-553`). canonical worktree에서 막히고
source root에서만 된다.

`#163`이 정한 orca 순서는 "orca 준비 뒤 링크를 붙인다"인데 그 시점에 lease는 활성이고 사용자는
워크트리에 있다. `branch prepare`의 `fallback_api`가 이 명령을 그대로 안내하므로 **안내와 실행
가능성이 어긋난다.**

## 설계

| # | 변경 |
|---|---|
| ① | `cleanup orphan`을 typed control plane에 등록 — `sync-base`·`switch-mode`와 같은 계약 |
| ② | `exactProviderBranchLink`를 새로 두고 `executionObservation`에서 호출 |

②를 `ExactReadOnlyShellCommand`에 넣지 않은 이유는 **그 이름이 계약**이기 때문이다.
`gh issue develop`은 provider에 브랜치를 만들므로 읽기가 아니다. 별도 판정으로 두고, 통과 근거를
"읽기라서"가 아니라 "IssueOps가 그 명령을 지시하고 형태를 정확히 고정할 수 있어서"로 남겼다.

### 형태를 좁히는 방식

`exactProviderBranchLink`는 두 형태만 인정한다:

```
gh issue develop <number> --repo <slug> --base <branch> --name <branch>
gh issue develop --list <number> --repo <slug>
```

토큰 수와 위치를 고정하므로 `create`·`close`·`edit`·`comment`가 통과하지 않고, 열거 밖 플래그가
하나라도 붙으면 거부한다. `exactReadOnlyGHPRCommand`가 같은 방식으로 좁힌 선례다.

## 수용 기준

- AC-01 `cleanup orphan`이 authority 활성 중 분류된다 — `TestCleanupOrphanTypedControlPlaneIsAdmitted`
- AC-02 `gh issue develop`이 canonical worktree에서 분류된다 — `TestGHIssueDevelopIsAdmittedFromTheCanonicalWorktree`
- AC-03 안내와 실행 가능성이 일치한다 — 위 둘로 충족
- AC-04 다른 mutation이 함께 열리지 않는다 — `TestOtherGHIssueMutationsStayBlocked`,
  `TestCleanupOrphanUnregisteredShapeStaysUnclassified`, 기존 가드 테스트 전체 통과
- AC-05 RED가 차단을 실증한다

## 비범위

- `cleanup finish`·`remote-branch`·`abandon` — lease writer가 없을 때만 동작하므로 authority
  활성 중 막히는 것이 계약과 일치한다. 이 세션에서 네 사이클을 그 경로로 정리했다
- `reset-legacy` mutation 경로 — `#170`이 제외한 근거(schema v0 전용, v1 갇힘을 풀지 못함)가
  유효하다

## 검증

```
go test ./internal/core/lifecycle/... -count=1
go test ./internal/core/commandparse/... -count=1
go test ./... -count=1
```
