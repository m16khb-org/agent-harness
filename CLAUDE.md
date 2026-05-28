# CLAUDE.md

Claude Code에서 이 저장소를 열면 먼저 `AGENTS.md`를 읽고 동일한 규칙을 따른다.

- 공용 하네스 결정과 작업 계약: `AGENTS.md`
- 상세 문서: `agent_docs/`
- Claude Code native skills: 기본은 `~/.claude/skills/*` (`atomic-commit-push`, `self-augment`). `.claude/skills/*`는 명시적 project-local attach 때만 사용한다.
- Claude Code MCP: 기본은 user-scope `agent-harness` 서버가 중앙 `bin/harness mcp`를 실행하고 shared daemon에 proxy한다. 이 레포의 `.mcp.json`은 dogfood/project-local 템플릿이다.
- LLM Wiki workflows: 이 하네스가 제공하지 않으므로 upstream `nvk/llm-wiki` plugin을 사용한다.
- 사용법은 `agent_docs/USAGE.md`를 따른다.
