---
name: cautions/lessons/2026-07-28-update-mcp-lifetime-host-owned.md
description: Dated lesson — update does not own host stdio MCP processes; pending requests are not auto-replayed.
---

# 2026-07-28 — update의 MCP 수명은 host 소유이며 pending 요청은 자동 재생하지 않는다

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: `ah update` 중 Codex agent_harness MCP 연결 종료 재현과 Claude Code 2.1.220 `mcp list` 번들 확인
- Summary: update는 host가 소유한 stdio MCP 프로세스나 외부 MCP를 열거·종료·접속하지 않는다. agent-harness proxy는 daemon generation 교체 뒤 초기 handshake의 protocol/capability projection이 동일할 때만 세션을 복구한다.
- Resolution: 단일 요청과 구버전 NDJSON batch의 미완료 request ID는 자동 재실행하지 않고 `outcome=unknown`과 reconcile 요구를 반환한다. reconnect는 전체 20초 deadline과 host EOF cancellation을 공유한다. handshake projection이 달라지거나 initialize가 거부되면 proxy를 종료해 host 재연결을 유도한다. 사용하지 않는 SDK logging capability는 광고하지 않는다. GitLab MCP/personal wrapper 동기화는 `scripts/sync-glab-mcp.sh`를 수동 실행할 때만 수행한다.
- Cleanup boundary: `mcp cleanup --apply`는 Darwin에서만 exact current-checkout `agent-harness mcp`, `PPID=1`, verified executable/start time, signal 직전 동일 identity를 모두 만족한 프로세스를 종료한다. Linux 컨테이너 등에서는 `PPID=1`이 살아 있는 host일 수 있으므로 `skip-unsupported-platform`으로 거부한다. live-parent proxy, PID reuse, 다른 checkout, DBHub/Context7/Kordoc/개인 wrapper도 fail-closed로 건너뛴다.

> Incident-time command, field, and state references are historical evidence, not current execution directives.
