# #176 orca 발행 base 못박기

이슈: https://github.com/m16khb/agent-harness/issues/176
lifecycle: io-ad7914ff844e
branch: 176-orca-publish-base (base 187d595c01eb8aa5bf697d00197338c7c836e90f)

## 결함

`gh issue develop --base <branch>`는 GitHub이 **그 시점** 브랜치 HEAD를 조회해
`CreateLinkedBranchInput.oid`로 쓴다. orca는 봉인된 base SHA에서 로컬 브랜치를 만들므로, 링크를
붙이는 사이 base 브랜치가 진행하면 두 base가 갈리고 push가 non-fast-forward로 거부된다.

그때 해소 경로가 전부 막힌다 — 봉인 가드가 `merge`를, 안전 훅이 force push를, `sync-base`가
completion 이전 실행을 막는다. `#147`에서 실측했고 cherry-pick으로 우회 발행해야 했다.

## 설계 재검토

이슈 본문의 후보 셋 중 둘이 실측으로 약해졌다. 근거를 본문에 기록했다.

| 후보 | 판정 |
|---|---|
| 1. orca base를 최신 main으로 재봉인 | 봉인 시점과 링크 시점 사이에 main이 또 진행하면 재발 |
| 2. `gh issue develop`에 base SHA 명시 | **CLI로는 불가** — `--base`는 브랜치 이름만 받는다(`--help` 확인) |
| 3. `sync-base` 조기 허용 | `execution_sync_base.go:176-178`의 게이트 ①과 주석이 "완결 이후 표면이다(AC-02)"를 근거로 명시. 다른 이슈의 계약을 깬다 |

### 후보 2를 GraphQL 경로로 정정

`CreateLinkedBranchInput.oid`는 **필수 필드**다(인트로스펙션 확인):

```
clientMutationId: SCALAR String
issueId: NON_NULL ID
oid: NON_NULL GitObjectID      ← 필수
name: SCALAR String
repositoryId: SCALAR ID
```

`gh issue develop`이 하는 일은 `--base` 브랜치 HEAD를 조회해 그것을 채우는 것뿐이다. 우리가
봉인 base SHA를 직접 채우면 갈림이 원리적으로 생기지 않는다.

**실측 검증** — 현재 main이 아닌 임의 SHA로 링크 브랜치를 만들었다:

```
$ gh api graphql -f query='mutation(...)' -F oid=2a56f2cc... -F name=176-oid-probe
176-oid-probe @ 2a56f2cc4d2e6b7b4fa99e3cdd71e3673ae060d2
```

지정한 SHA에 정확히 만들어지고 이슈에 연결됐다. 실측 브랜치는 정리했다.

## 변경

`Steps`에 `baseSHA` 인자를 추가하고 GitHub case를 `githubSteps`로 분리했다.

| baseSHA | 안내 |
|---|---|
| 있음 | 4단계: MCP 부재 → node id 조회 → `createLinkedBranch`(oid 못박음) → fail |
| 없음 | 3단계: MCP 부재 → `gh issue develop`(종전) → fail. **왜 oid를 고정 못 하는지 밝힌다** |

`branch prepare`가 이미 `--base-sha`를 받으므로 새 정보를 요구하지 않는다. direct 모드도 같은
경로를 쓰고, 그쪽은 봉인 base가 곧 `gh issue develop`이 쓰던 값이라 결과가 같다.

### node id 조회를 별도 단계로 두는 이유

한 명령에 셸 치환(`$(...)`)을 넣으면 가드가 정적으로 분류할 수 없고 이 저장소는 그런 명령을
거부한다. `fallback_api`는 지금도 사람이나 에이전트가 실행하는 안내이므로 값을 옮기는 것이 그
주체의 일이다 — `Description`에 명시했다.

### 가드 분류

`exactProviderBranchLink`에 `gh api` 두 형태를 추가했다.

```
gh api repos/<owner>/<repo>/issues/<number> --jq .node_id
gh api graphql -f query=<createLinkedBranch mutation> -F issueId=<id> -F oid=<sha> -F name=<branch>
```

query 본문이 그 mutation을 담고 있는지 확인하므로 임의 GraphQL이 통과하지 않는다. 플래그 위치와
개수도 고정한다.

### 인용 전제

GraphQL 변수(`$issueId` 등)는 셸에서 **단일 인용해야 한다.** 인용하지 않으면
`HasActiveParameterOrTildeExpansion`이 파라미터 확장으로 판정한다 — `tokens.go:344-350`이 단일
인용 안의 문자를 건너뛰므로 인용하면 통과한다. 그 전제를 `Description`에 명시하고 테스트로
고정했다.

## 수용 기준

- AC-01 발행 가능한 경로 — oid 못박기로 base 갈림이 생기지 않는다
- AC-02 base 갈림 진단 — baseSHA 부재 시 그 사실과 이유를 안내에 담는다
- AC-03 durable state와 발행 커밋 일치 — 봉인 base가 곧 원격 브랜치 base다
- AC-04 direct 모드 유지 — 같은 경로를 쓰고 결과가 같다
- AC-05 `#163` 순서 안내 반영 — orca 순서 문구를 두 분기에 모두 유지
- AC-06 RED 선행

## 검증

```
go test ./internal/core/issueops/branchprepare/ -count=1
go test ./internal/core/lifecycle/ -count=1
go test ./... -count=1
```

## 비범위

- GitLab 경로 — 일반 브랜치 API를 쓰고 `ref`에 SHA를 받는지 확인하지 않았다. 확인 없이 바꾸면
  `#136`의 추측 오류를 반복한다. 후속으로 남긴다
