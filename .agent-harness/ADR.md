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
- hook은 공통 `agent-harness hook user-prompt/post-tool-use/stop` CLI처럼 read-only routing, lifecycle queue 기록, reminder로 제한

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

### 2026-06-13 — Distribution decision gate

Decision: prefer a tarball/manual archive release first. Defer Homebrew until the release checklist, cross-platform build matrix, rollback criteria, and at least one dogfood release note are green.

Rationale:

- `scripts/release-repro-smoke.sh` verifies install planning without writing the operator's real home.
- `scripts/release-build-matrix.sh` verifies the current supported binary matrix: `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`.
- Windows is explicitly excluded until the daemon process setup no longer depends on Unix-oriented `syscall.SysProcAttr.Setsid`.
- Tarball/manual archive keeps the first release reversible without introducing package-manager tap maintenance.

Rollback criteria:

- Roll back when `inspect --json`, `docs --json`, `state migrate --json`, release smoke, or self-verify fails on the release checkout.
- Roll back by returning the checkout to the prior known-good SHA, then running `agent-harness update` and `agent-harness inspect --json`.

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

## 7. Decision Log Index

Current active decisions live in the sections above. Detailed dated ADR notes that are no longer needed in the hot agent reading path are preserved in `.agent-harness/archive/adr-history.md`.

Archived entries:

- LLM Wiki policy.
- 2026-05-29: upstream companion tools are opt-in dependencies.
- MCP-backed project memory records.
- Adapter separation, compatibility contract, audit log, and worker MVP.
- 2026-05-29: quality gate failures return normal MCP payloads.
- 2026-05-29: public command identity follows project identity.
- 2026-05-29: simple bootstrap/project-bootstrap commands.
- 2026-05-30: static project-doc catalog injection.
- 2026-05-30: host-specific UserPromptSubmit display contract.
- 2026-05-31: shared PreToolUse hook and prompt/tool lifecycle split.
- 2026-05-31: repo-local draft wiki staging before upstream LLM Wiki ingest.
- 2026-06-02: IssueOps split.
- 2026-06-04/05: next-action Stop hook is a trigger/evidence relay, not a judge/scorer/classifier/safety gate. The external-LLM gate remains disconnected (agy latency ~13-25s too high for the Stop hook), and the replacement is not a static scoring heuristic: UserPromptSubmit teaches the main agent the policy, while Stop only detects an explicit next-action review point and relays observed facts back to the main agent for all safety/reversibility/alignment/proceed-or-ask judgement. When re-entered by Stop choices, the main agent must state its rationale either way: why it is auto-proceeding now, or why it is not auto-proceeding and user confirmation is required. Auto-proceed result reports still end with `선택지:` so the next turn has an explicit user-facing action boundary. The primary flag is `--relay-next-action-judgement`; `--auto-proceed-next-actions` remains only as a deprecated compatibility alias.
- 2026-06-10: Stop-hook no-auto-proceed responses must not recreate next-action choices. A real recovery turn showed that the prior phrase "auto-proceed result reports still end with choices" was too easy to over-apply to no-auto-proceed decisions, causing a repeated `선택지:` block after the agent had already stopped for user confirmation. The corrected contract is explicit: auto-proceed result reports still end with choices; no-auto-proceed judgements state the rationale and stop without adding another choices block.
- 2026-06-06: A main-agent `no-auto-proceed` judgement is sticky across automated goal continuation. A real Codex goal continuation resumed implementation immediately after the agent had just said it would not auto-proceed at a Stop-hook next-action boundary, producing contradictory behavior. UserPromptSubmit now injects the explicit rule that the same action must not resume after `no-auto-proceed` unless the user selects a choice or gives a new instruction.
- 2026-06-06: `stop_hook_active` suppresses only missing-choice recovery loops, not valid next-action judgement relay. A real Stop-hook recovery turn produced well-formed `선택지:` choices, but `runHookStop` returned `{}` because the relay path was incorrectly gated by `!stop_hook_active`. That caused the main agent to stop without the required proceed-or-ask judgement. The corrected contract is: missing numbered choices no-op while `stop_hook_active=true`; valid next-action choices still re-enter the main agent once, with duplicate relay suppression preventing loops. A recommended continuation is not permission to continue forever: on every relay, the main agent must state its safety/reversibility/alignment judgement and stop when scope is complete, review risk is too high, or user confirmation is needed.
- 2026-06-05: IssueOps worktree tool-root drift is handled without asking the user to restart the host in a different cwd. CodeGraph remains usable because its CLI supports `--path` and its MCP tools support `projectPath`; IssueOps prepares the worktree CodeGraph index and the worktree PreToolUse guard requires CodeGraph `projectPath` to equal the expected worktree. Source-root-bound filesystem/Serena MCP tools are blocked in IssueOps worktree implementation unless their root can be proven to be the expected worktree. For next-action UX, numbered choices are only for real user-decision points; if the safe next step is continued implementation or verification, the main agent should execute instead of ending with choices.
- 2026-06-05: IssueOps worktree isolation is fail-closed once a code-editing phase begins. The prior guard only blocked edits after an exact `worktree_path` was linked, so an agent could create or switch to the issue branch inside the source checkout and implement there. The PreToolUse guard now blocks `git checkout -b`/`git switch -c` for a known IssueOps branch in the source checkout, blocks mutating source/worktree targets during `implement`, `ai-slop-clean`, `feedback`, and `pr` when the cycle has no linked worktree, and still allows `git worktree add ../<repo>.worktrees/...` so the agent can create the required sibling worktree before running `issueops link-worktree`.
- 2026-06-05: IssueOps lifecycle phase promotion is fail-closed at the same boundaries as the worktree guard. A real CLI walk showed `issueops phase --to ai-slop-clean` and then `--to pr` could advance before any linked worktree existed, leaving the edit hook as the only later barrier. `ai-slop-clean` now requires linked issue, provider-linked branch evidence, linked plan, and an existing worktree before phase entry; `pr` now requires strict PR readiness. The VCS linking parser also treats `--copy-issue-labels` as label evidence while still requiring an explicit assignee flag, including GitLab API-oriented `--assignee-id`/`--assignee-ids`.
- 2026-06-05: IssueOps implementation-state links are fail-closed, not just later phase gates. A follow-up real CLI matrix showed `link-plan`, `link-worktree`, and `done` could still advance too early: plan could move to `implement` without an issue or verified branch, worktree state could point at a nonexistent path, and `done` could bypass PR readiness entirely. `link-plan` and `link-worktree` now require linked issue plus verified provider branch evidence, `link-worktree` requires an existing directory, and `done` requires prior `pr` phase. The CLI also treats `issueops --help` as a successful usage request, and structured MCP-style remote artifact inputs now preserve nested `flags.copy_issue_labels` and `flags.assignee` before VCS linking checks.
- 2026-06-05: IssueOps remote artifact ownership treats GitLab `mr for` as MR creation. The installed `glab` help shows `glab mr for` is deprecated but still creates an MR for an issue, and the Codex glab MCP exposes the same surface as `glab_mr_for`. The VCS linking parser now normalizes `glab mr for`/`new-for`/`create-for` and structured `glab_mr_for` to create actions, counts `--with-labels`/`flags.with_labels` only as label evidence, and still blocks creation unless an explicit assignee is inspectable.
- 2026-06-05: IssueOps start and feedback phases are strict lifecycle gates. A live CLI matrix showed `issueops start --branch main` could create a dead-end cycle that later rejected provider branch preparation, and `issueops phase --to feedback` could skip `ai-slop-clean`. `start` now requires an issue-number-prefixed IssueOps branch from the beginning, feedback items may still be recorded early without advancing the phase, and explicit feedback phase entry requires recorded `ai-slop-clean` evidence.
- 2026-06-05: IssueOps plan links must be real files on the active implementation surface. A live CLI matrix showed `link-plan` could move a cycle to `implement` with a nonexistent plan file, and `ai-slop-clean` could start even when the linked worktree lacked the plan path. `link-plan` now requires the plan file to exist in the source repo at link time, while `ai-slop-clean` and PR readiness require the linked worktree to contain the same plan path before later phases can proceed.
- 2026-06-05: GitLab remote artifact assignee evidence must be concrete, not placeholder-shaped. The installed `glab mr create` help accepts usernames, while deprecated `glab mr for` accepts numeric user IDs. The VCS remote artifact guard now rejects GitLab placeholder assignees such as `@me` and rejects username-shaped assignees for `glab mr for`/structured `glab_mr_for`, so agents must resolve the current username or numeric id first and verify the remote assignee list.

## 2026-06-09 — Expose IssueOps gate contracts through MCP and skills

- Kind: `adr`
- Source: codex
- Summary: IssueOps readiness gates must be visible in CLI help, MCP schema descriptions, and SKILL.md so agents do not infer hidden flags or state fields.
- Context: A design review approval failure exposed design_review_evidence only as an internal missing key, causing repeated attempts to find nonexistent evidence flags, approval subcommands, decision records, and force transitions.
- Decision: Treat every IssueOps readiness gate key and conditional approval requirement as a public agent contract. New or changed gates must update CLI guidance, MCP schema descriptions, shared skill instructions, and focused contract tests in the same change.
- Consequences: Future IssueOps changes that hide required gate inputs from MCP or SKILL.md should fail contract tests before reaching installed binaries.

## 2026-06-16 — internal/core *_facade.go는 의도된 공개 표면으로 유지

- Kind: `adr`
- Source: agent-harness optimization P5
- Summary: 최적화 P5에서 facade의 순수 위임을 직접 import로 되돌리는 대신, facade를 의도된 안정적 공개 표면으로 유지하고 경계 규칙을 문서화하기로 결정했다.
- Context: 최적화 계획 P5는 internal/core/*_facade.go의 pure passthrough alias/one-line delegate를 owning package 직접 import로 되돌려 facade 표면을 축소하라고 제안했다. cmd/는 core를 단일 표면으로 import하고 core 내부 subpackage(core/issueops 등)는 직접 import하지 않는다.
- Decision: facade(issueops/workflow/utility/policy/project_doc/state_trace/draft_wiki/issueops_remote)를 의도된 공개 표면으로 유지한다. 허용 내용은 type alias 재노출, 타입 변환, 다중 subpackage 조합, boundary enforcement이며, 순수 1-line 위임도 표면 안정성/디커플링을 위해 허용한다. 규칙은 internal/core/doc.go에 codify했다.
- Consequences: facade에 새 도메인 로직 추가 금지(조합/변환/enforcement만). facade 함수가 그 이상으로 커지면 owning subpackage로 로직 이동. 향후 facade drift 방지를 위해 doc.go 규칙을 따른다.
- Evidence:
  - 133개 facade 심볼 전수조사 결과 미사용(dead) export 0개
  - utility_facade.SummarizeHookFailureStats는 hookfailure+hookmetrics를 조합하는 실제 boundary 로직
  - workflow_facade.RunReadOnlyWorkerJob/projectProfilesToLifecycle는 타입 변환 boundary
  - go build/vet/test ./internal/core 통과
- Alternatives / rejected options:
  - P5 원안대로 pure passthrough를 cmd 직접 subpackage import로 되돌리기 — cmd가 core 내부 구조에 결합되고, dead export가 0이라 표면 축소 이득도 없어 거부
  - facade 내부 unexported 위임(containsAny 등) 제거 — 내부 indirection만 줄고 core 내부 caller churn 발생, 한계 가치라 보류
