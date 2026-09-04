# 2026-07-08 — SQLite store maintenance policy

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: sqlite store maintenance cycle (plan `docs/superpowers/plans/2026-07-08-sqlite-store-maintenance.md`)
- Summary: Store maintenance (WAL checkpoint truncate + sidecar permission repair) runs automatically on the session-start hook at most once per 24h via a sentinel-mtime gate, with a manual `state maintain` CLI fallback. Orphan session bindings (cycle done or absent) are swept by `issueops cleanup stale --apply`. VACUUM is explicitly not adopted.
- Context: After the sqlite migration, three operational defects were measured: WAL files held high-water (issueops WAL 4.1MB vs DB 200KB), sidecar files could be created with 0644 under umask 022, and stale session bindings accumulated without any prune surface.
- Decision:
  - `sqlstore.Open` validates the exact state root as a real directory, repairs it to 0700, rejects symlink/non-regular known SQLite paths, and repairs the fixed main/sidecar set to 0600 before returning, including cached opens.
  - `sqlstore.Maintain` runs `PRAGMA wal_checkpoint(TRUNCATE)` and re-asserts 0600 only on the fixed known store file/sidecar set. It is safe concurrent with readers/writers; busy checkpoints are skipped (Checkpointed=false), not errors.
  - `state maintain` CLI/MCP covers four fixed roots (state, issueops, worker, loop) plus direct `projects/<repo-id>` directories that already contain a regular `issueops.db`. Missing fixed roots are reported as skipped; lifecycle-only project namespaces are neither listed nor materialized.
  - `MaybeMaintainStateStores(24h)` amortizes maintenance on the session-start hook via `.last-store-maintain` sentinel, mirroring `MaybeDetectStuckWorkerJobs(6h)`.
  - Session binding cleanup (`FindStaleBindings`/`PruneStaleBindings`) runs in `ScanStaleIssueOpsCycles` with TOCTOU re-checks.
- Rationale: WAL checkpoint is ms-scale and safe concurrent; a 24h amortization interval keeps the hot path predictable without needing a timer-based scheduler. Direct-only project discovery is bounded and runs only when the sentinel allows maintenance; a fresh-sentinel skip remains stat-only. VACUUM requires an exclusive lock and rewrites every page — unjustified at 200KB DB size. The sentinel pattern is already proven for stuck-worker detection.
- Consequences: WAL files across fixed and discovered project stores stay near header size; sidecars are always 0600; orphan bindings are prunable. The `.last-store-maintain` sentinel is recognized by the state doctor.
- Evidence:
  - internal/core/sqlstore/maintain.go, maintain_test.go
  - internal/core/sqlstore/permissions_test.go (exact root/file modes, cached drift repair, invalid-path and unrelated-file boundaries)
  - internal/core/sqlstore/resource_test.go (200 repeated opens preserve handle identity, handle-map size, and warmed connection counts)
  - internal/core/state/state_maintain.go, state_maintain_test.go
  - internal/core/issueops/issueops_stale_scan.go (session binding scan integration)
  - internal/core/issueops/session/session.go (FindStaleBindings, PruneStaleBindings)
  - cmd/issueops/hookcli/hookcatalog/catalog.go (MaybeMaintainStateStores wiring)
  - Dogfood: `state maintain --json` truncated issueops WAL from 1.2MB to 0; doctor healthy
- Alternatives / rejected options:
  - VACUUM / auto_vacuum — rejected: 200KB DB, exclusive lock cost >> space recovery. Revisit at multi-MB scale.
  - sqlstore handle eviction — rejected: the repeated-open measurement shows no handle-map or warmed connection growth for one cached root. This is not an OS-wide FD bound; revisit only with measured unique-root growth.
  - Timer-based scheduler in daemon — rejected: sentinel pattern is simpler and needs no daemon-side timer.
