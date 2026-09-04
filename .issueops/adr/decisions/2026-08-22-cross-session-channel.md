# Cross-session channel capability

> Family index: [`../../ADR.md`](../../ADR.md)

- Date: 2026-08-22
- Status: accepted

## Context

Codex, Claude Code, Omo 세 호스트 에이전트가 이미 같은 issueops state(SQLite,
daemon)를 공유하지만, 세션끼리 메시지를 주고받는 표면은 없었다. 사용자 요청은
"프론트와 서버가 각각의 세션에서 통신하며 작업하는 패턴" — 세션 간 조율의
원시가 필요하다.

대안 평가:

- **Orca orchestration** — 이 워크스테이션에 이미 있고 풍부하지만 Codex/Claude
  터미널 관리에 특화된 외부 의존이다. 하네스 소유 자산이 아니고 Omo 네이티브
  세션은 1급이 아니다.
- **IssueOps child cycles** — 계약 기반 비동기 협업에는 적합하지만 라이브
  ask/reply에는 무겁다(사이클 전체 라이프사이클이 필요).
- **state write/read 직접 사용** — 동작은 하지만 채널/발신자/순서/대기 의미론이
  없어 매번 재발명된다.

## Decision

1. **`channel` capability를 issueops state 위에 둔다.** `send`는 메시지를
   채널에 append하고, `recv`는 `--since <msg-id>` 이후 메시지를 읽는다.
   `--wait`는 새 메시지 또는 타임아웃까지 블로킹한다(기본 300s, 250ms 폴링).
   Codex/Claude/Omo 어느 세션이든 같은 state를 보므로 호스트 중립적으로
   통신한다.
2. **메시지 ID는 나노초 타임스탬프 hex + 난수** 다. 저장소 키 정렬이 곧 도착
   순서다 — 같은 초에 몰린 메시지가 뒤섞이는 초 단위 버전은 구현 중 테스트가
   잡아 폐기했다. 사라진 `since` ID는 위치를 알 근거가 없으므로 처음부터
   반환한다.
3. **CLI와 MCP가 같은 contract(schema v1)를 공유한다.** `channel send/recv`
   CLI와 `channel_send`/`channel_recv` MCP 도구. exit code: 0 메시지 반환,
   1 대기 타임아웃, 2 사용법 오류.
4. **조립은 looprun 패턴을 따른다.** StateDatabase 인터페이스 + 함수 변수
   주입 + composition root 배선. 채널 필터는 읽기 시점 전체 스캔이다 —
   개인 하네스 규모에 충분하고 인덱스는 실제 요구가 확인된 뒤에만 고려한다.
5. **의미론은 의도적으로 최소다.** 발신자/본문/시각만 있다. 수신 확인,
   요청-응답 상관, 브로드캐스트语义는 상위 계약(게이트 ledger, IssueOps
   child)이 이 원시 위에 필요할 때 쌓는다.

## Consequences

- 프론트/서버 패턴의 최소 단위가 동작한다: 서버 세션이 계약을 send → 프론트
  세션이 --wait로 블로킹 수신 → 질문/답장을 since 커서로 이어받기 (실동작
  데모로 검증).
- MCP tools/golden/response_contracts/Omo 카탈로그 다이제스트가 갱신된다.
- 채널은 인증 경계가 아니다 — 같은 state를 쓰는 세션끼리만 신뢰한다.
  크로스머신 통신은 webfetch/daemon 정책의 별도 관심사다.
