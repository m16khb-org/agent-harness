---
name: cautions/lessons/2026-07-10-local-mcp-gateway-fd-exhaustion.md
description: Dated lesson — loopback MCP gateway FD exhaustion resets all loopback MCP connections.
---

# 2026-07-10 — 로컬 MCP 게이트웨이 FD 고갈이 모든 loopback MCP 연결을 즉시 리셋시킨다

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: glab-service-api/glab-cloud-platform 동시 실패 진단 (mcp-proxy 로그 + lsof 실측)
- Summary: launchd가 띄운 loopback MCP 게이트웨이(예: mcp-proxy)가 stateful streamable-HTTP 모드에서 세션을 회수하지 않으면(관측: 생성 1,373 vs DELETE 26) 클라이언트 재시도 폭풍(정점 분당 318 세션)이 FD를 소진시킨다. launchd 기본 soft limit은 256이라 11분 만에 EMFILE(`socket.accept() ... Too many open files`)에 도달했고, 이후 포트는 LISTEN 상태지만 모든 연결이 즉시 ECONNRESET된다. Claude Code에서는 "△ tools fetch failed" 또는 "✘ failed"로 보인다.
- Context: initialize POST는 200으로 성공하면서 SSE GET이 실패하는 구간에서는 클라이언트가 핸드셰이크 전체를 재시도하며 매 시도가 새 stateful 세션을 서버에 남긴다 — 실패가 실패를 증폭시키는 구조다. 게이트웨이 하나가 여러 named server(glab, codegraph)를 서빙하므로 장애는 특정 서버가 아니라 게이트웨이 전체 단위로 발생한다.
- Triage: (1) `lsof -nP -iTCP:<port> -sTCP:LISTEN`으로 리스너 생존 확인, (2) curl로 MCP initialize POST — 즉시 리셋이면 FD 고갈 의심, (3) `lsof -p <pid> | wc -l`을 limit과 비교, (4) 게이트웨이 stderr 로그에서 `Errno 24` 확인. `agent-harness doctor`의 `mcp_gateway` 체크가 (2)(3)을 자동화한다 — 도달 불능이면 `mcp_gateway_unreachable`, FD 512 이상이면 `mcp_gateway_fd_pressure` warning.
- Resolution: 게이트웨이 재시작으로 즉시 복구. 재발 방지는 ① streamable-HTTP를 stateless 모드로 운영(세션 누적 원천 차단, 서버→클라이언트 알림이 필요 없는 도구에 적합), ② launchd plist에 `SoftResourceLimits:NumberOfFiles` 상향(기본 256은 재시도 폭풍 아래에서 수 분 버퍼에 불과). limit 변경은 `launchctl kickstart`가 아니라 bootout→bootstrap이어야 반영된다.

> Incident-time command, field, and state references are historical evidence, not current execution directives.
