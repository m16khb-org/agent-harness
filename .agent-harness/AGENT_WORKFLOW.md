---
name: AGENT_WORKFLOW.md
description: Agent start, execution, verification, and completion flow.
---

# Agent Workflow

## Start

1. `AGENTS.md`를 먼저 읽는다.
2. 세션 시작 시 `.agent-harness/CONSTITUTION.md`를 기본 원칙으로 확인한다.
3. 작업 종류에 맞는 `.agent-harness/` 문서를 확인한다.
4. 현재 파일과 명령 출력으로 문서의 추정 항목을 검증한다.

## Work

`AGENTS.md`의 Simplicity First/Surgical Changes 원칙을 기본으로 하고, 이 프로젝트에서는 다음 기록·안전 규칙을 추가한다.

- 기존 사용자 변경을 덮어쓰지 않는다.
- 새 dependency, 배포, destructive action은 명시 지시나 강한 근거가 있을 때만 진행한다.
- 문서가 현재 코드/사용자 컨센서스와 어긋나면 MCP `project_docs_read`로 현재 SHA를 확인하고 `project_docs_update`로 한 문서씩 갱신한다.
- 구조 선택이나 대안 기각 사유가 생기면 MCP `project_docs_record(kind=adr)`로 `.agent-harness/ADR.md`에 남긴다.
- 반복 실패, false case, 위험한 운영 주의는 MCP `project_docs_record(kind=caution)`으로 `.agent-harness/CAUTIONS.md`에 남긴다.

## Verify

`AGENTS.md`의 Goal-Driven Execution 원칙을 기본으로 하고, 이 프로젝트에서는 다음 검증 라우팅을 추가한다.

- 테스트를 작성/수정할 때는 `.agent-harness/TESTING.md`의 good/bad test 기준을 먼저 확인한다.
- CLI/MCP/API 문서 계약을 바꾸면 golden/schema/smoke 검증을 함께 실행한다.
- 실행한 테스트/빌드/정적검사 결과와 실행하지 못한 검증의 이유는 완료 보고에 포함한다.

## Finish

- 커밋이 필요하면 `.agent-harness/COMMIT_POLICY.md`를 따른다.
- 해결한 false case나 구조 결정은 필요한 경우 MCP `project_docs_record`로 기록한다.

## UserPromptSubmit hook

Codex/Claude host가 지원하면 `agent-harness hook user-prompt`가 매 사용자 지시마다 project-doc catalog와 host-compatible routing context를 주입한다. 이 힌트는 자동 실행 명령이 아니라 agent가 필요한 문서/MCP를 판단하기 위한 reminder다. Codex는 `--host codex`로 설치되어 visible `hook context:` row에 project-doc catalog만 주입하고, Claude Code는 `systemMessage`와 compact `additionalContext`를 분리해 richer routing/status hints를 유지할 수 있다. Hook은 차단/대행 실행을 하지 않고, project docs/API docs/CAUTIONS/ADR 갱신 후보만 제안해야 한다.

## MCP Usage Rule

- MCP는 모델 기억 대신 현재 repo 상태, 문서 라우팅, 정책 판정, state checkpoint, durable record가 필요할 때 사용한다.
- 단순 추론이나 이미 열린 파일의 요약에는 MCP를 쓰지 않는다.
- 작업 시작 시 가능한 경우 `project_docs_route`에 현재 task를 넣어 필요한 문서만 고른다.
- 문제가 발생했고 해결했다면 `project_docs_record(kind=caution)`으로 `.agent-harness/CAUTIONS.md`에 기록한다.
- 구조 결정이나 대안 기각 사유가 생겼다면 `project_docs_record(kind=adr)`로 `.agent-harness/ADR.md`에 기록한다.
- 많은 도구를 한 번에 쓰기보다 route/read/update/check/record처럼 의도가 분명한 도구를 좁게 사용한다.
- tool 결과는 경로, `exists`, warning, 검증 증거를 확인한 뒤 작업에 반영한다.

## .agent-harness Upkeep via MCP

최초 bootstrap 이후 `.agent-harness` 문서는 고정 산출물이 아니라 에이전트가 작업 증거와 사용자 컨센서스를 반영해 최신화하는 운영 문서다.

- 작업 시작: `project_docs_route`로 필요한 문서만 고른다.
- 문서 갱신: `project_docs_read`로 현재 content/SHA를 읽고, 보존할 내용과 새 근거를 합쳐 `project_docs_update(confirm=true)`로 한 문서씩 갱신한다.
- false case/반복 문제: `project_docs_record(kind=caution)`으로 append한다.
- 결정/대안 기각: `project_docs_record(kind=adr)`로 append한다.
- 불확실한 사실은 단정하지 말고 `Unknown / not confirmed`와 검증 방법을 남긴다.

## Evidence-backed MCP Heuristics

- Tool 선택은 자연어 설명 품질에 민감하므로 tool description은 목적, 사용 조건, 쓰기 여부, 반환 구조를 명확히 유지한다.
- 자주 필요한 문서 전체를 세션에 항상 주입하지 말고, task별 라우팅으로 context를 줄인다.
- 쓰기 도구는 append-only 또는 dry-run 기본값처럼 실패 반경을 제한한다.

## API documentation gate

- Endpoint/controller/DTO/schema/OpenAPI changes require the API documentation gate before completion.
- Prefer `agent-harness api-doc check` or MCP `api_doc_static_check` 후 `api_doc_review`; both default to staged API candidate files so legacy Swagger/OpenAPI debt is not failed all at once.
- For NestJS Swagger projects, the gate must catch missing `@ApiOperation`, missing/invalid operation descriptions, missing `@ApiParam`/`@ApiHeader`, missing 400/401 responses, and DTO `@ApiProperty`/`@ApiPropertyOptional`/`@IsOptional` mismatches.


## OpenAPI prompt source

Endpoint/controller/DTO/schema/OpenAPI 변경 시 `.agent-harness/OPEN_API_SPEC.md`를 프로젝트별 프롬프트 source로 사용한다. `agent-harness api-doc review`는 별도 `--prompt-file`이 없으면 이 문서를 자동으로 포함한다.
