package core

func renderOpenAPISpec() string {
	return `# OpenAPI Spec Guidance

## Purpose

This project-specific API documentation prompt is for agents and MCP routing when endpoint, controller, handler, DTO, schema, or OpenAPI files change.

## Gate order

1. Static gate: ` + "`agent-harness api-doc static-check --json`" + `
2. Agent gate: ` + "`agent-harness api-doc review --json`" + `
3. Combined gate: ` + "`agent-harness api-doc check --json`" + `

Default scope is staged API candidate files. Scan all legacy debt only when ` + "`--all`" + ` is explicitly supplied.

## Static omissions to block

- missing route operation summary/description
- description does not follow the repo's sectioned Markdown format
- missing path/query/header/body parameter documentation
- missing 400 response when validation surface exists
- missing 401 response for private/auth endpoints
- OpenAPI decorator or optional-validation mismatch on required/optional DTO fields

## Agent review prompt

Static checks catch decorator/comment-level omissions. Agent review reads directly related business logic to detect public API contract drift.

The agent must inspect service/usecase/domain/error-mapping code called by changed endpoints. If these errors can occur, they must appear in OpenAPI responses.

- entity/resource not found → 404
- auth/session/token failure → 401
- permission/ownership/tier/role failure → 403
- validation/body/query/header problem → 400
- duplicate/state conflict/idempotency conflict → 409

Documentation must not contradict real behavior. For example, if docs say the endpoint only reads cache but it changes payment state, or docs omit 404 while a service can throw NotFound, that is a blocking issue.

## Clean Swagger style

- Operation summary should be short and client-oriented.
- Prefer sectioned Markdown plus bullets for descriptions, such as ` + "`### Purpose`" + `, ` + "`### Request Rules`" + `/` + "`### Processing`" + `, and ` + "`### Auth/Notes`" + `.
- Path/query/header/body parameters should include name, requiredness, format, and example.
- Responses should include client-handled failure statuses with schema/description, not success-only docs.
- Document single-object responses as top-level objects without unnecessary wrapper objects. Exceptions: pagination/list envelopes, explicit metadata contracts, backward compatibility, and standard error envelopes.
- If public/admin/internal docs are separated, filter paths/schemas for the intended audience.
`
}
