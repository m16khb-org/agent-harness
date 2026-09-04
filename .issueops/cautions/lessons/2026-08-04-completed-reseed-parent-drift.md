---
name: cautions/lessons/2026-08-04-completed-reseed-parent-drift.md
description: Dated lesson — resolve parent drift before completing reseed on a released current completion.
---

# 2026-08-04 — completed reseed 전에 parent drift를 먼저 해소해야 한다

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: GitHub #318
- Summary: released current completion을 바로 reseed하면 stale parent 위에서 새 generation을 열어 동일 충돌을 다시 봉인할 수 있다.
- Resolution: replacement preview와 reseed CAS 내부가 같은 outbound base observer로 parent ancestry를 확인한다. Drift면 어떤 artifact/token/history/ledger/lease mutation보다 먼저 `post_completion_sync_base_required`와 stamped-generation preview 명령을 반환한다. `Completion.Generation == 0`은 요청값으로 보정하지 않는 invalid v1 state다.
- Recovery: released current completion + drift → generation-bound sync-base preview/apply → completed replacement preview → reseed/claim → verification → re-complete. History-only receipt, direct Git merge/rebase, force-push, state JSON backfill은 권위가 아니다.
- Boundary: port는 Request/Receipt/Inspector만 소유하고 typed public error/next command는 contract, Git 관측은 outbound adapter가 소유한다.
- Parent integration: #303의 success-result binder 밖에 있던 typed error `next_command`는 CLI/MCP의 실제 error 경로에서 봉인하고, CLI의 별도 `abort_command` field는 `next_command`와 같은 canonical executable/hash/generation observation을 사용한다. Observation/binding 실패 시 unbound command fallback 없이 structured provenance error로 종료한다.
- Fixture discipline: released sync-base gate 단위 테스트는 직접 fixture를 쓸 수 있지만 production reachability 증거로 대신하지 않는다. Claimable 초기 상태에서 public execution dispatcher와 실제 claim/complete handler를 통과해 released stamped completion을 만든 뒤 preview/apply/finalize와 completion 불변성을 검증한다.
- Adapter reachability: public catalog에 없는 action의 result binder/test는 production 계약 증거가 아니라 dead code다. #326처럼 실제 schema→handler 경로가 없는 MCP sync-base success case는 제거하고, catalog에 존재하는 resume/replace의 typed-error vertical만 유지한다.

> Incident-time IssueOps command, field, and state references are historical evidence only, not execution directives. The current contract is `skills/issueops/references/execution.md` and `.issueops/OPERATIONS.md`.
