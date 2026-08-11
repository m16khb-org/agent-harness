# 2026-08-10 — Default hooks are thin static context only

← [ADR index](../../ADR.md)

**결정:** Codex와 Claude의 기본 설치는 `SessionStart`와 `PostCompact`만 등록한다. 두 경로는 project-doc catalog를 host-compatible context로 렌더할 뿐 IssueOps, lifecycle reminder, runtime diagnostic, telemetry, SQLite maintenance, worker recovery, state write를 호출하지 않는다.

- 기존 `user-prompt`, `pre-tool-use`, `post-tool-use`, `pre-compact`, `stop` hook CLI는 명시적 compatibility/diagnostic surface로 보존하지만 installer가 등록하지 않는다.
- upgrade는 알려진 lifecycle event에서 agent-harness group만 제거하고 co-resident third-party group과 관련 없는 host 설정을 보존한다.
- IssueOps CAS, lease, verification, publication authority는 `agent-harness issueops ...` CLI/MCP에만 남는다. default hook output을 owner authority나 enforcement evidence로 해석하지 않는다.

**거절:** default hook에 PreToolUse enforcement, Stop relay, lifecycle-state upkeep을 유지하는 방식. 항상 실행되는 host hot path가 durable state와 ownership을 읽거나 쓸수록 latency·cross-worktree bleed·host-specific coupling이 커지고, explicit IssueOps authority와 중복되기 때문이다.
