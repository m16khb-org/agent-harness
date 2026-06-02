---
name: ADR.md
description: Structural decisions, rationale, and rejected alternatives.
---

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
- Migration: memory companion은 `rohitg00/agentmemory`에서 `thedotmack/claude-mem`으로 교체한다. full upstream setup은 legacy agentmemory plugin/marketplace 배선을 제거한다.
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


## 2026-05-30 — Static project-doc catalog injection (작업 레포 .agent-harness 메뉴)

- Kind: `adr`
- Source: user directive (이번 세션). 앞선 동일 날짜의 "opt-in agent-backed advisor" 검토안을 **이 결정으로 대체한다**(LLM 호출·blocking·넛지/검증 루프 모두 철회).
- Summary: UserPromptSubmit 훅이 **현재 작업 중인 대상 레포의 `.agent-harness/*.md`**(하네스 자신의 문서가 아님)를 훑어 "어떤 문서에 어떤 정보가 있는지" 컴팩트 카탈로그(메뉴)를 만들어 메인 루프 컨텍스트에 주입한다. **어느 문서를 읽을지의 판단은 메인 에이전트가** 자기 지능으로 한다. 하네스는 결정하지 않고 정확한 목록만 제공한다.
- Decision:
  - **하네스는 모델을 호출하지 않는다**: 카탈로그는 각 문서의 title + heading에서 정적·결정적으로 생성한다. provider/key/캐시/타임아웃/어댑터가 전혀 필요 없다 → host-neutral 완전 유지.
  - **판정 제거, 메뉴 제시**: 기존 정적 훅의 "required/consider로 문서를 *판정*"하던 부분 대신, "이 문서엔 이런 정보가 있다"는 메뉴를 제시한다. 틀릴 수 있는 판정은 메인 에이전트로 넘긴다.
  - **항상 주입(gating 없음)**: "프롬프트 관련성 있을 때만"을 정적으로 판단하는 것 자체가 신뢰할 수 없는 판정이므로 시도하지 않는다. 메뉴는 값싸므로(문서당 1줄) 매 턴 항상 주입한다.
  - **non-blocking**: 단순 컨텍스트 주입이라 메인 루프를 막지 않는다.
  - **결정적 → immutable_prefix**: 카탈로그는 결정적이라 context byte-determinism 계약의 immutable prefix에 그대로 들어간다. volatile region 불필요.
  - **대상-레포 한정**: 후보는 작업 레포의 `.agent-harness/*.md`로 한정한다(AGENTS.md/CLAUDE.md/임의 docs 제외; 필요 시 후속 ADR에서 확장). `.agent-harness/`가 없으면 카탈로그 생략(no-op).
- Rationale: 정적 분석이 "결정"하는 건 못 믿지만 "메뉴를 만드는" 정적 작업(존재하는 문서와 그 내용 요약 나열)은 신뢰할 수 있다. 판단을 메인 모델에 두면 LLM·blocking·넛지 없이도 정확성과 효율을 동시에 얻는다.
- Rejected:
  - 하네스-소유 opt-in LLM advisor(Arch A) | host-neutral·no-provider 포기 + 매 턴 비용/지연/비결정성. 메인 에이전트가 이미 판단 가능하므로 불필요.
  - blocking 사전 게이트 | 단순 메뉴 주입에 메인 루프를 막을 이유 없음.
  - async 추천 + Stop-time 넛지/검증 루프 | 메뉴를 항상 주입하면 "안 읽었을 때 알림"이 불필요하고, Read 여부 탐지는 이전 턴 로드/타 훅 주입 때문에 오탐 위험.
  - 정적 키워드로 required/consider 판정 유지 | 모호한 프롬프트에서 잘못된 문서를 prescribe할 수 있음.
- Consequences: §11.3의 "core가 모델 직접 호출 reject"는 **그대로 유지**된다(이 결정은 모델을 안 부른다). 후속 슬라이스: (S1) `DiscoverProjectDocs(repoRoot)` + 카탈로그 포맷 + UserPromptSubmit 훅 주입 배선 + 테스트. (S2, 선택) 카탈로그가 비대해질 경우의 길이 budget/요약 정책. 후보 source는 하네스 자신의 docs(`DocsIndex`/`ListDocs`의 skills 포함분)가 아니라 작업 레포의 project docs임에 주의한다.


## 2026-05-30 — Host-specific UserPromptSubmit display contract

- Kind: `adr`
- Source: Codex 0.135.0 runtime evidence and user directive.
- Summary: Codex and Claude Code can call the same `agent-harness hook user-prompt` CLI, but their visible rendering of hook output differs. The common core stays shared; host adapters select the output shape.
- Decision:
  - `project bootstrap` and `project bootstrap --sync` keep writing canonical frontmatter descriptions for `.agent-harness/*.md`; those descriptions are now concise English metadata to reduce catalog length while preserving meaning.
  - Codex installs `agent-harness hook user-prompt --host codex`. In that mode the hook omits `systemMessage` and injects only the full project-doc catalog through `hookSpecificOutput.additionalContext`; route/action/profile/pending-upkeep status blocks are excluded because Codex shows `additionalContext` in the TUI `hook context:` row.
  - Claude Code keeps the richer split: `systemMessage` can show a readable user-facing view while `hookSpecificOutput.additionalContext` carries compact model-facing routing/status hints.
  - The host-specific split is an adapter/output concern only. Project-doc discovery, canonical metadata, bootstrap/sync frontmatter, and routing hint construction remain in shared Go core.
- Rationale: Codex currently has no separate hidden UserPromptSubmit context channel; anything in `additionalContext` is also surfaced to the user. Keeping only the project-doc catalog in Codex preserves the information the agent must receive and avoids noisy route/profile/upkeep lines. English metadata is shorter for this technical catalog than the previous Korean prose in byte/character length and is easier to scan in the collapsed hook row.
- Rejected:
  - Re-adding the full route/action/profile/pending-upkeep block to Codex | it bloats the visible `hook context:` row and is not required for the agent to choose project docs.
  - Making Codex use only a short summary | it hides document descriptions from the agent.
  - Forking hook core by host | host drift would violate the shared CLI/MCP contract.
- Consequences: tests must cover both bootstrap/frontmatter metadata and host-specific `UserPromptSubmit` output. Future Codex runtime changes that add a hidden context channel may revisit this decision, but until then Codex output should preserve full doc descriptions and omit nonessential status blocks.


## 2026-05-31 — Shared PreToolUse hook and prompt/tool lifecycle split

- Kind: `adr`
- Source: user directive plus Codex 0.135.0 / Claude Code 2.1.158 runtime evidence.
- Summary: agent-harness now installs PreToolUse alongside UserPromptSubmit and PostToolUse for both Codex and Claude Code, but keeps PreToolUse non-blocking by default.
- Decision:
  - UserPromptSubmit owns per-turn advisory routing: prompt intent, repo profile, pending upkeep, and MCP/action hints. It must stay lightweight and must not execute work.
  - PreToolUse owns only fast deterministic preflight on the critical path before a tool call. The default host payload is `{}`; raw `--json` exposes an allow/no-op diagnostic result. Blocking or input rewriting requires a separate deterministic policy and host-schema tests.
  - PostToolUse owns observed successful tool-use bookkeeping. It may append lifecycle/doc-upkeep state for relevant files or commands, but it does not edit shared `.agent-harness` docs.
  - Codex and Claude adapters both install `agent-harness hook pre-tool-use` through the same Go CLI/core. Claude uses matcher `*`; Codex uses the same command without a host flag because no host-shaped payload is emitted.
- Rationale: both host runtimes expose PreToolUse, but it is high-friction because false positives can block normal tool use. The most efficient harness role is to split intent routing before a turn, no-op preflight before a tool, and state recording after successful tools.
- Evidence:
  - `strings ~/.local/share/claude/versions/2.1.158 | grep PreToolUse` shows Claude Code documents PreToolUse as “Run before tool, can block”.
  - `strings <codex 0.135.0 native binary> | grep PreToolUse` shows Codex has PreToolUse schema and validation strings.
  - `configs/codex/hooks.json` and `configs/claude/hooks.settings.json` now include PreToolUse.
- Rejected:
  - Put lifecycle queue writes in PreToolUse | tool execution may fail or be denied, causing false doc-upkeep records.
  - Block from PreToolUse immediately | host schema drift and false positives would make the harness intrusive.
  - Duplicate host-specific policy in adapters | violates the shared core contract and creates Codex/Claude drift.


## 2026-05-31 — Repo-local draft wiki staging before upstream LLM Wiki ingest

- Kind: `adr`
- Source: user directive
- Summary: claude-mem 같은 companion memory에서 장기기억 후보를 찾더라도 바로 LLM Wiki에 쓰지 않고, 적용 대상 repo의 `.agent-harness/draft-wiki/`에 reviewable draft로 먼저 저장한다.
- Decision:
  - 기존 `.agent-harness/*.md` 운영 문서는 루트에 유지한다. 경로가 project docs/bootstrap/routing contract에 이미 사용되므로 `agent-base-docs/` 같은 하위 폴더로 이동하지 않는다.
  - `.agent-harness/draft-wiki/{draft,approved,rejected}/`를 repo-local staging area로 둔다.
  - `agent-harness docs`/MCP `docs_index`는 `.agent-harness/draft-wiki/**`를 source-of-truth agent docs로 인덱싱하지 않는다.
  - `agent-harness project draft-wiki suggest`는 `agy -p`를 별도 CLI/worker 단계에서 호출한다. `agy` 모델 선택은 launch flag가 아니라 `~/.gemini/antigravity-cli/settings.json`의 `model` 값이다. `--agy-model`이 있으면 settings 값과 정확히 일치하는지 검증하고, 없으면 현재 settings 모델을 그대로 사용한다.
  - PostToolUse hook은 tool response/output/content 텍스트를 bounded/redacted user-state queue(`draft-wiki-queue.jsonl`)에 기록하는 데까지만 관여한다. `agent-harness worker draft-wiki`가 hook 밖에서 queue를 처리하고 `agy -p` 응답을 `.agent-harness/draft-wiki/draft`에 쓴다.
  - `agent-harness project draft-wiki promote --confirm`는 승인된 draft를 configured `nvk/llm-wiki` topic의 `raw/<type>/` note로 쓰고 `log.md`만 append한다. compile/query/index 관리는 upstream LLM Wiki workflow가 맡는다.
- Rationale: 사용자는 draft를 파일 diff로 직접 확인·수정·거절할 수 있어야 하며, 하네스는 claude-mem→wiki 승격의 승인 게이트를 제공하되 LLM Wiki core 책임을 침범하지 않아야 한다.
- Rejected:
  - 기존 `.agent-harness/*.md`를 `agent-base-docs/`로 이동 | bootstrap, routing, AGENTS required reading, golden contract를 대규모로 깨뜨린다.
  - claude-mem observation을 자동으로 LLM Wiki에 바로 ingest | privacy/secret/false-positive 위험이 크고 사용자 승인 게이트가 없다.
  - 하네스가 llm-wiki index/query/compile을 직접 관리 | LLM Wiki 재구현 금지 정책과 충돌한다.
- Consequences: draft-wiki CLI와 tests는 repo-local file boundary, docs index 제외, approve/reject 상태 이동, `agy` settings model 검증, llm-wiki raw-note write를 검증해야 한다. hook 연동은 PostToolUse 직접 LLM 호출이 아니라 lifecycle/user-state queue append와 `worker draft-wiki` 처리로 제한한다.

## 2026-06-02 — IssueOps split

- Kind: `adr`
- Source: IssueOps implementation
- Summary: IssueOps는 native skill이 절차를 오케스트레이션하고 CLI/MCP `issueops`가 durable state를 저장하며 hook은 routing hint만 제공한다.
- Context: 사용자가 문제파악 -> GitHub/GitLab issue -> issue-based plan -> TDD/subagent implementation -> feedback loop -> PR/MR 흐름을 하네스로 진행하고 싶다고 요청했다.
- Decision: `skills/issueops`를 공유 skill 원본으로 두고, `agent-harness issueops` CLI와 MCP `issueops_*` tools가 issue URL, plan path, feedback, readiness state를 저장한다. UserPromptSubmit hook은 `issueops`를 제안할 수 있지만 issue/PR/MR 생성이나 파일 수정은 수행하지 않는다.
- Consequences: 앞으로 issue-driven 작업 루프는 skill로 절차를 시작하고 필요 시 IssueOps state id를 기록한다. contract/golden tests must include new CLI and MCP surfaces.
- Evidence:
  - skills/issueops/SKILL.md
  - internal/core/issueops.go
  - cmd/harness/issueops.go
  - cmd/harness/main.go issueops_* MCP tools
  - internal/core/hook_prompt.go issueops routing hint
- Alternatives / rejected options:
  - MCP-only workflow: 에이전트 절차를 담기 어렵고 Codex/Claude native skill routing과 맞지 않는다.
  - Hook-driven automation: prompt hook critical path에서 remote issue/PR 생성이나 파일 수정을 수행하게 되어 안전 경계와 충돌한다.
  - Skill-only workflow without durable state: compaction, host handoff, feedback loop 추적이 약하다.
