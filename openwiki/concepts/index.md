# Files

- [Response Contract Surface](contract-surface.md) - How the versioned CLI/MCP response contract system works — one catalog source feeding CLI usage, MCP tools/list and dispatch, contract schema/check output, golden snapshots, and cross-host tool conformance.
- [Domain Glossary](domain-glossary.md) - Lookup table for the repository's recurring domain terms — IssueOps cycle and phase enum, phase ledger, execution lease and generation fence, native actor receipt, sealed owner context, intent-first mutation, reconcile, durable authority, gate ledger, loop contract, channel, policy catalog, and capability vertical.
- [State, SQLite Store, and Locking](state-and-sqlstore.md) - The user-level state root, per-store SQLite layout (harness.db plus harness.lock.db), BEGIN IMMEDIATE span serialization, fail-closed schema_version=1 validation, and the state write/read/list/prune/doctor/maintain CLI/MCP surfaces.
