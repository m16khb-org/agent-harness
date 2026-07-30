# CLAUDE.md

Claude Code에서 이 저장소를 열면 먼저 `AGENTS.md`를 읽고 동일한 규칙을 따른다.

- 공용 하네스 결정과 작업 계약: `AGENTS.md`
- 상세 문서: `.agent-harness/`
- Claude Code native skills: 기본은 `~/.claude/skills/*` (`atomic-commit-push`, `self-verify`, `self-augment`, `project-bootstrap`). `.claude/skills/*`는 명시적 project-local attach 때만 사용한다.
- Claude Code MCP: 기본은 user-scope `agent_harness` 서버가 중앙 `bin/agent-harness mcp`를 실행하고 shared daemon에 proxy한다. 이 레포의 `.mcp.json`은 dogfood/project-local 템플릿이다.
- 철학: 하네스 설치·업데이트·검증 경로는 독립 실행 가능해야 한다. 외부 도구가 필요하면 해당 도구의 공식 경로로 별도 설치하고, agent-harness는 그 설치를 대행하거나 readiness gate로 요구하지 않는다.
- 사용법은 `.agent-harness/OPERATIONS.md`를 따른다.

## API docs

- Endpoint/DTO/OpenAPI 변경 시 `.agent-harness/OPEN_API_SPEC.md`를 프롬프트로 포함하고, user-scope MCP 서버 `agent_harness`의 `api_doc_static_check` 후 `api_doc_review` 또는 `agent-harness api-doc check --json`을 사용한다.
- 대상 repo에 `npm run swagger:check`가 있으면 그 wrapper를 우선 실행한다.

<!-- OPENWIKI:START -->

## OpenWiki

This repository uses OpenWiki for code documentation. Start with `openwiki/quickstart.md`, then follow its links to architecture, workflows, domain concepts, operations, integrations, testing guidance, and source maps.

OpenWiki 갱신은 자동으로 실행하지 않는다. 필요할 때 사용자가 `openwiki code --update --print`를 수동으로 실행하고 변경 diff를 검토한다. 명시적 요청 없이 생성된 OpenWiki 페이지를 직접 수정하지 않는다.

<!-- OPENWIKI:END -->
