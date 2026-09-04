# CLI, MCP, and output contracts

> Family index: [`../CONVENTIONS.md`](../CONVENTIONS.md). This module owns the
> CLI and MCP tool schemas, the `--json`/exit-code contract, MCP response and
> single-object output shape, the api-doc gate, prompt structure, the
> project-docs bootstrap surface, and commit-message format. Go/package/port/
> adapter rules live in [`go-and-packages.md`](go-and-packages.md); runtime
> state, policy, guard, hook, and lifecycle rules live in
> [`state-policy-and-hooks.md`](state-policy-and-hooks.md).

## 3. CLI 컨벤션

- subcommand는 동사 중심으로 짧게 둔다: `inspect`, `state`, `run`, `mcp`, `worker`.
- 사람이 읽는 기본 출력과 agent가 파싱하는 `--json` 출력을 구분한다.
- `--json` 출력 field는 snake_case를 사용한다.
- 실패 시 stderr에는 짧은 설명, JSON 모드에는 machine-readable error code를 포함한다.
- exit code는 의미 있게 유지한다(`cmd/issueops/issueopsapp/root_command_facade.go`, `cmd/issueops/rootcmd/root_command.go`).
  - `0`: 성공
  - `1`: 일반 실패. flag/usage 오류도 현재는 `gates`를 제외하면 `1`이다.
  - `2`: 알 수 없는 subcommand, `gates` usage 오류
  - `3`: policy 거부(`policy`)와 guard 차단(`guard`)
  - workspace/config 전용 코드는 없다. 새 코드를 도입하면 이 목록과 `usage.golden.txt`를 함께 갱신한다.

---

## 4. MCP 컨벤션

- MCP tool 이름은 snake_case `<capability>_<verb>` 형태다(예: `harness_inspect`, `state_write`, `issueops_execution`). 목록은 `cmd/issueops/testdata/mcp_tools.golden.json`이 고정한다.
- CLI와 MCP는 같은 core request/response DTO를 공유한다.
- tool response에는 불필요한 대용량 파일 내용을 싣지 않는다. 요약, 경로, hash, line range를 우선한다.
- command 실행 tool은 policy 결과와 audit log id를 함께 반환한다.
- schema 변경은 golden test와 문서 업데이트를 동반한다.
- CLI/MCP contract golden은 `cmd/issueops/testdata/`에 둔다. 의도된 schema 변경일 때만 `go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -update -count=1`로 갱신한다.
- 실제 JSON response golden은 dynamic field(timestamp, temp path, audit id)를 normalize해서 host/session 차이로 인한 drift를 막는다.
- response golden 범위는 state/policy뿐 아니라 docs/inspect/preflight처럼 agent가 자주 의존하는 읽기 표면을 우선 포함한다.
- CLI와 MCP가 같은 `next_command`를 반환할 때 provenance 같은 composition evidence도 동일하게 결합한다. Port에는 contract 타입 대신 순수 observation receipt만 두고 application에서 contract evidence로 변환한다. 파일·프로세스 관측은 contract/core/cmd package에 넣지 않고 `issueopsapp`이 outbound adapter를 생성해 주입하며, 관측 실패를 빈 값이나 `unavailable`로 치환하지 않는다.

---

## API Documentation Gate

Reusable API documentation checks belong in `issueops api-doc check`, not in a single application repository or framework-specific hook. Keep the core prompt framework-agnostic: it may mention examples such as NestJS Swagger decorators, Go swaggo annotations, OpenAPI specs, Spring/FastAPI annotations, but must instruct reviewers to follow the target project's own convention and staged diff only. Project-specific strictness should be supplied via `--prompt-file` rather than hardcoded into harness core.

## Prompt Structure Convention

Reusable harness prompts follow the strong prompt shape from the user-provided example. Do not copy example-specific domain content into harness prompts; reuse the structure:

- Identity: name the role and expertise the model should inhabit.
- Objective: state the concrete outcome.
- Operating phases: break the work into explicit ordered stages.
- Inputs: name the data the model may use and how missing data is represented.
- Rules: state hard constraints, safety boundaries, and decision rules.
- Output contract: define the exact response format and forbidden wrapper text.
- Verification checklist: require a final self-check against the prompt contract.

New reusable prompts should use `internal/domain/prompt.BuildStructuredPrompt` where possible. JSON packet prompts may use equivalent structured keys instead of Markdown headings, but they still need the same identity/objective/phases/inputs/rules/output/checklist shape. Existing prompt-specific strictness, such as JSON-only output or no Markdown fences, must remain stronger than the generic structure.

## Project Docs Bootstrap 컨벤션

- 적용 대상 repo의 `.issueops/` 문서는 명시적 `issueops project bootstrap` 또는 `$project-bootstrap` 실행 때만 생성한다.
- 기본 설치나 MCP read는 대상 repo에 파일을 쓰지 않는다.
- `AGENTS.md`는 전체 덮어쓰기 금지다. bootstrap은 behavioral top block이 없으면 prepend하고, 이후에는 `ISSUEOPS` marker block만 관리한다.
- 생성 문서에는 추론된 명령과 기술스택의 Evidence/Confidence를 포함한다.
- MCP 라우팅은 문서를 무조건 주입하지 않고 작업별로 읽어야 할 문서 경로와 이유만 제공한다.

## API Documentation Convention

OpenAPI 요구사항의 정규 소유자는 [`OPEN_API_SPEC.md`](../OPEN_API_SPEC.md)다. API documentation gates are framework-agnostic and business-logic-aware. A changed endpoint's docs must reflect not only syntax-level params and DTO schemas but also directly reachable domain errors from service/usecase/error-mapping code. Keep Swagger/OpenAPI output clean: concise summaries, consistent sectioned descriptions, complete params, accurate required/optional fields, and explicit success/error responses.


NextCandle-style cleanliness means sectioned Markdown operation descriptions, complete params, accurate auth/error responses, DTO required/optional correctness, and Swagger outputs split/filtered for the intended audience where the framework supports it.

## Single-object response convention

단일 객체 응답은 불필요한 wrapper object로 한 번 더 감싸지 않는다. 공개 계약이 이미 하나의 resource/detail/view model이라면 controller/handler는 `{ data: object }`, `{ result: object }`, `{ item: object }`처럼 포장하지 말고 object fields를 top-level로 spread해 반환한다.

예외는 pagination/list envelope, metadata가 계약의 일부인 경우, backward compatibility 유지, 표준 error envelope처럼 wrapper 자체가 명시적 API 계약인 경우다. 새 endpoint나 DTO 리팩터링 시 OpenAPI schema도 이 규칙과 일치해야 한다.

## 10. 커밋 메시지 컨벤션

- 커밋 메시지는 [`COMMIT_POLICY.md`](../COMMIT_POLICY.md)를 따른다.
- subject는 Conventional Commit 형식(`<type>(<scope>)!?: <summary>`)을 사용한다.
- body에는 AI agent가 맥락을 회수할 수 있도록 `Lore:` 블록을 둔다.
- atomic commit 하나에는 Lore `Intent`도 하나만 있어야 한다.
