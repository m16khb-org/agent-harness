# E2E Resource Wait Admission Gate Design

**Date:** 2026-07-24
**Status:** Approved design; implementation not started

## Problem

Resource-intensive E2E runs can begin while the local machine is already under
CPU, memory, swap, disk, or pipe pressure. This can turn an environmental
condition into a slow, flaky, or misleading test failure.

The harness currently starts its own verification commands with timeouts, but
does not provide a reusable machine-resource admission gate before an external
E2E command. `issueops doctor` already diagnoses degraded pipe capacity and
MCP file-descriptor pressure, but transient machine load is not an operational
health failure and must not change the meaning of `doctor`.

## Decision

Add a read-only, bounded CLI gate:

```bash
issueops resource wait \
  --workspace-root "$PWD" \
  --profile e2e \
  --timeout 10m \
  --interval 5s \
  --progress jsonl \
  --json
```

The command waits until the machine satisfies the built-in `e2e` resource
profile for three consecutive sample intervals. It reports structured evidence
and exits without executing the E2E command.

Callers remain responsible for running the test immediately after a successful
gate:

```bash
issueops resource wait --workspace-root "$PWD" --profile e2e --timeout 10m --json &&
  npm run test:e2e
```

This is a CLI-only v1 capability. It is not exposed as a blocking MCP tool and
is not run by hooks, `doctor`, the daemon, or the worker.

## Goals

- Distinguish resource admission failures from test failures.
- Wait through transient resource pressure without waiting forever.
- Use adaptive thresholds that account for CPU count and machine size.
- Produce stable human-readable and JSON output for automation.
- Preserve a pure final JSON object on stdout when progress is enabled.
- Support macOS and Linux with deterministic parser and wait-loop tests.
- Reuse the existing degraded-pipe threshold without changing `doctor` output.

## Non-goals

- Execute, queue, schedule, or reserve the E2E command.
- Add arbitrary shell, write, network, or container execution to the worker.
- Make all tests or `self-verify` depend on current machine state.
- Treat transient high load as an unhealthy harness installation.
- Add repository policy files or per-metric threshold flags in v1.
- Persist samples, process command lines, environment variables, or audit state.
- Automatically kill processes, free disk, restart hosts, or remediate pressure.
- Guarantee that resources cannot change after the gate returns.

## CLI Contract

### Command

```text
issueops resource wait
  [--workspace-root PATH]
  [--profile e2e]
  [--timeout DURATION]
  [--interval DURATION]
  [--progress none|jsonl]
  [--json]
```

Defaults:

- `workspace-root`: current directory, resolved to a clean absolute path
- `profile`: `e2e`
- `timeout`: `10m`
- `interval`: `5s`
- `progress`: `none`

Validation:

- v1 accepts only profile `e2e`.
- `interval` must be between `1s` and `60s`.
- `timeout` must be at least `interval * 3` and at most `60m`.
- The workspace root must exist and be a directory.

No command argv follows `--`; the gate never owns the E2E process.

### Result

```json
{
  "ok": true,
  "kind": "resource_wait",
  "status": "ready",
  "profile": "e2e",
  "workspace_root": "/absolute/path",
  "started_at": "2026-07-24T00:00:00Z",
  "finished_at": "2026-07-24T00:00:15Z",
  "waited_ms": 15000,
  "sample_count": 4,
  "required_stable_samples": 3,
  "consecutive_stable_samples": 3,
  "thresholds": {
    "max_load_1m_per_cpu": 0.75,
    "min_available_memory_bytes": 6871947674,
    "min_available_memory_ratio": 0.2,
    "max_swap_io_bytes_per_sec": 1048576,
    "min_workspace_disk_available_bytes": 53687091200,
    "min_workspace_disk_available_ratio": 0.1,
    "min_temp_disk_available_bytes": 53687091200,
    "min_temp_disk_available_ratio": 0.1,
    "min_pipe_capacity_bytes": 8192
  },
  "latest_sample": {
    "sampled_at": "2026-07-24T00:00:15Z",
    "logical_cpu_count": 8,
    "load_1m": 4.96,
    "load_1m_per_cpu": 0.62,
    "total_memory_bytes": 34359738368,
    "available_memory_bytes": 12884901888,
    "available_memory_ratio": 0.375,
    "swap_io_bytes_per_sec": 0,
    "workspace_disk_total_bytes": 536870912000,
    "workspace_disk_available_bytes": 75161927680,
    "workspace_disk_available_ratio": 0.14,
    "temp_disk_total_bytes": 536870912000,
    "temp_disk_available_bytes": 75161927680,
    "temp_disk_available_ratio": 0.14,
    "pipe_capacity_bytes": 16384
  },
  "recent_samples": [],
  "blockers": [],
  "warnings": []
}
```

`recent_samples` contains at most the five most recent bounded samples. It does
not grow with the total wait duration.

### Status and Exit Code

| Status | Exit | Meaning |
|---|---:|---|
| `ready` | `0` | Three consecutive intervals satisfied every required threshold. |
| `timed_out` | `3` | The deadline arrived while one or more resource blockers remained. |
| `cancelled` | `3` | The caller sent `SIGINT` or `SIGTERM` while waiting. |
| `error` | `1` | A required probe, parser, platform, or workspace operation failed. |

`timed_out` and `cancelled` are typed non-admission errors so the root command
can preserve the existing exit-3 meaning used for intentional policy or guard
blocks. Invalid flags fail before the wait begins using the existing CLI usage
behavior.

In JSON mode, every post-parse terminal state prints its result before returning
the typed error. The root error message remains on stderr, never stdout.

## Resource Profile

The built-in `e2e` profile uses these initial thresholds:

| Metric | Ready when |
|---|---|
| Normalized one-minute load | `load_1m / logical_cpu_count <= 0.75` |
| Available memory | bytes are at least `max(4 GiB, total * 20%)` |
| Active swap I/O | combined swap-in/out rate is at most `1 MiB/s` |
| Workspace filesystem | available bytes are at least `max(10 GiB, total * 10%)` |
| Temp filesystem | available bytes are at least `max(10 GiB, total * 10%)` |
| Pipe capacity | at least `8192` bytes |

The result exposes the resolved numeric thresholds so operators can understand
the decision without reconstructing the profile. Each `min_*_bytes` field is the
rounded-up result of the profile's `max(absolute floor, ratio floor)` rule; the
companion ratio field records the ratio component used to resolve it.

Used swap is not a blocker by itself. Only counter deltas observed during the
sample interval contribute to `swap_io_bytes_per_sec`; stale swap allocation
can remain after pressure has ended.

When the workspace and temporary directory share a filesystem, the adapter may
measure it once but must project the same measurement into both response fields.

## Stability Algorithm

The first sample establishes cumulative-counter baselines and evaluates
absolute metrics. It does not count as a stable interval because swap I/O has no
delta yet.

For each following interval:

1. Sleep using the injected sleeper until the next monotonic deadline.
2. Collect one complete sample.
3. Derive normalized load, available ratios, filesystem ratios, and swap rate.
4. Evaluate every required threshold.
5. If there are no blockers, increment `consecutive_stable_samples`.
6. If any blocker exists, reset `consecutive_stable_samples` to zero.
7. Return `ready` when the counter reaches three.
8. Return `timed_out` at the overall deadline with the last sample and blockers.

The implementation uses a monotonic clock for duration and deadline behavior.
Wall-clock timestamps are evidence only.

A sample scheduled at or before the monotonic deadline is evaluated. If it
completes the third stable interval, `ready` wins; otherwise the command returns
`timed_out`. No new sample starts after the deadline. Individual probe timeouts
are clamped to the remaining overall wait. Cancellation observed before a sample
is committed returns `cancelled`.

An unsupported platform, missing required system utility, malformed required
output, numeric overflow, invalid page size, or failed filesystem probe returns
`error` immediately. V1 does not downgrade missing evidence to a warning.

## Blocker Contract

Each blocker uses a stable code and machine-readable comparison:

```json
{
  "code": "load_high",
  "metric": "load_1m_per_cpu",
  "observed": 1.2,
  "comparator": "<=",
  "threshold": 0.75,
  "unit": "ratio",
  "summary": "normalized one-minute load exceeds the e2e profile"
}
```

V1 blocker codes:

- `load_high`
- `memory_low`
- `swap_io_active`
- `workspace_disk_low`
- `temp_disk_low`
- `pipe_capacity_degraded`

Blocker ordering is deterministic in the order above.

## OS Collection

The resource policy and wait state machine are host-neutral. OS-specific
collectors implement one sampler contract.

### macOS

- `/usr/sbin/sysctl` for `vm.loadavg`, `hw.logicalcpu`, and `hw.memsize`
- `/usr/bin/memory_pressure -Q` for the available-memory percentage
- `/usr/bin/vm_stat` for page size and cumulative swap counters
- `golang.org/x/sys/unix.Statfs` for workspace and temp filesystems
- the existing nonblocking pipe-capacity probe

System commands use fixed paths, `LC_ALL=C`, bounded output, and short individual
timeouts. The collector derives available bytes from total memory and the
reported available percentage.

### Linux

- `/proc/loadavg`
- `/proc/meminfo`, using `MemTotal` and `MemAvailable`
- `/proc/vmstat`, using `pswpin` and `pswpout`
- `golang.org/x/sys/unix.Statfs`
- the shared pipe-capacity probe

Other operating systems fail with a stable `unsupported_platform` error in v1.

## Package and Dependency Boundaries

The proposed implementation surface is:

- `internal/port`: sampler request/result and interface
- `internal/core/resourcewait`: profile, pure evaluator, wait state machine,
  blockers, typed non-admission errors, shared pipe probe
- `internal/adapter/systemresource`: Darwin/Linux collection and parsers
- `cmd/issueops/resourcecli`: flags, signal-aware context, progress, human/JSON
  output
- `cmd/issueops/issueopsapp`: dependency wiring, root command registration, exit
  mapping
- `internal/adapter/cli`: top-level command catalog and usage
- `cmd/issueops/contractcli`: required `resource_wait` response fields

The adapter depends on the port contract. The core evaluator receives samples
and does not parse OS text. Tests inject sampler, clock, and sleeper
implementations.

The current private pipe measurement moves behind a shared resource probe. The
existing `doctor` check continues to consume it with the same `8192` threshold,
field names, issue codes, and human guidance.

No new third-party dependency is required.

## Progress and Human Output

`--progress=none|jsonl` follows the self-verification convention:

- final JSON is written once to stdout;
- JSONL progress is written to stderr;
- progress marshaling failure emits a bounded `progress_error` on stderr and
  does not corrupt the final result.

Progress events:

- `wait_started`: profile, interval, timeout, required stable samples
- `sample`: elapsed milliseconds, sample number, consecutive stable count,
  blocker codes
- `wait_finished`: elapsed milliseconds and final status

Progress events do not include process lists, command lines, environment values,
or unbounded sample history.

Human mode prints the resolved profile once, then prints only blocker-set
changes and the final status. This avoids five-second log spam while making a
long wait visibly active.

## Testing

### Pure Policy Tests

- Each metric immediately below, exactly at, and immediately above its threshold.
- Adaptive memory and disk byte floors versus percentage floors.
- Deterministic blocker ordering.
- High historical swap usage with zero active swap I/O passes.
- Swap counter deltas, page-size conversion, and numeric overflow handling.

### Wait-loop Tests

- Unstable samples followed by exactly three stable intervals return `ready`.
- An unstable sample between stable samples resets the counter.
- The first baseline sample never counts toward the three stable intervals.
- Exact deadline handling returns `timed_out` with the latest blockers.
- Context cancellation returns `cancelled`.
- Sampler failure returns `error`.
- Fake clock, sampler, and sleeper avoid real wall-clock sleep and machine state.
- Recent sample retention never exceeds five.

### Adapter Parser Tests

- Darwin fixtures for `sysctl`, `memory_pressure`, and `vm_stat`.
- Linux fixtures for `/proc/loadavg`, `/proc/meminfo`, and `/proc/vmstat`.
- Whitespace, locale-neutral decimal, malformed, missing-field, overflow, and
  invalid-page-size cases.
- Same-filesystem workspace/temp projection.
- Filesystem probe failure and unsupported-platform classification.

### CLI and Contract Tests

- Flag defaults and duration validation.
- JSON result for `ready`, `timed_out`, `cancelled`, and `error`.
- Exit codes `0`, `3`, `3`, and `1`.
- `--json --progress=jsonl` leaves stdout as one parseable JSON object and emits
  valid JSONL only to stderr.
- Signal cancellation uses injected context in unit tests; no process signal is
  required for the deterministic suite.
- Top-level command catalog and canonical usage include `resource wait`.
- Compatibility contract declares required `resource_wait` response fields.
- Response-contract and command-usage goldens are updated intentionally.
- Existing `doctor` pipe-capacity JSON and warning regression tests remain
  unchanged after probe extraction.

### Self-verification Evidence

Add a deterministic `resource admission contract` self-verification step using
fake samples. It verifies the new capability without waiting on live machine
resources. The normal `self-verify` success result must remain independent of
ambient CPU, memory, swap, and disk pressure.

### Verification Commands

Implementation verification will include:

```bash
go test ./internal/core/resourcewait ./internal/adapter/systemresource ./cmd/issueops/resourcecli ./cmd/issueops/issueopsapp ./cmd/issueops/contractcli -count=1
go test ./... -count=1
go test -race ./... -count=1
go build -o bin/issueops ./cmd/issueops
go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1
./bin/issueops contract check --json
```

A live `resource wait` smoke is observational: either a structurally valid
`ready` or `timed_out` result is acceptable depending on current machine state.
It is not a deterministic completion gate.

## Compatibility and Rollout

- The change is additive: one top-level CLI command and one response contract.
- No existing command changes behavior.
- No MCP tool is added, so MCP tool ordering and blocking behavior are unchanged.
- `doctor` keeps its existing public pipe-capacity contract.
- The command writes no state and requires no migration or cleanup.
- Removing the command and its catalog/contract entry fully rolls back v1.

Threshold tuning is intentionally deferred until actual runs provide evidence
about admission latency and timeout frequency. A future change may add a
versioned resource policy or reservation lease, but neither is part of v1.

## Acceptance Criteria

- `resource wait` never executes the caller's E2E command.
- Default execution requires three stable five-second intervals and has a
  ten-minute maximum wait.
- Every required metric and resolved threshold is visible in the final result.
- Timeout, cancellation, and collection error are distinguishable by status.
- JSON stdout remains parseable when progress is enabled.
- macOS and Linux behavior is covered by deterministic fixtures.
- Default tests and self-verification do not depend on live machine resources.
- Existing `doctor` pipe behavior and public fields do not drift.
- No repository policy file, persistent state, MCP tool, or worker execution is
  introduced.
