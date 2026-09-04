# #181 검증 보고서 — 문서 upkeep

이슈: https://github.com/m16khb-org/issueops/issues/181

## 무엇을 바꿨는가

이 세션에서 머지한 결함 8건(#147·#167·#169·#170·#171·#176·#177)과 새로 확인한 GitLab base SHA
수용(#180)의 결과를 공용 문서에 반영했다.

| 문서 | 변경 |
|---|---|
| `OPERATIONS.md` | orca 발행 절의 명령 블록을 `createLinkedBranch` 경로로 교체. `gh issue develop`을 쓰지 않는 이유와 형태 제약 두 가지. `switch-mode` 절과 자기-revoke·`--session-id` 함정 |
| `CONVENTIONS.md` | 가드 allowlist 3층 분류, matcher 규율, 정적 분류 제약, 외부 어휘 출처 규율, 분류 축 분리 |
| `TESTING.md` | 어휘 열거 축별 고정 기준, shape 불변식 기준. `gh issue develop` 참조를 과거 시제로 |
| `ARCHITECTURE.md`·`CAUTIONS.md` | execution 명령 열거에 `switch-mode` 추가. 자기-revoke·`--session-id` 주의 |
| `ADR.md` | `2026-07-26 — Linked branches are pinned to the sealed base SHA` |
| `skills/issueops/references/worktree-context.md` | GitHub 브랜치 생성 지시를 `createLinkedBranch`로 |

## 가장 실질적인 것

`OPERATIONS.md`가 안내하던 `gh issue develop`은 단순히 낡은 게 아니라 **문서를 따르면 실패하는**
경로였다. `#176`이 실측한 대로, 그 CLI는 base 브랜치의 그 시점 HEAD를 `oid`로 쓰므로 orca가 봉인한
base와 갈리고, 그 뒤 봉인 가드·안전 훅·`sync-base` completion 요구가 모든 해소 경로를 닫는다.

## 검증

`go test ./... -count=1` 전 패키지 PASS(두 번 — diff 점검 후 재실행).

**이 사이클 자체가 새 경로의 실증이다.** `#181`의 링크 브랜치를 `branch prepare`가 렌더한 두 단계로
만들었다.

```
gh api repos/m16khb/issueops/issues/181 --jq .node_id
→ I_kwDOSwu3Xc8AAAABKOATgg

gh api graphql -f 'query=mutation(...){createLinkedBranch(...)}' \
  -F issueId=I_kwDOSwu3Xc8AAAABKOATgg -F oid=5df26b20f519983a9f77e2881f82722a1ed9a81a \
  -F name=181-docs-upkeep-orca-publication
→ 181-docs-upkeep-orca-publication @ 5df26b20f519983a9f77e2881f82722a1ed9a81a
```

봉인 base SHA에 정확히 생성되고 이슈에 연결됐다. `branch prepare --link-verified`로 추적을
확인했다.

## 정리 단계에서 고친 것

diff를 문장 단위로 읽어 **자기모순**을 발견했다. 새 명령 블록이 `-F issueId="$NODE_ID"`처럼 셸 변수
확장을 쓰는데, 바로 아래 본문은 그 형태가 가드에 거부된다고 적고 있었다. 꺾쇠 리터럴 자리표시자로
바꾸고 "셸 변수가 아니다"를 명시했다.

같은 절의 낡은 문장 둘도 함께 고쳤다 — "IssueOps 정식 순서는 `gh issue develop`으로" 도입부와
"`createLinkedBranch`(= `gh issue develop`)" 등호 표기다. 새 블록만 바꾸면 절 안에서 어긋난다.

`rg`로 저장소 전체의 `gh issue develop` 참조를 전수 조사했다. 코드 주석은 이력 설명이라 유지하고,
운영 지시인 스킬 참조 문서 한 곳을 갱신했다.

## 남는 것

- **execution 명령 열거가 세 문서에 중복된다.** `switch-mode`가 빠져 있던 것이 그 결과다. 이번에는
  낡은 열거를 고쳤고 구조 정리는 별건이다.
- **`link-plan`의 CLI usage 문자열이 지원하는 actor 플래그를 표시하지 않는다.** 이 사이클에서
  부딪혀 확인했다(`--host`·`--session-id`·`--agent-id`·`--cwd`를 받지만 usage에는 없다). CLI 코드
  변경이라 이 이슈 범위 밖이다.
- **`#180`은 등록만 했다.** GitLab `ref=baseSHA`는 후속 이슈다.
