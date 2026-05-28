# CLAUDE.md

Claude Code에서 이 저장소를 열면 먼저 `AGENTS.md`를 읽고 동일한 규칙을 따른다.

- 공용 하네스 결정과 작업 계약: `AGENTS.md`
- 상세 문서: `.agent-harness/`
- Claude Code native skills: 기본은 `~/.claude/skills/*` (`atomic-commit-push`, `self-verify`, `self-augment`, `project-bootstrap`). `.claude/skills/*`는 명시적 project-local attach 때만 사용한다.
- Claude Code MCP: 기본은 user-scope `agent_harness` 서버가 중앙 `bin/harness mcp`를 실행하고 shared daemon에 proxy한다. 이 레포의 `.mcp.json`은 dogfood/project-local 템플릿이다.
- LLM Wiki workflows: 이 하네스가 제공하지 않으므로 upstream `nvk/llm-wiki` plugin을 사용한다.
- 사용법은 `.agent-harness/OPERATIONS.md`를 따른다.

## API docs

- Endpoint/DTO/OpenAPI 변경 시 `.agent-harness/OPEN_API_SPEC.md`를 프롬프트로 포함하고, user-scope MCP 서버 `agent_harness`의 `api_doc_static_check` 후 `api_doc_review` 또는 `harness api-doc check --json`을 사용한다.
- 대상 repo에 `npm run swagger:check`가 있으면 그 wrapper를 우선 실행한다.
