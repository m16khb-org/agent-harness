# Agent-Harness 품질·성능 향상 프로그램 v3 (2026-07-03)

> 작성일: 2026-07-03 · 방식: 4-lens 병렬 심층분석(아키텍처 / 코드품질 / 성능측정 / 테스트갭) + 기존 감사·프로그램 대조
> 위치: v1(2026-06-11 전수조사), v2(2026-06-15 외부벤치마킹), 최적화 계획(2026-06-16), 성능 계획(2026-07-02)을 **승계**한다(폐기 아님).
> 원칙 계승: 이미 FIXED 항목 재제안 금지 · 측정 없는 최적화 금지(성능 계획 2026-07-02 결정) · YAGNI 추상화 금지(PROJECT_AUDIT L1/C4 기각 선례) · 전문기능 재발명 금지(AGENTS.md).

## 0. 현재 상태 스냅샷 (2026-07-03 라이브 검증)

- 빌드/vet: 통과 (`go build ./...`, `go vet ./...` exit 0)
- 테스트: **`cmd/harness/harnessapp` `TestResponseContractsGolden` 결정적 FAIL** (§2.1) — 나머지 전 패키지 ok
- `quality inspect`: audit P1/P2 열린 항목 0 · 저커버리지 5개 패키지(52.9~56.4%, 게이트 60%) · branch candidate 299(구조적 노이즈로 기 triaged)
- 기존 감사 상태: PROJECT_AUDIT(2026-06-14)·ISSUEOPS_AUDIT — P0~P2 전량 해결/수용/범위외 처리, 2026-07-01 HEAD 재대조 완료
- v2 프로그램 이행 확인: A1 CI(`.github/workflows/ci.yml`) · A2 관측 분모(hookmetrics rate/percentile) · A3 pass^k(`issueops/benchmark/issueops_reliability.go`) 구현 완료
- 최근 개발 집중: 2026-06-20 이후 issueops(feat 14/fix 9/docs 8) — phase ledger, devils-advocate loop, regress-round cap
- 규모: 비테스트 55k LOC · 테스트 파일 415개 · internal/core 40+ 패키지 · issueops 비테스트 9,185 LOC(21 subpackage)

## 1. 아키텍처 분석 결과 (arch lens)

**총평: hexagonal 경계 거의 건전.** `core→adapter` 0건, `core→cmd` 0건, `port→core/adapter` 0건. facade는 god-package가 아니라 얇은 type-alias/위임 seam(root 1,465 LOC vs subpackage 24,787 LOC, 6%). issueops는 이미 21개 subpackage로 분해된 coordinator 구조로 외부 importer 3개뿐. MCP dispatch는 map 기반 handler registry로 건전.

| ID | 심각도 | 발견 | 근거 | 처방 |
|----|--------|------|------|------|
| A-1 | **P1** | adapter→cmd 의존 역전: gitlab provider가 `cmd/harness/issueopscli/remoteparse`를 import | `internal/adapter/provider/gitlab/provider.go:11,629` | `remoteparse`를 `internal/adapter/provider/remoteparse/`로 이동(stdlib-only라 안전). blast 3파일, 로직 무변경 |
| A-2 | P2 | cmd가 facade를 우회해 core subpackage 7종 직접 import (qualitycatalog 4, webfetch 3, externalllm 3 등) | grep import 스캔 | 새 facade 래퍼 남발 금지. (a) state/issueops는 facade 경유 통일, (b) 내부 도구는 `internal/core/doc.go`에 "cmd 직접 import 허용" 예외 명문화 |
| A-3 | P3 | `missingStrings`/`firstNonEmpty` 완전 중복 (github/gitlab provider) | `github/provider.go:356,371` = `gitlab/provider.go:687,702` | 무상태 순수 헬퍼만 `internal/adapter/provider/` root로 추출(~20 LOC). **CreateChild 등 본문 공통화는 거부**(gh/glab·REST/GraphQL 차이로 leaky — L1/C4와 동형) |
| A-4 | P3 | core root facade exported 395 심볼 / importer 99파일 → 재컴파일 결합 | 심볼 카운트 | **조치 불요, 모니터링만** — 별칭 위주라 분해는 과잉 |
| A-5 | P3 | `remotecmd/remote.go` 747 LOC 다중 관심사 | `cmd/harness/issueopscli/remotecmd/remote.go` | 필수 아님. 성장 시 verb 그룹으로 파일 분리 |

## 2. 코드 품질·테스트 분석 결과 (quality + test lens)

**총평: 방어적 코딩 수준 높음(P1 정합성 결함 0).** 상태 쓰기 temp+rename+flock, 스키마 버전 검증, reliability 계산의 Inf/NaN 차단은 모범적. 결함은 신규 관측/자기증강 기능의 의미적 불완전성과 테스트 인프라 갭에 집중.

### 2.1 즉시 수정 (머지 게이트 차단 중)

- **T-1 [P0-운영] `TestResponseContractsGolden` 결정적 FAIL**: 커밋 `6dda6cb`가 `model/types.go`에 `RegressEvents`(`regress_events`, omitempty) 필드를 추가하며 "response goldens unchanged"라 주장했지만, golden fixture의 IssueOps 시나리오가 regress를 유발해 필드가 실제로 채워짐 → `cmd/harness/testdata/response_contracts.golden.json` 5개 위치 diff. 3회 재실행 모두 FAIL(플레이키 아님). CI가 `go test ./... -count=1`을 돌리므로 **매 push/PR 머지 게이트 차단**. 수정: `-update` 후 diff 검토·커밋(TESTING.md §4 "의도적 계약 변경 시에만 golden 갱신" 절차 준수).

### 2.2 결함 목록

| ID | 심각도 | 발견 | 근거 | 처방 |
|----|--------|------|------|------|
| Q-1 | P2 | externalllm usage 관측이 **실패 호출을 전혀 기록하지 않음** — `OK` 필드가 false가 될 수 없는 죽은 코드. 타임아웃·레이트리밋이 비용/신뢰도 관측에서 불가시화(커밋 2347ac9 의도 훼손) | `internal/core/externalllm/print.go:174-182` (실패 반환 경로 129,141,158,161,164행 모두 관측 스킵) | 에러 경로에서 `OK:false, DurationMS:경과` 관측 1건 기록 (defer/wrapper) |
| Q-2 | P2 | self-augment lesson 패널티에 최근성 윈도우/감쇠 없음 → severe ≥2 후보는 개선 후에도 매 planning −30점 영구 강등, curriculum rotation에서 배제 | `cmd/harness/selfworkflow/augmentplan/plan_lesson_penalty.go:32-82` (decay/리셋 grep 0건) | 최근성 윈도우(N일) 또는 감쇠/총량 캡 또는 후보 close 시 lesson 무효화. ※ 영속이 의도된 설계인지 선확인 |
| Q-3 | P3 | usage/lesson 상태 레코드 무한 증가(자동 prune 없음) + lesson 스캔이 총 이력에 비례 | `internal/core/external_llm_usage.go:54`, `augmentlesson/self_augment_lesson_state.go:68` | 보존 정책 + 스케줄 prune, 스캔 윈도우 제한 (P-3과 묶음) |
| Q-4 | P3 | usage 관측 provider가 `defaultProvider`("zai") 하드코딩 / regress cap 거부 메시지가 실제 카운트 대신 상수 출력 | `print.go:176` / `issueops_regress.go:57-60` | 해소된 provider 변수 전달 / `len(record.RegressEvents)` 출력 |
| T-2 | P1 | **CI에 `-race` 부재** — 동시성 코드(state/worker/issueops/daemon) 다수인데 회귀 안전망 없음. 수동 실행 결과 현재 전부 PASS(state+worker ~50s, issueops 46s) | `.github/workflows/ci.yml` (race 언급 0) | CI job에 `go test -race ./internal/core/... -count=1` 추가 |
| T-3 | P2 | `internal/core` 55.4% 커버리지는 **facade 위임부 오탐** — 미커버 전부 `*_facade.go` 1-라인 위임. 실로직 갭 아님 | `go tool cover -func` 확인 | 커버리지 게이트에서 facade 파일 제외(또는 별도 임계). 단 인자 변환/에러 래핑 포함 facade는 smoke test 1개씩 |
| T-4 | P2 | 실제 로직 미검증 갭: `gjc.canWriteTo` 0%(설치 실패 처리 분기), `webfetchcli.loadFixtures` 24%(fixture shape 판별 파싱), `feedbackcleanup.RunCleanup` 36%(close-children 미커버) | `install.go:105-116`, `webfetch.go:118-174` | 각각 3~4 케이스 유닛 테스트 추가 |
| T-5 | P2 | `mcp_sdk_server.go` 9개 함수 전부 0% — 참조는 `mcp_transport.go:27` 1곳뿐이고 그 경로도 테스트 미실행. dead code인지 대기 코드인지 불명 | `cmd/harness/mcpcli/mcp_sdk_server.go` | 정체 판정 후 삭제 또는 통합 테스트 1개 (PROJECT_AUDIT M1의 "SDK transport 의도적 유지" 결정과 대조 필요) |
| T-6 | P3 | 시간 기반 유일성 anti-pattern 반복: 9f6b866의 나노초 fix도 `time.Now()` 의존, `makeWorkerJobID`·`audit_id.go`에 동일 구조 재발명 | `worker/store.go:242`, `policy/auditid/audit_id.go:17` | 표준 유일 ID 헬퍼(counter/UUID) 1개로 수렴 |
| QC-SYNC | P2 | `qualitycatalog` 후보 10건 중 4건이 이미 해결된 감사 항목과 중복(daemon-connection-limit, worker-stuck-running-detection, state-write-locking, draftwiki-stale-lock) → self-augment 루프가 죽은 후보를 계속 노출 | `quality inspect` 출력 vs PROJECT_AUDIT resolved 표 | 후보 카탈로그에 resolved 상태 반영(감사 문서와 동기화 규약) |

## 3. 성능 측정 결과 (perf lens, 실측)

**총평: hook 코드 경로와 MCP proxy는 건강. 병목의 실체는 무한 성장 상태 파일 3종.** MCP proxy는 영속 소켓 + 양방향 `io.Copy` zero-copy로 최적화 여지 없음. per-hook 순수 FS 비용 ~0–20ms.

실측 기준 지연(p50, 유휴, macOS arm64 / 측정 floor ~48ms):

| 경로 | p50 | floor 위 순수비용 | 빈도 |
|---|---|---|---|
| hook pre-tool-use / post-tool-use | 68 / 74ms | ~20 / 26ms | 매 tool 호출 |
| hook user-prompt / stop | 88 / 80ms | ~40 / 32ms | 매 턴 |
| **hook session-start** | **867–1092ms** | **~820ms** | 세션당 1회 |

| ID | 심각도 | 발견 (측정/추정) | 근거 | 처방 · 예상 효과 |
|----|--------|------|------|------|
| P-1 | **P1** | [측정] `hook-metrics.jsonl` **15.7MB/186k줄** 무한 성장 → session-start가 매번 `PruneHookMetricsLog(720h)`로 전량 unmarshal+재작성. session-start ~900ms의 지배 요인. 활성 사용 중엔 30일 기준으로 거의 안 지워져 단조 악화 | `hookcatalog/catalog.go:77`, `hookmetrics/metrics.go:149-220` | 나이 기준 prune → **줄수/바이트 상한 rotate**. session-start 900ms → <100ms |
| P-2 | **P1** | [측정] 중단된 prune의 고아 `.hook-metrics.jsonl-*.tmp` **22MB 누수** — hook 타임아웃/SIGKILL 시 defer 미실행, 청소 코드 없음 | `metrics.go:176` | 시작 시 stale `.tmp` 스윕 (P-1과 연동) |
| P-3 | **P1** | [측정] `doc-upkeep-queue.jsonl` **6.15MB** 무한 성장(compact 0건) + user-prompt/post-tool-use/stop 3개 훅이 매번 전량 스캔 — resolved 이벤트 영구 재파싱. 활성 레포에서 per-turn 수백ms 선형 증가 | `store.go:58,78-99`, `hook_prompt.go:78`, `hook_stop.go:35` | resolved compaction(pending만 rewrite). 활성 프로젝트 per-turn 수백ms 절감 |
| P-4 | P2 | [측정] metrics·doc-upkeep append가 flock 없음, prune도 lock 없는 read→rewrite → 다중 세션 동시 실행 시 이벤트 유실(정합성) | `metrics.go:85,149`, `store.go:58` | hookfailure와 동일한 `WithKeyLock` 적용 (`hookfailure/log.go:71` 패턴) |
| P-5 | P3 | [측정] post-tool-use가 relay 부재 시에도 매 tool 호출마다 project.json read + relay `os.Remove` | `hook_lifecycle.go:32`, `lifecycle_project_state_store.go:35` | relay 파일 `os.Stat` 선확인 후 skip |
| P-6 | P3 | [측정/추정] cold-start ~30ms×매 hook — 패키지 스코프 `regexp.MustCompile` 56개 전량 선컴파일, 바이너리 17.3MB. 200-tool 대화 기준 회피 가능분 ~10s | `--help` 78ms vs no-op 48ms | hot-path regexp 지연 컴파일, `-ldflags="-s -w"`. hook당 10–20ms (P-1~3보다 후순위) |
| P-7 | P3 | [측정] 테스트 스위트 ~40s는 sleep이 아니라 temp-dir FS I/O 지배(lifecycle: sys 1.28s≫user 0.55s, `t.TempDir()` 94회). 개발 속도 문제 | lifecycle uncached 실측 | 공유 fixture/`t.Parallel()`. 느린 패키지 40–60% 단축 |
| P-8 | — | [확인] daemon proxy 요청 경로 최적(재마샬 0회). 유일 낭비는 시작 시 probe dial 1회 중복 — 프로세스당 1회라 미미 | `daemon_proxy.go:30-64` | probe 생략 직행 dial (선택) |

참고: `webfetch/benchmark.go`·`issueops/benchmark/`는 성능 벤치마크가 아니라 **정확도 회귀 게이트**(품질 스코어링 하네스)임을 확인 — 지연시간 측정과 무관.

## 4. 실행 계획 (우선순위·의존성)

| 순번 | 항목 | 심각도 | 효과/노력 | 의존 | 검증 |
|------|------|--------|-----------|------|------|
| 0 | **T-1 golden 갱신** — `-update` 후 diff 검토·커밋 | P0-운영 | 高/小 | — (즉시) | `go test ./cmd/harness/... -count=1` green |
| 1 | **P-1+P-2 hook-metrics rotate + stale tmp 스윕** | P1 | 高/中 | — | session-start p50 재측정 <100ms, `.tmp` 0개 |
| 2 | **P-3 doc-upkeep compaction** | P1 | 高/中 | P-4와 같은 파일 — 함께 설계 | 큐 파일 크기 상한 유지, pending 정합성 테스트 |
| 3 | **A-1 remoteparse 이동** (adapter→cmd 역전 해소) | P1 | 高/小 | — | build + import grep 0건 |
| 4 | **T-2 CI `-race` 추가** | P1 | 高/小 | T-1 선행(게이트 green) | CI run green, wall time 허용 확인 |
| 5 | **Q-1 usage 실패 관측 기록** | P2 | 高/小 | — | 실패 경로 unit test에서 OK:false 레코드 확인 |
| 6 | **P-4 metrics/doc-upkeep flock** | P2 | 中/小 | 2와 동일 파일 | 동시 append/prune race 테스트 |
| 7 | **QC-SYNC 후보 카탈로그-감사 동기화** | P2 | 中/小 | — | `quality inspect` 후보에서 resolved 4건 제거 |
| 8 | **Q-2 lesson penalty 감쇠** (설계 의도 선확인) | P2 | 中/中 | 의도 확인 | penalty 윈도우 unit test |
| 9 | **T-3 커버리지 게이트 facade 제외 + T-4 실로직 테스트 3종** | P2 | 中/中 | — | `quality inspect` low-coverage 오탐 해소 |
| 10 | **T-5 mcp_sdk_server 정체 판정** | P2 | 中/小 | M1 결정 대조 | 삭제 or 통합 테스트 1개 |
| 11 | A-2 facade 우회 규약 명문화 / A-3 헬퍼 추출 / Q-3+P-5 prune·stat / Q-4 라벨·메시지 / T-6 ID 헬퍼 / P-6 cold-start / P-7 테스트 속도 | P3 | 低~中 | 틈새 | 각 항목 국소 테스트 |

병렬 가능: {0,3,5,7,10}은 상호 독립. {1,2,6}은 hookmetrics/doc-upkeep 파일을 공유하므로 순차 또는 단일 브랜치. 4는 0 이후.

## 5. 성공 판정 (evidence-bound)

- **운영**: CI 머지 게이트 green 복원(`go test ./... -count=1` + `-race` job) — 순번 0·4.
- **성능(측정 기준)**: session-start p50 900ms → **<100ms** · 고아 `.tmp` 0개 · doc-upkeep 큐 파일 크기 상한 유지 · 재측정은 perf lens와 동일한 방법(10회+ wall time, 유휴 상태)으로 before/after 기록.
- **품질**: externalllm usage 레코드에 실패 호출 포함(OK:false 존재 증명) · `quality inspect` 후보 목록에서 resolved 중복 0건 · low-coverage 오탐(facade) 해소 후 잔여 실로직 갭(gjc/webfetchcli/feedbackcleanup)에 테스트 추가.
- **아키텍처**: adapter→cmd import grep 0건 유지(CI grep 게이트 선택).
- **공통**: 각 항목 처방 반영 시 이 문서의 해당 행에 커밋 해시 기록(감사 문서 reconcile 규약 계승).

## 6. 비-목표

- 이미 FIXED P0~P2 재구현 · 멀티에이전트 실행 루프 · 전문기능 재발명(CodeGraph/LLM Wiki/claude-mem) — v2 §7 계승.
- 측정 근거 없는 마이크로 최적화 — 성능 계획 2026-07-02 결정 계승. P-6(cold-start)·P-8(probe dial)은 측정됐지만 절대값이 작아 P1~P2 완료 전 착수 금지.
- provider CreateChild/CloseChild 본문 공통 추상화 — gh/glab·REST/GraphQL 차이로 leaky (A-3 처방의 명시적 거부 항목).
- facade 분해 — A-4는 모니터링만.

## 부록: 분석 provenance

- 4-lens 병렬 서브에이전트: 아키텍처(opus, import 그래프+심볼 카운트), 코드품질(최근 churn 14파일 리뷰), 성능(실측: hook 10–15회 wall time + 상태 파일 실측), 테스트갭(coverprofile + race 실측 + FAIL 3회 재현).
- 교차 검증된 상호 확인: 성능 lens의 P-4(flock 부재)와 품질 lens의 상태 쓰기 견고성 평가는 **다른 파일**에 대한 것으로 모순 아님(issueops/state는 flock, hookmetrics/doc-upkeep은 미적용). 테스트 lens의 T-5(dead code 의심)는 아키텍처 lens의 "M1 의도적 유지" 결정과 대조 필요를 명기.
