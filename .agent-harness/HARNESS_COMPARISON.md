---
name: HARNESS_COMPARISON.md
description: Comparison research against OMC, OMX, and Hermes-style harnesses.
---

# Agent Harness 비교 리서치: OMC / OMX / Hermes 대비 agent-harness

작성일: 2026-05-30 KST  
범위: `agent-harness` 현재 worktree를 기준으로, OMC(oh-my-claudecode), OMX(oh-my-codex), Hermes Agent 계열 하네스와 비교해 부족한 점, 개선이 필요한 점, 바라는 점을 정리한다.

## 1. 요약 결론

`agent-harness`는 **Codex와 Claude Code가 같은 Go binary, 같은 MCP schema, 같은 command-policy/state/project-doc contract를 쓰게 하는 host-neutral 안전 코어**라는 포지션이 뚜렷하다. OMC/OMX처럼 “팀이 알아서 구현까지 밀어붙이는 오케스트레이션 런타임”이거나 Hermes처럼 “항상 켜져 있는 self-improving agent OS”는 아니다.

따라서 현재 강점은 다음이다.

- host-specific plugin에 core logic을 넣지 않는 구조가 명확하다.
- CLI/MCP/daemon response contract와 golden 테스트를 중시한다.
- command 실행을 바로 열지 않고 `policy check`/`fake-run`/audit부터 시작한다.
- repo-local 파일 쓰기를 기본으로 피하고 user-level integration을 기본값으로 둔다.
- upstream companion tool(LLM Wiki, CodeGraph, claude-mem)을 재구현하지 않고 연결한다.

하지만 사용자가 OMC/OMX/Hermes에서 기대하는 “하네스 경험”과 비교하면 부족한 점도 명확하다.

1. **실행 오케스트레이션이 약하다.** worker는 state-only/no-shell이고, 팀/분산 작업/워크트리 통합/장기 작업 실행은 아직 없다.
2. **사용자-facing 워크플로가 적다.** `atomic-commit-push`, `project-bootstrap`, `self-verify`, `self-augment` 외에 autopilot/team/QA/research/design 같은 즉시 체감 skill이 부족하다.
3. **관측성과 UI가 제한적이다.** doctor/contract/self-verify는 좋지만 HUD, team dashboard, mobile/chat gateway, log replay UX는 약하다.
4. **학습 루프가 좁다.** self-verify/self-augment는 harness 품질 개선용이고, Hermes식 “사용 중 생성·개선되는 portable skill/memory”까지는 아니다.
5. **cross-provider/remote/sandbox story가 초기 단계다.** Hermes의 Docker/SSH/Modal/Daytona backend나 OMC/OMX의 tmux worker/mixed-provider team과 비교하면 실행 환경 추상화가 부족하다.

## 2. 근거와 현재 상태

### 2.1 agent-harness 현재 상태

로컬 worktree에서 확인한 사실:

- `./bin/agent-harness inspect --json` 결과: 버전 `0.1.0`, shared skills 4개(`atomic-commit-push`, `project-bootstrap`, `self-augment`, `self-verify`), Codex/Claude user skill과 Codex MCP 설정이 감지됨.
- `./bin/agent-harness contract check --json` 결과: CLI command 20개, MCP tools 36개, contract hash 정상, warning 없음.
- `./bin/agent-harness --help` 결과: `inspect`, `preflight`, `doctor`, `docs`, `policy`, `guard`, `contract`, `state`, `api-doc`, `hook`, `project`, `install-native`, `update`, `bootstrap`, `daemon`, `worker`, `self-verify`, `self-augment`, `mcp`, `version` 제공.
- README와 `.agent-harness/ARCHITECTURE.md`는 worker를 의도적으로 **state-only/no-shell** MVP로 제한한다고 설명한다.
- `.agent-harness/ARCHITECTURE.md`는 핵심 설계를 “외부 Go core + thin host adapters + CLI/MCP/daemon”으로 둔다.

### 2.2 OMC(oh-my-claudecode) 참고점

공식/설치된 문서에서 확인한 핵심:

- OMC docs는 “Claude Code 위의 multi-agent orchestration layer”이며 19 agents와 37 skills를 제공한다고 설명한다. 주요 skill은 `autopilot`, `ralph`, `team`, `ralplan`이다.  
  Source: https://omc.vibetip.help/
- Getting Started 문서는 OMC가 Claude Code plugin으로 설치되며 `omc-setup`, `omc-doctor`, per-project/global setup을 제공한다고 설명한다.  
  Source: https://omc.vibetip.help/docs/getting-started
- 로컬 plugin cache 기준 OMC 4.14.4에는 agent prompt 19개, skill directory 39개, command 27개가 있다.
- OMC README는 `/team` staged pipeline을 `team-plan → team-prd → team-exec → team-verify → team-fix`로 제시하고, `omc team`은 tmux CLI workers(`claude`/`codex`/`gemini`)를 띄우는 별도 runtime이라고 설명한다.

### 2.3 OMX(oh-my-codex) 참고점

공식/설치된 문서에서 확인한 핵심:

- OMX homepage는 OpenAI Codex CLI용 multi-agent orchestration layer로, staged team pipeline, persistent memory/state MCP servers, extensible hooks를 강조한다.  
  Source: https://oh-my-codex.dev/index.html
- OMX GitHub README는 v2의 핵심을 durable `.omx/` state, tmux-aware team execution, real agent catalog, plugin SDK/Codex plugin bridge, first-party hook pack, CLI/MCP/docs/demos/assets로 설명한다.  
  Source: https://github.com/scalarian/oh-my-codex
- 로컬 OMX 0.18.6 plugin cache 기준 skill directory 29개가 있고 `$team`, `$ultragoal`, `$ultraqa`, `$ralph`, `$deep-interview` 같은 workflow skill이 있다.
- `$team` skill은 tmux worker runtime, shared task state/mailbox, worktrees, lifecycle control, long-running parallel execution을 native subagents와 구분한다.
- `$ultraqa`는 단순 build/test 체크가 아니라 adversarial dynamic e2e scenario matrix를 요구한다.
- `$ultragoal`은 `.omx/ultragoal/goals.json`과 `ledger.jsonl`로 durable multi-goal plan/checkpoint를 관리한다.

### 2.4 Hermes Agent 참고점

공식 문서에서 확인한 핵심:

- Hermes는 “server에 살고, 기억하고, 오래 돌수록 능력이 늘어나는 autonomous agent”로 포지셔닝한다.  
  Source: https://hermes-agent.nousresearch.com/
- docs는 built-in learning loop, agent-curated memory, autonomous skill creation, skill self-improvement, cross-session recall, 20+ messaging platforms, cron, isolated subagents, programmatic tool calling, open standard skills, MCP support를 강조한다.  
  Source: https://hermes-agent.nousresearch.com/docs
- GitHub README는 model/provider 자유도, full TUI, messaging gateway, scheduled automations, isolated subagents, Python RPC scripts, local/Docker/SSH/Singularity/Modal/Daytona backends, batch trajectory generation/compression을 강조한다.  
  Source: https://github.com/NousResearch/hermes-agent

## 3. 비교 매트릭스

| 축 | agent-harness 현재 | OMC | OMX | Hermes | 판단 |
|---|---|---|---|---|---|
| 핵심 포지션 | Codex/Claude 공통 Go core + policy/state/docs/MCP | Claude Code skill/agent orchestration | Codex CLI orchestration runtime | long-running self-improving agent OS | agent-harness는 안전한 접착제/계약 코어에 강함 |
| Host portability | Codex + Claude를 같은 binary/schema로 연결 | Claude 중심, CLI team으로 Codex/Gemini 보조 | Codex 중심, mixed provider team 가능 | CLI/gateway 중심, provider-agnostic | agent-harness의 cross-host contract는 차별점 |
| Workflow richness | shared skill 4개 | 37~39개 skill, 19 agents | 29개 skill, team/ralph/ultraqa/ultragoal | memory/skills/cron/gateway/toolsets | 사용자 체감 워크플로는 agent-harness가 가장 빈약 |
| Team/parallel execution | worker state-only/no-shell | native team + tmux CLI workers | tmux team + worktree lifecycle + mixed providers | isolated subagents + terminal backends | agent-harness 최대 격차 |
| Durable goal/state | state checkpoint, self-verify history, project lifecycle state | `.omc` state/memory/notepad/team APIs | `.omx` state/plans/logs/team/ultragoal ledger | persistent memory/session search/skills | agent-harness state는 안전하지만 실행 계획 ledger는 부족 |
| Safety/policy | command policy, fake-run, workspace boundary, redaction 중심 | workflow/hook 중심, Claude permission에 의존하는 면 큼 | hooks/safety presets/team ownership hardening | command approval + container isolation | agent-harness의 policy-first는 강점이나 실제 runner가 없음 |
| Verification | self-verify 95 gate, contract/golden, guard, api-doc gate | verifier/reviewer/team fix loop | UltraQA adversarial QA, Ralph audit, goal completion audit | long-running learning/evaluation trajectory | agent-harness는 harness 자체 검증 강함, product behavior QA 약함 |
| Context/code intelligence | docs index/project docs route; companion CodeGraph opt-in | LSP/AST/python repl tooling | explore/codegraph-like surfaces, hooks, state | web/browser/vision/toolsets/MCP | 재구현하지 않는 정책은 좋지만 integrated UX는 약함 |
| Install/update | Go binary + user-level Codex/Claude symlink/MCP/hooks | Claude plugin install/setup/doctor | npm/Codex plugin setup/doctor | one-line installer + setup/doctor/update | agent-harness는 안전하나 marketing-level first-run polish 부족 |
| UI/remote | CLI/MCP only | tmux/team panes, HUD류 | HUD/tmux/team state | TUI, messaging gateway, remote backends | user-visible control plane 부족 |

## 4. 모자란 점

### 4.1 “하네스”보다 “안전한 공통 library/adapter”에 가깝다

현재 agent-harness는 매우 좋은 기반이지만, 사용자가 OMC/OMX/Hermes를 보고 기대하는 것은 “작업을 맡기면 계획→실행→검증→수정 루프가 돈다”는 end-to-end 경험이다. agent-harness는 `self-verify`와 `self-augment`를 제외하면 일반 개발 작업을 실제로 분해·실행·검증하는 first-class workflow가 거의 없다.

### 4.2 Worker가 실행을 하지 않는다

`worker enqueue/status/list/cancel`은 state-only/no-shell이라 안전하지만, 경쟁 하네스의 핵심 가치인 long-running job, cancel/retry, isolated execution, queue, worktree, merge/integration을 제공하지 않는다. command policy가 준비돼 있지만 “정책을 통과한 명령을 제한적으로 실행하는 runner”까지 이어지지 않아 UX가 끊긴다.

### 4.3 팀/분산 작업의 파일 소유권과 통합 모델이 없다

OMX는 worker별 worktree, integration report, mixed-provider team을 명시하고 OMC도 native team과 tmux CLI team을 제공한다. agent-harness는 Codex/Claude 공통 contract는 좋지만, 여러 agent가 동시에 수정할 때의 ownership, conflict, merge, verification evidence를 관리하는 runtime artifact가 없다.

### 4.4 Product QA workflow가 부족하다

`go test`, golden, contract, guard는 harness 개발에는 적합하다. 그러나 사용자 repo에서 기능을 검증하는 adversarial scenario matrix, interactive CLI smoke, stale state/cancel/resume/dirty worktree/hung command/flaky test 같은 QA skill은 별도 구현되어 있지 않다. OMX `$ultraqa`가 가진 “실패를 찾기 위한 QA”와 차이가 크다.

### 4.5 Persistent memory/skill learning이 외부 의존이다

claude-mem, LLM Wiki, CodeGraph를 재구현하지 않는 결정은 옳다. 다만 Hermes식 “작업 중 배운 것을 skill로 승격하고, skill 사용 중 스스로 개선하며, memory nudge를 한다”는 UX는 agent-harness 안에서 통합된 lifecycle로 보이지 않는다. companion tool은 설치되지만, 언제 어떤 지식을 어디에 저장/승격할지 정책이 약하다.

### 4.6 Provider/model/runtime abstraction이 부족하다

현재 목적은 Codex/Claude 공통성이지만, Hermes/OMX는 OpenAI-compatible provider, Claude/Codex/Gemini workers, SSH/Docker/Modal/Daytona backend처럼 실행 장소와 model provider를 더 넓게 추상화한다. agent-harness가 이 방향으로 바로 확장할 필요는 없지만, 최소한 “adapter 추가 contract”와 “runner backend contract”가 더 명확해져야 한다.

### 4.7 UX/문서 정보 구조가 내부자 지향이다

README와 `.agent-harness` 문서는 정확하지만, 초심자에게 “그래서 오늘 어떤 명령으로 어떤 효과를 얻나”가 OMC/OMX/Hermes보다 덜 강하다. 4개 shared skill은 품질은 높지만, showcase/demo/tutorial/recipes가 적어 가치 전달이 약하다.

## 5. 개선이 필요한 점: 우선순위 제안

### P0 — 지금 설계 원칙을 유지하면서 빈틈 메우기

1. **경쟁 비교/로드맵 문서 유지**  
   이 문서를 `.agent-harness` source of truth로 두고 ADR/TECH_STACK/OPERATIONS에 후속 결정만 반영한다.

2. **Workflow taxonomy를 명확히 나누기**  
   agent-harness가 직접 제공할 것과 companion tool에 위임할 것을 표로 고정한다.
   - 직접 제공: cross-host install, contract, command policy, state, docs route, lifecycle hook, audit, minimal job lifecycle.
   - thin wrapper로 제공: CodeGraph/LLM Wiki/claude-mem invocation and health.
   - 제공하지 않음: full wiki engine, AST graph engine, memory compression engine.

3. **`agent-harness doctor`를 경쟁력 있는 status hub로 강화**  
   현재 doctor는 진단 중심이다. OMC/OMX HUD처럼 다음을 한 화면에서 보여주면 체감이 커진다.
   - Codex/Claude integration status
   - MCP/daemon status
   - companion tool status(CodeGraph/LLM Wiki/claude-mem)
   - latest self-verify score/history
   - pending project-doc upkeep queue
   - known risk/warning summary

### P1 — 안전한 실행 runtime으로 확장

4. **policy-backed real runner MVP**  
   fake-run 다음 단계로, allowlisted read-only command부터 실제 실행한다. write/network/shell은 별도 flag와 audit reason을 요구한다. 이때 secret redaction, timeout, cwd boundary, env allowlist, output truncation을 contract로 고정한다.

5. **worker job queue를 no-shell에서 policy-runner로 확장**  
   바로 arbitrary shell worker로 가지 말고 단계화한다.
   - Phase A: read-only command jobs only
   - Phase B: repo-local write jobs with explicit `write_allowed` and dry-run evidence
   - Phase C: isolated worktree jobs
   - Phase D: cancel/timeout/retry/orphan cleanup

6. **worktree-first team prototype**  
   OMC/OMX를 그대로 복제하지 말고 agent-harness식으로 “작은 coordinator”만 둔다.
   - `worker enqueue --worktree`로 isolated branch/worktree 생성
   - 각 job의 claimed files/evidence/test command 기록
   - integration report 생성
   - merge는 기본 dry-run, confirm 필요

### P2 — 사용자-facing workflow 확장

7. **`verify-work` skill/CLI 추가**  
   self-verify가 harness 자체 검증이라면, `verify-work`는 대상 repo 변경 검증용이다. 최소 범위:
   - changed files 요약
   - 관련 test/build/lint 후보 제안
   - stale state/dirty worktree/secret scan/guard check
   - evidence matrix 출력

8. **`ultraqa-lite` 또는 `adversarial-check` 제공**  
   OMX UltraQA 전체를 복제하지 말고, agent-harness의 guard/policy/test 기반으로 hostile scenario matrix template과 cleanup evidence만 제공한다.

9. **`research-compare` workflow 정식화**  
   이번 작업처럼 외부 하네스와 비교하는 연구형 산출물을 반복할 수 있게, sources/evidence/matrix/recommendation 템플릿을 만든다.

### P3 — memory/learning 통합 UX

10. **companion tool lifecycle router**  
    “이 사실은 `.agent-harness/CAUTIONS.md`에 기록”, “이 반복 작업은 skill candidate”, “이 긴 연구는 LLM Wiki”, “이 세션 기억은 claude-mem”처럼 목적별 저장소를 안내/실행하는 thin router를 만든다.

11. **skill promotion pipeline**  
    Hermes식 autonomous skill creation을 그대로 따라 하기보다, 안전하게 다음 gate를 둔다.
    - observation → candidate skill note
    - human-readable rationale
    - quick_validate/test
    - user/global install plan dry-run
    - explicit promotion

### P4 — remote/UI/ops

12. **remote backend는 SSH read-only부터**  
    Hermes처럼 다양한 backend를 바로 지원하지 말고, SSH target inventory + read-only doctor/status부터 시작한다.

13. **HUD/dashboard는 CLI JSON을 재사용**  
    새 UI를 만들기보다 `doctor --json`, `worker list --json`, `state list --json`, `self-verify history --json`을 합친 `status --json` 또는 `dashboard` command를 먼저 만든다.

## 6. 바라는 점: 제품 방향

### 6.1 “작고 안전한 공통 control plane”이 되어야 한다

agent-harness가 OMC/OMX/Hermes를 모두 따라잡으려 하면 철학과 충돌한다. 대신 다음 문장이 제품 정체성에 맞다.

> Codex, Claude Code, 그리고 companion tools 사이에서 동일한 정책·상태·문서·검증 contract를 제공하는 안전한 local control plane.

### 6.2 경쟁 하네스를 대체하지 말고 붙인다

- OMC/OMX: orchestration runtime으로 쓰되, agent-harness는 preflight/policy/doctor/project-doc/contract gate를 제공한다.
- Hermes: long-running personal agent/gateway로 쓰되, agent-harness는 repo-local 안전 경계와 verification gate를 제공한다.
- CodeGraph/LLM Wiki/claude-mem: upstream engine으로 쓰되, agent-harness는 설치 상태와 routing policy를 제공한다.

### 6.3 “실행 전 정책, 실행 중 관측, 실행 후 검증”을 제품 슬로건으로 삼을 수 있다

OMC/OMX/Hermes가 실행력을 강조한다면 agent-harness는 다음 세 가지에서 차별화할 수 있다.

1. 실행 전: command/workspace/secret policy check
2. 실행 중: worker/job/audit/state ledger
3. 실행 후: contract/self-verify/project-doc/guard evidence

## 7. 구체적 로드맵 후보

| 순서 | 산출물 | 왜 지금 필요한가 | 검증 |
|---|---|---|---|
| 1 | `.agent-harness/HARNESS_COMPARISON.md` 유지 | 경쟁 대비 방향성을 문서화 | 문서 존재, sources/evidence 포함 |
| 2 | `agent-harness status --json` | doctor/inspect/state/self-verify를 한 화면으로 | 구현됨: golden test + CLI smoke |
| 3 | `policy run --read-only` | fake-run에서 실행 가능한 안전 MVP로 이동 | 구현됨: timeout/redaction/boundary tests |
| 4 | `worker run --read-only` | long-running job queue의 첫 실용 단계 | 구현됨: command evidence가 있는 read-only job |
| 5 | `verify-work` CLI | 일반 repo 변경 검증 UX 제공 | 구현됨: preflight + guard + optional read-only command |
| 6 | companion status/router | wheel 재발명 없이 통합 UX 강화 | skip/missing/installed cases |
| 7 | worktree job prototype | team 실행 격차 축소 | dirty worktree preservation + merge dry-run |
| 8 | skill promotion gate | Hermes식 learning을 안전하게 변형 | quick_validate + install dry-run |

## 8. 리스크와 하지 말아야 할 것

- **OMC/OMX 복제 금지:** team/autopilot/ultraqa를 무작정 복제하면 agent-harness의 host-neutral safety core 정체성이 흐려진다.
- **Hermes 복제 금지:** memory/gateway/browser/toolset/skills hub를 한 프로젝트에 다 넣으면 범위가 폭발한다.
- **실제 shell runner를 성급히 열지 말 것:** 현재 command policy가 강점이므로 runner는 read-only부터 단계적으로 열어야 한다.
- **repo-local 파일 쓰기 기본값 변경 금지:** 지금의 user-level 기본, project-local explicit opt-in은 유지해야 한다.
- **companion tool source of truth 중복 금지:** LLM Wiki/CodeGraph/claude-mem core 기능은 upstream에 둔다.

## 9. 완료 기준 관점의 결론

이번 비교에서 확인한 `agent-harness`의 최우선 과제는 “더 많은 agent prompt를 추가”가 아니라, **안전 policy와 실제 실행/관측/검증 loop 사이의 빈 구간을 좁히는 것**이다. 가장 좋은 다음 구현은 `status --json`과 `policy run --read-only`/`worker run --read-only` 계열이다. 이 둘은 현재 architecture를 해치지 않으면서 OMC/OMX/Hermes 대비 가장 큰 체감 격차인 runtime/observability를 줄인다.

## 10. 2026-05-30 구현 메모

로드맵의 첫 실행 slice로 `status --json`, `policy run --read-only`, `worker run --read-only`, `verify-work` CLI를 추가했다. 범위는 의도적으로 안전한 argv-only read-only 실행에 한정한다. write/network/arbitrary shell/background queue/worktree merge는 아직 열지 않았고, 후속 phase의 명시적 policy contract와 테스트가 필요하다.

## 11. Qwen Code / DeepSeek-Reasonix 계약 강화 레이어 (2026-05-30 리서치)

범위: OMC/OMX/Hermes 비교(섹션 1~10)에 더해, Qwen Code(`QwenLM/qwen-code`)와 DeepSeek-Reasonix를 분석해 **host-neutral safety core + 바퀴 재발명 금지** 철학에 맞는 정책/계약 조각만 흡수한다. 두 도구의 내부 동작 주장은 리서치 기반 **미검증(unverified)**이며, 흡수 대상은 "엔진"이 아니라 거기서 분리한 정책/계약/측정 조각이다.

### 11.1 흡수 핵심 테마 3가지

1. **정책 게이트를 boolean → 명명 계약으로 승격.** Qwen의 tiered approval / plan-mode 차단 / delegation 권한 격탈을 *실행기를 만들지 않고* 단일 policy evaluator가 해석하는 명명 enum 계약(`PolicyTier`)으로 흡수. 출처: https://github.com/QwenLM/qwen-code
2. **응답·입력 계약을 cross-host 안정 스키마로 고정.** Qwen `llmContent vs returnDisplay`(→ `model_content vs display`) + `error.type` enum, Reasonix tool-call repair의 *검증 조각만* 떼어 contract/golden에 고정.
3. **컨텍스트 byte-determinism을 측정 가능한 불변식으로 강제.** Reasonix 3-region(Immutable Prefix / Append-Only Log / Volatile Scratch) 규율을 *캐시 엔진 복제 없이* region mutability 메타 + guard anti-pattern rule + golden byte-identical 회귀로 흡수.

### 11.2 우선순위(기존 P0~P4 위에 겹치는 계약 강화 레이어)

| 그룹 | 앵커 아이디어 | 출처 | 적용 표면 | value |
|---|---|---|---|---|
| P0 결정성·안전 기본값 | byte-determinism guard 3종 + golden 회귀, daemon `socket_perm_ok` 노출, MCP inputSchema 정규형 검증 | Reasonix/Qwen | guard·contract·daemon | high |
| P1 정책 강화 | `PolicyTier` 5단계 enum(YOLO/AUTO 제외), plan-mode 부작용 차단, delegation 권한 격탈, Storm 반복호출 신호, ambiguous-edit 사전 deny | Qwen/Reasonix | policy·CLI·MCP·audit | high |
| P2 사용자 workflow | `model_content vs display` + `error.type` enum, `did_you_mean`/path-canonical repair, `--stdin` Unix filter, `docs resolve --json` import 트리, skill path-gating | Qwen | contract·cli·mcp·docs | high |
| P3 컨텍스트·학습 | `ContextRegionContract`(통합 `CONTEXT_BUDGET.md`), `serialization_stability` 메트릭→95점 gate, uncertainty tiebreaker, 구조화 lesson harvesting, sandbox profile "선언만" ladder | Reasonix/Qwen | state·self-verify·self-augment·docs | med |

### 11.3 채택하지 않을 것 (reject)

| reject 항목 | 출처 | 사유 / 흡수 가능한 조각 |
|---|---|---|
| data-plane 공유 daemon(모델 세션·컨텍스트 HTTP+SSE 공유) | Qwen `qwen serve` | 모델 세션 호스팅은 host 책임 → host-neutral 경계 붕괴. 단 "control-plane만 공유" 비-목표를 ADR에 명문화. |
| provider hot-swap 추상화(ContentGenerator/modelProviders) | Qwen | provider 라우팅은 Codex/Claude가 이미 함 = 바퀴 재발명. 단 redaction을 provider-neutral 휴리스틱으로 일반화. |
| 세션 자동 승격(Proceed Always→AUTO_EDIT) / YOLO bypass | Qwen | 1회 승인→세션 전체 안전등급 상승은 "명시적 opt-in으로만 write/network/shell" 기본값 약화. per-invocation opt-in만 허용. |
| 메모리 Dream/Extract 백그라운드 압축 엔진 | Qwen | 전문 메모리 엔진 = upstream claude-mem 책임. install-native 카탈로그 라우팅만. |
| prefix-cache 빌링 최적화 + self-consistency N-sample 분기 | Reasonix | single-provider 추론/빌링 런타임 복제 = host-neutral·runner 금지 위배. byte-determinism·region 분리·측정 조각만 P0/P3로 흡수. |
| R1 reasoning distill 엔진(secondary 모델 호출) | Reasonix | core가 모델 직접 호출 = provider 결합 + golden 불가. plan-state 저장 스키마 슬롯만 흡수. |

reject 원칙: **"모델을 호출하거나, 단일 provider에 결합하거나, 세션 단위로 안전등급을 자동 상승시키는 것"은 전부 거부**하고, 분리 가능한 정책/계약/측정 조각만 흡수한다.

### 11.4 첫 실행 slice (2026-05-30 착수)

P3 `ContextRegionContract`의 토대로, 실행 경로·정책 결정을 바꾸지 않는 **직렬화 byte-determinism 회귀 안전망**을 먼저 깐다. 범위: (1) `docs`/`contract` JSON 빌더의 2회 호출 byte-identical golden 회귀, (2) guard `nondeterministic-context-serialization` rule 1종, (3) 직렬화 산출물 region/mutability 메타 자리 확보(noop). 검증은 golden diff empty + `contract check` warning 0건 + self-verify 95점 유지.

구현 결과(2026-05-30):
- `internal/core/context_region.go`: `Region*` 상수 + `VolatileContextFields` + `StableProjection`/`StableProjectionJSON`/`ContextSerializationStable` 추가. volatile field 어휘를 response-contract golden의 dynamic time key 정규화와 단일 source of truth로 통합.
- `nondeterministic-context-serialization` guard rule을 `// harness:immutable-prefix` marker opt-in + `volatile-ok` 면제로 추가(severity `warn`, 노이즈 0).
- 발견한 실제 결함: `DocsIndex.generated_at`가 immutable 콘텐츠에 인터리빙되어 golden이 `$TIMESTAMP`로 가리고 있었다. stable projection으로 분리해 회귀 테스트가 콘텐츠 drift를 잡도록 했다.
- byte-determinism 강제를 docs_index뿐 아니라 `contract_schema`, MCP `tools`/`resources` 등 **모든 재사용-context 빌더**로 확장(`ContextSerializationStable`).
- **P0 guard 3종 중 2종 의도 변경**: `prefix-region-reorder`(warn)와 `summary-replace-on-prefix`(block)는 Go map 순회/prefix 재배치 같은 의미 분석이 필요해 deterministic regex guard로는 신뢰성 있게 탐지할 수 없다. 코드 패턴을 추측하는 대신, **출력 byte-determinism을 데이터 레벨에서 직접 강제**(`ContextSerializationStable` 회귀)하는 방식으로 같은 보장을 더 확실히 달성한다. 향후 AST adapter가 생기면 code-level rule을 보강 신호로 추가할 수 있다.

### 11.5 P1 앵커 `PolicyTier` (2026-05-30 착수)

Qwen Code의 tiered-approval 모델을 흡수해 흩어진 capability 불리언을 host-neutral 명명 tier로 합성했다.

- `internal/core/policy.go`: `PolicyTier{Name, GrantedCapabilities, Rationale}` 타입 + `read_only`/`workspace_write`/`network_access`/`shell_exception` 상수 + `resolvePolicyTier`. `CommandPolicyEvaluation.Tier`로 모든 표면(CLI `policy check`/`run`, MCP `command_policy_check`/`fake_run`, `policy audit`)에 자동 노출. `CommandPolicySummary().tiers`로 래더를 `harness://command-policy` resource에 문서화.
- **순수 additive 분류**: tier는 요청이 시도할 수 있는 *권한 envelope*만 이름 붙이고, deny 로직·개별 명령 판정(`deny_reasons`)은 전혀 바꾸지 않는다. tier는 명령 허용 여부가 아니라 요청 플래그(write/network/shell)의 함수다.
- **YOLO/AUTO 의도적 제외**: 1회 승인이 세션 전체 안전등급을 올리는 자동 승격 tier는 만들지 않는다(reject 표 §11.3 원칙 준수).
- 검증: 8개 플래그 조합 → tier 매핑을 table test로 고정(`TestPolicyTierClassifiesEveryFlagCombination`). golden은 `tier` 객체 추가 + contract hash 갱신만 변동. self-verify 220/220 유지.
- 후속(같은 P1 그룹, 미착수): plan-mode 부작용 차단 게이트, delegation_context 권한 격탈, capability_profile monotone narrowing.
