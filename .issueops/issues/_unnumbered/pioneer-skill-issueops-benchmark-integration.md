# Pioneer Skill ↔ IssueOps 통합 기여도 측정 추가 플랜 (rev.5, 검증-적정성 리뷰 반영)

> rev.4: critic 2차 리뷰(T7/T8)를 검증 후 반영 — T7 map-vs-단일객체 decode 버그 정정, 3-단계 facade 보강, fail-closed 키검증, usage.go 동기화, T8 정직성 노트, remote-score 범위 제외.
> rev.5: verifier 검증-적정성 리뷰(verdict: request_changes)를 반영 — T1 N/A **양방향** 테스트(비대상 평균 수치불변 + 대상 evidence="" → MinimumScore==0), T2 AND-절별 음성+hollow 경계 케이스, T4 **CLI dispatch 경로 강제**(라이브러리 직호출 불충분)+비대상 evidence="" 단언, T7 누락/추가 키 fail-closed 음성 시나리오, F2·F4 **binary 명령화**, evidence 위생(gitignored 파일 대신 go test -v 출력).

## TL;DR
> **Summary**: issueops 품질 벤치마크에 ① "대상 fixture의 산출물이 pioneer 스킬(algorithm-optimization/database-design/debugging/code-quality-metrics)의 distinctive-method 시그니처를 담고 있는가"를 deterministic하게 채점하는 dimension·검출기·fixture를 추가하고, 기존 compare/gate로 시그니처 present/absent를 회귀-게이트한다. ② LLM judge 백엔드를 외부 `agy -p` shell-out에서 **fresh-context 서브에이전트가 채운 JSON 패킷**(`--judge file`)으로 전환하고 agy는 legacy로 강등한다.
> **Deliverables**: ① artifact 필드 `PioneerSkillEvidence` ② fixture 필드 `pioneer_skill_target`(데이터 기반 적용성) ③ dimension `pioneer_skill_contribution`(비대상 fixture는 정직한 N/A) ④ 4개 스킬 시그니처 검출기 ⑤ 4개 fixture ⑥ CLI `FromFixture` 배선 + 전체 fixture run 테스트 ⑦ present/absent 판별 A/B 테스트 ⑧ contract·judge-rubric·golden 정합 ⑨ **`--judge file` 백엔드(서브에이전트 판정 JSON 입력) + agy legacy 강등** ⑩ **issueops SKILL.md에 fresh-context 서브에이전트 judge 프로토콜 문서화**
> **Effort**: Medium-Large (judge 백엔드 흡수로 +2 task)
> **Parallel**: YES — 4 waves
> **Critical Path**: T1(스키마+dimension+N/A 평균로직) → T2(검출기) → T3(fixture) → T4(FromFixture 배선) → T5(A/B) → F-wave. judge 백엔드(T7,T8)는 pioneer 경로와 독립이라 병렬.

## Context

### Original Request
보고서(`.issueops/research/pioneer-skill-evaluation-and-issueops-integration-assessment.md`) 권고 1·2 실행. issueops 벤치마크(계층 B)를 pioneer-skill 쪽으로 확장.

### 적대적 리뷰 반영(rev.2)
critic 에이전트 검토(verdict: needs rework) 결과를 직접 검증 후 전량 수용. 핵심 수정 5건:
1. **[BLOCKER]** CLI facade `FromFixture`(`cmd/issueops/issueopscli/benchmarkartifact/issueops_benchmark_artifact.go:9`)가 누락됐었음 — `benchmarkcmd/benchmark.go:38`이 모든 fixture를 이걸로 변환해 production `benchmark run` 채점. 신규 dimension 추가 시 여기서 `PioneerSkillEvidence`를 안 채우면 전 fixture가 0점/critical로 떨어짐. → 신규 T4로 배선 + 전체-fixture CLI run 테스트 추가.
2. **[HIGH]** N/A를 fixture.ID 하드코딩 allowlist로 처리하면 비대상 fixture가 거짓 100점 → AverageScore 오염, rename에 취약. → fixture에 `pioneer_skill_target` 필드를 두어 **데이터 기반 적용성**으로 전환하고, 비대상은 평균에서 **제외**(정직한 N/A). score.go 평균 산출 로직 변경 포함.
3. **[MAJOR]** A/B 테스트와 Success Criteria가 "스킬 라우팅 기여를 증명"이라 **과대 주장**. 실제로는 손으로 쓴 evidence를 검출기가 매칭하는 동어반복. → "시그니처 present/absent를 판별한다"로 격하, "라이브 라우팅 측정 아님"을 명시.
4. **[MAJOR]** 검출기는 method 호출이 아니라 **키워드 존재**를 본다("prevents overfit" 과장). 정당한 산출물이 표현 차이로 거짓 실패 가능. → "필요조건 키워드 프록시"로 정직하게 규정 + 동의어 집합으로 거짓음성 완화.
5. **[MAJOR]** judge 프롬프트가 `Rubric: issueOpsBenchmarkDimensions`(judge_prompt.go:17)라 dimension 추가가 judge로 자동 유입 — "judge 경로 불변" 가드 암묵 위반. → judge 프롬프트에 dimension 1줄 정의 추가. F4 golden 추론을 dimension 배열이 아니라 **docs-index(.issueops/** 신규 파일)** 로 재조준.

### judge 백엔드 흡수(rev.3)
사용자 결정으로 LLM judge 백엔드 교체를 본 플랜에 흡수(T7,T8). 배경·근거:
- 현재 `--judge`는 `agy`(기본)|`none`만 받고(`benchmarkcmd/benchmark.go:26,48-64`), agy 경로는 fixture마다 외부 LLM을 shell-out(`RunIssueOpsAgyJudge` judge.go:19)한다. 외부 CLI 의존(설치·인증), 출력 불안정(judge_test.go 방어코드 대부분), 하네스 컨텍스트 미공유가 약점.
- **구조 제약**: Go CLI는 Claude 서브에이전트를 동기 spawn할 수 없다. 따라서 judge **산출**은 에이전트 계층(메인이 dispatch한 fresh-context 서브에이전트)이 맡고, Go 계층은 **패킷 생성 + 스키마 검증 + merge/compare/gate**만 유지한다. 점수 스키마(`IssueOpsBenchmarkScore`)는 이미 backend-neutral이라 재사용.
- **삽입점**: `--judge file --judge-file <path>`(+stdin) 백엔드 신설 — 서브에이전트가 쓴 판정 JSON을 읽어 기존 `decodeStrictIssueOpsBenchmarkScore`(judge.go:53)로 검증 후 `MergeIssueOpsBenchmarkScoreWithJudge`(run.go:69)로 병합. agy 경로는 보존하되 help/문서에서 legacy로 강등.
- **자기승인 금지(전역 CLAUDE.md)**: 메인 에이전트가 자기 워크플로 산출물을 judge하면 자기승인. judge 역할은 **fresh-context 서브에이전트**(상속 컨텍스트 없음, self-score 금지)로 한정 — pioneer scorecard rubric 88-124와 동일 패턴. 메인은 오케스트레이션·merge만. (사용자가 "메인 or 서브에이전트" 열어둔 것을 이 규칙 근거로 서브에이전트로 좁힘 — Defaults Applied에 명시.)
- **CI 결정성 불변**: deterministic check는 CI 하드게이트 유지, LLM judge(agy/서브에이전트 공통)는 원래도 에이전트-타임이라 회귀 아님.

### Gap Analysis

확정 사실(코드 정독, rev.2 검증 포함):
- 벤치마크는 라이브 호출이 아니라 **artifact 텍스트 블롭**을 키워드·구조 검사로 0/100 채점(`score.go:5,99-117`). "스킬이 불렸는가"는 산출물의 distinctive-method 흔적으로 **간접 측정**한다 — 이는 **충분조건이 아니라 필요조건 프록시**다(리뷰 반영, 과대 주장 금지).
- `Passed = no critical && no deterministic && MinimumScore>=100`(`score.go:117`). 신규 dimension이 비대상 fixture에서 0점이면 기존 5 fixture run/score 테스트가 깨짐. → 비대상은 평균/최소/판정에서 제외(T1).
- production 경로: `benchmark run` → `FromFixture`(전 fixture)(`benchmark.go:38`) → `RunIssueOpsBenchmark`. `FromFixture`는 `DomainContractEvidence` 등 전용 evidence 필드를 채우지만(artifact.go:126-129) `PioneerSkillEvidence`는 미설정 → T4 필수.
- judge rubric은 dimension 배열을 그대로 사용(`judge_prompt.go:17`).
- A/B/compare/gate는 이미 존재(`benchmark.go:73-110`, `gate_test.go`).
- dimension 배열은 golden(`cmd/issueops/testdata/response_contracts.golden.json`)에 **임베드되지 않음**(dimension명 grep 0건). golden 위험의 실제 원인은 docs-index가 `.issueops/**`(plans/operations 신규 문서)를 인덱싱하는 것 — `testdata/issueops/fixtures/`는 현재 미인덱스라 신규 fixture는 안전(리뷰 검증).

기존 측정 트랙과의 관계(리뷰 "What's Missing" 반영):
- `.issueops/operations/pioneer-skill-quality-rubric.md` + `pioneer-skill-rerun-fixtures.md`는 **격리 fresh-context 서브에이전트 + 5D rubric**으로 9개 스킬을 이미 채점(4.92/5). 본 플랜의 Go 키워드 검출기는 **다른 계층**이다: 전자는 "스킬 본문이 격리 상태에서 정확한가", 본 플랜은 "issueops 산출물이 스킬 시그니처를 담는가". **두 점수는 의미가 다르며 통합 점수로 합치지 않는다.** 이 구분을 dimension evidence 문자열과 본 플랜 Success Criteria에 명시한다(두 개의 발산하는 "pioneer 기여" 점수 위험 차단).

리스크/주의:
- 과적합·거짓음성: AND-of-substrings는 키워드 1→~3개로 바를 올릴 뿐 method 실행을 증명하지 못함 → 정직하게 표기(필요조건).
- `branch_worktree_gate_quality`(score.go:77)가 `feature/` 접두사를 요구하나 issueops SKILL.md 102행은 금지 — 모순. **본 플랜 범위 밖**(별도 이슈). 단 `FromFixture`(artifact_test.go:12)와 신규 fixture는 기존 검사 통과 위해 현행 `feature/` 형태 유지.

## Work Objectives

### Core Objective
issueops 벤치마크가 "대상 fixture의 산출물이 해당 pioneer 스킬의 distinctive method 시그니처를 담는가"를 deterministic·CI-안전·정직-라벨로 채점하고, 시그니처 present/absent를 기존 compare/gate로 회귀-게이트한다. (라이브 라우팅 측정이 아님을 명시.)

### Deliverables
1. `IssueOpsBenchmarkArtifact.PioneerSkillEvidence string`.
2. `IssueOpsBenchmarkFixture.PioneerSkillTarget string`(빈 값=비대상).
3. dimension `pioneer_skill_contribution` + score.go: 비대상 fixture는 이 dimension을 **평균/최소/판정에서 제외**(정직한 N/A, 거짓 100 금지).
4. 4개 스킬 시그니처 검출기(동의어 집합 포함, "필요조건 프록시"로 문서화).
5. 4개 fixture(`pioneer_skill_target` 설정).
6. **`FromFixture` 배선**: 대상 fixture에 시그니처 evidence 생성, 비대상엔 빈 값. + 전체 9-fixture CLI run에서 critical/deterministic 0건 테스트.
7. present/absent 판별 A/B 테스트(과대 주장 없는 문구).
8. judge 프롬프트 dimension 1줄 정의 + contract-test 확장 + golden 영향 확인/갱신.
9. **`--judge file` 백엔드**: 서브에이전트 판정 JSON 파일/stdin 입력 → 기존 decode·merge 재사용. agy 경로 보존하되 help/문서 legacy 강등. JSON 파일 fixture 테스트.
10. **issueops SKILL.md judge 프로토콜**: pr 단계 등에서 벤치마크 judge가 필요하면 메인이 fresh-context 서브에이전트를 dispatch해 rubric 패킷으로 채점 JSON을 받고 `--judge file`로 먹인다는 절차 문서화(자기승인 금지 준수).

### Definition of Done (검증 가능 조건)
- [ ] `go test ./internal/core/issueops/... -count=1` 통과.
- [ ] `go test ./cmd/issueops/issueopscli/... -count=1` 통과(신규 FromFixture·CLI run 테스트 포함).
- [ ] `go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1` 통과.
- [ ] `go build -o bin/issueops ./cmd/issueops` 성공.
- [ ] `go test ./... -count=1` 통과 · `git diff --check` 클린.
- [ ] `issueops benchmark run --fixtures testdata/issueops/fixtures`(테스트 내) 9 fixture 모두 critical=0, deterministic=0.
- [ ] A/B: 대상 fixture에서 시그니처-present는 `pioneer_skill_contribution`=100, absent는 0·평균하락·`Improved=true`. 비대상 fixture의 AverageScore는 신규 dimension에 영향받지 않음.
- [ ] `--judge file --judge-file <map.json>` 경로가 판정 JSON을 strict 검증·병합하고, noisy/unknown-field를 거부한다. agy 경로는 여전히 동작(legacy).
- [ ] issueops contract-test가 서브에이전트 judge·자기승인 금지 구절을 단언하며 통과한다.

### Must Have
- 검출기 = 복수 구성요소 AND + 동의어 집합. evidence 문자열에 "necessary keyword proxy, not live-routing proof" 명시.
- 비대상 fixture: 신규 dimension 평균 제외(정직 N/A).
- `FromFixture` 배선으로 production `benchmark run` 무회귀.

### Must NOT Have (guardrails)
- 라이브 스킬 호출·서브에이전트 실행 미도입.
- 5개 비대상 스킬 검출기 미작성(후속).
- `feature/` 모순 수정 안 함(별도 이슈).
- 새 compare/gate 타입 미추가.
- 키워드 검출 점수를 격리 rubric 4.92와 합산/혼동하지 않음.
- "스킬 라우팅 기여를 증명한다"는 문구 사용 금지(과대 주장).

## Verification Strategy
> ZERO HUMAN INTERVENTION — 모두 `go test`.
- Test decision: **TDD** — 검출기·N/A 평균로직·FromFixture를 테이블/통합 테스트로 RED 먼저.
- QA policy: task별 happy(시그니처 충족→통과) + failure(누락→정확한 dimension/critical 실패) + N/A(비대상→평균 불변).
- **Evidence(rev.5 — verifier 위생 수정)**: 검증 증거는 **`go test -run <Name> -v`의 출력 자체**다. `.issueops/evidence/`는 gitignored이고 비-hermetic golden 사고를 일으킨 전력이 있으므로, 증거를 그 경로의 파일로 남기지 않는다. 후대 리뷰어/CI는 테스트를 재실행해 확인한다(테스트가 hermetic하고 단언이 코드 안에 있으므로 재현 가능). 아래 개별 task의 "Evidence:" 줄은 모두 "해당 go test -v 출력"으로 읽는다.

## Execution Strategy

### Parallel Execution Waves
Wave 1: T1
Wave 2: T2 → (키워드 계약 확정 후) T3
Wave 3: T4(FromFixture, T2·T3 의존), T6(contract, T3 의존) 병렬
Wave 4: T5(A/B, T4 의존)
Judge Wave (T1–T6과 독립, 언제든 병렬): T7 → T8
Final: F1–F4

### Dependency Matrix

| Task | Depends On | Blocks | Parallel With |
|------|-----------|--------|---------------|
| T1 스키마+dimension+N/A평균 | — | T2,T3,T4,T5 | — |
| T2 검출기(동의어) | T1 | T3,T4 | — |
| T3 fixture(+target) | T1,T2 | T4,T6 | — |
| T4 FromFixture 배선+CLI run 테스트 | T1,T2,T3 | T5 | T6 |
| T5 A/B 판별 테스트 | T1,T2,T3,T4 | F | — |
| T6 contract+judge-rubric 정의 | T3 | F | T4 |
| T7 `--judge file` 백엔드 | — (독립) | T8,F | T1–T6 전부 |
| T8 issueops judge 프로토콜 문서 | T7 | F | T1–T6 |
| F1–F4 | T4,T5,T6,T7,T8 | — | — |

> judge 백엔드(T7,T8)는 pioneer 검출 경로(T1–T6)와 코드상 독립이므로 별도 Wave로 병렬 실행 가능.

## TODOs

- [ ] 1. artifact·fixture 필드 + dimension + N/A 평균 제외 로직

  **What to do**:
  - `issueops_benchmark.go`: `IssueOpsBenchmarkArtifact`에 `PioneerSkillEvidence string \`json:"pioneer_skill_evidence,omitempty"\``; `IssueOpsBenchmarkFixture`에 `PioneerSkillTarget string \`json:"pioneer_skill_target,omitempty"\``; `issueOpsBenchmarkDimensions` 끝에 `"pioneer_skill_contribution"`.
  - `score.go`: `checks` 맵에 `pioneer_skill_contribution` 엔트리(ok=`issueOpsPioneerSkillEvidenceComplete(fixture,artifact)` — T2 구현, 이 task는 스텁 false). **핵심**: 비대상 fixture(`fixture.PioneerSkillTarget==""`)는 이 dimension을 `DimensionScores`에 N/A(evidence="N/A: not a pioneer-targeted fixture")로 기록하되 **AverageScore/MinimumScore/Passed 계산에서 제외**. `summarizeIssueOpsDimensionScores`(또는 호출부)에 N/A skip 인자/플래그 추가. 비대상에선 신규 dimension이 deterministic-failure를 만들지 않아야 함.
  - 기존 테스트 헬퍼 `completeBenchmarkArtifactForTest`(`issueops_benchmark_fixtures_test.go:17`)에 `PioneerSkillEvidence` 채움 — **무조건**(리뷰: T1에서 조건부였으나 `gate_test.go:28`·`fixtures_test.go:12`가 Passed/MinimumScore≥100을 단언하므로 반드시 깨짐).
  **Must NOT do**: 기존 dimension 순서·이름 변경. 비대상 fixture를 거짓 100점 처리(N/A는 평균 제외이지 100 아님).

  **Recommended Agent**: deep — 스코어러 평균 로직 변경은 회귀 민감.

  **Parallelization**: Wave 1 | Blocks: T2,T3,T4,T5 | Blocked By: —

  **References**:
  - `issueops_benchmark.go:17-37,112-130` — struct·dimension.
  - `issueops_benchmark_score.go:96-119` — checks 루프·summarize 호출·Passed 조건(N/A skip 삽입 지점).
  - `issueops_benchmark_summary.go` — `summarizeIssueOpsDimensionScores` 정의(평균/최소 산출, N/A skip 추가 대상).
  - `issueops_benchmark_fixtures_test.go:12,17` / `issueops_benchmark_gate_test.go:28` — 깨질 단언.

  **Acceptance Criteria**:
  - [ ] `go test ./internal/core/issueops/benchmark/ -run 'Score|RunAndCompare' -count=1` 통과(헬퍼 갱신 후).
  - [ ] 비대상 fixture 1개로 score → 신규 dimension N/A, AverageScore가 기존(17 dim 시절)과 동일, Passed 영향 없음.

  **QA Scenarios** (rev.5 — verifier: N/A는 **양방향** 검증 필수. 단방향이면 전체 제외=silent no-op 가능):
  ```
  Scenario: 비대상 N/A — 평균 수치 불변(방향 A)
    Channel: bash (go test 테이블)
    Steps: PioneerSkillTarget="" 완전-아티팩트를 score. 신규 dimension 추가 전 AverageScore 값을 상수로 캡처해 비교.
    Expected: pioneer_skill_contribution이 DimensionScores에 N/A 엔트리로 존재 AND AverageScore == <17-dim 시절 정확값(상수 단언)> AND Passed=true. (단지 "Passed"가 아니라 수치 동일성 단언.)
    Evidence: go test -run 'Score' -v 출력
  Scenario: 대상 fixture, evidence="" → dimension이 실제로 참여(방향 B, 핵심)
    Channel: bash (go test 테이블)
    Steps: PioneerSkillTarget="algorithm-optimization", PioneerSkillEvidence="" 로 score
    Expected: MinimumScore == 0 AND DeterministicFailures에 "...pioneer..." 포함 AND Passed=false. (N/A 제외가 대상 fixture엔 적용되지 않음을 증명 — 없으면 전체 no-op을 못 잡음.)
    Evidence: go test -run 'Score' -v 출력
  Scenario: 헬퍼 미갱신 RED
    Channel: bash
    Steps: completeBenchmarkArtifactForTest 갱신 전 대상 fixture run
    Expected: MinimumScore<100 FAIL(RED 확인 후 헬퍼 갱신)
    Evidence: go test -v 출력(실패 캡처)
  ```
  **Commit**: YES | `feat(issueops-benchmark): add pioneer dimension with data-driven N/A exclusion` | Files: `issueops_benchmark.go`, `issueops_benchmark_score.go`, `issueops_benchmark_summary.go`, `*_test.go`

- [ ] 2. 4개 스킬 시그니처 검출기(동의어 집합, 필요조건 프록시)

  **What to do**:
  - 신규 `issueops_pioneer_checks.go`: `issueOpsPioneerSkillEvidenceComplete(fixture,artifact) bool` — `fixture.PioneerSkillTarget` 값으로 분기(`"algorithm-optimization"|"database-design"|"debugging"|"code-quality-metrics"`), 빈 값이면 호출되지 않음(T1이 N/A로 처리). 알 수 없는 target은 false.
  - 시그니처(복수 AND + 각 절에 **동의어 집합**으로 거짓음성 완화):
    - algorithm-optimization: 복잡도(`complexity`|`asymptotic`|`big-o`|`quadratic`|`linearithmic`) AND 스케일링(`scaling`|`n=`|`n →`|`100→1000`|`benchmark`) AND 전후(`before`|`after`|`baseline`).
    - database-design: 인덱스(`index`) AND 쓰기비용(`write penalty`|`write cost`|`insert cost`) AND 설계근거(`1nf`|`2nf`|`3nf`|`bcnf`|`normal`|`selectivity`|`row count`|`read:write`).
    - debugging: `reproduce` AND 원인(`root cause`|`isolate`|`hypothesis`) AND 검증(`verify`|`fix`|`regression`).
    - code-quality-metrics: `snr` AND 전(`before`|`baseline`) AND 후(`after`|`improved`) AND 지표(`entropy`|`redundancy`|`overhead`).
  - T1 스텁을 실호출로 교체. `issueops_quality_checks.go:38` switch에 `case strings.Contains(ruleText,"skips pioneer method") && !issueOpsPioneerSkillEvidenceComplete(fixture,artifact):` 추가.
  **Must NOT do**: 단일 키워드 통과. "prevents overfit" 표현(대신 "necessary keyword proxy"). 5개 비대상 스킬 함수.

  **Recommended Agent**: deep.

  **Parallelization**: Wave 2 | Blocks: T3,T4 | Blocked By: T1

  **References**:
  - `issueops_quality_checks.go:22-50` — 전용 검사 함수·switch 패턴(fixture+artifact 시그니처 이미 존재).
  - `issueops_text_match.go:14-36` — `containsAllFold`/`containsAnyFold`.
  - 시그니처 근거: issueops SKILL.md 50,123-125행.

  **Acceptance Criteria** (rev.5 — verifier: AND-절별 음성 + hollow 경계 명시):
  - [ ] `issueops_pioneer_checks_test.go`: 스킬별 (완전→true) + (**각 AND-절을 하나씩 제거→false**, 절 수만큼 음성 케이스 — OR 오구현 적발) + (동의어 변형→true).
  - [ ] **hollow 경계 케이스**: 필수 키워드를 모두 포함하되 무의미하게 나열한 artifact를 1개 테스트해 **현재 검출 한계를 문서화**한다(통과할 것 — 이는 검출기가 "필요조건 프록시"일 뿐 hollow-method를 막지 못함을 증명하는 의도적 케이스; rev.2의 과대주장 제거와 일관). 테스트 주석에 "by design: keyword proxy, not method proof" 명시.
  - [ ] `go test ./internal/core/issueops/benchmark/ -run Pioneer -count=1` 통과.

  **QA Scenarios**:
  ```
  Scenario: 동의어 변형 통과(거짓음성 방지)
    Channel: bash
    Steps: "asymptotic ... quadratic→linear ... N=100→10000 ... baseline vs after" algorithm-optimization evidence
    Expected: algorithm-optimization 시그니처 true(키워드 'complexity'/'scaling' 없이도)
    Evidence: .issueops/evidence/task-2-pioneer-benchmark.txt
  Scenario: 1요소 누락 실패
    Channel: bash
    Steps: 전후 표현 뺀 algorithm-optimization evidence
    Expected: false, "skips pioneer method" critical 트리거
    Evidence: .issueops/evidence/task-2-pioneer-benchmark-error.txt
  ```
  **Commit**: YES | `feat(issueops-benchmark): detect pioneer method signatures (necessary keyword proxy)` | Files: `issueops_pioneer_checks.go`, `issueops_quality_checks.go`, `issueops_benchmark_score.go`, `*_test.go`

- [ ] 3. 4개 fixture(+ pioneer_skill_target)

  **What to do**: `testdata/issueops/fixtures/pioneer-{algorithm-optimization,database-design,debugging,code-quality-metrics}.json`. 각 fixture에 `"pioneer_skill_target":"<skill>"`, 스킬을 부를 user_prompt/repo_context, `critical_failures`에 `"skips pioneer method"` + 기존 필수 critical(Korean, guideline, worker context 등) 포함, `expected_*` 기술.
  **Must NOT do**: 기존 5 fixture 수정. `feature/` 모순 회피(기존 검사 통과 형태).

  **Recommended Agent**: quick.

  **Parallelization**: Wave 2(후행) | Blocks: T4,T6 | Blocked By: T1,T2

  **References**: `testdata/issueops/fixtures/ambiguous-intent.json`; `issueops_benchmark_fixtures.go:47-63`(validate); `pioneer_skill_target`는 신규 optional 필드라 validate 무영향.

  **Acceptance Criteria**:
  - [ ] `LoadIssueOpsBenchmarkFixtures(...)`가 9개 로드, target 4개 set·5개 빈값.

  **QA Scenarios**:
  ```
  Scenario: 로드·target 파싱
    Channel: bash
    Steps: 로드 후 pioneer-algorithm-optimization.PioneerSkillTarget=="algorithm-optimization" 단언 테스트
    Expected: fixture_count=9, target 정확
    Evidence: .issueops/evidence/task-3-pioneer-benchmark.txt
  Scenario: 필수 필드 누락 거부
    Channel: bash
    Steps: critical_failures 빈 임시 fixture validate
    Expected: "critical_failures is required"
    Evidence: .issueops/evidence/task-3-pioneer-benchmark-error.txt
  ```
  **Commit**: YES | `test(issueops-benchmark): add pioneer-targeted fixtures` | Files: `testdata/issueops/fixtures/pioneer-*.json`

- [ ] 4. [BLOCKER 해소] FromFixture 배선 + 전체-fixture CLI run 테스트

  **What to do**:
  - `cmd/issueops/issueopscli/benchmarkartifact/issueops_benchmark_artifact.go`(`FromFixture`, 9행): 반환 struct(126-129행 근처)에 `PioneerSkillEvidence` 설정 — `fixture.PioneerSkillTarget`에 따라 해당 시그니처를 충족하는 evidence 텍스트 생성(`pioneerEvidenceFor(target)` 헬퍼); 빈 target이면 `""`.
  - 신규/기존 CLI 테스트(`issueops_benchmark_cli_test.go` 또는 `benchmarkartifact/artifact_test.go`)에: 전체 `testdata/issueops/fixtures`를 `FromFixture`→`RunIssueOpsBenchmark`로 돌려 **9 fixture 모두 critical=0, deterministic=0**, 대상 4개의 `pioneer_skill_contribution`=100, 비대상 5개는 N/A 단언.
  **Must NOT do**: 기존 fixture의 다른 필드 변경. 비대상에 가짜 evidence 주입.

  **Recommended Agent**: deep — production 채점 경로.

  **Parallelization**: Wave 3 | Blocks: T5 | Blocked By: T1,T2,T3

  **References**:
  - `cmd/issueops/issueopscli/benchmarkartifact/issueops_benchmark_artifact.go:9,126-129` — 배선 지점.
  - `cmd/issueops/issueopscli/benchmarkcmd/benchmark.go:38` — 호출부(전 fixture).
  - `cmd/issueops/issueopscli/benchmarkartifact/artifact_test.go:11` — 기존 FromFixture 테스트 형태.

  **Acceptance Criteria** (rev.5 — verifier: 라이브러리 직호출이 아닌 **CLI dispatch** 경로 강제 + 비대상 evidence 빈값 단언):
  - [ ] 테스트는 **`runIssueOps([]string{"benchmark","run","--fixtures",<path>,"--judge","none"})`** (CLI dispatch, `benchmarkcmd/benchmark.go:38`의 `FromFixture` 경로 실행)를 호출해야 한다 — `RunIssueOpsBenchmark` 라이브러리 직호출만으로는 blocker 경로 미검증이라 불충분.
  - [ ] 반환 결과에서 `CriticalFailureCount==0` AND 모든 fixture의 `DeterministicFailures` 비어 있음 AND 대상 4개 `pioneer_skill_contribution`=100 AND 비대상 5개 N/A.
  - [ ] `FromFixture`가 **비대상 fixture에 `PioneerSkillEvidence==""`**(가짜 evidence 주입 금지)를 반환함을 단언.
  - [ ] `go test ./cmd/issueops/issueopscli/... -count=1` 통과.

  **QA Scenarios**:
  ```
  Scenario: production 경로 무회귀(핵심)
    Channel: bash
    Steps: 전체 fixtures를 FromFixture→RunIssueOpsBenchmark
    Expected: CriticalFailureCount=0, 대상 pioneer dim=100, 비대상 평균 영향 없음
    Evidence: .issueops/evidence/task-4-pioneer-benchmark.txt
  Scenario: 배선 누락 시 RED
    Channel: bash
    Steps: FromFixture에 PioneerSkillEvidence 미설정 상태로 실행
    Expected: 대상 fixture deterministic-failure 발생(RED 확인 후 배선)
    Evidence: .issueops/evidence/task-4-pioneer-benchmark-error.txt
  ```
  **Commit**: YES | `fix(issueops-benchmark): wire pioneer evidence through FromFixture CLI path` | Files: `issueops_benchmark_artifact.go`, `*_test.go`

- [ ] 5. present/absent 판별 A/B 테스트 (과대 주장 없음)

  **What to do**: `issueops_pioneer_ab_test.go`: 대상 4 fixture로 baseline(`PioneerSkillEvidence`="") vs candidate(완전 시그니처) run → `CompareIssueOpsBenchmarkRuns` `Improved=true`,`AverageScoreDelta>0`,`Regressions=[]`; `EvaluateIssueOpsAutoresearchGate(TargetDimensions:["pioneer_skill_contribution"])` keep. **테스트 doc 주석에 "이것은 검출기가 시그니처 present/absent를 판별함을 검증할 뿐, 라이브 스킬 라우팅을 측정하지 않는다" 명시.**
  **Must NOT do**: "라우팅 기여 증명" 문구. 새 compare/gate 타입.

  **Recommended Agent**: deep.

  **Parallelization**: Wave 4 | Blocks: F | Blocked By: T1,T2,T3,T4

  **References**: `issueops_benchmark_gate_test.go:8-90` — run/compare/gate 사용법; `issueops_benchmark.go:73-110`.

  **Acceptance Criteria**:
  - [ ] present>absent, Improved=true, Regressions=[]; 한 시그니처 제거 시 `TargetDimensionRegressions`에 dim·KeepCandidate=false.

  **QA Scenarios**:
  ```
  Scenario: present/absent 판별
    Channel: bash
    Steps: go test -run PioneerAB -count=1
    Expected: candidate 평균>baseline, Improved=true
    Evidence: .issueops/evidence/task-5-pioneer-benchmark.txt
  Scenario: 회귀 감지
    Channel: bash
    Steps: candidate에서 한 시그니처 제거 후 gate
    Expected: KeepCandidate=false, regression에 pioneer_skill_contribution
    Evidence: .issueops/evidence/task-5-pioneer-benchmark-error.txt
  ```
  **Commit**: YES | `test(issueops-benchmark): A/B-discriminate pioneer signature present vs absent` | Files: `issueops_pioneer_ab_test.go`

- [ ] 6. contract-test 확장 + judge-rubric dimension 정의

  **What to do**:
  - `issueops_skill_contract_test.go`: 4개 pioneer fixture 존재 + 각 `"skips pioneer method"` rule 포함 + `pioneer_skill_target` set 단언.
  - `issueops_benchmark_judge_prompt.go`: judge 프롬프트 instructions에 `pioneer_skill_contribution` 1줄 정의 추가("대상 fixture의 산출물이 지정 pioneer 스킬의 distinctive method 흔적을 담는지; 비대상은 N/A") — dimension 자동 유입(line 17)에 의미 부여(리뷰 minor 해소).
  **Must NOT do**: 기존 contract 약화. judge JSON 스키마 변경.

  **Recommended Agent**: quick.

  **Parallelization**: Wave 3 | Blocks: F | Blocked By: T3

  **References**: `issueops_skill_contract_test.go:12-59,113-129`; `issueops_benchmark_judge_prompt.go:17,24-34`.

  **Acceptance Criteria**:
  - [ ] `go test ./internal/core/issueops/ -run Contract -count=1` 통과. judge 프롬프트 빌더 테스트(`issueops_benchmark_judge_test.go`) 통과.

  **QA Scenarios**:
  ```
  Scenario: fixture 존재+rule 보증
    Channel: bash
    Steps: go test -run 'Contract|Pioneer' -count=1
    Expected: PASS
    Evidence: .issueops/evidence/task-6-pioneer-benchmark.txt
  Scenario: fixture 누락 실패
    Channel: bash
    Steps: 한 fixture rename 후 실행
    Expected: t.Fatalf 누락 명시
    Evidence: .issueops/evidence/task-6-pioneer-benchmark-error.txt
  ```
  **Commit**: YES | `test(issueops): assert pioneer fixtures + define judge rubric dimension` | Files: `issueops_skill_contract_test.go`, `issueops_benchmark_judge_prompt.go`

- [ ] 7. `--judge file` 백엔드 신설 (agy legacy 강등)

  **What to do** (rev.4 — critic가 decode shape 버그·facade 누락 지적, 검증 후 수정):
  - **decode shape (핵심 수정)**: 기존 `decodeStrictIssueOpsBenchmarkScore`(judge.go:53)는 **단일** `IssueOpsBenchmarkScore`를 디코드하며, strict 레이어(`structured.go:74-90`)가 `DisallowUnknownFields()` + trailing-data 거부를 한다. 따라서 `{fixtureID: score}` **map을 그대로 단일-score 디코더에 넣으면 모든 fixtureID 키가 unknown-field로 거부된다.** 올바른 방식:
    1. 단일-score용 exported 래퍼 `DecodeIssueOpsBenchmarkJudgeJSON([]byte)(IssueOpsBenchmarkScore,error)`를 `benchmark` 패키지에 추가(내부적으로 `decodeStrictIssueOpsBenchmarkScore` 호출).
    2. CLI(`benchmarkcmd`)에서 판정 파일을 먼저 `map[string]json.RawMessage`로 디코드(바깥 map), **각 값**을 `core.DecodeIssueOpsBenchmarkJudgeJSON`로 strict 디코드.
  - **3-단계 facade 재노출(누락 수정)**: 새 export는 (a) `benchmark`(실구현), (b) **`internal/core/issueops/issueops_benchmark_facade.go`**(중간 — 기존 41/53행이 `benchmark.*`를 래핑; 이 계층이 빠지면 컴파일 불가), (c) `internal/core/issueops_facade.go`(최상위, 267/279행 패턴) — **세 곳 모두** 추가.
  - **fail-closed 키 검증(신규 명시)**: 디코드한 map의 키 집합이 run의 fixture ID 집합과 **정확히 일치**해야 함. 누락/추가/불일치 키는 merge 전에 **에러**. (이유: `MergeIssueOpsBenchmarkScoreWithJudge`에 zero-value score를 넘기면 `run.go:87-89`이 "judge returned no dimension scores"를 JudgeFailures에 추가해 조용히 `Passed=false`가 됨 — 명시적 사전검증으로 silent degrade 차단.)
  - `benchmark.go`: `--judge` 설명 `"none|file|agy(legacy)"`, `--judge-file` 플래그(경로; 빈 값이면 stdin) 추가. `"agy"` dispatch 유지하되 help에 `(legacy: external agy -p; prefer file)` 표기.
  - **usage 텍스트 동기화**: `internal/adapter/cli/usage.go:85`(`benchmark run`)의 `[--judge none|agy]`를 `[--judge none|file|agy]`로 갱신.
  **Must NOT do**: agy 경로 제거(legacy 보존). judge JSON **스코어 스키마** 변경(바깥 map 래핑은 스키마 변경 아님). Go에서 서브에이전트 spawn 시도. `remote score`의 agy 경로 건드리기(아래 scope note 참조).

  **Recommended Agent**: deep — CLI dispatch + core export + facade.

  **Parallelization**: 독립 Wave | Blocks: T8,F | Blocked By: —

  **References**:
  - `benchmarkcmd/benchmark.go:16,26,48-64` — 플래그·dispatch·merge·finalize 패턴(agy 블록 옆에 file 블록). 최상위 usage `fmt.Println`(16행)은 건드리지 않음(CLI 테스트가 정확 문자열 단언, cli_test.go:53).
  - `issueops_benchmark_judge.go:53-61` — 재사용할 **단일-score** strict decoder.
  - `externalllm/structured.go:74-90` — `DisallowUnknownFields`+trailing-data 거부(map을 단일-score 디코더에 넣으면 실패하는 근거).
  - `issueops_benchmark_run.go:69-94` — `MergeIssueOpsBenchmarkScoreWithJudge`(zero-value score → silent Passed=false; 사전 키검증 필요 근거).
  - `internal/core/issueops/issueops_benchmark_facade.go:41,53` / `internal/core/issueops_facade.go:267,279` — 3-단계 재노출 패턴.
  - `internal/adapter/cli/usage.go:85` — 동기화할 usage 텍스트.

  **Acceptance Criteria**:
  - [ ] `issueops benchmark run --fixtures ... --judge file --judge-file <map.json>`가 바깥 map을 디코드→각 값 strict 디코드→fixture별 merge 결과를 낸다.
  - [ ] judge map 키가 fixture ID와 불일치(누락/추가)면 merge 전 에러; 각 score 값의 noisy/unknown-field는 strict 거부.
  - [ ] `core.DecodeIssueOpsBenchmarkJudgeJSON`이 3-단계 facade로 컴파일된다.
  - [ ] `go test ./cmd/issueops/issueopscli/... -count=1` 통과; 기존 `cli_test.go:53,59`(usage·bogus) 무회귀.

  **QA Scenarios**:
  ```
  Scenario: file 백엔드 병합
    Channel: bash
    Steps: 유효 판정 map JSON 작성 후 --judge file --judge-file 로 run
    Expected: 각 fixture score가 judge dimension 반영해 병합, exit 0
    Evidence: .issueops/evidence/task-7-pioneer-benchmark.txt
  Scenario: noisy/unknown-field 거부
    Channel: bash
    Steps: prose 섞인 또는 unexpected 필드 든 판정 score 값으로 run
    Expected: strict 디코드 에러로 실패(메시지에 근거 포함)
    Evidence: go test -v 출력
  Scenario: 누락 키 fail-closed (verifier 필수)
    Channel: bash
    Steps: run의 fixture ID 중 하나를 map에서 빼고 --judge file
    Expected: merge 전 **에러**(JudgeFailures로의 silent Passed=false 아님 — run.go:87-89 우회 차단)
    Evidence: go test -v 출력
  Scenario: 추가/불일치 키 fail-closed (verifier 필수)
    Channel: bash
    Steps: 로드되지 않은 fixture ID 키를 map에 추가해 --judge file
    Expected: merge 전 에러(미지 키 거부)
    Evidence: go test -v 출력
  ```
  **Commit**: YES | `feat(issueops-benchmark): add --judge file backend; demote agy to legacy` | Files: `benchmarkcmd/benchmark.go`, `benchmark/issueops_benchmark_judge.go`, `issueops/issueops_benchmark_facade.go`(중간 facade), `issueops_facade.go`, `internal/adapter/cli/usage.go`, `*_test.go`

- [ ] 8. issueops SKILL.md에 fresh-context 서브에이전트 judge 프로토콜 문서화

  **What to do** (rev.4 — critic honesty 노트 반영):
  - `skills/issueops/SKILL.md`(또는 `references/`): 벤치마크 LLM judge 절차 추가 — 메인은 (a) `--judge none` deterministic run, (b) **rubric+artifact 패킷**으로 fresh-context 서브에이전트(상속 컨텍스트 없음, self-score 금지)를 dispatch해 판정 JSON 수집, (c) `--judge file`로 병합. agy는 외부 LLM 폴백(legacy).
    - **패킷 입력 계약(서브에이전트 deterministic input)**: 서브에이전트에 주는 것 = ① rubric dimension 목록+각 1줄 정의, ② 채점할 artifact 필드, ③ 출력은 `{fixtureID: IssueOpsBenchmarkScore}` map JSON only(서문 금지). 이 형식을 SKILL.md에 명시(critic open-question 해소).
    - **자기승인 한계 명시**: "이 제약은 **문서화된 프로토콜이며 Go `--judge file` 레이어에서 강제되지 않는다**(백엔드는 바이트만 봄). 강제는 메인이 서브에이전트를 dispatch하는 오케스트레이션 규율에 의존" — 1줄 추가.
  - contract 단언은 **최소 1구절**만 추가하고, 테스트에 주석으로 "이 단언은 SKILL.md의 *문서 존재*를 검증할 뿐 런타임 행동을 보증하지 않는다(보고서의 계층-C 텍스트-존재 한계와 동일; 의도적 최소화)"를 명시 — rev.3 전반의 정직성과 일관.
  **Must NOT do**: 기존 Phase Assist Map/Always-On Rules 약화. Go judge 동작 변경. 텍스트-존재 단언 과다 추가(계층-C 함정).

  **Recommended Agent**: quick.

  **Parallelization**: 독립 Wave(후행) | Blocks: F | Blocked By: T7

  **References**:
  - `skills/issueops/SKILL.md:50-53`(pr/ai-slop-clean 단계, judge·reviewer 언급 위치), 99-129(Always-On Rules).
  - `internal/core/issueops/issueops_skill_contract_test.go:62-84` — 구절 단언 추가 지점.
  - 자기승인 근거: 전역 CLAUDE.md "Never self-approve... use code-reviewer/verifier".

  **Acceptance Criteria**:
  - [ ] `go test ./internal/core/issueops/ -run Contract -count=1` 통과(신규 단언 포함).

  **QA Scenarios**:
  ```
  Scenario: 프로토콜 구절·contract 단언
    Channel: bash
    Steps: SKILL.md에 서브에이전트 judge 절 추가 후 contract 테스트
    Expected: PASS, 신규 구절 단언 충족
    Evidence: .issueops/evidence/task-8-pioneer-benchmark.txt
  Scenario: 구절 누락 실패
    Channel: bash
    Steps: 구절 빼고 테스트
    Expected: t.Fatalf 누락 구절 명시
    Evidence: .issueops/evidence/task-8-pioneer-benchmark-error.txt
  ```
  **Commit**: YES | `docs(issueops): document fresh-context subagent judge protocol (no self-approval)` | Files: `skills/issueops/SKILL.md`, `issueops_skill_contract_test.go`

## Final Verification Wave (MANDATORY) — rev.5: 전부 binary·agent-executable 명령
- [ ] F1. Plan Compliance — T1–T8 명세대로(필드/dimension/N/A평균/검출기/fixture/FromFixture/A/B/judge정의/file백엔드/서브에이전트프로토콜 모두 존재). 체크: 각 task의 commit이 명시 Files를 포함하는지 diff로 확인.
- [ ] F2. Code Quality (binary):
  - `go vet ./internal/core/issueops/benchmark/... ./cmd/issueops/issueopscli/...` → exit 0
  - `grep -rn '"prevents overfit"' ./internal/core/issueops/benchmark/` → 0건(exit 1)
  - `grep -c 'containsAnyFold\|containsAllFold' internal/core/issueops/benchmark/issueops_pioneer_checks.go` → ≥4(복수-구성요소 검출 존재)
  - `grep -n 'target.*==.*"".*100\|== 100' internal/core/issueops/benchmark/issueops_benchmark_score.go` → 비대상 하드코딩 100 없음 확인
- [ ] F3. Real QA — **각 task QA를 `go test -run <Name> -v`로 실행하고 그 출력 자체가 evidence**(gitignored 파일 아님). happy+음성(누락/제거 절)+N/A 모두 PASS 라인 확인.
- [ ] F4. Scope/Golden (binary):
  - `go test ./cmd/issueops/issueopsapp ./cmd/issueops/contractgolden -run Golden -count=1` → exit 0. 흔들리면 원인은 dimension 배열이 아니라 `.issueops/**` 신규 문서의 docs-index 유입(`response_contracts.golden.json`은 `.issueops/plans|operations` 인덱싱; `testdata/issueops/fixtures` 미인덱스). 비-hermetic golden 얽히면 golden 갱신 **별도 커밋**.
  - `ls testdata/issueops/fixtures/pioneer-*.json | wc -l` → 정확히 4(5도 9도 아님)
  - `grep -rn 'von.neumann\|verified-execution\|prompt-engineering\|git-operations\|berners' internal/core/issueops/benchmark/issueops_pioneer_checks.go` → 0건(비대상 5스킬 미추가)
  - `branch_worktree_gate` 모순 미수정 유지(grep로 score.go:77 변경 없음 확인).
  - `go test ./... -count=1` && `go build -o bin/issueops ./cmd/issueops` && `git diff --check` → 전부 통과.

## Commit Strategy
태스크당 1 atomic 커밋(Conventional + Lore, COMMIT_POLICY.md). golden 갱신 별도 커밋. push는 사용자 지시 시에만.

## Success Criteria (과대 주장 제거)
- issueops 벤치마크가 4개 대상 fixture에 대해 "산출물이 스킬 distinctive 시그니처를 담는가"를 deterministic·정직-N/A로 채점한다.
- A/B가 시그니처 present/absent를 **판별**한다(라이브 라우팅 측정이 아님 — 보고서의 "미측정 ≠ 실패" 정직성 유지).
- production `benchmark run` 포함 전체 테스트·빌드·golden 그린, 기존 5 fixture·dimension 무회귀.
- 이 키워드-프록시 점수는 격리 rubric(4.92)과 **별개 계층**으로 명시되어 두 점수가 혼동되지 않는다.

## Defaults Applied / 남은 설계 결정
- 전용 필드 `PioneerSkillEvidence` + fixture `PioneerSkillTarget`(데이터 기반 적용성) 채택.
- **[사용자 확인 권장]** N/A 처리 방식: 본 rev.2는 "비대상 fixture에서 신규 dimension을 평균/판정에서 **제외**"(score.go 평균 로직 변경)를 채택. 대안은 "N/A를 100으로 두되 evidence에 N/A 명시"(score.go 무변경, 단 AverageScore 약간 상향 오염). rev.2 권장안은 정직성↑·범위 약간↑.
- **judge 주체 = fresh-context 서브에이전트**(메인 아님): 사용자가 "메인 or 서브에이전트"를 열어두었으나, 전역 CLAUDE.md 자기승인 금지 + scorecard rubric 88-124 패턴에 근거해 **서브에이전트**로 확정. 메인은 패킷 준비·dispatch·merge 오케스트레이션만. (메인이 직접 채점을 원하면 별도 리뷰 lane으로 분리 필요 — 본 플랜은 서브에이전트 기본.)
- **judge 입력 메커니즘 = 단일 map 파일**(`{fixtureID: score}`): 대안 stdin/fixture별 파일. 서브에이전트 1회 반환에 가장 적합한 map 파일 채택.
- **agy 보존(legacy)**: 제거 대신 외부 LLM 폴백으로 유지 — 가역적이고 기존 테스트 보존.

## Out of Scope (후속 별도 이슈)
- implementation-planning/verified-execution/prompt-engineering/git-operations/web-research deterministic 시그니처(프로세스적 → LLM-judge 보강 필요).
- `branch_worktree_gate_quality`의 `feature/` vs 이슈번호-접두사 모순.
- 보고서 권고 3·4·5(scorecard 단서, n≥3 분산 표준화, firsthand 정례화).
- 격리 rubric 트랙과 본 키워드 트랙의 장기 통합/대시보드(지금은 "별개 계층"으로 분리만).
- **`issueops remote score`의 agy 경로**(`cmd/issueops/issueopscli/remotecmd/remote.go:40,58`, `RunIssueOpsRemoteAgyJudge`)는 벤치마크 judge와 별개 명령이라 본 플랜 범위 밖 — **의도적으로 agy-only 유지**. "fragile agy 제거" 논리가 거기엔 미적용임을 명시(critic 지적). 필요 시 후속 이슈에서 동일 file 백엔드 패턴 적용.
