# Implementation roadmap

← [ADR index](../ADR.md)

This module retains the original agent-harness implementation plan: target
architecture, phase-by-phase deliverables and acceptance criteria, MVP scope,
risk register, and next-work candidates. Phase status lines are historical
snapshots from when each phase was written; the current implementation surface
lives in root `ARCHITECTURE.md` and `OPERATIONS.md`. Foundational decisions
behind this plan are immutable records under `decisions/` and are linked from
the [ADR index](../ADR.md).

## 목표 아키텍처

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

## Phase별 계획

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

상태: `agent-harness mcp`가 shared `agent-harness daemon`을 자동 시작하고 stdio를 Unix socket으로 proxy한다. llm-wiki 전용 tools/resources는 별도 upstream CLI/MCP 서버 사용 원칙에 따라 제거됐다.

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
- hook은 공통 `agent-harness hook session-start` CLI 하나로 project-doc catalog 주입에 제한(2026-08-27 이후 enforcement/relay hook 없음)

Acceptance criteria:

- `agent-harness mcp`와 lifecycle hook 기반 설정이 문서화된다.
- slash command가 core CLI만 호출한다.
- hook에서 위험 shell을 직접 실행하지 않는다.

### Phase 7 — Hardening / release

Deliverables:

- install script 또는 Homebrew/tarball 배포 방식 결정
- cross-platform build matrix
- release checklist (`.agent-harness/operations/release-reproducibility.md`, `scripts/release-repro-smoke.sh`)
- 사용자 README
- dogfooding notes

Acceptance criteria:

- clean machine 설치 절차가 문서만 보고 재현된다.
- Codex와 Claude Code에서 같은 inspect/state workflow가 성공한다.

### Phase-era decisions

Decisions recorded against this phase plan now live as immutable records and
are linked from the [ADR index](../ADR.md):

- [2026-06-13 — Distribution decision gate](./decisions/2026-06-13-distribution-decision-gate.md)
- [2026-06-23 — IssueOps hook and state-machine boundary](./decisions/2026-06-23-issueops-hook-state-machine-boundary.md)
- [2026-06-23 — IssueOps execution decision gate](./decisions/2026-06-23-issueops-execution-decision-gate.md)
- [2026-06-24 — Skill local background separation](./decisions/2026-06-24-skill-local-background-separation.md)
- [2026-07-03 — Codex PreToolUse ask fallback](./decisions/2026-07-03-codex-pretooluse-ask-fallback.md)
- [2026-07-26 — Linked branches are pinned to the sealed base SHA](./decisions/2026-07-26-linked-branches-pinned-to-sealed-base-sha.md)

## MVP 범위

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

## 주요 위험과 대응

| 위험 | 대응 |
|------|------|
| host별 기능 drift | core DTO 공유, golden test |
| shell 실행 보안 | policy-first, argv 우선, timeout/redaction/audit log |
| worker lifecycle 복잡도 | CLI/MCP 안정화 후 worker 도입 |
| secret 유출 | adapter 경계 redaction, fixture secret 금지 |
| 과도한 초기 설계 | MVP 범위 고정, 필요 기반 phase 승격 |

## 다음 작업 제안

1. `internal/adapter/cli`, `internal/adapter/mcp`를 추가해 flag/JSON-RPC mapping을 분리
2. worker 도입 전 core DTO compatibility policy를 문서화
3. self-verify summary baseline promotion을 history 목록/자동 rotation 정책으로 확장할 필요가 있는지 dogfood 결과로 판단
4. response contract golden 범위를 새로 추가되는 capability까지 계속 넓히고, docs byte-size drift가 과하면 normalized subset 전략을 검토
5. state migration 정책을 multi-version fixture로 확장할 필요가 있는지 dogfood 결과로 판단
6. command policy catalog를 config로 확장할 필요가 있는지 dogfood 결과로 판단
