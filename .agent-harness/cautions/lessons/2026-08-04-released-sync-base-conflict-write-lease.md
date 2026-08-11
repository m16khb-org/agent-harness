---
name: cautions/lessons/2026-08-04-released-sync-base-conflict-write-lease.md
description: Dated lesson — released sync-base conflict needs a scoped resolution writer, not a reopened general lease.
---

# 2026-08-04 — released sync-base 충돌에 일반 write lease를 다시 열지 말 것

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: IssueOps #248/#332 live dogfood
- Summary: governed sync-base apply가 충돌을 남겨도 lifecycle lease는 released라서 첫 conflict edit이 `lease_released`로 차단됐다. finalize 명령만 제공하고 해소 writer를 기록하지 않으면 충돌 경로는 도달 불가능하다.
- Resolution: apply가 충돌한 released current completion에는 lease/completion generation, base OID, native actor receipt, exact conflict file set을 `sync_base_resolution`으로 봉인한다. lifecycle guard는 그 actor가 canonical root 안의 그 파일들만 바꾸는 경우에만 임시 권한을 주며 일반 active lease로 전환하지 않는다. finalize는 event append와 resolution 제거를 한 record write로 수행하고 abort도 merge 철회 뒤 resolution을 제거한다.
- Evidence:
  - `internal/core/issueops/execution_sync_base_test.go`
  - `internal/core/lifecycle/lifecycle_execution_matrix_test.go`
  - live `io-268bd6ac6e7a` generation 5 apply에서 12-conflict resolution receipt를 기록한 뒤 이전에 막힌 exact conflict `apply_patch` PASS
  - 같은 live merge의 governed abort가 receipt와 merge state를 함께 제거
- Boundary: unrelated file, 다른 actor/process, 다른 generation/completion, absolute/상위 경로 conflict entry는 계속 fail-closed한다. 충돌 해소 뒤 새 기능 변경이나 일반 검증이 필요하면 sync-base를 finalize한 다음 정상 reseed/claim lifecycle로 돌아간다.

> Incident-time IssueOps command, field, and state references are historical evidence only, not execution directives. The current contract is `skills/issueops/references/execution.md` and `.agent-harness/OPERATIONS.md`.
