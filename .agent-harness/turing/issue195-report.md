# Issue #195 Turing 검증 보고서

- 이슈: https://github.com/m16khb/agent-harness/issues/195
- 대상 브랜치: `117-hexagonal-architecture-migration`
- sealed base: `667e5d15b0773e2550cfbf5bc2780506e9eb2896`
- 구현·원격 CI 검증 기준 HEAD: `b0ff5abe5e01b9a0cd34f4807a0979898d512125`
- PR: [#216](https://github.com/m16khb/agent-harness/pull/216)
- 원격 CI: [push run 30709143814](https://github.com/m16khb/agent-harness/actions/runs/30709143814), [pull_request run 30709170374](https://github.com/m16khb/agent-harness/actions/runs/30709170374) 모두 성공

## 결과

원격 PR/MR 최초 생성과 `remote_pr_create` 복구를 하나의
`issueopspublication` contract/domain/application/inbound/outbound vertical로
이전했다. `cmd/harness/harnessapp`만 provider, raw CAS bridge, live verifier를
조립하고, CLI create와 CLI/MCP reconcile은 같은 request-scoped handler pair를
사용한다. Production의 legacy full-flow create/reconcile 함수와 fallback dependency
slot은 제거했으며 schema v1 payload와 공개 command/MCP response contract는 유지했다.

| acceptance | 결과 | 핵심 증거 |
|---|---|---|
| AC-195-01 | PASS | 6개 create와 10개 reconcile legacy/new differential이 result JSON, error text·typed invocation field, record row, `external_intent_v1` row를 동일하게 확인했고 CLI text, MCP `isError`, 두 golden, contract hash가 통과했다. |
| AC-195-02 | PASS | 기존 byte-preserving core bridge만 persistence를 수행하고 application provider 호출은 intent/retry CAS 뒤 lock 밖에서 실행된다. Focused core race/동시 read·replacement-preview 회귀가 통과했다. |
| AC-195-03 | PASS | exact candidate, mismatch, multiple, authoritative/non-authoritative zero, known URL verification failure, bounded retry 성공·terminal-not-invoked·ambiguous·exhausted 차등 행이 기존 결과와 동일하다. |
| AC-195-04 | PASS | AST ratchet이 publication caller의 concrete `provider.Resolve`와 non-test core의 legacy full-flow symbol을 거부한다. Public create/reconcile은 nil handler에서 fail closed하고 production fallback field가 없다. |
| AC-195-05 | PASS | focused unit/race, provider, CLI/MCP, architecture, golden, scoped vet, build, contract check, diff check가 모두 exit 0이고, push·pull_request CI에서 전체 test·race와 deterministic self-verify gate가 통과했다. |

공개 contract hash는
`bef9b8eeb380337c6b4a2e5431e1c2bc08c4a5ccceee63260dff66d934390474`다.

## TDD에서 확인한 호환성 경계

최종 legacy/new differential RED는 reconcile 실패 뒤 두 상태를 구분했다.
Durable record에는 최신 failure receipt가 저장되지만 기존 공개
`ExecutionReconcileResult`는 실패 시 호출 직전 record를 투영한다. 새 application
service의 `Latest` 의미는 유지하고, core가 pre-call snapshot을 inbound handler에
전달해 public projection에서만 기존 의미를 보존했다. 또한 terminal-not-invoked는
error를 반환하면서 `Reconciled=true`, `OK=true`인 기존 계약을 유지한다.

그 뒤 동일 fixture에서 16개 create/reconcile 행이 결과와 raw rows까지 같아졌고,
legacy orchestration을 frozen `_test.go` oracle로 이동한 뒤 architecture ratchet이
production definition/call 0개를 확인했다.

## 로컬 최종 gate

모든 명령은 canonical #195 worktree에서 실행했고 exit code는 0이다. 괄호 안은
UTC 시작 시각과 확인된 package 수 또는 핵심 결과다.

```text
go test ./internal/contract/issueopspublication ./internal/domain/issueopspublication ./internal/application/issueopspublication ./internal/adapter/inbound/issueopspublication ./internal/adapter/outbound/issueopspublication -count=1
  (2026-08-01T16:24:08Z, 5 packages ok)
go test -race ./internal/contract/issueopspublication ./internal/domain/issueopspublication ./internal/application/issueopspublication ./internal/adapter/inbound/issueopspublication ./internal/adapter/outbound/issueopspublication -count=1
  (2026-08-01T16:24:16Z, 5 packages ok)
go test ./internal/core/issueops -run 'RemotePullRequest|RemoteReconcile|Publication' -count=1
  (2026-08-01T16:24:26Z, 1 package ok)
go test ./internal/adapter/provider/github ./internal/adapter/provider/gitlab -run 'CreatePullRequest|ReconcilePullRequest' -count=1
  (2026-08-01T16:24:37Z, 2 packages ok)
go test ./cmd/harness/issueopscli ./cmd/harness/issueopscli/executioncmd ./cmd/harness/mcpcli ./cmd/harness/harnessapp -run 'RemotePullRequest|CreatePR|Reconcile|Publication|ExecutionHandler' -count=1
  (2026-08-01T16:24:47Z, 4 packages ok)
go test ./internal/architecture -run Dependency -count=1
  (2026-08-01T16:24:57Z, 1 package ok)
go test ./internal/architecture -run 'Dependency.*Publication|Production.*Publication|PublicationLegacy' -count=1
  (2026-08-01T16:25:02Z, 1 package ok)
go test ./cmd/harness/contractgolden -run Golden -count=1
  (2026-08-01T16:25:06Z, 1 package ok)
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
  (2026-08-01T16:25:13Z, 1 package ok)
go vet ./internal/contract/issueopspublication/... ./internal/domain/issueopspublication/... ./internal/application/issueopspublication/... ./internal/adapter/inbound/issueopspublication/... ./internal/adapter/outbound/issueopspublication/... ./internal/core/issueops ./cmd/harness/issueopscli/... ./cmd/harness/mcpcli/... ./cmd/harness/harnessapp/...
  (2026-08-01T16:25:24Z, no diagnostics)
go build -o bin/agent-harness ./cmd/harness
  (2026-08-01T16:25:29Z)
./bin/agent-harness contract check --json
  (2026-08-01T16:25:34Z, ok:true, warnings:[])
git diff --check
  (2026-08-01T16:25:39Z, no diagnostics)
go test -race ./internal/core/issueops -run 'RemotePublication|RemotePullRequest.*Lock' -count=1
  (2026-08-01T16:28:00Z, 1 package ok)
go test ./internal/core/issueops ./internal/adapter/inbound/issueopspublication ./internal/architecture ./cmd/harness/issueopscli/executioncmd ./cmd/harness/mcpcli -count=1
  (2026-08-01T16:28:14Z, 5 affected packages ok)
go vet ./internal/contract/issueopspublication/... ./internal/domain/issueopspublication/... ./internal/application/issueopspublication/... ./internal/adapter/inbound/issueopspublication/... ./internal/adapter/outbound/issueopspublication/... ./internal/core/issueops ./cmd/harness/issueopscli/... ./cmd/harness/mcpcli/... ./cmd/harness/harnessapp/...
  (2026-08-01T16:32:04Z, final recheck, no diagnostics)
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness contract check --json
  (2026-08-01T16:32:11Z, final recheck, build exit 0, contract ok:true, warnings:[])
git diff --check
  (2026-08-01T16:32:18Z, final recheck, no diagnostics)
go test ./internal/core/lifecycle ./cmd/harness/hookcli -count=1
go test -race ./internal/core/lifecycle ./cmd/harness/hookcli -count=1
go vet ./internal/core/lifecycle ./cmd/harness/hookcli
  (2026-08-01T16:52Z, Codex structured workdir hook regression 포함, 모두 exit 0)
./bin/agent-harness hook pre-tool-use --host codex --enforce-worktree --json
  (2026-08-01T16:49Z, 실제 source cwd + canonical tool_input.workdir + active lease payload decision=allow)
```

## 문서와 범위

- `.agent-harness/ARCHITECTURE.md`에 completed publication vertical과
  harnessapp-only composition/fail-closed routing을 반영했다.
- `.agent-harness/OPERATIONS.md`에 CLI create와 CLI/MCP reconcile의 shared
  publication handler 운영 계약을 반영했다.
- `.agent-harness/TESTING.md`에 raw-byte differential, lock-free external call,
  CLI/MCP/AST ratchet 게이트를 반영했다.
- `.agent-harness/CONVENTIONS.md`를 검토했다. 기존 capability-local package와
  consumer-owned port 규칙이 #195를 이미 포괄하므로 편집하지 않았다.
- OpenWiki 자동 갱신, schema migration, public DTO 확장, unrelated provider
  operation 변경은 하지 않았다.
- 프로젝트 계약에 따라 local full `go test ./...`와 full race는 실행하지 않았다.
  PR #216의 push·pull_request CI가 전체 test, 전체 race, deterministic self-verify를
  모두 통과했으며 위 원격 run URL로 결과를 고정했다.
