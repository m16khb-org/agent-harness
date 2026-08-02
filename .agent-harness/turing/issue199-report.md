# Issue #199 Turing 검증 보고서

- 이슈: https://github.com/m16khb/agent-harness/issues/199
- lifecycle: `io-ab4d4c69d7e5`, direct generation 1
- 대상 브랜치: `117-hexagonal-architecture-migration`
- source 브랜치: `199-issueops-execution-prepare-vertical`
- sealed base: `739de96aeca540cf7d5cf6333b345a192afcfd59`
- 구현·초기 원격 CI 검증 HEAD: `7f2907f581da2fb716f22e3b1f6b75dd34c58a08`
- draft PR: [#218](https://github.com/m16khb/agent-harness/pull/218)
- 초기 원격 CI: [run 30722218325](https://github.com/m16khb/agent-harness/actions/runs/30722218325) 성공

## 결과

IssueOps execution의 `prepare` action을
`contract/domain/application/inbound/outbound` 수직 경계로 이전했다.
`cmd/harness/harnessapp`만 Git worktree, Orca, provider issue snapshot과
SQLite repository를 조립하고 CLI와 MCP/daemon-backed MCP는 같은
request-scoped handler를 사용한다. Handler가 없으면 legacy fallback 없이
fail-closed한다.

Direct는 canonical workspace provision과 artifact materialization 뒤 schema-v1
record와 generation-1 active lease·holder reverse index를 한 transaction 의미로
기록한다. Orca는 외부 mutation 전에 durable intent를 저장하고 각 external
inspect/invoke를 SQLite span 밖에서 실행한 뒤 raw CAS receipt로 stage를 전진한다.
기존 prepare/resume/reconcile이 공유하던 marker와 intent bytes는 단일 codec으로
이전했고 public DTO, usage, error text, next command와 persisted schema는 바꾸지
않았다.

## Acceptance evidence

| acceptance | 결과 | 핵심 증거 |
|---|---|---|
| AC-199-01 | PASS | legacy/new differential이 public result JSON, error, text projection, next command와 schema-v1 record/index/intent raw bytes를 비교했고 contract/response golden과 CI가 통과했다. |
| AC-199-02 | PASS | `issueopspreparation` contract가 transport-neutral command/result와 prepare/resume 공용 intent codec을 소유하고 `issueopslease` stable projection을 재사용한다. Architecture ratchet이 production core/model의 두 번째 schema source를 거부한다. |
| AC-199-03 | PASS | pure domain decision matrix가 mode 정규화, existing pending/idempotent/mismatch/writerless, preview/apply, root conflict와 auto fallback을 고정한다. |
| AC-199-04 | PASS | application은 consumer-owned `Repository`, `Clock`, `OperationIDs`, `DirectWorkspace`, `OrcaGateway`, `PreparationEvidence`만 의존하며 concrete adapter를 import하지 않는다. |
| AC-199-05 | PASS | direct differential이 actor/process/CWD/base/root 검증, worktree provision, artifact materialization, generation-1 lease와 holder index의 result·bytes·error ordering을 기존 oracle과 동일하게 확인한다. |
| AC-199-06 | PASS | Orca differential과 repository tests가 intent-first, external call outside SQLite lock, fixed stage bound, invoking/receipt CAS, unknown outcome와 reconcile/claim artifact compatibility를 확인한다. |
| AC-199-07 | PASS | preview와 validation/persistence failure 행이 filesystem, record/index, staged artifact, Orca trace 무변경 및 legacy/new bytes·호출 순서 동등성을 확인한다. |
| AC-199-08 | PASS | CLI/MCP production wiring은 harnessapp의 단일 handler를 사용하고 nil handler는 `issueops execution prepare handler is not configured`로 fail-closed한다. Production legacy prepare caller와 fallback slot은 0이다. |
| AC-199-09 | PASS | focused hook wiring tests와 실제 hook command smoke가 Codex/Claude exact holder, source CWD, canonical subdir, symlink escape, foreign session, PID start/executable mismatch와 Orca writerless 상태를 같은 generation/root 권한으로 판정한다. |
| AC-199-10 | PASS | direct/Orca/auto/idempotency/root collision/parent worktree/branch/staged artifact/external-intent recovery differential, architecture, golden, vet, build, 전체 unit/race와 self-verify CI가 통과했다. |

## TDD에서 발견한 회귀와 수정

최초 quick self-verify는
`internal/core/issueops/execution_issue_snapshot_test.go`의 4개 테스트에서
`issueops execution prepare handler is not configured`를 검출했다. 조사 결과
단순히 stale test가 아니라, `ExecuteExecution`이 GitLab evidence를 위해 만든
request-scoped `ReadIssue` wrapper가 미리 조립된 preparation service에 도달하지
못해 provider fallback이 사용되는 production 결함이었다.

`TestIssueOpsPrepareWiringUsesRequestScopedIssueSnapshot`을 RED로 추가해
`context deadline exceeded`를 재현했다. 그 뒤
`ExecutionPrepareInvocation{ReadIssue}`를 내부 handler invocation에 추가하고
harnessapp이 요청마다 composition deps를 복사해 reader만 교체하도록 수정했다.
공개 request/result JSON과 persisted schema는 바꾸지 않았다. Focused regression,
affected race, 전체 unit, quick/full self-verify와 원격 전체 CI를 다시 실행했다.

## 활성형 훅 환경 검증

현재 Codex 실행의 `--disable hooks` 상태는 증거로 사용하지 않았다. 별도 temporary
source/worktree/state에 generation-1 active lease를 만든 뒤 빌드된 binary의 실제
`hook pre-tool-use` 엔트리포인트를 설치형 전체 flag와 외부 transcript metadata가
포함된 host payload로 호출했다.

```text
codex-holder: {}
codex-foreign: decision=block, code=holder_identity_mismatch,
  lifecycle_id=io-76b2c0bb0f2c, current_generation=1
claude-holder: {}
claude-foreign: hookSpecificOutput.permissionDecision=deny,
  code=holder_identity_mismatch, lifecycle_id=io-02fd6e3fa5e1,
  current_generation=1
```

`cmd/harness/harnessapp/issueops_preparation_hook_wiring_test.go`는 두 host의
canonical/source/subdirectory allow, symlink escape, foreign session, PID-reuse-safe
started-at/executable, Orca claimable/released/revoking의 exact deny code를 직접
`hookcli.RunHookPreToolUse`로 고정한다. 따라서 하네스가 native hooks를 활성화한
정상 설치 환경에서도 같은 admission contract가 적용된다.

## 정량 품질과 독립 리뷰

- Diff scope: sealed base부터 구현 HEAD까지 60 files, 7,481 insertions,
  666 deletions. Worktree staged/unstaged/untracked는 0이었다.
- Approximate added-Go SNR: `0.9917` = signal 6,231 / total 6,283,
  noise 52.
- AST-backed `golangci-lint dupl`: 0 issues.
- Approximate channel overhead: production Go 29 files 중 added boilerplate
  50% 초과 0.
- `cyclop`은 byte/stage compatibility 경계 17개를 default 10 초과로 표시했다.
  특히 `validateShape` 75, application `Prepare` 25, repository
  `ApplyReceipt` 23이다. 독립 reviewer는 exact-byte differential, contract,
  full-suite coverage 아래 현 extraction 범위의 ceiling으로 WAIVE했고, 구체적
  correctness failure 없이 helper를 늘리는 cosmetic refactor는 하지 않되 이후
  기능 추가 전에 분해하도록 제한했다.
- Fresh `code-reviewer`가
  `739de96a...7f2907f` 전체 diff와 predecessor를 독립 검토했다. Verdict
  `PASS`, P0-P3 finding 0건, AC-199-01부터 AC-199-10까지 모두 PASS였다.

## 검증

다음 검증은 canonical #199 worktree에서 실행했다.

```text
go test ./internal/contract/issueopspreparation ./internal/domain/issueopspreparation ./internal/application/issueopspreparation ./internal/adapter/inbound/issueopspreparation ./internal/adapter/outbound/issueopspreparation -count=1
go test -race ./internal/contract/issueopspreparation ./internal/domain/issueopspreparation ./internal/application/issueopspreparation ./internal/adapter/inbound/issueopspreparation ./internal/adapter/outbound/issueopspreparation -count=1
go test ./internal/core/issueops -run 'ExecutionPrepare|PreparationDifferential|OrcaIntent|IssueSnapshot' -count=1
go test ./cmd/harness/issueopscli ./cmd/harness/issueopscli/executioncmd ./cmd/harness/mcpcli ./cmd/harness/harnessapp -run 'ExecutionPrepare|Prepare|ExecutionHandler|ResponseContract' -count=1
go test ./internal/core/lifecycle ./cmd/harness/hookcli -run 'AtomicPreflight|Canonical|Workdir|OwnerMutation|ExecutionPrepare' -count=1
go test -race ./internal/core/lifecycle ./cmd/harness/hookcli -count=1
go test ./internal/architecture -run 'Dependency|Preparation' -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
go vet ./internal/contract/issueopspreparation/... ./internal/domain/issueopspreparation/... ./internal/application/issueopspreparation/... ./internal/adapter/inbound/issueopspreparation/... ./internal/adapter/outbound/issueopspreparation/... ./internal/core/issueops ./cmd/harness/issueopscli/... ./cmd/harness/mcpcli/... ./cmd/harness/harnessapp/...
go build -o bin/agent-harness ./cmd/harness
git diff --check 739de96aeca540cf7d5cf6333b345a192afcfd59...HEAD
go test ./... -count=1
./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --save-state --state-key self-verify-199-quick --json
  result: 25/25, minimum_goal_score=100, coverage_gaps=[]
./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --save-state --state-key self-verify-199-full --json
  result: 250/250, minimum_goal_score=100, termination_eligible=true,
  coverage_gaps=[]
golangci-lint run --no-config --enable-only dupl --tests=false --new-from-rev=739de96aeca540cf7d5cf6333b345a192afcfd59 <affected packages>
  result: 0 issues
GitHub Actions run 30722218325 at 7f2907f581da2fb716f22e3b1f6b75dd34c58a08
  result: format/vet/build/full test/full race/deterministic self-verify success
```

## 변경 파일

<details>
<summary>구현 HEAD의 exact 60 paths</summary>

```text
cmd/harness/harnessapp/issueops_policy_facade.go
cmd/harness/harnessapp/issueops_preparation_hook_wiring_test.go
cmd/harness/harnessapp/issueops_preparation_wiring.go
cmd/harness/harnessapp/issueops_preparation_wiring_test.go
cmd/harness/harnessapp/issueops_reconcile_wiring_test.go
cmd/harness/harnessapp/mcp_facade.go
cmd/harness/issueopscli/executioncmd/execution.go
cmd/harness/issueopscli/executioncmd/snapshot_file_test.go
cmd/harness/issueopscli/exports.go
cmd/harness/issueopscli/issueops_execution_cli.go
cmd/harness/issueopscli/issueops_execution_cli_test.go
cmd/harness/mcpcli/mcp_tool_issueops_execution.go
cmd/harness/mcpcli/mcp_tool_issueops_execution_test.go
cmd/harness/mcpcli/mcp_tools.go
docs/superpowers/plans/2026-08-02-issueops-execution-prepare-vertical.md
docs/superpowers/specs/2026-08-02-issueops-execution-prepare-vertical-design.md
internal/adapter/inbound/issueopspreparation/prepare.go
internal/adapter/inbound/issueopspreparation/prepare_test.go
internal/adapter/outbound/issueopspreparation/evidence.go
internal/adapter/outbound/issueopspreparation/evidence_test.go
internal/adapter/outbound/issueopspreparation/orca.go
internal/adapter/outbound/issueopspreparation/orca_test.go
internal/adapter/outbound/issueopspreparation/repository.go
internal/adapter/outbound/issueopspreparation/repository_orca_test.go
internal/adapter/outbound/issueopspreparation/repository_test.go
internal/adapter/outbound/issueopspreparation/workspace.go
internal/adapter/outbound/issueopspreparation/workspace_test.go
internal/application/issueopspreparation/orca_test.go
internal/application/issueopspreparation/ports.go
internal/application/issueopspreparation/prepare.go
internal/application/issueopspreparation/prepare_test.go
internal/architecture/dependency_test.go
internal/architecture/testdata/legacy_imports.txt
internal/contract/issueopslease/stable_v1.go
internal/contract/issueopspreparation/intent.go
internal/contract/issueopspreparation/intent_test.go
internal/contract/issueopspreparation/prepare.go
internal/contract/issueopspreparation/prepare_test.go
internal/core/issueops/execution_api.go
internal/core/issueops/execution_contract_test.go
internal/core/issueops/execution_issue_snapshot_test.go
internal/core/issueops/execution_orca_intent.go
internal/core/issueops/execution_orca_intent_test.go
internal/core/issueops/execution_orca_marker.go
internal/core/issueops/execution_orca_marker_test.go
internal/core/issueops/execution_prepare.go
internal/core/issueops/execution_prepare_bridge.go
internal/core/issueops/execution_prepare_intent_codec_spike_test.go
internal/core/issueops/execution_prepare_legacy_oracle_test.go
internal/core/issueops/execution_prepare_vertical_differential_test.go
internal/core/issueops/execution_reconcile_bridge.go
internal/core/issueops/execution_resume.go
internal/core/issueops/execution_resume_bridge.go
internal/core/issueops/issueops_cleanup_abandon.go
internal/core/issueops/issueops_cleanup_abandon_test.go
internal/core/sqlstore/sqlstore.go
internal/domain/issueopspreparation/decision.go
internal/domain/issueopspreparation/decision_test.go
internal/port/orca_defaults_test.go
internal/port/transactional_record_store.go
```

</details>

## 커밋과 범위

구현은 계획/handler seam/codec/domain/direct/Orca/differential/cutover/hook/fix의
11개 원자 커밋으로 구성했다. 단일 커밋으로 squash하지 않은 이유는 각 TDD
checkpoint와 architecture cutover, self-verify에서 발견한 request-scoped evidence
회귀 수정을 독립 검토·revert 가능하게 보존하기 위해서다. OpenWiki, public schema,
새 CLI/MCP surface와 비목표 lifecycle은 수정하지 않았다.

## Turing evidence block

Success criteria: AC-199-01부터 AC-199-10까지 각각 위 표의 관찰 조건으로 PASS.

Evidence artifact: `.agent-harness/turing/issue199-report.md`, PR #218,
GitHub Actions run 30722218325, self-verify checkpoints
`self-verify-199-quick.json`과 `self-verify-199-full.json`.

Cleanup receipt: hook smoke source/worktrees/SQLite state는
`/Users/m16khb/.Trash/agent-harness-hook-smoke.qLi626`로, reviewer가 만든
비추적 binary는 `/Users/m16khb/.Trash/issue199-review-harness`로, PR body
runtime draft는 `/Users/m16khb/.Trash/issue199-pr-body.md`로 이동해 canonical
worktree와 active harness state에서 제거했다. 모두 recoverable이며 원래 경로
부재를 확인했다.

Verification mode: high-risk full loop. Focused/race/architecture/golden/vet/build,
실제 Codex/Claude hook command, local deterministic self-verify 10회와 GitHub
전체 unit/race/self-verify를 실행했다.

Skipped checks: 필수 check는 없음. Optional LSP/ast-grep tool은 reviewer host에
노출되지 않아 go vet, architecture ratchet, rg와 full test로 대체했다.

## Completion hygiene

- final implementation diff: 60 files, +7,481/-666.
- remote body는 구현 HEAD와 일치하고 label `enhancement`, assignee
  `m16khb`, target `117-hexagonal-architecture-migration`을 IssueOps
  readback으로 검증했다.
- PR review thread는 아직 0건이며 독립 local implementation review는 PASS다.
- report-only commit push 뒤 최종 PR CI를 다시 통과시킨 후 generation 1 completion
  receipt를 기록한다.
- child PR merge 뒤 #199만 close/cleanup하고 parent #117 branch/worktree와 issue는
  유지한다. 다음 순차 child는 #200이다.
