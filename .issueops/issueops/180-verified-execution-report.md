# #180 검증 보고서 — GitLab 브랜치 생성 계약

이슈: https://github.com/m16khb-org/issueops/issues/180

## 왜 결함이 셋이었는가

이 이슈를 처음 낼 때 근거는 GitLab 공식 문서의 `ref` 설명 하나였다. `glab`이 로컬에 없다는 것을
"더 확인할 수 없다"의 근거로 삼았기 때문이다.

그 판단이 틀렸다는 지적을 받고 `glab` 상류 소스를 읽었다. `ref` 문제와 **독립된 두 번째 결함**이
나왔다 — MCP 도구 인자가 실제 스키마와 달라, 그 단계를 그대로 따르면 스키마 검증에서 실패했다.
도구가 없다고 멈췄으면 못 봤다.

## 확정한 사실 (gitlab-org/cli, main)

**도구 이름과 opt-in** — `internal/commands/mcp/serve/server.go`:

```go
if !mcpannotations.HasAnnotation(cmd.Annotations) { continue }
toolName := "glab_" + strings.Join(path, "_")
```

`glab api`는 `mcp:destructive`를 달고 있어 통과한다(`internal/commands/api/api.go`). 우리가 쓰던
도구 이름 `glab_api`는 맞았다.

**입력 스키마** — 같은 파일의 `buildToolFromCommand`가 모든 도구에 같은 네 키만 만든다:
`args`(문자열 배열), `flags`(객체), `limit`, `offset`. `flags`의 키는
`strings.ReplaceAll(flag.Name, "-", "_")`로 만들어진다.

**플래그 구분** — `internal/commands/api/api.go`:

```go
fl.StringArrayVarP(&opts.magicFields, "field", "F", nil, "Add a parameter of inferred type. ...")
fl.StringArrayVarP(&opts.rawFields, "raw-field", "f", nil, "Add a string parameter.")
```

브랜치 이름과 ref는 문자열이므로 `raw_field`가 맞다.

## 무엇을 바꿨는가

`gitlabSteps(branch, baseBranch, baseSHA)`를 추출해 `githubSteps`와 대칭으로 만들었다.

| 결함 | 전 | 후 |
|---|---|---|
| base 못박기 | `ref=` base 브랜치 이름 | `baseSHA`가 있으면 `ref=` 그 SHA. 없으면 종전 경로 + 왜 못박지 못하는지 안내 |
| MCP 인자 형태 | 최상위 `endpoint`·`method`·`field` | `args` 배열 + `flags` 객체 중첩 |
| 플래그 일치 | MCP는 `field`, CLI는 `-f` | 양쪽 다 `raw_field`/`--raw-field` |

단계 수(3)와 전략 어휘(`mcp`/`fallback_api`/`fail`)는 그대로다. GitLab은 사전 조회가 필요 없다.

확정 근거를 `gitlabSteps` 주석에 인용으로 남겼다 — `CONVENTIONS.md`의 외부 어휘 출처 규율(`#181`)을
따른다.

## 검증

**RED가 세 결함을 각각 실증했다.**

```
--- FAIL: TestGitLabPinsRefToTheSealedBaseSHA        flags는 객체다: <nil>
--- FAIL: TestGitLabMCPArgumentsMatchTheGlabAPISchema
    최상위 "endpoint"는 glab_api 스키마에 없다:
    map[endpoint:... field:[branch=16-demo ref=main] method:POST]
--- FAIL: TestGitLabMCPAndFallbackAgreeOnRawField    flags는 객체다: <nil>
--- FAIL: TestGitLabWithoutBaseSHAExplainsWhyItCannotPin
```

GREEN 후 `go test ./... -count=1` 전 패키지 PASS. `response_contracts.golden.json`을 의도된 계약
변경으로 갱신했고, diff 288줄이 전부 16개 fixture에 반복되는 같은 GitLab 항목임을 확인했다 — 다른
표면의 drift는 없다.

**실환경 검증은 불가능하다.** 이 저장소는 GitHub에 있고 `glab` MCP도 이 세션에 등록돼 있지 않다
(`#164`가 같은 제약을 기록했다). 근거는 스키마를 **생성하는** 소스 코드이며, 문서 추측으로 어휘를
넓혀 틀린 `#136`과는 성질이 다르다.

이 사이클의 링크 브랜치는 `#176`이 만든 경로로 봉인 base `a71a800`에 붙였다.

## 정리 단계에서 함께 고친 것

- `branch_prepare_base_oid_test.go`의 주석이 "GitLab은 이 문제의 대상이 아니다 — 확인 없이 적용하면
  `#136`을 반복한다"고 했다. 이제 확인했으므로 "같은 계약을 다른 수단으로 맞췄다"로 바꿨다. assertion
  자체(GitLab이 GraphQL·node id를 쓰지 않는다)는 여전히 참이라 유지했다.
- 첫 GREEN에서 `Description` 문장이 "from branch main, whose ... (#180) through the GitLab MCP
  ..."로 어색하게 끼어들었다. golden mismatch 출력을 읽고 발견해 문장을 분리했다.
- `OPERATIONS.md`가 "`#180`이 그것을 다룬다"고 예고했다. 해결됐으므로 결과와 MCP 인자 형태로 갱신했다.
- `CONVENTIONS.md`의 어휘 출처 규율에 이번 실패 방식을 추가했다 — 도구 미설치는 확인 불가가 아니다.

## 남는 것

- **glab 버전 의존.** `buildToolFromCommand`가 바뀌면 우리 안내가 다시 어긋난다. 주석의 출처가 그
  재확인 지점이다.
- **`cleanup status`와 `cleanup finish`의 게이트 이름.** `#181` 정리 중 `remote_branch_present`와
  `remote_branch_absent`가 같은 상태를 반대로 보고하는 것처럼 읽혔다(실제 원격 브랜치는 존재했다).
  확인하지 않았고 이 이슈 범위 밖이다.
- **CLI usage 불일치 두 건.** `link-plan`이 지원하는 actor 플래그를 usage에 표시하지 않고,
  `remote verify-artifact`의 `ACTOR_FLAGS` 표기가 실제 spec(4종)보다 넓게 읽힌다. `#181`에서
  기록했고 여전히 미해결이다.
