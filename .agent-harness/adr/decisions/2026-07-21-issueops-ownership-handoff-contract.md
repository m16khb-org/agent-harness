# 2026-07-21 — IssueOps uses one ownership handoff contract

← [ADR index](../../ADR.md)

- Context: source-CWD and session-binding inference allowed an unrelated active
  cycle to capture new source work and made exact worker IssueOps calls
  ambiguous when several cycles shared one source checkout.
- Decision: keep one unversioned ownership state machine. A literal new-cycle
  start at the exact source root selects no existing cycle. An exact lifecycle
  ID resolves before source-wide inference and must match the current source or
  worker or linked-worktree context. A prep-only linked cycle remains outside
  unrelated ownership fences. After dispatch, the acknowledged owner alone
  performs implementation, publication, and completion for that cycle.
- Cleanup: completion stops at `cleanup_pending_human_decision`. After a human
  merges the MR/PR, any fresh authenticated exact-source session may preview;
  only the human-approved session records ordered cleanup receipts.
- Cutover: removed handoff protocol fields, coordinator/worker finish and
  acceptance transitions, compatibility handlers, raw-worktree migration, and
  obsolete operational artifacts are rejected or deleted rather than adapted.
- Rejected: repository-wide source fences, generic session binding as authority,
  background conversion, inferred cleanup authority, and compatibility wrappers.
