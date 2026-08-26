# Unit, integration, fixture, golden, and contract tests

[← TESTING.md](../TESTING.md) owns the test-strategy index and minimum completion
gate. This module owns unit/integration/fixture/golden/contract standards and
the Go-code basic verification that backs them. Race, process, lock, and
nondeterminism procedures live in [concurrency-and-race.md](concurrency-and-race.md);
CLI/MCP/host parity lives in [cli-mcp-and-hosts.md](cli-mcp-and-hosts.md); the
IssueOps execution vertical contract lives in
[issueops-execution.md](issueops-execution.md).

## Go 코드 변경 기본 검증

Go 코드가 추가되면 기본 검증은 다음이다. `self-verify`는 working tree risk를 보고 `risk QA tier`에서 `go vet ./...` 또는 `go test -race ./... -count=1`를 조건부 실행한다.

```bash
gofmt -l $(git ls-files '*.go')
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/agent-harness ./cmd/harness
```

`gofmt -l`은 CI의 Format check와 같은 파일 집합(`git ls-files '*.go'`)을 검사하며 출력이
비어 있어야 통과다. `self-verify`의 `gofmt` 단계는 같은 조건을 working tree 변경 여부와
무관하게 무조건 실행한다(`risk QA tier`는 변경분에만 반응한다). CI는 gofmt에서 끊기면 그
뒤의 test/race/self-verify 결과를 보여주지 않으므로, 로컬 배터리의 게이트 집합은 CI와
같아야 한다(2026-08-26 lesson).

작은 변경은 targeted test를 먼저 실행하고, 완료 전 영향 범위에 맞게 전체 테스트를 실행한다.

### Architecture fitness ratchet

```bash
go test ./internal/architecture -count=1
```

이 test는 `go list -json ./...`의 direct production import inventory를 두 번 수집해 byte-stable 정렬을 확인한다. synthetic case는 rule name과 `importer -> imported` 진단을 고정하고, real graph는 unconditional layer rule과 sorted legacy baseline의 new/stale edge를 함께 검증한다.

### Operational-health and stability delegation

- Pure classifier tests pin the 15-minute heartbeat boundary, invocation-only preserves, duplicate/incomplete inventory failure, and exact resource ownership.
- External-vocabulary enumerations are pinned per axis, and each case cites the upstream definition rather than an observed sample. `knownDispatchStatus`/`settledDispatchStatus`/`knownGateStatus` must accept the full upstream union — including values a local run rarely produces (`circuit_broken`, `timeout`) — so an unobserved value is not classified as unknown (#171). When an upstream union grows, the test changes with the citation.
- Stale-scan integration must prove `operational_dead_owner` is report-only (`needs-review`, `releasable=false`) for missing/stale heartbeat while existing confirmed worktree/remote evidence remains releasable after the locked fresh re-probe.
- Stability audit unit tests must prove it calls the freshly built top-level `doctor`, forwards only a non-empty exact `ORCA_TERMINAL_HANDLE`, requires exit zero plus JSON `ok=true` and `healthy=true`, and stores only bounded issue code/summary failure details.
- Final live reconciliation verification runs `python3 skills/stability-audit/scripts/e2e_stability_audit.py --cleanup-stale --json` only after the external recovery manifest/journal is sealed and cleanup readbacks are complete. Orca snapshot evidence is archival-only; reset ambiguity follows forward recovery, never an inferred rollback.

core 패키지 변경 시 최소 targeted 검증:

```bash
go test ./internal/core -count=1
go test ./cmd/harness -count=1
```

CLI/MCP contract를 의도적으로 바꾼 경우에만 golden 파일을 갱신한다. Codex/Claude native 설치 adapter 계약을 바꾼 경우에는 adapter matrix golden도 함께 갱신한다.

```bash
go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -update -count=1
go test ./internal/adapter -run TestNativeInstallAdapterContractMatrix -update-adapter-contract -count=1
```

## 테스트 작성 기준

### Well-structured tests

- 변경된 공개 동작/계약을 직접 검증하고 구현 세부사항에 과도하게 묶이지 않는다.
- 실패 시 원인을 좁힐 수 있는 fixture 이름, assertion, 에러 메시지를 둔다.
- 저장소 내부 심볼을 찾는 contract는 indexed repo에서 CodeGraph 우선, 비인덱스 repo에서만 `rg`/직접 읽기다. local symbol discovery에 web search를 쓰는 fallback은 허용하지 않는다.
- regression test는 재발했던 입력, false case, 기대 결과를 명확히 담는다.
- 기존 helper와 style을 재사용하고, golden/snapshot 변경은 의도와 범위를 설명한다.

### Poorly-structured tests

- 실제 요구사항과 무관한 내부 구조만 고정한다.
- 통과를 위해 production behavior를 약화하거나 오류 처리를 숨긴다.
- 실패해도 원인을 알 수 없는 거대한 fixture나 broad snapshot만 둔다.
- 테스트 이름이 “works” 수준이라 어떤 계약을 지키는지 알 수 없다.

- core policy와 adapter transport를 분리해서 테스트한다.
- CLI/MCP/worker는 같은 core DTO를 쓰는지 contract test를 둔다. 설치 경로는 `core.InstallNative` + `port.HostInstaller` adapter 단위 테스트와 `internal/adapter/testdata/native_install_contract_matrix.golden.json` matrix fixture로 고정한다.
- command execution은 실제 위험 명령 대신 fake runner로 검증한다.
- fake는 대상 게이트와 **같은 fail-closed 규율**을 따른다. 모르는 입력에 성공을 돌려주는 fake는 새로 추가된 검사를 무흔적으로 통과시킨다 — #153에서 `fakeRemoteBranchGit`의 default가 exit 0을 반환해 신규 ancestry 검사가 조용히 통과했다. 게이트가 fail-closed인데 fake가 fail-open이면 테스트가 게이트를 무력화한다. 처리하지 않는 입력은 명시적으로 실패시킨다.
- 픽스처가 **실환경 순서를 재현하는지** 확인한다. #149의 브랜치 충돌 사전 확인은 로컬 브랜치를 만드는 픽스처로 GREEN이 됐지만, 당시 IssueOps 정식 순서였던 `gh issue develop`은 원격 브랜치만 만들기 때문에 실환경에서 그대로 뚫렸다(#176이 그 단계를 `createLinkedBranch`로 대체했고, 원격만 만드는 성질은 같다). 픽스처가 만드는 상태가 사용자가 실제로 도달하는 상태와 같은지 물어야 한다 — 통과하는 테스트가 옳은 테스트를 뜻하지는 않는다.
- **shape 불변식 테스트는 우연한 수치를 고정하지 않는다.** #176에서 `TestStepsKeepTheirExistingShape`가 "3단계"를 불변식으로 검사했지만, 실제 계약은 첫 단계가 MCP이고 마지막이 `fail`이며 사이에 provider CLI `fallback_api`가 오고 `Order`가 연속이라는 것이었다. 단계를 하나 늘리는 정당한 변경이 그 테스트를 깨뜨렸다. 이름이 "shape"인 테스트는 구조를 검사하고, 개수·인덱스는 그 구조가 요구할 때만 고정한다.
- filesystem test는 temporary directory를 사용하고, workspace root 밖 접근 거부를 검증한다.
- secret redaction test는 token-like fixture가 로그/응답에 남지 않는지 확인한다.
- shipped skill의 executable shell fence는
  `python3 scripts/verify-skill-shell.py`로 syntax, swallowed failure,
  fabricated zero, unsafe command expansion/word splitting, destructive
  annotation을 검사한다. Markdown 설명용 `text` fence는 실행 대상으로 취급하지
  않는다. 명시한 input path가 없거나 skill contract Markdown을 하나도 찾지
  못하면 exit 2로 실패하고, `--help`는 usage를 출력한다. 따라서 CI path typo가
  `0 file(s)` green으로 통과할 수 없다.
- Parent issue create 회귀는 provider 호출 전 intent 존재, concurrent begin
  차단, proven non-invocation만 retry, started/unknown 자동 재시도 차단,
  zero/one/many marker candidate, delayed candidate, title/body digest mismatch,
  live verification failure, URL+receipt atomic write, dry-run no-mutation을
  고정한다. Provider command test는 start failure와 post-start ambiguity를
  `IssueProviderCreateError.Invoked`로 구분한다.

## Contract / Golden tests

다음은 golden test 대상으로 둔다.

- `agent-harness inspect --json` output shape
- `agent-harness docs --json` output shape
- `agent-harness policy check/fake-run` allow/deny/fake execution output shape
- `agent-harness guard check` portable anti-pattern output shape
- MCP tool schema와 response shape
- daemon-backed MCP smoke response
- `cmd/harness/testdata/usage.golden.txt`
- `cmd/harness/testdata/mcp_tools.golden.json`
- `cmd/harness/testdata/mcp_resources.golden.json`
- `cmd/harness/testdata/response_contracts.golden.json`
- `agent-harness loop start/record-attempt/status/stop` CLI/MCP schema and response-contract entries
- `internal/adapter/testdata/native_install_contract_matrix.golden.json` — Codex/Claude user-global 기본 설치와 project-local opt-in 계약
- `agent-harness self-verify` 10회 반복 결과
- `agent-harness self-verify`의 `risk QA tier` step과 `risk_qa` goal score
- `agent-harness self-verify --json`의 `summary.contract`/`goal_scores`/`coverage_gaps`/`failure_class`/`rerun_commands`/`step_duration_stats` field
- `agent-harness self-verify candidates --json` candidate curriculum export and state save/read smoke
- `agent-harness self-verify compare` step budget p95 regression fixture for labels outside `slowest_steps`
- `agent-harness self-verify` install dry-run smoke for temp HOME/CODEX_HOME/HARNESS_ROOT no-write assertions
- `scripts/release-repro-smoke.sh` clean-machine release install reproducibility smoke
- `scripts/release-build-matrix.sh` cross-platform release build matrix smoke
- `agent-harness self-verify --save-state` summary checkpoint serialization
- `agent-harness self-verify history` summary checkpoint discovery and retention dry-run/confirm safety
- `agent-harness self-verify` native integration fixture for Claude MCP conflicting-scope warning classification
- `agent-harness self-verify` daemon resilience step for stale lock/socket recovery and socket permission checks
- `agent-harness self-verify compare` summary checkpoint regression comparison
- `agent-harness self-verify promote` dry-run/confirm baseline promotion
- `agent-harness self-augment --json` planner/candidate curriculum
- `agent-harness inspect/doctor/docs/preflight/policy/state` 실제 JSON response normalization 결과
- `agent-harness doctor --json` comprehensive diagnostics output shape
- `agent-harness state write/read/list/prune/doctor/migrate` output shape
- `issueops prune` unreadable-row total과 bounded `{id,code}` diagnostics
- `issueops remote create-issue/reconcile-issue` intent and adoption fields
- command policy allow/deny 결과
- command policy catalog table과 `CommandPolicySummary().catalog` 노출
- state write/read/prune/doctor/migrate serialization
- redaction 결과

Golden file은 사람이 읽을 수 있게 작게 유지하고, schema 변경 시 의도와 migration을 문서화한다. 실제 CLI/MCP JSON response golden은 timestamp, temp path, audit id, git sha, home/harness path를 `$TIMESTAMP`, `$STATE_DIR`, `$WORKSPACE`, `$GIT_REPO`, `$GIT_SHA`, `$HOME`, `$HARNESS_ROOT`, `$AUDIT_ID` 같은 placeholder로 normalize한 뒤 비교한다.

## Contract/audit/worker verification

CLI/MCP DTO를 변경할 때는 `agent-harness contract check --json`과 golden test를 실행해 command name, MCP tool name, required response field가 machine-visible하게 유지되는지 확인한다. policy audit 동작 변경은 JSONL record가 append-only이고 secret-like argument가 redacted 되는지 검증한다. generic worker 변경은 no-shell MVP 범위이므로 enqueue/status/list/cancel을 테스트한다.

## Lifecycle state tests

- project lifecycle namespace tests must use `HARNESS_STATE_DIR` with `t.TempDir()` and must not write runtime state under the target repo.
- bootstrap tests should verify dry-run plans `projects/<repo-id>/project.json` without creating it, and normal `project bootstrap` writes lifecycle profile metadata in user-state.
- hook tests should cover fallback behavior when lifecycle state is missing/corrupt so prompt routing remains useful.
- kubectl live-access hook tests must cover Codex first-block/token reuse/exact approval/one allow/re-block, session/workspace/cwd/tool/command mismatch, 10-minute pending/granted expiry, concurrent single consume, `0600` state without raw command, and unchanged Claude native `ask` behavior.
- doctor tests should cover repo-local `.agent-harness/state/` and namespace mismatch warnings.
