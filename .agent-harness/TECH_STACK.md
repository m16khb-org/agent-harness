---
name: TECH_STACK.md
description: Chosen languages, runtimes, tools, and rationale.
---

# 기술 스택

현재 저장소는 Go module과 `cmd/harness` 기반 CLI/MCP MVP가 구현된 상태다. 아래는 계속 유지할 기술 기준이다.

---

## 1. 언어 선택

| 후보 | 장점 | 단점 | 판단 |
|------|------|------|------|
| Go | 단일 바이너리 배포, 빠른 컴파일, goroutine 기반 동시성, CLI/daemon/MCP 구현 생산성, 현재 로컬 `go1.26.3` 확인 | Rust보다 메모리 안전성의 정적 보장이 약함 | **채택** |
| Rust | 강한 메모리 안전성, 고성능, 단일 바이너리 | 러닝커브와 구현 속도 비용, 현재 로컬 toolchain 확인 안 됨 | 추후 sandbox/security critical component에만 재검토 |

결론: 개인 하네스는 빠른 반복과 host integration 생산성이 중요하므로 **Go**로 시작한다. untrusted code sandbox나 고위험 parser가 필요해지면 해당 component만 Rust를 재검토한다.

---

## 2. 런타임 / 패키지 관리

| 항목 | 기준 |
|------|------|
| 언어 | Go |
| 로컬 확인 toolchain | `go version go1.26.3 darwin/arm64` |
| 패키지 관리 | Go modules |
| 기본 바이너리 | `bin/agent-harness` (`cmd/harness` source) |
| 실행 모드 | CLI one-shot, MCP stdio proxy, user-level daemon, future local job worker |
| 설정 prefix | `HARNESS_` |

## 2.1 External tools are not dependencies

아래 도구들은 agent-harness core dependency가 아니다. 하네스 설치, 업데이트, self-verify, readiness gate는 이 도구들이 없어도 재현 가능해야 한다. 기능을 하네스에 복제하지 않고, 필요한 경우 사용자가 각 도구의 공식 설치/설정 경로를 별도로 따른다.

| 도구 | Package/source | 역할 | 하네스 경계 |
|------|------------------|------|-----------|
| LLM Wiki | `m16khb/llm-wiki` | OKF-native local wiki validation, linting, indexing, graphing, and bounded query-pack MCP tools | 하네스는 외부 wiki ingest/lint/index/query-pack을 실행하지 않는다. |
| CodeGraph | `@colbymchenry/codegraph` | AST 기반 symbol graph와 MCP code intelligence | 하네스 readiness는 CodeGraph 설치를 요구하지 않는다. 사용 가능하면 명시 evidence로만 기록한다. |
| claude-mem | `thedotmack/claude-mem` | session memory capture/compression | 하네스는 memory capture/compression/store logic이나 hook 설치를 대행하지 않는다. |
| LazyCodex | `code-yeongyu/oh-my-openagent` / `lazycodex-ai` | Codex Light LazyCodex/OMO skills, hooks, LSP/AST tooling | 하네스는 해당 skill/hook/tool 동작을 core나 설치 경로에 복제하지 않는다. |

These tools are not installed by `agent-harness install`, `bootstrap`, `update`, or `scripts/install-native.sh`.

## 2.2 Project skills

agent-harness의 `skills/` 디렉토리에는 **19개** 스킬이 있다(`ls skills/` 기준). 두 부류로
나뉘며 분해 합계는 **pioneer-namesake 11 + operational 8 = 19**이다. 상세한 namesake 설명은
`README.md` "Skills & Their Namesakes" 표를 참조한다.

**Pioneer-namesake (11)** — 컴퓨터 과학 선구자의 이름을 딴 language/tech agnostic 스킬:

| 스킬 | 역할 |
|------|------|
| `berners-lee` | Web Research — 다중 소스 출처 인용 조사 |
| `brooks` | Devil's-advocate design/plan critic — 구현 전 계획 적대 검증 |
| `codd` | Database Design & Optimization |
| `dijkstra` | Algorithm Design & Complexity Optimization |
| `engelbart` | Meeting-record augmentation / team-memory |
| `hopper` | Systematic Debugging — 과학적 디버깅 |
| `karpathy` | Prompt Engineering & Optimization |
| `shannon` | Signal-to-Noise Quality Measurement |
| `torvalds` | Git Operations — rebase, bisect, conflict, reflog |
| `turing` | Evidence-Bound Execution — 증거 기반 목표 실행 |
| `von-neumann` | Strategic Planning — decision-complete 계획 수립 |

**Operational (8)** — 하네스 운영 workflow 스킬(설계상 harness-specific):

| 스킬 | 역할 |
|------|------|
| `atomic-commit-push` | Torvalds sub-skill — 안전한 atomic 커밋/푸시 |
| `draft-wiki-promoter` | draft-wiki 후보 판정·승격 |
| `gitlab-usecase` | GitLab/glab/IssueOps remote 가이드 |
| `issueops` | Issue-Driven Work Cycle Router |
| `project-bootstrap` | repo-local AGENTS.md + `.agent-harness/` 문서 생성 |
| `self-augment` | 자가 개선 루프(95점 게이트) |
| `self-verify` | 자가 검증 루프(95점 게이트) |
| `stability-audit` | 설치/안정성 전수조사 |

모든 스킬은 `scripts/install-native.sh`를 통해 사용자 홈(`~/.codex/skills/`, `~/.claude/skills/`)에
symlink로 설치된다. Pioneer-namesake 11개는 language/tech agnostic을 원칙으로 하고(6f31c55 검증 완료),
operational 8개는 agent-harness 운영에 특화되어 있다. 스킬 mesh는 hub-and-spoke cross-reference(
`turing`·`issueops`가 hub)를 형성한다.

---

## 3. 확정 라이브러리

핵심 dependency는 구현되어 확정되었다. 아래는 `go.mod`와 실제 import 기준의 확정값이다.

| 영역 | 확정 | 근거 |
|------|------|------|
| CLI | 표준 `flag` | `cmd/harness/**`의 63개 파일이 stdlib `flag` 사용; Cobra는 도입하지 않음 |
| Config/State 직렬화 | 표준 `encoding/json` | 설정·상태는 JSON으로 직렬화; 외부 config 라이브러리(yaml.v3/toml)는 의존성에 없음 |
| Logging | 표준 `log/slog` | secret redaction은 host 어댑터 계층에서 처리 |
| MCP | `github.com/modelcontextprotocol/go-sdk` v1.6.1 | daemon socket transport의 기본 SDK. 분리 reader/writer stdio smoke를 위한 legacy JSON-RPC 경로를 병행 유지(ADR "MCP go-sdk 채택" 참조) |
| IPC | Unix socket | MCP proxy daemon은 Unix socket 사용. localhost HTTP는 future worker 필요 시 검토 |
| State 저장 | SQLite (`modernc.org/sqlite`, pure Go) | `HARNESS_STATE_DIR` 또는 `~/.local/state/agent-harness/`; state root마다 `harness.db`(WAL, records(bucket,id,data) JSON blob) + `harness.lock.db`(BEGIN IMMEDIATE span lock). 동시성은 per-root sqlstore span으로 직렬화 |
| Testing | 표준 `testing`, golden file, `net/http/httptest` | 외부 agent host 없이 core contract 검증; `httptest`는 13개 파일에서 사용 중(대부분 `*_test.go`) |

직접 의존성(`go.mod`): `golang.org/x/term`, `golang.org/x/sys`, `github.com/modelcontextprotocol/go-sdk` v1.6.1.

---

## 4. 예정 명령어

현재 검증/빌드:

```bash
go test ./... -count=1
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness inspect --json
./bin/agent-harness status --json
./bin/agent-harness docs --json
./bin/agent-harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
./bin/agent-harness policy run --read-only --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
./bin/agent-harness worker run --read-only --kind smoke --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
./bin/agent-harness verify-work --json -- git status --short
./bin/agent-harness daemon status --json
./bin/agent-harness self-verify --seed=100 --target-score=95 --json
./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --save-state --state-key self-verify-latest --json
./bin/agent-harness self-verify history --prefix self-verify --json
./bin/agent-harness self-verify compare --baseline-key self-verify-baseline --candidate-key self-verify-latest --json
./bin/agent-harness self-verify promote --from-key self-verify-latest --baseline-key self-verify-baseline --confirm --json
./bin/agent-harness self-augment --cycles=1 --target-score=95 --json
./scripts/install-native.sh
./bin/agent-harness bootstrap --dry-run
./scripts/install-native.sh --skip-build
./bin/agent-harness install-native --json
./bin/agent-harness install-native --dry-run --json
```

예정 사용 예:

```bash
agent-harness inspect --json
agent-harness docs --json
agent-harness status --json
agent-harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
agent-harness policy run --read-only --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
agent-harness worker run --read-only --kind smoke --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
agent-harness verify-work --json -- git status --short
agent-harness policy fake-run --workspace-root "$PWD" --cwd "$PWD" --write --json -- touch marker
harness state write --key checkpoint --input checkpoint.json --json
harness state read --key checkpoint --json
harness state list --json
harness state prune --max-age 720h --json
harness state prune --max-age 720h --confirm --json
harness state doctor --json
harness state migrate --json
harness state migrate --confirm --json
agent-harness daemon start --json
agent-harness daemon status --json
agent-harness daemon stop --json
agent-harness mcp
agent-harness self-verify --seed=100 --target-score=95
agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --save-state --state-key self-verify-latest --json
agent-harness self-verify history --prefix self-verify --json
agent-harness self-verify compare --baseline-key self-verify-baseline --candidate-key self-verify-latest --json
agent-harness self-verify promote --from-key self-verify-latest --baseline-key self-verify-baseline --confirm --json
agent-harness self-augment --cycles=1 --target-score=95 --json
agent-harness worker start
```

---

## 5. 주요 설정/상태 위치

| 종류 | 위치 |
|------|------|
| 루트 규칙 | `AGENTS.md`, `CLAUDE.md` |
| 에이전트 문서 | `.agent-harness/` |
| 사용자 설정 | `~/.config/agent-harness/config.yaml` |
| 사용자 state/log | OS별 state dir 또는 `~/.local/state/agent-harness/` |
| workspace cache | `.harness/` |
| daemon socket/pid/log | `~/.local/state/agent-harness/daemon/` 또는 `HARNESS_DAEMON_DIR` |
| Codex 템플릿 | `configs/codex/` |
| Claude 템플릿 | `configs/claude/` |

---

## Native integration 확인

```bash
test -f ~/.codex/skills/atomic-commit-push/SKILL.md
test -f ~/.claude/skills/atomic-commit-push/SKILL.md
codex mcp get agent_harness
claude mcp list | grep agent_harness
```


## 구현된 hardening commands

```bash
agent-harness contract schema --json
agent-harness contract check --json
agent-harness policy audit --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
agent-harness worker enqueue --kind smoke --payload "..." --json
agent-harness worker status --id JOB_ID --json
agent-harness worker list --json
agent-harness worker cancel --id JOB_ID --json
agent-harness worker run --read-only --kind smoke --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
```

이번 phase의 worker command는 기본 job lifecycle에 policy-backed read-only execution을 추가한다. write/network/arbitrary shell/background worker는 아직 열지 않는다.
