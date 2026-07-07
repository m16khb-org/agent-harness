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

### 2026-06-24 — Skill local background separation

Decision: keep reusable skill instructions portable and store team-specific meeting-minutes background in gitignored `skills/*/background.local.md` files. Tracked skills may reference the local background path and provide an example template, but must not embed private team rosters or service operating context directly in `SKILL.md`.

Rationale:

- `scripts/release-repro-smoke.sh` verifies install planning without writing the operator's real home.
- `scripts/release-build-matrix.sh` verifies the current supported binary matrix: `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`.
- Windows is explicitly excluded until the daemon process setup no longer depends on Unix-oriented `syscall.SysProcAttr.Setsid`.
- Tarball/manual archive keeps the first release reversible without introducing package-manager tap maintenance.

Rollback criteria:

- Roll back when `inspect --json`, `docs --json`, `state migrate --json`, release smoke, or self-verify fails on the release checkout.
- Roll back by returning the checkout to the prior known-good SHA, then running `agent-harness update` and `agent-harness inspect --json`.

### 2026-06-23 — IssueOps hook and state-machine boundary

Decision: IssueOps remains a main-agent state machine backed by `agent-harness issueops ...` CLI/MCP durable state. Lifecycle hooks enforce only fast deterministic violations that can be inspected from the current tool event: worktree target, Korean remote artifact text, VCS issue-linking metadata, PR/MR target branch, label/assignee evidence, and numbered next-action shape.

Rationale:

- Phase transitions and missing-gate names already live in `internal/core/issueops/issueops_phase.go` and `internal/core/issueops/issueops_readiness.go`, so durable state owners are the right place for issue, plan, worktree, design review, ai-slop-clean, feedback, PR/MR, and cleanup evidence.
- Hook decisions already chain deterministic guards in `internal/core/lifecycle/lifecycle_state.go`; adding workflow work there would make host behavior stateful, slow, and harder to share across Codex and Claude Code.
- Remote writes, tests, branch/worktree preparation, background waits, review replies, merge, and cleanup require main-agent judgment about safety, reversibility, and user intent.

Rejected alternative: let hooks advance IssueOps phases or create provider artifacts automatically. This was rejected because hooks lack enough context to judge ambiguity, ownership, credentials, destructive cleanup, and review/CI tradeoffs.

### 2026-06-23 — IssueOps execution decision gate

Decision: `implement` readiness requires a durable `execution_decision` record. The record captures auto-proceed boundaries, hook-blocked workflow work, human gates, and sub-agent usage. Sub-agent use defaults to `none`; `planned` is allowed only when the plan records a documented pattern slug, expected benefit, tradeoffs, net-positive rationale, scope, verification, and fallback.

Rationale:

- Sub-agent research in `.agent-harness/research/subagent-tradeoffs.md` confirms that context isolation, tool gating, model specialization, fresh review, and parallel independent research can be valuable, but they carry latency/token overhead, summary-only visibility, and weaker mid-run steering.
- Hooks cannot make that tradeoff because the decision depends on current task scope, user intent, reversibility, and whether the main agent still needs continuous control.
- Persisting the decision prevents silent auto-advance from plan/tool prep into implementation when the human-in-the-loop or sub-agent boundary has not been stated.

Rejected alternative: infer sub-agent usage from hook hints or phase names. This was rejected because it would hide the tradeoff analysis and make sub-agent use depend on host-specific runtime behavior instead of durable IssueOps state.

### 2026-07-03 — Codex PreToolUse ask fallback

Decision: Codex PreToolUse "ask" outcomes are emitted as a normal `decision="block"` response, while Claude Code and Reasonix keep `hookSpecificOutput.permissionDecision="ask"`.

Rationale:

- Codex CLI 0.142.5 rejects `hookSpecificOutput.permissionDecision="ask"` with `unsupported permissionDecision:ask`, so emitting native ask breaks the hook before the user sees the gate reason.
- The core lifecycle decision still records `decision="ask"` in JSON analysis, preserving the domain meaning and hook metrics.
- A block fallback is fail-closed and host-compatible for live-access gates such as `kubectl exec` and `kubectl port-forward`.

Rejected alternative: allow Codex ask-style gates by emitting unsupported `permissionDecision="ask"` and relying on the host to recover. This was rejected because it turns a deliberate safety gate into a hook runtime failure.

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

## 2026-06-18 — IssueOps implementation requires durable worktree tool preparation

- Kind: `adr`
- Source: codex
- Summary: IssueOps implementation entry is gated on persisted worktree dependency and CodeGraph preparation evidence, not only on linked worktree and plan paths.
- Context: Investigation showed `issueops worktree prepare-tools` already installed supported pnpm dependencies and initialized CodeGraph for the linked worktree, but its result was transient. `link-plan` moved the cycle directly to `implement`, so agents could start implementation before proving dependencies, manual symlink/copy/install work, or CodeGraph readiness for the exact worktree.
- Decision: Store `worktree_tools` on the IssueOps record, keep `link-plan` as plan attachment only, and let `prepare-tools` persist evidence and unlock `implement` when readiness is complete.
- Consequences: CLI, MCP, skills, docs, and response contracts must expose `worktree_tools_prepared`, `worktree_dependencies_ready`, and `codegraph_ready` as public gates. Manual dependency reuse remains explicit: perform the symlink/copy/install in the linked worktree, rerun `prepare-tools`, then proceed.

## 2026-06-18 — IssueOps plan-prep evidence gate

- Kind: `adr`
- Source: agent-harness issueops cycle (branch 12-issueops-planprep-evidence-gate)
- Summary: `plan` phase 진입에 결정사항 조회·유사이슈 scoring·웹서치 근거를 fail-closed 게이트로 강제하되, intent class가 trivial이면 스킵하고 각 항목은 사유로 면제(waive)할 수 있게 했다.
- Context: 기존 `IssueOpsPlanReadiness`는 intent contract + issue_url만 검사해, 이슈 생성 이전 정보수집(ADR/decision 조회, related-issue scoring, berners-lee 웹서치)이 권고에만 머물고 강제되지 않았다.
- Decision: record에 `IssueOpsPlanPrep` sub-record(prior_decisions/related_issues/web_research; 각 evidence|waived)를 추가하고, intent contract에 `IntentClass`를 추가했다. `IssueOpsPlanReadiness`가 non-trivial class에서 세 항목을 검사한다(missing 키 `plan_prep_decisions`/`plan_prep_related_issues`/`plan_prep_web_research`). 기록은 `issueops plan-prep record` CLI/MCP(`issueops_plan_prep_record`)가 담당하고, intent class는 `intent record --intent-class`로 명시한다. 미기록 class는 `standard`로 정규화해 기본 강제한다.
- Consequences: non-trivial 사이클은 plan 진입 전 세 증거(또는 면제 사유)를 남겨야 한다. design review는 plan-prep을 전제하지 않는다(plan phase 내부 활동이라 plan 진입 시 이미 강제됨) — `RecordDesignReview`는 `plan_prep_*` missing을 무시한다. CLI/MCP/contract/SKILL.md/issue-preflight를 같은 변경에서 갱신했다.
- Alternatives / rejected options:
  - intent contract에 3개 필드 직접 추가 — '의도 계약'과 '증거 수집'이 한 곳에 뒤섞여 거부하고 별도 PlanPrep sub-record 채택
  - 무조건 강제(waive 불가) — trivial/순수 내부 작업까지 막혀 마찰이 커 거부하고 사유 기반 면제 채택
  - design review까지 plan-prep 강제 — 수많은 design 검증 테스트가 깨지고 실제 흐름상 이중 검사라 거부하고 plan-phase 진입에만 강제

## 2026-06-26 — IssueOps compatibility review phase

- Kind: `adr`
- Source: codex
- Summary: IssueOps implementation entry now requires a dedicated `compatibility-review` phase for backward compatibility, side effects, rollback, and verification judgement.
- Context: The existing `execution_decision` gate recorded auto-proceed and HITL/sub-agent judgement, but backward compatibility and side-effect review were not first-class state. Hook-side judgement would make progress host-event-dependent and hard to replay.
- Decision: Add `compatibility-review` between `plan` and `implement`, persist `compatibility_review` on the IssueOps record, expose CLI/MCP owner commands, and make `implement` fail closed until the review is approved and blocker-free.
- Consequences: Agents must run `issueops compatibility review` or MCP `issueops_record_compatibility_review` before implementation. Missing readiness keys are public contract (`compatibility_review`, `backward_compatibility`, `side_effects`, `rollback_plan`, `compatibility_verification`, `compatibility_blockers`, `compatibility_approval`) and must stay documented with CLI/MCP/schema changes.

## 2026-06-29 — IssueOps phase ledger, grill gate, and Brooks devil's-advocate regression

- Kind: `adr`
- Source: agent-harness issueops phase-ledger work (spec `docs/superpowers/specs/2026-06-29-issueops-phase-ledger-design.md`)
- Summary: IssueOps records carry an additive `phase_ledger` indexing phase completion; `problem`/`grill` get fail-closed completion gates; net-new source-of-truth fields back previously-unrecorded artifacts; a Brooks devil's-advocate `stop` regresses the cycle to `grill` for re-plan.
- Context: The lifecycle had per-artifact gates but no phase-level ledger, and `problem`/`grill` had no completion gate at all (first gate was at `plan`). Several matrix artifacts (`domain_review`, `cleanup_evidence`, `verification_evidence`, `feedback_resolution`, `target_branch_match`) had no backing field, so they could not be enforced from existing state without creating duplicate truth. Plan rejection had no structured way to send the cycle back for re-grilling.
- Decision: (1) Add `IssueOpsPhaseLedger`/`IssueOpsPhaseLedgerEntry` (map key authoritative; entry `Phase` a self-describing copy) plus net-new fields `DomainReview`, `AISlopCleanCategories`, `AISlopCleanVerification`, `Feedback[].Resolution`, `RemoteArtifactVerification.TargetBranch`. (2) `grill` entry requires problem completion (`intent_contract`); `plan` entry requires grill completion (`issue_url`+`branch`+`plan_prep`+`split_decision`+`domain_review`), `split_decision` derived from existing child `IssueLinks`/scope `Decisions`. (3) New recorders + CLI/MCP: `issueops domain-review record`/`issueops_record_domain_review`, ai-slop-clean evidence, `issueops feedback resolve`/`issueops_resolve_feedback`, remote target-branch capture, and `issueops regress`/`issueops_regress_for_replan`. (4) Ledger stamping happens only at phase-transition sites (real `entered_at`/`completed_at`), and status derives a sentinel-timestamped ledger when none is stored — keeping golden stable. (5) `target_branch_match` enters strict PR readiness. (6) The `brooks` skill is a sub-agent-only devil's advocate routed at `plan`; a `stop` regresses to `grill`, clears design approval, and marks the plan/compatibility-review ledger entries stale.
- Consequences: New cycles must record `domain_review` and a `split_decision` before `plan`. Net-new fields/commands are public contract (golden fixtures regenerated for the three new MCP tools). Completion/derivation uses non-strict PR readiness (no git fetch) while the pr-entry gate stays strict. `karpathy` is documented as cross-cutting prompt augmentation before every sub-agent dispatch.
- Alternatives / rejected options:
  - Make `domain_review`/`split_decision` non-blocking warnings — rejected: weakens the fail-closed guarantee that is the ledger's purpose.
  - Stamp the ledger at the `touchAndWriteIssueOps` leaf — rejected: every recorder write would attach a ledger, churning every record output and golden fixture; stamping only at phase-change sites is golden-safe.
  - Place `issue_url`/`branch` in `problem` completion — rejected: the workflow creates the issue during grill (`problem -> grill -> issue -> plan`), so they gate `plan` entry as grill artifacts instead, preserving the free `problem -> grill` step.

## 2026-07-01 — MCP transport: adopt modelcontextprotocol/go-sdk with a retained legacy JSON-RPC path

- Kind: `adr`
- Source: agent-harness doc reconcile (BC5)
- Summary: The MCP transport stack is settled. `github.com/modelcontextprotocol/go-sdk` v1.6.1 is the confirmed SDK for daemon-socket MCP; a legacy hand-rolled JSON-RPC path is retained for the split reader/writer stdio smoke that the SDK transport cannot cover.
- Context: TECH_STACK §3 previously listed the MCP layer as an open candidate — "안정적인 Go MCP SDK 또는 직접 JSON-RPC 최소 구현 (SDK 선택 전 schema 안정성 확인)". The harness has since shipped both: `go.mod` pins `github.com/modelcontextprotocol/go-sdk` v1.6.1 (used by `cmd/harness/mcpcli/mcp_sdk_server.go`), and PROJECT_AUDIT item M1 records the dual-transport reality where `serveMCPStreamLegacy` is load-bearing for the stdio smoke path.
- Decision: Adopt `github.com/modelcontextprotocol/go-sdk` v1.6.1 as the confirmed MCP SDK for the daemon socket transport, and keep the legacy JSON-RPC stream (`serveMCPStreamLegacy`) intentionally for the split reader/writer stdio smoke. Both transports are kept by design (M1 accepted), not as a cleanup target.
- Consequences: TECH_STACK lists go-sdk as a confirmed dependency rather than a candidate. A go-sdk version bump must re-run the MCP tool-catalog and response-contract goldens (`cmd/harness/testdata/mcp_tools.golden.json`, `cmd/harness/testdata/response_contracts.golden.json`). Removing the legacy path would drop the stdio-smoke coverage, so it stays until an SDK equivalent exists.

## 2026-07-01 — IssueOps devil's-advocate is a fail-closed loop, not just skill prose

- Kind: `adr`
- Source: agent-harness issueops devil's-advocate loop (spec/plan 2026-07-01)
- Summary: The brooks devil's advocate becomes a machine-enforced gate — implement entry requires a recorded verdict (pass, or a stop/revise explicitly waived), and a `stop`'s findings must be reflected into the remote issue before `regress` rewinds to grill.
- Context: brooks was only skill-mandated (SKILL prose "MUST run"); the state machine had no invariant that it ran, and a `stop`'s findings stayed local (regress recorded a scope decision and never touched the remote issue). The loop was advisory, not enforced.
- Decision: (1) Add a first-class `IssueOpsDevilsAdvocateReview` record (verdict pass|revise|stop, findings, waiver, IssueReflectedAt) mirroring the design/compatibility review pattern. (2) `IssueOpsImplementationReadiness` fails closed on a missing review or an unwaived stop/revise (missing key `devils_advocate_review`). (3) A new `UpdateIssueBodySection` provider method (github/gitlab) idempotently splices findings into a delimited issue-body section; `issueops remote reflect-devils-advocate` writes it and stamps `IssueReflectedAt`. (4) `regress` requires a recorded stop whose findings were reflected, and clears the review so the re-planned cycle earns a fresh verdict.
- Consequences: The new record field, two CLI subcommands (`devils-advocate review`, `remote reflect-devils-advocate`), two MCP tools (`issueops_record_devils_advocate_review`, `issueops_remote_reflect_devils_advocate`), and the provider interface method are public contract; mcp_tools/response goldens were regenerated. The enforced loop is stop → reflect → regress → re-plan → fresh verdict.
- Alternatives / rejected options:
  - Keep brooks skill-only — rejected: machine enforcement is the point (the loop-engineering ask).
  - Extend `ValidateArtifactURL` for issue verification — rejected as dead code; reflection uses `UpdateIssueBodySection`, not the pr/mr verify chain.
  - Have `regress` write the issue itself — rejected: keeping regress local/pure and ordering the steps (record → reflect → regress) is a cleaner fail-closed sequence.

## 2026-07-01 — IssueOps phase transition is a pure reducer over the record; impurity lives in wrappers

- Kind: `adr`
- Source: 12-factor-agents #12 (stateless reducer) 적용 조사 (research `.agent-harness/research/harness-engineering-12factor.md`), Brooks devil's-advocate 리뷰 후 doc-only로 축소
- Summary: IssueOps phase 전이의 판정은 이미 `IssueOpsRecord`만 읽는 순수 함수이고, 비결정·side-effect(clock/git/FS/session/disk)는 wrapper가 소유한다. 이 불변식을 CONVENTIONS "State machine reducer contract"로 명문화한다. 코드 변경은 없다.
- Context: 조사에서 12-factor #12(`(state,event)→state` 순수 변환으로 replay/복구 보장)를 적용 후보로 도출했다. 코드 검증 결과 readiness 게이트(`issueops_phase.go:46-112`)는 이미 record만 읽어 순수했고, 유일한 비결정 요소는 wall-clock(`:114`)·git/FS read(`:120-121`)·disk write(`:124`)·session unbind(`:31-32`)로 모두 wrapper에 국소화돼 있었다. ledger 결정성은 이미 `DeriveIssueOpsPhaseLedger` 테스트로 보장된다.
- Decision: (1) CONVENTIONS.md에 "State machine reducer contract" 섹션을 추가해 "판정은 record-순수, 비결정성은 wrapper 소유"를 규율로 고정한다. (2) 코드는 바꾸지 않는다 — 순수 함수 추출(`reducePhase`)은 **채택하지 않는다**.
- Consequences: 신규 상태머신은 판정 로직에 clock/IO를 섞지 않아야 한다. `AdvanceIssueOpsPhase`(`issueops`, `issueopscli`, `mcpcli`, `harnessapp` golden, `lifecycle`, `hookcli`, `adapter/mcp` 파급의 최고-민감 함수, §28)는 동작 무변화 목적으로 건드리지 않는다. `.agent-harness/*.md` 편집이므로 `response_contracts.golden.json` docs_index를 재생성한다(§27).
- Alternatives / rejected options:
  - `reducePhase(record, to, now, head, fingerprint)` 순수 함수 추출 — rejected: Brooks 판정. clock은 phase 결정에 영향을 주지 않아 replay 테스트가 잡는 버그가 없고(ledger 결정성은 이미 테스트됨), 라인 100 절단은 앞쪽 6개 동등-순수 게이트(`:46-99`)를 wrapper에 남겨 개념적 무결성을 깬다. 최고-민감 함수를 zero-behavior-change로 건드리는 것은 negative-EV.
  - 문서 없이 코드만 정리 — rejected: 규율(신규 상태머신 가이드)이 핵심 가치이고 거의 무비용이다.

## 2026-07-01 — Defer harness-side tool-error context injection; spike a nudge before building persistence

- Kind: `adr`
- Source: 12-factor-agents #3/#9 (own your context window / compact errors) 적용 조사, Brooks devil's-advocate 리뷰
- Summary: PostToolUse가 tool 에러를 구조화 이벤트로 압축·주입하는 기능(신규 `ToolFailureEvent` store/queue/signature)은 **빌드하지 않는다**. 먼저 Claude `systemMessage`에 수기 에러 넛지를 넣어 에이전트 행동 개선을 측정하는 스파이크를 거친 뒤 최소 구현 여부를 결정한다.
- Context: 현재 PostToolUse는 `tool_response`를 파싱하지 않아 하네스가 주입하는 에러 바이트는 0이다(`cmd/harness/hookcli/hook_lifecycle.go:33-35`; 유일한 feedback은 B3 lint-gate `:60-72`). "에러 압축으로 토큰 절약"의 기준선이 존재하지 않으므로, 기능은 오늘 대비 컨텍스트를 순증가만 시킨다. 또한 12-factor #3은 메시지 배열을 소유하는 커스텀 루프용 원칙인데, 하네스는 컨텍스트를 소유하지 않는 hook guest이고 호스트(Claude/Codex)가 이미 tool 에러를 인라인 표시한다. 계획이 "재사용할 기존 인프라"로 지목한 docupkeep `Resolve()`는 실제로 존재하지 않아(write-side resolve/dedup은 전부 신규) 비용이 과소평가돼 있었다.
- Decision: (1) v1에서 `ToolFailureEvent` store/JSONL queue/cross-turn capsule/signature 해시를 만들지 않는다. (2) 검증 순서는 스파이크-먼저 — Claude `systemMessage`에만(Codex `additionalContext` 금지, §14) `- errors: N unresolved (tool: summary)` 넛지를 수기 주입해 행동 delta를 측정한다. (3) 행동 개선이 확인되면 최소 구현(세션 한정 카운트, 문자열 동등 기반 success-resolve, persistent store 없음)만 검토한다. 확인 안 되면 폐기한다.
- Consequences: 지금은 코드 변경이 없다. 성공 지표는 "기능 내부 fold 바이트 비교"가 아니라 "에이전트 행동 개선"으로 정의한다. `hook_lifecycle.go` PostToolUse boundary(§16.1 관찰-only, 대행 금지)는 유지된다.
- Verdict (2026-07-01 spike): **KILL** — 정적 넛지 계측기(`spikeErrorNudgeUserView`, env `HARNESS_SPIKE_ERROR_NUDGE`, Claude systemMessage 전용)를 붙여 subagent A/B로 H1을 판정. 양쪽 arm 모두 인라인 미해결 빌드 에러를 "다음 할 일 #1"로 배치해 **한계효과 0**(control 3/3 = treatment 3/3). 라이브 컨텍스트에서 넛지는 호스트 인라인 표시와 중복이라 최소 구현조차 정당화 안 됨. 유일한 비중복 가치인 cross-compaction 재부상(H2)은 정적 넛지가 아니라 capsule-first를 요구하는 별건으로, 실제 장기 세션에서 통증이 관찰될 때만 착수. 상세: `.agent-harness/research/spike-tool-error-nudge.md`.
- Alternatives / rejected options:
  - 계획대로 store/queue/signature/capsule 빌드 — rejected: 존재하지 않는 기준선 대비 토큰 절약을 주장했고, resolve 인프라 신규 비용을 과소평가했으며, 아키텍처적으로 "own the context"를 소유하지 않는 hook에 이식하는 우발적 복잡도.
  - Codex `additionalContext`에도 에러 digest 주입 — rejected: §14(사용자 노출 산문 경계) + Codex TUI가 이미 tool 에러 표시 → 중복 노이즈.
  - resolved 판정에 "N턴 미재발" 규칙 추가 — rejected: wall-clock/turn-count 도입으로 결정성 훼손.

## 2026-07-02 — External LLM stays Z.AI-only until a second provider is real

- Kind: `adr`
- Source: quality optimization Track 1 L1 cleanup
- Summary: Delete the unused `internal/port.ExternalLLM` interface and keep external LLM invocation as the concrete Z.AI wrapper in `internal/core/externalllm`.
- Context: `internal/port/externalllm.go` defined `ExternalLLM`, but no implementation or caller used it. Real call sites (`commitsuggest`, `draftwiki`, `issueops` scoring/benchmark, `lintdiagnose`, `nextaction`, self-verify LLM eval) call `internal/core/externalllm.RunExternalLLMPrint`, which already hard-rejects non-Z.AI providers. Keeping the orphan interface made the audit appear solved by an abstraction that did not exist in the runtime path.
- Decision: Remove `port.ExternalLLM`. Do not add a replacement provider abstraction in this pass. Treat Z.AI-only as an explicit YAGNI decision until a second provider is required by a real caller and can be tested through the same CLI/MCP contracts.
- Consequences: There is no CLI/MCP public contract change. Provider extensibility is intentionally deferred; future provider work must introduce the abstraction together with a concrete second implementation, tests, and an ADR update.
- Alternatives / rejected options:
  - Wire existing call sites through `port.ExternalLLM` now — rejected because there is only one provider and no observed variation point.
  - Keep the unused interface as a placeholder — rejected because it falsely signals decoupling and is not exercised by tests or runtime.

## 2026-07-02 — Self-augment planner consumes Reflexion lessons as score penalty

- Kind: `adr`
- Source: article-insights improvement plan T1
- Summary: The self-augment planner now loads self-augment-lesson-* state snapshots and demotes candidates with repeated severe failure lessons via a score penalty, instead of leaving lessons write-only.
- Context: 2026-07-02 article-insight plan (LINE multi-agent oscillation/warning-label mechanisms, Introspection trace-to-pattern loop) cross-checked against the repo showed self_augment_lesson snapshots are recorded but never consumed by augmentplan; only success-side satisfaction rules feed back into the curriculum. Brooks devil's-advocate review confirmed this gap is real (plan.go loads no lesson snapshot) and trimmed the scope to score-penalty demotion only.
- Decision: Consume lessons in the planner as an advisory score penalty: candidates accumulating severe lessons lose curriculum rank but are never auto-failed or removed (LINE "advisory signal, not judgement" principle). No new lesson DTO fields (Category/RootCauses/SeenCount/Confidence deferred until a second consumer exists). This differs from the KILLed tool-error context injection (2026-07-01): that was an in-session PostToolUse static nudge duplicating host inline display; this is cross-run Reflexion memory feeding planner candidate scoring — different consumer, loop, and layer, so the KILL rationale does not apply.
- Consequences: Planner reads lesson state at plan time; repeated failed attempts on one candidate naturally rotate the curriculum to the next best candidate. Score penalty parameters live in augmentcatalog. Golden contracts unchanged unless plan DTO shape changes.
- Evidence:
  - cmd/harness/selfworkflow/augmentplan/plan.go (no lesson consumption before this change)
  - cmd/harness/selfworkflow/augmentlesson/self_augment_lesson_state.go: self-augment-lesson-<slug>-<ts> state keys
  - .omc/plans/2026-07-02-article-insights-harness-improvement-plan.md (v2, brooks-reviewed)
- Alternatives / rejected options:
  - Warning-label schema with new DTO fields (Category/RootCauses/SeenCount/Confidence) rendered into plan output — rejected as second-system: 2 of 3 intended consumers (pattern report, pattern-to-judge) do not exist yet
  - Auto-fail/remove repeatedly failing candidates — rejected: violates the advisory-signal safeguard; past patterns may prompt extra scrutiny but must not auto-fail current work
  - A→B→A findings-signature oscillation detection — deferred: requires normalized-hash infrastructure with no observed occurrence evidence

## 2026-07-02 — IssueOps regress rounds are capped with a human-decision escalation

- Kind: `adr`
- Source: article-insights improvement plan T2
- Summary: RegressIssueOpsForReplan now appends an IssueOpsRegressEvent audit entry per successful regression and refuses fail-closed once a cycle reaches 3 stop-to-re-plan rounds, demanding a human decision instead of another automatic re-plan.
- Context: The 2026-07-02 article-insight plan (LINE multi-agent: repeated-failure loops waste tokens and need escalation to a differently-scoped reviewer) cross-checked with the repo showed the stop→reflect→regress→re-plan loop had no round counter or upper bound. Brooks review trimmed the original proposal (A→B→A findings-signature oscillation detection) to a count cap only, since each round already requires a fresh stop verdict plus remote reflection and no signature-oscillation occurrence has been observed.
- Decision: Add RegressEvents ([]{reason, from_phase, at}, omitempty) to IssueOpsRecord as the audit trail; cap successful regressions at issueOpsRegressCap=3 per cycle. At the cap the regress is refused before any mutation (phase, ledger, and events untouched) with an error stating the plan is thrashing and a human decision is required. Phase judgement stays a pure reducer; the cap lives in the regress wrapper action alongside the existing stop/reflect preconditions.
- Consequences: Cycles that hit the cap require deliberate human action (e.g. force-release, re-scope, or a fresh cycle). RegressEvents is omitempty so existing records and response goldens are unaffected until a regression occurs.
- Evidence:
  - internal/core/issueops/issueops_regress.go (cap + event append)
  - internal/core/issueops/model/types.go (IssueOpsRegressEvent, RegressEvents)
  - internal/core/issueops/issueops_regress_cap_test.go (below-cap allowed, at-cap refused without mutation)
- Alternatives / rejected options:
  - A→B→A findings-signature oscillation detection — deferred: needs normalized-hash infrastructure with no observed occurrence evidence
  - Unbounded regressions (status quo) — rejected: stop→regress rounds can thrash indefinitely, burning tokens without converging
  - Auto-closing the cycle at the cap — rejected: destructive; the cycle stays intact so a human can waive, re-scope, or abandon deliberately

## 2026-07-02 — External LLM calls emit per-call usage observation records

- Kind: `adr`
- Source: article-insights improvement plan T3
- Summary: Every successful RunExternalLLMPrint call now parses the provider usage block and appends an external-llm-usage-* state record (provider, model, tokens, duration) via a recorder hook wired in core root; recording is best-effort and can never block the call.
- Context: The 2026-07-02 article-insight plan T3 (Introspection: trace every run with turns/tokens/cost; Sonnet 5 tokenizer change showed model-side cost shifts are invisible without usage accounting) found the harness had zero usage observability. Brooks review trimmed scope to append-only observation records, deferring compare regression rules until baselines accumulate.
- Decision: Parse the OpenAI-compatible usage block into ExternalLLMPrintResult (Model, DurationMS, Usage) in the externalllm package, which stays state-free; a package-level usage recorder hook defaults to no-op and core root wires it to a StateWrite-backed writer in init(). This observes every production caller (leaf packages call externalllm directly, but every shipped binary links core root) while leaf-package unit tests record nothing. Keys use nanosecond suffixes (lesson key-collision lesson applied). All recorder failure paths return silently — observation must never fail the observed call. Z.AI-only scope per the existing YAGNI ADR.
- Consequences: Usage history accumulates as state records for future pattern aggregation (P1-2) and long-loop stability metrics (P2-3). Tests exercising the LLM path through core root must isolate HARNESS_STATE_DIR (llmeval helper updated).
- Evidence:
  - internal/core/externalllm/usage.go, print.go (usage parse + recorder hook)
  - internal/core/external_llm_usage.go (init wiring + best-effort writer)
  - internal/core/external_llm_usage_test.go (record written; broken state dir does not block the call)
- Alternatives / rejected options:
  - Recording inside the core facade only — rejected: most production callers (benchmark judge, remote judge, lintdiagnose, draftwiki, commitsuggest, nextaction) call externalllm directly and would be unobserved
  - Recording unconditionally inside externalllm — rejected: externalllm would depend on state, and every leaf-package unit test would write into the developer's real ~/.local/state dir
  - token_usage compare regression rules now — deferred until real baselines accumulate (brooks trim)

## 2026-07-07 — IssueOps orchestration state model

- Kind: `adr`
- Source: codex Task 15 project docs update from codex-orchestration implementation plan
- Summary: IssueOps orchestration uses additive child-cycle fields, scoped session bindings, and workpool state while keeping the harness host-neutral and non-spawning.
- Context: D5 documentation update for delegated child cycles and workpool orchestration. The existing architecture keeps hooks observe-only and IssueOps as a durable main-agent state machine.
- Decision: Represent delegated child cycles as additive IssueOps record fields, represent parent/child session binding as scoped user-state bindings, and represent pool execution in a separate workpool namespace. Main agents dispatch and accept results; child/pool workers own their isolated worktree and heartbeat; hooks only relay or block deterministic violations. The harness records state, gates, leases, and results but does not spawn host agents.
- Consequences: Future CLI/MCP work must preserve schema_version=1 additive compatibility for these fields, keep locks single-entity, and make worker liveness lease/heartbeat based. Any child or pool dispatch must reference a recorded execution_decision with an allowed sub-agent pattern slug and explicit verification/fallback.
- Evidence:
  - .agent-harness/ARCHITECTURE.md IssueOps and workpool state model update
  - .agent-harness/AGENT_WORKFLOW.md resume, heartbeat, and pool-worker contract update
  - .agent-harness/SUB_AGENT_PATTERNS.md D1/D3 mapping update
- Alternatives / rejected options:
  - Spawn agents directly from the harness CLI or daemon — rejected because host auth, context, and approval boundaries are host-specific and would hide user-visible costs/control.
  - Store child and pool progress only in issue comments or docs — rejected because resume, lease expiry, and acceptance gates need durable local state that survives compaction and host changes.
  - Let lifecycle hooks advance orchestration — rejected because hooks lack full safety/reversibility/user-intent context and must remain fast observe/relay/block surfaces.
  - Use nested or multi-entity locks for parent/child/pool updates — rejected because the current store guarantees per-entity atomicity only and nested lock re-entry can self-deadlock.
