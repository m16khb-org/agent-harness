# agent-harness

<p align="center">
  <a href="#english">English</a> ·
  <a href="#한국어">한국어</a>
</p>

<a id="english"></a>

## English

<p align="right"><a href="#한국어">한국어로 보기</a></p>

<p align="center">
  <img src="docs/assets/agent-harness-hero.png" alt="agent-harness hero illustration: one Go harness core connecting Codex and Claude Code through CLI, MCP, policy, state, and skills" width="100%" />
</p>

**agent-harness** is a personal automation harness for AI coding agents. It gives **Codex** and **Claude Code** the same local Go binary, the same MCP tools, the same command-policy checks, and the same shared skill source tree.

The project is intentionally not “just a Codex plugin” or “just Claude commands.” The reusable behavior lives in a host-neutral Go core; host integrations are thin adapters that call that core.

The project philosophy is **do not reinvent the wheel**: agent-harness owns the small shared core for orchestration, policy, state, project docs, and install glue. Specialized knowledge/retrieval tools stay upstream — for example `nvk/llm-wiki`, `colbymchenry/codegraph`, and `thedotmack/claude-mem` are installed or configured as optional dependencies instead of being reimplemented inside the harness.

> Status: early but functional MVP. The CLI, daemon-backed MCP proxy, policy checker, state checkpoints, project-doc tools, API-doc review gate, native skill installer, self-verification loop, and self-augmentation loop are implemented. The worker surface is currently **state-only / no-shell** by design.

---

## Why this exists

AI coding agents become hard to trust when every host has different prompts, tools, state, and safety rules. `agent-harness` keeps those concerns in one portable place.

Use it when you want to:

- run the same agent workflow from Codex, Claude Code, MCP, or a shell;
- keep shared skills in one `skills/` source of truth instead of copying them per host;
- expose repo operating docs to agents in a structured way;
- check command safety before execution rather than letting agents run arbitrary shell strings;
- store small, inspectable agent checkpoints outside the repository;
- continuously verify and improve the harness itself.

## What you get

| Area | Commands / files | What it does |
| --- | --- | --- |
| Install and inspection | `install-native`, `inspect`, `preflight`, `version` | Build and connect user-level Codex/Claude integrations, then inspect the installation. |
| MCP backend | `agent-harness mcp`, `daemon start/status/stop` | Run a stdio MCP proxy backed by a user-level daemon so Codex and Claude see the same tools. |
| Command policy | `policy check`, `policy fake-run`, `policy audit` | Evaluate argv, workspace root, cwd, timeout, write/network intent, and audit metadata without a real shell runner. |
| State checkpoints | `state write/read/list/prune/doctor/migrate` | Store small JSON checkpoints in user state, not tracked repo files. |
| Project docs | `project bootstrap/docs/route-docs`; MCP `project_docs_record` | Generate, index, route, and append project operating docs under `.agent-harness/`. |
| API docs gate | `api-doc check/static-check/review` | Catch endpoint/DTO/OpenAPI documentation drift. |
| Shared skills | `skills/atomic-commit-push`, `skills/project-bootstrap`, `skills/self-verify`, `skills/self-augment` | Codex and Claude Code use the same skill definitions. |
| Self-improvement | `self-verify`, `self-augment` | Run a 95-point verification gate and select safe improvement candidates. |
| Worker MVP | `worker enqueue/status/list/cancel` | Record job lifecycle state only; it does not execute shell commands yet. |

## Architecture

```mermaid
flowchart LR
    Codex[Codex\nAGENTS.md · skills · MCP] --> MCP[agent-harness mcp\nstdio proxy]
    Claude[Claude Code\nCLAUDE.md · skills · MCP] --> MCP
    Human[Human shell] --> CLI[harness CLI]

    MCP --> Daemon[agent-harness daemon\nuser-level Unix socket]
    CLI --> Core[Go core use cases\npolicy · docs · state · install]
    Daemon --> Core
    Core --> Ports[ports / DTOs]
    Ports --> FS[fs · git adapters]
    Ports --> State[state · audit log]
    Core -. future .-> Worker[local job worker\nqueue · watch · long tasks]
```

Design rules:

1. **Host-neutral core first** — core behavior belongs in Go, not in host-specific plugin code.
2. **Same contract everywhere** — CLI JSON, MCP responses, and daemon responses must mean the same thing.
3. **Safe by default** — command policy, workspace boundaries, audit records, redaction, and dry-run/default no-shell behavior come first.
4. **One skill source** — `skills/<name>/` is the source of truth; user-level Codex/Claude skill paths point back to it.
5. **Incremental worker** — persistent worker functionality starts with lifecycle state before process execution.
6. **Do not reinvent the wheel** — integrate upstream tools such as llm-wiki, CodeGraph, and claude-mem through their own installers/plugins; do not copy their core behavior into agent-harness.

## Repository map

```text
cmd/harness/              Go binary entrypoint and CLI/MCP/daemon commands
internal/core/            Host-neutral use cases: docs, policy, state, install, worker contracts
internal/port/            Core-facing interfaces and DTOs
internal/adapter/         Host/install adapter tests and integration boundaries
configs/codex/            Codex MCP and hook templates
configs/claude/           Claude MCP template
skills/                   Shared native skills used by Codex and Claude Code
.agent-harness/           Project operating docs, ADRs, cautions, testing rules
scripts/install-native.sh Native install convenience script
bin/agent-harness               Locally built binary
```

## Requirements

- Go toolchain available on your machine.
- Codex and/or Claude Code if you want native host integration.
- A Unix-like local environment for the current daemon/socket implementation.

The project currently has no external Go module dependencies beyond the standard library; check the current tree before assuming additional dependencies.

## Quick start

From the repository root:

```bash
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness version
./bin/agent-harness inspect --json
./bin/agent-harness docs --json
```

Check command policy without executing the command:

```bash
./bin/agent-harness policy check \
  --workspace-root "$PWD" \
  --cwd "$PWD" \
  --json -- git status --short
```

Smoke-test the daemon lifecycle:

```bash
./bin/agent-harness daemon status --json
./bin/agent-harness daemon start --json
./bin/agent-harness daemon status --json
./bin/agent-harness daemon stop --json
```

Install or update user-level Codex/Claude integration after reviewing the dry run. First-time setup should use `agent-harness bootstrap`; later refreshes should use `agent-harness update`, like `claude-mem update`. Both commands use the recommended full path by default: they rebuild this checkout, refresh host integrations, and install/update upstream companion tools. The installer also creates/refreshes `~/.local/bin/agent-harness`, so new shells can run `agent-harness ...` from anywhere:

```bash
./bin/agent-harness install-native --dry-run --json

# Recommended first-time full setup.
./bin/agent-harness bootstrap

# Recommended ongoing update, like `claude-mem update`.
agent-harness update

# Minimal/low-level path: update only agent-harness native Codex/Claude integration.
./scripts/install-native.sh --skip-upstream-tools
./bin/agent-harness install-native --json
```

`agent-harness update` and `agent-harness bootstrap` delegate to `./scripts/install-native.sh --with-upstream-tools`. The script rebuilds `bin/agent-harness` from the current checkout every run, refreshes `~/.local/bin/agent-harness`, and then updates host integrations, so an existing install is updated in place. It does not run `git pull`; update the checkout yourself first when you want remote changes. Use `--skip-build` only when you intentionally want to leave the existing binary unchanged, and `--skip-upstream-tools` only for a minimal harness-only refresh.

Default installation targets user-level host locations. It must not create project-local `.claude/skills`, `.claude/settings.json`, or `.mcp.json` files in a target repository unless project-local mode is explicitly requested.

`--with-upstream-tools` is the recommended full setup for this harness philosophy: do not reinvent the wheel, and keep specialized capabilities on their upstream implementations. It modifies user-level Codex/Claude/plugin/MCP configuration and may use the network. It wires these upstream tools without vendoring or reimplementing them:

| Tool | Upstream | What the installer does |
| --- | --- | --- |
| LLM Wiki | `nvk/llm-wiki` | Adds/updates the Codex and Claude `wiki@llm-wiki` plugin. |
| CodeGraph | `colbymchenry/codegraph` | Installs `@colbymchenry/codegraph`, registers its MCP server for Codex/Claude, and initializes this repo's `.codegraph/` index when enabled. |
| claude-mem | `thedotmack/claude-mem` | Adds/updates the Codex and Claude `claude-mem` plugin. |

Set `HARNESS_INSTALL_UPSTREAM_TOOLS=1` for the same behavior, or `HARNESS_INIT_CODEGRAPH=0` to skip local CodeGraph indexing.

## Common commands

```bash
# Installation and environment
./bin/agent-harness inspect --json
./bin/agent-harness preflight --json "$PWD"
./bin/agent-harness contract check --json

# Docs and routing
./bin/agent-harness docs --json
./bin/agent-harness project route-docs --repo "$PWD" --task "update command policy" --json

# State checkpoints
./bin/agent-harness state write --key checkpoint --value "ready" --json
./bin/agent-harness state read --key checkpoint --json
./bin/agent-harness state doctor --json

# Policy-only command handling: no real shell runner
./bin/agent-harness policy fake-run --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
./bin/agent-harness policy audit --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short

# Worker MVP: state only, no shell execution
./bin/agent-harness worker enqueue --kind smoke --payload "hello" --json
./bin/agent-harness worker list --json

# API documentation gate
./bin/agent-harness api-doc check --json
./bin/agent-harness api-doc review --json

# Harness verification and improvement loops
./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --json
./bin/agent-harness self-augment --cycles=1 --target-score=95 --json
```

## Native host integration

### Codex

`install-native` links shared skills into user-level Codex skill paths and registers the MCP server/hook configuration.

Useful checks:

```bash
test -f ~/.codex/skills/atomic-commit-push/SKILL.md && echo "Codex skill linked"
codex mcp get agent_harness
```

### Claude Code

`install-native` links the same shared skills into user-level Claude skill paths and registers a user-scope MCP server.

Useful checks:

```bash
test -f ~/.claude/skills/atomic-commit-push/SKILL.md && echo "Claude skill linked"
claude mcp list
```

## Shared skills

| Skill | Purpose |
| --- | --- |
| `atomic-commit-push` | Review local changes, split focused commits, and push safely with a Conventional Commit subject plus Lore body. |
| `project-bootstrap` | Generate or update repo-local agent operating docs from repository evidence. |
| `self-verify` | Run or interpret the harness 95-point verification loop. |
| `self-augment` | Choose one safe high-value harness improvement, implement it, and verify it. |

## Safety model

- Secret values must not appear in prompts, docs, logs, fixtures, CLI JSON, or MCP responses.
- Host adapters must not bypass core policy.
- Policy commands reason about argv form, workspace root, cwd, write/network intent, timeout, and audit metadata.
- Current policy/fake-run/audit commands do not provide a general shell execution capability.
- Worker commands are intentionally no-shell lifecycle records until queueing, cancellation, redaction, and audit boundaries are hardened.
- Runtime state belongs in user state directories or ignored workspace state, not committed source files.

## Development

Recommended baseline:

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness inspect --json
./bin/agent-harness docs --json
./bin/agent-harness contract check --json
```

For documentation-only changes, at minimum check file paths, buildability, and the docs index:

```bash
find . -maxdepth 3 -type f | sort
find .agent-harness -maxdepth 1 -type f -name '*.md' | sort
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness docs --json
```

## Project documentation

| Document | Role |
| --- | --- |
| `AGENTS.md` | Root agent rules and project decisions. |
| `CLAUDE.md` | Claude Code entrypoint and pointer to shared rules. |
| `.agent-harness/CONSTITUTION.md` | Source-of-truth hierarchy and safety principles. |
| `.agent-harness/ARCHITECTURE.md` | Target architecture and boundaries. |
| `.agent-harness/CONVENTIONS.md` | Implementation and integration conventions. |
| `.agent-harness/TESTING.md` | Verification expectations. |
| `.agent-harness/OPERATIONS.md` | Install, CLI, MCP, and skill usage. |
| `.agent-harness/ADR.md` | Implementation roadmap and architectural decisions. |

## Roadmap

- Harden daemon and MCP lifecycle checks.
- Keep CLI/MCP response contracts golden-tested.
- Expand API-documentation drift review for endpoint-heavy repos.
- Promote state-only worker records toward a real job worker only after policy, audit, cancellation, and redaction boundaries are strong enough.
- Keep Codex and Claude Code integrations thin and contract-compatible.

## Documentation style note

This README follows common open-source README guidance: explain what the project does, why it is useful, how to start, and where to get help. See [GitHub Docs on repository READMEs](https://docs.github.com/articles/about-readmes) and [Open Source Guides on starting a project](https://opensource.guide/starting-a-project/) for the documentation shape that informed this rewrite.

## License

No license file is present in this repository at the time of this README update. Add a `LICENSE` file before distributing this as an open-source project.

---

<a id="한국어"></a>

## 한국어

<p align="right"><a href="#english">View in English</a></p>

**agent-harness**는 AI 코딩 에이전트를 위한 개인용 자동화 하네스입니다. **Codex**와 **Claude Code**가 같은 로컬 Go 바이너리, 같은 MCP 도구, 같은 command-policy 검사, 같은 공유 skill 원본을 사용하게 만듭니다.

이 프로젝트는 “Codex plugin 하나”나 “Claude command 모음”이 아닙니다. 재사용 가능한 동작은 host-neutral Go core에 두고, host별 통합은 그 core를 호출하는 얇은 adapter로 유지합니다.

> 현재 상태: 초기이지만 동작 가능한 MVP입니다. CLI, daemon-backed MCP proxy, policy checker, state checkpoint, project-doc tooling, API-doc review gate, native skill installer, self-verification loop, self-augmentation loop가 구현되어 있습니다. worker 표면은 의도적으로 **state-only / no-shell** 상태입니다.

---

## 왜 필요한가

AI 코딩 에이전트는 host마다 prompt, tool, state, safety rule이 달라지면 신뢰하기 어려워집니다. `agent-harness`는 그 공통 관심사를 하나의 portable core에 모읍니다.

하네스의 철학은 **바퀴를 재발명하지 않는다**입니다. agent-harness는 orchestration, policy, state, project docs, install glue 같은 작은 공통 core를 맡고, 전문 knowledge/retrieval 기능은 upstream을 그대로 연결합니다. 예를 들어 `nvk/llm-wiki`, `colbymchenry/codegraph`, `thedotmack/claude-mem`은 하네스 내부에 복제하지 않고 optional dependency로 설치/설정합니다.

다음이 필요할 때 사용합니다.

- Codex, Claude Code, MCP, shell에서 같은 agent workflow를 실행하고 싶을 때
- shared skill을 host별로 복사하지 않고 `skills/` 하나를 source of truth로 쓰고 싶을 때
- repo 운영 문서를 agent가 구조적으로 읽게 하고 싶을 때
- agent가 임의 shell string을 실행하기 전에 command safety를 먼저 판단하게 하고 싶을 때
- 작은 agent checkpoint를 repo 밖 user state에 저장하고 싶을 때
- 하네스 자체를 계속 검증하고 개선하고 싶을 때

## 제공 기능

| 영역 | 명령 / 파일 | 역할 |
| --- | --- | --- |
| 설치와 점검 | `install-native`, `inspect`, `preflight`, `version` | user-level Codex/Claude integration을 만들고 설치 상태를 확인합니다. |
| MCP backend | `agent-harness mcp`, `daemon start/status/stop` | user-level daemon 뒤의 stdio MCP proxy를 실행해 Codex와 Claude가 같은 tool을 보게 합니다. |
| Command policy | `policy check`, `policy fake-run`, `policy audit` | 실제 shell runner 없이 argv, workspace root, cwd, timeout, write/network intent, audit metadata를 평가합니다. |
| State checkpoint | `state write/read/list/prune/doctor/migrate` | 작은 JSON checkpoint를 repo가 아니라 user state에 저장합니다. |
| Project docs | `project bootstrap/docs/route-docs`; MCP `project_docs_record` | `.agent-harness/` 운영 문서를 생성, 색인, 라우팅, append 기록합니다. |
| API docs gate | `api-doc check/static-check/review` | endpoint/DTO/OpenAPI 문서 drift를 찾습니다. |
| Shared skills | `skills/atomic-commit-push`, `skills/project-bootstrap`, `skills/self-verify`, `skills/self-augment` | Codex와 Claude Code가 같은 skill 정의를 사용합니다. |
| Self-improvement | `self-verify`, `self-augment` | 95점 검증 gate와 안전한 개선 후보 선택 루프를 실행합니다. |
| Worker MVP | `worker enqueue/status/list/cancel` | job lifecycle state만 기록합니다. 아직 shell을 실행하지 않습니다. |

## 아키텍처

```mermaid
flowchart LR
    Codex[Codex\nAGENTS.md · skills · MCP] --> MCP[agent-harness mcp\nstdio proxy]
    Claude[Claude Code\nCLAUDE.md · skills · MCP] --> MCP
    Human[Human shell] --> CLI[harness CLI]

    MCP --> Daemon[agent-harness daemon\nuser-level Unix socket]
    CLI --> Core[Go core use cases\npolicy · docs · state · install]
    Daemon --> Core
    Core --> Ports[ports / DTOs]
    Ports --> FS[fs · git adapters]
    Ports --> State[state · audit log]
    Core -. future .-> Worker[local job worker\nqueue · watch · long tasks]
```

설계 규칙:

1. **Host-neutral core first** — 핵심 동작은 host plugin이 아니라 Go core에 둡니다.
2. **Same contract everywhere** — CLI JSON, MCP response, daemon response는 같은 의미를 가져야 합니다.
3. **Safe by default** — command policy, workspace boundary, audit record, redaction, dry-run/no-shell 기본값을 먼저 둡니다.
4. **One skill source** — `skills/<name>/`이 원본이고 user-level Codex/Claude skill 경로는 이를 가리킵니다.
5. **Incremental worker** — persistent worker는 process 실행보다 lifecycle state부터 검증합니다.
6. **바퀴를 재발명하지 않기** — llm-wiki, CodeGraph, claude-mem 같은 upstream 도구는 각자의 installer/plugin으로 연결하고 core 동작을 agent-harness에 복제하지 않습니다.

## 저장소 구조

```text
cmd/harness/              Go binary entrypoint와 CLI/MCP/daemon 명령
internal/core/            host-neutral usecase: docs, policy, state, install, worker contract
internal/port/            core-facing interface와 DTO
internal/adapter/         host/install adapter test와 integration boundary
configs/codex/            Codex MCP와 hook template
configs/claude/           Claude MCP template
skills/                   Codex와 Claude Code가 공유하는 native skill 원본
.agent-harness/           project operating docs, ADR, caution, testing rule
scripts/install-native.sh native install 편의 스크립트
bin/agent-harness               로컬 build binary
```

## 요구 사항

- 로컬 Go toolchain
- native host integration을 쓰려면 Codex 또는 Claude Code
- 현재 daemon/socket 구현을 위한 Unix 계열 로컬 환경

현재 `go.mod` 기준으로 표준 라이브러리 외부 Go module dependency는 없습니다. dependency를 가정하기 전에 현재 tree를 확인하세요.

## 빠른 시작

저장소 루트에서 실행합니다.

```bash
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness version
./bin/agent-harness inspect --json
./bin/agent-harness docs --json
```

명령을 실행하지 않고 command policy만 확인합니다.

```bash
./bin/agent-harness policy check \
  --workspace-root "$PWD" \
  --cwd "$PWD" \
  --json -- git status --short
```

Daemon lifecycle smoke test:

```bash
./bin/agent-harness daemon status --json
./bin/agent-harness daemon start --json
./bin/agent-harness daemon status --json
./bin/agent-harness daemon stop --json
```

user-level Codex/Claude integration은 dry-run 확인 후 설치하거나 갱신합니다. 첫 설치는 `agent-harness bootstrap`, 이후 갱신은 `claude-mem update`처럼 `agent-harness update`를 권장합니다. 두 명령은 기본적으로 현재 checkout binary를 다시 build하고 host integration과 upstream companion tools를 함께 갱신합니다. installer는 `~/.local/bin/agent-harness`를 생성/갱신하므로 새 shell에서는 어디서든 `agent-harness ...`를 사용할 수 있습니다.

```bash
./bin/agent-harness install-native --dry-run --json

# 권장 첫 설치: agent-harness와 upstream companion tools를 함께 세팅합니다.
./bin/agent-harness bootstrap

# 권장 갱신: claude-mem update처럼 사용합니다.
agent-harness update

# 최소 설치: agent-harness native Codex/Claude integration만 갱신합니다.
./scripts/install-native.sh --skip-upstream-tools
./bin/agent-harness install-native --json
```

`agent-harness update`와 `agent-harness bootstrap`은 내부적으로 `./scripts/install-native.sh --with-upstream-tools`를 호출합니다. 스크립트는 매 실행마다 현재 checkout 기준으로 `bin/agent-harness`를 다시 build하고, `~/.local/bin/agent-harness`를 갱신한 뒤 host integration을 갱신하므로, 이미 설치된 agent-harness도 제자리에서 업데이트됩니다. 단, local 변경을 덮어쓰지 않기 위해 `git pull`은 자동 실행하지 않습니다. 원격 변경까지 반영하려면 checkout을 먼저 직접 갱신하세요. 기존 binary를 의도적으로 유지하려면 `--skip-build`, 최소 harness-only 갱신이 필요하면 `--skip-upstream-tools`를 사용합니다.

기본 설치는 user-level host 위치만 대상으로 합니다. target repo에 project-local `.claude/skills`, `.claude/settings.json`, `.mcp.json` 파일을 만들려면 명시적인 project-local mode가 필요합니다.

`--with-upstream-tools`는 이 하네스의 철학인 “바퀴를 재발명하지 않는다”에 맞는 권장 full setup입니다. user-level Codex/Claude/plugin/MCP 설정을 바꾸고 네트워크를 사용할 수 있으며, 다음 upstream 도구를 vendoring/reimplementation 없이 연결합니다.

| 도구 | Upstream | installer 동작 |
| --- | --- | --- |
| LLM Wiki | `nvk/llm-wiki` | Codex/Claude `wiki@llm-wiki` plugin을 추가/갱신합니다. |
| CodeGraph | `colbymchenry/codegraph` | `@colbymchenry/codegraph`를 설치하고 Codex/Claude MCP server를 등록하며, 설정 시 이 repo의 `.codegraph/` index를 초기화합니다. |
| claude-mem | `thedotmack/claude-mem` | Codex/Claude `claude-mem` plugin을 추가/갱신합니다. |

같은 동작은 `HARNESS_INSTALL_UPSTREAM_TOOLS=1`로도 켤 수 있고, local CodeGraph indexing은 `HARNESS_INIT_CODEGRAPH=0`으로 끌 수 있습니다.

## 자주 쓰는 명령

```bash
# 설치와 환경 확인
./bin/agent-harness inspect --json
./bin/agent-harness preflight --json "$PWD"
./bin/agent-harness contract check --json

# 문서와 라우팅
./bin/agent-harness docs --json
./bin/agent-harness project route-docs --repo "$PWD" --task "update command policy" --json

# 상태 체크포인트
./bin/agent-harness state write --key checkpoint --value "ready" --json
./bin/agent-harness state read --key checkpoint --json
./bin/agent-harness state doctor --json

# Policy-only command handling: 실제 shell runner 아님
./bin/agent-harness policy fake-run --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
./bin/agent-harness policy audit --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short

# Worker MVP: state only, no shell execution
./bin/agent-harness worker enqueue --kind smoke --payload "hello" --json
./bin/agent-harness worker list --json

# API 문서 gate
./bin/agent-harness api-doc check --json
./bin/agent-harness api-doc review --json

# 하네스 자체 검증과 개선 루프
./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --json
./bin/agent-harness self-augment --cycles=1 --target-score=95 --json
```

## Native host integration

### Codex

`install-native`는 shared skill을 user-level Codex skill 경로에 연결하고 MCP server/hook 설정을 등록합니다.

확인 명령:

```bash
test -f ~/.codex/skills/atomic-commit-push/SKILL.md && echo "Codex skill linked"
codex mcp get agent_harness
```

### Claude Code

`install-native`는 같은 shared skill을 user-level Claude skill 경로에 연결하고 user-scope MCP server를 등록합니다.

확인 명령:

```bash
test -f ~/.claude/skills/atomic-commit-push/SKILL.md && echo "Claude skill linked"
claude mcp list
```

## Shared skills

| Skill | 역할 |
| --- | --- |
| `atomic-commit-push` | local change를 검토하고 focused commit으로 나누며 Conventional Commit subject + Lore body 형식으로 안전하게 push합니다. |
| `project-bootstrap` | repository evidence를 바탕으로 repo-local agent operating docs를 생성하거나 갱신합니다. |
| `self-verify` | 하네스 95점 verification loop를 실행하거나 결과를 해석합니다. |
| `self-augment` | 안전하고 가치 있는 하네스 개선 후보 1개를 선택, 구현, 검증합니다. |

## 안전 모델

- secret 원문은 prompt, docs, logs, fixtures, CLI JSON, MCP response에 남기지 않습니다.
- host adapter는 core policy를 우회할 수 없습니다.
- policy 명령은 argv form, workspace root, cwd, write/network intent, timeout, audit metadata를 명시적으로 다룹니다.
- 현재 policy/fake-run/audit 명령은 범용 shell 실행 기능이 아닙니다.
- worker 명령은 queue, cancellation, redaction, audit 경계가 단단해질 때까지 no-shell lifecycle record로 유지합니다.
- runtime state는 user state directory나 ignored workspace state에 두고 source file로 commit하지 않습니다.

## 개발과 검증

권장 baseline:

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness inspect --json
./bin/agent-harness docs --json
./bin/agent-harness contract check --json
```

문서만 변경한 경우에도 최소한 파일 경로, build 가능 여부, docs index를 확인합니다.

```bash
find . -maxdepth 3 -type f | sort
find .agent-harness -maxdepth 1 -type f -name '*.md' | sort
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness docs --json
```

## 프로젝트 문서

| 문서 | 역할 |
| --- | --- |
| `AGENTS.md` | root agent rule과 project decision |
| `CLAUDE.md` | Claude Code entrypoint와 shared rule pointer |
| `.agent-harness/CONSTITUTION.md` | source-of-truth hierarchy와 safety principle |
| `.agent-harness/ARCHITECTURE.md` | target architecture와 boundary |
| `.agent-harness/CONVENTIONS.md` | implementation/integration convention |
| `.agent-harness/TESTING.md` | verification expectation |
| `.agent-harness/OPERATIONS.md` | install, CLI, MCP, skill 사용법 |
| `.agent-harness/ADR.md` | implementation roadmap과 architecture decision |

## Roadmap

- daemon과 MCP lifecycle check를 더 단단하게 만듭니다.
- CLI/MCP response contract를 golden test로 유지합니다.
- endpoint-heavy repo를 위한 API-documentation drift review를 확장합니다.
- policy, audit, cancellation, redaction 경계가 충분히 강해진 뒤에만 state-only worker를 실제 job worker로 확장합니다.
- Codex와 Claude Code integration은 얇고 contract-compatible하게 유지합니다.

## README 작성 기준 메모

이 README는 일반적인 오픈소스 README 관례를 따릅니다. 즉, 프로젝트가 무엇을 하는지, 왜 유용한지, 어떻게 시작하는지, 어디에서 더 볼 수 있는지를 먼저 설명합니다. 이번 개편에서는 [GitHub Docs의 repository README 안내](https://docs.github.com/articles/about-readmes)와 [Open Source Guides의 starting-a-project 안내](https://opensource.guide/starting-a-project/)를 참고했습니다.

## License

이 README를 갱신한 시점에는 repository에 `LICENSE` 파일이 없습니다. 공개 오픈소스 배포 전에 `LICENSE` 파일을 추가하세요.
