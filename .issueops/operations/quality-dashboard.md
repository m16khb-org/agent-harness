# Agent-Harness 품질 대시보드 (단일 가시성 인덱스)

품질향상 프로그램 **Q5** (`.issueops/issues/_unnumbered/harness-quality-improvement-program.md`).
산재한 정량 지표를 한 페이지로 인덱스한다 — 각 지표군의 **최신값 · 측정일 · 다음 측정 예정 · 출처 문서**.
자동화는 후순위, 우선 수동 갱신 규약(아래 §갱신 규약).

최종 갱신: **2026-08-14**

> **계층 분리 단서 (S6 — 고정 표기, 합산 금지)**
> 아래 지표는 서로 다른 **3개 계층**을 측정하며 한 점수로 합산하지 않는다:
> ① **격리 rubric** (스킬 SKILL.md를 fresh-context에 단독 주입한 품질) — 측정면 1·2,
> ② **통합 벤치마크** (issueops 산출물의 19-dimension 결정적 채점 + pioneer 시그니처 + skill routing fidelity) — 측정면 3,
> ③ **런타임 텔레메트리** (hook latency/failure/gate) — 측정면 4·5.
> 격리 점수 4.9가 통합 기여 100을 "증명"하지 않으며 그 반대도 아니다. 세 계층은 삼각측량이다.

---

## 측정면 요약 (한눈)

| # | 측정면 | 계층 | 최신값 | 측정일 | 게이트 | 다음 측정 |
|---|--------|------|--------|--------|--------|-----------|
| 1 | Pioneer 스킬 12종 | fresh-context holdout + historical rubric | **11 PASS / 1 capability-BLOCKED / 0 FAIL**; historical 9종 v2 post ≈4.78 | 2026-08-14 | ✅ no defect; Boehm visual verification blocked honestly | 분기 도그푸드(S1) 또는 SKILL.md 변경 시 |
| 2 | 비-pioneer 스킬 7종 | 격리 rubric | **7/7 측정 완료**, 전 skill ≥4.86, holdout ≥4.8 | 2026-06-13 | ✅ GREEN | 분기 도그푸드(S1) 또는 SKILL.md 변경 시 |
| 3 | IssueOps 산출물 벤치마크 | 통합 | **100/100** (avg·min), critical 0, 18 fixtures, canonical namesake 12/12 | 2026-08-14 | ✅ GREEN | pre-push 상시 |
| 4 | Hook 런타임 메트릭 | 런타임 | 전 hook p95 **<40ms**, blocks 0 | 2026-06-13 | ✅ stop p95 38ms (<1s) | 상시 누적(자동) |
| 5 | Hook 실패율 | 런타임 | total 38, last_7d **5**, last_24h 0 | 2026-06-13 | ✅ | 상시 누적(자동) |
| 6 | 운영 안정성 baseline | 런타임 | final audit green: zombie 0, MCP 끊김 0, self-verify 230/230 | 2026-06-13 | ✅ GREEN | STA 재측정/안정성 회귀 의심 시 |

커버리지: 스킬 visible 측정 **16/16 완료** (STA-P D-추정 해소), hook 지표 **0→4** 달성.
strict 잔여였던 ACP-O 재측정, STA-H holdout n≥3, IssueOps judge file 실전 1회를 2026-06-13 모두 닫았다.

---

## 측정면 1 — Pioneer 스킬 9종 (historical cohort, 격리 rubric)

`historical 2026-06-16 isolated-rubric cohort: 9 skills`는 당시 재채점 대상 수를 뜻한다. `current IssueOps benchmark fixture loader: 13 pioneer-targeted fixtures`는 canonical 12 namesake와 별도 operational `issueops` target의 합계다. Deterministic benchmark는 총 18 fixture를 읽으며 reproduction fixture는 historical rubric 점수를 재측정한 수치가 아니다.

Gate policy는 evidence 상태와 함께 fail-closed로 동작한다.

- collector, requested baseline write, walk/open/scan failure는 해당 signal을
  `error`로 만들고 `collection=error`, `health=unknown`, `gate=block`으로
  종료한다.
- `.issueops/PROJECT_AUDIT.md`의 active P0는 blocking finding이다.
- `--trend`에서 code-SNR이 저장 baseline보다 0.01 초과 하락하면
  `code-snr-regression` blocking finding을 만든다. 승인된 새 baseline은
  명시적 `--save-baseline`으로만 갱신한다.

- **v2 단일척도 holdout 평균: 4.78/5.0** (2026-06-16 A6 재채점, range 4.7–4.9, 전 9종 ≥4.7, n=1/skill).
  - **단일척도 정렬**: 직전 **4.92**는 *같은 run*을 v1 앵커(5.0=완전충족)로 채점한 값. v2 5.0-유보 규칙("흠잡을 데 없음"=4.8)에서 그 5.0들이 4.8로 재보정 → **4.92 → 4.78은 척도 재보정이지 품질 회귀가 아니다.**
  - **오프라인 artifact-bound 재채점**(2026-06-11 기록 run 재채점, 신규 dispatch·신규 측정 아님). live 다중-seed 재측정은 **user opt-in 후속**.
  - **진짜 단일척도 delta (2026-06-16)**: 27-case visible baseline도 v2로 재채점 → **v2 baseline ≈3.07** (v1 3.10에서 −0.03; 저점은 gate-capped 결함·sub-5 차원 지배라 5.0-유보 영향 거의 0). ⟹ **≈3.07(v2 visible, pre-opt) → 4.78(v2 holdout, post-opt) = +1.71** (단일척도). naive v1-혼합 delta 3.10→4.92=+1.82는 high-end가 더 압축돼 **+0.11 과대평가**였다. **잔여**: 두 끝점은 여전히 다른 *케이스셋*(27 visible vs 9 holdout) — same-set pre/post는 27 visible의 post-opt live 재실행 필요(user opt-in).
  - proportionality는 narrative가 관측한 곳만 채점, 미관측은 *inferred* 표기(날조 금지).
- **내구 재현 하네스**: `testdata/pioneer-holdouts/`에 9종 holdout *입력만* 커밋(파일시스템 6 + in-prompt 2 + live-web 1 honest 표기). 답(`result.yaml`: 점수·root cause·fix)은 gitignored evidence 트리에 잔류; `internal/holdoutdeleak` Go 테스트가 누출 토큰 부재 + evidence 미추적을 기계검증. **정직 표기**: 커밋된 fixture는 *reproduction harness*이지 더 이상 blind holdout이 아니며, 원본 run이 아닌 *케이스*를 재현한다(원본 /tmp dir 소실).
- 강점 스킬: algorithm-optimization·verified-execution v2 4.9(no-change redirect / stale-tool 거부 = 요구 초과 가치). 가드레일 수정 대상이었던 algorithm-optimization(hollow-method), prompt-engineering(overfit)는 수정 후 holdout 통과.
- 출처: `pioneer-v2-regrade-2026-06-16.md`(재채점·잔여갭), `pioneer-skill-quality-scorecard.md`, 증거 `evidence/pioneer-skills-quality/reruns/post-optimization-measurement-2026-06-11.md`.
- 단서: 격리·단일런(holdout n=1, 오프라인 재채점). 통합 기여는 측정면 3이 별도 담당. **오염 caveat**: 일반 엔지니어링 과제라 "post-cutoff"는 verbatim 암기만 방어 — 점수는 케이스 기본역량이지 held-out 일반화가 아니다.

## 측정면 2 — 비-pioneer 스킬 7종 (격리 rubric)

2026-06-13 측정 (가중 평균 / holdout n≥3 평균±범위):

| 스킬 | 가중 점수 | Holdout | gate flag | evidence |
|------|-----------|---------|-----------|----------|
| issueops | 5.0 | 5.0 ±0 (n=3) | none | A·A/B + H:C |
| atomic-commit-push | 4.97 | 4.9 ±0 (n=3) | none | A + H:C |
| self-verify | 5.0 | 4.9 ±0 (n=3) | none | A + H:C |
| self-augment | 5.0 | 4.9 ±0 (n=3) | none | A + H:C |
| project-bootstrap | 4.86 | 5.0 ±0 (n=3) | none | A + H:A |
| draft-wiki-promoter | 4.94 | 4.8 ±0 (n=3) | none | A/C/A + H:C |
| stability-audit | 4.86 | 5.0 ±0 (n=3) | none | A + H:A |

- **STA-P 종료**: 2026-06-13 fast-path audit fresh run으로 A-grade evidence 확보(`ok=true`, `failures=[]`, MCP ids 1-8, zombie/legacy/temp 0, self-verify 10/10·230/230·min score 100). 상세: `.issueops/evidence/harness-skills-quality/sta-p-2026-06-13.md`.
- **strict 잔여 종료**: STA-H run 2·3 재실행 green(`/tmp/issueops-sta-h-run2-fixed-20260613.json`, `/tmp/issueops-sta-h-run3-20260613.json`)으로 n=3 달성. ACP-O는 현재 script surface 2개(`git_preflight.py`, `api_doc_gate.py`)와 `api_doc_gate_test.py`를 재측정해 stale-contract cap 제거.
- **v2 세분화**(0.1 단위 + 5.0 유보 규칙 + proportionality)는 신규 측정부터 의무. 천장 효과(21케이스 중 17건 5.0) 실측이 동기.
- 출처: `harness-skill-quality-scorecard.md`.

### 2026-08-14 fresh-context 12종 실행

- Canonical 12종을 각각 별도 child context에 skill 하나만 주입하고
  `testdata/pioneer-holdouts/<skill>/TASK.md`를 실제 사용자 작업처럼 수행했다.
  Benchmark scorer/expected output은 읽지 않았고 deterministic fixture 점수와
  합산하지 않았다.
- 결과: **11 PASS / 1 BLOCKED / 0 FAIL**.
- 재현 가능한 provenance는
  `testdata/pioneer-holdouts/evaluation-manifest.json`에 fixture SHA-256,
  isolated child ID, host/model route, verdict, capability-block reason,
  bounded task-output receipt SHA-256/byte count를 기록한다. 각 namesake의
  `evidence-records/<skill>.json`은 case별 deterministic assertion ID,
  semantic grade, host capability를 포함하고 manifest가 그 record hash를
  검증한다. answer/분석 본문은 기록하지 않으며 모든 case는
  `hidden_holdout=false`다.
- 각 namesake는 primary/boundary/operational 세 축으로 실행되며 manifest는
  총 36개 case를 요구한다. 12종 headline은 skill-level summary이고 36개
  row가 실제 evidence denominator다. 실행 context 수는 **24개**다:
  primary 12개는 각각 독립 실행했고, boundary/operational은 namesake별
  독립 child 하나가 두 축을 함께 실행했다. 36개 row를 36개 독립 child
  실행으로 과장하지 않는다.
- `quality inspect`는 이 manifest를 fixture hash와 대조하고
  `isolated_expected/observed/passed/blocked/failed`,
  `isolated_execution_count`, bounded blocked case/reason 및
  `hidden_holdout_observed`를 별도 projection한다. 따라서 `BLOCKED`는
  pass로 합산되지 않고, committed reproduction 입력은 hidden evidence로
  주장되지 않는다.
  `requirements-analysis`은 텍스트 요구사항·모순·구현 보류 분석을 완료했지만 Kordoc MCP와 원본
  screenshot이 없어 visual/OCR 재검증을 `BLOCKED`로 유지했다. 이를 pass로
  승격하지 않았다.
- 나머지 11종은 각 task의 실제 surface를 충족했다: live multi-source research,
  adversarial plan review, database/index trade-off, profiled algorithm decision,
  meeting-memory handoff, deterministic debugging, adversarial prompt handling,
  staged/unstaged/untracked SNR, recoverable destructive Git operation,
  evidence-bound one-line execution, proportionate planning.
- 두 child가 evaluator isolation 지시와 달리 holdout/report 파일을 만들었으나
  parent가 즉시 해당 child-created residue만 제거했고, canonical holdout input과
  research directory에 평가 잔여 파일이 없음을 확인했다. 이는 skill deliverable
  판정과 별도로 기록한 evaluator-harness hygiene finding이다.

## 측정면 3 — IssueOps 산출물 벤치마크 (통합)

- **19-dimension** 결정적 채점 + pioneer 시그니처 + skill_routing_fidelity(recorded-trace proxy) + N/A 제외 + A/B 게이트.
- 최신 (2026-08-14, `--judge none`, fixtures 18건): **average 100 / minimum 100 / critical_failure 0**, canonical namesake coverage **12/12** → 게이트 GREEN.
- 이 결과는 deterministic artifact-structure regression gate다. Committed
  reproduction fixture는 hidden holdout이 아니며, fresh-context 실제 스킬 실행
  evidence와 합산하지 않는다.
- `--judge file` 백엔드 practical run 완료: `/tmp/issueops-issueops-judge-map-20260613.json`을 strict decode/merge해 `/tmp/issueops-issueops-benchmark-judge-file-20260613.json` 생성, **average 100 / minimum 100 / critical_failure 0 / judge_failures 0**. 단, 현재 도구 정책상 fresh-context sub-agent dispatch는 사용자 명시 요청 없이는 사용하지 못해 judge map은 deterministic run output에서 생성했다.
- **A7 judge provenance (자기참조 가드)**: `--judge file`은 이제 wrapped 포맷(`source_run_id`+`provenance`+`scores`)만 받고(legacy flat map은 decode 거부), `ValidateJudgeProvenance`가 *source_run_id가 scored run과 다르며 실제 영속 run으로 resolve됨*을 merge 전 fail-closed로 강제한다. **정직한 범위(적대리뷰 보정)**: 이는 *자기참조 가드*이지 judge 독립성 증명이 아니다 — 저자가 다른 run id를 명명할 뿐, 실제 독립 judge가 다른 artifact를 평가했음을 증명하지는 않는다. calibration 지표는 `JudgeDownwardOverrideRate`(comparable dim = non-N/A·judge가 채점한 차원, 그중 judge가 낮춘 비율; "agreement"가 아니라 *하향 발산률*). 결정적 run은 100/100이고 merge는 lowering만 허용하므로 깨끗한 run에선 구성상 **0**이며, 위 2026-06-13 self-referential map은 이 가드로 *거부*된다 → 비퇴화 calibration 수치는 genuine 독립 judge run을 대기한다.
- **B6 self-consistency (Wang 2022, 판정 분산 집계)**: `ConsensusJudgeVerdict`가 동일 artifact에 대한 **N개 OFFLINE-기록 judge 샘플**을 majority-vote(binarized pass/fail; tie→fail-closed) + median(평균 아님 — 결정적 채점기는 0/100 bimodal이라 mean은 어떤 샘플도 내지 않은 값에 떨어짐) + 경험적 분산으로 집계한다. **정직한 범위(적대리뷰 보정)**: ① self-consistency는 같은 모델 샘플 간 *판정 분산*만 줄이고 judge의 체계적 *편향*은 줄이지도 탐지하지도 못한다(편향된 judge의 합의는 여전히 편향됨); ② headline 레짐(깨끗한 100/100 + judge-lowers-only)에서는 판정이 bimodally pinned → 분산이 **퇴화(0)**이라 줄일 것이 없다 — 분산 수치는 *판정이 실제로 갈리는* 곳에서만 의미. ③ 독립성 가드는 A3/A7에서 이식: 샘플 id distinct + provenance non-empty를 강제해 "한 judge를 N명 유권자로 분장"하는 가짜 자유도를 fail-closed로 거부. **live N-judge는 범위 밖**(externalllm import 없음 — CI에서 judge를 N회 실행하면 결정적-eval 비목표 위반); CLI는 `issueops benchmark consensus --samples <offline.json>`로 *기록된* 샘플만 집계한다. ⟹ A7의 calibration 수치와 마찬가지로 *비퇴화* variance-reduction 실측치는 genuine 독립 judge 샘플을 대기한다.
- 실행: `issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json`; 합의 집계 `issueops benchmark consensus --samples <offline.json> --json`.
- 출처: `testdata/issueops/fixtures/`, `internal/core/issueops/benchmark/` (`issueops_self_consistency.go`).

## 측정면 4 — Hook 런타임 메트릭 (Q2 신규 측정면)

> 2026-08-27: hook metrics/failures 원장은 legacy hook 표면과 함께 제거됐다. 아래 표는 당시 측정값의 기록이다. 남은 `session-start` context hook은 p50 17.5ms(프로세스 스폰 하한 13ms) 수준이다.

2026-06-13 (`issueops hook metrics`, 총 303 events):

| hook | count | p50 | p95 | max | blocks |
|------|-------|-----|-----|-----|--------|
| pre-tool-use | 167 | 5ms | 11ms | 22ms | 0 |
| post-tool-use | 100 | 0ms | 1ms | 4ms | 0 |
| stop | 11 | 14ms | 38ms | 38ms | 0 |
| user-prompt | 18 | 12ms | 17ms | 17ms | 0 |
| pre-compact | 3 | 12ms | 30ms | 30ms | 0 |
| session-start | 2 | 9ms | 12ms | 12ms | 0 |
| post-compact | 2 | 1ms | 2ms | 2ms | 0 |

- **수용 기준 충족**: Stop hook p95 38ms ≪ 1s 목표(LLM 게이트 제거 후 heuristic이라 달성). 전 hook 5s 예산 대비 여유 大.
- 출처: `internal/adapter/hookmetrics/metrics.go`, 로그 `~/.local/state/issueops/hook-metrics.jsonl`.

## 측정면 5 — Hook 실패율 (Q2)

2026-06-13 (`issueops hook failures stats`):
- total **38**, last_24h **0**, last_7d **5**, oldest 2026-06-04 / newest 2026-06-11.
- by_hook: pre-tool-use 26, user-prompt 5, --help 4, stop 2, post-tool-use 1.
- **ErrHelp 노이즈 제거**: 초기 38건 중 16건이 help 요청이었음 → `Record`가 `flag.ErrHelp` 스킵하도록 수정(실 결함만 집계).
- **rotation**: SessionStart hook이 720h 초과 항목 자동 prune(무한 성장 P1 해소). 로그 13KB.
- 출처: `cmd/issueops/hookcli/hookfailure/`, `internal/adapter/hookfailure/stats.go`.

## 측정면 6 — 운영 안정성 baseline (Q4, 2회분 실측 완료)

- **골격 신설 완료** (2026-06-13): `stability-baseline.md`에 측정 명령·핵심 지표 6종 정의(잔존 daemon/zombie/RSS 추이/MCP 끊김/legacy 잔재/self-verify)·시계열 표·갱신 규약 작성.
- **분류 로직 contract test ✅** (2026-06-13): `classify_processes` 4버킷 라우팅을 `ClassifyProcessesTest` 6케이스로 핀하고, hook/MCP smoke false-fail 회귀를 추가 테스트로 고정(11/11 그린) — Q4 종결조건 ② 충족.
- **실측 2회분 ✅ + final audit ✅** (2026-06-13): `e2e_stability_audit.py --json` evidence-first mode baseline 2회와 final quality audit 모두 green. 각 회차 `host_mcp_checks`, `daemon_mcp_stress`(MCP ids 1–8, temp leak 0), `process_hygiene`(zombie 0, legacy/temp 0), `rss_stability`, `go test ./...`, `go test -race ./...`, `go build`, `self-verify`(10/10·230/230·min score 100) 통과.
- 잔존 daemon은 2개로 관찰됐다: user-level `/Users/m16khb/.local/bin/issueops daemon --internal` + 이 repo dogfood `/Users/m16khb/Workspace/issueops/bin/issueops daemon --internal`.

---

## 갱신 규약 (수동)

- **측정면 1·2** (격리 rubric): SKILL.md 본문 변경 시 또는 분기 도그푸드(S1) 회차마다. holdout은 n≥3 평균±범위, evidence A–C만 최종값(D 추정 금지).
- **측정면 3** (벤치마크): fixtures/dimension 변경 시 + pre-push 상시. judge 실전(Q3)은 별도 1회.
- **측정면 4·5** (hook 텔레메트리): 로그가 상시 누적 → 분기 또는 회귀 의심 시 스냅샷 갱신.
- **측정면 6**: STA 재측정 회차마다 1행 추가.
- 모든 갱신은 측정일을 명기하고, 본 표의 "다음 측정" 칸을 함께 갱신한다.

## 프로그램 종결 판정 (참고)

정량 성공 기준(프로그램 §4): 스킬 visible 측정 16/16(달성, STA-P D-추정 해소), hook 지표 0→4(달성), 벤치마크 상시 게이트(달성), 안정성 시계열 2회분(달성), STA-H n≥3(달성), ACP-O stale-contract fix 후 재측정(달성), IssueOps judge file 실전 1회(달성). D(추정)로 닫힌 항목 0 유지.
