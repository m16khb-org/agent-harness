# E2E Resource Wait Implementation Plan

Issue: #74
Lifecycle: `io-425bd56304f9`
Worktree: `/Users/habin/workspace/agent-harness.worktrees/74-e2e-resource-wait`
Base: `63f512e14257a9100bcc87101a1d6b1b25dc84ae`

## Scope

Implement the approved CLI-only `resource wait` admission gate from
`docs/superpowers/specs/2026-07-24-e2e-resource-wait-design.md`. It samples
host resources until three consecutive stable intervals, without executing an
E2E command or adding MCP, hook, daemon, worker, state, or policy surfaces.

## Tasks

1. Define `internal/port` sampler DTOs and `internal/core/resourcewait`.
   - Contract: the `e2e` thresholds, deterministic blocker ordering, bounded
     five-sample history, three-stable-sample state machine, typed terminal
     statuses, and monotonic deadline semantics are host-neutral.
   - RED: pure evaluator and fake sampler/clock/sleeper tests cover every
     threshold boundary, baseline behavior, reset, deadline, cancellation,
     failure, overflow, and retained-sample bound.
   - GREEN: add the smallest pure profile/evaluator/wait implementation.

2. Add `internal/adapter/systemresource` collectors and share the pipe probe.
   - Contract: Darwin and Linux parsing creates normalized samples; workspace
     and temp disk observations use `Statfs`; unsupported/malformed required
     evidence fails as an error; `doctor` keeps its existing public pipe
     fields and threshold.
   - RED: platform parser fixtures cover valid, whitespace, malformed,
     missing-field, page-size, overflow, same-filesystem, and probe failures.
   - GREEN: implement fixed-path Darwin/Linux probes and move the existing pipe
     measurement behind the shared resource boundary without altering doctor
     behavior.

3. Add the `cmd/harness/resourcecli` adapter and root/contract wiring.
   - Contract: only `resource wait` is accepted; flag defaults and validation
     match the approved spec; JSONL progress goes only to stderr; final JSON is
     one stdout object; ready/timed_out/cancelled/error map to 0/3/3/1.
   - RED: CLI tests prove invalid flags, each terminal result, progress stream
     separation, top-level usage/catalog entry, and response-contract fields.
   - GREEN: wire the adapter in `harnessapp`, `adapter/cli`, `contractcli`,
     response/usage goldens, and deterministic self-verify fixture.

4. Verify, inspect the diff for unnecessary abstraction or legacy behavior,
   and commit the isolated change.
   - Run focused package tests, response/usage goldens, full and race suites,
     build, contract check, and the observational live smoke specified by the
     issue.
   - Record AC-01 through AC-08 in
     `.agent-harness/turing/issueops-v1-d1a6f8649c6d2589.json`; do not push,
     create a PR, complete the execution, merge, or clean up.

## Compatibility Review

- Additive root command only; existing commands, MCP tool ordering, doctor
  pipe fields, worker authority, and persistent state remain unchanged.
- Rollback is removal of the resource command and its private packages;
  callers retain ownership of E2E execution.
- Primary implementation risk is OS metric interpretation. Deterministic
  parser fixtures and fake wait dependencies isolate it from ambient load.

## Devil's-Advocate Review

The design rejects embedding waiting in `doctor` or running the E2E command
from the harness. The implementation must reject arbitrary profiles and argv,
must not turn transient pressure into an installation-health finding, and must
not weaken existing doctor pipe regression assertions.
