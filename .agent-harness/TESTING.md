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
./bin/agent-harness project draft-wiki init --dry-run --json
./bin/agent-harness project draft-wiki list --json
./bin/agent-harness project draft-wiki suggest --input .agent-harness/ADR.md --target-wiki dev-fundamentals --dry-run --json
tmp_state="$(mktemp -d)" && printf 'draft wiki smoke\n' | HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness project draft-wiki queue --repo "$PWD" --stdin --json && rm -rf "$tmp_state"
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

### Cross-host tool contract conformance

기본 self-verify에는 network/auth가 없는 다음 deterministic baseline이 포함된다.

```bash
./bin/agent-harness contract conformance baseline --json
```

baseline은 representative schema 3개와 `valid`, `unknown_key`, `coercible_type_drift`, `noncoercible_type_drift` payload class의 preregistered 10 cases를 정확히 판정하고, 승격된 behavioral regression fixture가 있으면 handler 호출 0회·동일한 state digest·정규화된 final result를 재생한다.

Live 측정은 CI와 기본 self-verify에 포함하지 않는다. `HARNESS_TOOL_CONFORMANCE_LIVE=1`과 host/model/auth 입력을 명시한 뒤 clean-context `3 hosts × 3 fixtures = 9 completed episodes`를 수집한다. environment/transport/no-call attempt는 model denominator에서 제외하며, case당 최대 3회 retry 후 9 episodes를 채우지 못하면 `inconclusive`다. invalid raw call은 동일 host/schema/diagnostic signature가 2회 이상 재현되어야 regression fixture와 canonical production enforcement 후보가 된다. 한 번뿐인 관측은 승격하지 않는다.

환경 실패율 5%는 조사 warning일 뿐 pass/fail threshold가 아니다. context-pressure profile과 10/20 reproduction batch는 clean initial matrix와 denominator를 합치지 않고 별도 승인·비용 경계로 실행한다. evidence는 `.agent-harness/evidence/tool-conformance/`에 mode 0600/0700으로 저장하고 git에 추가하지 않는다.

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
- workpool `--pilot` / `pilot_required` CLI/MCP schema entries and pilot field response compatibility
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

CLI/MCP DTO를 변경할 때는 `agent-harness contract check --json`과 golden test를 실행해 command name, MCP tool name, required response field가 machine-visible하게 유지되는지 확인한다. policy audit 동작 변경은 JSONL record가 append-only이고 secret-like argument가 redacted 되는지 검증한다. generic worker 변경은 no-shell MVP 범위이므로 enqueue/status/list/cancel을 테스트한다. draft-wiki worker 변경은 fake `agy`와 temp settings/hub/state를 사용해 명시 `project draft-wiki queue` 적재와 `worker draft-wiki`의 `.agent-harness/draft-wiki/draft` 파일 생성을 함께 검증한다. PostToolUse hook이 draft-wiki queue를 자동 생성하지 않는 회귀 테스트를 유지한다.

## Lifecycle state tests

- project lifecycle namespace tests must use `HARNESS_STATE_DIR` with `t.TempDir()` and must not write runtime state under the target repo.
- bootstrap tests should verify dry-run plans `projects/<repo-id>/project.json` without creating it, and normal `project bootstrap` writes lifecycle profile metadata in user-state.
- hook tests should cover fallback behavior when lifecycle state is missing/corrupt so prompt routing remains useful.
- kubectl live-access hook tests must cover Codex first-block/token reuse/exact approval/one allow/re-block, session/workspace/cwd/tool/command mismatch, 10-minute pending/granted expiry, concurrent single consume, `0600` state without raw command, and unchanged Claude native `ask` behavior.
- doctor tests should cover repo-local `.agent-harness/state/` and namespace mismatch warnings.

## Optional Orca handoff verification

Normal tests and self-verification must remain green without Orca. Use an injected fake runner to prove byte-exact JSON/text `auto` fallback with no state rewrite, explicit-mode failure, pending-operation ordering, at-most-once external calls, post-invocation `recovery_required`, exact-one reconciliation, stale ownership rejection, accepted-terminal retry rejection, and CLI/MCP parity. Sole-writer tables cover pre-existing baseline terminals that are connected or writable, server-filtered dispatched-task conflicts, immediate pre-create/pre-dispatch re-attestation, and retry without exact quiescence; only the designated active worker must be both connected and writable. Publication tables cover wrong local FinalHead, missing/stale/wrong remote receipts, provider/branch/ref drift, GitHub/GitLab parity, and PR/MR body-file rejection. Cancellation/cleanup tables cover submitted terminal-projection force-cancel/finalize, approval-before-mutation, ordered idempotent receipts, stale-quiescence denial of raw removal, incomplete worktree-row refusal, and the accepted publish boundary. Retry tests must allow only `closed/worker_failed` and `closed/cancelled` after durable retry cleanup receipts; `closed/accepted` remains unchanged and fails closed. Schema fixtures keep missing/zero legacy records inline-compatible, prove v4 rejects v5 and frozen v5 rejects raw v6 byte-equivalently, reject raw v5 `remote_create_claim` or copied `coordinator_session` authority before rewrite, and reject future versions. Literal HEAD-era v5 receipt fixtures omit `coordinator_session`, prove implicit sealing fails byte-identically, prove explicit source-native seal plus locked target/local/remote re-attestation writes v6 atomically, and keep the invalid closed authority visible to lifecycle hooks. MCP authority tests include foreign namespace suffix collisions for every privileged handoff tool.

Completed projection tests must begin from `claimed` and observe the crash seam immediately after the one locked write: the durable row must already contain both `submitted` result evidence and projection `intent`, while the external call count remains zero. Success and every attempted ambiguous/malformed/timeout path call the dedicated adapter exactly once; every failed precondition calls it zero times. A persisted intent, sent result, or failed diagnostic makes identical and concurrent finish calls return stable evidence without retry. Payload assertions derive sender from the sealed historical `WorkerMailboxHandle`, recipient from the sealed coordinator mailbox, and task/dispatch/files/report/final-head/host/attempt only from the durable row; a different rollover `WorkerTerminalHandle` must remain live-control evidence and must not become the sender.

Dispatch context tests require the official exact `Your coordinator's terminal handle is:` and `Your task ID is:` lines plus an exact `--dispatch-id` token. Substring-only, wrong-label, wrong-task, wrong-dispatch, oversized, or missing preambles fail closed after one dispatch attempt. Recovery rejects group, shell-like, or overlong coordinator recipients before `ShowDispatchFrom`. Root schema tests cover v3→v4 migration without inventing a coordinator recipient: the current attempt and every prior attempt copy missing live control from the legacy mailbox and clear mailbox authority when no dispatch exists. `DispatchID` and `WorkerMailboxHandle` are either both absent or both present in v4. A frozen ContextVersion-1 fixture proves an empty `coordinator_recipient` remains omitted and byte/hash compatible while a nonempty recipient participates in preview/confirm context and source hashes; a prior-v3 decoder still rejects a v4 row without changing one byte.

Filesystem evidence is a second CAS dimension, not a substitute for durable record equality. Deterministic seam tests must mutate source fingerprint, branch, HEAD, clean status, or submitted report after outer validation and prove the same evidence is rechecked inside the cycle lock immediately before every affected write. Dispatch-stage seams must additionally prove completed prior identities remain exact and `pending_operation` stays nil. Progressing-clock fixtures must distinguish every operation `started_at` from its post-call completion/failure timestamp.

The PreToolUse matrix must prove that coordinator plan file edits require both request CWD and repo identity to equal the source `record.Repo`. It must also table-test installed Orca terminal controls (`send`, `stop`, `create`, `switch`, `focus`, `close`, `rename`, `split`) plus write/input/type/paste aliases from a claimed worker and non-source session, while preserving read-only `list`, `show`, `read`, and `wait`. The only positive terminal steering fixture is exact source-coordinator single-line guidance in `claimed` state with `--terminal` equal to a uniquely matching persisted worker handle. Two active cycles with distinct handles select the exact handle; an unknown or duplicated persisted handle blocks. Duplicate discovery of one stable cycle ID does not create false ambiguity. Decoded guidance table cases must block backspace, tab, ESC, and DEL while allowing ordinary Korean/Unicode. Sentinel assertions must prove the source command is denied without relying on a target terminal hook.

When Orca is installed in the verification environment, add one disposable live E2E; this is release evidence, not a default unit-test dependency. Create a uniquely named repo/branch/worktree/terminal/task, let a fresh worker claim and submit, accept from the coordinator, then remove every disposable repo/branch/worktree/terminal resource and record completed task ids. Never use a global Orca reset.

Use this exact focused package set for the handoff and hook contracts. Hook input is under the CLI adapter; `./internal/core/hookinput` does not exist.

```bash
go test ./internal/core/issueops ./internal/core/issueops/handoff ./internal/adapter/orca ./internal/core/lifecycle ./internal/core/commandparse ./internal/core/skillcontract ./cmd/harness/hookcli ./cmd/harness/hookcli/hookinput ./cmd/harness/issueopscli ./cmd/harness/harnessapp -count=1
```

Native ownership smokes are required for Codex, Claude, and GJC. Feed each installed adapter a distinct native `session_id`, assert coordinator/wrong-session mutation produces that host's real block shape, and verify a matching claimed worker passes. GJC coverage must exercise the HookAPI `(event, ctx)` bridge so `ctx.sessionManager.getSessionId()` and `ctx.cwd` reach the common hook command.

Supervised Codex startup tests require four separate assertions: unattested confirmed start makes zero terminal/task/dispatch calls; installed probe help contains `--dangerously-bypass-hook-trust`; an attested no-confirm preview and otherwise identical confirm render the same context hash; and fake-runner terminal argv uses the bypass flag only for attested Codex, never Claude or GJC. Legacy closed version-1 retry preserves all delivery options while resetting/re-attesting only the optional bypass bool.

Runtime-rollover fixtures must use the installed public shape: changed runtime/handle/PTY, stable tab/leaf, dynamic terminal title, stable custom title under `visualLayouts[].root.tabs[]`, and a complete current-runtime worktree row. Positive coverage includes both changed and equal nonempty worktree instance IDs. Missing/conflicting instance, terminal/worktree mismatch, wrong marker/HEAD/branch, incomplete or duplicate inventory, dynamic-title-only legacy fallback, and connected reissued cancellation all fail closed. A deterministic after-inventory seam changes the durable journal, context source, or worker cleanliness before the locked write; every row must preserve the old runtime/instance/handle/PTY/tab/leaf and pending `runtime_refresh` with zero later Orca mutation. Terminal-create capability tests cover both fixed built-in `--agent` and generated fixed `--command`; help loss after provisioning keeps `pending_operation=terminal_create` in `recovery_required` even when no create command ran.

Provider-aware worktree tests must prove GitHub alone requires/passes the public `--issue` flag and GitLab passes no issue flag or invented metadata option. Use the installed nullable `linkedGitLabIssue` shape: null/zero persists an unavailable observation and stable warning, exact metadata persists an exact observation without warning, and conflicting GitHub or mismatched GitLab metadata fails closed. Re-read the durable row and reproject prepare status to prove the warning survives restart, and assert both JSON and human CLI warning surfaces. GitLab `auto` with missing, unready, or capability-failed Orca—and a post-probe inline fallback—must remain warning-free and leave `execution_handoff` absent; nil `BranchPrepare` retains its legacy inline behavior.

The lifecycle hook has two minimal direct-command prefilters. A unique invalid or duplicate explicit `orca orchestration send --type` blocks with the installed eight-value set, while no type and valid no-record values fall through without new authority. Any explicit `--inject` or equals form on direct `orca orchestration check` blocks before record selection; exact `check --all --json` and unrelated observations fall through. The skill recipe then selects exact current task/dispatch/sequence and never treats a live handle as historical mailbox identity.

After native installation, exercise the installed GJC TypeScript shim through that behavior boundary:

```bash
bun scripts/smoke-gjc-native-hook.ts "$HOME/.gjc/agent/hooks/agent-harness.ts"
```

Require JSON `ok=true`, `host=gjc`, the fixed smoke session/cwd, and `blocked=true`. Do not use a literal `--host gjc` grep: the shim stores argv as adjacent TypeScript array elements, so a shell-string grep is not behavior evidence.
