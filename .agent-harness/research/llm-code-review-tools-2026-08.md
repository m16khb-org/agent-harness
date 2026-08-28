# Research: LLM 코드 리뷰 도구·연구의 비용/오탐 설계 — parnas 에 가져올 것

## TL;DR
**결론**: 2025–26 년의 리뷰 도구들은 예외 없이 (1) 후보 생성과 검증을 분리하고, (2) 컨텍스트를 자유 탐색이 아니라 큐레이션된 팩·그래프로 주며, (3) 임계값·중복 제거를 코드로 처리하고, (4) 👍/👎를 규칙·임베딩으로 되먹인다. parnas 의 현재 구조(팩 × 샤드, prescreen, 2단계 tracer, 역할별 모델)는 이 흐름과 일치하며, 즉시 가져올 차이점은 **검증자의 정보 비대칭(finder 증거를 주지 않기)**, **모델 자기평가 점수 배제**, **반박 이력 임베딩 기반 억제**, **incremental 재리뷰** 네 가지다.
**Confidence**: High (핵심 패턴은 5개 이상 독립 출처가 합의) / 개별 수치는 벤더 자체 보고가 많아 Medium.
**Sources**: 독립 출처 ~90개, 핵심 주장 12개 중 Confirmed 9, Single-sourced 3.

## Method
- 3개 각도 병렬 조사(2026-08-28): ① OSS 소스 직접 읽기(PR-Agent, Kodus) + CodeRabbit 문서 ② 상용 제품 문서·블로그(Claude Code Review/ultrareview, Bugbot, Greptile, Ellipsis, Sourcery, Copilot) ③ arXiv 논문·엔지니어링 글(오탐, 비용, 실행 검증, 학습 루프, Anthropic 캐싱 가이드).
- 교차검증: 같은 주장이 벤더 문서 + 독립 논문/제3자 글에 모두 있을 때만 Confirmed.

## Findings

### F1. 후보 생성 ↔ 검증 분리는 업계 표준이고, 검증자는 finder 보다 *적은* 컨텍스트를 봐야 한다 — **Confirmed**
- Claude Code Review: "verification step checks candidates against actual code behavior to filter out false positives" (code.claude.com/docs/en/code-review). Bugbot v1: 8회 랜덤 순서 패스 → 다수결 → validator 모델 (cursor.com/blog/building-bugbot). Ellipsis: gatekeeper 가 "already handled somewhere the reviewer did not look / speculative" 를 거부, 검증 모델은 opus 권장 — "verifying a claim against real code is the same job as finding one" (ellipsis.dev/docs/code-review/gatekeeper). CodeRabbit: verification agents + grep/ast-grep 로 증거 스크립트 생성 (coderabbit.ai/blog/...massive-codebases). Kodus: safeguard 단계 (docs-internal/planning-agent-first-review.md).
- **OpenCodeReview (arXiv 2608.09290, Alibaba)**: 반박자(reflector)가 "sees only the diff, not the agent's tool-augmented exploration" — 정보 비대칭으로 자기강화 편향을 끊음. AACR-Bench 정밀도 25.2–37.8% vs Claude Code 7.2–15.9%, 토큰 5–15× 절감.
- **Adversarial Review (arXiv 2608.18167)**: reviewer+critic 3명이 5명 팀을 이김; 명시적 반대 없이는 "false-consensus" 로 수렴.
- parnas 대비: 우리 tracer 는 후보 JSON 에 finder 의 `evidence`·`upstream`·`downstream` 을 그대로 받는다 → 반박자가 finder 의 추적을 재확인만 하고 끝날 유인. **개선: tracer 1차에는 path/line/title/why 만 주고 evidence 는 숨긴다(정보 비대칭). confirm(opus) 단계에서만 finder 증거와 1차 반박을 함께 보여 대조.**

### F2. 모델의 자기 심각도/신뢰도 점수는 믿지 말고, 임계값은 코드에 둔다 — **Confirmed**
- Greptile: LLM-as-judge 심각도(1–10, <7 폐기)가 "nearly random", 프롬프트로 nit 줄이기 실패 (greptile.com/blog/make-llms-shut-up).
- arXiv 2608.02677: 프롬프트에 비용/정책을 넣으면 보고 확률이 13.6–16.9pp 이동 → "Elicit risk without policy, apply cost thresholds in code."
- 반례로 PR-Agent 의 self-reflection 0–10 점수는 작동한다고 보고(qodo docs, self_reflection.md) — 단 "무엇이 0점인가"를 열거한 루브릭이 있을 때. **Single-sourced(벤더)**.
- parnas 대비: 현재 rubric(25/50/75/90) 은 근거 종류로 정의돼 있어 F2 와 정합. 다만 `severity_adjust` 를 skeptic 에게 맡기는 부분은 근거 없이 흔들릴 수 있음 → **severity 는 category+scenario 로 코드에서 결정하거나, 조정 시 근거 path:line 필수.**

### F3. 컨텍스트는 "큐레이션된 팩/그래프" 가 자유 탐색과 diff-only 둘 다를 이긴다 — **Confirmed**
- OpenCodeReview: rule-guided 파일 디스패치 + 큐레이션 도구 + 파일 단위 병렬 서브에이전트 → 5–15× 토큰 절감·정밀도 상승. cubic: 도구 프루닝(<10% 사용 도구 제거) + 마이크로에이전트 → 오탐 51% 감소 (cubic.dev/blog/learnings-from-building-ai-agents). Greptile/CodeRabbit/Kodus 모두 정의·호출·import 그래프를 사전 계산해 "focused subgraph" 를 준다.
- PR-Agent `dynamic_context`: hunk 를 감싸는 함수/클래스 헤더까지 위로 확장(최대 10줄) — 팩의 diff 를 "함수 경계" 로 넓히는 값싼 기법 (git_patch_processing.py).
- 반례: OpenAI 는 repo 접근+실행이 더 강하다고 주장(alignment.openai.com/scaling-code-verification) — 단 "targeted hypothesis + checks", 무제한 탐색이 아님.
- parnas 대비: 팩 구조가 정확히 이 방향. 5차 실측(Opus): finder 턴 105→27, cache_read 146.6M→28.3M. **추가 개선: (a) hunk 를 enclosing function 경계로 확장해 팩에 넣기, (b) 팩에 "이 파일과 같이 바뀌는 파일"(co-change, CodeRabbit) 을 git log 로 붙이기.**

### F4. 검증 비용은 finder 보다 훨씬 작게 만들 수 있고, 실행 검증이 정밀도의 핵심 — **Confirmed(정성) / 수치는 Single-sourced**
- OpenAI: 반증은 "targeted hypothesis generation and checks" 라 생성자 토큰의 일부로 고심각도 대부분을 잡음; 52.7% 코멘트가 코드 변경으로 이어짐. Anthropic ultrareview: "every reported finding is independently reproduced and verified". Anthropic Code Review 오탐 <1%(내부 보고, Single-sourced).
- parnas 대비: reproducer 가 실제로 `npx tsx`·`SchemaObjectFactory` 로 재현한 5차 finding 들이 이 패턴. **reproducer 를 sonnet 으로 내리는 결정은 "재현 실패 ≠ 반박" 규칙 덕에 안전(F1 의 Ellipsis 와 반대 방향이지만 역할이 다름 — Ellipsis gatekeeper 는 우리 tracer confirm 에 해당하며 그건 opus 유지).**

### F5. 싼 모델 앙상블은 recall 을 올리지만 precision 은 못 올린다 — **Confirmed**
- SWR-Bench: Gemini Flash 5회 종합 F1 20.5%@$0.0037 vs Pro 1회 19.4%@$0.0059 — recall +119%, precision 동일. Bugbot v1 의 8패스 다수결도 같은 논리.
- parnas 대비: **finder 등급을 낮추면 recall 보상을 위해 패스 수를 늘려야 하고, precision 은 verify 가 책임진다.** 6차에서 contract=sonnet, intent=fable 로 둔 실험의 판정 기준은 "확정 finding 수·심각도 / $" 여야 함(토큰만 보면 안 됨).

### F6. 학습 루프: 반박·👎 이력을 텍스트 유사도가 아니라 임베딩/규칙으로 되먹인다 — **Confirmed**
- Greptile: 팀별 벡터 DB, 👎 3개 이상과 코사인 유사 → 차단, 👍 3개 이상 → 통과; address rate 19%→55%. 보안/메모리누수/검증 코멘트는 절대 억제 안 함. Kodus: 임베딩 k-means(≤50 클러스터), 만장일치 부정 클러스터 → DISCARD, `SIMILARITY_THRESHOLD 0.6`, KODY_RULES/BREAKING_CHANGES 는 항상 유지 (kodyFineTuning.service.ts). Bugbot: 👎·답글·사람 리뷰 코멘트 → 후보 규칙 → 유입 PR 로 평가 후 승격, 부정 신호 누적 시 비활성(44k 규칙). CodeRabbit: 자연어 "learnings" 를 매 코멘트 생성 전 주입. Claude Code Review: `REVIEW.md` 에 "Verification bar / nit cap / skip path".
- parnas 대비: `priorLessons` 를 bigram ≥0.5 로 비교하는 prescreen 은 너무 거칠다(3차·5차 모두 prescreen 0건). **개선: (a) 반박된 후보를 `refuted.jsonl` 로 누적하고 임베딩(또는 최소 TF-IDF) 유사도로 prescreen, (b) security/data 카테고리는 억제 예외, (c) `rule_candidates` 를 Bugbot 식 "후보 규칙 → 다음 MR 에서 평가 → 승격" 으로.**

### F7. Incremental 재리뷰와 게이팅이 반복 비용을 결정한다 — **Confirmed**
- Ellipsis: "A review covers the commits since the last review, never the whole pull request again." Sourcery: 자동 재리뷰 5회 상한. Claude: push 모드에서 해결된 finding 자동 정리, 트리거 모드(once/every push/manual). Bugbot: 이전 코멘트를 컨텍스트로 사용, 이전 실행과 dedupe.
- parnas 대비: 같은 MR 을 head 가 바뀔 때마다 전체 재실행(오늘 5회). **개선: `prior_review_threads` + 이전 `workflow-result.json` 을 읽어 (a) 이전 finding 라인이 diff 에서 바뀌었으면 "해결 확인" 만, (b) 새 커밋이 건드린 파일의 샤드만 재조사.**

### F8. 캐싱은 prefix 안정성과 워밍업 순서가 좌우 — **Confirmed(문서)**
- Anthropic 캐싱 문서: 순서 tools→system→messages, 마지막 동일 prefix 블록에 breakpoint, 동시 요청은 "a cache entry only becomes available after the first response begins" → 병렬 fan-out 전에 1회 워밍업. Claude Code 서브에이전트: fork 는 부모 캐시 재사용. Greptile v3: 컨텍스트 3× 늘리고도 캐시 hit 로 추론비 75% 절감.
- parnas 대비: FINDER_COMMON 을 앞에 두는 현재 배치는 맞음. **개선: 첫 finder 1명을 먼저 띄워 캐시를 만든 뒤 나머지를 fan-out(pipeline 의 첫 항목만 await).** 효과는 미측정.

### F9. 비용 실측 참고치
- Anthropic Code Review $15–25/리뷰(~3M Opus 토큰, ~20분), ultrareview $5–25, Bugbot 평균 $1–1.5/실행, Greptile $1/리뷰(초과분), CodeRabbit ≈$0.25/파일. parnas 5차(Opus 전량): 48.8M cache_read + 7.4M create + 38K out ≈ $72 정가 기준 — Claude Code Review 의 3배. **6차 혼합 구성의 목표는 $30–40.**

## Cross-Check Results
- ≥2 독립 출처 확인: F1, F2(반례 포함), F3, F5, F6, F7, F8 — 7
- Single-sourced: Anthropic "<1% 오탐", OpenCodeReview 의 구체 수치, Bugbot 해결률 80% — 3 (모두 벤더/저자 보고)
- Disputed: "repo 접근+실행이 큐레이션 팩보다 낫다"(OpenAI) vs "결정성이 이긴다"(OpenCodeReview) — 실제로는 "제한된 표적 실행" 으로 수렴.

## Adversarial Review
- "검증 단계가 오탐을 줄인다" 는 주장에 대한 반증 시도: CR-Bench(arXiv 2603.11078)에서 Reflexion 에이전트는 recall 을 올리고 SNR 을 5.11→1.95 로 **낮췄다**. 즉 "자기 성찰로 더 찾기" 는 오탐을 늘리고, "독립 검증자로 지우기" 만 정밀도를 올린다. parnas 의 critic(더 찾기) 단계는 반드시 verify 를 거치게 해야 하며, critic 자체가 정밀도를 올리진 않는다 — 현재 설계와 일치.

## parnas 적용 우선순위 (근거 → 작업)
| 순위 | 작업 | 근거 | 예상 효과 |
|---|---|---|---|
| 1 | tracer 1차에 finder evidence 숨기기(정보 비대칭); confirm 에서만 대조 | F1 | 오탐 억제, 비용 중립 |
| 2 | 반박 이력 임베딩 prescreen + security/data 억제 예외 | F6 | 반복 오탐 제거(현재 0건 작동) |
| 3 | incremental 재리뷰(바뀐 샤드만) | F7 | 같은 MR 재실행 비용 -60~80% |
| 4 | hunk → enclosing function 확장, co-change 파일 목록 | F3 | finder 탐색 턴 추가 감소 |
| 5 | fan-out 전 캐시 워밍업 1명 | F8 | cache_create 감소(미측정) |
| 6 | severity 조정에 근거 path:line 필수 | F2 | 심각도 흔들림 방지 |

## Open Questions
- 싼 모델 finder(sonnet/fable) 의 "확정 finding / $" — 6차(wf_243b4bfc) 실측 대기.
- 반박 이력 임베딩을 어디에 둘지(로컬 `.agent-harness/issues/*/review/` 는 gitignore 됨 → 레포 공유 저장소 필요).

## Source Index
(각 조사 에이전트의 인덱스를 합침 — 전부 2026-08-28 조회)
- Claude Code Review docs https://code.claude.com/docs/en/code-review · ultrareview https://code.claude.com/docs/en/ultrareview · 플러그인 https://github.com/anthropics/claude-code/blob/main/plugins/code-review/README.md · InfoQ https://www.infoq.com/news/2026/04/claude-code-review/
- Cursor Bugbot https://cursor.com/blog/building-bugbot · https://cursor.com/blog/bugbot-learning · https://cursor.com/blog/may-2026-bugbot-changes · docs https://cursor.com/docs/bugbot
- Greptile https://www.greptile.com/blog/make-llms-shut-up · https://www.greptile.com/blog/greptile-v3-agentic-code-review · https://www.greptile.com/docs/how-greptile-works/memory-and-learning.md · graph https://www.greptile.com/docs/how-greptile-works/graph-based-codebase-context.md
- Ellipsis https://www.ellipsis.dev/docs/code-review/gatekeeper · https://www.ellipsis.dev/docs/code-review/reviewers · https://www.nsbradford.com/blog/how-we-built-ellipsis
- Sourcery https://docs.sourcery.ai/reviews/anatomy-of-a-review/ · https://docs.sourcery.ai/reviews/review-rules/
- Copilot https://docs.github.com/en/copilot/concepts/agents/code-review · https://github.blog/changelog/2026-08-07-copilot-code-review-effort-levels-are-generally-available/
- PR-Agent 소스 https://raw.githubusercontent.com/qodo-ai/pr-agent/main/pr_agent/algo/pr_processing.py · utils.py · git_patch_processing.py · settings/configuration.toml · pr_code_suggestions_reflect_prompts.toml · docs core-abilities/{compression_strategy,self_reflection,dynamic_context}.md
- CodeRabbit https://docs.coderabbit.ai/knowledge-base/learnings.md · https://docs.coderabbit.ai/reference/configuration · https://docs.coderabbit.ai/tools/list · https://www.coderabbit.ai/blog/how-coderabbit-delivers-accurate-ai-code-reviews-on-massive-codebases · https://theaiengineer.substack.com/p/how-coderabbit-actually-works
- Kodus https://raw.githubusercontent.com/kodustech/kodus-ai/main/libs/kodyFineTuning/infrastructure/adapters/services/kodyFineTuning.service.ts · .../agents/engine/dedup-prompt.ts · classify-severity.ts · docs-internal/planning-agent-first-review.md · https://docs.kodus.io/how_to_use/en/code_review/learning/kody_learning.md
- 논문 arXiv 2608.09290(OpenCodeReview) · 2608.18167(Adversarial Review) · 2607.03316(CodeRabbit in the wild) · 2603.11078(CR-Bench) · 2509.01494(SWR-Bench) · 2601.19072(HalluJudge) · 2501.15134(BitsAI-CR) · 2608.02677(정책→확률 이동) · 2601.14470(Tokenomics) · 2601.19494(AACR-Bench)
- 엔지니어링 https://alignment.openai.com/scaling-code-verification/ · https://www.cubic.dev/blog/learnings-from-building-ai-agents · https://www.augmentcode.com/blog/how-we-built-high-quality-ai-code-review-agent · https://tianpan.co/blog/2026-05-05-llm-code-review-production-diff-pipeline · 독립 실측 https://dev.to/_vjk/best-ai-code-reviewer-in-2026-we-ran-4-in-parallel-for-3-weeks-146-prs-679-findings-1c0f
- Anthropic https://platform.claude.com/docs/en/docs/build-with-claude/prompt-caching · https://code.claude.com/docs/en/sub-agents · https://www.anthropic.com/engineering/multi-agent-research-system
