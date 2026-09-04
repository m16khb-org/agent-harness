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

### 시스템 전역 프로브는 주입한다

`lsof`, `ps` 같은 시스템 전역 프로브는 호스트의 프로세스·열린 파일 목록과 그
프로브 상한에 결과를 묶으므로 `local machine state` 의존의 대표 사례다. 유휴
머신에서 상한에 여유가 있어도 전체 스위트가 패키지를 병렬로 돌리면 상한을
넘겨 확률적으로 깨진다.

- 프로덕션 코드가 quiescence·점유·liveness를 시스템 전역 프로브로 판정한다면,
  그 관측을 의존성 시임으로 노출하고 해당 상태 기계만 검증하는 테스트는
  결정적 관측자를 주입한다. 시임은 비공개 필드로 두고 nil이면 기본 구현으로
  떨어뜨려 프로덕션 경로의 동작을 바꾸지 않는다.
- 프로브 지연으로 깨지는 테스트는 상한을 늘려 덮지 않는다. 상한 상향은 실패
  확률만 낮추고 결정성 결여를 남긴다.
- 수정의 검증은 프로브를 굶긴 상태(상한을 극단적으로 축소)에서 테스트가
  통과하는지로 한다. 수정 전에는 그 조건에서 확정 실패해야 한다.
- 프로브 상한 초과는 `ctx.Err()`가 wrap 없이 올라와 `context deadline exceeded`
  한 줄로만 보인다. 어느 프로브가 터졌는지는 메시지로 구분되지 않으므로,
  상한을 하나씩 축소해 실패를 재현하는 방식으로 원인을 가른다.

선례: `executionQuiescenceFingerprint`의 워크스페이스 점유 관측
([2026-08-28 caution](../cautions/2026-08-28-lease-quiescence-lsof.md)).

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
