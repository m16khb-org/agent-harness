# 2026-08-04 — Post-completion base synchronization uses contract-owned authority

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: GitHub #318
- Decision: completed replacement preview and reseed share one injected
  `issueopsbasesync.Inspector`. Drift returns the contract-owned typed error
  before mutation. Released sync-base requires current stamped completion
  generation, canonical cwd, live actor, and preview fingerprint; active-holder
  authorization remains unchanged and may synchronize an in-progress branch
  before current completion or remote artifact exists.
- Ownership: the port contains only Request/Receipt/Inspector, the outbound
  adapter runs the four read-only Git observations, and
  `internal/contract/issueops` owns the public error and exact next command.
- Consequences: successful sync appends one event without changing completion,
  history, or phase. Hooks admit only exact durable-state-matching commands for
  both hosts and block near misses before they can bypass the released fence.
- Integration: #303의 generated-command provenance를 error output과 CLI의 복수
  command result에 확장한다. Typed drift `next_command`는 CLI와 MCP의 실제 error
  경로에서 봉인하고, CLI conflict `next_command`/`abort_command`는 한 번 관측한
  executable path/hash/lease generation을 공유한다. MCP catalog에는 sync-base action이
  없으므로 도달 불가능한 success binder와 test는 #326 해결로 제거한다.
- Rejected: direct Git recovery, restoring history into current completion,
  port-owned public errors, aliases/shims, rebase, force-push, and reseed-first.
