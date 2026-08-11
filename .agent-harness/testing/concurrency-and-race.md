# Race, process, lock, and nondeterminism rules

[← TESTING.md](../TESTING.md) owns the test-strategy index. This module owns
race, process, lock, and nondeterminism rules. Unit/fixture/golden standards
live in [unit-and-contract.md](unit-and-contract.md); the IssueOps focused race
package set is documented in [issueops-execution.md](issueops-execution.md).

## Race detector

`go test -race ./... -count=1` is part of the basic Go verification in
[unit-and-contract.md](unit-and-contract.md). `self-verify` reports working-tree
risk and runs `go vet ./...` or `go test -race ./... -count=1` conditionally in
its `risk QA tier`. The IssueOps execution vertical also pins a focused race
package set listed in [issueops-execution.md](issueops-execution.md).

## Determinism contract

테스트는 deterministic해야 한다. well-structured 기준과 poorly-structured anti-pattern:

- deterministic하며 test order, wall-clock sleep, real network, local machine state에 의존하지 않는다.
- sleep, real external service, global mutable state, 실행 순서에 의존한다.

## Process and lock substrate

- daemon/proxy test는 socket path override, MCP stream, start/status/stop smoke, stale lock 복구를 포함한다.
- worker test는 timeout, cancellation, stale lock, concurrent job을 포함한다.

Operational-health stale-scan integration must prove `operational_dead_owner`
is report-only for missing/stale heartbeat while existing confirmed
worktree/remote evidence remains releasable after the locked fresh re-probe
(the full classifier-test rule is in
[unit-and-contract.md](unit-and-contract.md)).

## Concurrent stdout/stderr capture

stdout/stderr를 `os.Pipe`로 캡처하는 테스트는 직접 write-then-read 헬퍼를 만들지 말고 `internal/testsupport`의 동시-reader 캡처 헬퍼를 사용한다.
