# Issue #31 bootstrap coordinator seal plan

1. Add a failing lifecycle guard test for a `coordinator_preparing` record whose
   coordinator seal is absent: an exact source-root `handoff start` bootstrap
   request with one concrete `term_…` recipient must return a generated command
   containing the native event identity.
2. Keep malformed recipient, wrong ID/root, extra flag, and all other
   lifecycle actions blocked.
3. Implement only the bootstrap-guidance branch of the hook; the returned
   command must pass the existing exact lifecycle allowlist and the core
   handoff preview still performs all durable sealing checks.
4. Run focused lifecycle and IssueOps handoff-start tests, then re-attempt a
   live coordinator preview for #23 from the current source coordinator.
