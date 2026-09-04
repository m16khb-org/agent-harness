# #180 구현 계획 — GitLab 브랜치 생성 계약

이슈: https://github.com/m16khb-org/issueops/issues/180

## 결함 세 개

모두 `branchprepare`의 `Steps`에서 gitlab case 한 블록 안에 있다.

1. **`ref`가 base 브랜치 이름을 받는다.** GitLab이 그 시점 브랜치 HEAD에서 브랜치를 만들므로 orca가
   봉인한 base와 갈린다 — `#176`이 GitHub에서 고친 것과 같은 결함이다. `ref`는 커밋 SHA도 받는다
   (GitLab 공식 문서: "Branch name or commit SHA").
2. **MCP `ToolArguments`가 실제 스키마와 다르다.** `endpoint`·`method`·`field`를 최상위에 두는데
   `glab_api`의 input schema는 `args`(배열)·`flags`(객체)·`limit`·`offset` 네 키뿐이다. 그대로
   따르면 스키마 검증에서 실패한다.
3. **MCP 단계와 CLI 폴백이 다른 플래그를 지시한다.** MCP는 `field`, CLI는 `-f`(= `--raw-field`).
   `glab api`에서 둘은 별개 플래그다.

## 근거 — 상류 소스

`glab` MCP가 이 세션에 없다. 그것이 확인 불가의 근거가 되지 않는다는 지적을 받고 공개 소스를 읽었다.

**도구 이름과 opt-in** — `internal/commands/mcp/serve/server.go`:

```go
if !mcpannotations.HasAnnotation(cmd.Annotations) {
    continue
}
toolName := "glab_" + strings.Join(path, "_")
```

`glab api`는 `mcpannotations.Destructive: "true"`를 달고 있어(`internal/commands/api/api.go`)
`HasAnnotation`이 true다. 도구 이름 `glab_api`는 맞다.

**입력 스키마** — 같은 파일의 `buildToolFromCommand`:

```go
inputSchema := map[string]any{
    "type": "object",
    "properties": map[string]any{
        argsParam:   {...},  // "args", 문자열 배열
        flagsParam:  {...},  // "flags", 객체
        limitParam:  {...},  // "limit"
        offsetParam: {...},  // "offset"
    },
}
```

`flagsProperties`의 키는 `strings.ReplaceAll(flag.Name, "-", "_")`로 만들어진다 — 따라서
`raw-field`는 `raw_field`다.

**플래그 구분** — `internal/commands/api/api.go`:

```go
fl.StringVarP(&opts.requestMethod, "method", "X", "GET", ...)
fl.StringArrayVarP(&opts.magicFields, "field", "F", nil, "...inferred type...")
fl.StringArrayVarP(&opts.rawFields, "raw-field", "f", nil, "Add a string parameter.")
```

브랜치 이름과 ref는 문자열이므로 `raw_field`가 맞다.

## 변경 단위

### RED

`branch_prepare_gitlab_contract_test.go`를 새로 만든다. 세 결함을 각각 실증한다.

- `baseSHA`를 준 GitLab 안내가 `ref`에 그 SHA를 넘기는지 — 현재는 base 브랜치 이름이라 실패한다.
- MCP `ToolArguments`가 `args`·`flags` 중첩인지 — 현재는 최상위 `endpoint`라 실패한다.
- `flags`가 `raw_field`를 쓰고 CLI 폴백의 `-f`와 일치하는지 — 현재는 `field`라 실패한다.

### GREEN

`gitlabSteps(issueURL, branch, baseBranch, baseSHA)`를 추출해 `githubSteps`와 대칭으로 만든다.

- `baseSHA`가 있으면 `ref`에 그 값. 없으면 base 브랜치 이름으로 떨어지되 왜 고정하지 못하는지
  `Description`에 밝힌다 — GitHub 분기와 같은 계약이다.
- MCP `ToolArguments`를 `{"args": [endpoint], "flags": {"method": "POST", "raw_field": [...]}}`로.
- 확정 근거를 주석에 인용으로 남긴다(`CONVENTIONS.md`의 외부 어휘 출처 규율, `#181`).

단계 수(3)와 전략 어휘(`mcp`/`fallback_api`/`fail`)는 그대로다. GitLab은 사전 조회가 필요 없다.

## 검증

```
go test ./internal/core/issueops/branchprepare/... -count=1
go test ./... -count=1
```

**실환경 검증은 불가능하다.** 이 저장소는 GitHub에 있고 `glab` MCP도 이 세션에 없다 — `#164`가 같은
제약을 기록했다. 근거는 스키마를 생성하는 소스 코드이며, 문서 추측으로 어휘를 넓혀 틀린 `#136`과는
성질이 다르다.

## 비범위

- `glab` 설치나 MCP 서버 등록. 외부 도구는 각자 공식 경로로 설치한다.
- `cleanup status`의 `remote_branch_present`와 `cleanup finish`의 `remote_branch_absent`가 같은
  상태를 반대로 보고하는 것처럼 읽힌 관측(`#181` 정리 중). 별건이다.
