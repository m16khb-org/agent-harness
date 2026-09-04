# Agent-Harness 품질향상 프로그램 v2 — 외부 벤치마킹 기반

> 작성일: 2026-06-15 · 방식: 멀티에이전트 조사·종합(내부 baseline 3 + 외부 survey 5, 528K 토큰) → gap 분석(opus) → 3-lens 적대 리뷰(opus) → 메인 통합
> 모델/effort 배정: 내부 추출·웹 breadth = sonnet, 평가방법론·학술논문 deep + gap/draft/critique = opus
> 위치: 기존 `harness-quality-improvement-program.md`(Q1-Q6/S1-S6, 2026-06-11~13)를 **승계·확장**한다(폐기 아님).

이 프로그램의 차별 가치는 **외부 벤치마킹으로만 드러나는 다음 파동**이다. 기존 프로그램의 정량/정성 결함 대부분은 이미 해소됐고(§0.1), 비교 대상은 OSS 하네스(OpenHands·SWE-agent·Aider·Cline·Goose), 상용(Claude Code·Codex·Cursor·Devin·Amp·Factory·Jules·Warp·Augment), 오케스트레이션 프레임워크(LangGraph·AG2·CrewAI·OpenAI Agents SDK·DSPy·LlamaIndex·Pydantic AI), 평가방법론(SWE-bench·tau-bench·TerminalBench·Aider polyglot), 학술(Self-Consistency·Reflexion·Self-Refine·PRM·CRITIC·self-correction 한계)이다.

---

## 0. 현재 상태 스냅샷 (라이브 검증, evidence-bound)

### 0.1 이미 해소됨 — 재제안 금지
- **PROJECT_AUDIT(2026-06-14)의 P1 8종 + P0 전부 FIXED**(2026-06-15 코드 재검증): lifecycle init race(`createJSONAtomic` os.Link EEXIST, `lifecycle_project_state_store.go:172-204`), state write locking(`withStateLock` flock LOCK_EX, `state_io.go:33`), worker stuck-job/concurrent-guard(`DetectStuckWorkerJobs`+`withWorkerJobLock`), draft-wiki stale lock(PID+5m), external-LLM port 추상화(`port.ExternalLLM`+compile-time check), command-policy config(`.issueops/policy.json` additive), session↔IssueOps 바인딩(`LinkIssueOpsWorktree`+테스트).
- **`H1` hook-failure-log 무한성장: CLOSED** — `PruneHookFailureLog`+`PruneHookMetricsLog`가 `RunSessionStart`서 자동 발화(720h, `hookcatalog/catalog.go:76`).
- Q6 lifecycle 동시성 레이스: `go test -race ./internal/core/lifecycle -count=3` clean.
- 측정 인프라: 스킬 16/16 스코어카드, 18-dim 결정적 벤치마크(0/100), self-verify 95-게이트(결정적·seed 고정·13 goal), hook latency telemetry(p50/p95/max+block), 3계층 A/B/C 삼각화, evidence A-D(D 최종 금지), 비-pioneer n≥3, S2 fresh-context 적대 리뷰, 12 sub-agent 패턴.
  - ※ 단, v1 Q2 deliverable 중 **gate hit rate·queue depth는 미배달**(latency·block count만 실제 emit) — A2가 종결한다. v1 Q2를 완료로 세지 않는다.

### 0.2 이미 field를 앞선 점 — 노력 낭비 금지 (적대 리뷰 확인)
- **3계층 측정 삼각화**(격리 rubric → 18-dim 결정적 → 런타임 telemetry, 비대체)는 단일 숫자 리더보드를 내는 Aider/OpenHands보다 엄격.
- **Scorer 결정성 이미 해결**: `--judge none` 완전 결정적, SUT-vs-scorer 분리 실재, live-skill-CI는 명시 non-goal → 평가방법론이 *우려하는 지점을 이미 앞섬*.
- evidence A-D ladder(D 최종 금지), anti-gaming holdout(fresh-context·overfit cap 3.6·fix가 holdout 인용 금지)는 오염방어 가이드를 직접 구현(prompt-engineering가 holdout 최초 실패로 가치 입증).
- **hooks-as-deterministic-enforcement**가 이미 코어 아키텍처 — 상용 #1 인사이트(hook 100% vs prompt 70-90%)를 "발견"할 필요 없이 그 위에 서 있음.
- 동시성 primitive(persistent-inode flock·os.Link EEXIST·PID+walltime stale-lock)는 textbook-correct, IssueOps lock incident 미재발.
- pioneer A/B 회귀 게이트(`CompareIssueOpsBenchmarkRuns`+`EvaluateIssueOpsAutoresearchGate`, `issueops_pioneer_ab_test.go`)의 keep/discard 배관이 이미 존재 — 다만 **proxy 신호로 게이트**(→ G2).

### 0.3 잔존/신규 결함 (라이브 확인)
- **PARTIAL**: `W1`(DetectStuckWorkerJobs 자동 호출처 없음, 수동 `worker cleanup-stuck`만), `M2`(MCP 그룹 내 dispatch 수동 switch ~20 case).
- **OPEN**: `TestResponseContractsGolden` FAIL(golden 84줄 drift; 새 research doc + hidden skill dirs; `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -args -update`).
- **CI 자동화 전무**: `.github`에 ISSUE_TEMPLATE만, `workflows/` 없음 — 전 검증 로컬·수동.
- **측정 사각지대**: ① 오케스트레이션/통합 기여 미측정(keyword proxy), ② hook 강제 게이트 효과 깜깜이(gate-hit-rate 없음), ③ check-adequacy 미검증(나쁜 산출물이 게이트를 통과하는지 red-team 안 됨), ④ pioneer v2 단위 미재측정·holdout n=1·--judge file 자기참조.

---

## 1. 전제 (외부 벤치마킹이 바꾼 관점)

1. **신뢰도(reliability) > 역량(capability).** tau-bench: 90% pass@1 → pass^8 ~57%. 하네스 품질은 "한 번 됨"이 아니라 k회 전부 성공(pass^k)으로 말한다.
2. **점수는 점추정이 아니라 분포다.** "Sober Look"(arXiv:2504.07086): pass@1이 seed별 5-15p 변동. ≥3-5 seed, mean±std, 소표본 Clopper-Pearson/Bayesian 구간("Don't Pass@k", arXiv:2510.04265).
3. **결정성은 SCORER의 속성이다(SUT 아님).** 시스템(확률적)과 채점기(결정적) 분리 → 레포의 "라이브=비결정이라 CI 불가" 고민 해소. 같은 하네스+과제셋+config면 비교 가능; 다른 config 점수 비교 금지.
4. **강제는 prompt가 아니라 hook이다.** 상용 전수 수렴(hook 100% vs prompt/config 70-90%) — 단 게이트가 *실제 발화하는지* 관측해야 한다.
5. **자기교정은 외부 신호로만 작동한다.** 내재적 self-correction은 추론을 *악화*(Huang DeepMind; Kamoi TACL); 유명 결과 다수가 oracle 라벨 사용. self-verify·리뷰는 객관 외부 신호에 앵커(이미 그러함).
6. **Self-Consistency(샘플-투표) > debate (동일비용).** 채점가능 출력엔 최고 ROI; 적대 리뷰는 vote 불가한 정성 결함(보안·계약위반)에만(debate는 3-5배 토큰에 1.5-5.3% 이득).
7. **관측엔 분모가 필요하다.** 현 telemetry는 분자(block·failure·latency)만 있고 분모(invocations·gate evals·queue capacity)가 없어 *비율*을 못 낸다 — 공유 invocations 카운터 하나가 G4/G5/G6를 동시 해결.

---

## 2. 외부-내부 비교 매트릭스 (요약)

| 차원 | issueops 현재 | 외부 best practice(출처) | 판정 | → 워크스트림 |
|------|--------------------|--------------------------|------|--------------|
| 신뢰도 단위 | pass/fail 단일런 | pass^k(tau-bench) | ❌ | A3 |
| 점수 분산 | pioneer n=1 다수 | ≥3-5 seed, credible interval | ◐ | A3/A6 |
| 강제 관측성 | block count, rate 없음 | gate-hit-rate(상용) | ❌ | A2 |
| 통합 기여 측정 | keyword proxy(`_score.go:98`) | end-state/routing 검증 | ❌ | **A5(PRIMARY)** |
| check-adequacy | red-team 안 됨 | nbad가 fail 확인(UTBoost) | ❌ | A4 |
| CI 자동화 | 워크플로 0개 | agent=human 동일 게이트(Amp) | ❌ | A1 |
| linter-as-gate | `lint_diagnose` 보유, 미연결 | 편집전 lint 차단(SWE-agent/Aider) | ◐ | B3 |
| retry 전략 | fail-fast | retry-with-feedback(CrewAI/DSPy), budget realloc(SWE-agent) | ❌ | B5 |
| 채점 축 분리 | 통합 점수 | outcome/recovery/protocol(tau-bench) | ◐ | A4 |
| self-consistency | 없음 | 샘플-투표(학술 TIER1) | ❌ | B6 |
| 자기교정 가드 | 외부신호 앵커 | CRITIC, intrinsic 금지 | ✅ | B1(명문화) |
| typed contract | `contract_check` 보유 | 8프레임워크 수렴 | ✅ | 유지 |
| plan-before-exec | 스킬有, 게이트 약 | 전상용 plan 승인(Jules/Factory) | ◐ | B4 |
| incident→hook | CAUTIONS 기록 | Hashimoto: 실수→영구 hook | ◐ | B2 |
| 실제과제 회귀 | artifact 품질 | e2e 골든셋(OpenHands/Aider) | ◐ | A4 |

위는 load-bearing 16행 요약이다. 나머지 차원(평가 결정성·오염/테스트적합성·컨텍스트 관리·best-of-N 랭킹·사전 readiness·judge 독립성·termination/circuit·partial-result)은 §0.2(이미 앞선 점)·각 워크스트림·§7(비목표)에 흡수됐다.

---

## 3. 정량 트랙 (측정 가능) — A1~A7

> 원칙: 모두 **기존 표면 확장**(self-verify 게이트, 18-dim 벤치마크, hook-metrics JSONL, A/B 게이트). 새 프레임워크·멀티에이전트·live-skill-CI 없음.

### A1. CI 자동화 — 결정적 게이트를 GitHub Actions로 [G9, high/小, 최우선]
- **무엇**: 이미 결정적인 self-verify 게이트(`--judge none`, `--seed` 고정)와 `go test ./...`를 PR CI 워크플로로 미러. LLM eval 기본 off이므로 *기존 명령의 배선*일 뿐 — live-skill·모델 불필요.
- **왜**: 현재 main에 `TestResponseContractsGolden`이 golden drift로 실패 중 — CI가 잡았어야 할 회귀 클래스. 전 검증이 로컬·수동.
- **수용(binary)**: `.github/workflows/`에 `go test ./... -count=1` + `self-verify --seed=100 --target-score=95 --json`이 PR에서 그린/레드를 게이트; golden drift 1건 `-update`로 선수정 후 그린.
- **실행 단위**: 직접(docs+yaml). **외부 ref**: Amp "same checks for agent as human"; Claude Code CI hook mirroring.

### A2. 관측 분모 도입 — 공유 invocations 카운터 → gate-hit-rate + failure-rate + queue-depth [G4+G5+G6, med/小, 묶음]
- **무엇**: hook-metrics JSONL에 hook별 **invocations 카운터** 1개 추가 → 기존 `Blocks` 분자가 `gate_hit_rate = blocks/invocations`로, hook-failure 로그와 조인해 `failure_rate = failures/invocations`로 환산(read-time). worker queue depth(running/queued) telemetry 추가.
- **왜**: 사각지대 ②(게이트 비활성화 회귀가 latency·failure 변화 0으로 탐지 불가, 보안 함의). 분자만 있고 분모가 없어 비율 산출 불가. **회계 정직성(적대리뷰 보정)**: v1 Q2가 "enforcement gate hit rate", "draft-wiki 큐 깊이"를 deliverable로 적었으나 라이브 코드상 *실제 emit된 적 없음* — A2는 신규 발명이 아니라 **v1 Q2의 미배달분(gate hit rate, queue depth)을 종결**한다(이중계상 방지: §0.1의 Q2를 "done"으로 세지 않음).
- **수용(binary)**: `issueops hook metrics --json`이 hook별 `gate_hit_rate`·`failure_rate`·`queue_depth` 반환 + Go 테스트; 게이트를 인위 비활성화한 negative 테스트서 hit_rate가 ~0으로 떨어짐을 assert.
- **실행 단위**: **issueops 사이클**(Go 변경). **외부 ref**: Augment "error rate per 1K tasks"; orchestration "circuit breaker".
- **동반(W1 종결)**: `DetectStuckWorkerJobs`를 `RunSessionStart` 자동 트리거에 연결(prune와 동일 패턴, `catalog.go:76`) → stuck job 자가치유.

### A3. 신뢰도·분산 보고 — pass^k + credible interval [G1, high/中]
- **무엇(적대리뷰 보정 — SUT/scorer 분산 분리)**: pass^k·mean±std·Clopper-Pearson 구간은 **확률적 SUT**(실제 스킬/에이전트 실행을 오프라인 기록한 k회, 또는 fresh-context holdout 채점 k회)에만 적용한다 — 결정적 게이트(`passed×100/total`)를 같은 seed로 반복하면 동일 결과라 pass^k=pass@1로 trivial(분산 구조적 0). **결정적 게이트**에는 분산>0을 요구하지 않고, *고정 seed서 interval이 점으로 붕괴함(width==0)을 assert하는 determinism check*를 둔다(premise #3 "결정성=scorer 속성"과 정합, nonGoal 4 비위반). pioneer holdout(확률적 fresh-context 채점)의 n≥3 분산 규율을 **전 pioneer로 승격**해 n=1 holdout과 n=27 v1 baseline 혼용 제거.
- **왜**: 모든 headline(4.92 holdout, 100/100, 95-gate)이 소수 런 점추정. seed는 이미 고정(`--seed=100`). **주의(적대리뷰 보정)**: candidate 11 `self-verify-flake-classifier`는 이미 status=Satisfied이고 scope가 failure-clustering이라 재사용 불가 → **NEW 후보 17 `self-verify-score-distribution`(category reliability, status open)을 신설**하고, interval(Clopper-Pearson) 수학은 신규 코드임을 명시(effort에 반영).
- **수용(binary)**: (a) 확률적 SUT(실 런 기록 또는 holdout) k≥3~8회의 `pass_at_1`·`pass_pow_k`·`interval`(Clopper-Pearson) 산출; (b) 결정적 게이트는 고정 seed서 `interval_width==0`(점 붕괴) assert; (c) 전 pioneer holdout이 n≥3 mean±range로 재기록.
- **실행 단위**: 측정 세션 + 소 Go 변경(interval 수학은 신규 코드). **외부 ref**: tau-bench pass^k; "Sober Look"; "Don't Pass@k".

### A4. Check-adequacy + 채점 축 분리 — 음성 fixture red-team [G3, high/中]
- **무엇**: 18-dim 벤치마크에 **고의로 틀린 산출물 음성 fixture**(TDD 증거 누락, 조작된 tool 출력, 잘못된 스킬 발화, 빈 plan)를 추가해 *반드시 100 미만*을 검증(binary 벤치마크판 mutation testing). 동시에 18-dim을 **outcome(end-state)·recovery(self-repair)·protocol-compliance** 3축으로 명시 분류.
- **왜**: 게이트가 *실패할 수 있는지* red-team된 적 없음(keyword-rich 오답이 100 가능). SWE-bench 솔루션 누출 ~33%; OpenAI SWE-bench Verified 감사서 *138개 o3-inconsistent 문제 중 59.4%*가 모델 한계가 아닌 테스트 결함(전체 실패의 59%가 아님 — 분모 주의); 레포 자체도 "borderline anchor 부재"(`harness-skill-quality-scorecard.md:29`) 자인.
- **수용(binary)**: 음성 fixture N개가 타겟 dim서 <100; **invalid-sample-rate를 전 fixture corpus 분모로 명시**해 1개 산출; 18-dim에 3축 라벨.
- **실행 단위**: **issueops 사이클**. **외부 ref**: UTBoost; tau-bench/TerminalBench/Aider end-state 채점.

### A5. 통합 기여 측정 — 결정적 오프라인 routing trace 리플레이 [G2, high/大, PRIMARY]
- **무엇**: live-skill-CI non-goal를 어기지 않고, **실제(비-CI) issueops 런 중 "어느 스킬이 어느 phase서 발화했는가"를 hook-metrics JSONL에 구조화 routing-trace 이벤트로 캡처** → fixture로 저장 → 벤치마크 dim이 기록된 trace를 expected routing과 대조 채점. keyword proxy를 **리플레이 기반 routing 증명**으로 전환.
- **왜**: 자기진단 #1 구멍. `pioneer_skill_contribution`은 literal keyword proxy(`issueops_benchmark_score.go:98`); A/B 게이트(`issueops_pioneer_ab_test.go`)는 *proxy로 게이트* — 스킬이 조용히 발화를 멈춰도 키워드만 맞으면 미탐지. 인프라(hook telemetry + 벤치마크 fixture + A/B 게이트) 이미 존재, CI는 캡처 trace를 리플레이하므로 결정적.
- **수용(binary)**: routing-trace 이벤트 타입 emit + 1개 fixture에 기록 trace vs expected routing 채점 dim; "스킬 발화 중단 but 키워드 유지" 시나리오가 이 dim서 <100.
- **실행 단위**: **issueops 사이클**(중간 규모). **외부 ref**: eval "grade end-state not narration"; PRM "Let's Verify Step by Step".

### A6. v2 baseline 단일척도 재측정 + 내구 fixture [G8, med/中]
- **무엇**: pioneer family를 **v2 단위로 1회 재측정**해 baseline·현재를 한 척도에 정렬(현 3.10 v1 ↔ 4.92 v1 혼용 제거); holdout 케이스의 **내구 fixture 아티팩트를 repo에 커밋**(현재 gitignored 런타임 증거라 fresh checkout 재현 불가; dated/post-cutoff로 오염 방어).
- **수용(binary)**: 9 pioneer 전부 v2 점수 1건(스킬당 seed 수 명시, grade-B 판정); holdout fixture가 커밋돼 fresh checkout서 재현; dashboard에 단일척도 delta 1줄.
- **실행 단위·노력(적대리뷰 보정)**: "no code=cheap" 아님 — (1) 현 holdout fixture는 **gitignored 런타임 증거**라 내구화엔 **gitignore/commit-policy 변경**(별도 sub-task)이 필요하고, (2) v2 재측정은 fresh-context 서브에이전트 채점이 드는 **grade-B 측정 노력**, (3) 커밋 전 **de-leak/오염 점검** 단계 추가. effort 中→中상. **외부 ref**: eval "standardize on one config", "held-out time-windowed sets".

### A7. --judge file 독립 검증 [G7, med/小]
- **무엇**: `--judge none`을 결정적 headline 게이트로 유지하되, doc/spec 품질 등 프로그램적 검사가 어려운 dim에 `--judge file`을 쓸 땐 judge map을 **런 자체 출력이 아닌 별도 기록 평가**서 생성하도록 요구하고, judge-vs-deterministic 일치도 1회를 calibration 아티팩트로 dashboard에 기록.
- **왜**: 유일 실전 judge 런(2026-06-13)이 같은 결정적 출력서 map 생성(자기참조, `quality-dashboard.md:66`) — 독립 검증·일치도 부재. LLM-judge 편향(위치·장황·자기선호) — temp=0.
- **수용(binary, 적대리뷰 보정)**: judge map에 **source-run ID를 저장**하고 Go 테스트가 그것이 *scored run ID와 다름*을 assert(provenance check — "독립"을 honor-system 아닌 기계검증으로); judge-vs-deterministic 일치도 수치 1건 dashboard 기록.
- **실행 단위**: 직접(process+소 검사). **외부 ref**: eval "LLM-as-judge biased, mitigate".

---

## 4. 정성 트랙 (측정이 못 잡음) — B1~B6

### B1. 자기교정 가드레일 명문화 [G10, low/小]
- **무엇**: "모든 self-augment/self-verify 교정 스텝은 **외부 tool 신호**(test/lint/typecheck/api-doc/결정적 벤치마크)로 게이트되고, 모델 자기비판은 주관 축(문서 가독성)에 한해 advisory — correctness 게이트로 절대 사용 금지"를 불변식으로 CONVENTIONS/CAUTIONS에 명문화 + augment 후보가 tool-grounded 검증을 반드시 동반하는지 검사.
- **왜**: 학술 TIER-3 가장 load-bearing 발견(intrinsic self-correction이 추론 악화; oracle 라벨 누출). **위치(적대리뷰 보정)**: 이는 새 원칙이 아니라 **v1 S5(keep/discard "measured gap or no EDIT")와 S6(정직성 단서)를 승계**해, *문서 규약*을 **Go-test 강제 불변식**(augment-candidate catalog 대상)으로 격상하는 enforcement-hardening이다(CLAUDE.md 제약과도 정합).
- **수용(binary)**: 불변식 1절 + augment 후보 schema에 external-signal 필드 강제 + 그것을 검증하는 Go 테스트 1건. **외부 ref**: CRITIC; Huang/Kamoi.

### B2. Incident→hook 매핑 규약 (Hashimoto 원칙) [매트릭스#17, qual/中]
- **무엇**: CAUTIONS.md의 반복 실수 항목을 가능한 한 **결정적 hook/테스트로 전환**하는 규약 — "관찰된 실패 → 영구 강제장치" 매핑 테이블을 운영 문서에 신설. (이미 CAUTIONS가 기록은 함; 강제화가 빠짐.)
- **왜**: 상용 메타-품질 패턴("every mistake → engineer it can never happen again"). 하네스의 hook 아키텍처가 이를 가장 잘 실현.
- **수용(binary)**: incident→hook 매핑 테이블 1개; 기존 CAUTIONS 항목 ≥3건이 hook/테스트로 전환됐는지 표기.

### B3. Linter-as-gate PostToolUse 후크 [매트릭스#8, qual→quant/小]
- **무엇**: 보유 중인 `lint_diagnose`를 **PostToolUse(edit) 후크**에 연결 — 편집 후 lint 실행, 실패 시 에러 텍스트를 다음 LLM 호출에 피드백 주입(차단형은 옵션).
- **왜**: OSS 전수 수렴 — 가장 보편적 품질 primitive(SWE-agent 편집전 차단, Aider 자동 lint+재프롬프트). 인프라 이미 존재.
- **수용(binary)**: edit 후 lint 실패가 후크 출력에 피드백으로 반영되는 테스트 1건. **외부 ref**: SWE-agent ACI; Aider auto-lint.

### B4. Plan-before-execute 게이트 1급화 [매트릭스#16, qual/中]
- **무엇**: implementation-planning/planner 스킬의 plan 산출을 **워크플로 1급 primitive**로 — 코드 변경 사이클은 plan 승인 게이트(human/critic)를 통과해야 진입(현 S2 사전 리뷰 규약을 게이트로 강제화).
- **왜**: 전 상용 보편(Claude Code Plan Mode·Codex Plan.md·Jules 필수 plan 승인·Factory Delegator). issueops는 스킬엔 있으나 강제가 약함.
- **수용(binary)**: issueops plan phase가 승인 없이 implement phase 진입을 차단하는 deterministic check 1건. **외부 ref**: Jules/Factory plan gate.

### B5. Retry-with-feedback + budget reallocation [매트릭스#9, qual/中]
- **무엇**: self-verify의 fail-fast를 보완 — 실패 스텝 정보를 다음 시도에 **피드백 주입**(blind retry 아님), 실패 시도의 미사용 budget을 후속에 재배분(SWE-agent RetryAgent), 동시 회귀 진단 위해 전 스텝 실패 수집 옵션.
- **왜**: self-verify는 첫 실패서 중단 → 동시 다중 회귀 진단 어려움. 프레임워크 전수 수렴(CrewAI guardrail-retry-feedback, Pydantic ModelRetry, DSPy assert backtrack).
- **수용(binary)**: 실패사유 주입 재시도 경로 1개 + 전-스텝-수집 모드 테스트. **외부 ref**: SWE-agent RetryAgent; CrewAI; Pydantic AI.

### B6. Self-Consistency 도입 (채점가능 sub-agent 출력) [매트릭스#12, qual→quant/中]
- **무엇**: 채점/추출 가능한 verdict를 내는 sub-agent(예: 적대 리뷰의 판정, judge)에 **N-샘플 다수결** 도입; vote 불가한 정성 결함(보안·계약위반)에만 debate 유지.
- **왜**: 학술 TIER1 — 동일비용서 debate를 능가하는 최고 ROI. workflow의 critique를 best-of-N ranker로 재구성 가능(G2/A/B 게이트와 결합).
- **수용(binary)**: 1개 votable 리뷰 경로에 N=3+ 다수결 적용 + 단일런 대비 분산 감소 기록. **외부 ref**: Self-Consistency(Wang 2022); academic TIER1.

### 잔여 정리 (소): `TestResponseContractsGolden` golden drift `-update` 선수정(A1 선행); `M2` MCP 그룹 내 dispatch를 catalog-driven으로 완성(잔여 ~20 case switch 제거).

---

## 5. 우선순위·의존성·실행 단위

| 순번 | 항목 | 트랙 | 효과/노력 | 실행 단위 | 의존 |
|------|------|------|-----------|-----------|------|
| 0 | golden drift 수정 + **A1 CI 자동화** | 정량 | 高/小 | 직접 | — (즉시) |
| 1 | **A2 관측 분모**(G4+5+6) + W1 자동트리거 | 정량 | 高/小 | issueops | — |
| 2 | **A5 routing trace 리플레이**(PRIMARY) | 정량 | 高/大 | issueops | A2(telemetry) |
| 3 | A4 check-adequacy 음성 fixture + 3축 | 정량 | 高/中 | issueops | — |
| 4 | A3 pass^k + interval | 정량 | 中/中 | 측정+소Go | — |
| 5 | B1 자기교정 가드 명문화 | 정성 | 高/小 | docs | — |
| 6 | B3 linter-as-gate 후크 | 정성 | 中/小 | issueops | — |
| 7 | A6 v2 baseline 재측정 + 내구 fixture | 정량 | 中/中 | 측정 | A3 |
| 8 | A7 judge 독립검증 / B2 incident→hook / B4 plan 게이트 / B5 retry / B6 self-consistency | 양쪽 | 中/中 | 혼합 | 틈새 |

병렬 가능: {0,1,3,4,5}는 상호 독립. A5는 A2의 telemetry 표면에 의존. {6,7,8}은 후행/틈새.

---

## 6. 성공 판정 (프로그램 수준)
- **정량**: gate-hit-rate·failure-rate·queue-depth 비율 산출(분모 도입); pass^k(k≥8) + credible interval가 전 headline에; 음성 fixture red-team로 false-pass rate 공개; routing trace 리플레이 dim 1개; CI가 PR서 결정적 게이트 강제.
- **정성**: 자기교정 가드 불변식 명문화; incident→hook 매핑 ≥3건; linter-gate·plan-gate·retry-feedback 각 1경로; self-consistency 1경로.
- **공통**: 각 항목 evidence-bound(A-C)로 종결 — D 금지; 모든 변경은 기존 표면 확장(재발명 0); live-skill-CI·멀티에이전트 확장 0.

## 7. 비-목표 (이번에도 하지 않음)
- 라이브 스킬 호출 CI 측정(결정성) — keyword proxy + **리플레이 trace** + 도그푸드 + judge로 삼각측량.
- 이미 FIXED P1/P0 재구현. 멀티에이전트 실행 루프(SWE-bench 단일에이전트 SOTA). 전문기능 재발명(code graph→CodeGraph, LLM Wiki→nvk/llm-wiki, memory→claude-mem). 벤치마크 자동 대시보드화(수동 규약 우선).

## 부록 A — 외부 출처 인덱스
- 평가: tau-bench(arXiv:2406.12045) · "A Sober Look"(arXiv:2504.07086) · "Don't Pass@k"(arXiv:2510.04265) · SWE-bench Verified · Aider polyglot · TerminalBench · UTBoost.
- 학술: Self-Consistency(Wang 2022) · Reflexion · Self-Refine · "Let's Verify Step by Step"(PRM800K) · CRITIC · Huang et al.(DeepMind, self-correction 한계) · Kamoi et al.(TACL survey).
- 하네스: OpenHands(arXiv:2407.16741) · SWE-agent(arXiv:2405.15793) · Aider · Goose · Factory Agent Readiness · Anthropic 멀티에이전트 엔지니어링 블로그(filesystem-as-handoff).
- 프레임워크: LangGraph · AG2 · CrewAI · OpenAI Agents SDK · DSPy(arXiv:2312.13382, 164%/37%) · Pydantic AI · LlamaIndex.

## 부록 B — 도출 근거 및 적대 리뷰 기록 (provenance)
- **gap 매핑**: §3-4의 A1-A7/B1-B6은 opus gap 분석 10갭(G1-G10) + 비교 매트릭스(§2, 22차원 중 16행 요약) 보조항목에서 도출. 각 갭의 file:line 근거는 §0·§2·각 워크스트림 본문에 인용.
  - A1←G9, A2←G4+G5+G6, A3←G1, A4←G3, A5←G2(PRIMARY), A6←G8, A7←G7, B1←G10, B3/B4/B5/B6←매트릭스#8/#16/#9/#12, B2←매트릭스#17.
- **적대 리뷰(레포 S2 규약, fresh-context 3-lens opus)**: feasibility 8/10, philosophy 8/10, evidence-rigor 8/10 — 구조적 반려 0, blocker 9건 전부 본 문서에 반영(코드검증). 주요 반영:
  - (evidence-rigor, 최중대) 결정적 게이트엔 pass^k/분산>0 부적용 → 확률적 SUT에만 pass^k, 게이트는 determinism check로 분리(A3).
  - (feasibility/philosophy) candidate 11은 Satisfied → NEW 후보 17 신설(A3); v1 Q2 미배달분 종결로 재라벨(A2, §0.1); held-out fixture gitignored → 내구화에 gitignore/commit-policy 변경 명시(A6); judge 독립성 provenance check로 binary화(A7); B1은 v1 S5+S6 승계·Go-test 강제화.
  - (evidence-rigor) o3 테스트결함 수치 분모 정정(138 audited subset의 59.4%, A4).
- **모델/effort 배정 기록(사용자 요청)**: 내부 baseline 추출·외부 하네스 breadth = sonnet(증거수집·폭, 빠름); 평가방법론·학술논문 deep 추출 + gap/draft/critique 종합 = opus(엄밀·깊이). 총 13 서브에이전트, ~930K 토큰.
