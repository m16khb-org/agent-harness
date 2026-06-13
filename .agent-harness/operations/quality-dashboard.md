# Agent-Harness 품질 대시보드 (단일 가시성 인덱스)

품질향상 프로그램 **Q5** (`.agent-harness/plans/harness-quality-improvement-program.md`).
산재한 정량 지표를 한 페이지로 인덱스한다 — 각 지표군의 **최신값 · 측정일 · 다음 측정 예정 · 출처 문서**.
자동화는 후순위, 우선 수동 갱신 규약(아래 §갱신 규약).

최종 갱신: **2026-06-13**

> **계층 분리 단서 (S6 — 고정 표기, 합산 금지)**
> 아래 지표는 서로 다른 **3개 계층**을 측정하며 한 점수로 합산하지 않는다:
> ① **격리 rubric** (스킬 SKILL.md를 fresh-context에 단독 주입한 품질) — 측정면 1·2,
> ② **통합 벤치마크** (issueops 산출물의 18-dimension 결정적 채점 + pioneer 시그니처) — 측정면 3,
> ③ **런타임 텔레메트리** (hook latency/failure/gate) — 측정면 4·5.
> 격리 점수 4.9가 통합 기여 100을 "증명"하지 않으며 그 반대도 아니다. 세 계층은 삼각측량이다.

---

## 측정면 요약 (한눈)

| # | 측정면 | 계층 | 최신값 | 측정일 | 게이트 | 다음 측정 |
|---|--------|------|--------|--------|--------|-----------|
| 1 | Pioneer 스킬 9종 | 격리 rubric | holdout **4.92/5.0** (전 9종 ≥4.8) | 2026-06-11 | ✅ ≥4.2 | 분기 도그푸드(S1) 또는 SKILL.md 변경 시 |
| 2 | 비-pioneer 스킬 7종 | 격리 rubric | 6/7 측정 완료, 전 측정 holdout ≥4.8 | 2026-06-13 | ⚠️ STA-P 1건 잔여 | STA 재측정(호스트 한도 해제 후) |
| 3 | IssueOps 산출물 벤치마크 | 통합 | **100/100** (avg·min), critical 0, 9 fixtures | 2026-06-13 | ✅ GREEN | pre-push 상시 + judge file 실전(Q3) |
| 4 | Hook 런타임 메트릭 | 런타임 | 전 hook p95 **<40ms**, blocks 0 | 2026-06-13 | ✅ stop p95 38ms (<1s) | 상시 누적(자동) |
| 5 | Hook 실패율 | 런타임 | total 38, last_7d **5**, last_24h 0 | 2026-06-13 | ✅ | 상시 누적(자동) |
| 6 | 운영 안정성 baseline | 런타임 | **골격 신설**(실측 대기), `stability-baseline.md` | 2026-06-13 | ⬜ 2회분 잔여 | STA 재측정과 동시 채움 |

커버리지: 스킬 측정 **16/16 착수** (STA-P 1건만 D-추정 금지 원칙으로 미확정), hook 지표 **0→4** 달성.

---

## 측정면 1 — Pioneer 스킬 9종 (격리 rubric)

- **Holdout 평균: 4.92/5.0** (2026-06-11, firsthand-dogfood SKILL.md 편집 후 fresh-context 재측정, 전 9종 ≥4.8, 회귀 0).
  - 직전: 4.80/5.0 (karpathy·codd 가드레일 수정 후).
- **27-case visible baseline: 3.10/5.0** (v1 척도, 최적화 이전 — 개선 폭의 출발선).
- 강점 스킬: codd 4.36. 가드레일 수정 대상이었던 dijkstra(hollow-method), karpathy(overfit)는 수정 후 holdout 통과.
- 출처: `pioneer-skill-quality-scorecard.md`, 증거 `evidence/pioneer-skills-quality/reruns/post-optimization-measurement-2026-06-11.md`.
- 단서: 격리·단일런(holdout n=1 fresh-context). 통합 기여는 측정면 3이 별도 담당.

## 측정면 2 — 비-pioneer 스킬 7종 (격리 rubric)

2026-06-13 측정 (가중 평균 / holdout n≥3 평균±범위):

| 스킬 | 가중 점수 | Holdout | gate flag | evidence |
|------|-----------|---------|-----------|----------|
| issueops | 5.0 | 5.0 ±0 (n=3) | none | A·A/B + H:C |
| atomic-commit-push | 4.1 (cap 3.4) | 4.9 ±0 (n=3) | stale-contract(수정완료) | A + H:C |
| self-verify | 5.0 | 4.9 ±0 (n=3) | none | A + H:C |
| self-augment | 5.0 | 4.9 ±0 (n=3) | none | A + H:C |
| project-bootstrap | 4.86 | 5.0 ±0 (n=3) | none | A + H:A |
| draft-wiki-promoter | 4.94 | 4.8 ±0 (n=3) | none | A/C/A + H:C |
| stability-audit | — (STA-P 보류) | 5.0 (n=1, 재측정 대기) | none | A + H:A |

- **잔여**: STA-P(풀 audit 스크립트 실행 포함)는 2026-06-11 호스트 사용량 한도로 중단 → D-추정 금지 원칙대로 미확정. STA-H도 n=1만 확보(run 2·3 대기). ACP-O는 stale-contract cap 수정 커밋 반영 후 재측정 시 cap 3.4 해제 예정.
- **v2 세분화**(0.1 단위 + 5.0 유보 규칙 + proportionality)는 신규 측정부터 의무. 천장 효과(21케이스 중 17건 5.0) 실측이 동기.
- 출처: `harness-skill-quality-scorecard.md`.

## 측정면 3 — IssueOps 산출물 벤치마크 (통합)

- **18-dimension** 결정적 채점 + pioneer 시그니처 + N/A 제외 + A/B 게이트.
- 최신 (2026-06-13, `--judge none`, fixtures 9건): **average 100 / minimum 100 / critical_failure 0** → 게이트 GREEN.
- `--judge file` 백엔드(fresh-context 서브에이전트 JSON 채점 map) 구축 완료. **judge 실전 1회전(Q3)은 잔여** — deterministic-only 점수와의 분별력 비교 미수행.
- 실행: `agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json`.
- 출처: `testdata/issueops/fixtures/`, `internal/core/issueops/benchmark/`.

## 측정면 4 — Hook 런타임 메트릭 (Q2 신규 측정면)

2026-06-13 (`agent-harness hook metrics`, 총 303 events):

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
- 출처: `internal/core/hookmetrics/metrics.go`, 로그 `~/.local/state/agent-harness/hook-metrics.jsonl`.

## 측정면 5 — Hook 실패율 (Q2)

2026-06-13 (`agent-harness hook failures stats`):
- total **38**, last_24h **0**, last_7d **5**, oldest 2026-06-04 / newest 2026-06-11.
- by_hook: pre-tool-use 26, user-prompt 5, --help 4, stop 2, post-tool-use 1.
- **ErrHelp 노이즈 제거**: 초기 38건 중 16건이 help 요청이었음 → `Record`가 `flag.ErrHelp` 스킵하도록 수정(실 결함만 집계).
- **rotation**: SessionStart hook이 720h 초과 항목 자동 prune(무한 성장 P1 해소). 로그 13KB.
- 출처: `cmd/harness/hookcli/hookfailure/`, `internal/core/hookfailure/stats.go`.

## 측정면 6 — 운영 안정성 baseline (Q4, 골격 신설·실측 대기)

- **골격 신설 완료** (2026-06-13): `stability-baseline.md`에 측정 명령·핵심 지표 6종 정의(잔존 daemon/zombie/RSS 추이/MCP 끊김/legacy 잔재/self-verify)·시계열 표·갱신 규약 작성.
- **실측 행은 대기**: `e2e_stability_audit.py --json` 2회분 측정 + 스크립트 분류 로직 contract test 1개가 종결 조건. D-추정으로 행 미기록.
- **STA-P/STA-H 재측정(측정면 2)과 함께** 호스트 사용량 한도 해제 후 채움.

---

## 갱신 규약 (수동)

- **측정면 1·2** (격리 rubric): SKILL.md 본문 변경 시 또는 분기 도그푸드(S1) 회차마다. holdout은 n≥3 평균±범위, evidence A–C만 최종값(D 추정 금지).
- **측정면 3** (벤치마크): fixtures/dimension 변경 시 + pre-push 상시. judge 실전(Q3)은 별도 1회.
- **측정면 4·5** (hook 텔레메트리): 로그가 상시 누적 → 분기 또는 회귀 의심 시 스냅샷 갱신.
- **측정면 6**: STA 재측정 회차마다 1행 추가.
- 모든 갱신은 측정일을 명기하고, 본 표의 "다음 측정" 칸을 함께 갱신한다.

## 프로그램 종결 판정 (참고)

정량 성공 기준(프로그램 §4): 스킬 측정 16/16(STA-P 잔여 1건), hook 지표 0→4(달성), 벤치마크 상시 게이트(달성), 안정성 시계열 2회분(잔여), 전 측정 n≥3 분산 규율(신규 측정 적용). D(추정)로 닫힌 항목 0 유지.
