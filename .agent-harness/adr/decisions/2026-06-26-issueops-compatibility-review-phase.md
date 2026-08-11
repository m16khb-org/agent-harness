# 2026-06-26 — IssueOps compatibility review phase

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: codex
- Summary: IssueOps implementation entry now requires a dedicated `compatibility-review` phase for backward compatibility, side effects, rollback, and verification judgement.
- Context: The existing `execution_decision` gate recorded auto-proceed and HITL/sub-agent judgement, but backward compatibility and side-effect review were not first-class state. Hook-side judgement would make progress host-event-dependent and hard to replay.
- Decision: Add `compatibility-review` between `plan` and `implement`, persist `compatibility_review` on the IssueOps record, expose CLI/MCP owner commands, and make `implement` fail closed until the review is approved and blocker-free.
- Consequences: Agents must run `issueops compatibility review` or MCP `issueops_record_compatibility_review` before implementation. Missing readiness keys are public contract (`compatibility_review`, `backward_compatibility`, `side_effects`, `rollback_plan`, `compatibility_verification`, `compatibility_blockers`, `compatibility_approval`) and must stay documented with CLI/MCP/schema changes.
