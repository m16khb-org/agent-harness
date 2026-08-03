---
name: TESTING.md
description: Verification standards, test practices, and required checks.
---

# 테스트 컨벤션

이 문서는 `agent-harness`의 문서·코드 변경 검증 규칙이다.

---

## 1. 현재 문서 단계 검증

문서만 변경한 경우에도 최소한 다음을 확인한다.

```bash
find . -maxdepth 3 -type f | sort
find .agent-harness -maxdepth 1 -type f -name '*.md' | sort
grep -R "외부 Go 하네스\|Go\|MCP\|Codex\|Claude" -n AGENTS.md CLAUDE.md .agent-harness
python3 scripts/validate-skill.py skills/atomic-commit-push
go test ./... -count=1
go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1
go build -o bin/agent-harness ./cmd/harness
./scripts/install-native.sh
./bin/agent-harness bootstrap --dry-run
./bin/agent-harness install-native --json
./bin/agent-harness install-native --dry-run --json
./bin/agent-harness inspect --json
./bin/agent-harness docs --json
./bin/agent-harness guard check --staged --json
printf '{"prompt":"endpoint와 DTO를 추가해줘"}' | ./bin/agent-harness hook user-prompt
./bin/agent-harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
./bin/agent-harness policy fake-run --workspace-root "$PWD" --cwd "$PWD" --write --json -- touch marker
tmp_state="$(mktemp -d)"
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness doctor --repo . --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness state write --key smoke --value "ok" --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness state read --key smoke --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness state list --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness state prune --max-age 720h --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness state doctor --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness state migrate --json
HARNESS_DAEMON_DIR="$tmp_state/daemon" ./bin/agent-harness daemon status --json
HARNESS_DAEMON_DIR="$tmp_state/daemon" ./bin/agent-harness daemon start --json
HARNESS_DAEMON_DIR="$tmp_state/daemon" ./bin/agent-harness daemon stop --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --save-state --state-key self-verify-smoke --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness self-verify history --prefix self-verify --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness self-verify history --prefix self-verify --retention-limit 1 --prune-retention --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness self-verify compare --baseline-key self-verify-smoke --candidate-key self-verify-smoke --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness self-verify promote --from-key self-verify-smoke --baseline-key self-verify-baseline --json
./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
./bin/agent-harness self-augment --cycles=1 --target-score=95 --json
./bin/agent-harness self-augment --cycles=1 --target-score=95 --save-state --state-key self-augment-latest --json
./bin/agent-harness self-augment lesson --candidate reflexion-state-memory --lesson "test lesson" --next-action "test next action" --state-key self-augment-lesson-test --json
./bin/agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json
./bin/agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge agy --json
grep -R "Conventional Commit\|Lore:" -n AGENTS.md .agent-harness/COMMIT_POLICY.md skills/atomic-commit-push/SKILL.md
```

IssueOps benchmark fixtures must stay repo-agnostic. They should score portable workflow evidence rather than one target repository's domain facts: domain invariants vs exact/equivalent mechanisms, API-doc gate evidence, live runtime evidence matrices, review-feedback accountability, and completion hygiene. A passing deterministic benchmark means `average_score == 100`, `minimum_score == 100`, and `critical_failure_count == 0`.

확인할 것:

Native integration smoke:

```bash
test -f ~/.codex/skills/atomic-commit-push/SKILL.md
test -f ~/.claude/skills/atomic-commit-push/SKILL.md
codex mcp get agent_harness
claude mcp list | grep agent_harness
```

- `AGENTS.md`와 `CLAUDE.md`가 같은 source of truth를 가리키는가
- `.agent-harness/`의 링크가 실제 파일과 맞는가
- plugin vs worker 결정과 Go 선택이 여러 문서에서 충돌하지 않는가
- shared skill 원본(`skills/*`)과 user-level host 연결(`~/.codex/skills/*`, `~/.claude/skills/*`)이 drift 없이 같은 대상을 가리키는가
- 커밋 정책이 `AGENTS.md`, `.agent-harness/COMMIT_POLICY.md`, `atomic-commit-push` skill에서 충돌하지 않는가

---

## 2. Go 코드 변경 기본 검증

Go 코드가 추가되면 기본 검증은 다음이다. `self-verify`는 working tree risk를 보고 `risk QA tier`에서 `go vet ./...` 또는 `go test -race ./... -count=1`를 조건부 실행한다.

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/agent-harness ./cmd/harness
```

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

### GitLab Issue Snapshot

GitLab MCP/CLI snapshot 계약을 바꿀 때는 다음 bounded set을 먼저 실행한다.

```bash
go test ./internal/core/issueops ./internal/adapter/provider/gitlab -run 'IssueSnapshot|ExecutionIssueSnapshot' -count=1
go test ./internal/adapter/mcp ./cmd/harness/mcpcli ./internal/core/toolconformance -count=1
go test ./cmd/harness/issueopscli/executioncmd ./cmd/harness/issueopscli -run 'Snapshot|ExecutionCLI|Usage' -count=1
go test ./internal/core/skillcontract -run TestGitLabSnapshotSkillsPinPortableVCSContract -count=1
python3 scripts/validate-skill.py skills/gitlab-usecase
python3 scripts/validate-skill.py skills/issueops
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
go build -o bin/agent-harness ./cmd/harness
```

설치 갱신 뒤에는 installed Codex/Claude MCP schema에 `issue_snapshot`의 exact
다섯 필드만 있는지 확인하고, GitLab-linked lifecycle preview에서
`resolved_mode=orca`와 `issue_snapshot_source=glab_mcp|glab_cli`를 확인한다.
이 smoke는 worktree나 lease를 만들지 않는 preview로 제한한다.

### Cross-host tool contract conformance

기본 self-verify에는 network/auth가 없는 다음 deterministic baseline이 포함된다.

```bash
./bin/agent-harness contract conformance baseline --json
```

baseline은 representative schema 3개와 `valid`, `unknown_key`, `coercible_type_drift`, `noncoercible_type_drift` payload class의 preregistered 10 cases를 정확히 판정하고, 승격된 behavioral regression fixture가 있으면 handler 호출 0회·동일한 state digest·정규화된 final result를 재생한다.

Live 측정은 CI와 기본 self-verify에 포함하지 않는다. `HARNESS_TOOL_CONFORMANCE_LIVE=1`과 host/model/auth 입력을 명시한 뒤 clean-context `3 hosts × 3 fixtures = 9 completed episodes`를 수집한다. environment/transport/no-call attempt는 model denominator에서 제외하며, case당 최대 3회 retry 후 9 episodes를 채우지 못하면 `inconclusive`다. invalid raw call은 동일 host/schema/diagnostic signature가 2회 이상 재현되어야 regression fixture와 canonical production enforcement 후보가 된다. 한 번뿐인 관측은 승격하지 않는다.

환경 실패율 5%는 조사 warning일 뿐 pass/fail threshold가 아니다. context-pressure profile과 10/20 reproduction batch는 clean initial matrix와 denominator를 합치지 않고 별도 승인·비용 경계로 실행한다. evidence는 `.agent-harness/evidence/tool-conformance/`에 mode 0600/0700으로 저장하고 git에 추가하지 않는다.

Reversible child-host smoke는 일반 live matrix와 별도다. `scripts/verify-child-host-smoke.sh`는 literal `--confirm-user-activation`, clean local HEAD, exact singleton remote ref가 모두 일치할 때만 user-scope integration을 잠시 활성화한다. 활성화 직후 두 host의 managed `SessionStart`/`PreToolUse` handler는 command·type·timeout·key set까지 exact contract로 검증하며, enforcement flag 누락이나 shell suffix 추가를 거부한다. Codex `exec --json`은 hook lifecycle notification을 JSONL에 투영하지 않으므로 실제 child hook process가 private marker에 `SessionStart`/`PreToolUse` 이름만 기록한다. Claude child command에는 marker 경로를 직접 주입한다. Codex는 검증된 활성화 handler 자체를 user config·plugin·co-resident hook을 로드하지 않는 private episode `CODEX_HOME`에 투영한 뒤 invocation-scoped `--dangerously-bypass-hook-trust`를 사용하며 trust state는 수정·저장하지 않는다. Host runner는 marker와 native MCP result를 boolean/count/SHA-256/exit/duration projection으로 합친 뒤 원문 stream과 marker를 폐기한다. 어떤 post-activation 실패도 source installer 1회, 원래 네 설정 파일의 private byte snapshot 원자 복원, before/restore raw+semantic digest equality를 모두 통과하지 못하면 `verdict=pass`가 될 수 없다.

managed regular command adoption 테스트는 기본 refusal과 승인 dry-run 무변경, 실제 staged candidate의 정적 build identity, file matrix/size boundary, atomic exchange 시점의 destination drift 보존, apply/finalize, injected rollback, transition-fenced Begin/Seal/Abort, direct-vs-explicit cleanup ownership을 각각 검증한다. child smoke에서 adoption은 literal confirmation 이후 child activation 한 번에만 전달되어야 하며 source restore에는 전달되지 않아야 한다.

---

## 3. 테스트 작성 기준

### Well-structured tests

- 변경된 공개 동작/계약을 직접 검증하고 구현 세부사항에 과도하게 묶이지 않는다.
- 실패 시 원인을 좁힐 수 있는 fixture 이름, assertion, 에러 메시지를 둔다.
- deterministic하며 test order, wall-clock sleep, real network, local machine state에 의존하지 않는다.
- 저장소 내부 심볼을 찾는 contract는 indexed repo에서 CodeGraph 우선, 비인덱스 repo에서만 `rg`/직접 읽기다. local symbol discovery에 web search를 쓰는 fallback은 허용하지 않는다.
- regression test는 재발했던 입력, false case, 기대 결과를 명확히 담는다.
- 기존 helper와 style을 재사용하고, golden/snapshot 변경은 의도와 범위를 설명한다.
- stdout/stderr를 `os.Pipe`로 캡처하는 테스트는 직접 write-then-read 헬퍼를 만들지 말고 `internal/testsupport`의 동시-reader 캡처 헬퍼를 사용한다.

### Poorly-structured tests

- 실제 요구사항과 무관한 내부 구조만 고정한다.
- 통과를 위해 production behavior를 약화하거나 오류 처리를 숨긴다.
- sleep, real external service, global mutable state, 실행 순서에 의존한다.
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
- daemon/proxy test는 socket path override, MCP stream, start/status/stop smoke, stale lock 복구를 포함한다.
- worker test는 timeout, cancellation, stale lock, concurrent job을 포함한다.

---

## 4. Contract / Golden tests

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
- command policy allow/deny 결과
- command policy catalog table과 `CommandPolicySummary().catalog` 노출
- state write/read/prune/doctor/migrate serialization
- redaction 결과

Golden file은 사람이 읽을 수 있게 작게 유지하고, schema 변경 시 의도와 migration을 문서화한다. 실제 CLI/MCP JSON response golden은 timestamp, temp path, audit id, git sha, home/harness path를 `$TIMESTAMP`, `$STATE_DIR`, `$WORKSPACE`, `$GIT_REPO`, `$GIT_SHA`, `$HOME`, `$HARNESS_ROOT`, `$AUDIT_ID` 같은 placeholder로 normalize한 뒤 비교한다.

---

## 부분 검증 상태 금지 (all-or-nothing verification)

다단계 검증 시나리오에서 한 단계라도 실패하면 이전 단계의 통과를 재사용하지 않고 1단계부터 전체 재실행한다.

완료 보고의 evidence는 마지막 "전 단계 통과" 단일 run에서 나온 것이어야 하며, 서로 다른 run의 부분 통과를 조합하지 않는다.

재실행 비용이 큰 경우에도 부분 통과 상태를 "검증됨"으로 승격하지 않는다. 비용이 문제면 검증 시나리오를 더 작은 독립 시나리오로 분리한다.

---

## 5. 완료 보고 기준

완료 보고에는 다음을 포함한다.

- 실제 실행한 검증 명령과 결과
- 실패가 있었다면 실패 원인과 수정/미해결 상태
- 실행하지 않은 검증과 이유(`Not-tested`)
- 변경 파일 요약과 남은 위험

## 자기 검증 QA gate

`agent-harness self-verify`는 테스트 실행뿐 아니라 QA gate를 포함한다. QA gate는 루프 문서, `GENIUS_THINK.md`, shared skill metadata, native integration 설치 상태, redaction audit, bounded stdout/stderr metadata, Mermaid 문서 lint를 확인하며, 모든 목표 점수가 95점을 초과해야 종료 가능하다. Mermaid lint는 `GENIUS_THINK.md`의 따옴표/`<br/>` 규칙을 기준으로 문서 다이어그램 파싱 오류를 조기에 막는다.

## Web-Fetch Live Parity

The default web-fetch battery is deterministic and must not require network access. Opt-in live parity uses `HARNESS_WEBFETCH_LIVE=1` and the fixture file at `testdata/webfetch/live/public-fixtures.json`; follow `.agent-harness/operations/web-fetch-live-parity.md` before interpreting live results or comparing against a generic baseline executable.

## Standalone Verification Policy

Agent-harness tests must verify harness core and native integration contracts without requiring external toolchains, external accounts, or companion MCP servers. External companion tools are not prerequisites for install/update/self-verify readiness.

The standard deterministic self-verify commands pin `--llm-eval=false` so an intentional ambient `HARNESS_SELF_VERIFY_LLM_EVAL=gate` cannot turn the project gate into the prompt-only diagnostic path. Enabling `advisory` or `gate` currently renders a read-only evaluator prompt and sends no Z.AI request; `gate` is therefore expected to remain non-passing without an ingested external verdict. Record the explicit override and rerun the verification sequence from its first gate after any interrupted or prompt-only attempt.

If a test fixture models data produced by an external tool, keep it as plain local input and verify only the harness boundary that consumes that input. Do not add tests that clone, install, patch, or register external tools as part of normal verification.

## API documentation checks

Endpoint/DTO 변경 시 Swagger/OpenAPI 문서 검사를 필수로 실행한다.

```bash
agent-harness api-doc check --json
# target repo에 package script가 있으면 그 repo에서는 다음 wrapper를 권장한다.
npm run swagger:check
npm run swagger:check -- --all
```

기본 검사 범위는 staged controller/DTO/handler/OpenAPI 후보 파일이어야 하며, 기존 레거시 전체 부채를 한 번에 실패시키지 않는다.

NestJS Swagger 프로젝트에서 blocking으로 잡아야 하는 누락:

- REST route method의 `@ApiOperation` 누락
- `@ApiOperation.description` 누락 또는 repo의 문서 섹션 형식 위반
- `:id` 등 path param이 있는데 `@ApiParam` 누락
- `@Headers` 사용인데 `@ApiHeader` 누락
- `@Body`, `@Query`, `@Headers` 사용으로 validation 400 가능성이 있는데 400 Swagger response 누락
- private/auth endpoint인데 401 Swagger response 누락
- DTO required property의 `@ApiProperty` 누락
- DTO optional property의 `@ApiPropertyOptional` 누락
- DTO optional property의 `@IsOptional` 누락

### Business logic error response coverage

Swagger/OpenAPI 검사는 decorator/comment 존재 여부만 보지 않는다. 변경된 endpoint가 호출하는 service/usecase/domain error mapping을 확인해 비즈니스 로직상 가능한 public error response가 스펙에 있는지 확인한다.

예:

- entity lookup 실패 → 404 response 필요
- ownership/permission 실패 → 403 response 필요
- duplicate/state conflict → 409 response 필요
- validation/body/query/header 문제 → 400 response 필요
- private/auth endpoint → 401 response 필요

깔끔한 Swagger 문서는 success-only가 아니라, client가 실제로 처리해야 하는 성공/실패 계약을 모두 보여줘야 한다.

### Agent-backed verification boundary

비즈니스 로직의 실제 404/403/409 가능성과 OpenAPI 누락 여부는 정적 테스트만으로 신뢰 있게 판정하지 않는다. 정적 테스트는 후보 파일 선택, `--all` wiring, prompt contract, MCP schema 같은 배선을 검증하고, 실제 API 문서 품질 판정은 `agent-harness api-doc review`/MCP `api_doc_review`가 렌더한 prompt/schema를 host agent가 수행한 뒤 결과 JSON을 `--result`/`result_file`로 기록해 수행한다.

## Prompt contract tests

Reusable LLM prompts should follow the shared prompt structure through `internal/core.BuildStructuredPrompt` or equivalent JSON packet keys. When adding or changing prompt builders, add or update tests that check for identity, objective, operating phases, inputs, rules, output contract, and verification checklist. Strict-output prompts must also keep tests for JSON-only/no-fence/no-preamble behavior.

`nextcandle-api`에서 확인한 좋은 기준:

- `@ApiOperation.description`은 `### 목적`, `### 요청 규칙`/`### 처리 방식`, `### 권한/주의사항`처럼 Markdown section + bullet로 구성한다.
- path/query/header/body와 auth/tier/public 여부가 response 문서와 일치한다.
- service/usecase의 `NotFoundException`, `ForbiddenException`, `ConflictException` 등 public error가 endpoint response에 반영된다.
- public/admin Swagger document를 분리하고, 사용하지 않는 schema를 필터링해 client가 읽는 문서를 깔끔하게 유지한다.


## OpenAPI prompt source

Endpoint/controller/DTO/schema/OpenAPI 변경 시 `.agent-harness/OPEN_API_SPEC.md`를 프로젝트별 프롬프트 source로 사용한다. `agent-harness api-doc review`는 별도 `--prompt-file`이 없으면 이 문서를 자동으로 포함한다.


## Contract/audit/worker verification

CLI/MCP DTO를 변경할 때는 `agent-harness contract check --json`과 golden test를 실행해 command name, MCP tool name, required response field가 machine-visible하게 유지되는지 확인한다. policy audit 동작 변경은 JSONL record가 append-only이고 secret-like argument가 redacted 되는지 검증한다. generic worker 변경은 no-shell MVP 범위이므로 enqueue/status/list/cancel을 테스트한다.

## Lifecycle state tests

- project lifecycle namespace tests must use `HARNESS_STATE_DIR` with `t.TempDir()` and must not write runtime state under the target repo.
- bootstrap tests should verify dry-run plans `projects/<repo-id>/project.json` without creating it, and normal `project bootstrap` writes lifecycle profile metadata in user-state.
- hook tests should cover fallback behavior when lifecycle state is missing/corrupt so prompt routing remains useful.
- kubectl live-access hook tests must cover Codex first-block/token reuse/exact approval/one allow/re-block, session/workspace/cwd/tool/command mismatch, 10-minute pending/granted expiry, concurrent single consume, `0600` state without raw command, and unchanged Claude native `ask` behavior.
- doctor tests should cover repo-local `.agent-harness/state/` and namespace mismatch warnings.

## IssueOps v1 execution and optional Orca verification

`execution release` production vertical은 `internal/core/issueops` differential
test로 schema v1·missing/zero legacy schema·rich sidecar·holder index와 denial
atomicity를 비교한다. 변경 시 core focused race, outbound focused race,
architecture ratchet, CLI/MCP contract, golden, scoped vet와 build를 실행하며,
전체 suite는 PR CI가 회귀 증거로 담당한다.

`issueopspublication` vertical은 fixed operation ID와 fixed clock을 쓰는 frozen
legacy oracle/new vertical differential로 create·reconcile의 result JSON, error
text, record row, `external_intent_v1` row를 byte-for-byte 비교한다. Provider
create·inventory·live verification 중에는 cycle lock이 해제되어 동시 read와
replacement preview가 완료되어야 한다. CLI text, MCP `isError`, production
provider resolver caller-zero, non-test legacy full-flow 부재를 각각 adapter 및
AST ratchet 테스트로 고정한다.

Normal tests and self-verification must remain green without Orca. Use injected
workspace, provider, process, and Orca adapters for the default suite. The
current execution contract is `issueops_v1` with `schema_version=1`; legacy
rows and files are reset through the explicit fingerprint-CAS maintenance path,
never migrated or dual-read by normal execution.

Execution tests must cover:

- `direct`, explicit `orca`, and `auto`; `auto` may fall back only when the
  read-only Orca probe fails before the first external mutation.
- claimable → active → revoking/released generation transitions, the
  `lease_holder_v1` reverse index, exact native process identity, and stale
  holder/generation rejection across CLI, MCP, and hooks.
- every record-specific hook block exposes exactly `code`, `lifecycle_id`,
  `expected_root`, `current_generation`, and `next_command` in raw JSON. The
  Codex/Claude native reason string must decode to the same five-field object
  without adding fields rejected by either host's strict hook schema.
- replacement preview, revoke, and finalize with PID-reuse-safe process
  observation plus complete Orca owner inventory. A live terminal, task, or
  dispatch blocks finalization.
- a sealed context packet and owner prompt with exact digests, no raw claim
  token, no unresolved placeholder, only current catalog commands, and the
  exact ordered 14-field owner report golden.
- preparation and every Orca reseed persist artifact identity version 1 and one
  complete issue-body, packet, and prompt digest identity. Resume must survive an owner-prompt template
  upgrade without rerendering, while independent prompt, packet, issue-body,
  or stored-digest drift fails before an Orca mutation. Unversioned all-empty
  legacy bindings route through preview and generation-CAS reseed; versioned
  all-empty, unversioned-complete, partial, and future-version identities are
  invalid. Producer tests must prove new prepare and reseed outputs carry both
  the version marker and all three digests.
- Orca plan readiness tests must prove a non-empty staged `plan` before fresh
  owner evidence/mutation, atomic `plan_path` persistence with the worktree
  receipt, exact staged/sealed/durable digest equality on reseed and resume,
  and zero operation/worktree/terminal/Run/task/dispatch/lease mutations on
  failure. Released recovery staging is limited to a clean holderless Orca
  generation and changes only the next reseal input. Run the focused regressions
  with `go test ./internal/core/issueops ./cmd/harness/harnessapp -run 'PlanArtifact|Preparation.*Plan|Owner.*Plan|Replace.*Plan|Intent.*Plan|Resume.*Plan' -count=1`
  plus `go test ./internal/core/issueops ./internal/core/lifecycle -run 'Artifact.*Released|Released.*Artifact' -count=1`.
- completion only from `pr` with the durable verified PR/MR projection; the
  completion receipt, lease release, reverse-index deletion, and `done` phase
  transition are one atomic write. An identical retry is idempotent only when
  all terminal invariants still hold.
- completed replacement preview and reseed must test parent drift and no-drift
  against the same outbound observer. Drift must preserve the raw record,
  completion/history/ledger/lease, token paths, artifact prepare count, and
  repository commit count.
- current completion generation 0/missing is invalid in preview and reseed even
  when the request supplies a generation. No selected-generation compatibility
  fallback or legacy wording is permitted.
- released-completion sync-base tests must cover matching/missing/wrong/history
  generation, claimable/history-only state, canonical cwd, live/mismatched
  process receipt, pending intent, stale fingerprint, immutable completion, and
  exact apply/finalize/abort/retry commands. Hook matrices must run the exact
  forms for Codex and Claude and block duplicates, wrappers, shell expansion,
  wrong cwd/lifecycle, stale history generation, multiple modes, and unknown
  flags.
- architecture tests must scan every production Go file in
  `internal/port/issueopsbasesync` and allow only Request, Receipt, Inspector,
  and the `context` import.
- typed-error
  `next_command` and conflict `abort_command` cannot escape generated-command
  provenance binding. GREEN requires canonical executable, hash, and generation
  provenance on both fields, or a tested conversion to non-executable guidance.
- released sync-base production reachability must start from a claimable fixture
  and use the public execution dispatcher with the production claim and complete
  handlers before preview/apply/finalize. Direct completion/active record writes
  are allowed only for isolated gate tests and cannot serve as vertical evidence.
- generated `next_command` tests must cover every production-reachable adapter
  path. CLI covers prepare/status/replace/resume/sync-base/switch-mode preview;
  MCP covers its advertised prepare/status/replace/resume/reconcile/complete
  surface plus typed base-sync-required errors from resume/replace. Because MCP
  has no sync-base action, a success-result sync-base binder/test is dead code.
  Both adapters also cover
  execution reseed preview, cleanup finish preview/apply, exact current-binary path/hash binding,
  observation failure with no command fallback, and stale installed-binary versus
  newer worktree-binary rejection before a mutation handler is entered. The same
  envelope must decode to equivalent Codex and Claude hook decisions. A transition
  such as switch-mode apply that removes execution authority must return non-command
  guidance instead of an executable `next_command`.
- Codex exec hook tests must include its stable command-only payload shape: top-level
  turn cwd points at the source checkout and `tool_input` has no workdir. A current-
  generation absolute generated IssueOps mutation may proceed only when its exact
  `--cwd` selects the canonical worktree and the CLI independently proves that
  `os.Getwd()` matches it before mutation. Bare commands, mismatched process cwd,
  stale provenance, and delegation commands whose `--parent` identity is missing
  must remain fail-closed.

Orca external-intent tests treat worktree, terminal, Run create, Run bind, task,
and dispatch as six separate durable stages. For every stage, exercise authoritative 0, exact 1,
multiple candidates, transport failure, post-mutation crash, and CAS identity
change. Zero may invoke only with durable `not_invoked_proven` evidence and at
most one proven-not-invoked retry. The idempotent Run-bind stage may converge an
unknown outcome within the same two-attempt bound. Exact one adopts; every
ambiguous create outcome retains the intent without fallback or duplicate mutation.

The prepared runtime ID is mandatory on every terminal/task/dispatch receipt
and inventory row. Task title/display name and dispatch assignee/injection must
match the sealed intent. A terminal handle is runtime-scoped and is never
durable authority: later stages re-resolve the current handle from exact
worktree ID plus PTY. Owner quiescence uses the complete task inventory and
checks the bound dispatch independently, so a dispatched/running task cannot be
hidden by a ready-only listing.

Use this focused package set before the full repository gates:

```bash
go test ./internal/core/issueops/... ./internal/core/lifecycle ./internal/adapter/orca ./internal/adapter/codex ./internal/adapter/claude ./cmd/harness/issueopscli ./cmd/harness/hookcli ./cmd/harness/hookcli/hookinput ./cmd/harness/mcpcli -count=1
go test -race ./internal/core/issueops/... ./internal/core/lifecycle ./internal/adapter/orca ./cmd/harness/issueopscli ./cmd/harness/hookcli ./cmd/harness/hookcli/hookinput ./cmd/harness/mcpcli -count=1
```

Native activation tests use isolated temporary homes. They require same-
directory staged build, smoke, atomic rename, strict Codex/Claude semantic and
raw-file readback, and a sealed activation receipt written last. Every injected
crash before that final receipt must leave destructive legacy reset blocked.

When Orca is installed, a disposable live E2E may be added as release evidence;
it is never a default unit-test dependency. Resolve exact runtime/repo/worktree/
PTY/task/dispatch identities, never use a global reset, and remove only the
uniquely named disposable resources after an explicit cleanup decision. When
Orca is absent, prove explicit Orca mode fails before mutation and `auto`
returns the deterministic direct fallback projection.

After native installation, exercise Codex and Claude hook fixtures through the
common hook input boundary and require exact host, session, process, cwd, and
allow/block projections.
