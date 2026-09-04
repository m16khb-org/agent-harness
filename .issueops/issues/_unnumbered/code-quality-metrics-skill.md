# Claude Shannon — Signal-to-Noise Quality Gate

## TL;DR
> **Summary**: 정보이론의 아버지 Claude Shannon의 개념(Signal-to-Noise Ratio, Entropy, Redundancy, Channel Capacity, Compression)을 코드 품질에 적용하는 **정량적 측정·게이트 스킬**을 설계한다. 기존 `ai-slop-clean`(정성적 휴리스틱)의 상호보완재로, PR 전 SNR 측정 → 목표 미달 시 개선 방향을 수치로 제시 → Turing Final Quality Gate에 통합.
> **Deliverables**: Shannon 스킬(SKILL.md + references), SNR/Entropy/Redundancy 측정 프로토콜, Turing 품질 게이트 연동
> **Effort**: Medium
> **Parallel**: YES — 2 waves
> **Critical Path**: Task 1 → Task 2 → Task 5 → Final Verification

## Context
### Original Request
"Claude Shannon 스킬을 정보이론·비트 개념에 맞게 최적화, AI slop 해결 등등 할 수 있게 만들어줘. oh-my-openagent 등에서 비슷한 스킬 조사해서 계획을 세워줘."

### Research Findings (oh-my-openagent + issueops + academic)

**oh-my-openagent `remove-ai-slops`:**
- 10개 slop 카테고리 (가장 포괄적): Obvious comments, Over-defensive code, Excessive complexity, Needless abstraction, Boundary violations, Dead code, Duplication, Performance equivalences, Missing tests, Oversized modules
- 6단계 프로세스: scope 결정 → regression test lock → cleanup plan → parallel batch agents → 5 quality gate → rework loop
- 250 LOC/file cap, batch size 5, 3회 실패 시 user escalation

**issueops `ai-slop-clean`:**
- 6-pass 집중 클린업: Diff reality check, Slop removal, Claim audit, Contract check, Minimality check, Fresh evidence
- Turing Final Quality Gate + adversarial reviewer (패턴 #2)

**핵심 Gap 발견:** 두 시스템 모두 **정성적 카테고리 기반**. SNR·엔트로피·중복도를 **정량적 수치**로 측정하는 도구는 없음. Shannon이 이 gap을 메운다.

### Gap Analysis (Shannon vs 기존 도구)

| 차원 | remove-ai-slops (omo) | ai-slop-clean (harness) | Shannon (신규) |
|------|----------------------|------------------------|----------------|
| 접근법 | 정성적: 10개 카테고리 휴리스틱 | 정성적: 6-pass 수동 검사 | **정량적**: SNR, 엔트로피, 중복도 수치 측정 |
| 출력 | "N개 slop 발견·제거" | "pass/fail per pass" | **"SNR: 0.72→0.58 하락. 파일 X의 중복도 84%"** |
| 회귀 감지 | 불가능 (정성적) | 불가능 (정성적) | **가능**: 이전 PR 대비 SNR 변화 추적 |
| 작동 시점 | PR 전 클린업 | 구현 후 cleanup phase | **PR 전 측정 + cleanup에 목표값 제공** |
| Turing 연동 | 없음 | Final Quality Gate | **Quality Gate에 SNR threshold 추가** |
| 자동화 | sub-agent 병렬 실행 | 메인 에이전트 직접 | **메인 에이전트가 측정 명령 실행 + 결과 해석** |

---

## 정보이론 → 코드 품질 매핑

| Shannon 개념 | 코드 품질 대응 | 측정 방법 |
|-------------|---------------|-----------|
| **Signal** | 동작 변경 시 테스트가 실패하는 라인 (비즈니스 로직, 알고리즘, 계약) | `git diff`에서 해당 라인 제거 → `go test` 실패 확인 |
| **Noise** | 제거해도 테스트가 깨지지 않는 라인 (재진술 주석, dead code, pass-through wrapper) | `git diff --stat` 대비 coverage 변화 없는 라인 |
| **SNR** | Signal / (Signal + Noise) per diff | `signal_lines / total_changed_lines` |
| **Entropy** | Cyclomatic complexity 분포 | McCabe > 10인 함수 개수 / 전체 함수 |
| **Redundancy** | AST 구조가 80%+ 유사한 코드 블록 쌍 | Jaccard similarity on token n-grams |
| **Channel Capacity** | 파일당·함수당 가독 가능한 LOC 상한 | 250 LOC/file, 50 LOC/func, 5 params/func |
| **Compression** | 동일 동작을 더 적은 라인으로 표현 가능 여부 | "Could same AC pass with fewer changed lines?" (minimality check) |
| **Error Correction** | 테스트 커버리지 — signal corruption 감지기 | `go test -cover` per package |

---

## Work Objectives

### Core Objective
Claude Shannon의 정보이론 개념을 코드 품질 측정에 적용하는 정량적 감사 스킬을 설계한다. 기존 정성적 slop removal 도구들과 상호보완적이며, SNR 측정 → 목표값 제시 → cleanup에 피드백 → Turing Quality Gate 통합의 파이프라인을 형성한다.

### Deliverables
- `skills/code-quality-metrics/SKILL.md` — Shannon 스킬 본체 (~400줄, 기존 스킬과 유사한 구조)
- `skills/code-quality-metrics/references/` — SNR 측정 프로토콜, 엔트로피 임계값, 중복도 계산 가이드
- Turing 스킬 연동: Final Quality Gate에 SNR metric 추가
- IssueOps 연동: `ai-slop-clean` phase 전 Shannon 측정 단계

### Definition of Done
- [ ] Shannon SKILL.md 작성 (4-phase 프로토콜 + 정보이론 매핑 테이블)
- [ ] SNR·Entropy·Redundancy 측정 방법이 메인 에이전트가 실행 가능한 명령어로 문서화됨
- [ ] 기존 스킬과 중복되지 않음 (측정·게이트 전용, slop removal은 ai-slop-clean에 위임)
- [ ] Turing Final Quality Gate에 SNR metric 연동
- [ ] IssueOps phase map에 Shannon 측정 phase 반영 (ai-slop-clean 직전)
- [ ] `go test ./... -count=1` 통과

### Must Have
- 측정 가능한 4가지 지표: SNR, Entropy Score, Redundancy Ratio, Channel Overhead
- Pre-PR gate: SNR ≥ 0.6, Entropy Score ≤ threshold
- 회귀 감지: 이전 측정값과 현재 측정값 비교
- 기존 ai-slop-clean / Turing Reviewer와 협력적 (측정 → 제시 → 검증 파이프라인)

### Must NOT Have
- 자체 slop removal 기능 (ai-slop-clean, omo remove-ai-slops와 중복)
- 복잡한 AST 분석 도구 의존성 (메인 에이전트가 기존 grep/diff/git/wc 도구로 측정 가능해야 함)
- 외부 바이너리 의존성 (omo comment-checker 같은 binary hook 금지)
- 구현 코드 (.go 파일) 변경

---

## Shannon Skill Architecture

### 4-Phase Protocol

```
Phase 0: BASELINE — 현재 상태 측정
  → git diff 기반 SNR 계산
  → 함수별 McCabe complexity 근사 (grep 기반)
  → 파일 크기 분포 + 250 LOC 초과 파일 리스트

Phase 1: REGRESSION CHECK — 이전 측정값과 비교
  → issueops state에서 이전 Shannon snapshot 로드
  → SNR 하락, Entropy 증가, Redundancy 증가 감지
  → 회귀 발견 시 ai-slop-clean에 우선순위 목표 제시

Phase 2: TARGET — cleanup 목표 설정
  → SNR 목표 (기본 0.6, 이전 값 기반)
  → Entropy ceiling (함수 복잡도 상한)
  → Channel overhead budget (파일 크기 제한)

Phase 3: GATE — cleanup 완료 후 재측정
  → Phase 0 재실행
  → 목표 대비 달성률 계산
  → Turing Final Quality Gate에 Shannon metrics 추가
```

### Turing Final Quality Gate 통합

```json
{
  "shannonAudit": {
    "snr": {"before": 0.58, "after": 0.74, "target": 0.60, "passed": true},
    "entropyScore": {"before": 12, "after": 3, "ceiling": 8, "passed": true},
    "redundancyRatio": {"before": 0.15, "after": 0.06, "target": 0.10, "passed": true},
    "channelOverhead": {"oversizedFiles": 3, "remaining": 0, "passed": true}
  }
}
```

### IssueOps Phase Map 통합

| Phase | Shannon 역할 |
|-------|-------------|
| `ai-slop-clean` **직전** | Shannon Phase 0-2: Baseline 측정 → 회귀 체크 → 목표 제시 |
| `ai-slop-clean` **완료 후** | Shannon Phase 3: Gate — 목표 달성 확인, Turing에 metrics 전달 |

---

## Execution Strategy

### Parallel Execution Waves

Wave 1 (스킬 본체):
├── Task 1: Shannon SKILL.md 작성 (4-phase protocol + 정보이론 매핑 + 측정 명령어)
└── Task 2: 참조 문서 작성 (SNR 계산 프로토콜 + entropy heuristic + channel capacity rule)

Wave 2 (통합):
├── Task 3: Turing Final Quality Gate에 Shannon metric 연동
├── Task 4: IssueOps ai-slop-clean 전후 Shannon phase 통합
└── Task 5: Response contracts golden 갱신 + go test 검증

## TODOs

- [ ] 1. Shannon SKILL.md 본체 작성

  **What to do**:
  - `<identity>`: Claude Shannon — "The fundamental problem of communication is that of reproducing at one point either exactly or approximately a message selected at another point." (1948). 이 통찰을 코드에 적용: PR의 signal(code change intent)이 noise(boilerplate, dead code, over-abstraction) 없이 전달되는가?
  - 4-phase protocol: Baseline → Regression Check → Target → Gate
  - 정보이론 매핑 테이블 (Signal/Noise/Entropy/Redundancy/Channel/Compression/Error Correction)
  - Phase별 실행 가능한 셸 명령어 (grep, diff, git, wc, go test 기반)
  - "You MEASURE, not CLEAN" — slop removal은 ai-slop-clean에 위임
  - 기존 스킬(Turing, Von Neumann)과 동일한 구조: `<identity>`, `<mission>`, Phases, Plan Template, Critical Rules, Stop Rules, IssueOps Integration

  **Must NOT do**:
  - 자체 slop removal 기능 구현 금지
  - 외부 도구 의존성 추가 금지
  - Von Neumann/Turing의 구조적 탬플릿을 깨는 변경 금지

  **Recommended Agent**: deep — 구조적 설계와 Shannon 개념의 정확한 적용 필요

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: Task 3, 4 | Blocked By: none

  **References**:
  - Shannon, C.E. "A Mathematical Theory of Communication." Bell System Technical Journal, 1948.
  - 기존 Turing SKILL.md (`skills/verified-execution/SKILL.md`) — 구조 탬플릿 참조
  - 기존 ai-slop-clean (`skills/issueops/references/ai-slop-clean.md`) — 중복 방지
  - omo remove-ai-slops 10 categories — Shannon이 보완하는 정성적 접근

  **Acceptance Criteria**:
  - [ ] SKILL.md 400줄 이내, 기존 스킬 구조 준수
  - [ ] 4-phase protocol 완전 명시
  - [ ] 7개 정보이론 개념 + 측정 방법 포함
  - [ ] "MEASURE, not CLEAN" 원칙 명시

  **QA Scenarios**:
  ```
  Scenario: SNR calculation command works
    Channel: bash
    Steps: git diff --stat | awk로 signal/noise 라인 계산; SNR 출력
    Expected: 0.0~1.0 범위 숫자 출력
    Evidence: .issueops/evidence/code-quality-metrics/task-1-snr.txt

  Scenario: Entropy heuristic produces complexity flag
    Channel: bash
    Steps: grep 기반 함수당 라인 수, 중첩 depth 추정; 50줄 초과 함수 플래그
    Expected: 초과 함수 리스트 출력
    Evidence: .issueops/evidence/code-quality-metrics/task-1-entropy.txt
  ```

  **Commit**: YES | Message: `feat(code-quality-metrics): add Shannon signal-to-noise quality gate skill` | Files: `skills/code-quality-metrics/SKILL.md`

- [ ] 2. 참조 문서 작성 (SNR protocol + Entropy heuristic + Channel capacity rules)

  **What to do**:
  - `skills/code-quality-metrics/references/snr-protocol.md`: diff 기반 SNR 계산 단계별 가이드
  - `skills/code-quality-metrics/references/entropy-heuristic.md`: cyclomatic complexity 근사법 (중첩 if/for/while count, params count)
  - `skills/code-quality-metrics/references/channel-capacity.md`: 파일·함수 크기 상한 규칙 + 측정 명령어

  **Must NOT do**:
  - 이론적 배경만 장황하게 쓰지 않는다 (실행 가능한 명령어가 주)

  **Recommended Agent**: quick — 문서 작성, 창의적 판단 적음

  **Parallelization**: Can Parallel: YES | Wave 1 (Task 1과 병렬) | Blocks: none | Blocked By: none

  **Acceptance Criteria**:
  - [ ] 3개 참조 문서 각각 실행 가능한 셸 명령어 포함
  - [ ] 각 문서 50줄 이내 (핵심만)
  - [ ] SNR protocol에 회귀 감지 절차 포함

  **QA Scenarios**:
  ```
  Scenario: SNR protocol commands are executable
    Channel: bash
    Steps: snr-protocol.md 내 각 명령어를 복사해 실행
    Expected: 각 명령어가 오류 없이 완료
    Evidence: .issueops/evidence/code-quality-metrics/task-2-protocol.txt
  ```

  **Commit**: YES | Message: `docs(code-quality-metrics): add SNR protocol, entropy heuristic, channel capacity references` | Files: `skills/code-quality-metrics/references/*.md`

- [ ] 3. Turing Final Quality Gate에 Shannon metric 연동

  **What to do**:
  - `skills/verified-execution/SKILL.md`의 Final Quality Gate 섹션에 Shannon audit 항목 추가
  - Quality gate record JSON에 `shannonAudit` 블록 추가
  - "Shannon 측정 통과 → SNR ≥ 0.6 + Entropy ≤ ceiling" 조건 명시

  **Must NOT do**:
  - 기존 Quality Gate 항목(aiSlopCleaner, verification, codeReview) 제거 금지
  - Turing의 main-agent-direct 원칙 위반 금지

  **Recommended Agent**: quick — JSON 템플릿 수정

  **Parallelization**: Can Parallel: YES | Wave 2 (Task 4와 병렬) | Blocks: none | Blocked By: Task 1

  **Acceptance Criteria**:
  - [ ] Turing Quality Gate에 Shannon metric 항목 존재
  - [ ] SNR/Entropy/Redundancy/ChannelOverhead 4개 지표 포함
  - [ ] 기존 항목 유지

  **QA Scenarios**:
  ```
  Scenario: Turing Quality Gate includes Shannon metrics
    Channel: bash
    Steps: rg "shannonAudit\|Shannon.*SNR\|entropyScore" skills/verified-execution/SKILL.md
    Expected: 1개 이상 매치
    Evidence: .issueops/evidence/code-quality-metrics/task-3-verified-execution-gate.txt
  ```

  **Commit**: YES | Message: `feat(verified-execution): integrate Shannon metrics into final quality gate` | Files: `skills/verified-execution/SKILL.md`

- [ ] 4. IssueOps Shannon phase 연동

  **What to do**:
  - `skills/issueops/SKILL.md`의 `ai-slop-clean` phase 설명 앞에 Shannon 측정 단계 추가
  - "Shannon으로 baseline 측정 → 목표 제시 → ai-slop-clean 실행 → Shannon으로 재측정"
  - IssueOps phase table에 Shannon 언급 추가 (별도 phase가 아니라 ai-slop-clean과 쌍으로)

  **Must NOT do**:
  - 새로운 IssueOps phase를 추가하지 않는다 (기존 9-phase 유지)

  **Recommended Agent**: quick — 설명 텍스트 추가

  **Parallelization**: Can Parallel: YES | Wave 2 (Task 3과 병렬) | Blocks: none | Blocked By: Task 1

  **Acceptance Criteria**:
  - [ ] `ai-slop-clean` phase에 Shannon baseline + gate 설명 추가
  - [ ] `.issueops/SUB_AGENT_PATTERNS.md` 12패턴 유지 (Shannon은 sub-agent 사용 안 함)

  **QA Scenarios**:
  ```
  Scenario: IssueOps ai-slop-clean mentions Shannon
    Channel: bash
    Steps: rg -A3 "ai-slop-clean" skills/issueops/SKILL.md | grep -i code-quality-metrics
    Expected: Shannon 언급 1개 이상
    Evidence: .issueops/evidence/code-quality-metrics/task-4-issueops.txt
  ```

  **Commit**: YES | Message: `feat(issueops): integrate Shannon measurement before ai-slop-clean` | Files: `skills/issueops/SKILL.md`

- [ ] 5. Response contracts golden 갱신 + 검증

  **What to do**:
  - `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1 -update`
  - `go test ./... -count=1` 전체 검증
  - Shannon 스킬이 docs index + skill manifest에 정상 노출되는지 확인

  **Must NOT do**:
  - 구현 코드 변경 금지 (이번 phase는 스킬 문서만)

  **Recommended Agent**: quick — golden update + test run

  **Parallelization**: Can Parallel: NO | Wave FINAL | Blocks: none | Blocked By: Task 1-4

  **Acceptance Criteria**:
  - [ ] Golden test PASS
  - [ ] `go test ./... -count=1` — 0 FAIL
  - [ ] `./bin/issueops docs --json`에 code-quality-metrics 포함

  **QA Scenarios**:
  ```
  Scenario: Full test suite passes
    Channel: bash
    Steps: go test ./... -count=1
    Expected: ALL ok, 0 FAIL
    Evidence: .issueops/evidence/code-quality-metrics/task-5-tests.txt
  ```

  **Commit**: YES | Message: `chore: update golden snapshots for Shannon skill` | Files: `cmd/issueops/testdata/*.golden.json`

## Final Verification Wave
- [ ] F1. Plan Compliance Audit — 5 tasks 완료, 각 spec 충족?
- [ ] F2. Code Quality Review — Shannon SKILL.md에 AI slop/중복/모순 없음?
- [ ] F3. Real Manual QA — 각 task QA 시나리오 통과?
- [ ] F4. Scope Fidelity Check — 자체 slop removal 미구현, 외부 의존성 없음, 기존 스킬 구조 유지?

## Commit Strategy
Task별 1커밋 (총 5커밋). Wave 1: Task 1+2 순차, Wave 2: Task 3+4 병렬 후 Task 5 마무리.

## Success Criteria
- Shannon 스킬이 정량적 측정 도구로서 기존 정성적 slop removal과 중복되지 않음
- 정보이론 개념(Signal, Noise, Entropy, Redundancy, Channel Capacity, Compression, Error Correction)이 코드 품질에 정확히 매핑됨
- Turing + IssueOps와 매끄럽게 통합됨
- 기존 issueops 구조·컨벤션·테스트를 깨지 않음
