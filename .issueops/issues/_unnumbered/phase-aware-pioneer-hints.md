# Phase-인지 pioneer 스킬 힌트: UserPromptSubmit hook이 활성 IssueOps phase의 기대 스킬을 주입

## TL;DR
> **Summary**: UserPromptSubmit hook이 활성 IssueOps cycle의 현재 phase를 읽어 해당 phase의 기대 pioneer 스킬을 compact secondary hint로 주입한다. 관측 전용(게이트 아님), fail-open, 기대 페어링 테이블은 단일 Go 소스로 신설해 routing fidelity 채점의 기본값으로도 재사용한다.
> **Deliverables**: `internal/core/issueops`의 phase→기대 스킬 canonical 테이블, hookprompt phase-인지 힌트 주입, `issueops routing score`의 `--expect` 생략 시 기본 기대값, 문서/ADR 동기화.
> **Effort**: Short (1일 내)
> **Parallel**: YES — 2 waves
> **Critical Path**: T1(테이블) → T2(hook 주입) → T4(검증)

## Context

### Original Request
사용자가 선택지 1을 승인: "phase-인지 pioneer 힌트 갭 개선 계획 수립 — UserPromptSubmit hook이 활성 IssueOps cycle의 현재 phase를 읽어 해당 phase의 기대 스킬을 secondary hint로 주입(관측 전용, 게이트 아님)".

### 배경 (갭 분석)
- 스킬 계층(`skills/issueops/SKILL.md:246-283`)에는 phase별 pioneer 스킬 매핑이 강하게 존재하고 design-review는 fail-closed 게이트로 기계 강제됨.
- hook 계층(`internal/core/hookprompt/rules.go`)은 **키워드 기반** secondary hint만 있음 — 사용자가 plan phase에서 계획 키워드 없이 말하면 implementation-planning 힌트가 뜨지 않는다.
- 상태 계층의 routing fidelity(`internal/core/issueops/issueops_routing.go`)는 기대 페어링을 CLI `--expect`로만 받는 opt-in 채점이라 canonical 기대 테이블이 코드에 없음.

### Gap Analysis
- **주입 지점 실재 확인**: `BuildUserPromptMCPHints`(hookprompt/hook_prompt.go:43-135)는 이미 `req.Repo` 조건 분기에서 lifecycle profile·pending upkeep·gitlab 힌트를 주입한다. 같은 분기에 phase 힌트를 추가하면 hook 계약(fast/deterministic, routing hints only)을 벗어나지 않는다.
- **phase 조회 경로 실재 확인**: `issueops/session.Read(store, repo)`(session.go:96)가 repo의 primary binding(CycleID)을 주고, hookprompt는 이미 lifecycle 의존(`dependencies.go:31`)이 있다. 추가 조회는 sqlite Get 2회(binding + record)로 ms 수준 — 기존 `ResolveProjectLifecycleState` per-turn 조회와 동급.
- **드리프트 리스크**: 기대 테이블을 SKILL.md와 Go 두 곳에 두면 어긋난다 → Go 테이블이 기계 소비(힌트+채점)의 단일 소스, SKILL.md는 서사 설명으로 역할 분리하고 테스트가 SKILL.md 표의 스킬 집합과 Go 테이블의 일치를 고정한다.
- **프롬프트 예산**: hook_prompt_test.go에 컨텍스트 길이 budget 테스트가 있음 — 힌트는 1줄 compact 형식으로 제한하고 budget 테스트를 갱신한다.
- **비게이트 원칙**: readiness/phase 전이에 이 힌트를 연결하지 않는다. 힌트 미준수는 어떤 것도 막지 않는다(관측·기록은 기존 `issueops_record_routing`이 담당).

## Work Objectives

### Core Objective
활성 IssueOps cycle이 있는 repo에서, 프롬프트 키워드와 무관하게 현재 phase의 기대 pioneer 스킬이 UserPromptSubmit 컨텍스트에 1줄로 뜨게 한다.

### Definition of Done (검증 명령 포함)
- bound cycle(phase=plan)에서 `printf '{"prompt":"이거 진행해줘","repo":"<repo>"}' | ./bin/issueops hook user-prompt` 출력에 `issueops-phase` 힌트 라인과 implementation-planning이 포함
- unbound repo/비 IssueOps repo에서는 힌트 없음(기존 출력 불변)
- `go test ./internal/core/hookprompt ./internal/core/issueops ./cmd/issueops/hookcli -count=1` 통과
- `go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1` 통과(force 시 갱신 검수)

### Must Have
- fail-open: binding/record 읽기 오류·missing·corrupt 시 힌트 생략, 에러를 표면화하지 않음
- 힌트는 `PrioritySecondary` 1줄, phase명 + 기대 스킬 나열 + routing 기록 안내
- 기대 테이블은 `done`/`cleanup` 등 스킬 없는 phase에 빈 목록 허용
- 테이블-SKILL.md 일치 고정 테스트

### Must NOT Have (guardrails)
- readiness/phase 게이트 연결 금지(관측 전용)
- hook에서 RoutingTrace 자동 기록 금지(기록은 main agent 판단 — 기존 원칙)
- 새 CLI/MCP 명령 추가 금지(`routing score`의 기본값 채움만)
- 힌트 다중 줄/카탈로그 재주입 금지(프롬프트 예산)

## Verification Strategy
> 구현+테스트 동일 task. hookcli 스모크는 실제 바이너리 파이프 실행. Evidence: `.issueops/evidence/phase-hint-*.txt`

## Execution Strategy
- **Wave 1**: T1(canonical 테이블), 병렬 없음(소규모)
- **Wave 2**: T2(hook 주입) ∥ T3(routing score 기본값)
- **Wave 3**: T4(golden/문서/검증)

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| T1 | — | T2, T3 |
| T2 | T1 | T4 |
| T3 | T1 | T4 |
| T4 | T2, T3 | — |

## TODOs

- [ ] 1. canonical phase→기대 스킬 테이블 신설

  **What to do**: `internal/core/issueops/routing_expectations.go` 신설 — `func ExpectedSkillRoutings() []SkillRouting`과 `func ExpectedSkillsForPhase(phase string) []string`. 내용은 SKILL.md 273-283 표 그대로: problem/grill/issue/plan→implementation-planning(+grill: web-research, issue/plan: database-design, plan: prompt-engineering·algorithm-optimization·design-review), compatibility-review→design-review, implement→verified-execution·algorithm-optimization·database-design·debugging·git-operations, ai-slop-clean→code-quality-metrics·verified-execution·prompt-engineering·algorithm-optimization, feedback→verified-execution·debugging·web-research, pr→verified-execution·prompt-engineering·git-operations, cleanup→verified-execution·git-operations. prompt-engineering는 cross-cutting이므로 plan/ai-slop-clean/pr에만 명시(SKILL.md 표 기준). 테스트: (a) 모든 phase 키가 유효 phase enum인지, (b) `skills/issueops/SKILL.md`의 요약 표(273-283행)를 파싱해 스킬 집합 일치 고정(드리프트 가드).

  **Recommended Agent**: quick
  **References**: `skills/issueops/SKILL.md:273-283`, `internal/core/issueops/benchmark/issueops_routing_checks.go:38`(SkillRouting 타입), phase enum은 `internal/core/lifecycle`의 IssueOpsPhase 목록
  **Acceptance Criteria**: `go test ./internal/core/issueops -run Routing -count=1` 통과; SKILL.md 표 변경 시 테스트가 깨지는지 fixture 수정으로 확인
  **QA**:
  ```
  Scenario: 테이블-문서 일치 (happy)
    Channel: bash — go test ./internal/core/issueops -run 'ExpectedSkillRoutings' -v
    Expected: SKILL.md 표 파싱 결과와 Go 테이블 diff 0 assert PASS
    Evidence: .issueops/evidence/phase-hint-table.txt
  Scenario: 미지 phase (failure)
    Channel: bash — 동일 테스트 내 ExpectedSkillsForPhase("nonexistent")
    Expected: 빈 목록 반환, panic/error 없음
    Evidence: 동일 파일
  ```
  **Commit**: YES | `feat(issueops): add canonical phase-to-skill routing expectations` | Files: `internal/core/issueops/routing_expectations.go`, `internal/core/issueops/routing_expectations_test.go`

- [ ] 2. hookprompt phase-인지 secondary 힌트 주입

  **What to do**: `BuildUserPromptMCPHints`의 `req.Repo != ""` 분기(hook_prompt.go:101-112)에 추가 — `session.Read`로 primary binding을 읽고 CycleID가 있으면 `issueops.ReadIssueOps`로 record를 읽어 현재 phase를 얻은 뒤, `ExpectedSkillsForPhase(phase)`가 비어 있지 않으면 `addPriority("issueops-phase", "현재 <phase> 단계 — 기대 스킬: <s1, s2, ...>. 실제 사용 시 issueops_record_routing으로 기록.", hintPrioritySecondary)` 1건 주입. 모든 오류는 무시(fail-open). hookprompt→issueops import cycle 여부 확인: cycle이 생기면 `dependencies.go` 패턴대로 함수 주입(변수 훅)으로 역전. 테스트: bound+phase→힌트 존재, unbound→없음, record corrupt→없음, budget 테스트 상한 갱신.

  **Recommended Agent**: deep
  **References**: `internal/core/hookprompt/hook_prompt.go:99-112`(주입 분기), `internal/core/issueops/session/session.go:96-144`(Read/ActiveCycleID), `internal/core/hookprompt/dependencies.go:31`(의존 역전 선례), `internal/core/hookprompt/hook_prompt_test.go`(budget 테스트)
  **Acceptance Criteria**: `go test ./internal/core/hookprompt -count=1` 통과; import cycle 없이 `go build ./...` 성공
  **QA**:
  ```
  Scenario: plan phase 힌트 주입 (happy)
    Channel: bash — temp ISSUEOPS_STATE_DIR에 cycle+binding 시드 후 printf '{"prompt":"진행해줘","repo":"<repo>"}' | ./bin/issueops hook user-prompt
    Expected: additional_context에 "issueops-phase"와 "implementation-planning" 문자열 포함
    Evidence: .issueops/evidence/phase-hint-inject.txt
  Scenario: unbound repo 불변 (failure/경계)
    Channel: bash — binding 없는 repo로 동일 실행
    Expected: 출력에 "issueops-phase" 부재, 기존 힌트 구조 불변
    Evidence: .issueops/evidence/phase-hint-absent.txt
  ```
  **Commit**: YES | `feat(hookprompt): inject phase-aware pioneer skill hints for bound cycles` | Files: `internal/core/hookprompt/hook_prompt.go`(또는 신규 `phase_hint.go`), 관련 테스트

- [ ] 3. routing score의 기대값 기본 채움

  **What to do**: `cmd/issueops/issueopscli/issueops_subcommands.go:237-248` — `--expect` 생략 시 `parseExpectedRouting` 대신 T1의 `ExpectedSkillRoutings()`를 기본값으로 사용. usage 문구에 "(기본: canonical 테이블)" 반영. 테스트: --expect 없이 실행 시 canonical 기대로 채점되는지.

  **Recommended Agent**: quick
  **References**: `cmd/issueops/issueopscli/issueops_subcommands.go:237-256`
  **Acceptance Criteria**: `go test ./cmd/issueops/issueopscli -count=1` 통과
  **QA**:
  ```
  Scenario: 기본 기대 채점 (happy)
    Channel: bash — temp state에 routing 기록 후 ./bin/issueops routing score --id <ID> --json (--expect 없음)
    Expected: missing 목록이 canonical 테이블 기준으로 계산됨
    Evidence: .issueops/evidence/phase-hint-score-default.txt
  Scenario: --expect 명시 우선 (경계)
    Channel: bash — --expect 지정 실행
    Expected: 명시값이 기본값을 대체
    Evidence: 동일 파일
  ```
  **Commit**: YES | `feat(issueops): default routing score expectations to canonical table` | Files: `cmd/issueops/issueopscli/issueops_subcommands.go`, 테스트

- [ ] 4. golden/문서/전체 검증

  **What to do**: (1) usage 문구 변경이 있으면 `go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -update -count=1` 후 diff 추가분만 검수. (2) `.issueops/ARCHITECTURE.md` hook 절과 `skills/issueops/SKILL.md`의 hook 설명(24행 부근)에 phase-인지 힌트 1-2줄 반영, ADR에 결정 기록(관측 전용/게이트 기각/단일 테이블 근거). (3) TESTING.md §2 기본 검증 + hookcli 스모크. **주의**: 테스트 실행 전 CAUTIONS의 파이프 KVA 항목 확인 — stdout-capture 테스트가 512B 강등 상태에서 행할 수 있으니 실행 전 `.test` 고아/파이프 압력을 점검한다.

  **Recommended Agent**: quick
  **References**: `.issueops/TESTING.md:105-110`, `.issueops/CAUTIONS.md`의 2026-07-09 파이프 KVA 항목
  **Acceptance Criteria**: 전체 `go test ./... -count=1`(파이프 압력 정상 상태에서) + golden 통과 + `./bin/issueops docs --json` 정상
  **QA**:
  ```
  Scenario: E2E 스모크 (happy)
    Channel: bash — Definition of Done의 hook user-prompt 파이프 실행 2종
    Expected: bound→힌트 있음 / unbound→없음
    Evidence: .issueops/evidence/phase-hint-e2e.txt
  Scenario: golden additive (failure 감시)
    Channel: bash — git diff cmd/issueops/testdata | rg '^-' | rg -v '^---'
    Expected: 삭제 라인 0건
    Evidence: .issueops/evidence/phase-hint-golden.txt
  ```
  **Commit**: YES | `docs(harness): document phase-aware pioneer hints and record ADR` | Files: 문서 4종 + golden

## Final Verification Wave
- [ ] F1. 계획 준수: Must NOT Have grep(게이트 연결·자동 기록·새 명령 없음)
- [ ] F2. 품질: 힌트 1줄 예산 준수, fail-open 경로 리뷰
- [ ] F3. QA evidence 실재 확인
- [ ] F4. 스코프: unbound repo 출력 byte-diff 0 확인

## Commit Strategy
T1→T2→T3→T4 순 원자 커밋, Conventional Commit + Lore body. push는 사용자 지시 시.

## Success Criteria
- IssueOps cycle이 바인딩된 repo에서 키워드 없는 프롬프트에도 phase 기대 스킬이 1줄로 표시
- 비 IssueOps repo의 hook 출력 완전 불변
- 기대 페어링의 단일 소스(Go 테이블)와 SKILL.md 드리프트 가드 테스트 존재
