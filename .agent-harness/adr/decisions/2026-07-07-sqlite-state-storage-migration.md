# 2026-07-07 — State storage moves from JSON files + flock to SQLite (sqlstore)

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: user decision in follow-up session ("파일락이 아니라 sqlite3 기반으로 구현"), scope confirmed as full migration of all five lock families with a fresh start
- Summary: Every harness state layer (issueops cycles, session bindings, state KV, worker jobs) persists as rows in a per-state-root SQLite database (`harness.db`), and every read-modify-write span serializes through a held `BEGIN IMMEDIATE` transaction on a dedicated lock database (`harness.lock.db`). The flock layer is deleted.
- Context: The previous layout stored one JSON file per record with per-entity `flock` advisory locks. That left a documented P1 gap on `!unix` platforms (in-process mutex only), accumulated `.lock` inode files that could never be deleted safely, required orphan-lock sweeps, and offered no transactional listing. The five with*Lock families shared the same discipline but four separate implementations.
- Decision: `internal/core/sqlstore` owns storage and spans. Per state-root directory: `harness.db` (WAL, `records(bucket,id,data)` JSON blob rows) plus `harness.lock.db` used only as a crash-safe cross-process span lock (transaction dies with the process, exactly like flock). Data writes autocommit so a span's own writes stay visible mid-span, matching flock-era semantics. `WithSpan(ctx, fn)` propagates an ordered active-root chain: a root may appear only once, same-root or cyclic re-entry returns `*NestedSpanError` before waiting, and distinct roots are allowed only in a documented acyclic order. The retained production order is `remote-create-live/<id>` child root followed by the main IssueOps root. Existing JSON state is NOT migrated (fresh start); legacy `*.json`/`*.lock` files are ignored by the state doctor. Record JSON schemas, IDs, and CLI/MCP response shapes are unchanged; `path` fields keep the legacy `<dir>/<key>.json` shape as a stable per-record identifier.
- Rationale: SQLite gives real cross-process locking on every platform (closing the `!unix` gap), removes lock-inode lifecycle rules and orphan sweeps, and consolidates five storage implementations into one. The pure-Go driver (`modernc.org/sqlite`) keeps the single-binary standalone policy — no cgo, no external service.
- Consequences: State roots now contain two SQLite files instead of JSON trees; raw-file inspection is replaced by `state read`/`state list`/`issueops status` CLI surfaces or any sqlite3 client. Concurrency granularity is per state root, not per entity — conservative but correct. Pre-migration state is inert on disk until manually removed.
- Evidence:
  - internal/core/sqlstore/span_context_test.go (`TestWithSpanRejectsActiveRootReentry`, `TestWithSpanAllowsDistinctRootsAndRejectsCycle`, local/SQLite cancellation and panic cleanup)
  - internal/core/sqlstore/process_crash_test.go (`TestWithSpanRecoversAfterHolderProcessIsKilled`, repeated normal and race runs)
  - internal/core/issueops/issueops_remote_create_claim_test.go (create/reconcile durable projection tests retain child-root-to-main-root order)
  - Migration commits with package + consumer + race batteries green
  - internal/core/state, internal/core/worker ports with doctor/migrate/prune operating on rows
- Alternatives / rejected options:
  - Keep JSON files and replace only the lock mechanism with sqlite — rejected: two storage layers, none of sqlite's transactional benefits.
  - Windows LockFileEx implementation of the flock layer — rejected: solves one platform gap while keeping lock-inode lifecycle complexity.
  - One long-lived data transaction per span — rejected: routing all inner reads/writes through the span transaction requires goroutine-identity plumbing; the two-database design preserves existing visibility semantics with none of that.
