# 2026-07-15 — Supervised handoff coordinator isolation and bounded self-heal

← [ADR index](../../ADR.md)

**결정:** coordinator authority와 mutation lease는 전역 singleton이 아니라 `IssueOps record ID + worker worktree + sealed native session` 범위에만 결속한다. Orca terminal handle은 권한 증명이 아닌 routing metadata다.

- source worktree의 connected+writable candidate가 정확히 하나일 때만 recipient를 자동 resolve한다. 0개·다수 candidate와 다른 active record가 이미 seal한 handle은 fail-closed하며 task/dispatch를 만들지 않는다.
- worker worktree의 connected+writable baseline terminal은 정확히 하나일 때만 adopt한다. 없으면 하나를 생성하고, 다수·partial checkpoint·runtime mismatch는 recovery evidence를 남기고 멈춘다.
- self-heal은 task, dispatch, worker session, result, pending external mutation이 모두 없는 pre-dispatch 상태에만 한정한다. terminal을 자동 stop하거나 partial dispatch를 자동 취소하지 않는다.
- 서로 다른 record/worktree는 동시 진행 가능하지만 같은 record의 durable mutation은 existing record lock과 checkpoint revalidation으로 exactly-once를 유지한다.

**거절:** 전역 coordinator registry/lock은 독립 worktree의 throughput을 직렬화하고 unrelated handoff까지 deadlock 범위를 넓히므로 도입하지 않는다.
