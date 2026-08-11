---
name: cautions/lessons/2026-07-07-issueops-orchestration-locks-additive-fields-worker-leases.md
description: Dated lesson — IssueOps orchestration single-entity lock boundaries and additive mixed-binary compatibility.
---

# 2026-07-07 — IssueOps orchestration locks, additive fields, and worker leases

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: codex Task 15 project docs update from codex-orchestration implementation plan
- Summary: IssueOps child orchestration must preserve single-entity lock boundaries and additive mixed-binary compatibility.
- Context: Delegated child cycles add orchestration records that can be touched by multiple sessions. The existing store is per-key/per-entity atomic and host-neutral, not a cross-entity transaction manager or process supervisor.
- Resolution: Use only one entity lock at a time and never call a same-entity with\*Lock helper from inside another same-entity lock callback. At the July 7 baseline the non-ownership orchestration fields remained additive under schema v1; issue #16 introduced schema v3 for supervised ownership/stable terminal identity, v4 for sealed mailbox/completion projection, and now v5 for publish/cleanup authority. Verify the active binary, docs, CLI/MCP schema, and daemon readback before trusting mixed-version state.
- Evidence:
  - .agent-harness/ARCHITECTURE.md actor model
  - .agent-harness/AGENT_WORKFLOW.md child-cycle contract
  - .agent-harness/CAUTIONS.md existing state and worktree lock cautions
- Alternatives / rejected options:
  - Nested parent/child/pool locks — rejected because lock ordering is hard to prove and same-entity re-entry can self-deadlock.
  - Schema-version bump for additive fields — rejected until destructive migration or incompatible read semantics are required.
  - PID-based worker ownership — rejected because agents may run across host sessions and compaction; timestamp leases are portable and recoverable.

> Incident-time IssueOps command, field, and state references are historical evidence only, not execution directives. The current contract is `skills/issueops/references/execution.md` and `.agent-harness/OPERATIONS.md`.
