---
name: cautions/lessons/2026-07-07-sqlite-sqlstore-span-discipline.md
description: Dated lesson — all state/locks go through sqlstore; span serializes per state root with an active-root chain.
---

# 2026-07-07 — SQLite sqlstore span 규율: active-root chain, per-root 직렬화, fresh start

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: JSON+flock → sqlite 전면 전환 세션 (사용자 결정: 전체 일괄 전환 + fresh start)
- Summary: 모든 상태 저장/락은 `internal/core/sqlstore`를 통해야 한다. span은 state root 단위로 직렬화되며, 전달된 context의 active-root chain에 이미 있는 root로 재진입하면 `*NestedSpanError`로 즉시 실패한다. 서로 다른 root는 문서화된 비순환 순서에서만 중첩할 수 있다. 레거시 JSON/lock 파일은 무시된다(마이그레이션 없음).
- Context: 4개 with\*Lock 계열(issueops, session, state, worker)이 전부 sqlstore span으로 이동했다. 같은 root 재진입과 `A -> B -> A`는 self-deadlock 위험이 있지만, remote-create는 `remote-create-live/<id>` child root에서 main IssueOps root로 이어지는 실제 `A -> B` 순서를 필요로 한다.
- Resolution: 모든 lock helper는 `WithSpan(ctx, fn)`의 `spanCtx`를 내부 호출에 전달한다. 같은 root나 chain cycle은 금지하고, distinct-root 중첩은 호출부에 순서를 명시한다. 현재 허용된 production 순서는 remote-create child root 다음 main IssueOps root다. multi-entity 작업은 가능한 한 순차 single-span 단계 + read-repair로 유지한다. 새 저장 표면은 파일 I/O가 아니라 sqlstore bucket(Get/Put/List/Delete)으로 추가한다. `harness.db`/`harness.lock.db`와 그 sidecar(-wal/-shm/-journal)는 삭제하지 않는다. 테스트 픽스처는 raw 파일 쓰기 대신 `sqlstore.Open(dir).Put(bucket, id, raw)`로 심는다. 레거시 `<key>.json`/`.lock`/`.state-lock` 파일은 fresh start 정책상 읽지도 지우지도 않는다(doctor는 무시).
- Evidence:
  - internal/core/sqlstore/span_context_test.go의 same-root, distinct-root, cycle, cancellation 회귀 테스트
  - internal/core/issueops/issueops_remote_create_claim.go의 child-root-to-main-root context 전달
  - .issueops/ADR.md "State storage moves from JSON files + flock to SQLite"
- Alternatives / rejected options:
  - per-entity sqlite 락 db — 거부: 파일 수가 flock 시절로 회귀하고 span 규율 단순성이 사라진다.
  - 레거시 JSON 자동 마이그레이션 — 거부(사용자 결정): fresh start; 필요 시 수동 재생성이 더 단순하다.

> Incident-time command, field, and state references are historical evidence, not current execution directives.
