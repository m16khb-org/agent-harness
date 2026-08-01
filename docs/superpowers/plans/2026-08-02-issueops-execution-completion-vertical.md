# #198 — IssueOps execution completion vertical 구현 계획

- GitHub issue: https://github.com/m16khb/agent-harness/issues/198
- lifecycle: `io-3cec83770d84`
- child branch: `198-issueops-execution-completion-vertical`
- parent branch: `117-hexagonal-architecture-migration`
- exact base: `990ec6051fa13660e9a6f0c64a5249baa2915ad6`
- 설계: `docs/superpowers/specs/2026-08-02-issueops-execution-completion-vertical-design.md`

## Task 0 — deterministic legacy oracle

1. legacy completion에 test-only injected clock seam을 최소 추가한다.
2. success/idempotent/deny/persistence/settlement fixture의 raw result, record bytes,
   holder index와 side-effect trace를 캡처한다.
3. 아직 없는 vertical observer를 호출하는 differential test를 먼저 RED로 확인한다.

검증:

```bash
go test ./internal/core/issueops -run 'CompletionVerticalDifferential' -count=1 -v
```

## Task 1 — contract와 pure domain decision

1. contract command/snapshot/receipt/result를 legacy fixture가 요구하는 최소 필드로 정의한다.
2. apply/idempotent/deny와 evidence equality 테스트를 RED로 작성한다.
3. 순수 transition을 구현하고 domain이 contract 외 internal package를 import하지 않게 한다.

검증:

```bash
go test ./internal/contract/issueopscompletion ./internal/domain/issueopscompletion -count=1
```

## Task 2 — application orchestration

1. consumer-owned Repository, EnvironmentVerifier, Clock, TaskSettler port와 fakes를 만든다.
2. legacy validation/observation/apply/settle 순서를 test trace로 RED 고정한다.
3. atomic apply failure, direct/orca/nil/failing settler, idempotent retry를 구현한다.

검증:

```bash
go test ./internal/application/issueopscompletion -count=1
go test -race ./internal/application/issueopscompletion -count=1
```

## Task 3 — inbound/outbound adapters와 differential GREEN

1. core DTO/result mapping inbound adapter를 만든다.
2. 기존 raw record/index transaction과 Git/filesystem/artifact/clock/Orca 구현을 outbound
   adapter 뒤에 둔다.
3. complete matrix의 legacy/new bytes, errors, side effects를 모두 GREEN으로 만든다.

검증:

```bash
go test ./internal/adapter/inbound/issueopscompletion ./internal/adapter/outbound/issueopscompletion -count=1
go test ./internal/core/issueops -run 'ExecutionComplete|CompletionVerticalDifferential' -count=1
```

## Task 4 — production routing과 caller-zero cleanup

1. complete handler/dependency를 공개 compatibility seam에 추가한다.
2. harnessapp가 새 service를 조립하고 CLI/MCP가 같은 handler를 사용하게 한다.
3. missing handler fail-closed와 legacy fallback 금지 fitness test를 먼저 RED로 만든다.
4. production caller 0을 증명한 completion 전용 orchestration만 제거한다.

검증:

```bash
go test ./cmd/harness/issueopscli ./cmd/harness/mcpcli ./cmd/harness/harnessapp -run 'ExecutionComplete|ExecutionHandler|ResponseContract' -count=1
go test ./internal/architecture -run 'Dependency|Completion' -count=1
```

## Task 5 — review, publication, merge, cleanup

1. gofmt, focused unit/race, architecture, golden, scoped vet/build를 실행한다.
2. AI slop cleanup과 fresh independent implementation review를 통과한다.
3. Turing report를 최신 head evidence로 기록한다.
4. atomic commit/push 후 base parent branch의 draft PR을 생성하고 readback한다.
5. final head GitHub CI 전체 test/race/self-verify를 확인해 merge한다.
6. completion reflection, #198 close, parent acceptance, child branch/worktree/lifecycle
   cleanup을 수행한다.

검증:

```bash
go test ./internal/domain/issueopscompletion ./internal/application/issueopscompletion ./internal/adapter/inbound/issueopscompletion ./internal/adapter/outbound/issueopscompletion -count=1
go test -race ./internal/domain/issueopscompletion ./internal/application/issueopscompletion ./internal/adapter/inbound/issueopscompletion ./internal/adapter/outbound/issueopscompletion -count=1
go test ./internal/core/issueops -run 'ExecutionComplete|CompletionVerticalDifferential' -count=1
go test ./internal/architecture -run 'Dependency|Completion' -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
go vet ./internal/domain/issueopscompletion/... ./internal/application/issueopscompletion/... ./internal/adapter/inbound/issueopscompletion/... ./internal/adapter/outbound/issueopscompletion/...
go build -o bin/agent-harness ./cmd/harness
git diff --check
```
