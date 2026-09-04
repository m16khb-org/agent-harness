# IssueOps execution reconcile vertical 구현 계획

> TDD로 각 단계의 RED를 먼저 확인하고 최소 구현으로 GREEN을 만든다. 로컬 full test/full race와 OpenWiki 자동 갱신은 하지 않는다.

## 1. Contract와 domain

- `internal/contract/issueopslease/reconcile.go`
- `internal/domain/issueopslease/reconcile.go`
- 대응 unit tests

candidate 1/0/multiple, authoritative zero, invocation state, attempts 0/1/2를 고정한다. 생성 mutation은 `not_invoked_proven && attempts < 2`만 retry하고, 멱등 `run_bind`는 기존 unknown-outcome bounded retry 계약을 보존한다.

## 2. Application one-stage service

- `internal/application/issueopslease/reconcile.go`
- `internal/application/issueopslease/ports.go`
- 대응 unit tests

inspection port는 attempted bool을 반환한다. 여섯 stage 각각 inspect 최대 1회, invoke 최대 1회, apply 최대 1회다. ambiguous와 attempts 2는 invoke 0회와 raw/pending 보존을 증명한다.

## 3. Handler seam과 inbound mapping

- `internal/core/issueops/execution_api.go`: handler type, unavailable error, dependency slot
- `internal/adapter/inbound/issueopslease/reconcile.go`
- core/inbound seam tests

nil service/handler를 fail-closed하고 actor, CWD, request-scoped reader, 공개 result/error를 exact mapping한다.

## 4. Outbound ports와 core compatibility bridge

- `internal/adapter/outbound/issueopslease/reconcile_repository.go`
- `internal/adapter/outbound/issueopslease/reconcile_orca.go`
- `internal/core/issueops/execution_orca_intent.go`: narrow purpose-bound wrappers
- 대응 repository/adapter/CAS tests

outbound는 core를 import하지 않고 `ReconcileEffects`만 의존한다. issueopsapp의 `coreReconcileEffects`가 canonicalize/read/request/mark-invoking/record-failure/apply-receipt wrapper를 호출한다.

## 5. Kind-local router와 production composition

- `internal/core/issueops/execution_api.go`
- `cmd/issueops/issueopsapp/issueops_reconcile_wiring.go`
- CLI/MCP dependency plumbing과 behavioral tests

세 Orca kind confirm은 handler 1회, remote PR/preview/no-pending/unsupported는 0회다. `worktree_create` receipt만 request-scoped reader를 사용한다. 새 wiring은 legacy orchestration helper를 호출하지 않는다.

## 6. Differential과 architecture ratchet

같은 fixture에서 JSON result, structured error, exact text, record raw, external-intent raw를 비교한다. attempts 0/1/2와 여섯 stage를 포함한다. architecture test는 domain/application/inbound/outbound의 core·adapter·SQLite·Orca concrete import와 새 wiring의 legacy helper 호출을 거부한다.

`reconcileOrcaExecutionIntent` production caller가 0이면 제거한다. `executeOrcaIntentStage`와 raw CAS/stage/remote PR shared primitives는 보존한다.

## 7. 문서, CI, verification

필요한 경우 `.issueops/ARCHITECTURE.md`, `CONVENTIONS.md`, `OPERATIONS.md`, `TESTING.md`를 최소 갱신한다. `.github/workflows/ci.yml`은 remote full test와 `go test -race ./... -count=1`을 실행한다.

```bash
go test ./internal/contract/issueopslease ./internal/domain/issueopslease ./internal/application/issueopslease ./internal/adapter/inbound/issueopslease ./internal/adapter/outbound/issueopslease -run Reconcile -count=1
go test ./internal/core/issueops -run 'Reconcile|OrcaIntent|IssueSnapshot' -count=1
go test ./cmd/issueops/issueopscli ./cmd/issueops/issueopscli/executioncmd ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp -run 'Reconcile|ExecutionHandler|IssueSnapshot|ResponseContractsGolden' -count=1
go test ./internal/architecture -run Dependency -count=1
go test -race ./internal/contract/issueopslease ./internal/domain/issueopslease ./internal/application/issueopslease ./internal/adapter/inbound/issueopslease ./internal/adapter/outbound/issueopslease ./internal/core/issueops ./cmd/issueops/issueopscli ./cmd/issueops/issueopscli/executioncmd ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp -run 'Reconcile|ExecutionHandler|IssueSnapshot' -count=1
go vet ./internal/contract/issueopslease ./internal/domain/issueopslease ./internal/application/issueopslease ./internal/adapter/inbound/issueopslease ./internal/adapter/outbound/issueopslease ./internal/core/issueops ./cmd/issueops/issueopscli ./cmd/issueops/issueopscli/executioncmd ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
go build -o bin/issueops ./cmd/issueops
./bin/issueops contract check --json
git diff --check
```

명령, exit code, 핵심 output과 원격 CI run URL을 `.issueops/verified-execution/issue194-report.md`에 기록한다. 독립 implementation review 후 atomic commit/push와 parent-base draft PR을 만든다.
