# agent-harness 구현 계획

작성일: 2026-05-25

---

## 1. 최종 결정

### 1.1 Plugin 방식 vs 외부 worker 방식

**결정: 외부 하네스 코어 + 얇은 plugin/host adapter의 hybrid 구조를 채택한다.**

- Codex plugin-only는 Claude Code와 공유하기 어렵다.
- Claude Code command/hook-only는 Codex에서 같은 기능을 쓰기 어렵다.
- 외부 CLI/MCP/worker core는 양쪽에서 같은 binary와 schema를 호출할 수 있다.
- plugin은 core가 아니라 설치 UX, 문서, 자주 쓰는 명령 wrapper 역할만 맡긴다.

초기에는 persistent worker보다 **CLI one-shot + MCP stdio**를 먼저 구현한다. worker daemon은 장기 작업·상태 공유·watch가 실제로 필요해진 뒤 추가한다.

### 1.2 언어 선택

**결정: Go를 사용한다.**

근거:

- 현재 로컬에서 `go version go1.26.3 darwin/arm64`가 확인됐다.
- 단일 바이너리 배포, 빠른 컴파일, 동시성, CLI/daemon 구현 생산성이 좋다.
- 개인 하네스는 빠른 반복과 단순 운영이 Rust의 엄격한 안전성보다 우선이다.
- Rust는 추후 untrusted code sandbox, 고위험 parser, 성능 critical component가 필요할 때 재검토한다.

---

## 2. 목표 아키텍처

```text
Codex / Claude Code / Human
        │
        ├─ harness CLI
        ├─ agent-harness mcp     (MCP stdio proxy)
        └─ agent-harness daemon        (user-level shared MCP backend)
                │
          internal/core
                │
          fs/git/process/state/config/wiki adapters
                │
```

핵심 원칙:

- core는 host neutral해야 한다.
- adapter는 core 호출과 입출력 변환만 한다.
- command execution과 workspace access는 policy를 통과해야 한다.
- CLI JSON, MCP response, daemon-backed MCP response는 같은 DTO를 공유한다.

---

## 3. Phase별 계획

### Phase 0 — 문서 기반 세팅

상태: 현재 작업에서 완료 목표.

Deliverables:

- `AGENTS.md`
- `CLAUDE.md`
- `.agent-harness/CONSTITUTION.md`
- `.agent-harness/ARCHITECTURE.md`
- `.agent-harness/CONVENTIONS.md`
- `.agent-harness/TESTING.md`
- `.agent-harness/CAUTIONS.md`
- `.agent-harness/TECH_STACK.md`
- `.agent-harness/ADR.md`
- `.agent-harness/COMMIT_POLICY.md`
- `.agent-harness/OPERATIONS.md`
- `skills/self-augment/SELF_AUGMENTATION.md`
- `skills/atomic-commit-push`
- user-level Codex/Claude skill 경로가 `skills/atomic-commit-push` 단일 원본을 참조
- project-local skill 연결은 기본 설치에서 제외하고 명시적 attach/project-local 모드로만 생성

Acceptance criteria:

- plugin vs worker 판단이 문서화되어 있다.
- Go 선택 근거가 문서화되어 있다.
- Codex와 Claude Code 모두 같은 source of truth를 읽도록 연결되어 있다.
- 첫 shared skill이 단일 원본에서 검증된다.
- Conventional Commit + Lore body 하이브리드 커밋 정책이 문서와 skill에 반영되어 있다.

### Phase 1 — Go 프로젝트 부트스트랩

상태: `cmd/harness`, `go.mod`, `bin/agent-harness`, `inspect`, `preflight`, `mcp` 기본 구현 완료.

Deliverables:

- `go.mod`
- `cmd/harness/main.go`
- 기본 version/build info
- `agent-harness inspect --json` smoke command
- `.gitignore`에 `.harness/`, build artifacts 추가

Acceptance criteria:

```bash
go test ./... -count=1
go build ./cmd/harness
./agent-harness inspect --json
```

### Phase 2 — Core capability MVP

상태: MVP 구현 완료. `internal/core`의 `inspect`, `preflight`, `docs` indexer, `state` checkpoint read/write/list/prune/doctor/migrate API, catalog 기반 `command policy check + fake runner`가 CLI/MCP와 자기 검증 루프 smoke까지 연결됐다. self-verify summary 저장·history·compare·promote도 state 기반으로 검증된다. CLI usage와 MCP tools/resources golden test는 self-verify와 self-augment planner를 함께 포함한다. 남은 adapter 분리는 hardening 과제다.

Deliverables:

- workspace root detection
- `AGENTS.md`/`CLAUDE.md`/`.agent-harness` indexer
- state checkpoint read/write/list/prune/doctor/migrate API
- command policy model(type only, runner는 fake 우선)
- JSON DTO와 error code 정리

Acceptance criteria:

- temp workspace 기반 unit test
- docs index golden test
- state read/write/list/prune/doctor/migrate roundtrip test
- root 밖 path 접근 거부 test

### Phase 3 — MCP stdio proxy/server

상태: `agent-harness mcp`가 shared `agent-harness daemon`을 자동 시작하고 stdio를 Unix socket으로 proxy한다. llm-wiki 전용 tools/resources는 upstream plugin 사용 원칙에 따라 제거됐다.

Deliverables:

- `agent-harness mcp` command
- `agent-harness daemon start/status/stop`
- MCP tools/resources:
  - agent-harness inspect/docs/state/policy/self-verify/self-augment tools
  - `daemon_status`
- CLI DTO와 MCP response 공유
- Claude Code MCP config template + hook helper template

Acceptance criteria:

- MCP tool/resource schema golden test
- daemon-backed MCP stdio smoke test
- Claude Code/Codex MCP config template 문서화

### Phase 4 — Local job worker daemon

상태: MCP backend daemon은 Phase 3에서 구현됨. Phase 4는 별도 job queue/watch worker를 도입할 때만 진행한다.

Deliverables:

- `agent-harness worker start|stop|status` 또는 daemon 하위 job API
- local Unix socket 또는 localhost API
- job lifecycle: queued/running/succeeded/failed/cancelled
- audit log와 redaction
- stale lock/orphan process 처리

Acceptance criteria:

- worker health/version handshake test
- concurrent job test
- timeout/cancel test
- stale lock cleanup test

### Phase 5 — Codex adapter

상태: `internal/adapter/codex`가 `port.HostInstaller`를 구현한다. 기본 설치는 사용자 홈의 Codex skill symlink와 `~/.codex/config.toml` MCP 서버만 갱신한다. 적용 대상 repo에는 파일을 쓰지 않는다.

Deliverables:

- `configs/codex/` 템플릿
- Codex plugin/skill이 필요한 경우 최소 wrapper 작성
- AGENTS.md에서 `agent-harness` 사용 흐름 설명

Acceptance criteria:

- Codex 환경 없이도 wrapper가 호출하는 실제 command가 테스트된다.
- plugin/skill 내부에 core policy가 복제되지 않는다.

### Phase 6 — Claude Code adapter

상태: `internal/adapter/claude`가 `port.HostInstaller`를 구현한다. 기본 설치는 `~/.claude/skills`, user-scope MCP 등록 경로, `~/.claude/settings.json` lifecycle hook 등록을 사용한다. `.claude/skills`, `.claude/settings.json`, `.mcp.json` 같은 repo-local 파일은 명시적 `--project-local`에서만 쓴다.

Deliverables:

- `configs/claude/` MCP 설정과 hook 설정 템플릿
- 자주 쓰는 slash command 템플릿
- hook은 공통 `agent-harness hook user-prompt/post-tool-use/stop` CLI처럼 read-only routing, lifecycle queue 기록, reminder로 제한

Acceptance criteria:

- `agent-harness mcp`와 lifecycle hook 기반 설정이 문서화된다.
- slash command가 core CLI만 호출한다.
- hook에서 위험 shell을 직접 실행하지 않는다.

### Phase 7 — Hardening / release

Deliverables:

- install script 또는 Homebrew/tarball 배포 방식 결정
- cross-platform build matrix
- release checklist
- 사용자 README
- dogfooding notes

Acceptance criteria:

- clean machine 설치 절차가 문서만 보고 재현된다.
- Codex와 Claude Code에서 같은 inspect/state workflow가 성공한다.

---

## 4. MVP 범위

처음 구현할 MVP는 다음으로 제한한다.

1. `agent-harness inspect --json`
2. project docs index
3. state checkpoint read/write/list/prune/doctor/migrate
4. command policy type과 fake runner
5. MCP `inspect`/`state` tools

MVP에서 제외:

- 원격 서버
- 분산 queue
- 무제한 shell runner
- plugin marketplace packaging
- 복잡한 multi-user auth

---

## 5. 주요 위험과 대응

| 위험 | 대응 |
|------|------|
| host별 기능 drift | core DTO 공유, golden test |
| shell 실행 보안 | policy-first, argv 우선, timeout/redaction/audit log |
| worker lifecycle 복잡도 | CLI/MCP 안정화 후 worker 도입 |
| secret 유출 | adapter 경계 redaction, fixture secret 금지 |
| 과도한 초기 설계 | MVP 범위 고정, 필요 기반 phase 승격 |

---

## 6. 다음 작업 제안

1. `internal/adapter/cli`, `internal/adapter/mcp`를 추가해 flag/JSON-RPC mapping을 분리
2. worker 도입 전 core DTO compatibility policy를 문서화
3. self-verify summary baseline promotion을 history 목록/자동 rotation 정책으로 확장할 필요가 있는지 dogfood 결과로 판단
4. response contract golden 범위를 새로 추가되는 capability까지 계속 넓히고, docs byte-size drift가 과하면 normalized subset 전략을 검토
5. state migration 정책을 multi-version fixture로 확장할 필요가 있는지 dogfood 결과로 판단
6. command policy catalog를 config로 확장할 필요가 있는지 dogfood 결과로 판단

## LLM Wiki 정책

LLM Wiki 기능은 agent-harness가 직접 제공하지 않는다. 중복 구현을 피하기 위해 upstream `nvk/llm-wiki`의 Codex/Claude plugin 또는 portable AGENTS.md를 사용한다. 하네스 CLI/MCP에 llm-wiki 전용 명령, tool, resource, SessionStart hook을 추가하지 않는다.

## 2026-05-29 — Upstream companion tools are opt-in dependencies

- Kind: `adr`
- Source: user consensus
- Summary: agent-harness의 철학은 **바퀴를 재발명하지 않는다**이며, llm-wiki, CodeGraph, claude-mem은 하네스 core 기능이 아니라 upstream dependency로 연결한다.
- Decision: `scripts/install-native.sh --with-upstream-tools`는 `nvk/llm-wiki`, `colbymchenry/codegraph`, `thedotmack/claude-mem`의 공식 installer/plugin 경로를 호출하는 opt-in convenience path를 제공한다. 기본 `install-native`는 하네스 자체 Codex/Claude integration만 설치한다.
- Rationale: knowledge wiki, code graph, memory compression은 각 upstream이 이미 전문적으로 제공한다. 하네스는 이들을 재구현하지 않고 설치/설정 접착제만 제공해야 core policy, MCP, state, docs 책임이 흐려지지 않는다.
- Rejected: llm-wiki/codegraph/claude-mem 기능을 하네스 CLI/MCP tool로 복제하는 대안 | 중복 구현과 drift를 만들고 host-neutral core를 비대하게 만든다.
- Consequences: companion tool 설치는 네트워크와 user-level host 설정 변경을 수반하므로 opt-in이어야 한다. 실패 시 하네스 core contract를 약화하지 말고 upstream 설치 경로나 문서를 고친다.

## ADR note: MCP-backed project memory records

- Decision: CAUTIONS and ADR records should be appended through `project_docs_record` MCP when an agent solves a concrete problem or makes a decision with rationale.
- Rationale: this keeps durable project knowledge in repo-local markdown while giving agents a precise tool-use situation.
- Rejected: relying only on session memory or always-on context injection, because it is less durable and can overfill context.
- Consequence: record writes must stay append-only, narrow, and test-covered.

## ADR note: Adapter 분리, compatibility contract, audit log, worker MVP

- Decision: CLI usage metadata는 `internal/adapter/cli`, MCP adapter-owned tool descriptor는 `internal/adapter/mcp`에 둔다. `cmd/harness`는 process entrypoint와 concrete handler mapping을 유지한다.
- Decision: `agent-harness contract schema|check`는 DTO 변경 전 CLI/MCP command, tool, required response field, stable hash를 확인하는 compatibility contract 표면이다.
- Decision: `agent-harness policy audit`는 command를 실행하지 않고 redacted command-policy JSONL record만 append한다.
- Decision: `agent-harness worker enqueue|status|list|cancel`은 Phase 4의 no-shell lifecycle record MVP다. 실제 process execution은 audit, cancellation, timeout, policy hardening이 더 확장된 뒤 도입한다.
- Rationale: 추천 구현 순서를 수행하면서 host adapter가 core policy를 우회하지 않는다는 project invariant를 유지하기 위해서다.
- Rejected: 첫 worker 단계에서 실제 shell/job runner를 추가하는 대안. command execution은 더 강한 audit, timeout, cancellation, redaction gate가 필요하다.
- Consequence: future worker work는 별도 queue 표면을 만들지 말고 no-shell lifecycle DTO 위에 확장한다.

## 2026-05-29 — Return quality gate failures as normal MCP payloads

- Kind: `adr`
- Source: codex
- Summary: MCP tools must reserve JSON-RPC errors for transport, input, execution, or persistence failures; quality gate pass/fail outcomes are normal tool payload fields.
- Context: api_doc_review returned a JSON-RPC error when the review completed successfully but verdict was fail, making agents confuse a gate finding with an MCP/tool failure.
- Decision: API documentation static/review gates and self-verification gate failures now use sentinel errors internally, and MCP handlers suppress those sentinels so callers receive normal payloads with ok/verdict/findings/violations/summary fields.
- Consequences: Future MCP quality gates should model failed checks as structured payload fields and only raise rpcError for real execution failures; CLI commands may still return non-zero for automation gate enforcement.
- Evidence:
  - cmd/harness/gate_errors.go
  - cmd/harness/main.go api_doc_review/api_doc_static_check/self_verify handlers
  - cmd/harness/mcp_quality_gate_test.go
- Alternatives / rejected options:
  - Continue returning JSON-RPC errors for gate failures | conflates successful review outcomes with tool/runtime failures and hides structured result handling behind exception paths
  - Suppress all errors with result payloads | would hide real IO, invalid argument, save-state, subprocess, and transport failures


## 2026-05-29 — Public command identity follows project identity

- Kind: `adr`
- Source: user directive
- Summary: agent-harness의 설치 UX와 문서에서 public command 이름은 프로젝트 이름과 같은 `agent-harness`로 통일한다.
- Decision: canonical built binary와 user-facing command는 `bin/agent-harness` 및 PATH command shim `~/.local/bin/agent-harness`이다. 설치 스크립트는 기존 checkout의 binary를 매번 갱신하고, `~/.local/bin/agent-harness -> <checkout>/bin/agent-harness` symlink를 생성/갱신한다. 이전 실험에서 만든 managed shell alias block은 제거해 alias와 symlink가 서로 다른 표면으로 drift하지 않게 한다. 예전 `bin/harness` compatibility symlink는 남기지 않는다.
- Rationale: command, binary, 로그, README, MCP 설정 이름이 갈라지면 사용자는 어떤 표면이 최신/정식인지 판단해야 한다. 이름 통일성은 개인 하네스의 신뢰성과 학습 비용에 직접 영향을 준다.
- Rejected: `bin/harness`를 계속 남기거나 shell alias만 public command 표면으로 삼는 대안 | legacy alias가 drift를 만들고, alias는 non-interactive script/daemon 환경에서 일관되게 동작하지 않는다.
- Consequences: docs, golden fixtures, install templates, MCP command paths는 `agent-harness`를 기준으로 유지한다. 내부 Go source directory `cmd/harness`는 구현 경로일 뿐 public UX 이름이 아니다.


## 2026-05-29 — Simple bootstrap/project-bootstrap commands

- Kind: `adr`
- Source: user directive
- Summary: public setup UX is split into `agent-harness bootstrap` for user-level installation and `agent-harness project bootstrap` for repo-level docs/profile initialization.
- Decision: `agent-harness bootstrap` installs or refreshes user-level harness integrations; `--sync` additionally refreshes optional upstream companion tool versions. `agent-harness project bootstrap` creates/refreshes target-repo `AGENTS.md`, `.agent-harness/*.md`, and user-state repo profile metadata; `--sync` means refresh from current templates and repo evidence. The old `update` command remains a compatibility alias but is not the recommended UX.
- Rationale: users should remember two nouns instead of several low-level install/update options: harness-level bootstrap and repo-level project bootstrap. `--sync` consistently means “bring versions/docs/profile back to current evidence.”
- Rejected: separate `update`, `--write`, and upstream-specific flags as primary UX | they exposed implementation detail and made first setup vs refresh harder to explain.
- Consequences: docs, usage, skills, and stability audits should recommend only `bootstrap`, `bootstrap --sync`, `project bootstrap`, and `project bootstrap --sync`; low-level flags/scripts stay compatibility/automation surfaces.
