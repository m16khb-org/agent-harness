# 163 Turing 수용 리포트

사이클: `io-34454fa20b90`
이슈: https://github.com/m16khb/agent-harness/issues/163
브랜치: `163-orca-linked-branch-contract` (base `main` @ 10e0598)

## 결론이 두 번 바뀌었다

이 사이클은 "orca 모드와 linked-branch 요구의 관계를 계약으로 정한다"로 시작해 후보
A(현 상태 인정)를 골랐다가, 사용자 질문 두 개가 그 판단을 무너뜨렸다.

1. **"후보 B가 없어도 orca 모드 사용이 가능한가?"** — 확인해보니 아니었다.
   `issueops_pr_readiness.go:103-104`가 `link_verified`를 요구하므로 브랜치를 만들지
   않고 orca를 쓰면 PR 단계에서 막힌다. 내가 "브랜치 없는 사이클에서는 정상 작동한다"고
   쓴 것은 **검증하지 않은 주장**이었다.
2. **"orca도 쓰고 이슈 브랜치 추적도 할 수는 없나?"** — 이 질문이 #159의 실측 조건을
   다시 보게 했다.

## #159의 결론이 틀렸다

#159는 "GitHub이 기존 브랜치를 이슈에 사후 연결하지 못한다"고 단정하고 닫혔다. 그 실측은
**원격에 push한 뒤** 시도한 것이었고 그것을 일반화했다.

| 조건 | `gh issue develop --name <이름>` |
|---|---|
| 원격에 이미 있음 (#159) | `API returned empty branch name` 실패 |
| **로컬에만 있음** (이번 실측) | **성공, Development 섹션에 연결됨** |

실제 제약은 **원격 이름 충돌**이었다. Orca는 로컬 워크트리와 로컬 브랜치만 만들고
push하지 않으므로, 순서를 뒤집으면 둘 다 얻는다.

## 수용 기준 판정

| AC | 판정 | 근거 |
|---|---|---|
| AC-01 관계를 계약으로 정하고 문서화 | **달성** | `OPERATIONS.md`에 순서와 명령을 적었다. 계약은 "불가능"이 아니라 "순서를 뒤집으면 가능"이다 |
| AC-02 auto 폴백을 계약의 일부로 설명 | **달성** | 순서를 틀렸을 때의 안전망으로 문서에 남겼다 |
| AC-03 provider별 차이 명시 | **달성** | GitLab은 이름 규칙이 연결 수단이라 순서 주의가 없고 `gitlab_issue_metadata_unsupported`로 먼저 걸러진다. 테스트로 고정 |
| AC-04 `link_verified` 모드별 의미 | **달성** | 갈리지 않는다. 순서만 바뀌고 의미는 같으며 4단계에서 정직하게 갱신된다 |
| AC-05 RED 선행 | **달성** | GitHub 안내 테스트만 실패, GitLab·구조 테스트는 그때도 통과 |

## 변경

`Steps`의 GitHub `fallback_api` 안내가 Orca 모드의 순서 차이를 설명한다. 코드가 순서를
**강제하지 않고 안내만** 한다 — `branch prepare`가 브랜치 존재를 검증하지 않으므로 순서
자체는 이미 가능했고, 운영자가 그것을 몰랐던 것이 문제였다.

`OPERATIONS.md`에 실행 가능한 명령 순서를 적었다.

## 이 사이클이 #158을 실증했다

lease active 중 `decision add`가 **성공했다.** 이전 사이클(#152)에서는 같은 명령이
`unsafe_mutation`으로 막혀 결정을 문서에만 남겨야 했다. #158이 세 곳(`ParseExact
IssueOpsCommand` 두 단어 목록, `IssueOpsCommandSpec`, allowlist)을 고쳤고 설치본을
재빌드한 뒤 실제로 동작한다.

## 세션 재시작 중 lease 재수립

사이클 중 세션이 재시작되어 홀더 PID가 21327 → 51154로 바뀌었다. `holder_identity_
mismatch`(축: `session_process_ancestry`)로 막혔고 revoke → finalize → claim으로
재수립했다. generation이 1 → 3으로 올라갔다.

`release`가 "only the current holder may release"로 거부된 것이 정확했다 — 낡은 정체로는
해제할 수 없고 `replace`가 그 상황을 위한 경로다.

## 검증

```
go test ./internal/core/issueops/branchprepare/ -count=1
go test ./... -count=1
```

RED는 GitHub 안내 테스트 하나만 실패했다. 구현 후 3건 GREEN, 전체 회귀 통과.

## 비범위

- `--mode auto` 폴백 제거 — 순서를 모르는 사용자를 보호한다
- 사전 확인 제거 — 순서를 틀렸을 때 mutation 이전에 막는 안전망
- 순서 강제 — `execution prepare`가 `branch prepare`를 되부르는 것은 계약 근간 변경이고
  실익이 적다

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
메인 에이전트가 직접 반론을 수행했고 저자와 검토자가 분리되지 않았다.

이 사이클의 초기 게이트(intent·design)는 "계약 문서화"로 기록됐고 실측 후 방향이 바뀌었다.
`decision add`로 그 전환을 durable state에 남겼다.
