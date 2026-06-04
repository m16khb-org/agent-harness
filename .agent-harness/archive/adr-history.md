---
name: adr-history.md
description: Archived dated ADR notes preserved outside the hot agent reading path.
---

# ADR History Archive

This file preserves detailed dated ADR notes that no longer need to be in the active `ADR.md` reading path. Keep current decisions and active tradeoffs in `ADR.md`; append older or superseded decision details here when they are still useful as historical evidence.

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

## 2026-06-04 — Auto-proceed gate is heuristic + prompt injection, not an external LLM call

- Kind: `adr`
- Source: Stop-hook auto-proceed iteration (this session)
- Summary: The Stop-hook auto-proceed decision uses ONLY the static heuristic (`EvaluateNextActionAutoProceed`). The external-LLM gate (`EvaluateNextActionAutoProceedLLM`) is disconnected from every live path; the gate's intent is delivered to the main agent as a per-turn policy injected via the UserPromptSubmit hook (`nextActionPolicyHint`).
- Context: An external-LLM gate (agy → Gemini 3.5 Flash Fast) was built to judge whether a recommended next action is safe to auto-execute. Measured real-world latency was ~13-25s per call, which exceeds a Stop hook's usable budget (host timeout was raised 5→30 to fit it, but every auto-proceed turn then paid ~14s and frequently hit the 25s internal timeout). This is unusable in practice.
- Decision: Disconnect the LLM gate from the live Stop hook. Auto-proceed is decided by the fast static heuristic alone. The judgment intent moves "up front" into the main agent via prompting: the UserPromptSubmit hook injects a concise next-action/auto-proceed policy every turn so the agent frames its own turn-ending choices (mark `(추천)` only on safe/reversible/confident forward steps; present destructive/irreversible/uncertain actions without recommending them), and the heuristic gate acts on that framing with zero external latency. When auto-proceed does not engage, the Stop hook emits a non-blocking `systemMessage` so the user is told to choose (both Claude and Codex). The Stop hook host timeout is reverted 30→5. The LLM gate code and its tests are preserved (unused) for possible future use behind a faster model.
- Consequences: No per-turn external LLM latency. Auto-proceed quality now depends on (a) heuristic quality and (b) the agent following the injected policy — keep the policy text and `nextActionForwardVerbs`/`nextActionDestructive*` lists aligned. continue:true contract and stop_hook_active anti-loop guard are unchanged; CC/Codex parity holds because all of this is shared core. Do not re-wire `EvaluateNextActionAutoProceedLLM` into a latency-bounded hook without re-checking the model's real latency against the hook timeout.
- Evidence:
  - cmd/harness/hook_user_prompt.go (runHookStop: heuristic-only auto-proceed + systemMessage notice; no LLM call)
  - internal/core/hook_prompt.go (const nextActionPolicyHint, appended in renderHookMCPHintContext)
  - internal/core/next_action_autoproceed_llm.go (DEPRECATED/UNUSED note; preserved)
  - internal/adapter/claude/install.go:149, internal/adapter/codex/install.go:332 (Stop Timeout reverted to 5)
- Alternatives / rejected options:
  - Synchronous LLM gate in the Stop hook with a raised timeout: ~14s per auto-proceed turn and frequent timeouts; unusable UX.
  - Async/advisory LLM logging: adds no runtime gating benefit for the current turn.
  - Deleting the LLM gate code outright: kept instead, since a faster model could make it viable later.
