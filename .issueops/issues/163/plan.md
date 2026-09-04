# 163 — orca 모드로 IssueOps를 완주하면서 linked branch 추적도 유지한다

이슈: https://github.com/m16khb-org/issueops/issues/163
사이클: io-34454fa20b90
브랜치: `163-orca-linked-branch-contract` (base `main` @ 10e0598)

## #159의 결론이 틀렸다

#159는 "GitHub이 기존 브랜치를 이슈에 사후 연결하지 못한다"고 단정하고 닫혔다. 그
실측은 **원격에 브랜치를 push한 뒤** 시도해 실패한 것이었고, 그것을 "기존 브랜치는
연결 불가"로 일반화했다.

이번 사이클에서 다른 조건을 실측했다.

| 조건 | `gh issue develop --name <이름>` |
|---|---|
| 원격에 이미 있음 (#159) | `API returned empty branch name` 실패 |
| **로컬에만 있음** (#163) | **성공, Development 섹션에 연결됨** |

```
$ git branch 163-local-only-probe origin/main
$ gh issue develop 163 --base main --name 163-local-only-probe
github.com/m16khb-org/issueops/tree/163-local-only-probe

$ gh issue develop --list 163
163-orca-linked-branch-contract  .../tree/163-orca-linked-branch-contract
163-local-only-probe             .../tree/163-local-only-probe
```

실제 제약은 **원격 이름 충돌**이지 "기존 브랜치 연결 불가"가 아니었다. 실측 브랜치는
원격·로컬 모두 정리했다.

## 성립하는 순서

Orca `worktree create`는 로컬 워크트리와 로컬 브랜치만 만들고 **push하지 않는다.**
그래서 순서를 뒤집으면 둘 다 얻는다.

```bash
issueops branch prepare --id ID --base-sha "$BASE_HEAD" ...   # 기록만, 브랜치 없음
issueops execution prepare --id ID --mode orca ... --confirm  # Orca가 로컬 브랜치 생성
gh issue develop "$ISSUE_URL" --base "$BASE" --name "$BRANCH" # 원격 생성 + 이슈 연결
issueops branch prepare --id ID ... --link-verified           # 추적 확인 후 갱신
```

1단계가 성립하는 근거: `branch prepare`는 브랜치 존재를 검증하지 않고 **이름 규칙과
레코드 일치만** 본다(`branch_prepare.go:44-59`). `executionWorkspaceRequest`도
`BranchPrepare.BaseSHA`만 요구한다(`execution_prepare.go:360`).

**후보 A(불가능 인정)도 후보 B(추적 포기)도 필요 없다.**

## 변경

`Steps`의 GitHub `fallback_api` 안내가 Orca 모드의 순서 차이를 설명하게 한다. 그 안내가
없으면 운영자는 정식 순서(`gh issue develop` 먼저)를 따르고 Orca가 이름 충돌로
막힌다(#149·#152·#154).

`OPERATIONS.md`의 Orca 섹션에 같은 순서를 명령과 함께 적는다.

GitLab은 브랜치 이름 규칙이 연결 수단이라 순서 주의가 필요 없고, 애초에
`gitlab_issue_metadata_unsupported`로 Orca보다 먼저 걸러진다. GitHub 안내를 복사하면
거짓이 되므로 테스트로 막는다.

## 비범위

- `--mode auto`의 폴백 제거. #152가 넣었고 순서를 모르는 사용자를 보호한다.
- 사전 확인(`ensureOrcaBranchIsFree`) 제거. 순서를 틀렸을 때 mutation 이전에 막는
  안전망으로 유효하다.
- `branch prepare`를 두 단계로 쪼개는 것. 같은 명령을 두 번 부르면 되므로 불필요하다.

## 검증

```bash
go test ./internal/core/issueops/branchprepare/ -count=1
go test ./... -count=1
```

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).

사이클 중 세션이 재시작되어 홀더 PID가 바뀌었고(21327 → 51154) lease를 revoke →
finalize → claim으로 재수립했다. generation이 1 → 3으로 올라갔다.
