# 테스트 컨벤션

이 문서는 `agent-harness`의 문서·코드 변경 검증 규칙이다.

---

## 1. 현재 문서 단계 검증

문서만 변경한 경우에도 최소한 다음을 확인한다.

```bash
find . -maxdepth 3 -type f | sort
find .agent-harness -maxdepth 1 -type f -name '*.md' | sort
grep -R "외부 Go 하네스\|Go\|MCP\|Codex\|Claude" -n AGENTS.md CLAUDE.md .agent-harness
python3 ${CODEX_HOME:-$HOME/.codex}/skills/.system/skill-creator/scripts/quick_validate.py skills/atomic-commit-push
go test ./... -count=1
go test ./cmd/harness -run Golden -count=1
go build -o bin/agent-harness ./cmd/harness
./scripts/install-native.sh
./bin/agent-harness bootstrap --skip-upstream-tools --dry-run
./bin/agent-harness update --skip-upstream-tools --dry-run
./scripts/install-native.sh --with-upstream-tools --dry-run
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
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --save-state --state-key self-verify-smoke --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness self-verify history --prefix self-verify --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness self-verify history --prefix self-verify --retention-limit 1 --prune-retention --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness self-verify compare --baseline-key self-verify-smoke --candidate-key self-verify-smoke --json
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness self-verify promote --from-key self-verify-smoke --baseline-key self-verify-baseline --json
./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --json
./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --progress=jsonl --json
./bin/agent-harness self-augment --cycles=1 --target-score=95 --json
./bin/agent-harness self-augment --cycles=1 --target-score=95 --save-state --state-key self-augment-latest --json
./bin/agent-harness self-augment lesson --candidate reflexion-state-memory --lesson "test lesson" --next-action "test next action" --state-key self-augment-lesson-test --json
grep -R "Conventional Commit\|Lore:" -n AGENTS.md .agent-harness/COMMIT_POLICY.md skills/atomic-commit-push/SKILL.md
```

확인할 것:

Native integration smoke:

```bash
test -f ~/.codex/skills/atomic-commit-push/SKILL.md
test -f ~/.claude/skills/atomic-commit-push/SKILL.md
codex mcp get agent_harness
claude mcp list | grep agent_harness
```

Optional upstream companion smoke:

```bash
./scripts/install-native.sh --with-upstream-tools --dry-run
codex plugin list | grep -E 'wiki@llm-wiki|claude-mem@claude-mem-local'
claude plugin list | grep -E 'wiki@llm-wiki|claude-mem@thedotmack'
command -v codegraph
codegraph status --json .  # after a real --with-upstream-tools run with HARNESS_INIT_CODEGRAPH enabled
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
go test ./cmd/harness -run Golden -update -count=1
go test ./internal/adapter -run TestNativeInstallAdapterContractMatrix -update-adapter-contract -count=1
```

---

## 3. 테스트 작성 기준

### Well-structured tests

- 변경된 공개 동작/계약을 직접 검증하고 구현 세부사항에 과도하게 묶이지 않는다.
- 실패 시 원인을 좁힐 수 있는 fixture 이름, assertion, 에러 메시지를 둔다.
- deterministic하며 test order, wall-clock sleep, real network, local machine state에 의존하지 않는다.
- regression test는 재발했던 입력, false case, 기대 결과를 명확히 담는다.
- 기존 helper와 style을 재사용하고, golden/snapshot 변경은 의도와 범위를 설명한다.

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
- `internal/adapter/testdata/native_install_contract_matrix.golden.json` — Codex/Claude user-global 기본 설치와 project-local opt-in 계약
- `agent-harness self-verify` 10회 반복 결과
- `agent-harness self-verify`의 `risk QA tier` step과 `risk_qa` goal score
- `agent-harness self-verify --json`의 `summary.contract`/`goal_scores`/`coverage_gaps`/`failure_class`/`rerun_commands`/`step_duration_stats` field
- `agent-harness self-verify candidates --json` candidate curriculum export and state save/read smoke
- `agent-harness self-verify compare` step budget p95 regression fixture for labels outside `slowest_steps`
- `agent-harness self-verify` install dry-run smoke for temp HOME/CODEX_HOME/HARNESS_ROOT no-write assertions
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

## 5. 완료 보고 기준

완료 보고에는 다음을 포함한다.

- 실제 실행한 검증 명령과 결과
- 실패가 있었다면 실패 원인과 수정/미해결 상태
- 실행하지 않은 검증과 이유(`Not-tested`)
- 변경 파일 요약과 남은 위험

## 자기 검증 QA gate

`agent-harness self-verify`는 테스트 실행뿐 아니라 QA gate를 포함한다. QA gate는 루프 문서, `GENIUS_THINK.md`, shared skill metadata, native integration 설치 상태, redaction audit, bounded stdout/stderr metadata, Mermaid 문서 lint를 확인하며, 모든 목표 점수가 95점을 초과해야 종료 가능하다. Mermaid lint는 `GENIUS_THINK.md`의 따옴표/`<br/>` 규칙을 기준으로 문서 다이어그램 파싱 오류를 조기에 막는다.

## LLM Wiki 정책

LLM Wiki 기능은 agent-harness가 직접 제공하지 않는다. 중복 구현을 피하기 위해 upstream `nvk/llm-wiki`의 Codex/Claude plugin 또는 portable AGENTS.md를 사용한다. 하네스 CLI/MCP에 llm-wiki 전용 명령, tool, resource, SessionStart hook을 추가하지 않는다.

같은 원칙으로 CodeGraph와 claude-mem도 하네스 내부에 재구현하지 않는다. `scripts/install-native.sh --with-upstream-tools`는 upstream installer/plugin을 연결하는 convenience path이며, 테스트는 하네스 core 기능이 아니라 설치 배선과 opt-in/dry-run 동작을 검증한다.

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

비즈니스 로직의 실제 404/403/409 가능성과 OpenAPI 누락 여부는 정적 테스트만으로 신뢰 있게 판정하지 않는다. 정적 테스트는 후보 파일 선택, `--all` wiring, prompt contract, MCP schema 같은 배선을 검증하고, 실제 API 문서 품질 판정은 `agent-harness api-doc review`/MCP `api_doc_static_check` 후 `api_doc_review`가 Codex 에이전트를 호출해 수행한다.

`nextcandle-api`에서 확인한 좋은 기준:

- `@ApiOperation.description`은 `### 목적`, `### 요청 규칙`/`### 처리 방식`, `### 권한/주의사항`처럼 Markdown section + bullet로 구성한다.
- path/query/header/body와 auth/tier/public 여부가 response 문서와 일치한다.
- service/usecase의 `NotFoundException`, `ForbiddenException`, `ConflictException` 등 public error가 endpoint response에 반영된다.
- public/admin Swagger document를 분리하고, 사용하지 않는 schema를 필터링해 client가 읽는 문서를 깔끔하게 유지한다.


## OpenAPI prompt source

Endpoint/controller/DTO/schema/OpenAPI 변경 시 `.agent-harness/OPEN_API_SPEC.md`를 프로젝트별 프롬프트 source로 사용한다. `agent-harness api-doc review`는 별도 `--prompt-file`이 없으면 이 문서를 자동으로 포함한다.


## Contract/audit/worker verification

CLI/MCP DTO를 변경할 때는 `agent-harness contract check --json`과 golden test를 실행해 command name, MCP tool name, required response field가 machine-visible하게 유지되는지 확인한다. policy audit 동작 변경은 JSONL record가 append-only이고 secret-like argument가 redacted 되는지 검증한다. 현재 worker 변경은 no-shell MVP 범위이며 process execution 없이 enqueue/status/list/cancel을 테스트해야 한다.

## Lifecycle state tests

- project lifecycle namespace tests must use `HARNESS_STATE_DIR` with `t.TempDir()` and must not write runtime state under the target repo.
- bootstrap tests should verify dry-run plans `projects/<repo-id>/project.json` without creating it, and `--write` creates it in user-state.
- hook tests should cover fallback behavior when lifecycle state is missing/corrupt so prompt routing remains useful.
- doctor tests should cover repo-local `.agent-harness/state/` and namespace mismatch warnings.
