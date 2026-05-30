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
