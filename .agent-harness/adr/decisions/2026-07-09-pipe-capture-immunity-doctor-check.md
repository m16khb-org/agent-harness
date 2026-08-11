# 2026-07-09 — Pipe-capture immunity and pipe-capacity doctor check

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: `.agent-harness/plans/pipe-pressure-and-session-conflict.md`
- Summary: macOS pipe KVA pressure is handled with three layers: tests are immune to small pipe buffers, doctor observes pipe capacity, and the runbook keeps host-process restart as the operator action.
- Context: Under system-wide pipe pressure, new macOS pipes were observed at 512B instead of 16KB. CLI tests that wrote stdout/stderr completely before reading from the pipe could block forever once JSON output exceeded the degraded buffer. The problem was amplified by long-lived host processes and orphan `.test` binaries.
- Decision:
  - Keep the mitigation in test code, not kernel tuning: `internal/testsupport` owns pipe-safe stdout/stderr capture helpers that start reader goroutines before executing the captured function.
  - Convert CLI test capture helpers to delegate to `internal/testsupport`; multi-stream helpers start concurrent readers before command execution.
  - Add `agent-harness doctor` `pipe_capacity_bytes` plus a `pipe_capacity` check. Capacity below 8192B emits `pipe_capacity_degraded` as a warning with the CAUTIONS 2026-07-09 runbook pointer.
  - Keep host-process restart as an explicit user/operator action, not an automatic harness action.
- Consequences: Tests no longer rely on OS pipe buffer size for stdout/stderr capture. Doctor can show when the machine is still degraded, but degradation no longer blocks the converted tests. New capture helpers must use `internal/testsupport` unless they have a documented reason and concurrent reader proof.
- Evidence:
  - `internal/testsupport/capture.go` and `capture_test.go`
  - CLI test-helper sweep in `cmd/harness/*`
  - `internal/core/doctor/checks.go`, `doctor.go`, and `pipe_capacity_test.go`
  - Evidence files under `.agent-harness/evidence/pipefix-*.txt`
- Alternatives / rejected options:
  - Kernel/sysctl tuning — rejected because it is host-specific, privileged, and does not make tests portable.
  - Stress tests that intentionally exhaust kernel pipe resources — rejected because they mutate global machine state and would be non-deterministic.
  - Simultaneous MCP proxy lifecycle/churn changes — rejected because the user-visible failure is test capture blocking; proxy lifecycle is a separate scope.
  - Auto-restarting Codex or other host processes from doctor — rejected because the leaking process belongs to the host/user session boundary.
  - Immediately folding the existing harnessapp helper into `internal/testsupport` — rejected because it already uses the safe concurrent-reader pattern and was explicitly out of the sweep scope.
