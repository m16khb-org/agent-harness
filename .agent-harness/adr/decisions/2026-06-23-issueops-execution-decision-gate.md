# 2026-06-23 — IssueOps execution decision gate

← [ADR index](../../ADR.md)

Decision: `implement` readiness requires a durable `execution_decision` record. The record captures auto-proceed boundaries, hook-blocked workflow work, human gates, and sub-agent usage. Sub-agent use defaults to `none`; `planned` is allowed only when the plan records a documented pattern slug, expected benefit, tradeoffs, net-positive rationale, scope, verification, and fallback.

Rationale:

- Sub-agent research in `.agent-harness/research/subagent-tradeoffs.md` confirms that context isolation, tool gating, model specialization, fresh review, and parallel independent research can be valuable, but they carry latency/token overhead, summary-only visibility, and weaker mid-run steering.
- Hooks cannot make that tradeoff because the decision depends on current task scope, user intent, reversibility, and whether the main agent still needs continuous control.
- Persisting the decision prevents silent auto-advance from plan/tool prep into implementation when the human-in-the-loop or sub-agent boundary has not been stated.

Rejected alternative: infer sub-agent usage from hook hints or phase names. This was rejected because it would hide the tradeoff analysis and make sub-agent use depend on host-specific runtime behavior instead of durable IssueOps state.
