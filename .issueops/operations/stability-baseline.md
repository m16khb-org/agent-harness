# Agent-Harness 운영 안정성 Baseline (시계열)

품질향상 프로그램 **Q4** (`.issueops/issues/_unnumbered/harness-quality-improvement-program.md`).
`e2e_stability_audit.py --json`의 핵심 지표를 시계열로 누적해, 이번 세션에서도 재현된 MCP 재접속 끊김처럼
"빈도·조건 미기록이라 개선 불가"하던 안정성 신호에 측정면을 부여한다.

이 문서는 **Q5 대시보드 측정면 6**의 출처다. 갱신 시 `quality-dashboard.md`의 해당 행도 함께 갱신한다.

최종 갱신: **2026-06-13** (Q4 evidence-first 실측 + final quality audit 기록)

> **현황**: Q4 baseline 수용 기준(최소 2회분 측정치 + stability-audit 스크립트 contract
> test)을 2026-06-13 충족했다. 두 회차 모두 `e2e_stability_audit.py --json` evidence-first
> mode로 실행했고, D-등급(추정) 행은 기록하지 않았다.

---

## 측정 명령

증거 우선 audit (프로세스 죽이거나 config 변경 안 함):

```bash
python3 skills/stability-audit/scripts/e2e_stability_audit.py --json > /tmp/sta-$(date +%Y%m%d).json
```

풀 설치 + 정리 패스 (호스트 부수효과 — install E2E 과제일 때만):

```bash
python3 skills/stability-audit/scripts/e2e_stability_audit.py --full-install --cleanup-stale --json
```

보고서 top-level: `ok`, `started_at`/`finished_at`, `steps[]`, `failures[]`.
스텝: `git_status` · `build` · `install_checks` · `host_mcp_checks` · `hook_smoke` ·
`temp_state_worker_policy` · `process_hygiene` · `cleanup_stale` · `rss_sample` · `regression`.

## 핵심 지표 정의 (프로그램 Q4 명시 4종 + 보강)

| 지표 | 출처 스텝 / 필드 | 정상 판정 |
|------|------------------|-----------|
| 잔존 daemon 수 | `process_hygiene.classified.current_daemons[]` 길이 | 의도된 user-level + 이 repo dogfood daemon 외 0 |
| zombie 수 | `process_hygiene.classified.zombies[]` 길이 (state `Z` ∧ issueops/bin/harness/codegraph) | 0 |
| RSS 추이 | `rss_sample` (rounds×calls 후 daemon RSS) | 라운드 간 단조 증가 ✗ (단일 Go 런타임 warmup 점프는 허용) |
| MCP 재접속 끊김 | `host_mcp_checks`(claude/codex mcp) + daemon_and_mcp_stress의 mcp 왕복 실패 | 0 (도그푸드 관찰도 비고에 병기) |
| legacy/temp 잔재 | `classified.legacy_harness[]`, `temp_watchers[]` | stale로 확인된 것만, 0 지향 |
| self-verify | `regression.self-verify.summary` | ok ∧ termination_eligible |

## 시계열 측정 기록

> 채우는 규칙: 1행 = 1회 audit. `ok`는 `failures[]` 비었는지. RSS는 첫 라운드→마지막 라운드 KB.
> MCP 끊김은 audit 스텝 실패 + 세션 중 도그푸드 관찰을 합산(관찰은 괄호 병기).

| 측정일 | mode | ok | 잔존 daemon | zombie | RSS 추이(KB) | MCP 끊김 | self-verify | 비고 |
|--------|------|----|-----------|--------|-------------|----------|-------------|------|
| 2026-06-13 11:51–12:05 KST | evidence-first | ✅ | 2 (user daemon + repo dogfood daemon) | 0 | 15152→16800 (+1648), deltas 768/592/288 | 0 | ✅ 10/10, 230/230, min score 100 | `/tmp/issueops-sta-run1-final-20260613.json`; MCP ids 1–8, temp leak 0, legacy/temp 0 |
| 2026-06-13 12:06–12:19 KST | evidence-first | ✅ | 2 (user daemon + repo dogfood daemon) | 0 | 15072→16400 (+1328), deltas 720/480/128 | 0 | ✅ 10/10, 230/230, min score 100 | `/tmp/issueops-sta-run2-final-20260613.json`; MCP ids 1–8, temp leak 0, legacy/temp 0 |
| 2026-06-13 13:40–13:48 KST | evidence-first | ✅ | 2 (user daemon + repo dogfood daemon) | 0 | 14272→17168 (+2896), deltas 960/1712/224 | 0 | ✅ 10/10, 230/230, min score 100 | `/tmp/issueops-final-stability-audit-20260613.json`; MCP ids 1–8, temp leak 0, legacy/temp 0, regression ok |

## 갱신 규약

- 측정 회차마다 위 표에 1행 추가, 측정일 명기. `quality-dashboard.md` 측정면 6 동시 갱신.
- 회귀 의심(daemon 누수·zombie 발생·RSS 단조 증가·MCP 끊김 증가) 시 `failures[]` 원문을 비고에 인용하고 debugging 진단으로 라우팅.
- RSS는 단일 warmup 점프와 다회 단조 증가를 구분(스크립트 주석 규약 유지) — 1회 점프를 누수로 판정하지 않는다.

## 잔여 작업 (Q4 종결 조건)

1. ✅ **실측 2회분** (2026-06-13): evidence-first audit 2회 모두 green. `host_mcp_checks`, `daemon_mcp_stress`, `process_hygiene`, `rss_stability`, `go test ./...`, `go test -race ./...`, `go build`, `self-verify --full --iterations=10 --seed=100 --target-score=95` 통과.
2. ✅ **분류 로직 contract test** (2026-06-13): `classify_processes`의 daemon/legacy/temp-watcher/zombie 4버킷 라우팅을 `e2e_stability_audit_test.py::ClassifyProcessesTest` 6케이스로 핀(zombie는 Z-state ∧ harness-command 동시 조건, 무관 프로세스 미분류, 빈 입력 포함). 계층-C 한계(시그니처 매칭 핀이지 라이브 ps 열거/cleanup 정확성 증명 아님)를 테스트 docstring에 명시. `python3 skills/stability-audit/scripts/e2e_stability_audit_test.py` 11/11 그린.
