# Runtime, daemon, MCP, state, and process topology

> Family index: [`../ARCHITECTURE.md`](../ARCHITECTURE.md). This module owns the
> execution modes, the daemon/MCP/worker surfaces, the docs/state/config/log
> topology, and the command-policy model. Component boundaries and dependency
> direction live in [`hexagonal-core.md`](hexagonal-core.md); IssueOps execution
> state and threat model live in [`issueops.md`](issueops.md); host adapters
> live in [`host-integration.md`](host-integration.md).

## 실행 모드

| 모드 | 도입 단계 | 용도 | 원칙 |
|------|----------|------|------|
| `agent-harness` CLI one-shot | 구현됨 | 모든 host에서 공통으로 호출 가능한 최소 표면 | `bin/agent-harness inspect/preflight/doctor/docs/policy/state/self-verify/self-augment` 사용 |
| `agent-harness mcp` stdio proxy | 구현됨 | Codex/Claude Code가 같은 MCP schema로 daemon에 연결 | `agent-harness` daemon을 자동 시작하고 stdio를 Unix socket으로 proxy한다. |
| `agent-harness daemon` user-level daemon | 구현됨 | 여러 host/session의 공통 MCP backend, 상태 공유 | `HARNESS_DAEMON_DIR` 또는 `~/.local/state/agent-harness/daemon`; stale lock, pid, socket, stop/status 제공 |
| `agent-harness issueops` | 구현됨 | issue-driven 루프의 durable 상태와 direct/Orca execution v1 lease | IssueOps가 단일 authority다. Orca는 readiness, workspace, native owner launch/inventory만 제공하고 generation/actor/CWD fence는 core가 소유한다. |
| `agent-harness loop` | 구현됨 | verify-until-done 루프 계약의 durable 상태와 PR readiness 게이트 | 하네스는 검증 명령을 실행하지 않고 `verify_argv`, 시도 evidence, stop 상태를 기록·게이트한다. |
| `agent-harness worker` one-shot jobs | 부분 구현 | no-shell lifecycle job record | 현재 daemon은 MCP proxy backend이며 장기 상주 job daemon이 아니다. |
| Codex plugin/skill | Phase 5 | Codex에서 설치·명령·문서 UX 개선 | core 로직 금지, CLI/MCP 호출 래퍼만 허용 |
| Claude commands/hooks | Phase 6 | Claude Code UX 개선 | core 정책 우회 금지 |

## Docs / state / config / logs

현재 `agent-harness docs`는 에이전트가 읽어야 할 markdown source of truth를 index로 노출한다. `agent-harness project bootstrap`은 적용 대상 레포에 명시 실행될 때만 `AGENTS.md` marker block, `.agent-harness/*.md` 프로젝트 운영 문서, user-state repo profile metadata를 생성/갱신한다.

- 대상: `AGENTS.md`, `CLAUDE.md`, `GENIUS_THINK.md`, `.agent-harness/*.md`, `skills/self-verify/*.md`, `skills/self-augment/*.md`
- 필드: relative path, absolute path, title, headings, byte size
- 제공 표면: CLI `docs --json`, MCP `docs_index`, resource `harness://docs`

Project docs bootstrap:

- 대상: 적용 대상 repo의 `AGENTS.md`, `.agent-harness/ARCHITECTURE.md`, `.agent-harness/CAUTIONS.md`, `.agent-harness/COMMIT_POLICY.md`, `.agent-harness/CONSTITUTION.md`, `.agent-harness/CONVENTIONS.md`, `.agent-harness/TECH_STACK.md`, `.agent-harness/TESTING.md`, `.agent-harness/ADR.md`, `.agent-harness/OPERATIONS.md`, `.agent-harness/AGENT_WORKFLOW.md`
- 기본 동작: `agent-harness project bootstrap`은 누락된 파일과 user-state repo profile metadata를 생성한다. 계획만 볼 때는 `--dry-run`, 기존 문서/프로필을 현재 템플릿과 repo evidence로 다시 맞출 때는 `--sync`를 쓴다.
- 안전: `AGENTS.md` 전체를 덮어쓰지 않고 `AGENT_HARNESS` marker block만 관리한다.
- MCP: `project_docs_bootstrap_plan`, `project_docs_route`, `harness://project-docs`와 lifecycle profile metadata로 어떤 작업에 어떤 문서/레포 맥락을 확인해야 하는지 제공한다.

현재 `agent-harness state`는 작은 에이전트 체크포인트를 state root의 SQLite 데이터베이스(`harness.db`의 `state` bucket row)로 저장한다. project lifecycle state는 같은 user-state root 아래 `projects/<repo-id>/`에 격리되며 target repo의 `.agent-harness/`에는 쓰지 않는다. IssueOps v1 상태는 독립 namespace `issueops_v1/harness.db`의 `issueops_v1` bucket에 저장해 Codex와 Claude 세션을 넘어 이어간다. Loop 상태는 같은 user-state root 아래 `loop/harness.db`의 `loop` bucket에 저장한다. 모든 read-modify-write span은 해당 root의 `harness.lock.db`에 BEGIN IMMEDIATE 트랜잭션을 유지하는 sqlstore span으로 직렬화된다(프로세스 사망 시 자동 해제, span 중첩 금지).

- 기본 위치: `~/.local/state/agent-harness/`
- project lifecycle 위치: `~/.local/state/agent-harness/projects/<repo-id>/project.json` 및 `doc-upkeep-queue.jsonl`; `<repo-id>`는 repo fingerprint hash라 같은 머신의 여러 repo가 섞이지 않는다.
- Loop 위치: `~/.local/state/agent-harness/loop/<loop-id>.json`. CLI `loop start/record-attempt/status/stop`와 MCP `loop_start/loop_record_attempt/loop_status/loop_stop`가 같은 state machine을 사용한다. 같은 repo+name의 active loop는 resume되고 terminal loop는 새 name이 필요하다. strict PR readiness는 같은 repo의 `active`/`exhausted` loop를 `loop_incomplete:<loop-id>`로 막고, `stopped`/`succeeded` loop는 통과한다.
- override: `HARNESS_STATE_DIR`
- 파일: `<key>.json`
- key 제한: `[A-Za-z0-9._-]`, 최대 128자, `/`, `\`, `..` 금지
- schema: current `schema_version=1`; version이 없는 legacy record는 read-compatible하고 `state migrate`로 승격한다.
- 제공 표면: CLI `state write/read/list/prune/doctor/migrate`, MCP `state_write/state_read/state_list/state_prune/state_doctor/state_migrate`, resource `harness://state`
- cleanup: `state prune --max-age DURATION`은 기본 dry-run이고, 실제 삭제에는 `--confirm`이 필요하다.
- integrity: `state doctor`는 checkpoint 파일을 수정하지 않고 invalid JSON, key mismatch, byte count drift, timestamp 오류를 보고한다.
- comprehensive diagnostics: `agent-harness doctor`는 state doctor를 포함해 install, hooks, MCP, daemon, project docs, lifecycle namespace, repo-local runtime/schema 흔적을 종합 점검한다.
- migration: `state migrate`는 기본 dry-run이고, 실제 legacy schema rewrite에는 `--confirm`이 필요하다.
- self-verify summary checkpoint는 `self-verify history/compare/promote`와 MCP `self_verify_history/self_verify_compare/self_verify_promote`로 조회·비교·승격한다.

IssueOps v1 execution state, schema authority, capability verticals, and the
actor model live in [`issueops.md`](issueops.md).

기준:

| 종류 | 권장 위치 | 추적 여부 |
|------|-----------|----------|
| 프로젝트 지식 | `.agent-harness/`, `AGENTS.md`, `CLAUDE.md` | git 추적 |
| 사용자 전역 설정 | `~/.config/agent-harness/config.yaml` | git 비추적 |
| 사용자 전역 state/log | `~/.local/state/agent-harness/` 또는 OS별 state dir | git 비추적 |
| workspace local cache | `.harness/` | `.gitignore` 대상 |
| secret | OS keychain 또는 env reference | 원문 저장 금지 |

구현 시 XDG base directory를 우선 검토하고, macOS에서도 예측 가능한 fallback을 둔다.

## Command / policy model

명령 실행 기능은 가장 위험한 capability이므로 별도 policy로 관리한다.

현재 구현은 실제 shell runner가 아니라 **policy check + fake runner**다.

- CLI: `policy check`, `policy fake-run`
- MCP: `command_policy_check`, `command_fake_run`
- Resource: `harness://command-policy`
- fake runner는 policy 결과와 audit id만 반환하며 명령을 실행하지 않는다.
- allow/deny 목록은 `internal/core/policy.go`의 catalog table이 source of truth이며, `CommandPolicySummary()`의 `catalog` 필드로 노출된다.

필수 필드:

- `workspace_root`
- `cwd`
- `argv` 배열(shell string보다 우선)
- `timeout`
- `env_allowlist` 또는 scrub rule
- `network_allowed` 여부
- `write_allowed` 여부
- `audit_log_id`

기본 정책:

- read-only inspection은 허용 범위를 넓게, write/process/network는 명시적으로 좁게 시작한다.
- shell interpolation이 필요한 경우 이유를 기록하고, 가능하면 argv 실행을 사용한다.
- stdout/stderr에서 secret pattern을 redaction한다.

현재 기본 거부:

- `cwd`가 `workspace_root` 밖인 요청
- path-like argv가 `workspace_root` 밖 파일/디렉터리를 가리키는 요청(`~/path`, symlink escape 포함)
- shell interpreter(`sh`, `bash`, `zsh` 등) without reason
- `network_allowed=false`에서 network성 명령
- `write_allowed=false`에서 write성 명령
- read-only allowlist 밖 명령
- secret-like path/argument

현재 catalog 범주:

- shell interpreters: `sh`, `bash`, `zsh`, `fish`, `dash`, `ksh`
- network commands/subcommands: `curl`, `wget`, `ssh`, package manager류, `git fetch/pull/push/clone` 등
- write commands/subcommands: 파일 변경 명령, `go build/test/run`, `git add/commit/reset/...` 등
- read-only commands/subcommands: `ls`, `cat`, `rg`, `git status/diff/log`, `go env/list/version` 등

## MCP tool design guidance

- Tool descriptions must state: purpose, when to use, whether it writes, required arguments, and expected result shape.
- Prefer bounded, task-specific tools over catch-all tools.
- Keep tool list ordering deterministic for stable client caching and golden tests.
- Use resources for reusable context, tools for actions, and project docs routing for deciding what to read.
- Writable MCP tools should either be dry-run by default or append-only with narrow target files.

## Standalone Runtime Policy

`agent-harness install`, `bootstrap`, `update`, and `scripts/install-native.sh` install only agent-harness native integrations. They must not clone, install, patch, or register third-party toolchains as a side effect.

External tools may be useful in a user's environment, but they are not agent-harness dependencies. If a workflow benefits from one of them, the user installs it through that tool's own documented path and the harness consumes only explicit, inspectable boundaries such as files, command output, or MCP data the user has already configured.

Readiness gates, self-verification, install/update success, and core CLI/MCP contracts must remain reproducible without external accounts, API keys, companion hooks, or companion MCP servers. Do not add fallback shims that patch external plugin caches or weaken harness contracts when an external tool is missing or broken.
