# Agent-Harness 품질향상 프로그램 (전수조사 기반, 정량+정성 투트랙)

작성일: 2026-06-11
근거: ① 이 세션의 평가체계 진단 보고서(`.agent-harness/research/pioneer-skill-evaluation-and-issueops-integration-assessment.md`),
② 16개 스킬 전수 인벤토리(Explore), ③ hooks 7-이벤트 전수 인벤토리(Explore), ④ pioneer 벤치마크 통합 구현 완료
(commits 5e2cc4d..b59bc8a, 18-dimension + `--judge file`), ⑤ CAUTIONS/PROJECT_AUDIT/ISSUEOPS_AUDIT의 기존 P0–P2.

## 0. 전수조사 핵심 발견 (현재 상태 스냅샷)

### 측정 커버리지 비대칭이 가장 큰 구조적 결함이다

| 영역 | 측정 현황 | 출처 |
|------|-----------|------|
| Pioneer 스킬 9종 | rubric+27케이스+홀드아웃, 4.92/5 (단일런) | scorecard |
| 비-pioneer 스킬 7종 (issueops, atomic-commit-push, self-verify, self-augment, project-bootstrap, karpathy*, draft-wiki-promoter, stability-audit) | **스코어카드/홀드아웃 0** (Go 테스트는 있음: issueops 64파일 ↔ stability-audit 0파일로 편차 극단) | 스킬 인벤토리 |
| issueops 산출물 | 18-dimension 벤치마크 + pioneer 시그니처 + A/B 게이트 (이번 세션 구축) | 5e2cc4d..b59bc8a |
| hooks 7이벤트 | 테스트 LOC 2.2K > 구현 1.4K로 양호하나 **런타임 메트릭 0개**(latency/failure rate/gate hit rate/큐 깊이 전부 미측정) | hooks 인벤토리 |
| 운영 안정성 | e2e_stability_audit.py 존재하나 수동·비정기, contract test 0 | stability-audit 인벤토리 |

*karpathy는 pioneer 스코어카드에 포함되나 Go contract test 0건.

### 강점 (유지·확산할 패턴)
- 16/16 스킬 모두 상단 활성 게이트 보유(이전 도그푸드 최적화의 성과) — "묻힌 게이트" 문제 해소됨.
- stale CLI 명령 0건(과거 `--command-argv`, `spawn_agent` 등 모두 제거 확인).
- 13/16 스킬 NEVER/Forbidden 안전 규칙 보유.
- hooks 차단 게이트 8종이 실측 테스트로 핀됨.

### 이번 전수조사가 발굴한 신규 실측 결함

- **[P1, 신규] lifecycle state 동시성 레이스**: `go test ./internal/core/lifecycle ./cmd/harness/... -count=3` 병렬 부하에서
  `TestInitProjectLifecycleStateConcurrentNoDuplicates`가 재현 실패(`lifecycle_state_test.go:189: unexpected end of JSON input`).
  단독 실행은 통과 → 동시 InitProjectLifecycleState 중 부분 기록된 상태 파일을 읽는 read-while-write 레이스 의심.
  **이것이 stability-audit fast-path의 self-verify(go test 스텝) 간헐 실패의 원인이다**(실측). hopper 진단 → 원자적 쓰기
  (temp+rename) 또는 파일 잠금으로 수정. → 신규 항목 **Q6**로 트랙.

### 기존 문서화된 결함 (재발견 아님, 미해결 잔존)
- **P0**: 세션↔IssueOps 사이클 바인딩 부재 — git 브랜치로만 추정, 세션 재시작 시 유실 (ISSUEOPS_AUDIT 2.1).
- **P1**: hook failure log 무한 성장(rotation 없음, `hookfailure-*.jsonl`); `HARNESS_EXPECTED_WORKTREE` env가 compaction에 휘발 (ISSUEOPS_AUDIT 2.2).
- **P2**: PIPE_BUF 초과 동시 append interleave; Codex/Claude 렌더링 drift.
- 세션 중 `agent_harness` MCP 재접속 끊김이 이번 세션에서도 재현됨(도그푸드 관찰) — 안정성 신호.

---

## 1. 정량 트랙 (측정 가능한 품질)

> 원칙: "측정 없는 개선 주장 금지"는 이미 레포 철학(벤치마크 design non-goal, rubric evidence-bound). 비어 있는 측정면을 채우고, 이미 있는 측정을 정기화한다.

### Q1. 비-pioneer 스킬 스코어카드 확장 — 커버리지 9/16 → 16/16 [최우선]
- **무엇**: 기존 `pioneer-skill-quality-rubric.md`(5-dimension, gate flag, A–D evidence, holdout 프로토콜)를 그대로 재사용해
  `.agent-harness/operations/harness-skill-quality-scorecard.md`(비-pioneer 7종)를 신설. 스킬당 3 visible 케이스 + 1 holdout.
- **왜 정량**: 현재 비-pioneer 스킬의 품질 주장은 전부 근거 등급 D(추정). rubric은 D를 최종 판정에 금지한다 — 자기 기준 위반 상태.
- **수용 기준**: 7스킬 × 4케이스 결과 기록, 전 스킬 ≥4.2 또는 gate-flag별 개선 task 발행. 측정은 fresh-context 서브에이전트(rubric 88–124행 프로토콜).
- **분산 규율**: holdout은 n≥3 평균±범위로 기록(보고서 권고 4 — 단일런 4.92의 낙관 제거를 신규 측정에는 처음부터 적용).
- 규모: 측정 작업(코드 변경 없음). 1~2 세션.

### Q2. Hook 런타임 메트릭 도입 — 현재 0개 → 4개 [신규 측정면]
- **무엇**: hookcli에 경량 계측 추가 —
  (a) 이벤트별 latency(5s 예산 대비 실측 분포), (b) 실패율(`hookfailure-*.jsonl` 집계 명령 `hook failures stats --json`),
  (c) enforcement gate hit rate(차단/통과 카운트), (d) draft-wiki 큐 깊이.
- **왜**: 8개 차단 게이트가 매 tool call 임계 경로에 있는데 회귀를 감지할 수단이 없음(hooks 인벤토리 §6 "currently unmeasured").
- **수용 기준**: `agent-harness hook stats --json`이 4지표 반환 + Go 테스트; Stop hook p95 latency < 1s(LLM 게이트 제거 후 heuristic이므로 달성 가능해야 정상).
- **동반 수정(P1)**: failure log rotation을 설치 시 기본화(`prune --max-age 720h`를 SessionStart 또는 주기 실행에 연결).
- 규모: Go 변경 소. 1 세션. **issueops 사이클로 실행.**

### Q3. issueops 벤치마크 정기화 + judge file 실전 1회전 [구축물 활용]
- **무엇**: (a) 이번에 구축한 `--judge file` 백엔드의 첫 실전 사이클 — fresh-context 서브에이전트가 9-fixture 채점 map을 생산하고 병합, deterministic-only 점수와 비교해 judge 레이어의 분별력 기록.
  (b) `benchmark run --judge none`을 CI/pre-push 체크리스트에 명문화(TESTING.md)해 18-dimension 회귀를 상시 게이트화.
- **수용 기준**: judge-merged run 결과 1건이 state에 저장되고 비교 리포트가 scorecard에 1절로 기록; TESTING.md에 벤치마크 게이트 1줄.
- 규모: 소. 1 세션.

### Q4. 운영 안정성 정량 baseline [stability-audit 정례화]
- **무엇**: `e2e_stability_audit.py --json`을 (a) 주기 실행 가능한 형태로 두고(예: 주 1회 수동 or cron), (b) 핵심 지표 — 잔존 daemon 수, zombie 수, RSS 추이, MCP 재접속 끊김 횟수 — 를 시계열로 `.agent-harness/operations/stability-baseline.md`에 누적.
- **왜**: 이번 세션에서도 MCP 서버 끊김이 재현됐으나 빈도·조건이 미기록이라 개선 불가.
- **수용 기준**: baseline 문서에 최소 2회분 측정치; stability-audit 스크립트의 Go contract test 1개 이상(현재 0 — 685줄 Python이 무테스트).
- 규모: 소~중.

### Q6. lifecycle state 동시성 레이스 수정 [신규 실측 결함, P1]
- **무엇**: §0의 신규 발견 — InitProjectLifecycleState의 read-while-write 레이스를 hopper 7-step으로 진단 후
  원자적 쓰기(temp 파일 + rename) 또는 flock으로 수정. 재현 명령(`-count=3` 병렬)이 이미 확보돼 있어 RED가 공짜.
- **왜 정량**: self-verify 95-게이트의 간헐 실패 원인(실측) — 이걸 고쳐야 Q4 안정성 baseline의 신호가 깨끗해진다.
- **수용 기준**: 재현 명령 10회 연속 그린; `go test -race ./internal/core/lifecycle -count=3` 그린; self-verify 재실행 통과.
- **issueops 사이클로 실행** (동시성 코드 변경).

### Q5. 정량 대시보드 단일화 [통합 가시성] — ✅ 완료 (2026-06-13)
- **무엇**: 산재한 정량 지표(스킬 스코어카드 2종, 벤치마크 run, hook stats, stability baseline)를 한 문서
  `.agent-harness/operations/quality-dashboard.md`로 인덱스(링크+최신값+측정일). 자동화는 후순위, 우선 수동 갱신 규약.
- **수용 기준**: 5개 지표군의 최신값·측정일·다음 측정 예정일이 1페이지에서 보임.
- 규모: 소.
- **결과**: `quality-dashboard.md` 신설 — 6개 측정면(pioneer 4.92, 비-pioneer 7종, 벤치마크 100/100,
  hook 메트릭 p95<40ms, hook 실패율, 안정성 baseline 잔여)을 계층 분리 단서(S6)와 함께 1페이지 인덱스.
  실측값 전부 라이브 CLI(`hook metrics`/`hook failures stats`/`benchmark run`)로 채움.

## 2. 정성 트랙 (측정이 못 잡는 품질)

> 원칙: 이번 라운드에서 입증된 두 채널 — ① firsthand 도그푸드(격리 측정이 못 잡는 마찰 적발: ugrep 결함, 묻힌 게이트), ② 다관점 적대 리뷰(critic×2+verifier가 decode 버그·facade 누락·silent no-op을 구현 전 차단) — 를 제도화한다.

### S1. Firsthand 도그푸드 정례화 [입증된 채널의 제도화]
- **무엇**: `pioneer-skill-optimization-strategy.md`의 방법(직접 호출→강점/마찰/EDIT-MEASURE-HOLD 분류)을 분기당 1회, 비-pioneer 스킬까지 확장해 운영 절차로 명문화(rubric에 1절 추가).
- **왜 정성**: 격리 holdout이 4.92여도 직접 사용에서만 드러난 결함이 다수였음(보고서 §Q1 한계 1). 점수가 못 보는 면을 보는 유일한 채널.
- **수용 기준**: 절차가 rubric에 기록되고, 첫 회차에서 스킬당 ①강점 ②마찰 ③전략 레코드 생산.

### S2. 다관점 사전 리뷰를 플랜 게이트로 [입증된 채널 2]
- **무엇**: "코드 변경 플랜은 구현 전 critic(정확성) + verifier(검증-적정성) 2-pass를 통과"를 SUB_AGENT_PATTERNS.md 또는 CONVENTIONS.md에 규약화. 이번 라운드에서 blocker 3건을 구현 전에 잡은 실측 근거 인용.
- **수용 기준**: 규약 1절 + 다음 코드 플랜 1건에 적용된 기록.

### S3. 결손 contract test 3건 신설 [정성→정량 전환]
- **무엇**: contract test 0건인 karpathy(451줄)·draft-wiki-promoter(50줄)·stability-audit(90줄+685줄 스크립트)에 최소 핀 테스트. 단, 계층-C 한계(텍스트 존재≠행동)를 테스트 주석에 명시하는 이번 라운드 관례 유지.
- **수용 기준**: 3개 테스트 파일, `go test ./...` 그린.

### S4. P0/P1 구조 결함 해소 [정성적 신뢰성]
- **무엇**: (a) **P0** 세션↔사이클 바인딩: SessionStart hook이 활성 IssueOps 사이클 ID를 state에 기록·복원.
  (b) **P1** `HARNESS_EXPECTED_WORKTREE`를 IssueOps 레코드에 영속화하고 hook이 env+state 양쪽 확인.
- **왜 정성**: 사용자가 체감하는 "세션이 끊겨도 흐름이 이어진다"는 신뢰성 — 메트릭보다 워크플로 정합성 문제.
- **수용 기준**: 세션 재시작 시뮬레이션 테스트(기존 hook 테스트 패턴 재사용) 그린; ISSUEOPS_AUDIT 해당 항목 resolved 표기.
- **issueops 사이클로 실행** (코드 변경 중).

### S5. Proportionate mode 확산 5/16 → 적용 가능 전수 [검증된 패턴 확산]
- **무엇**: turing/karpathy/codd/shannon/von-neumann에서 검증된 비례 모드를, 고정-무게로 남은 스킬 중 적용 가치가 있는 것(berners-lee quick-lookup은 일부 반영됨 → issueops 경량 사이클, atomic-commit-push 단일 파일 fast-path 등)에 확장. 단 **measured gap 없으면 EDIT 금지**(keep/discard 규율 유지) — S1 도그푸드 결과로 대상 선별.
- **수용 기준**: S1 결과에서 비례성 마찰이 기록된 스킬에만 EDIT; 각 EDIT 후 해당 스킬 holdout 재측정 무회귀.

### S6. 정직성 단서 일괄 정비 [보고서 권고 3 잔여분]
- **무엇**: scorecard의 4.92에 "격리·단일런 한정, 통합·활성 미측정" 단서 명시(이번 벤치마크 구축으로 '통합' 일부는 측정 시작됨 — 그 사실도 반영), pioneer 키워드-프록시 점수와 격리 rubric 점수가 별개 계층임을 dashboard(Q5)에 고정 표기.
- **수용 기준**: scorecard·dashboard 문구 반영.

## 3. 우선순위·의존성·실행 단위

| 순번 | 항목 | 트랙 | 효과/노력 | 실행 단위 | 의존 |
|------|------|------|-----------|-----------|------|
| 0 | **Q6 lifecycle 레이스(신규 P1)** | 정량 | 高/小 | **issueops 사이클** | — (재현 확보, RED 공짜) |
| 1 | Q1 비-pioneer 스코어카드 | 정량 | 高/中 | 측정 세션(코드 무변경) | — |
| 2 | S4 P0/P1 구조 결함 | 정성 | 高/中 | **issueops 사이클** | — |
| 3 | Q2 hook 메트릭+rotation | 정량 | 高/小 | **issueops 사이클** | — |
| 4 | S3 contract test 3건 | 정성 | 中/小 | 직접 또는 Q1과 병행 | — |
| 5 | Q3 벤치마크 정기화+judge 실전 | 정량 | 中/小 | 직접 | 이번 구축물 |
| 6 | S1 도그푸드 정례화 | 정성 | 高/小(문서)+中(첫 회차) | 직접 | — |
| 7 | S5 비례 모드 확산 | 정성 | 中/中 | S1 결과 의존 | S1 |
| 8 | Q4 안정성 baseline | 정량 | 中/小 | 직접 | — |
| 9 | S2 리뷰 게이트 규약화 | 정성 | 中/小 | 문서 | — |
| 10 | Q5+S6 dashboard+단서 | 양쪽 | 中/小 | 문서 | Q1–Q4 |

병렬 가능: {1,2,3,4}는 상호 독립. {5,8,9}는 틈새 실행. {7,10}은 후행.

## 4. 성공 판정 (프로그램 수준)

정량: 스킬 측정 커버리지 16/16, hook 지표 0→4, 벤치마크 게이트 상시화, 안정성 시계열 2회분 이상, 전 측정에 n≥3 분산 규율.
정성: P0/P1 해소 검증 테스트 그린, contract test 결손 0, 도그푸드·리뷰 게이트가 운영 문서에 제도화, 모든 점수에 적용범위 단서.
공통: 각 항목은 evidence-bound(rubric A–C)로 종결 — D(추정)로 닫히는 항목 없음.

## 5. 비-목표 (이번 프로그램에서 하지 않음)

- 라이브 스킬 호출 측정(CI 결정성 원칙 유지 — 키워드 프록시·도그푸드·judge 3채널로 충분히 삼각측량).
- 비-pioneer 스킬 본문 선제 개편(측정 Q1 결과 없이 EDIT 금지).
- `issueops remote score`의 agy 경로 교체(별도 후속 — 이번 judge file 패턴 재사용 가능).
- `branch_worktree_gate_quality`의 `feature/` 접두사 모순 수정(별도 이슈로 이미 기록).
- hook 메트릭의 자동 대시보드화(수동 규약 우선, 자동화는 가치 입증 후).
