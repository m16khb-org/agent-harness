# 2026-07-24 — Workpool removal

← [ADR index](../../ADR.md)

- Kind: `adr`
- Decision: Removed the bounded task-pool feature. It did not enforce host spawning; native Codex concurrency owns thread bounds, and IssueOps child cycles/execution v1 own durable delegation.
- Consequences: Existing `~/.local/state/issueops/workpool` bytes are deliberately left inert and are not deleted.
