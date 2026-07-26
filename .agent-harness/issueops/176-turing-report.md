# #176 Turing 리포트 — orca 발행 base 못박기

lifecycle: `io-ad7914ff844e`
issue: https://github.com/m16khb/agent-harness/issues/176
branch: `176-orca-publish-base` (base `187d595c01eb8aa5bf697d00197338c7c836e90f`)

## 판정

| AC | 판정 | 증거 |
|---|---|---|
| AC-01 발행 가능한 경로 | 충족 | `TestGitHubStepsPinTheSealedBaseSHA` |
| AC-02 base 갈림 진단 | 충족 | `TestGitHubStepsFallBackWhenTheBaseSHAIsAbsent` |
| AC-03 durable state와 발행 커밋 일치 | 충족 | 봉인 base가 곧 원격 브랜치 base — 실측으로 확인 |
| AC-04 direct 모드 유지 | 충족 | 같은 경로, 봉인 base가 곧 종전 값 |
| AC-05 `#163` 순서 안내 반영 | 충족 | `TestGitHubStepsExplainTheOrcaOrdering` (두 분기 모두) |
| AC-06 RED 선행 | 충족 | 아래 |

## RED

```
--- FAIL (build): too many arguments in call to Steps
    have (string, string, string, string, string)
    want (string, string, string, string)
```

시그니처 확장 후 가드 테스트에서 두 번째 RED:

```
--- FAIL: TestLinkedBranchOIDPathIsAdmitted/mutation
    Reason: shell substitution or wrapper target is not statically resolvable
```

## 설계 재검토 — 이슈 본문의 후보 셋 중 둘이 약해졌다

| 후보 | 판정 | 근거 |
|---|---|---|
| 1. orca base를 최신 main으로 재봉인 | 재발 여지 | 봉인·링크 시점 사이 main 진행. 이 세션에서 네 번 진행했다 |
| 2. `gh issue develop`에 base SHA | **CLI로는 불가** | `--help`: `--base string   Name of the remote branch` |
| 3. `sync-base` 조기 허용 | 계약 위반 | `execution_sync_base.go:176-178`이 "완결 이후 표면이다(AC-02)"를 근거로 명시 |

이슈 본문에 그 근거와 새 결론을 기록했다.

### 후보 2를 GraphQL 경로로 정정

`CreateLinkedBranchInput.oid`는 필수 필드다(인트로스펙션):

```
oid: NON_NULL GitObjectID
```

`gh issue develop`이 `--base` 브랜치 HEAD를 조회해 그것을 채우는 것뿐이므로, 봉인 SHA를 직접
채우면 갈림이 원리적으로 생기지 않는다.

**실측**: 현재 main이 아닌 임의 SHA(`2a56f2cc...`)로 링크 브랜치를 만들었고 정확히 그 SHA에
생성·연결됐다. 실측 브랜치는 정리했다.

## 변경

`internal/core/issueops/branchprepare/branch_prepare.go`

`Steps`에 `baseSHA` 인자 추가, GitHub case를 `githubSteps`로 분리.

| baseSHA | 단계 |
|---|---|
| 있음 | MCP 부재 → node id 조회 → `createLinkedBranch`(oid 못박음) → fail (4) |
| 없음 | MCP 부재 → `gh issue develop` + 이유 안내 → fail (3) |

`internal/core/lifecycle/lifecycle_execution_guard.go`

`exactProviderBranchLink`에 `gh api` 두 형태 추가. query 본문에 `createLinkedBranch`가 있는지
확인하고 플래그 위치·개수를 고정하므로 임의 GraphQL이 통과하지 않는다.

## 구현에서 확인한 것

### 인용 전제 — 가드 수정이 필요 없었다

첫 시도에서 mutation 명령이 `shell substitution ... not statically resolvable`로 거부됐다.
`HasActiveParameterOrTildeExpansion`을 읽어 원인을 특정했다 — `tokens.go:344-350`이 단일 인용
안의 문자를 건너뛰므로 `$` 검사(`:365`)에 도달하지 않는다.

**즉 단일 인용을 쓰면 통과한다.** 실측할 때 `'...'`를 썼기 때문에 성공했던 것이고, 테스트가
인용 없는 문자열을 쓴 것이 실패 원인이었다. 가드를 약화하지 않고 표기 전제를 `Description`에
명시하는 것으로 해결했다(`TestGitHubStepsWarnAboutQuotingTheQuery`).

### 기존 테스트 정정

`TestStepsKeepTheirExistingShape`가 "3단계"를 불변식으로 검사했다. GitHub이 4단계가 되므로 그
전제를 고쳤다 — 불변식은 "첫 단계가 MCP 경로, 마지막이 fail, 사이에 provider CLI를 쓰는
`fallback_api`가 있고 Order가 1부터 연속"이다. 단계 **수**는 불변식이 아니다.

## 검증

```
go build ./...                                              성공
go test ./internal/core/issueops/branchprepare/ -count=1    PASS
go test ./internal/core/lifecycle/ -count=1                 PASS
go test ./... -count=1                                      PASS (전 패키지)
```

## 비범위

- GitLab 경로 — `ref`에 SHA를 받는지 확인하지 않았다. 확인 없이 바꾸면 `#136`의 추측 오류를
  반복한다. 후속으로 남긴다
