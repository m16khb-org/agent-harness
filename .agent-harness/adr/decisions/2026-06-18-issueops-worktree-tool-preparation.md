# 2026-06-18 — IssueOps implementation requires durable worktree tool preparation

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: codex
- Summary: IssueOps implementation entry is gated on persisted worktree dependency and CodeGraph preparation evidence, not only on linked worktree and plan paths.
- Context: Investigation showed `issueops worktree prepare-tools` already installed supported pnpm dependencies and initialized CodeGraph for the linked worktree, but its result was transient. `link-plan` moved the cycle directly to `implement`, so agents could start implementation before proving dependencies, manual symlink/copy/install work, or CodeGraph readiness for the exact worktree.
- Decision: Store `worktree_tools` on the IssueOps record, keep `link-plan` as plan attachment only, and let `prepare-tools` persist evidence and unlock `implement` when readiness is complete.
- Consequences: CLI, MCP, skills, docs, and response contracts must expose `worktree_tools_prepared`, `worktree_dependencies_ready`, and `codegraph_ready` as public gates. Manual dependency reuse remains explicit: perform the symlink/copy/install in the linked worktree, rerun `prepare-tools`, then proceed.
