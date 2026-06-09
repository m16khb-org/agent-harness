# Von Neumann Quality Upgrade

## TL;DR
> **Summary**: Prometheus(ulw-plan)의 8가지 구조적 우위를 Von Neumann에 주입하되, "simplicity first" 원칙을 유지하며 핵심 품질 향상 6가지만 적용한다. 전체 재작성이 아닌 증분 개선으로, 각 변경은 기존 계약을 깨지 않는다.
> **Deliverables**: Intent Classification 세분화, Anti-Duplication 규칙, QA Scenario Anti-Pattern 가드, Agent Profile 추천, Turn Termination 강제, Draft Cleanup
> **Effort**: Medium
> **Parallel**: YES — 2 waves
> **Critical Path**: Task 1 → Task 2,3,4 (병렬) → Task 5 → Task 6 → Final Verification

## Context
### Original Request
"Von Neumann가 ulw-plan(Prometheus)보다 모자란 부분을 개선해서 더 좋은 품질을 내도록 계획을 수립해줘"

### Gap Analysis (Von Neumann vs Prometheus)
Prometheus(66KB)가 Von Neumann(~12KB)보다 가진 구조적 우위:

| # | Gap | 심각도 | 적용 판단 |
|---|-----|--------|-----------|
| 1 | **Intent Classification 8종** (Trivial/Refactoring/Build/Collaborative/Architecture/Research/Spec-Driven) vs Von Neumann 3단계 | **High** — Refactoring/Build/Research 다른 전략 필요 | **적용**: 5종으로 확장 (불필요한 세분화 제외) |
| 2 | **Anti-Duplication** — subagent에 위임한 탐색을 재수행 금지 | **High** — 토큰 낭비 방지 | **적용** |
| 3 | **QA Scenario Anti-Patterns** — "verify it works" "check API" 같은 모호한 시나리오 금지 예시 | **High** — 구현자가 잘못된 QA 작성 방지 | **적용** |
| 4 | **Agent Profile 추천** — Task별 category(visual-engineering 등)와 skills 추천 | **Medium** — 구현자가 올바른 worker 선택 | **적용** (간소화 버전) |
| 5 | **Turn Termination 강제** — "질문 혹은 완료"로만 턴 종료 가능 | **Medium** — 면담 품질 | **적용** |
| 6 | **Draft Cleanup** — Plan 완료 후 draft 삭제 | **Medium** — 상태 정리 | **적용** |
| 7 | **Metis 자동 Gap Analysis** (별도 agent) | Low — Von Neumann는 self-review로 충분 | **미적용** (복잡도 증가, self-review 유지) |
| 8 | **Momus Adversarial Review Loop** | Low — Turing Final Quality Gate가 커버 | **미적용** (Turing에 위임) |
| 9 | **Spec-Driven Detection** (OpenSpec/Spec Kit) | Low — agent-harness 생태계에 부재 | **미적용** (미래 검토) |
| 10 | **Task Label Format 강제** (bare numbers) | Low — 현재 템플릿으로 충분 | **미적용** |

### 핵심 원칙
- **Simplicity first**: Von Neumann는 12KB로 유지되어야 한다. 66KB로 불리는 것은 목표가 아니다.
- **증분 변경**: 각 변경은 기존 Clearance Checklist, Plan Template, IssueOps 통합을 깨지 않는다.
- **측정 가능한 품질**: 각 변경 후 QA 시나리오로 검증.

## Work Objectives
### Core Objective
Von Neumann 스킬이 Prometheus 수준의 계획 품질을 내면서도 agent-harness의 간결함을 유지하도록 6가지 증분 개선을 적용한다.

### Deliverables
- `skills/von-neumann/SKILL.md` — 6가지 개선이 반영된 최종 스킬 파일
- 각 변경에 대한 characterization test (변경 전후 행동 검증)

### Definition of Done
- [ ] Intent Classification: 5종(Trivial/Standard/Refactoring/Architecture/Research) + 각각 전략
- [ ] Anti-Duplication: subagent 위임 후 동일 탐색 금지 규칙 포함
- [ ] QA Anti-Patterns: 5개 이상의 구체적 금지 예시
- [ ] Agent Profile: Task 템플릿에 추천 category/skills 섹션 추가
- [ ] Turn Termination: 면담 턴 종료 전 강제 체크리스트
- [ ] Draft Cleanup: plan 완료 후 draft 삭제 단계
- [ ] `go test ./... -count=1` 통과
- [ ] 기존 Von Neumann contract를 깨는 변경 없음

### Must Have
- 각 개선은 독립적으로 revert 가능해야 함
- 기존 Clearance Checklist(6항목) 유지
- 기존 Plan Template 구조 유지
- 기존 IssueOps 통합 유지

### Must NOT Have
- Metis/Momus 같은 추가 agent 도입
- 66KB로의肥大화
- 기존 plan template breaking change
- 새로운 외부 의존성
- 구현 코드 (.go 파일) 변경

## Verification Strategy
> ZERO HUMAN INTERVENTION — all verification is agent-executed.
- Test decision: Characterization tests first (변경 전 스킬 행동 측정), then targeted diff tests
- QA policy: 각 Task 완료 후 tmux 기반 QA 시나리오
- Evidence: `.agent-harness/evidence/von-neumann-upgrade/`

## Execution Strategy
### Parallel Execution Waves

Wave 1 (foundation — template structure):
├── Task 1: Intent Classification 확장 (3→5종)
├── Task 2: Anti-Duplication 규칙 추가
└── Task 3: Turn Termination 강제

Wave 2 (task quality — plan output):
├── Task 4: QA Scenario Anti-Patterns
├── Task 5: Agent Profile 추천
└── Task 6: Draft Cleanup

Wave FINAL: F1-F4 검증

### Dependency Matrix

| Task | Depends On | Blocks | Can Parallelize With |
|------|-----------|--------|---------------------|
| T1   | —         | —      | T2, T3              |
| T2   | —         | —      | T1, T3              |
| T3   | —         | —      | T1, T2              |
| T4   | —         | —      | T5, T6              |
| T5   | —         | —      | T4, T6              |
| T6   | —         | —      | T4, T5              |

## TODOs

- [ ] 1. Intent Classification 5종으로 확장

  **What to do**:
  - 현재 Phase 0의 3단계(Tier) 테이블을 5종 분류로 교체
  - 추가: **Refactoring** (기존 코드 변경; 안전·테스트 커버리지 초점), **Research** (목표는 있으나 경로 불명확; 병렬 probe 후 Exit Criteria)
  - 각 유형별 면담 전략을 2-3문장으로 명시 (Prometheus의 장황한 전략 대신 핵심만)
  - 기존 Trivial / Standard / Architecture는 유지

  **Must NOT do**:
  - Prometheus의 8종 전부를 복사하지 않는다 (Collaborative, Spec-Driven 등은 agent-harness 생태계에 불필요)
  - 각 전략 설명이 5문장을 넘지 않게 한다

  **Recommended Agent**: quick

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: none | Blocked By: none

  **References**:
  - 현재 코드: `skills/von-neumann/SKILL.md:71-79` — Phase 0: Classify Intent

  **Acceptance Criteria**:
  - [ ] Phase 0 테이블에 Trivial, Standard, Refactoring, Architecture, Research 5종 존재
  - [ ] 각 유형별 면담 전략 2-3문장
  - [ ] 기존 3종(Trivial/Standard/Architecture)의 전략이 변경되지 않음

  **QA Scenarios**:
  ```
  Scenario: Refactoring intent triggers safety-focused interview
    Channel: bash
    Steps: rg "Refactoring" skills/von-neumann/SKILL.md 후 전략 텍스트 확인
    Expected: "test coverage" 또는 "behavior preservation" 또는 "risk" 중 2개 이상 포함
    Evidence: .agent-harness/evidence/von-neumann-upgrade/task-1-intent.txt

  Scenario: Research intent triggers investigation strategy
    Channel: bash
    Steps: rg "Research" skills/von-neumann/SKILL.md
    Expected: "parallel" 또는 "probe" 또는 "exit criteria" 중 하나 이상 포함
    Evidence: .agent-harness/evidence/von-neumann-upgrade/task-1-research.txt
  ```

  **Commit**: YES | Message: `feat(von-neumann): expand intent classification to 5 types` | Files: `skills/von-neumann/SKILL.md`

- [ ] 2. Anti-Duplication 규칙 추가

  **What to do**:
  - Phase 1(Ground)에 "subagent에 위임한 탐색을 직접 재수행하지 않는다" 규칙 추가
  - FORBIDDEN: subagent가 탐색 중인 파일/패턴을 직접 grep/search
  - ALLOWED: non-overlapping 작업 지속
  - 구체적 위반 예시(Wrong) + 올바른 예시(Correct) 코드 블록 포함

  **Must NOT do**:
  - 도구 제한을 추가하지 않는다 (규칙적 가이드일 뿐, enforcement는 아님)

  **Recommended Agent**: quick

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: none | Blocked By: none

  **References**:
  - 현재 코드: `skills/von-neumann/SKILL.md:83-95` — Phase 1: Ground

  **Acceptance Criteria**:
  - [ ] Phase 1에 "Anti-Duplication" 섹션 존재
  - [ ] FORBIDDEN / ALLOWED 구분 명시
  - [ ] Wrong + Correct 예시 포함

  **QA Scenarios**:
  ```
  Scenario: Anti-duplication rule is explicit
    Channel: bash
    Steps: rg "Anti.Duplication|subagent.*재수행|FORBIDDEN.*explor" skills/von-neumann/SKILL.md
    Expected: 1개 이상 매치
    Evidence: .agent-harness/evidence/von-neumann-upgrade/task-2-anti-dup.txt
  ```

  **Commit**: YES | Message: `feat(von-neumann): add anti-duplication rule` | Files: `skills/von-neumann/SKILL.md`

- [ ] 3. Turn Termination 규칙 강화

  **What to do**:
  - Output Discipline 섹션 하단에 "Turn Termination Rules" 추가
  - 면담 턴 종료 전 강제 체크리스트: "□ 질문을 했는가? □ 다음 행동이 명확한가?"
  - 금지된 종료 패턴: "Let me know if you have questions", "When you're ready...", 질문 없는 요약만
  - ALL YES → 종료 가능, ANY NO → 계속 작업

  **Must NOT do**:
  - Clearance Checklist와 혼동되지 않도록 독립적 섹션 배치

  **Recommended Agent**: quick

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: none | Blocked By: none

  **References**:
  - 현재 코드: `skills/von-neumann/SKILL.md:35-42` — Output Discipline

  **Acceptance Criteria**:
  - [ ] Output Discipline 아래 "Turn Termination Rules" 존재
  - [ ] 2항목 체크리스트(질문 + 다음 행동 명확성)
  - [ ] 금지된 종료 패턴 3개 이상 명시

  **QA Scenarios**:
  ```
  Scenario: Turn termination rules present
    Channel: bash
    Steps: rg "Turn Termination|종료.*규칙|let me know" skills/von-neumann/SKILL.md
    Expected: 매치 1개 이상
    Evidence: .agent-harness/evidence/von-neumann-upgrade/task-3-termination.txt
  ```

  **Commit**: YES | Message: `feat(von-neumann): enforce turn termination checklist` | Files: `skills/von-neumann/SKILL.md`

- [ ] 4. QA Scenario Anti-Pattern 가드 추가

  **What to do**:
  - Plan Template의 QA Scenarios 섹션에 "Anti-patterns" 박스 추가
  - 금지 예시 5개: "Verify it works correctly", "Check the API returns data", "Test the component renders", "Should respond with...", "Looks correct"
  - 각 금지 패턴 옆에 WHY 설명 1문장 + 올바른 대체 예시
  - 최대 15줄 이내로 유지

  **Must NOT do**:
  - 기존 QA Scenario 템플릿 구조를 변경하지 않는다
  - 과도한 specificity 규칙을 추가하지 않는다

  **Recommended Agent**: quick

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: none | Blocked By: none

  **References**:
  - 현재 코드: `skills/von-neumann/SKILL.md:300-313` — QA Scenarios 템플릿

  **Acceptance Criteria**:
  - [ ] Plan Template QA 섹션에 "Anti-patterns" 존재
  - [ ] 금지 패턴 5개 + WHY 설명
  - [ ] 올바른 대체 예시 2개 이상

  **QA Scenarios**:
  ```
  Scenario: Anti-pattern examples are concrete
    Channel: bash
    Steps: rg "Verify it works|Check.*API.*returns|should respond with|looks correct" skills/von-neumann/SKILL.md
    Expected: 3개 이상 매치 (금지 예시로 명시됨)
    Evidence: .agent-harness/evidence/von-neumann-upgrade/task-4-anti-pattern.txt
  ```

  **Commit**: YES | Message: `feat(von-neumann): add QA scenario anti-pattern guard` | Files: `skills/von-neumann/SKILL.md`

- [ ] 5. Agent Profile 추천 추가

  **What to do**:
  - Task 템플릿("**What to do**" 앞)에 "**Recommended Agent**:" 라인 추가
  - Category: `quick` / `deep` / `visual-engineering` 중 선택 + 이유 1문장
  - 스킬 파일 서두에 3가지 category의 간략한 정의(각 1문장) 추가

  **Must NOT do**:
  - Prometheus의 복잡한 skills 추천 시스템 도입 금지
  - category를 3종 이상으로 늘리지 않는다

  **Recommended Agent**: quick

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: none | Blocked By: none

  **References**:
  - 현재 코드: `skills/von-neumann/SKILL.md:285-315` — Task template

  **Acceptance Criteria**:
  - [ ] Task 템플릿에 "**Recommended Agent**: [quick | deep | visual-engineering]" 필드 존재
  - [ ] 스킬 서두에 3 category 정의 (각 1문장)
  - [ ] 기존 필드 변경 없음

  **QA Scenarios**:
  ```
  Scenario: Agent recommendation field exists
    Channel: bash
    Steps: rg "Recommended Agent|quick.*deep.*visual" skills/von-neumann/SKILL.md
    Expected: 매치 1개 이상
    Evidence: .agent-harness/evidence/von-neumann-upgrade/task-5-agent.txt
  ```

  **Commit**: YES | Message: `feat(von-neumann): add agent profile recommendation` | Files: `skills/von-neumann/SKILL.md`

- [ ] 6. Draft Cleanup 단계 추가

  **What to do**:
  - Phase 3 Step 5(Offer Choice) 뒤에 "Step 6: Draft Cleanup" 추가
  - Plan 완료 후 `.agent-harness/drafts/<slug>.md` 삭제 지시
  - "Draft was working memory; plan is the single source of truth" 설명
  - `rm .agent-harness/drafts/<slug>.md` 명령어 예시 포함

  **Must NOT do**:
  - 자동화 코드 추가 금지 (절차적 규칙만)
  - plan 파일 조작 금지

  **Recommended Agent**: quick

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: none | Blocked By: none

  **References**:
  - 현재 코드: `skills/von-neumann/SKILL.md:222-227` — Step 5: Offer Choice

  **Acceptance Criteria**:
  - [ ] Phase 3에 "Step 6: Draft Cleanup" 존재
  - [ ] draft 삭제 이유 + `rm` 명령어 예시 포함

  **QA Scenarios**:
  ```
  Scenario: Draft cleanup step documented
    Channel: bash
    Steps: rg "Draft Cleanup|draft.*삭제|rm.*drafts" skills/von-neumann/SKILL.md
    Expected: 매치 1개 이상
    Evidence: .agent-harness/evidence/von-neumann-upgrade/task-6-cleanup.txt
  ```

  **Commit**: YES | Message: `feat(von-neumann): add draft cleanup after plan completion` | Files: `skills/von-neumann/SKILL.md`

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> ALL must APPROVE. Present consolidated results to the user and get explicit "okay" before completing.
- [ ] F1. Plan Compliance Audit — 6 tasks 모두 spec대로 구현되었는가?
- [ ] F2. Code Quality Review — SKILL.md에 AI slop, Dead rule, Contradiction 없는가?
- [ ] F3. Real Manual QA — 각 task QA 시나리오 통과 + 전체 스킬 20KB 미만 유지?
- [ ] F4. Scope Fidelity Check — 추가 agent 미도입, template breaking change 없음, 기존 contract 유지?

## Commit Strategy
- Task별 1커밋 (총 6커밋). Task 1-3은 Wave 1에서 순차 커밋, Task 4-6은 Wave 2에서 순차 커밋.

## Success Criteria
- `wc -c skills/von-neumann/SKILL.md` < 20000 (20KB 미만)
- 기존 Clearance Checklist(6항목) 변경 없음
- 기존 Plan Template 구조 유지
- 모든 개선 사항이 SKILL.md 내에서 grep으로 확인 가능
- `./bin/agent-harness docs --json` 에 von-neumann 정상 출력
