# Agent-Harness 성능/안정성 개선 계획

## TL;DR
> **Summary**: 현재 daemon은 reachable이고 `go test ./...` 및 `cmd/harness/mcpcli` 단독 테스트는 통과하지만, self-verify quick은 daemon-backed MCP smoke에서 실패한다. 개선은 "MCP smoke transport 재현/수정 -> 로그 노이즈 축소 -> 스킬 컨텍스트 비용 축소 -> 검증/기준선 기록" 순서로 진행한다.
> **Deliverables**:
> - self-verify quick 실패 재현 여부와 원인 분류
> - `harness://commit-policy` resource read의 cwd/root 의존성 회귀 테스트
> - benign MCP stream close 로그의 noisy error 축소
> - pioneer skills 크기/로딩 비용 기준선과 `codd` 분할안
> - 최종 self-verify/go-test/daemon smoke evidence
> **Effort**: Medium
> **Parallel**: YES - 2 waves
> **Critical Path**: T1 MCP smoke transport 재현 -> T2 daemon-backed MCP multi-request 처리 수정 -> T3 daemon log classification -> T5 final verification

## Context

### Original Request

사용자는 Reasonix MCP/daemon/skill 전수 검증 뒤 "성능은?"이라고 물었고, 이어 "분석 후 개선 계획 상세히 수립"을 요청했다.

### Confirmed Evidence

- `git status --short --branch` 결과: `## main...origin/main`; 현재 working tree는 이 계획 파일 작성 전 clean 상태였다.
- `./bin/agent-harness daemon status --json` 결과: daemon reachable, 현재 PID `95419`, socket `/Users/habin/.local/state/agent-harness/daemon/agent-harness.sock`.
- `go test ./cmd/harness/mcpcli -count=1` 결과: 통과, `ok agent-harness/cmd/harness/mcpcli 1.109s`.
- `./bin/agent-harness self-verify --seed=100 --target-score=95 --json` 결과: 실패. `go test ./... -count=1`은 self-verify 내부에서 통과했고 69,818ms가 걸렸지만, `MCP smoke`가 `expected 12 MCP responses, got 1`로 실패했다. self-verify의 `MCP smoke` duration은 84ms라서 느린 병목이 아니라 response contract/transport 문제다.
- 같은 self-verify 결과에서 slowest steps는 `go test` 69,818ms, `go build` 1,336ms, `binary drift` 398ms, `harness invariants` 115ms, `MCP smoke` 84ms였다.
- daemon log tail에는 `mcp stream error: close ... use of closed network connection`가 반복된다. 이 로그는 `cmd/harness/daemoncli/daemon_server.go`의 `serveMCPStream` error logging path에서 발생한다.
- pioneer skill `SKILL.md` 총량은 215,173 bytes다. 최대 파일은 `skills/codd/SKILL.md` 56,298 bytes이고, 다음은 `karpathy` 24,333, `dijkstra` 24,059, `turing` 22,584, `von-neumann` 20,845 bytes다.
- CodeGraph 기준 `ReadProjectDoc`는 `cmd/harness/mcpcli/mcp_tool_project.go` 호출은 있으나 covering test가 부족하다고 보고됐다. resource read 테스트는 `cmd/harness/mcpcli/mcp_tool_project_test.go`에 있지만 현재는 default `HarnessRoot()`에 기대는 형태다.

### Gap Analysis

- 붙여준 이전 self-verify 실패(`cmd/harness/mcpcli/.agent-harness/COMMIT_POLICY.md` missing)는 현재 `go test ./cmd/harness/mcpcli`와 self-verify 내부 `go test ./...`에서 재현되지 않는다. 현재 재현되는 실패는 `cmd/harness/validationcli/mcpsmoke`의 daemon-backed MCP smoke가 12개 요청 중 1개 응답만 받는 문제다.
- `cmd/harness/mcpcli`의 direct stream tests는 newline-delimited 요청을 통과하지만 self-verify는 `binary mcp` daemon proxy 경로를 사용한다. 따라서 수정 위치는 먼저 `cmd/harness/daemoncli` proxy/server 또는 SDK stream handling 경계에서 찾아야 한다.
- daemon stream close 로그는 기능 실패가 아닐 수 있지만, 운영 로그에서 error처럼 보이는 노이즈다. "숨김"이 아니라 graceful client close와 실제 stream failure를 구분해야 한다.
- skill 크기 최적화는 runtime latency보다 LLM context 비용 문제다. core 기능으로 `run_skill` lazy-loading을 재구현하면 `LLM Wiki/CodeGraph/companion tool 재구현 금지`와 같은 철학을 어길 수 있으므로, 먼저 skill 문서 구조 축소와 측정 기준선으로 제한한다.
- self-verify의 72초대 시간은 Go test suite 실행 비용으로 확인됐다. harness overhead 최적화라고 주장하려면 step duration JSON/evidence를 먼저 확보해야 한다.

## Work Objectives

### Core Objective

agent-harness의 현재 성능 병목과 운영 노이즈를 근거 기반으로 분류하고, 실제 결함은 재발 방지 테스트와 함께 수정하며, 컨텍스트 비용은 source-of-truth drift 없이 축소한다.

### Definition of Done

- `./bin/agent-harness self-verify --seed=100 --target-score=95 --json` 결과가 통과하거나, 실패 시 `MCP smoke` failure class와 재현 명령이 문서화된다.
- `go test ./cmd/harness/mcpcli -count=1`와 관련 targeted test가 통과한다.
- daemon stream close 처리 변경 시 `go test ./cmd/harness/daemoncli -count=1`가 통과한다.
- skill 구조 변경 시 `python3 scripts/validate-skill.py skills/<changed-skill>`가 통과한다.
- 전체 변경 후 `go test ./... -count=1`, `go build -o bin/agent-harness ./cmd/harness`, `./bin/agent-harness daemon status --json`, `./bin/agent-harness inspect --json`가 통과한다.
- 설치/runtime freshness가 필요한 경우 `./scripts/install-native.sh` 후 daemon PID/inode freshness를 확인한다.

### Must Have

- 현재 재현되지 않는 실패는 먼저 재현 게이트를 둔다.
- core behavior는 Go core/CLI/MCP 공통 경계에 둔다.
- user daemon과 테스트 daemon은 `HARNESS_DAEMON_DIR`로 분리한다.
- skill source of truth는 `skills/<name>/SKILL.md` 유지. host별 복사본 금지.

### Must NOT Have

- MCP/daemon 결함이 아닌 것을 MCP 결함으로 보고하지 않는다.
- 로그 노이즈를 없애려고 실제 stream failure까지 삼키지 않는다.
- `run_skill` 또는 host skill loader를 agent-harness core에 재구현하지 않는다.
- 단일 사용처를 위해 큰 abstraction을 만들지 않는다.

## Verification Strategy

- Test decision: tests-after, because current evidence is mixed and first task is reproduction/classification.
- QA policy: every implementation task includes a happy path and failure/noise path.
- Evidence location: `.agent-harness/evidence/performance-improvement/`.

## Execution Strategy

### Parallel Execution Waves

Wave 1:
- T1 self-verify MCP smoke transport reproduction and root-cause classification
- T3 daemon stream-close classification
- T4 skill size/context-cost baseline

Wave 2:
- T2 daemon-backed MCP multi-request handling fix
- T3 implementation if daemon log classification confirms benign close can be safely downgraded
- T4 codd split or compression only after baseline and validation criteria are fixed
- T5 final verification

### Dependency Matrix

| Task | Depends On | Blocks | Can Parallelize With |
|------|------------|--------|----------------------|
| T1 | - | T2, T5 | T3, T4 |
| T2 | T1 | T5 | T3, T4 |
| T3 | - | T5 | T1, T4 |
| T4 | - | T5 | T1, T3 |
| T5 | T1, T3, T4; T2 if needed | - | - |

## TODOs

- [ ] 1. Reproduce and classify daemon-backed MCP smoke failure

  **What to do**: Reproduce the current self-verify failure using `cmd/harness/validationcli/mcpsmoke.MCPSmokeInput()` against `./bin/agent-harness mcp` with isolated `HARNESS_STATE_DIR` and `HARNESS_DAEMON_DIR`. Compare three paths: direct in-process `mcpcli.ServeMCPStream`, `HARNESS_MCP_DIRECT=1 ./bin/agent-harness mcp`, and default daemon-backed `./bin/agent-harness mcp`. Capture whether the failure is in SDK stream handling, daemon proxy forwarding, or self-verify command runner stdin lifecycle.
  **Must NOT do**: Do not add `.agent-harness/COMMIT_POLICY.md` under `cmd/harness/mcpcli`; that older failure is not current. Do not weaken the 12-response smoke contract to pass.

  **Recommended Agent**: deep
    Reason: This crosses self-verify orchestration, MCP resource reading, cwd, env, and tests.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T2, T5 | Blocked By: -

  **References**:
  - `cmd/harness/mcpcli/dependencies.go` - `HarnessRoot()` walks upward for `go.mod` and `skills`.
  - `cmd/harness/mcpcli/resources/resources.go` - `harness://commit-policy` maps to `.agent-harness/COMMIT_POLICY.md`.
  - `cmd/harness/validationcli/mcpsmoke/validation_mcp_contract.go` - 12-request MCP smoke input and expected markers.
  - `cmd/harness/validationcli/mcpsmoke/validation_mcp.go` - self-verify invokes `binary mcp` with isolated state/daemon dirs.
  - `cmd/harness/mcpcli/mcp_stream_test.go` and `mcp_transport_test.go` - direct stream coverage that currently passes.
  - `cmd/harness/daemoncli/daemon_server.go` - daemon invokes `ServeMCPStream(conn, conn, logFile)`.
  - `.agent-harness/operations/verification.md` - quick/full self-verify commands.

  **Acceptance Criteria**:
  - [ ] Current `./bin/agent-harness self-verify --seed=100 --target-score=95 --json` result captured and shows `MCP smoke` classification.
  - [ ] `go test ./cmd/harness/mcpcli -count=1` result captured.
  - [ ] Exact failing command/env is documented with response count and stdout sample.
  - [ ] Direct vs daemon-backed behavior is compared so T2 has a single owner path.

  **QA Scenarios**:

  ```
  Scenario: Current quick gate
    Channel: bash
    Steps: ./bin/agent-harness self-verify --seed=100 --target-score=95 --json
    Expected: exit non-zero until fixed, failed_step is MCP smoke, error is expected 12 MCP responses got 1
    Evidence: .agent-harness/evidence/performance-improvement/task-1-self-verify.json

  Scenario: Transport comparison
    Channel: bash
    Steps: feed MCPSmokeInput to direct and daemon-backed MCP modes with isolated HARNESS_STATE_DIR/HARNESS_DAEMON_DIR
    Expected: direct path returns 12 responses; failing path is identified by response count and stderr/log sample
    Evidence: .agent-harness/evidence/performance-improvement/task-1-transport-compare.txt
  ```

  **Commit**: NO

- [ ] 2. Fix daemon-backed MCP multi-request smoke handling

  **What to do**: Add a regression test that exercises the same path self-verify uses: newline-delimited 12-request `MCPSmokeInput()` through `binary mcp`/daemon proxy, or a daemon loop test with a fake conn if that is the smallest deterministic surface. Fix the owner path from T1. Likely candidates are stdin copy lifecycle in the proxy, SDK transport request loop behavior when input closes, or command-runner interaction with daemon startup. Keep the direct `ServeMCPStream` tests intact.
  **Must NOT do**: Do not reduce `MCPSmokeInput()` coverage, do not accept only initialize as success, and do not bypass daemon-backed mode in self-verify unless an ADR explicitly changes the contract.

  **Recommended Agent**: deep
    Reason: A small code change can affect MCP resources, stream transport, and golden contracts.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T5 | Blocked By: T1

  **References**:
  - `cmd/harness/validationcli/mcpsmoke/validation_mcp.go` - failing self-verify path.
  - `cmd/harness/validationcli/mcpsmoke/validation_mcp_contract.go` - expected 12 responses and markers.
  - `cmd/harness/daemoncli/daemon_proxy.go` - stdio proxy to daemon socket.
  - `cmd/harness/daemoncli/daemon_server.go` - daemon-side stream handling.
  - `cmd/harness/mcpcli/mcp_transport.go` - direct legacy vs SDK transport selection.
  - `cmd/harness/harnessapp/mcp_facade.go` - configured app-level `ReadHarnessFile` binding.

  **Acceptance Criteria**:
  - [ ] A regression test fails before the fix and passes after it.
  - [ ] `./bin/agent-harness self-verify --seed=100 --target-score=95 --json` no longer fails at `MCP smoke`.
  - [ ] `go test ./cmd/harness/mcpcli ./cmd/harness/daemoncli ./cmd/harness/validationcli -count=1` passes.
  - [ ] `go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1` passes unless an intentional schema change is made.

  **QA Scenarios**:

  ```
  Scenario: Daemon-backed smoke returns all responses
    Channel: bash
    Steps: feed MCPSmokeInput to ./bin/agent-harness mcp with isolated HARNESS_STATE_DIR/HARNESS_DAEMON_DIR
    Expected: exactly 12 JSON-RPC result lines, including tools/list and harness://project-doc-upkeep
    Evidence: .agent-harness/evidence/performance-improvement/task-2-daemon-mcp.txt

  Scenario: Direct stream behavior remains compatible
    Channel: bash
    Steps: go test ./cmd/harness/mcpcli -run 'TestServeMCPStreamListsHarnessTools|TestMCPTransportCoversInitAndToolsWithSDK' -count=1
    Expected: direct stream tests still pass
    Evidence: .agent-harness/evidence/performance-improvement/task-2-direct-stream.txt
  ```

  **Commit**: YES | Message: `fix(mcp): preserve daemon smoke request stream` | Files: `cmd/harness/daemoncli/*`, `cmd/harness/mcpcli/*`, `cmd/harness/validationcli/*` as needed

- [ ] 3. Reduce benign daemon MCP stream close log noise

  **What to do**: In `runDaemonAcceptLoop`, classify errors from `serveMCPStream`. Treat `net.ErrClosed`, `io.EOF`, and strings containing `use of closed network connection` or a verified equivalent SDK close message as benign client disconnects. Either do not log them or log them with a non-error label such as `mcp stream closed`. Keep real stream failures logged as `mcp stream error`.
  **Must NOT do**: Do not suppress all stream errors. Do not change max connection behavior or graceful shutdown semantics.

  **Recommended Agent**: quick
    Reason: The code path is localized in daemon server loop tests and implementation.

  **Parallelization**: Can Parallel: YES | Wave 1/2 | Blocks: T5 | Blocked By: -

  **References**:
  - `cmd/harness/daemoncli/daemon_server.go` - current `mcp stream error` logging path.
  - `cmd/harness/daemoncli/daemon_server_loop_test.go` - existing tests for stream error logging, connection limit, and graceful shutdown.
  - `.agent-harness/CAUTIONS.md` - daemon lifecycle drift and MCP tool-use risks.

  **Acceptance Criteria**:
  - [ ] Benign close no longer appears as `mcp stream error`.
  - [ ] Real `stream failed` still appears as `mcp stream error`.
  - [ ] `go test ./cmd/harness/daemoncli -count=1` passes.

  **QA Scenarios**:

  ```
  Scenario: Real stream failure remains visible
    Channel: bash
    Steps: go test ./cmd/harness/daemoncli -run TestRunDaemonAcceptLoopLogsAcceptAndStreamErrors -count=1
    Expected: log contains temporary accept failure and mcp stream error for synthetic stream failed
    Evidence: .agent-harness/evidence/performance-improvement/task-3-real-error.txt

  Scenario: Benign close is downgraded
    Channel: bash
    Steps: add/run targeted test with serveMCPStream returning net.ErrClosed or use-of-closed-network-connection
    Expected: log does not contain mcp stream error for benign close
    Evidence: .agent-harness/evidence/performance-improvement/task-3-benign-close.txt
  ```

  **Commit**: YES | Message: `fix(daemon): downgrade benign mcp stream closes` | Files: `cmd/harness/daemoncli/*`

- [ ] 4. Shrink pioneer skill context cost without host drift

  **What to do**: Establish byte/token baseline for the 9 pioneer skills. For `codd`, move long examples/checklists that are not first-turn operating rules into `references/` only if the skill body explicitly points to when to read them. Keep `SKILL.md` self-contained enough for safe activation. Validate changed skills with `quick_validate.py`. Do not implement lazy loading in agent-harness core.
  **Must NOT do**: Do not split source of truth across Codex/Claude copies. Do not remove safety rules or IssueOps integration. Do not add README/changelog files inside skill folders.

  **Recommended Agent**: deep
    Reason: This is documentation architecture with safety and host integration constraints.

  **Parallelization**: Can Parallel: YES | Wave 1/2 | Blocks: T5 | Blocked By: -

  **References**:
  - `skills/codd/SKILL.md` - largest current skill, 56,298 bytes.
  - `skills/*/SKILL.md` - pioneer skill set totals 215,173 bytes.
  - `.agent-harness/CONVENTIONS.md` - shared skill conventions and no host-specific copies.
  - `.agent-harness/CAUTIONS.md` - shared skill drift and companion tool reimplementation warnings.

  **Acceptance Criteria**:
  - [ ] Baseline and after byte counts are recorded.
  - [ ] `codd` `SKILL.md` shrinks only if references preserve the required detail.
  - [ ] `quick_validate.py skills/codd` passes for any changed skill.
  - [ ] User-level symlink targets remain pointed at repo `skills/<name>` if install refresh is performed.

  **QA Scenarios**:

  ```
  Scenario: Skill byte baseline
    Channel: bash
    Steps: wc -c skills/{berners-lee,codd,dijkstra,hopper,karpathy,shannon,torvalds,turing,von-neumann}/SKILL.md
    Expected: report includes total bytes and identifies codd as the largest
    Evidence: .agent-harness/evidence/performance-improvement/task-4-byte-baseline.txt

  Scenario: Changed skill validates
    Channel: bash
    Steps: python3 scripts/validate-skill.py skills/codd
    Expected: validation exits 0
    Evidence: .agent-harness/evidence/performance-improvement/task-4-codd-validate.txt
  ```

  **Commit**: YES | Message: `docs(skills): reduce pioneer skill context cost` | Files: `skills/codd/**` and any explicitly changed pioneer skill references

- [ ] 5. Final performance and stability verification

  **What to do**: Run targeted tests first, then repo-level verification. Capture daemon status, inspect/docs smoke, self-verify quick, and if code changed, full Go test/build. If install/runtime behavior changed, rebuild/install and verify daemon freshness by PID/inode.
  **Must NOT do**: Do not declare performance improvement from subjective latency. Use command duration, byte counts, log count, or self-verify step stats.

  **Recommended Agent**: deep
    Reason: Final verification crosses tests, daemon runtime, MCP contract, and skill install state.

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: - | Blocked By: T1, T3, T4, and T2 if needed

  **References**:
  - `.agent-harness/TESTING.md` - Go and self-verify gates.
  - `.agent-harness/operations/verification.md` - quick/full verification commands.
  - `.agent-harness/CAUTIONS.md` - daemon freshness and stale proxy guidance.

  **Acceptance Criteria**:
  - [ ] `go test ./... -count=1` passes.
  - [ ] `go build -o bin/agent-harness ./cmd/harness` passes.
  - [ ] `./bin/agent-harness self-verify --seed=100 --target-score=95 --json` passes or has a classified non-regression failure.
  - [ ] `./bin/agent-harness daemon status --json` reports reachable daemon.
  - [ ] If daemon logging changed, post-change log sample shows benign close no longer emitted as error.

  **QA Scenarios**:

  ```
  Scenario: Full Go verification
    Channel: bash
    Steps: go test ./... -count=1 && go build -o bin/agent-harness ./cmd/harness
    Expected: both commands exit 0
    Evidence: .agent-harness/evidence/performance-improvement/task-5-go.txt

  Scenario: Runtime smoke
    Channel: bash
    Steps: ./bin/agent-harness daemon status --json && ./bin/agent-harness inspect --json && ./bin/agent-harness docs --json
    Expected: daemon reachable and inspect/docs return ok JSON
    Evidence: .agent-harness/evidence/performance-improvement/task-5-runtime.json
  ```

  **Commit**: NO

## Final Verification Wave

- [ ] F1. Plan Compliance Audit - every required task either completed or explicitly skipped with evidence.
- [ ] F2. Code Quality Review - no broad abstraction, no host-specific policy duplication, no hidden cwd assumptions.
- [ ] F3. Performance Evidence Review - compare before/after log noise count, skill byte count, and self-verify duration.
- [ ] F4. Scope Fidelity Check - no unrelated docs, generated files, install config, or repo-local host files changed.

## Commit Strategy

Use separate commits by concern:

1. `fix(mcp): preserve daemon smoke request stream` for the current self-verify MCP smoke failure.
2. `fix(daemon): downgrade benign mcp stream closes` for daemon logging.
3. `docs(skills): reduce pioneer skill context cost` for skill restructuring.

Do not combine code and skill-documentation changes unless the implementation requires a single contract update, which this plan currently does not.

## Success Criteria

- The self-verify failure report is no longer ambiguous: current `MCP smoke` 12-response failure is fixed with regression coverage, and the older `COMMIT_POLICY.md` missing report is classified as stale unless reproduced separately.
- Daemon logs distinguish real stream failures from normal client disconnects.
- Pioneer skill context cost has a measured baseline and, if edited, a lower `SKILL.md` first-load footprint without losing required safety rules.
- Final answer includes commands run, pass/fail status, and any residual risk.
