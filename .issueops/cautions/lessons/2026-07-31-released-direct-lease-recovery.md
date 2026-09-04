---
name: cautions/lessons/2026-07-31-released-direct-lease-recovery.md
description: Dated lesson — released direct execution recovery must render a finite next_command chain.
---

# 2026-07-31 — released direct lease 복구는 유한한 next_command 체인을 제공해야 한다

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: IssueOps direct lease dogfood
- Summary: released direct execution의 status와 replacement 결과가 다음 명령을 누락하거나 fingerprint 없는 reseed를 안내하면 write lease 복구가 중간에서 막힌다.
- Context: 부모 #117 generation 9가 released인 상태에서 child accept가 active write lease 가드에 거부됐고, execution status는 next_command를 반환하지 않았다. 기존 prepare 안내도 inventory fingerprint 없이 reseed를 지시했다.
- Resolution: StatusExecution은 완료되지 않은 writerless execution에 상태별 next_command를 반환하고, released는 replacement preview부터 시작한다. preview는 generation과 inventory fingerprint가 포함된 reseed를, direct reseed/finalize는 current token path가 포함된 claim을 반환한다. Orca claimable은 generation 상태와 관계없이 멱등 resume로 안내한다.
- Evidence:
  - internal/core/issueops/execution_lease.go
  - internal/core/issueops/execution_prepare.go
  - TestReleasedDirectRecoveryRendersFiniteCommandChain RED 후 PASS
  - TestExecutionWriterAbsentCurrentOrcaGenerationStillPointsToResume RED 후 PASS
  - focused race PASS
  - CLI/MCP response golden 두 번 연속 PASS
- Alternatives / rejected options:
  - fingerprint 없이 reseed를 수동 실행하는 우회는 generation-CAS 계약을 깨므로 기각
  - status에 token placeholder만 남기는 방식은 durable resume 시 실제 claim 경로를 다시 추론해야 하므로 기각
  - Orca current-generation claimable에서 직접 claim을 안내하는 방식은 resume가 제공하는 sealed digest handoff를 건너뛰므로 기각

> Incident-time IssueOps command, field, and state references are historical evidence only, not execution directives. The current contract is `skills/issueops/references/execution.md` and `.issueops/OPERATIONS.md`.
