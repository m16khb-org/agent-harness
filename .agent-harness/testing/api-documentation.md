# OpenAPI static and agent review gates

[← TESTING.md](../TESTING.md) owns the test-strategy index. This module owns
the API documentation checks: Swagger/OpenAPI static gates, business-logic error
response coverage, the agent-backed review boundary, the OpenAPI prompt source,
and prompt contract tests.

## API documentation checks

Endpoint/DTO 변경 시 Swagger/OpenAPI 문서 검사를 필수로 실행한다.

```bash
agent-harness api-doc check --json
# target repo에 package script가 있으면 그 repo에서는 다음 wrapper를 권장한다.
npm run swagger:check
npm run swagger:check -- --all
```

기본 검사 범위는 staged controller/DTO/handler/OpenAPI 후보 파일이어야 하며, 기존 레거시 전체 부채를 한 번에 실패시키지 않는다.

NestJS Swagger 프로젝트에서 blocking으로 잡아야 하는 누락:

- REST route method의 `@ApiOperation` 누락
- `@ApiOperation.description` 누락 또는 repo의 문서 섹션 형식 위반
- `:id` 등 path param이 있는데 `@ApiParam` 누락
- `@Headers` 사용인데 `@ApiHeader` 누락
- `@Body`, `@Query`, `@Headers` 사용으로 validation 400 가능성이 있는데 400 Swagger response 누락
- private/auth endpoint인데 401 Swagger response 누락
- DTO required property의 `@ApiProperty` 누락
- DTO optional property의 `@ApiPropertyOptional` 누락
- DTO optional property의 `@IsOptional` 누락

### Business logic error response coverage

Swagger/OpenAPI 검사는 decorator/comment 존재 여부만 보지 않는다. 변경된 endpoint가 호출하는 service/usecase/domain error mapping을 확인해 비즈니스 로직상 가능한 public error response가 스펙에 있는지 확인한다.

예:

- entity lookup 실패 → 404 response 필요
- ownership/permission 실패 → 403 response 필요
- duplicate/state conflict → 409 response 필요
- validation/body/query/header 문제 → 400 response 필요
- private/auth endpoint → 401 response 필요

깔끔한 Swagger 문서는 success-only가 아니라, client가 실제로 처리해야 하는 성공/실패 계약을 모두 보여줘야 한다.

### Agent-backed verification boundary

비즈니스 로직의 실제 404/403/409 가능성과 OpenAPI 누락 여부는 정적 테스트만으로 신뢰 있게 판정하지 않는다. 정적 테스트는 후보 파일 선택, `--all` wiring, prompt contract, MCP schema 같은 배선을 검증하고, 실제 API 문서 품질 판정은 `agent-harness api-doc review`/MCP `api_doc_review`가 렌더한 prompt/schema를 host agent가 수행한 뒤 결과 JSON을 `--result`/`result_file`로 기록해 수행한다.

## Prompt contract tests

Reusable LLM prompts should follow the shared prompt structure through `internal/core.BuildStructuredPrompt` or equivalent JSON packet keys. When adding or changing prompt builders, add or update tests that check for identity, objective, operating phases, inputs, rules, output contract, and verification checklist. Strict-output prompts must also keep tests for JSON-only/no-fence/no-preamble behavior.

`nextcandle-api`에서 확인한 좋은 기준:

- `@ApiOperation.description`은 `### 목적`, `### 요청 규칙`/`### 처리 방식`, `### 권한/주의사항`처럼 Markdown section + bullet로 구성한다.
- path/query/header/body와 auth/tier/public 여부가 response 문서와 일치한다.
- service/usecase의 `NotFoundException`, `ForbiddenException`, `ConflictException` 등 public error가 endpoint response에 반영된다.
- public/admin Swagger document를 분리하고, 사용하지 않는 schema를 필터링해 client가 읽는 문서를 깔끔하게 유지한다.

## OpenAPI prompt source

Endpoint/controller/DTO/schema/OpenAPI 변경 시 `.agent-harness/OPEN_API_SPEC.md`를 프로젝트별 프롬프트 source로 사용한다. `agent-harness api-doc review`는 별도 `--prompt-file`이 없으면 이 문서를 자동으로 포함한다.
