# LangChain “Improving Deep Agents with harness engineering” 분석

작성일: 2026-05-30  
원문: https://www.langchain.com/blog/improving-deep-agents-with-harness-engineering  
대상: `agent-harness`의 설계/검증/자가개선 루프 개선 후보 도출  
검증: 본 문서의 claim·인용은 2026-05-30 코드베이스(commit `81b0e9c` 기준) 대조로 확인했다. 인용 7건은 모두 정확했고, 그동안의 구현 변경으로 stale해진 두 곳(아래 §3 "Hook 기반 context delivery", §4 P2)을 정정했다.

추가 구현 검증(2026-05-31): P0 `trace analyze`의 1차 CLI/core/test slice가 현재 코드에 반영되어 있다(`cmd/harness/trace.go`, `internal/core/trace_analyze.go`, `internal/core/trace_analyze_test.go`). 따라서 아래 로드맵은 “미구현 제안”이 아니라, P0 baseline은 완료되고 completion audit·loop detection·context local·reasoning metadata가 남은 상태로 읽는다.

## 1. 원문 핵심 요약

LangChain 글은 모델 자체를 바꾸지 않고 agent를 둘러싼 **harness**만 조정해 coding agent 성능을 끌어올린 사례를 설명한다. 실험에서는 `deepagents-cli`를 Terminal Bench 2.0에서 평가했고, 고정 모델 `gpt-5.2-codex`로 점수를 52.8에서 66.5까지 올렸다고 보고한다. 핵심은 “더 좋은 prompt” 하나가 아니라 다음을 반복적으로 조정하는 시스템 엔지니어링이다.

1. **Trace 기반 실패 분석**: LangSmith trace를 가져오고, 병렬 error-analysis agent들이 실패를 분류한 뒤, main agent가 harness 변경 후보를 합성한다.
2. **Build → self-verify → fix loop 강제**: agent가 그럴듯한 첫 해답에서 멈추지 않도록 task spec 기준 검증, 테스트 실행, full output 판독, 수정 루프를 system prompt와 completion 직전 middleware로 강제한다.
3. **환경 context 주입**: cwd/상하위 directory, available tools, timeout, 평가 기준을 agent가 직접 추론하게 두지 않고 harness가 deterministic하게 제공한다.
4. **doom loop 감지**: 같은 파일을 과도하게 수정하는 pattern을 tool-call hook으로 감지해 “접근을 재고하라”는 context를 주입한다.
5. **reasoning budget 배분**: 모든 단계에 최대 reasoning을 쓰지 않고 planning/verification에 더 많이 쓰는 식으로 token/time budget을 조절한다.

## 2. 원문의 일반화 가능한 주장

| 주장 | 의미 | agent-harness 적용성 |
|---|---|---|
| Harness는 model intelligence를 task 성능/latency/token 효율 목표에 맞게 성형한다. | model 교체보다 prompt, tools, hooks, skills, middleware, flow 변경이 독립 변수다. | 높음. 이 repo도 Go core + thin adapters로 host별 prompt/hook drift를 줄이는 방향이다. |
| Trace는 agent 개선의 outer loop feedback이다. | 실패 trace를 사람이 손으로 보지 않고 반복 가능한 skill/agent loop로 분석한다. | 높음. 현재 self-verify summary/history는 있으나 trace mining 전용 analyzer는 없다. |
| Self-verification은 agentic coding의 핵심 hill-climbing signal이다. | 테스트 결과를 읽고 spec과 비교하는 루프가 agent 성능을 직접 올린다. | 이미 강함. `self-verify` QA gate가 존재하지만, 일반 작업 completion hook 수준의 강제력은 제한적이다. |
| Context engineering은 agent 대신 harness가 해야 한다. | repo 구조, tool availability, constraints, grading criteria를 안정적으로 주입한다. | 이미 일부 구현됨. project-doc catalog/session hook은 이 방향과 일치한다. |
| Guardrail은 현재 모델 약점을 보완하는 임시 설계다. | blind retry, no-test completion, doom loop는 hook/middleware로 줄인다. | 적용 가능. guard/static checks는 있으나 per-file edit-count loop detector는 미구현이다. |
| Model별 harness tuning이 필요하다. | Codex/Claude/Gemini에 같은 prompt/flow를 강요하면 성능 차이가 날 수 있다. | 중요. 이 repo는 Codex/Claude 공통 core를 유지하되 host adapter 차이를 얇게 인정한다. |

## 3. 현재 agent-harness와의 정합성

### 이미 정렬된 부분

- **Host-neutral core + thin adapter**: `internal/core.InstallNative`, `internal/port`, Codex/Claude adapter 경계가 문서화되어 있고, 새 host는 `HostInstaller` 구현체만 추가하는 구조다 (`.agent-harness/ARCHITECTURE.md:48-58`). 이는 LangChain 글의 “harness를 실험 가능한 시스템으로 관리”하는 관점과 잘 맞는다.
- **공통 실행 표면**: CLI, MCP proxy, daemon, skill/hook 통합이 실행 모드로 분리되어 있다 (`.agent-harness/ARCHITECTURE.md:62-71`). 원문의 tools/middleware/skills knob를 host별로 흩뜨리지 않는 구조다.
- **검증 gate**: `agent-harness self-verify`가 테스트뿐 아니라 docs, skill metadata, native integration, redaction, output budget, Mermaid lint를 포함하고 goal score가 target을 넘어야 완료되는 QA gate로 정의되어 있다 (`.agent-harness/TESTING.md:196-198`).
- **Hook 기반 context delivery**: 안정적 project-doc catalog는 SessionStart/PostCompact hook이, dynamic per-turn routing hint(routing/actions/profile/pending upkeep/rule)는 UserPromptSubmit hook이 나눠 주입하는 구조로, 원문의 LocalContextMiddleware와 같은 계열이다 (`cmd/harness/hook_user_prompt.go:225-253`, `:69-83`; `internal/core/hook_prompt.go:104-113`). 카탈로그를 매 turn이 아니라 session 경계로 옮긴 것 자체가 per-turn token 비용을 줄이는 harness tuning이다. (이전 `AGENT_WORKFLOW.md` 서술은 catalog를 UserPromptSubmit에서 주입한다고 했으나 commit `876738f`/`06c7668`에서 session 경계로 이전됐고, 본 분석과 함께 정정했다.)
- **Evidence-first workflow**: 작업 시작 시 현재 파일/명령 출력으로 문서 추정을 검증하고, completion report에 실행 검증을 포함하는 규칙이 있다 (`.agent-harness/AGENT_WORKFLOW.md:8-13`, `:25-31`).
- **자가검증 후보 catalog**: self-verify candidates가 progress heartbeat, redaction audit, coverage gap, rerun recipe, budget baseline, daemon resilience 같은 개선 후보/증거를 명시한다 (`cmd/harness/self_verify_candidates.go:162-179`).

### 부족하거나 다음 개선 후보인 부분

1. **Trace analyzer 1차 구현 완료, outer loop 통합은 남음**
   - 현재는 `agent-harness trace analyze --input <jsonl|state-key> --json`이 self-verify summary/progress JSONL, guard result, doc-upkeep lifecycle event를 “실패 유형 → harness 변경 후보” finding으로 합성한다.
   - 아직 LangSmith식 외부 trace import, 여러 run의 자동 clustering, self-augment 후보 승격까지 이어지는 full outer loop는 남아 있다.

2. **Completion 직전 강제 검증 hook의 강도**
   - Stop hook은 lifecycle reminder 중심이고, host schema 호환을 위해 빈 JSON 또는 작은 reminder를 반환하는 방향이다. 이는 안전하지만, LangChain의 `PreCompletionChecklistMiddleware`처럼 exit를 막고 verification을 강제하는 수준은 아니다.
   - 현 설계와 충돌하지 않으려면 “강제 중단”보다 `agent-harness status/verify-work` 또는 `guard check`를 completion audit에 더 직접 연결하는 쪽이 안전하다.

3. **Doom-loop detector 미구현**
   - Guard는 staged/static source file 검사 중심이다. 같은 파일 edit count, 같은 command retry, 같은 실패 signature 반복을 runtime lifecycle state에서 감지하는 기능은 별도 후보로 남아 있다.

4. **Reasoning budget telemetry 부족**
   - self-verify에는 step duration/budget baseline이 있지만, model reasoning effort를 phase별로 기록·비교하는 contract는 현재 핵심 문서에서 두드러지지 않는다.
   - OMX prompt에는 role별 reasoning effort가 있으나, `agent-harness` core artifact로 평가/비교되지는 않는다.

5. **Benchmark harness의 scoring loop**
   - Terminal Bench 같은 외부 benchmark를 직접 재현할 필요는 없지만, “고정 모델 + harness 변경만 비교”하는 regression protocol은 agent-harness 자체 self-augment에서 더 명확히 표현할 수 있다.

### 원문 5개 기법 → agent-harness 매핑

원문 §1의 5개 harness 기법을 현재 코드의 구체 컴포넌트·gap·제안 knob에 직접 대응시키면 다음과 같다(모든 행은 2026-05-30 코드 대조로 확인).

| 원문 기법 | 기존 컴포넌트 (근거) | gap | 제안 knob |
|---|---|---|---|
| 1. Trace 기반 실패 분석 | self-verify 내부 실패 분류 `classifySelfVerificationFailure`/`selfVerificationFailureClusters` (`cmd/harness/main.go:1866-1912`); `trace analyze` CLI/core/test (`cmd/harness/trace.go`, `internal/core/trace_analyze.go`, `internal/core/trace_analyze_test.go`) | 1차 analyzer는 구현됨. 외부 trace import, 다중 run clustering, self-augment 후보 자동 승격은 남음 | P0 baseline 완료; 다음 knob는 trace import/outer-loop integration |
| 2. Build → self-verify → fix 강제 | `self-verify` QA gate (`.agent-harness/TESTING.md:196-198`), `verify-work` CLI exit 1 (`cmd/harness/status_verify.go:110`), guard exit 3 | Stop hook은 `{}` no-op만 반환 (`cmd/harness/hook_user_prompt.go:316`), completion에 verify를 강제 연결하는 경로 없음 | P1 completion-evidence audit |
| 3. 환경 context 주입 | SessionStart/PostCompact catalog, `preflight`, `AnalyzeProjectSignals` (`internal/core/project_docs.go:609-668`) | 데이터가 여러 명령에 분산, 통합 onboarding view 없음 | P2 `context local` (consolidation) |
| 4. doom loop 감지 | PostToolUse는 passive doc-upkeep recorder (`cmd/harness/hook_user_prompt.go:130`), `DocUpkeepEvent`에 count/retry/signature 필드 없음 | per-file edit count·command retry·failure signature 미기록·미감지 | P1 loop detection (append-only count → hint) |
| 5. reasoning budget 배분 | step duration/step-budget baseline + compare (`cmd/harness/main.go:1082`, `:1175`) | phase별 reasoning-effort telemetry 없음 (`reasoning_effort`는 api-doc-review 입력 플래그뿐) | P2 reasoning/effort metadata |

## 4. 권장 실행 로드맵

### P0 — Trace 분석 산출물 표준화

**목표**: self-verify/self-augment/hook lifecycle 결과를 trace-like evidence로 묶어 실패 유형과 harness 변경 후보를 자동 도출한다.

**현재 상태(2026-05-31 검증)**: 1차 구현 완료. `agent-harness trace analyze --input <jsonl|state-key> --json`은 read-only로 동작하고, file/stdin/state-key 입력을 받아 `trace_analysis` JSON을 출력한다. finding schema는 `failure_class`, `recurring_pattern`, `proposed_knob`, `overfit_risk`, `verification_command`로 고정되어 있으며, fixture tests가 self-verify summary, doc-upkeep JSON/JSONL, state key, empty input rejection, secret redaction을 검증한다.

- 새 CLI 후보: `agent-harness trace analyze --input <jsonl|state-key> --json`
- 입력: self-verify summary, failed step outputs, rerun commands, lifecycle queue, guard findings
- 출력: failure class, recurring pattern, proposed harness knob, overfit risk, verification command
- 검증: fixture 기반 golden test + synthetic failed trace sample

**Acceptance criteria (완료 기준)**:

- `agent-harness trace analyze --input <jsonl|state-key> --json`은 read-only로 동작하고, 입력이 없거나 비면 non-zero exit + usage를 출력한다.
- finding마다 출력 스키마를 고정한다: `failure_class`, `recurring_pattern`, `proposed_knob`, `overfit_risk`, `verification_command`.
- 입력은 self-verify summary JSON과 doc-upkeep queue(jsonl)를 받아들이고, 알 수 없는 필드는 무시한다(forward-compatible).
- secret 원문을 출력에 남기지 않는다(redaction 게이트 통과, Critical Invariant와 일치).
- fixture 기반 golden test + synthetic failed-trace sample로 결정적 출력을 고정한다.
- analyzer는 "제안"만 만들고 구조 변경을 직접 수행하지 않는다(§5 과적합 방지 원칙과 일치).

이 기능은 LangChain의 Trace Analyzer Skill과 가장 직접적으로 대응한다. 단, LangSmith를 재구현하지 말고 기존 state/summary JSON과 optional external trace import만 다룬다. 현재 구현은 이 원칙의 baseline이며, 다음 단계는 여러 trace를 묶어 recurrent pattern을 강화하고 self-augment 후보로 연결하는 것이다.

### P1 — Completion audit 강화

**목표**: agent가 “읽어봤으니 괜찮다”에서 멈추지 않도록, completion 직전에 spec 대비 검증 evidence를 구조화한다.

- 현재 user-facing 지침을 유지하되 `agent-harness verify-work --json -- <command>` 또는 `guard check` 결과를 final report evidence로 요구하는 문서/skill 경로를 강화한다.
- Stop hook이 host 작업을 위험하게 block하지 않도록, 강제 실행 대신 “missing verification evidence”를 lifecycle queue에 남기는 방식이 안전하다.
- self-verify candidate로 `completion-evidence-audit`를 추가할 수 있다.

### P1 — Loop detection middleware 후보

**목표**: 같은 파일/명령/실패 signature에 대한 반복을 runtime lifecycle state로 감지한다.

- PostToolUse hook에서 path/edit count와 command fingerprint를 append-only로 기록한다.
- threshold 초과 시 UserPromptSubmit/PostCompact context에 “접근 재고” hint를 짧게 주입한다.
- 검증: temp state에서 동일 파일 N회 event를 기록했을 때 hint가 생성되는지 unit test.

### P2 — Context discovery 품질 개선

**목표**: LocalContextMiddleware에 해당하는 repo onboarding context를 안전하게 제공한다.

- 이 onboarding 데이터는 신규 기능이 아니라 **이미 여러 명령에 분산 구현돼 있다**: cwd/git root는 `agent-harness preflight --json`(`internal/core/preflight.go:48-72`, `git rev-parse --show-toplevel`)이, language/tool hints와 test/build/lint command 후보(evidence + confidence 포함)는 `agent-harness project bootstrap --dry-run --json`의 `AnalyzeProjectSignals`(`internal/core/project_docs.go:609-668`)가 제공한다. 따라서 `agent-harness context local --json`은 이 둘을 하나의 read-only onboarding view로 묶는 **consolidation/ergonomics** 작업으로 범위를 좁혀야 하며, 중복 데이터 모델을 새로 만들지 않는다.
- shell command discovery는 policy/redaction/audit를 통과해야 한다. `.agent-harness/ARCHITECTURE.md`의 command policy 모델과 맞춰야 한다.

### P2 — Reasoning/compute budget 기록

**목표**: phase별 reasoning effort 선택을 실험 가능한 harness knob로 만든다.

- self-verify/self-augment summary에 model, role, reasoning effort, phase(planning/build/verify)를 optional metadata로 기록한다.
- 비교는 성능 점수뿐 아니라 token/time/timeout risk를 함께 본다.

## 5. 과적합 방지 원칙

LangChain 글도 task-specific overfit을 경계한다. agent-harness에 적용할 때는 다음을 지켜야 한다.

- 특정 benchmark task를 맞추는 prompt hack보다, 실패 유형별 일반 guardrail을 우선한다.
- 새 hook은 작업을 대신 수행하지 않고 context/reminder/state 기록으로 제한한다.
- Codex/Claude host 차이는 adapter에서만 다루고 core policy/DTO 의미는 공유한다.
- 검증은 fixture/golden/CLI smoke로 고정하되, legacy 전체 부채를 한 번에 실패시키지 않는다.
- trace analyzer는 “제안”을 만들고, 실제 구조 결정은 ADR/CAUTIONS 업데이트와 테스트로 고정한다.

## 6. 결론

이 글은 `agent-harness`가 이미 가는 방향—host-neutral core, hook/context delivery, self-verification, evidence-backed lifecycle state—을 강화하는 외부 근거다. P0 경량 analyzer baseline은 구현됐으므로, 가장 가치 있는 다음 단계는 새 model/provider 도입이 아니라 **trace-driven outer loop를 self-augment/verification workflow에 더 직접 연결하는 것**이다. 즉, 실패 trace를 자동으로 분류하고 작은 harness 변경 후보와 검증 명령까지 제안하는 기능을 여러 run·후보 catalog·completion evidence audit와 연결하는 것이 agent-harness 철학과 가장 잘 맞는다.
