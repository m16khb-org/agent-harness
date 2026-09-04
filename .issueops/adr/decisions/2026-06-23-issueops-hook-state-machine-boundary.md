# 2026-06-23 — IssueOps hook and state-machine boundary

← [ADR index](../../ADR.md)

Decision: IssueOps remains a main-agent state machine backed by `issueops ...` CLI/MCP durable state. Lifecycle hooks enforce only fast deterministic violations that can be inspected from the current tool event: worktree target, Korean remote artifact text, VCS issue-linking metadata, PR/MR target branch, label/assignee evidence, and numbered next-action shape.

Rationale:

- Phase transitions and missing-gate names already live in `internal/core/issueops/issueops_phase.go` and `internal/core/issueops/issueops_readiness.go`, so durable state owners are the right place for issue, plan, worktree, design review, ai-slop-clean, feedback, PR/MR, and cleanup evidence.
- Hook decisions already chain deterministic guards in `internal/core/lifecycle/lifecycle_state.go`; adding workflow work there would make host behavior stateful, slow, and harder to share across Codex and Claude Code.
- Remote writes, tests, branch/worktree preparation, background waits, review replies, merge, and cleanup require main-agent judgment about safety, reversibility, and user intent.

Rejected alternative: let hooks advance IssueOps phases or create provider artifacts automatically. This was rejected because hooks lack enough context to judge ambiguity, ownership, credentials, destructive cleanup, and review/CI tradeoffs.
