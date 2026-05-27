# 테스트 컨벤션

이 문서는 `agent-harness`의 문서·코드 변경 검증 규칙이다.

---

## 1. 현재 문서 단계 검증

문서만 변경한 경우에도 최소한 다음을 확인한다.

```bash
find . -maxdepth 3 -type f | sort
find agent_docs -maxdepth 1 -type f -name '*.md' | sort
grep -R "외부 Go 하네스\|Go\|MCP\|Codex\|Claude" -n AGENTS.md CLAUDE.md agent_docs
python3 ${CODEX_HOME:-$HOME/.codex}/skills/.system/skill-creator/scripts/quick_validate.py skills/atomic-commit-push
python3 ${CODEX_HOME:-$HOME/.codex}/skills/.system/skill-creator/scripts/quick_validate.py skills/llm-wiki
go test ./... -count=1
go test ./cmd/harness -run Golden -count=1
go build -o bin/harness ./cmd/harness
./scripts/install-native.sh
./bin/harness install-native --json
./bin/harness install-native --dry-run --json
./bin/harness inspect --json
./bin/harness docs --json
./bin/harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
./bin/harness policy fake-run --workspace-root "$PWD" --cwd "$PWD" --write --json -- touch marker
./bin/harness llm-wiki inventory --json
./bin/harness llm-wiki session-context --json
./bin/harness llm-wiki search --query "llm wiki" --limit 5 --json
./bin/harness llm-wiki read --page 00-meta/index.md --json
tmp_state="$(mktemp -d)"
HARNESS_STATE_DIR="$tmp_state" ./bin/harness state write --key smoke --value "ok" --json
HARNESS_STATE_DIR="$tmp_state" ./bin/harness state read --key smoke --json
HARNESS_STATE_DIR="$tmp_state" ./bin/harness state list --json
HARNESS_STATE_DIR="$tmp_state" ./bin/harness state prune --max-age 720h --json
HARNESS_STATE_DIR="$tmp_state" ./bin/harness state doctor --json
HARNESS_STATE_DIR="$tmp_state" ./bin/harness state migrate --json
HARNESS_DAEMON_DIR="$tmp_state/daemon" ./bin/harness daemon status --json
HARNESS_DAEMON_DIR="$tmp_state/daemon" ./bin/harness daemon start --json
HARNESS_DAEMON_DIR="$tmp_state/daemon" ./bin/harness daemon stop --json
HARNESS_STATE_DIR="$tmp_state" ./bin/harness self-verify --iterations=10 --seed=100 --target-score=95 --save-state --state-key self-verify-smoke --json
HARNESS_STATE_DIR="$tmp_state" ./bin/harness self-verify history --prefix self-verify --json
HARNESS_STATE_DIR="$tmp_state" ./bin/harness self-verify history --prefix self-verify --retention-limit 1 --prune-retention --json
HARNESS_STATE_DIR="$tmp_state" ./bin/harness self-verify compare --baseline-key self-verify-smoke --candidate-key self-verify-smoke --json
HARNESS_STATE_DIR="$tmp_state" ./bin/harness self-verify promote --from-key self-verify-smoke --baseline-key self-verify-baseline --json
./bin/harness self-verify --iterations=10 --seed=100 --target-score=95 --json
./bin/harness self-verify --iterations=10 --seed=100 --target-score=95 --progress=jsonl --json
./bin/harness self-augment --cycles=1 --target-score=95 --json
./bin/harness self-augment --cycles=1 --target-score=95 --save-state --state-key self-augment-latest --json
./bin/harness self-augment lesson --candidate reflexion-state-memory --lesson "test lesson" --next-action "test next action" --state-key self-augment-lesson-test --json
grep -R "Conventional Commit\|Lore:" -n AGENTS.md agent_docs/COMMIT_POLICY.md skills/atomic-commit-push/SKILL.md
```

확인할 것:

Native integration smoke:

```bash
test -f ~/.codex/skills/atomic-commit-push/SKILL.md
test -f ~/.codex/skills/llm-wiki/SKILL.md
test -f ~/.claude/skills/atomic-commit-push/SKILL.md
test -f ~/.claude/skills/llm-wiki/SKILL.md
grep -q "session-start-llm-wiki.sh" ~/.claude/settings.json
codex mcp get agent_harness
claude mcp list | grep agent-harness
```


- `AGENTS.md`와 `CLAUDE.md`가 같은 source of truth를 가리키는가
- `agent_docs/`의 링크가 실제 파일과 맞는가
- plugin vs worker 결정과 Go 선택이 여러 문서에서 충돌하지 않는가
- shared skill 원본(`skills/*`)과 user-level host 연결(`~/.codex/skills/*`, `~/.claude/skills/*`)이 drift 없이 같은 대상을 가리키는가
- llm-wiki root(`~/workspace/knowledge-base/llm-wiki`)가 존재하고 `00-meta/AGENTS.md`, `00-meta/index.md`, `00-meta/log.md`가 읽히는가
- 커밋 정책이 `AGENTS.md`, `agent_docs/COMMIT_POLICY.md`, `atomic-commit-push` skill에서 충돌하지 않는가

---

## 2. Go 코드 변경 기본 검증

Go 코드가 추가되면 기본 검증은 다음이다. `self-verify`는 working tree risk를 보고 `risk QA tier`에서 `go vet ./...` 또는 `go test -race ./... -count=1`를 조건부 실행한다.

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/harness ./cmd/harness
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

- `harness inspect --json` output shape
- `harness docs --json` output shape
- `harness policy check/fake-run` allow/deny/fake execution output shape
- MCP tool schema와 response shape
- daemon-backed MCP smoke response
- `harness llm-wiki inventory/session-context/search/read` output shape
- `cmd/harness/testdata/usage.golden.txt`
- `cmd/harness/testdata/mcp_tools.golden.json`
- `cmd/harness/testdata/mcp_resources.golden.json`
- `cmd/harness/testdata/response_contracts.golden.json`
- `internal/adapter/testdata/native_install_contract_matrix.golden.json` — Codex/Claude user-global 기본 설치와 project-local opt-in 계약
- `harness self-verify` 10회 반복 결과
- `harness self-verify`의 `risk QA tier` step과 `risk_qa` goal score
- `harness self-verify --json`의 `summary.contract`/`goal_scores`/`coverage_gaps`/`failure_class`/`rerun_commands`/`step_duration_stats` field
- `harness self-verify candidates --json` candidate curriculum export and state save/read smoke
- `harness self-verify compare` step budget p95 regression fixture for labels outside `slowest_steps`
- `harness self-verify` install dry-run smoke for temp HOME/CODEX_HOME/HARNESS_ROOT no-write assertions
- `harness self-verify --save-state` summary checkpoint serialization
- `harness self-verify history` summary checkpoint discovery and retention dry-run/confirm safety
- `harness self-verify` llm-wiki fixture guard step for temp fixture root and isolated HOME default-root leakage
- `harness self-verify` parallel isolation step for temp state, daemon dir, llm-wiki fixture, and artifact path collisions
- `harness self-verify` native integration fixture for Claude MCP conflicting-scope warning classification
- `harness self-verify` daemon resilience step for stale lock/socket recovery and socket permission checks
- `harness self-verify compare` summary checkpoint regression comparison
- `harness self-verify promote` dry-run/confirm baseline promotion
- `harness self-augment --json` planner/candidate curriculum
- `harness inspect/docs/preflight/policy/state` 실제 JSON response normalization 결과
- `harness state write/read/list/prune/doctor/migrate` output shape
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

`harness self-verify`는 테스트 실행뿐 아니라 QA gate를 포함한다. QA gate는 루프 문서, `GENIUS_THINK.md`, shared skill metadata, native integration 설치 상태, redaction audit, bounded stdout/stderr metadata, Mermaid 문서 lint를 확인하며, 모든 목표 점수가 95점을 초과해야 종료 가능하다. Mermaid lint는 `GENIUS_THINK.md`의 따옴표/`<br/>` 규칙을 기준으로 문서 다이어그램 파싱 오류를 조기에 막는다.
