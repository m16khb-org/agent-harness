# #181 구현 계획 — 문서 upkeep

이슈: https://github.com/m16khb-org/issueops/issues/181

## 왜 지금인가

이 세션에서 결함 8건을 머지했고 훅이 `ARCHITECTURE.md`·`CONVENTIONS.md`·`OPERATIONS.md`·`TESTING.md`
검토를 upkeep 대기 항목으로 보고했다. 그중 `OPERATIONS.md:150-155`는 단순히 낡은 게 아니라 **문서를
따르면 실패하는** 상태다 — `gh issue develop`을 안내하는데 `#176`이 그 경로가 orca 발행을 불가능하게
만든다는 것을 실측했다.

## 변경 단위

문서 변경이라 TDD의 RED가 없다. 대신 각 문장의 근거를 확인한 뒤 적는 것을 규율로 삼는다(AC-07).

### 1. `OPERATIONS.md` — orca 발행 절 교체

`:140-169`의 명령 블록에서 3행을 두 단계로 바꾼다: 이슈 node ID 조회 → `createLinkedBranch`에 봉인
base SHA를 `oid`로 넘김. 이어서 세 가지를 본문으로 적는다.

- **왜 `gh issue develop`이 아닌가**: `--base`가 브랜치 이름만 받고 GitHub이 그 시점 HEAD를 `oid`로
  쓴다. 봉인 base와 갈리면 push가 `non-fast-forward`이고, 그 뒤 봉인 가드·안전 훅·`sync-base` 요구가
  모든 해소 경로를 닫는다.
- **형태 제약 두 가지**: GraphQL 변수는 단일 인용해야 가드를 통과한다. node ID는 `$(...)` 없이 단계를
  나눠 옮겨 적는다.
- **GitLab**: `#180`으로 `ref`도 SHA를 받는 것을 확인했으므로 "순서 주의가 필요 없다"만으로 끝내지
  않는다.

원본은 `branchprepare`의 `Steps`다. 문서는 그것을 따르면 된다고 밝힌다.

### 2. `OPERATIONS.md` — `switch-mode`와 lease 함정

`sync-base` 절 뒤에 붙인다.

- `execution switch-mode`의 용도와 게이트 5종. `prepare`가 다른 `--mode`를 거부한다는 것(`#167`).
- 자기 lease에 `replace --revoke`를 쓰면 갇힌다. `release --generation N`이 맞다(`#170`).
- `--session-id`는 host가 주는 값이고 세션 재시작에도 불변이다. 재시작 후 `holder_identity_mismatch`는
  홀더가 죽은 것이 아니라 id가 잘못 기록된 것이다.

### 3. `CONVENTIONS.md` — Guard 컨벤션

- lifecycle execution guard allowlist 3층: 읽기 허용(`executionObservation`) / typed control
  plane(`executionTypedControlPlane`) / owner mutation(`exactIssueOpsOwnerMutation`). 층 선택 기준과
  각 층의 사례(`#170`의 `reset-legacy --preview`, `#177`의 `cleanup orphan`).
- matcher는 형태를 고정하되 본문 특징 문자열까지 확인한다(`#176`의 `exactProviderBranchLink`).
- 정적 분류를 깨는 명령 형태를 안내에 넣지 않는다. 단일 인용 예외의 근거는 `tokens.go`다.

### 4. `CONVENTIONS.md` — 외부 orchestration 어휘

- 외부 시스템의 어휘는 그 시스템의 정의에서 인용하고 출처를 코드에 남긴다. CLI 출력은 표본이지 어휘가
  아니다(`#171`·`#147`·`#180`).
- 분류 축을 섞지 않는다. dispatch가 `failed`여도 task는 `ready`다.

### 5. `TESTING.md`

- 어휘 열거는 축별로 고정하고 상류 정의를 인용한다. 로컬에서 드물게 나오는 값(`circuit_broken`,
  `timeout`)도 포함한다.
- shape 불변식 테스트가 우연한 단계 수를 고정하지 않는다(`#176`의 `TestStepsKeepTheirExistingShape`).

### 6. `ARCHITECTURE.md`·`CAUTIONS.md`

낡은 execution 명령 열거에 `switch-mode`를 넣는다. `CAUTIONS.md`에 자기-revoke와 `--session-id`
항목을 추가한다.

### 7. `ADR.md`

`2026-07-26 — Linked branches are pinned to the sealed base SHA`. 결정·근거·기각 대안(base 재봉인,
`sync-base` 조기 허용)·`#163`과의 관계.

## 검증

```
go test ./... -count=1
./bin/issueops docs --json
```

각 문장의 근거는 이미 확인했다: `execution --help`로 `switch-mode` 존재, `lifecycle_state.go:128`로
직접 브랜치 생성 차단, `docs.gitlab.com/ee/api/branches.html`로 GitLab `ref` 의미, `gh api graphql`
introspection으로 `oid` NON_NULL. 이 사이클 자체가 새 발행 경로의 실증이다 — `#181`의 링크 브랜치를
`createLinkedBranch`로 만들었다.

## 위험

문서가 다시 낡을 수 있다. `switch-mode`가 열거에서 빠져 있던 것과 같은 경로다. 완전한 해소는 execution
명령 열거를 한 곳으로 모으는 것이고 이번 범위가 아니다.
