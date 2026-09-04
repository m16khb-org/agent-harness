# Pioneer 스킬 평가기준·issueops 통합 다각도 진단 보고서

작성일: 2026-06-11
작성자: main agent (조사 기반, evidence-bound)
조사 범위: 평가기준 적합성 / 추가 기준 필요성 / 스킬 단독 충분성 / issueops 내 pioneer 스킬 호출·퍼포먼스

이 보고서의 모든 주장은 레포 파일·라인 근거에 묶여 있다. 근거 등급은 rubric의 A/B/C 정의를 따른다
(A=라이브 산출물, B=레포/CLI 소스+정적 검증, C=경계 시뮬레이션).

---

## 0. 한 줄 결론

현재 평가기준(rubric) 자체는 **잘 설계된 단일-스킬 격리 측정기**다. 그러나 품질 시스템에는
**서로 분리된 세 측정 계층** 사이에 구조적 공백이 있다 — 격리된 스킬 품질(scorecard)은 4.92/5로 천장에
닿았다고 선언됐지만, **issueops 안에서 pioneer 스킬이 실제로 호출되어 산출물을 바꾸는지는 어떤 계층도
측정하지 않는다.** 따라서 "스킬만으로 충분한가"의 답은 **아니오**이며, 다음 투자처는 스킬 본문이 아니라
**오케스트레이션(통합) 측정**이다.

---

## 1. 현재 측정 시스템의 실제 구조 (근거 기반)

품질을 떠받치는 측정 계층은 세 개다. 셋은 서로를 검증하지 않는다.

### 계층 A — Pioneer Skill Quality Rubric / Scorecard (격리 측정)
- 위치: `.issueops/operations/pioneer-skill-quality-rubric.md`, `...-scorecard.md`
- 측정 방식: fresh-context 서브에이전트에 **target SKILL.md + holdout 케이스 + fixture만** 주입하고
  main 평가자가 5개 dimension으로 채점 (rubric 88–124행).
- 현재 수치: holdout 평균 4.80 → 4.92/5, 9개 스킬 모두 ≥4.8 (scorecard 49–51, 95–114행).
- 성격: **스킬 본문이 격리 상태에서 정확한 산출물을 내는가**만 측정. activation(서술 매칭)·오케스트레이션·
  호스트 환경은 설계상 우회된다(스킬을 직접 주입하므로).

### 계층 B — IssueOps Quality Benchmark (산출물 측정)
- 위치: `internal/core/issueops/benchmark/*.go`, 설계 `docs/superpowers/specs/2026-06-02-issueops-quality-benchmark-design.md`
- 측정 방식: fixture(`testdata/issueops/fixtures/*.json`)의 user_prompt에 대한 **artifact bundle**(issue
  draft, plan, PR draft 등)을 deterministic check + LLM judge(agy)로 12개 dimension 채점.
- 12개 dimension: intent_understanding, issue_quality, plan_quality, task_decomposition, tdd_quality,
  subagent_orchestration, implementation_readiness, pr_mr_quality, phase_control_quality,
  branch_worktree_gate_quality, isolation_compliance, worktree_cleanup_quality (design 79–92행).
- 성격: **issueops가 내놓은 결과물의 품질**을 측정. 어떤 스킬이 그 결과를 만들었는지는 보지 않는다.

### 계층 C — IssueOps Skill Contract Test (텍스트 존재 측정)
- 위치: `internal/core/issueops/issueops_skill_contract_test.go`
- 측정 방식: `skills/issueops/SKILL.md` **문자열 안에** 9개 pioneer 스킬 이름과 핵심 구절
  ("7-step Hopper Method", "1NF→BCNF", "O(n²)→O(n log n)" 등)이 들어 있는지 `strings.Contains`로 검사
  (test 12–59행).
- 성격: **SKILL.md가 스킬을 언급하는지**만 보증. 런타임 호출·기여는 전혀 보지 않는다.

```
계층 A: "스킬 본문이 격리 상태에서 동작하는가"  → 4.92/5  (측정됨)
계층 B: "issueops 산출물이 좋은가"             → 12 dim   (측정 인프라 존재)
계층 C: "SKILL.md가 스킬 이름을 적었는가"       → text grep (측정됨)
        ─────────────── 공백 ───────────────
        "issueops 런 중 올바른 스킬이 실제로 호출되어 산출물을 바꿨는가" → 어디에서도 측정 안 됨
```

---

## 2. 각 질문에 대한 진단

### Q1. 지금의 평가기준이 적합한가?

**부분적으로 적합. 단일-스킬 품질 측정기로는 강하지만, 시스템 품질 측정기로는 불완전하다.**

강점(유지할 것):
- rubric은 falsifiable / discriminative(1점 분리) / repeatable(±0.5) / evidence-bound / anti-gaming(holdout)
  / safety-aware의 self-quality-bar를 갖춘다 (rubric 20–35행). 일반 평가 척도보다 엄격하다.
- gate flag(unsafe/stale-contract/fake-tool/hollow-method/overfit 등)로 좋은 산문이 심각한 결함을 가리지
  못하게 한다 (rubric 245–261행).
- 근거 등급 A–D와 "D는 최종 점수 불가" 규칙으로 추정만으로 통과하는 것을 막는다 (rubric 262–273행).

한계(보완 필요):
1. **격리 측정의 맹점.** rubric은 SKILL.md를 직접 주입하므로 (a) 실제 서술(description) 기반 activation이
   되는지, (b) 다른 스킬과의 핸드오프가 되는지를 측정하지 못한다. 이 맹점은 이미 실증됐다 —
   `pioneer-skill-optimization-strategy.md`의 firsthand 도그푸드에서 **서브에이전트 holdout이 놓친 결함들**
   (code-quality-metrics의 ugrep 명령 오작동 27–34행, algorithm-optimization/database-design/git-operations의 "묻힌 결정 게이트" 124–135행)이 직접
   사용에서야 드러났다. 즉 격리 측정은 firsthand/통합 측정이 잡는 것을 구조적으로 놓친다.
2. **단일-런 분산.** 4.92는 single-run holdout 점수다. n=3 분산 재측정에서 git-operations 4.77, algorithm-optimization 4.53으로
   내려간다 (scorecard 52–53, 89–91행). 즉 "모두 ≥4.8"은 표본 1회의 진술이고, gate(≥4.2)는 통과하나 4.92라는
   가족 평균은 분산을 평탄화한 낙관적 수치다.
3. **"천장 도달" 결론의 적용 범위 오해 위험.** scorecard 113–114행은 "body edit의 실질적 천장"이라 결론한다.
   이는 **계층 A(격리) 한정으로만 참**이다. 통합(계층 B↔C 공백)은 측정조차 안 됐으므로 천장 근처가 아니라
   바닥(미측정)이다. 이 결론이 "더 할 게 없다"로 읽히면 위험하다.

판정: rubric은 폐기 대상이 아니다. 그러나 **"스킬 품질 = 시스템 품질"이라는 암묵적 등치**가 rubric의 적용
한계를 넘어선 결론을 낳고 있다.

### Q2. 품질 향상을 위해 새 평가기준을 도입할 필요가 있는가?

**있다. 단, 기존 rubric의 5개 dimension을 늘리는 게 아니라, 측정 *대상*을 격리 스킬에서 오케스트레이션으로
확장해야 한다.** 구체적으로 다음 3종의 기준이 비어 있다.

1. **Activation Fidelity (활성 충실도)** — 실제 서술/트리거로 스킬이 *맞게 켜지고 안 켜지는가*. 현재 rubric의
   "Request Fit" dimension(165–181행)은 개념적으로 이를 노리지만, 측정 시 SKILL.md를 직접 주입하므로 실제
   activation을 검증하지 못한다. issueops 런 중 "DDL 변경인데 database-design가 호출됐는가 / 알고리즘 아닌데 algorithm-optimization가
   오작동(overbroad)하지 않았는가"를 보는 기준이 필요하다.

2. **Orchestration / Handoff Quality (오케스트레이션·핸드오프 품질)** — issueops SKILL.md의 Phase Assist
   Map(SKILL.md 44–54행)은 implementation-planning→database-design→algorithm-optimization→...→code-quality-metrics→verified-execution의 파이프라인을 규정한다. 그러나
   스킬 A의 산출물이 스킬 B의 입력으로 *실제로 올바르게 전달되는지*를 보는 기준이 없다. 이는 rubric의
   `hollow-method` gate(스킬이 방법을 언급하나 산출물을 바꾸지 않음, 256행)를 **issueops 수준으로 끌어올린**
   개념이다 — 현재는 단일 스킬 안에서만 적용된다.

3. **Integration Contribution (통합 기여도)** — 같은 issueops fixture를 (a) pioneer 스킬 라우팅 있음 vs
   (b) 없음으로 돌려 12개 dimension 점수 차이를 보는 A/B 기준. 이게 있어야 "pioneer 스킬이 issueops 품질을
   실제로 올린다"가 주관이 아닌 측정이 된다. 계층 B 벤치마크가 이미 A/B 비교 인프라
   (`CompareIssueOpsBenchmarkRuns`, gate_test)를 갖췄으므로 **재발명이 아니라 fixture/artifact 확장**으로 닿는다.

### Q3. 단순히 스킬만으로 충분한가?

**아니오. 두 가지 이유로 스킬 본문은 시스템 품질의 일부일 뿐이다.**

1. **load-bearing 게이트는 스킬이 아니라 Go 하네스에 있다.** issueops의 강제력(intent_contract,
   branch_link_verified, `pr-readiness --strict`, Korean remote artifact 훅 가드)은 SKILL.md "Always-On
   Rules/Gate Quick Reference"(SKILL.md 99–141행)에 *문서화*돼 있을 뿐, 실제 차단은 deterministic check와
   PreToolUse/Stop 훅(Go)이 수행한다. 스킬은 advisory, 하네스가 enforcing이다. 스킬만 좋아도 게이트가 없으면
   계약이 무너진다.
2. **스킬은 하네스 결함을 드러내지만 고치지 못한다.** firsthand 도그푸드가 debugging+git-operations로 비-hermetic
   golden 결함(`optimization-strategy.md` 100–107행)을 진단했으나, 수정은 Go 변경이며 스킬 범위 밖이다. 즉
   품질 천장은 스킬 본문이 아니라 **스킬↔하네스 경계의 정확성**이 결정한다.

결론: 스킬은 "방법의 질"을 책임지고, 하네스 게이트·벤치마크는 "그 방법이 실제로 강제·기여되는지"를 책임진다.
지금 후자가 pioneer 스킬에 대해 비어 있다.

### Q4. issueops에서 CS pioneer 스킬이 적절히 호출되어 적절한 퍼포먼스를 보이는가?

**현재 증거로는 "보증되지 않음(unverified)"이 정확한 답이다. 보이는 게 아니라 *측정된 적이 없다*.**

근거:
- issueops와 pioneer 스킬의 유일한 자동 연결은 **계층 C 텍스트 contract test**다 — SKILL.md가 스킬 이름을
  적었는지만 검사(test 35–54행). 이름이 적혀 있다고 호출되는 것은 아니다.
- 계층 B 벤치마크의 fixture는 5개 모두 워크플로 계약(worktree-gate, evidence-contract, feedback-loop,
  remote-feature, ambiguous-intent)이며 **pioneer-skill 호출을 검증하는 fixture는 0개**다
  (`testdata/issueops/fixtures/` 실측).
- 벤치마크 artifact 스키마·12개 dimension 어디에도 pioneer-skill 호출/기여 필드가 **없다**
  (`issueops_benchmark.go`/`_score.go` grep 결과 0건; design 79–92행).
- `.issueops/ISSUEOPS_AUDIT.md`는 pioneer 스킬을 **한 번도 언급하지 않는다**(grep 0건).
- 계층 A scorecard는 스킬을 issueops 밖 격리 상태로만 측정한다.

즉 "issueops가 algorithm-optimization/database-design/debugging를 적시에 호출해 산출물을 개선한다"는 명제는 **참도 거짓도 아닌 미측정**
상태다. Phase Assist Map은 잘 설계된 *의도*이지만, 그 의도가 런타임에 실현되는지에 대한 evidence는
계층 어디에도 없다. rubric 자신의 기준으로 보면 이 명제는 근거 등급 **D(추정)** 이며, rubric은 D를 최종
판정에 쓰지 못하게 한다(rubric 273행). 따라서 "적절한 퍼포먼스를 보인다"고 보고하는 것은 rubric 위반이다.

---

## 3. 다각도 종합: 측정 토폴로지의 공백

```
        [활성]            [방법의 질]          [통합/기여]          [강제]
Q:  서술로 켜지는가?   본문이 정확한가?   런 중 산출물 바꾸나?  게이트가 막나?
A:  미측정             계층 A 4.92/5      미측정(핵심 공백)      계층 B 일부 + 훅
                      (단일런·격리)                            (스킬 아님, Go)
```

- 가장 잘 측정된 곳(계층 A, 방법의 질)이 가장 많이 투자됐고 천장에 닿았다.
- 가장 덜 측정된 곳(활성·통합 기여)이 정확히 사용자가 묻는 곳이며, 추가 투자 대비 효용이 가장 높다.
- 계층 B 벤치마크는 통합을 측정할 **인프라는 갖췄으나 pioneer-skill용으로 *조준되지 않았다.***

---

## 4. 권고 (우선순위, 모두 가역적·증거 기반)

| 순위 | 권고 | 닿는 질문 | 재사용 자산 | 비고 |
|------|------|-----------|-------------|------|
| 1 | **pioneer-skill 호출 fixture 추가**: 알고리즘/DDL/디버그/스키마 시나리오 fixture를 만들고 expected에 "해당 pioneer 스킬의 distinctive 산출물(복잡도 클래스, write-penalty, 7-step 진단 등)"을 요구 | Q4, Q2 | 계층 B fixture 스키마, deterministic check | issueops가 스킬을 *부르는지*를 산출물 특징으로 간접 검증 |
| 2 | **Integration Contribution A/B 게이트**: 같은 fixture를 스킬 라우팅 on/off로 돌려 12 dim 델타 측정 | Q2, Q4 | `CompareIssueOpsBenchmarkRuns`, autoresearch gate | "스킬이 issueops 품질을 올린다"를 측정으로 전환 |
| 3 | **scorecard 4.92에 적용범위 단서 명시**: "격리 holdout single-run 한정. 통합·활성은 미측정"을 본문에 못박아 '천장=완료' 오독 차단 | Q1 | scorecard 본문 | 1줄 honesty note, 즉시 가능 |
| 4 | **분산을 1급 시민으로**: holdout을 n≥3로 표준화하고 평균±분산으로 보고(현재 일부만 n=3) | Q1 | 기존 변산 재측정 절차 | single-run 낙관 제거 |
| 5 | **firsthand 도그푸드를 정례 채널로**: 격리 holdout이 놓친 결함(ugrep, 묻힌 게이트)을 도그푸드가 잡았으므로 사이클마다 1회 firsthand 패스를 rubric에 추가 | Q1, Q3 | `optimization-strategy.md` 방법론 | 격리 맹점 보완 |

권고 1–2가 핵심이다. 둘 다 **새 측정 프레임워크를 만드는 게 아니라** 이미 있는 계층 B 벤치마크를
pioneer-skill 쪽으로 조준하는 확장이며, 레포 철학("바퀴를 재발명하지 않는다")에 부합한다.

## 5. 명시적 비-주장 (정직성)

- 본 보고서는 issueops가 pioneer 스킬을 호출하지 *않는다*고 주장하지 않는다. **호출 여부가 측정된 적 없다**고
  주장한다(미측정 ≠ 실패).
- 계층 A 4.92 수치를 부정하지 않는다. 그 수치의 *적용 범위*가 격리·단일런이라는 점만 명확히 한다.
- 권고는 모두 fixture/문서/측정 확장이며 스킬 본문 재작성을 요구하지 않는다(스킬 본문은 천장 근처라는
  scorecard 결론을 그대로 수용).

## 부록 A — 근거 인덱스

| 주장 | 파일:라인 | 등급 |
|------|-----------|------|
| rubric self-quality-bar 존재 | rubric 20–35 | B |
| 격리 주입 측정 방식 | rubric 88–124 | B |
| holdout 4.92 단일런 | scorecard 49–51, 95–114 | B |
| n=3 분산 4.77/4.53 | scorecard 52–53, 89–91 | B |
| firsthand가 격리 맹점 적발 | optimization-strategy 27–34, 124–135 | B |
| Phase Assist Map(의도) | issueops/SKILL.md 44–68 | B |
| contract test=텍스트 존재만 | issueops_skill_contract_test.go 35–54 | B |
| 벤치마크 12 dim에 스킬 필드 없음 | benchmark design 79–92; grep 0건 | A |
| fixture 5개에 스킬 호출 없음 | testdata/issueops/fixtures/ 실측 | A |
| ISSUEOPS_AUDIT pioneer 미언급 | grep 0건 | A |
| 게이트 강제는 Go/훅 | issueops/SKILL.md 99–141 | B |
| 비-hermetic golden은 Go 결함 | optimization-strategy 100–107 | B |
