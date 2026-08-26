---
name: OPEN_API_SPEC.md
description: Endpoint, DTO, and OpenAPI documentation gate rules.
---

# OpenAPI Spec Guidance

## Purpose

Endpoint/controller/handler/DTO/schema/OpenAPI 변경 시 에이전트와 MCP가 가져가야 하는 프로젝트별 API 문서화 프롬프트다. 하네스 기본 프롬프트는 언어/프레임워크 공통 기준을 제공하고, 이 문서는 repo별 Swagger/OpenAPI 스타일과 blocking 기준을 고정한다.

## Gate order

```bash
harness api-doc static-check --json
harness api-doc review --json          # renders prompt/schema for the host agent
harness api-doc review --result FILE --json
# 또는 통합 실행
harness api-doc check --result FILE --json
```

기본 범위는 staged API candidate files다. 기존 레거시 전체 부채는 `--all`을 명시한 경우에만 본다.

## Repository mode

`OPEN_API_SPEC.md` frontmatter의 `api_doc_mode`가 정적 게이트 동작을 고정한다.

- 미설정 또는 `swagger`: 기존 Swagger/OpenAPI decorator 정적 검사를 수행한다.
- `contract-tests`: Swagger decorator 정적 검사를 건너뛰되, staged API candidate에
  대한 host-agent contract review는 계속 수행한다.

`contract-tests`는 OpenAPI 미채택을 명시적으로 결정하고 route/contract test가
API 계약을 소유하는 저장소에서만 사용한다. 문서 본문 prose를 추론해 자동으로
선택하지 않는다.

## Static omissions to block

정적 게이트가 확정적으로 잡아야 하는 누락:

- route operation summary/description 누락
- description이 repo의 sectioned Markdown 형식을 따르지 않음
- path/query/header/body parameter 문서화 누락
- validation surface가 있는데 400 response 누락
- private/auth endpoint인데 401 response 누락
- DTO required/optional field의 OpenAPI decorator 및 optional validation mismatch
- DTO required/optional drift: TypeScript/검증 상태와 Swagger 문서가 반대로 기술된 경우(`required: false` 객체형 포함) `required_optional_mismatch`

정적 게이트의 route 블록 조립은 괄호/중괄호 밸런스로 decorator 객체 내부(`summary:`/`description:` 줄)까지 포함한다. 다중 줄 `@ApiOperation`을 쓰는 문서화 잘 된 컨트롤러가 오히려 검사를 우회하지 않는지는 `cmd/harness/apidoc/dogfood` 픽스처가 회귀 테스트로 고정한다.

`@ApiResponse({ status: 404 })`, `@ApiResponse({ status: HttpStatus.NOT_FOUND })`, `@ApiResponses([{ status: 400 }, ...])` 객체/enum/배열 형태를 모두 상태 문서로 인식한다.

## Agent review prompt

정적 검사는 decorator/comment 수준 누락을 잡고, 에이전트 리뷰는 직접 관련된 business logic을 읽어 public API contract drift를 확인한다.

`api-doc review`가 프롬프트를 렌더할 때 Business Logic Error Contract Evidence 섹션이 함께 번들된다. 하네스가 변경된 controller/handler 파일에서 정적으로 추출한:

- 호출한 service method와 그 `throw new <Exception>` 지점(파일:라인, 동일 클래스 1단계 전이 호출 포함)
- `ClientProxy.send/emit` 패턴 → 원격 `@MessagePattern`/`@EventPattern` 핸들러 → 해당 service의 throw 지점(마이크로서비스 hop)
- `@Catch` exception filter의 HTTP status 매핑

호스트 에이전트는 이 증거를 문서화된 response 목록과 대조해야 한다. 증거에 도달 가능한 public error(404/403/409/400/401, RPC 매핑 status)가 문서에 없으면 blocking이다. 증거는 입력일 뿐 판정이 아니며, 증거가 잘못 추출되었을 리뷰에서 확인해야 한다.

에이전트는 변경된 endpoint가 호출하는 service/usecase/domain/error-mapping 코드를 확인해야 한다. 다음 오류가 실제로 발생할 수 있으면 OpenAPI responses에 반영되어야 한다.

- entity/resource not found → 404
- auth/session/token failure → 401
- permission/ownership/tier/role failure → 403
- validation/body/query/header 문제 → 400
- duplicate/state conflict/idempotency conflict → 409

문서 설명은 실제 처리와 달라서는 안 된다. 예를 들어 문서에는 캐시만 조회한다고 되어 있는데 실제로 결제 상태를 변경하거나, 문서에는 404가 없지만 service가 NotFound를 던지면 blocking issue다.

## Clean Swagger style

- operation summary는 짧고 client 관점의 동작을 말한다.
- description은 `### 목적`, `### 요청 규칙`/`### 처리 방식`, `### 권한/주의사항` 같은 sectioned Markdown + bullet 형식을 선호한다.
- path/query/header/body parameter는 이름, 필수 여부, 형식, 예시를 포함한다.
- response는 success-only가 아니라 client가 처리해야 하는 실패 status와 schema/description을 포함한다.
- public/admin/internal 문서가 분리된 repo라면 의도한 audience에 맞게 paths/schema를 필터링한다.

## Single-object response shape

단일 객체 응답은 객체로 다시 감싸지 않고 top-level object로 문서화한다.

- 권장: `UserDetailDto` fields가 response schema의 top-level properties로 노출된다.
- 비권장: `{ data: UserDetailDto }`, `{ result: UserDetailDto }`, `{ item: UserDetailDto }`처럼 단일 객체를 wrapper로 감싼다.

예외는 pagination/list envelope, response metadata가 명시 계약인 경우, backward compatibility, 표준 error envelope이다. 에이전트 리뷰는 새 endpoint/DTO 변경에서 단일 객체 wrapper가 생기면 기존 프로젝트 컨벤션과 예외 근거를 확인하고, 근거가 없으면 blocking 또는 warning으로 보고한다.
