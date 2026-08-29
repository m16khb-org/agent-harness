---
type: state-sqlstore
title: State, SQLite Store, and Locking
description: The user-level state root, per-store SQLite layout (harness.db plus harness.lock.db), BEGIN IMMEDIATE span serialization, fail-closed schema_version=1 validation, and the state write/read/list/prune/doctor/maintain CLI/MCP surfaces.
tags: [state, sqlstore, sqlite, locking, wal, doctor, maintenance, cli, mcp, fail-closed]
verified:
  - by: openwiki/0.4.3
    at: 2026-08-29T12:09:25.684Z
sources:
  - id: openwiki-source-6db0bcf7889d6c88ab8e842e
    resource: repo://.agent-harness/adr/decisions/2026-07-07-sqlite-state-storage-migration.md
  - id: openwiki-source-b46ceec10f6309d019ca768f
    resource: repo://.agent-harness/adr/decisions/2026-07-08-sqlite-store-maintenance-policy.md
  - id: openwiki-source-e31e8beb2f56c36939086f18
    resource: repo://.agent-harness/architecture/runtime.md
  - id: openwiki-source-41b97487b84851d33148edda
    resource: repo://.agent-harness/cautions/lessons/2026-07-07-sqlite-sqlstore-span-discipline.md
  - id: openwiki-source-c7aee5585e8e4064f6575c6b
    resource: repo://.agent-harness/cautions/runtime.md
  - id: openwiki-source-5dac37a40f9db429ffba74cc
    resource: repo://cmd/harness/daemoncli/daemonpaths/paths.go
  - id: openwiki-source-aa98ca3461d666bdda04e164
    resource: repo://cmd/harness/harnessapp/sqlstore_wiring.go
  - id: openwiki-source-9d4d8f56542ec48ffbb1d942
    resource: repo://cmd/harness/harnessapp/state_store_wiring.go
  - id: openwiki-source-d1726846b36e25ca6ea88c29
    resource: repo://cmd/harness/harnessapp/state_wiring.go
  - id: openwiki-source-092c7f3dd75fc49176e014f3
    resource: repo://cmd/harness/hookcli/hookcatalog/catalog.go
  - id: openwiki-source-87229c1e74ad7e40b120fc63
    resource: repo://cmd/harness/mcpcli/mcp_tool_policy_state.go
  - id: openwiki-source-9e49de3a53f4c879efe63888
    resource: repo://cmd/harness/statecli/state_cli_maintenance.go
  - id: openwiki-source-b531f63e819f051a6d073b62
    resource: repo://cmd/harness/statecli/state_cli_router.go
  - id: openwiki-source-b51e99c7a396b31a46b51463
    resource: repo://cmd/harness/statecli/state_cli.go
  - id: openwiki-source-f07af8c530869a914f8817b8
    resource: repo://internal/adapter/channel/store.go
  - id: openwiki-source-1e14623752320222c82aedcd
    resource: repo://internal/adapter/doctor/doctor.go
  - id: openwiki-source-0c2020222f616904797ead6f
    resource: repo://internal/adapter/issueops/issueops_lock.go
  - id: openwiki-source-e4a419531c5514f69bc19557
    resource: repo://internal/adapter/issueops/issueops_state.go
  - id: openwiki-source-f35fe7cd6aea34637f934763
    resource: repo://internal/adapter/looprun/store.go
  - id: openwiki-source-c3887c23fe2e34af73a225d1
    resource: repo://internal/adapter/outbound/issueopsrecord/observer.go
  - id: openwiki-source-024e6a24ddef6edaebcb53e9
    resource: repo://internal/adapter/outbound/nativeactivation/sqlite.go
  - id: openwiki-source-74082511010f7dfdd748b382
    resource: repo://internal/adapter/outbound/sqlstore/maintain_test.go
  - id: openwiki-source-bf437d3817e582fee2085857
    resource: repo://internal/adapter/outbound/sqlstore/maintain.go
  - id: openwiki-source-2c56929d9aaf460a27946d05
    resource: repo://internal/adapter/outbound/sqlstore/maintenance_test.go
  - id: openwiki-source-3013eb594fa8c8d1ab8caa95
    resource: repo://internal/adapter/outbound/sqlstore/permissions_test.go
  - id: openwiki-source-e993f056d23b36ef8a297e79
    resource: repo://internal/adapter/outbound/sqlstore/process_crash_test.go
  - id: openwiki-source-a472426c515ab39f690a4a03
    resource: repo://internal/adapter/outbound/sqlstore/process_mutex_test.go
  - id: openwiki-source-5b31729f0e3018035617f871
    resource: repo://internal/adapter/outbound/sqlstore/resource_test.go
  - id: openwiki-source-5877bafa30342786debcc64d
    resource: repo://internal/adapter/outbound/sqlstore/span_backoff_test.go
  - id: openwiki-source-400514c90d2cc616423d0225
    resource: repo://internal/adapter/outbound/sqlstore/span_context_test.go
  - id: openwiki-source-85e8ea6130a6ec0df2988bda
    resource: repo://internal/adapter/outbound/sqlstore/span_observer.go
  - id: openwiki-source-69578dbeed4977bc57e4a732
    resource: repo://internal/adapter/outbound/sqlstore/sqlstore_test.go
  - id: openwiki-source-1bb7e294c7243e8798131d47
    resource: repo://internal/adapter/outbound/sqlstore/sqlstore.go
  - id: openwiki-source-34800373e8fe079a603560c1
    resource: repo://internal/adapter/outbound/state/state_doctor_test.go
  - id: openwiki-source-08a207a68821457821e839f4
    resource: repo://internal/adapter/outbound/state/state_doctor.go
  - id: openwiki-source-c533884dbfc75fac1cf26f24
    resource: repo://internal/adapter/outbound/state/state_maintain_test.go
  - id: openwiki-source-9ecb2810604096d48e010933
    resource: repo://internal/adapter/outbound/state/state_maintain.go
  - id: openwiki-source-62541dc488864afddb9a1e15
    resource: repo://internal/adapter/outbound/state/statedir_dependency.go
  - id: openwiki-source-e25ce8207d3653343a14b667
    resource: repo://internal/adapter/worker/store.go
  - id: openwiki-source-6ff6e4ad125ca0ca70428aa6
    resource: repo://internal/adapter/worker/worker_lock.go
  - id: openwiki-source-a74e833e3a9c5b46abfe81c6
    resource: repo://internal/application/state/doctor.go
  - id: openwiki-source-a9c6aa5c7be7fde7980e3ec1
    resource: repo://internal/application/state/prune.go
  - id: openwiki-source-4416f1a4d87ae1e92a73f458
    resource: repo://internal/application/state/service.go
  - id: openwiki-source-39a5224cce66838904370e0a
    resource: repo://internal/contract/state/invalid_matrix_test.go
  - id: openwiki-source-660bea4cf69b44f9d13257fe
    resource: repo://internal/contract/state/record.go
  - id: openwiki-source-e829704a2d7936db9389c1e7
    resource: repo://internal/contract/state/results.go
  - id: openwiki-source-e406de4445a86d12685110ac
    resource: repo://internal/domain/mcp/state_catalog.go
  - id: openwiki-source-ef6884b1f1e60b71765edce4
    resource: repo://internal/domain/state/validation.go
  - id: openwiki-source-657fc589a9a7e591a72d1b50
    resource: repo://internal/domain/statepath/file_path.go
  - id: openwiki-source-0984b01a67693f4755053601
    resource: repo://internal/domain/statepath/path.go
  - id: openwiki-source-47468e46ff084325333686d8
    resource: repo://internal/port/state/state.go
generated: { by: "openwiki/0.4.3", at: "2026-08-29T17:13:20.810Z" }
---

# State, SQLite Store, and Locking

All durable agent-harness state — agent checkpoints, IssueOps cycles, loop runs,
worker jobs, cross-session channels, native-activation transitions, and project
lifecycle namespaces — lives under **one user-level state root** and is stored as
rows in per-directory **SQLite** databases. Every read-modify-write span is
serialized by a `BEGIN IMMEDIATE` transaction held on a dedicated lock database,
so a crashed holder can never deadlock later writers. This page describes the
layout, the `sqlstore` engine, the locking discipline, the fail-closed schema
validation, and the `state` CLI/MCP maintenance and diagnostic surfaces.

Related pages: [Domain Glossary](domain-glossary.md),
[Source Map](../architecture/source-map.md), [Runbook](../operations/runbook.md),
[IssueOps Cycle](../workflows/issueops-cycle.md),
[Runtime Surfaces](../workflows/runtime-surfaces.md).

## State root layout

The root defaults to `~/.local/state/agent-harness/` and is overridden by
`HARNESS_STATE_DIR` (resolved to an absolute path); with no home directory it
falls back to a temp-dir path. One directory is one **store root**, and each
store root that holds records owns exactly two SQLite files:

- `harness.db` — all records as `(bucket, id, data-JSON)` rows (WAL journal).
- `harness.lock.db` — exists only to carry the cross-process span lock.
- `-wal`, `-shm`, and `-journal` sidecars appear transiently next to either file.

<!-- openwiki: mermaid parse failed and this diagram was converted to a text fence so it does not break rendering. Fix the diagram source and restore the mermaid fence. Parser error: Heuristic: an unescaped angle bracket inside a label breaks rendering; rephrase the label. -->
```text
flowchart TD
    ROOT["State root<br/>~/.local/state/agent-harness/<br/>(override HARNESS_STATE_DIR)"] --> ROOTDB["harness.db<br/>bucket: state<br/>agent checkpoints, self-workflow state"]
    ROOT --> LOCK["harness.lock.db<br/>span lock only"]
    ROOT --> LOOPDB["loop/harness.db<br/>bucket: loop<br/>one row per loop id"]
    ROOT --> IODB["issueops_v1/harness.db<br/>buckets: issueops_v1, lease_holder_v1,<br/>external_intent_v1, artifact_stage_v1"]
    ROOT --> PROJ["projects per repo fingerprint hash<br/>project.json and doc-upkeep-queue.jsonl<br/>plus harness.db when a store exists"]
    ROOT --> DAEMON["daemon dir<br/>socket, pid, lock, log files"]
    ROOT --> WORKERDB["worker/harness.db<br/>bucket: worker<br/>(HARNESS_WORKER_DIR override)"]
    ROOT --> CHDB["channel/harness.db<br/>bucket: channel_v1"]
    ROOT --> ACTDB["native-activation/harness.db<br/>bucket: native_activation_v1"]
```

*The user-level state root and its per-store layout. Each node names the SQLite
buckets that live inside that store's `harness.db`; the daemon directory holds
no database, only socket/pid/lock/log files.*

The daemon directory resolves as `HARNESS_DAEMON_DIR`, else `<state root>/daemon`,
else the home fallback, so it moves with `HARNESS_STATE_DIR`. The root also holds
recognized non-SQLite artifacts: `hook-failures.jsonl`, `hook-metrics.jsonl`, the
`.last-store-maintain` sentinel, and the `audit/` and `issueops-benchmarks/`
directories — the state doctor treats exactly these (plus the database files and
their sidecar-prefixed names) as harness-owned.

## The sqlstore engine

`internal/adapter/outbound/sqlstore` owns storage and spans for every store
root, using the pure-Go `modernc.org/sqlite` driver (no cgo, no external
service — the single-binary policy). Opening a store creates the directory at
`0700` and both databases at `0600`, then initializes:

- `records(bucket TEXT, id TEXT, data BLOB, PRIMARY KEY (bucket,id)) WITHOUT ROWID`
  on `harness.db`, opened with `journal_mode(WAL)`, `synchronous(NORMAL)`, and a
  10s busy timeout;
- a single-row `span(id INTEGER PRIMARY KEY CHECK (id = 1))` table on
  `harness.lock.db`, with the busy handler disabled so `WithSpan` can implement
  its own typed, context-aware retry.

Handles are cached per absolute directory, so every caller in one process shares
the same in-process span gate and connection pool; repeated opens keep handle
identity and connection counts stable. Handles for roots that have been removed
are closed and evicted on the next `Open`. `CloseRoot` is a deliberately narrow
API for destructive maintenance: it drops the cached handle after writers have
stopped, so the store files can be removed.

Read paths that must never create or repair state use the `*Existing` family:
`GetExisting`, `ListExisting`, `GetAllExisting`, and `InspectExisting` open the
data database read-only (`mode=ro`, `query_only`) and return `fs.ErrNotExist`
for a missing store instead of materializing one. `InspectExisting` additionally
reports the buckets and non-internal schema objects of an existing root so
maintenance callers can refuse to delete state whose layout they do not
understand. Existing reads tolerate brief SQLite contention (writer commits,
daemon WAL checkpoints) with a bounded 2s busy timeout instead of failing
spuriously — this keeps lifecycle-hook lookups responsive without unbounded
waits.

### Permission discipline

`sqlstore.Open` validates the state root as a real directory (symlinks and
non-regular files are rejected without touching their targets), repairs a
permissive root to `0700`, and repairs every known database file and sidecar to
`0600` — including on cached opens, so drift is fixed on first contact rather
than trusted. Sidecars inherit the mode of the database file, and SQLite is
pre-created at `0600` so `-wal`/`-shm` can never appear at a wider mode under a
permissive umask. Unrelated files in the root are left untouched.

## Spans: BEGIN IMMEDIATE serialization

`WithSpan(ctx, fn)` is the serialization primitive. It is two-stage:

1. **In-process** — a per-directory gate (a buffered-channel token) ensures one
   span at a time among goroutines sharing the cached handle. Waiting honors
   `ctx` cancellation.
2. **Cross-process** — a `BEGIN IMMEDIATE` transaction is opened on
   `harness.lock.db` and held for the duration of the callback. Because the
   write lock lives inside the transaction, **the lock dies with the process**:
   a holder killed mid-span releases it exactly like flock, and a contender
   acquires it immediately afterward. A subprocess test kills a holder process
   and proves the contender proceeds, and a second subprocess test proves total
   ordering across two live processes.

`SQLITE_BUSY`/`SQLITE_LOCKED` during span acquisition is retried with typed,
bounded backoff — 1ms doubling to a 10ms cap, up to a 60s ceiling — always
selecting on the caller's context, so a canceled waiter never blocks behind the
lock. Initial store creation is treated as cross-process contention too: `Open`
retries only typed busy errors for up to 10s and never masks permission,
symlink, or schema errors as contention.

### Nested spans and the active-root chain

`WithSpan` propagates an ordered **active-root chain** in the context. A root
may appear only once: same-root or cyclic re-entry returns a
`*NestedSpanError` (naming the requested and active roots) **before waiting**,
which is what makes self-deadlock structurally impossible rather than merely
discouraged. Distinct-root nesting is allowed only in a documented acyclic
order — historically the `remote-create-live/<id>` child root followed by the
main IssueOps root, an ordering that was retained in tests and retired together
with the legacy remote-create claim path. Within a span, the span's own writes
are visible because data writes **autocommit** on `harness.db` (flock-era
visibility semantics). The narrow exceptions are `Apply` and
`CompareAndApply`/`CompareAndApplyFunc`, which commit multiple record mutations
(upserts, deletes, and `RequireAbsent` insert-preconditions) atomically in one
`harness.db` transaction; raw-byte compare-and-set failures surface as
`RawCASError`. Note that these `Apply`-family calls bypass the span gate by
design — destructive paths that must order against concurrent spans wrap them
in `WithSpan` rather than lowering the gate inward (a recorded lesson).

Consumers see spans through `port/state`'s `Store` interface (`Get`, `Mutate`,
`List`, `WithSpan`); the IssueOps cycle lock, worker job lock, loop lifecycle,
and the state service's `Update`/`WithKeyLock` all route through the same
primitive. Per-root (not per-entity) granularity is deliberate: conservative
but correct, and the lock has no inode lifecycle to manage.

### Span observability

`WithSpanObserver` attaches a callback that receives `SpanObservation`
(outcome `success`/`error`/`canceled`/`nested`, contended flag, wait and hold
durations) after the lock is released. The IssueOps record store wraps this
into a JSONL observer that logs only contended or slow spans (default threshold
100ms), so contention is measurable without a hot-path tax.

## Fail-closed schema validation

State KV records are `RecordEnvelope` JSON: `schema_version`, `key`, `content`,
`updated_at` (RFC3339Nano UTC), and `bytes`. Validation is **current-only and
fail-closed**:

- `schema_version` must equal `1` — missing, `0`, and future versions are all
  rejected; there is no promotion path, and the `state migrate` surface was
  removed. A non-current record means repair or delete it, not upgrade it.
- The record's `key` must match the requested key, and `bytes` must equal the
  content length (`ValidateRecord`).
- Decoding is strict: unknown fields (e.g. legacy authority fields), malformed
  JSON, and trailing JSON all fail.
- `state doctor` additionally parses `updated_at`.

Every failure mode collapses to one generic typed error, `invalid state`
(`ErrInvalidState`), so CLI, MCP, and application projections cannot leak schema
details. An absent record is a plain not-found, distinct from invalid. The
invalid matrix is pinned by test: missing/zero/future schema, malformed JSON,
legacy field, key mismatch, and byte mismatch each return `invalid state`.

Key constraints are enforced by `statepath.NormalizeKey`: keys match
`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$` (max 128 chars), trimmed, with no `/` or
`\` and no `..`, so a key can never traverse out of the store. Although records
live in SQLite, response `path` fields keep the legacy `<dir>/<key>.json` shape
as a stable per-record identifier — JSON-era record schemas, IDs, and response
shapes were preserved by the SQLite migration, and pre-migration `*.json` /
`*.lock` files are inert: never read, never migrated (fresh start), and ignored
by the state doctor.

## State CLI and MCP surface

The `agent-harness state` command and the MCP `state_*` tools share one
composition-root-injected store adapter; transports know keys and result shapes,
never storage mechanics.

| Command / tool | Behavior |
| --- | --- |
| `state write --key KEY (--value TEXT \| --input FILE \| --stdin)` / `state_write` | Writes one `state`-bucket row inside a span; exactly one content source allowed; key must normalize. |
| `state read --key KEY` / `state_read` | Read-only `GetExisting` lookup; missing key is not-found; prints content (or JSON). |
| `state list` / `state_list` | Lists valid records (key, `updated_at`, bytes, schema) sorted by key. |
| `state prune --max-age DURATION [--confirm]` / `state_prune` | Dry-run by default; `--confirm` deletes. |
| `state doctor` / `state_doctor` | Read-only integrity inspection (below). |
| `state maintain` / `state_maintain` | WAL checkpoint + permission repair across store roots (below). |

The `harness://state` MCP resource returns the same JSON index as `state list`.
`state prune` computes `cutoff = now − max_age` over record `updated_at` values,
selects matching records older than the cutoff, reports pruned/kept lists
sorted by key, and deletes only with `confirm` — the result always marks
`dry_run: !confirm`, so an automated caller can never delete implicitly. The
application layer also offers a prefix-bounded variant (`PrunePrefix`, with an
optional retained-count cap used by self-workflow retention); the CLI exposes
only `--max-age`.

## state doctor: reports, never fixes

`state doctor` reads the `state` bucket with the non-materializing existing
reader (an absent store is healthy and empty, not an error) and **only
reports**; it never modifies checkpoints. Issues:

- `invalid_state` (severity `error`) — a row that fails strict decode, record
  invariants, or timestamp parsing: this is how missing/zero/future
  `schema_version` rows, byte-count drift, key mismatches, and malformed JSON
  surface. Because validation is fail-closed, an invalid row is inert: reads of
  it fail, and doctor names it.
- `invalid_key` (severity `error`) — a row ID that fails key normalization.
- `unexpected_file` / `unexpected_directory` (severity `warning`) — entries in
  the state root outside the harness-owned allowlist (the two database names
  plus sidecars, the hook ledgers, the maintain sentinel, and the owned
  directories such as `projects`, `daemon`, `worker`, `loop`, `issueops_v1`,
  `native-activation`, `audit`, `issueops-benchmarks`).

The result reports `checked`/`valid` counts alongside issues and sets
`healthy = len(issues) == 0`; issues are deterministically sorted. The
comprehensive `agent-harness doctor` embeds state doctor output as its
`state_store` check (surviving issues become `state_*` issues) and, when state
itself cannot be inspected, suggests checking directory permissions or pointing
`HARNESS_STATE_DIR` at a writable location. Repair is always a deliberate
operator action — typically deleting the offending row — never an automatic
rewrite.

## state maintain: WAL checkpoint and 0600 repair

`state maintain` iterates store roots and runs `sqlstore.Maintain` on each:

- **Coverage.** Four fixed roots — the base root (`state` bucket),
  `issueops_v1`, `worker` (honoring `HARNESS_WORKER_DIR`), and `loop` — plus
  every `projects/<repo-id>/` directory that already contains a regular
  `harness.db`. A root without a store is reported as `skipped`, never
  materialized; lifecycle-only project namespaces (profile JSON with no
  database) are neither listed nor created, and project symlinks are not
  followed.
- **WAL checkpoint.** `PRAGMA wal_checkpoint(TRUNCATE)` truncates
  `harness.db-wal`, reported as `wal_bytes_before` → `wal_bytes_after`.
  Maintenance is safe concurrently with readers and writers: when a writer
  holds the WAL busy, the checkpoint is skipped for this pass
  (`checkpointed: false`) instead of erroring.
- **Permission repair.** Re-asserts `0600` on the fixed known set of database
  files and sidecars, listing repaired names in `permissions_fixed`.

The maintenance service is dependency-injected (root discovery, existence
check, per-store maintain) so the application layer stays free of filesystem
plumbing. **Hooks never auto-run maintenance**: the registered SessionStart
context hook renders only the static project-doc catalog and performs no
SQLite work. (The 2026-07-08 ADR originally amortized maintenance onto the
session-start hook via the `.last-store-maintain` sentinel; the hook surface
has since been reduced to thin static context, so maintenance is explicitly
invoked — `state maintain --json` — when WAL high-water or sidecar modes need
attention. The sentinel remains a recognized harness-owned file.) VACUUM is
deliberately not part of maintenance: at typical store sizes its exclusive-lock
cost exceeds its benefit.

## Focused tests that matter

- `sqlstore/span_context_test.go` — nested-span rejection, acyclic distinct
  roots, local and SQLite waiter cancellation.
- `sqlstore/process_crash_test.go`, `process_mutex_test.go` — subprocess proof
  that the lock dies with the holder process and that `BEGIN IMMEDIATE`
  serializes two live processes.
- `sqlstore/permissions_test.go` — exact `0700`/`0600` repair on open,
  symlink/non-regular rejection without target mutation, exact maintain repair.
- `sqlstore/maintain_test.go` — WAL truncation and sidecar permission restore;
  `resource_test.go` — handle identity/connection stability across 200 opens.
- `adapter/outbound/state/state_maintain_test.go` — loop + project store
  discovery, no materialization of lifecycle-only namespaces, symlink
  isolation, discovery errors.
- `contract/state/invalid_matrix_test.go` — the full fail-closed invalid matrix.
- `adapter/outbound/state/state_doctor_test.go` — corrupt-record detection,
  healthy-empty behavior, harness-owned auxiliary allowlist.
- `sqlstore/sqlstore_test.go` — existing reads never create stores and stay
  bounded under persistent contention.
tent contention.
