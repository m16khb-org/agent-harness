---
name: TECH_STACK.md
description: Chosen languages, runtimes, tools, and rationale.
---

# 기술 스택

현재 저장소는 Go module과 `cmd/harness` 기반 0.1.0 v0 표면(CLI, MCP, daemon, IssueOps, one-shot worker, native host integration)이 구현된 상태다.

---

## 1. 언어 선택

| 후보 | 장점 | 단점 | 판단 |
|------|------|------|------|
| Go | 단일 바이너리 배포, 빠른 컴파일, goroutine 기반 동시성, CLI/daemon/MCP 구현 생산성, 현재 로컬 `go1.26.3` 확인 | Rust보다 메모리 안전성의 정적 보장이 약함 | **채택** |
| Rust | 강한 메모리 안전성, 고성능, 단일 바이너리 | 러닝커브와 구현 속도 비용, 현재 로컬 toolchain 확인 안 됨 | 추후 sandbox/security critical component에만 재검토 |

빠른 반복과 host integration 생산성을 위해 **Go**를 채택한다. untrusted code sandbox나 고위험 parser가 필요해지면 해당 component만 Rust를 재검토한다.

---

## 2. 런타임 / 패키지 관리

| 항목 | 기준 |
|------|------|
| 언어 | Go |
| 로컬 확인 toolchain | `go version go1.26.3 darwin/arm64` |
| 패키지 관리 | Go modules |
| 기본 바이너리 | `bin/agent-harness` (`cmd/harness` source) |
| 실행 모드 | CLI one-shot, MCP stdio proxy, user-level daemon, state-first one-shot worker jobs |
| 설정 prefix | `HARNESS_` |

## 2.1 Core independence and optional upstream provisioning

`agent-harness` core는 외부 companion 도구에 의존하지 않는다. Native install, readiness, self-verify, CLI/MCP 계약은 외부 계정·키·도구 없이 재현되어야 하며, 전문 기능은 core에 복제하지 않는다.

다만 `configs/upstream.json`은 native activation 후 실행하는 **선택적** provisioning catalog다. 현재 v0는 Claude Code용 plugin 4개와 Git skill 1개를 선언하며, 없는 항목만 Claude CLI 또는 shallow sparse clone으로 준비한다. dry-run에는 이 계획을 표시한다. network·host CLI 실패는 `upstream ...` 메시지로 보고하지만 native install은 실패시키지 않는다. 이 제한된 adapter는 외부 도구를 core/readiness dependency로 만들지 않는다.

위키, 코드 인텔리전스, 세션 메모리, 서드파티 toolchain은 사용자가 각 도구의 공식 경로로 별도 설치한다. `agent-harness`는 명시적 파일, command output, 이미 구성된 MCP처럼 검증 가능한 경계만 소비한다.

### Optional Orca boundary

Orca는 `exec.CommandContext`로 설치된 CLI를 호출하는 선택적 IssueOps adapter다. `internal/adapter/orca`는 JSON status/repo/worktree/terminal/task/dispatch projection, bounded timeout, stdout/stderr 분리, redacted error를 담당한다. Orca를 Go module dependency, native installer target, daemon plugin, generic driver registry로 추가하지 않는다. Orca가 없어도 core/CLI/MCP/native install/self-verify는 독립적으로 동작하며, IssueOps `auto` 모드는 mutation 전 probe 실패에만 기존 inline worktree 계약을 반환한다.

## 2.2 Project skills

`skills/`에는 현재 **34개** shared skill이 있다. 현재 inventory는 `agent-harness inspect --json`의 `skills` 배열과 각 `skills/<name>/SKILL.md`로 확인할 수 있으며, 분해 합계는 **pioneer-namesake 12 + operational 22 = 34**이다.

**Pioneer-namesake (12)** — 컴퓨터 과학 선구자의 이름을 딴 language/tech agnostic 스킬:

| 스킬 | 역할 |
|------|------|
| `berners-lee` | Web Research — 다중 소스 출처 인용 조사 |
| `boehm` | Risk-driven planning-document analysis — Kordoc·OCR·시각 증거 조정 |
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

**Operational (22)** — host·workflow·문서·QA 운영 스킬:

| 범주 | 스킬 |
|------|------|
| 협업 | `slack-delegate`, `sharing-backend-work` |
| Git / IssueOps | `atomic-commit-push`, `gitlab-usecase`, `issueops`, `issueops-prepare`, `issueops-cleanup` |
| Project docs | `project-bootstrap`, `project-docs-bootstrap`, `project-docs-update`, `project-docs-optimize` |
| Browser QA | `aside-functional-qa`, `aside-visual-qa`, `aside-web-qa`, `read-public-artifact` |
| Code review | `parnas`, `review-agent-feedback` |
| 운영 개선 | `self-verify`, `self-augment`, `stability-audit` |
| 작성·시각화 | `fluent-korean`, `diagram-design` |

기본 설치는 같은 `skills/` 원본을 `~/.codex/skills/`, `~/.claude/skills/`, `~/.omo/agent/skills/`에 연결한다. repo-local skill link는 `--project-local`에서만 생성하고, upstream catalog에서 받은 외부 skill cache는 이 34개 원본과 분리한다. trigger와 사용 계약은 각 `SKILL.md`가 정의한다.

---

## 3. 확정 라이브러리

핵심 dependency는 `go.mod`와 실제 import를 기준으로 확정했다.

| 영역 | 확정 | 근거 |
|------|------|------|
| CLI | 표준 `flag` | `cmd/harness` CLI와 command package에서 stdlib `flag` 사용; Cobra는 도입하지 않음 |
| Config/State 직렬화 | 표준 `encoding/json` | 설정·상태는 JSON으로 직렬화; 외부 config 라이브러리(yaml.v3/toml)는 의존성에 없음 |
| Logging | 표준 `log/slog` | secret redaction은 host 어댑터 계층에서 처리 |
| MCP | `github.com/modelcontextprotocol/go-sdk` v1.6.1 | daemon socket transport의 기본 SDK. 분리 reader/writer stdio smoke를 위한 legacy JSON-RPC 경로를 병행 유지(ADR "MCP go-sdk 채택" 참조) |
| IPC | Unix socket | MCP proxy daemon은 Unix socket 사용. localhost HTTP는 future worker 필요 시 검토 |
| State 저장 | SQLite (`modernc.org/sqlite`, pure Go) | `HARNESS_STATE_DIR` 또는 `~/.local/state/agent-harness/`; state root마다 `harness.db`(WAL, records(bucket,id,data) JSON blob) + `harness.lock.db`(BEGIN IMMEDIATE span lock). 동시성은 per-root sqlstore span으로 직렬화 |
| Testing | 표준 `testing`, golden file, `net/http/httptest` | 외부 agent host 없이 core contract를 검증하며 HTTP boundary 격리에만 `httptest` 사용 |

직접 의존성(`go.mod`): `golang.org/x/term` v0.43.0, `golang.org/x/sys` v0.44.0, `modernc.org/sqlite` v1.53.0, `github.com/modelcontextprotocol/go-sdk` v1.6.1.

---

## 4. 명령어

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
./bin/agent-harness self-verify --seed=100 --target-score=95 --save-state --state-key self-verify-latest --json
./bin/agent-harness self-verify history --prefix self-verify --json
./bin/agent-harness self-verify compare --baseline-key self-verify-baseline --candidate-key self-verify-latest --json
./bin/agent-harness self-verify promote --from-key self-verify-latest --baseline-key self-verify-baseline --confirm --json
./bin/agent-harness self-augment --cycles=1 --target-score=95 --json
./scripts/install-native.sh
./bin/agent-harness bootstrap --dry-run
./scripts/install-native.sh --skip-build
./bin/agent-harness install --json
./bin/agent-harness install --dry-run --json
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
agent-harness state write --key checkpoint --input checkpoint.json --json
agent-harness state read --key checkpoint --json
agent-harness state list --json
agent-harness state prune --max-age 720h --json
agent-harness state prune --max-age 720h --confirm --json
agent-harness state doctor --json
agent-harness state maintain --json
agent-harness daemon start --json
agent-harness daemon status --json
agent-harness daemon stop --json
agent-harness mcp
agent-harness self-verify --seed=100 --target-score=95
agent-harness self-verify --seed=100 --target-score=95 --save-state --state-key self-verify-latest --json
agent-harness self-verify history --prefix self-verify --json
agent-harness self-verify compare --baseline-key self-verify-baseline --candidate-key self-verify-latest --json
agent-harness self-verify promote --from-key self-verify-latest --baseline-key self-verify-baseline --confirm --json
agent-harness self-augment --cycles=1 --target-score=95 --json
agent-harness worker list --json
```

---

## 5. 주요 설정/상태 위치

| 종류 | 위치 |
|------|------|
| 루트 규칙 | `AGENTS.md`, `CLAUDE.md` |
| 에이전트 문서 | `.agent-harness/` |
| 사용자 설정 | 현재는 `HARNESS_*` env와 generated host config를 사용하며, `~/.config/agent-harness/config.yaml` loader는 계획 상태 |
| 사용자 state/log | OS별 state dir 또는 `~/.local/state/agent-harness/` |
| workspace cache | `.harness/`는 예약 경로이며 현재 생성하거나 읽지 않음 |
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

이번 phase의 worker command는 기본 job lifecycle에 policy-backed read-only execution만 추가한다. write, network, arbitrary shell, background worker는 아직 지원하지 않는다.
