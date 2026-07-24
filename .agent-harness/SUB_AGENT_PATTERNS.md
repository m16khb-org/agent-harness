# Sub-Agent Usage Patterns

> **원칙:** 메인 에이전트가 직접 작업을 수행한다. Sub-agent는 메인 컨텍스트를 오염시키지 않고, 다른 관점이 필요하거나, 병렬화가 의미 있거나, 다른 권한·모델이 필요한 경우에만 예외적으로 사용한다.

---

## 근거 요약

### 현대 에이전트 방법론 수렴점 (2024-2026)

| 출처 | 핵심 발견 |
|------|-----------|
| **Anthropic 공식 문서** | Sub-agent: "context isolation. Work happens separately, only summary returns." Skill vs Subagent vs Agent Team 4계층 분류. |
| **SWE-agent** (Yang et al., arXiv:2405.15793) | 단일 에이전트 구조로 SOTA 달성. Interface design이 model size보다 중요. |
| **OpenHands** (Wang et al., ICLR 2025) | Multi-agent 지원하지만 "multi가 single보다 낫다"는 주장 없음. |
| **LangChain DeepAgents** (Trivedy, 2026.2) | Harness engineering으로 52.8→66.5 향상. 내부 실행 루프는 단일 에이전트 + middleware. 병렬 sub-agent는 오직 외부 개선 루프에만 사용. |
| **Claude Code Workflows** | 동시 16 / 총 1,000 agent. batch/audit 용 — 각 sub-agent가 독립 작업일 때만. |
| **SWE-bench 리더보드** | 최상위권 전부 단일 에이전트 시스템. Multi-agent가 single-agent를 이긴 사례 없음. |
| **Anthropic 가이드** | "For tasks where the scope is clear and the fix is small, ask Claude to do it directly." |

### 핵심 결론

1. **메인 에이전트 직접 작업이 기본** — SWE-bench, LangChain DeepAgents 실증 데이터, Anthropic 공식 가이드 모두 이 방향으로 수렴.
2. **Sub-agent는 예외적 최적화** — Context isolation, adversarial review, 병렬 독립 작업, 권한 분리 등 특정 상황에서만 net-positive.
3. **Sub-agent spawning overhead > 직접 작업 비용** 인 경우가 더 흔함. 작은 작업, 컨텍스트 의존 작업, 교차 판단이 필요한 작업은 메인이 직접.

---

## 12가지 Net-Positive Sub-Agent 패턴

`agent-harness issueops execution decide`와 MCP `issueops_record_execution_decision`은 아래 slug만 허용한다. Sub-agent 계획은 이 slug 중 하나, 기대 이득, 알려진 tradeoff, 그리고 그 tradeoff에도 불구하고 net-positive인 이유를 함께 기록해야 한다.

| # | Pattern | Validation slug |
|---|---------|-----------------|
| 1 | 대량 탐색 (High-Volume Exploration) | `high-volume-exploration` |
| 2 | 격리 작업 (Isolated Worktree Work) | `isolated-worktree-work` |
| 3 | Forked Context 탐색 (Forked Context Exploration) | `forked-context-exploration` |
| 4 | 악마의 변호인 (Devil's Advocate / Adversarial Review) | `devils-advocate-review` |
| 5 | 교차 검증 / 합의 (Cross-Verification / Consensus) | `cross-verification-consensus` |
| 6 | 병렬 독립 탐색 (Parallel Independent Research) | `parallel-independent-research` |
| 7 | 작업 분해·조정 (Task Fan-out with Coordination) | `task-fan-out-coordination` |
| 8 | 장시간 백그라운드 작업 (Background Long-Running Work) | `background-long-running-work` |
| 9 | 모델 특화 / 비용 라우팅 (Model Specialization / Cost Routing) | `model-specialization-cost-routing` |
| 10 | 도구·권한 제한 (Tool/Permission Gating) | `tool-permission-gating` |
| 11 | 계획-실행 분리 (Plan-then-Execute Separation) | `plan-then-execute-separation` |
| 12 | Triage → 전문가 라우팅 (Triage → Specialist Routing) | `triage-specialist-routing` |

허용된 기대 이득 slug: `context_isolation`, `parallel_speed`, `fresh_review`, `tool_gating`, `long_running`, `model_specialization`, `isolated_worktree`.

### Category A: 컨텍스트 오염 방지

#### 1. 대량 탐색 (High-Volume Exploration)
- **설명:** 수십~수백 파일 검색·읽기 결과가 메인 컨텍스트를 오염시키는 경우. Sub-agent가 자체 컨텍스트에서 탐색 후 요약만 반환.
- **근거:** Claude Code Explore (built-in, Haiku, read-only). Anthropic: "Use one when a side task would flood your main conversation with search results, logs, or file contents you won't reference again."
- **agent-harness 적용:** Von Neumann Phase 1 (Ground)의 explorer/librarian subagent. 대규모 코드베이스 패턴 검색.

#### 2. 격리 작업 (Isolated Worktree Work)
- **설명:** Git worktree로 분리된 공간에서 독립적 편집. 메인 작업공간과 충돌 방지.
- **근거:** Claude Code `isolation: worktree`. IssueOps의 worktree 격리 패턴.
- **agent-harness 적용:** IssueOps implement phase의 worktree 기반 작업.
- **D1 delegated child cycle 적용:** parent가 `issueops execution decide`에 `isolated-worktree-work` 또는 `task-fan-out-coordination`을 기록한 뒤, child별 isolated worktree와 scoped session binding을 만든다. Child는 `issueops resume --id <child-id>`로 자기 cycle만 재개하고, parent가 결과를 검증해 accept/reject한다.

#### 3. Forked Context 탐색 (Forked Context Exploration)
- **설명:** 메인 대화의 전체 컨텍스트를 복제한 뒤, 분기 탐색만 하고 메인을 오염시키지 않음.
- **근거:** Claude Code forked subagents.
- **agent-harness 적용:** 검토 필요 — 현재 미구현. Claude Code 전용 기능.

### Category B: 다른 관점 필요

#### 4. 악마의 변호인 (Devil's Advocate / Adversarial Review)
- **설명:** 작업한 모델이 스스로 채점하지 않도록, fresh model이 결과를 반박·검증.
- **근거:** Claude Code "code-reviewer" 예제. Anthropic: "a second opinion — a verification subagent or dynamic workflow that checks its own findings has a fresh model try to refute the result."
- **agent-harness 적용:** Turing Final Quality Gate의 Reviewer. IssueOps pr phase의 reviewer agent.

#### 5. 교차 검증 / 합의 (Cross-Verification / Consensus)
- **설명:** 같은 문제를 여러 sub-agent가 독립적으로 풀고 결과 비교. Multiple angles → higher confidence.
- **근거:** Claude Code dynamic workflows: "cross-checks their results, for work that needs more than a single pass."
- **agent-harness 적용:** Von Neumann Further Review 옵션. 복수 reviewer의 합의.

### Category C: 병렬화 의미

#### 6. 병렬 독립 탐색 (Parallel Independent Research)
- **설명:** 서로 의존성 없는 read-only 조사를 동시에 여러 방향으로 진행. Wall-clock speedup.
- **근거:** Claude Code "Run parallel research." DeepAgents: "Parallelize tasks — Delegate to general or specialized subagents running in isolated context windows."
- **agent-harness 적용:** Von Neumann Research intent의 "fan out exploration subagents."

#### 7. 작업 분해·조정 (Task Fan-out with Coordination)
- **설명:** 의존성 있는 작업을 lead agent가 분해·할당·동기화. Agent Teams 패턴.
- **근거:** Claude Code agent teams, Google ADK Parallel workflow templates.
- **agent-harness 적용:** 규모가 크고 자연스럽게 분해되는 마이그레이션·audit 작업에만 제한적 사용.
- **D1 delegated child cycle 적용:** parent issue가 child cycle들을 만든 뒤, main agent가 dependency order와 final acceptance를 소유한다. 각 child dispatch 전에는 `task-fan-out-coordination` 실행 결정을 durable record에 남긴다.
- **Native fan-out과 child cycle 적용:** 일시적 독립 fan-out은 host-native concurrency로 수행한다. durable delegation은 isolated canonical worktree와 generation-fenced ownership을 가진 IssueOps child cycle로 기록하고 parent가 accept/reject validation을 수행한다.

#### 8. 장시간 백그라운드 작업 (Background Long-Running Work)
- **설명:** 메인 대화를 차단하지 않고 비동기 실행. 진행상황 체크·취소 가능.
- **근거:** OpenAI Sandbox Agents, DeepAgents async subagents.
- **agent-harness 적용:** agent-harness worker system (no-shell lifecycle MVP). draft-wiki worker.

### Category D: 다른 권한·모델 필요

#### 9. 모델 특화 / 비용 라우팅 (Model Specialization / Cost Routing)
- **설명:** 싼 모델(Haiku)로 탐색·검색, 비싼 모델(Opus)로 추론. 작업 특성에 맞는 모델 선택으로 비용 최적화.
- **근거:** Claude Code `model: haiku|sonnet|opus` per subagent. Explore → Haiku, Plan → main model.
- **agent-harness 적용:** Von Neumann의 Agent Categories (quick/deep/visual-engineering).

#### 10. 도구·권한 제한 (Tool/Permission Gating)
- **설명:** Sub-agent에 Read-only 도구만 주고 Write/Bash 차단. 탐색 중 실수로 파일 변경 방지.
- **근거:** Claude Code `tools: [Read, Glob, Grep]`. Explore/Plan subagent의 도구 제한.
- **agent-harness 적용:** Von Neumann의 read-only explorer subagent. Shell·write 권한 없는 탐색 전용 worker.

#### 11. 계획-실행 분리 (Plan-then-Execute Separation)
- **설명:** Plan 단계는 read-only + research에 집중, Execute 단계는 write·bash 사용. 권한 분리.
- **근거:** Claude Code Plan subagent (built-in, read-only).
- **agent-harness 적용:** Von Neumann(planner)와 Turing(executor)의 분리. 이미 구조적으로 구현됨.

### Category E: 전문 도메인 분리

#### 12. Triage → 전문가 라우팅 (Triage → Specialist Routing)
- **설명:** 고객응대 시스템 등에서 도메인별 전문 sub-agent로 라우팅. 주문조회·환불·FAQ 각각 전문가.
- **근거:** OpenAI Agents SDK handoffs. 실제 프로덕션 고객응대 시스템에서 검증됨.
- **agent-harness 적용:** 현재 agent-harness 범위 밖 (코딩 에이전트 하네스). 미래 multi-domain 확장 시 검토.

---

## Sub-agent가 Net-Negative인 경우 (금지 패턴)

| 상황 | 이유 | 근거 |
|------|------|------|
| 단일 파일 소규모 편집 | Spawning overhead > 직접 비용 | Anthropic: "scope clear and fix small → do it directly" |
| 전체 대화 컨텍스트 필요 | Sub-agent는 기본적으로 빈 컨텍스트로 시작 | Claude Code: sub-agents don't inherit conversation history |
| 복합 다단계 추론 | Sub-agent는 단순·독립 작업용 | Claude Code: general-purpose subagent needed for "complex reasoning to interpret results" |
| 교차 아키텍처 판단 | 코드베이스 전체 이해 필요 | SWE-bench: single-agent 우세 |
| 안전·되돌림·정렬 결정 | 메인 에이전트의 판단 책임 | agent-harness CAUTIONS: "main agent owns safety, reversibility, user-intent alignment" |
| Sub-agent 중첩 (nesting) | 복잡도 폭발, 무한 루프 위험 | Claude Code: Plan subagent prevents nesting |
| 메인·서브 간 중복 작업 | Anti-Duplication Rule 위반 | Von Neumann SKILL.md: "DO NOT perform the same search yourself" |

---

## Turing 스킬에의 적용

### 현재 (잘못된) 모델
```
YOU ARE A CONDUCTOR. NOT A SOLO PERFORMER.
Delegate EVERY code edit, test write, bug fix, QA to workers.
```

### 수정된 (올바른) 모델
```
YOU ARE THE MAIN AGENT. You write code, fix bugs, run QA directly.
Spawn sub-agents ONLY for context-isolated work (12 patterns).
NEVER delegate work that requires your full context or judgement.
```

### 구체적 변경 사항

| 섹션 | 현재 | 변경 |
|------|------|------|
| `<identity>` | "CONDUCTOR. NOT A SOLO PERFORMER." | "MAIN AGENT. You perform the work directly." |
| `<identity>` | "delegate every code edit, test write, bug fix, QA" | "You write code, fix bugs, write tests, and drive QA yourself." |
| Delegation Model 표 | 모든 작업 → worker 매핑 | 좌측: "메인 직접" / 우측: "Sub-agent (12 cases)" 구분 |
| Execution Loop | "DELEGATE-IN-PARALLEL" | "EXECUTE-DIRECTLY, delegate only isolated tasks" |
| Constraints | "DELEGATE all code edits" | "PERFORM work directly. Sub-agents only per 12 patterns." |

---

## 참고 문헌

- Anthropic. "Create custom subagents." docs.anthropic.com/en/docs/claude-code/sub-agents
- Anthropic. "Orchestrate subagents at scale with dynamic workflows." docs.anthropic.com/en/docs/claude-code/agents
- Anthropic. "Introducing dynamic workflows in Claude Code." claude.com/blog, May 2026.
- Yang, J. et al. "SWE-agent: Agent-Computer Interfaces Enable Automated Software Engineering." arXiv:2405.15793, 2024.
- Wang, X. et al. "OpenHands: An Open Platform for AI Software Developers as Generalist Agents." arXiv:2407.16741, 2024 (ICLR 2025).
- Trivedy, V. "Improving Deep Agents with harness engineering." LangChain Blog, Feb 2026.
- OpenAI. "Agents SDK — Handoffs." openai.github.io/openai-agents-python
- Google. "Agent Development Kit — Parallel Workflow." google.github.io/adk-docs
- SWE-bench Leaderboard. swebench.com
- Frankynwa. "Hermes Model Benchmark." github.com/Frankynwa/hermes-model-bench, May 2026.

## 사전 리뷰 게이트 (S2, 2026-06-12 규약화)

**코드 변경을 동반하는 플랜은 구현 착수 전 2-pass 적대 리뷰를 통과해야 한다:**

1. **critic pass(정확성)**: 플랜의 사실 주장·파일 참조·잊은 소비자를 코드 대조로 반박 시도.
2. **verifier pass(검증-적정성)**: 각 task의 acceptance가 "테스트는 통과하나 목적은 silent 실패"를
   허용하지 않는지(양방향 테스트, CLI-경로 강제, fail-closed 음성 케이스, binary 기준) 검사.

근거(실측, 2026-06-11): pioneer 벤치마크 플랜에서 이 2-pass가 구현 전에 blocker 3건을 차단했다 —
잊힌 CLI 소비자(FromFixture), map-vs-단일객체 decode 버그, N/A silent no-op 가능성. 셋 다 구현 후
발견 시 재작업 비용이 컸을 결함이다.

규칙: 리뷰 지적은 맹목 수용하지 않고 코드로 직접 검증 후 반영한다(receiving-code-review 원칙).
같은 활성 컨텍스트의 자기승인은 금지 — 리뷰어는 fresh-context 서브에이전트여야 한다.
