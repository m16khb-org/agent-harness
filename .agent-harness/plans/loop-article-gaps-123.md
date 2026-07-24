# 루프 아티클 갭 2종 적용: loop-contract 프리미티브 · 부분 검증 금지

## TL;DR
> **Summary**: Claude Code "Getting Started with Loops" 아티클 갭 분석에서 승인된 제안 1,2,3을 brooks 악마의 변호인 검토로 축소한 범위로 구현한다. P1은 소비자 게이트(strict PR readiness)를 포함한 4-tool 최소 스키마, P2는 TESTING.md 단일 규범 소유, P3는 명시적 `--pilot` 지정 방식.
> **Deliverables**: `internal/core/looprun` 패키지 + `agent-harness loop` CLI/MCP 4종 + strict readiness `loop_incomplete:` 게이트, TESTING.md "부분 검증 상태 금지" 규범 + 스킬 2종 포인터, golden/contract/문서 동기화, ADR 기록.
> **Effort**: Large (P2 0.5일 · P3 1–2일 · P1 3–5일, brooks 추정)
> **Parallel**: YES — 4 waves + Final wave
> **Critical Path**: T3(looprun core) → T5(loop wiring) → T6(consumer 게이트) → T7(golden 재생성) → T9(self-verify)

## Context

### Original Request
사용자: "1,2 전부 적용하기 위한 구체적이고 실행가능한 계획을 수립해줘" — 앞선 분석 턴에서 확정한 갭 제안 1(host-neutral durable loop-contract 프리미티브), 2(부분 검증 상태 금지 명문화)를 구현하는 계획. 제안 4(이벤트 경계)는 범위 밖.

### Interview Summary
사용자 결정이 이미 명확해 인터뷰는 생략하고 repo 탐색 + brooks sub-agent 악마의 변호인 검토로 대체했다. brooks 판정: P1 TRIM(4 tools, name-키, list/success_criteria 삭제, 소비자 1개 명시 필수), P2 PROCEED(단일 규범 소유), P3 TRIM(명시적 `--pilot` 플래그, pilot-accepted 조건, 버전 스큐 문서화). brooks는 P1에 대해 "1주 state-write 컨벤션 스파이크 후 위반 실측 시 착수"를 권했으나, 사용자가 3종 전부 적용을 명시 지시했으므로 스파이크 대신 **소비자 게이트를 v1 스코프에 포함**하는 것으로 강제력 부재 리스크를 상쇄한다(이 결정은 ADR에 기록).

### Gap Analysis
- **P1 소비자 부재 리스크**: 아무 게이트도 loop 상태를 읽지 않으면 fail-closed 규칙이 opt-in에게만 구속됨 → T6에서 `IssueOpsStrictPRReadiness`가 같은 repo의 active/exhausted loop을 `loop_incomplete:<id>`로 차단(기존 `pool_incomplete:` 패턴 복제).
- **id=hash(repo+goal) footgun**: goal free-text 드리프트 → 좀비 active loop 축적 → `hash(repo+name)` 슬러그 키로 교체. 같은 name 재시작 = durable resume.
- **P3 "첫 추가 task=pilot" 암묵 규칙**: 의도 감사 불가 → `add-task --pilot` 명시 지정으로 교체. 게이트 조건은 "accepted task 존재"가 아니라 "**pilot task가 accepted**"(다른 경로 accepted로 게이트가 조용히 열리는 버그 씨앗 차단).
- **P2 3파일 드리프트**: TESTING.md가 규범 전문을 소유하고 스킬 2종은 1문장 + 규범 출처 포인터만.
- **golden 연쇄**: `response_contracts.golden.json`(11,336줄, cli 58/mcp 43 엔트리), `usage.golden.txt`, `mcp_tools.golden.json`, adapter catalog 패리티 테스트가 새 command/tool마다 연쇄로 깨짐 → golden 재생성은 T7 단일 task로 직렬화.
- **버전 스큐**: 구버전 바이너리는 `pilot_required`를 무시하고 claim 가능 → CAUTIONS.md 1줄(T8).

## Work Objectives

### Core Objective
아티클의 루프 운영 원칙(검증 가능한 정지 조건, 부분 검증 금지, 대규모 실행 전 파일럿)을 agent-harness의 기존 문법(fail-closed 게이트, durable state, no-shell, actor model)으로 기계 강제한다.

### Deliverables
- `internal/core/looprun` 패키지(types/store/lifecycle) + `internal/core/loop_facade.go`
- `cmd/harness/loopcli` + `internal/adapter/cli/usage.go` catalog + `internal/adapter/mcp/loop_catalog.go`(tools: `loop_start`/`loop_record_attempt`/`loop_status`/`loop_stop`)
- `IssueOpsStrictPRReadiness`의 `loop_incomplete:<id>` 차단 + doctor active-loop 경고
- TESTING.md "부분 검증 상태 금지" 규범 절 + `skills/self-verify/SKILL.md`·`skills/turing/SKILL.md` 포인터
- golden/contract 갱신, ARCHITECTURE/AGENTS/OPERATIONS/CAUTIONS/SUB_AGENT_PATTERNS/ADR 문서 동기화

### Definition of Done (검증 명령 포함)
- `go test ./... -count=1 && go test -race ./... -count=1 && go vet ./...` 전체 통과
- `go build -o bin/agent-harness ./cmd/harness && ./bin/agent-harness contract check --json` OK
- `go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1` 통과(갱신 후)
- `python3 scripts/validate-skill.py skills/self-verify && python3 scripts/validate-skill.py skills/turing` 통과
- `HARNESS_STATE_DIR=$(mktemp -d) ./bin/agent-harness self-verify --seed=100 --target-score=95 --json` gate 통과
- 아래 각 task의 QA 시나리오 evidence 캡처 완료

### Must Have
- P1 fail-closed 3규칙: attempt에 evidence 필수 / `stop --success`는 마지막 attempt verdict=pass 필수 / attempts ≥ max_attempts 시 auto-exhaust + 이후 attempt 거부
- P1 소비자: strict PR readiness가 같은 repo의 active/exhausted loop을 차단
- 하네스는 `verify_argv`를 절대 실행하지 않음(no-shell) — 기록만
- P3 게이트 조건은 `pool.PilotTaskID` task의 `status == "accepted"` 검사
- 모든 신규 freeform 텍스트(goal, evidence)는 기존 secret 패턴 redaction 경유

### Must NOT Have (guardrails)
- 스케줄러/타이머/데몬 상주 루프 금지(ADR 기각 결정 유지)
- 하네스의 에이전트 spawn·shell 실행 금지(actor model)
- hook에서 loop 자동 진행/자동 기록 금지(hook은 observe/block/relay만)
- `loop list` CLI/MCP 표면 금지(v1; 내부 enumeration은 store 함수로만) · `success_criteria` 별도 필드 금지(goal에 포함)
- 토큰 사용량 계측 금지(host 내부 데이터) — attempt 수 + evidence로 대체
- 기존 pool 동작 변경 금지: `PilotRequired=false` pool은 현재와 byte-level 동일 동작

## Verification Strategy
> ZERO HUMAN INTERVENTION — 모든 검증은 에이전트 실행.
- Test decision: tests-after 아님 — **구현+테스트 동일 task**(repo 컨벤션: table-driven Go test, `t.Setenv("HARNESS_STATE_DIR", t.TempDir())` 격리, TESTING.md §3 기준)
- QA policy: 모든 task에 happy + failure 시나리오, channel은 bash(CLI 직접 실행)
- Evidence: `.agent-harness/evidence/task-{N}-{slug}.txt`
- Golden: 의도된 contract 변경이므로 T7에서만 `-update` 실행(TESTING.md §2 절차)

## Execution Strategy

### Parallel Execution Waves
- **Wave 1**: T1(P2 문서 규범), T3(P1 looprun core+테스트) — 서로 다른 파일/패키지, 완전 병렬
- **Wave 2**: T4(P3 CLI/MCP 배선), T5(P1 CLI/MCP 배선) — golden은 건드리지 않고 코드/카탈로그만
- **Wave 3**: T6(P1 소비자 게이트 + doctor), T7(golden/contract 일괄 재생성) — T7은 T4·T5·T6 완료 후 단일 실행
- **Wave 4**: T8(문서 동기화 + ADR), T9(전체 검증 + self-verify)
- **Final Wave**: F1–F4

### Dependency Matrix

| Task | Depends On | Blocks | Can Parallelize With |
|------|-----------|--------|---------------------|
| T1 | — | T9 | T2, T3 |
| T2 | — | T4 | T1, T3 |
| T3 | — | T5, T6 | T1, T2 |
| T4 | T2 | T7 | T5 |
| T5 | T3 | T6, T7 | T4 |
| T6 | T3, T5 | T7 | — |
| T7 | T4, T5, T6 | T8, T9 | — |
| T8 | T7 | T9 | — |
| T9 | T1, T7, T8 | — | — |

## TODOs

- [ ] 1. P2: "부분 검증 상태 금지" 규범을 TESTING.md에 신설하고 스킬 2종에 포인터 추가

  **What to do**:
  1. `.agent-harness/TESTING.md`의 "## 5. 완료 보고 기준" 앞에 새 절 `## 부분 검증 상태 금지 (all-or-nothing verification)` 추가. 규범 전문: (a) 다단계 검증 시나리오에서 한 단계라도 실패하면 이전 단계의 통과를 재사용하지 않고 1단계부터 전체 재실행한다, (b) 완료 보고의 evidence는 마지막 "전 단계 통과" 단일 run에서 나온 것이어야 하며 서로 다른 run의 부분 통과를 조합하지 않는다, (c) 재실행 비용이 큰 경우에도 부분 통과 상태를 "검증됨"으로 승격하지 않는다 — 비용이 문제면 검증 시나리오를 더 작은 독립 시나리오로 분리한다.
  2. `skills/self-verify/SKILL.md` "## Gate" 절 끝에 1문장 + 포인터: "다단계 검증에서 한 단계라도 실패하면 1단계부터 재실행하며 부분 통과 evidence를 재사용하지 않는다 (규범 출처: `.agent-harness/TESTING.md` 부분 검증 상태 금지 절)."
  3. `skills/turing/SKILL.md` "## Final Quality Gate" 절(316행 부근)에 동일한 1문장 + 포인터 추가.
  4. 두 스킬 모두 문장은 동일하게 유지(드리프트 방지) — 규범 전문은 TESTING.md에만 존재.

  **Must NOT do**: 세 파일에 규범 전문을 각기 풀어쓰지 않는다. AGENT_WORKFLOW.md/AGENTS.md는 건드리지 않는다(추후 필요 시 별도).

  **Recommended Agent**: quick — 문서 3개의 국소 편집.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T9 | Blocked By: —

  **References**:
  - `.agent-harness/TESTING.md:184` — "## 5. 완료 보고 기준" 앞 삽입 위치
  - `skills/self-verify/SKILL.md:31-35` — Gate 절 구조
  - `skills/turing/SKILL.md:316` — Final Quality Gate 절
  - 아티클 근거: verify-frontend-change 예시 "단계 실패 시 1단계부터 재실행, 부분 검증 상태 금지"

  **Acceptance Criteria**:
  - [ ] `rg -n "부분 검증 상태 금지" .agent-harness/TESTING.md skills/self-verify/SKILL.md skills/turing/SKILL.md`가 3파일 모두에서 hit
  - [ ] `python3 scripts/validate-skill.py skills/self-verify && python3 scripts/validate-skill.py skills/turing` exit 0
  - [ ] 스킬 2종의 삽입 문장이 diff 상 동일 문자열

  **QA Scenarios**:
  ```
  Scenario: 규범 단일 소유 확인
    Channel: bash
    Steps: rg -c "1단계부터" .agent-harness/TESTING.md skills/self-verify/SKILL.md skills/turing/SKILL.md
    Expected: TESTING.md는 규범 전문(≥3줄 관련 내용), 스킬 2종은 각 1회만 언급 + "규범 출처" 문자열 포함
    Evidence: .agent-harness/evidence/task-1-no-partial-verify.txt

  Scenario: 스킬 스키마 검증 실패 회귀 없음
    Channel: bash
    Steps: python3 scripts/validate-skill.py skills/self-verify; python3 scripts/validate-skill.py skills/turing
    Expected: 두 명령 모두 exit 0
    Evidence: .agent-harness/evidence/task-1-skill-validate.txt
  ```

  **Commit**: YES | `docs(testing): forbid partial verification state with single normative source` | Files: `.agent-harness/TESTING.md`, `skills/self-verify/SKILL.md`, `skills/turing/SKILL.md`

- [ ] 3. P1 core: `internal/core/looprun` 패키지(types/store/lifecycle) + 테스트

  **What to do**:
  1. `internal/core/looprun/types.go`:
     - `const LoopRunCurrentSchemaVersion = 1`
     - `LoopRun{OK bool, SchemaVersion int, ID string, Repo string, Name string, Goal string, VerifyArgv []string, MaxAttempts int, Status string, Attempts []LoopAttempt, StopReason string \`json:",omitempty"\`, CreatedAt, UpdatedAt string}` — status enum: `active`/`succeeded`/`exhausted`/`stopped`
     - `LoopAttempt{Seq int, Verdict string, Evidence []string, At string}` — verdict enum: `pass`/`fail`
     - `StartLoopRequest{Repo, Name, Goal string, VerifyArgv []string, MaxAttempts int}`, `RecordAttemptRequest{Verdict string, Evidence []string}`
  2. `internal/core/looprun/store.go`: 기존 sqlstore 패턴을 사용 — `StateRoot() = filepath.Join(state.StateDir(), "loop")`, sqlstore bucket `"loop"`, `newLoopID(repo, name) = "loop-" + sha256(repo+"\x00"+name)[:12]`, `normalizeLoopID`(`loop-` prefix, `..`/`/`/`\` 금지), `ReadLoop`/`writeLoop`/`ListLoopIDs`(내부 전용, CLI 미노출), `normalizeLoopSchemaVersion`(future version fail-safe 거부), `loopNow = time.Now` 테스트 훅.
  3. `internal/core/looprun/lifecycle.go`:
     - `Start(req)`: repo abs 정규화, name/goal 필수, `MaxAttempts` 기본 5·최대 50, goal은 기존 secret 패턴으로 redaction. 같은 id의 기존 record가 있으면 — `active`면 기존 record 반환(durable resume), terminal이면 `loop_terminal` 에러(새 name 안내).
     - `RecordAttempt(id, req)`: `withLoopLock`(sqlstore WithSpan) 안에서 — status != active → `loop_not_active`; evidence 비면 `evidence_required`; verdict는 pass/fail만; append 후 `verdict == "fail" && len(Attempts) >= MaxAttempts`면 status=`exhausted`로 자동 전이; `verdict == "pass"`는 상태를 바꾸지 않음(성공 선언은 stop의 책임).
     - `Stop(id, success bool, reason string)`: success=true면 마지막 attempt가 존재하고 verdict==pass여야 함(`loop_success_requires_pass`), status=`succeeded`; success=false면 reason ≥10자 필수(`stop_reason_too_short`), status=`stopped`. exhausted 상태에서도 stop 가능(사유 기록 종결).
     - `Status(id)`: record + 파생 요약(attempt count, last verdict) 반환.
     - 하네스는 `VerifyArgv`를 절대 실행하지 않음 — 저장만.
  4. 테스트(`lifecycle_test.go`, `store_test.go`): start-resume 멱등성, terminal 재시작 거부, evidence 필수, auto-exhaust 경계(max번째 fail에서 전이, pass는 미전이), exhausted 후 attempt 거부, success-stop의 pass 요구, stop reason 길이, schema future-version 거부, redaction(`token=abc` → `<redacted>`), `HARNESS_STATE_DIR` temp 격리.

  **Must NOT do**: `List` CLI/MCP 표면, `success_criteria` 필드, goal 해시 키, verify 명령 실행, hook 연동. `MaxAttempts=0`을 무제한으로 해석하지 않는다(0 → 기본값 5).

  **Recommended Agent**: deep — 신규 패키지의 상태 기계 + fail-closed 경계.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T5, T6 | Blocked By: —

  **References**:
  - `internal/core/sqlstore` — open/Get/Put/List, id 정규화, schema version 처리와 span 규칙
  - 기존 secret-redaction helper — 동일 정규식 사용; looprun에 복제하되 공용화는 하지 않음
  - `.agent-harness/ARCHITECTURE.md` §5 — state root 격리 원칙

  **Acceptance Criteria**:
  - [ ] `go test ./internal/core/looprun -count=1 && go test -race ./internal/core/looprun -count=1` 통과
  - [ ] `go vet ./internal/core/looprun` clean
  - [ ] fail-closed 3규칙(evidence 필수/success-requires-pass/auto-exhaust) 각각 전용 테스트 존재

  **QA Scenarios**:
  ```
  Scenario: durable resume 멱등성 (happy)
    Channel: bash (go test)
    Steps: go test ./internal/core/looprun -run 'StartResume' -count=1 -v
    Expected: 같은 repo+name 2회 Start → 같은 ID, 같은 record, attempts 보존 assert PASS
    Evidence: .agent-harness/evidence/task-3-loop-resume.txt

  Scenario: evidence 없는 성공 선언 차단 (failure)
    Channel: bash (go test)
    Steps: go test ./internal/core/looprun -run 'SuccessRequiresPass|EvidenceRequired|AutoExhaust' -count=1 -v
    Expected: 3규칙 위반이 각각 loop_success_requires_pass / evidence_required / exhausted 후 loop_not_active 에러로 거부됨
    Evidence: .agent-harness/evidence/task-3-loop-failclosed.txt
  ```

  **Commit**: YES | `feat(looprun): add durable loop-contract state machine core` | Files: `internal/core/looprun/{types,store,lifecycle}.go`, `internal/core/looprun/{store,lifecycle}_test.go`

- [ ] 5. P1 wiring: `agent-harness loop` CLI + MCP 4-tool 배선

  **What to do**:
  1. `internal/core/loop_facade.go`: 기존 core facade 패턴대로 `StartLoopRun`/`RecordLoopAttempt`/`LoopRunStatus`/`StopLoopRun` + type alias.
  2. `cmd/harness/loopcli/loop.go` 신설: 서브커맨드 `start`(`--repo`, `--name`, `--goal`, `--max-attempts`, verify argv는 `--` 뒤 나머지 인자 — policy check와 동일 문법), `record-attempt`(`--id`, `--verdict pass|fail`, `--evidence TEXT` 반복 허용), `status`(`--id` 또는 `--repo`+`--name`), `stop`(`--id`, `--success` 또는 `--reason TEXT`). 모두 `--json` 지원, 사람이 읽을 text 출력 병행(AGENTS.md §7 계약).
  3. dispatch 등록: `cmd/harness/harnessapp/cli_facade.go` + `root_command_facade.go`의 기존 도메인 등록 방식 복제.
  4. `internal/adapter/cli/usage.go`: catalog에 `{Name: "loop", Description: "track durable verify-until-done loop contracts"}` + usage 4줄 추가.
  5. `internal/adapter/mcp/loop_catalog.go` 신설: `loop_start`/`loop_record_attempt`/`loop_status`/`loop_stop` descriptor(각각 쓰기 여부, 필수 인자, 결과 shape 명시). `internal/adapter/mcp/catalog.go`의 집계 목록에 결정적 순서로 추가. MCP handler를 `cmd/harness` MCP 매핑에 추가.
  6. `cmd/harness/loopcli/loop_cli_test.go`: flag 파싱, `--` argv 캡처, verdict enum 거부, JSON shape 스모크. golden은 건드리지 않음(T7).

  **Must NOT do**: `loop list` 표면 금지. daemon/hook 연동 금지. CLI와 MCP 의미 차이 금지.

  **Recommended Agent**: deep — 다층 배선(cli facade/rootcmd/mcp catalog/handler)이라 등록 누락 리스크.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T6, T7 | Blocked By: T3

  **References**:
  - `cmd/harness/harnessapp/cli_facade.go`, `cmd/harness/harnessapp/root_command_facade.go` — dispatch 등록 지점
  - `internal/adapter/mcp/catalog.go` — MCP catalog 등록·순서 규칙
  - `internal/adapter/cli/usage.go:38,118-126` — catalog entry/usage 라인 형식
  - `./bin/agent-harness policy check ... --json -- git status` — `--` argv 문법 선례

  **Acceptance Criteria**:
  - [ ] `go test ./cmd/harness/loopcli ./cmd/harness/harnessapp -count=1` 통과(golden 제외 대상은 T7 이후 재확인)
  - [ ] `go build -o bin/agent-harness ./cmd/harness` 성공
  - [ ] `./bin/agent-harness loop start --repo "$PWD" --name smoke --goal "lint clean" --max-attempts 3 --json -- go vet ./...` 정상 JSON 반환

  **QA Scenarios**:
  ```
  Scenario: 루프 전체 수명주기 CLI 스모크 (happy)
    Channel: bash
    Steps: tmp=$(mktemp -d); export HARNESS_STATE_DIR=$tmp
      ./bin/agent-harness loop start --repo "$PWD" --name qa-loop --goal "tests pass" --max-attempts 2 --json -- go test ./...
      ./bin/agent-harness loop record-attempt --id <ID> --verdict fail --evidence "go test: 1 failure in core" --json
      ./bin/agent-harness loop record-attempt --id <ID> --verdict pass --evidence "go test ./... ok all packages" --json
      ./bin/agent-harness loop stop --id <ID> --success --json
      ./bin/agent-harness loop status --id <ID> --json
    Expected: status가 "succeeded", attempts 배열 길이 2, verify_argv에 ["go","test","./..."]
    Evidence: .agent-harness/evidence/task-5-loop-cli-smoke.txt

  Scenario: auto-exhaust와 성공 종료 거부 (failure)
    Channel: bash
    Steps: 같은 방식으로 max-attempts 1 루프 생성 → verdict fail 1회 기록 → record-attempt 재시도 → stop --success 시도
    Expected: 두 번째 attempt는 loop_not_active 계열 에러, status는 "exhausted", stop --success는 loop_success_requires_pass 에러
    Evidence: .agent-harness/evidence/task-5-loop-exhaust.txt
  ```

  **Commit**: YES | `feat(loop): wire loop-contract CLI and MCP surfaces` | Files: `internal/core/loop_facade.go`, `cmd/harness/loopcli/*`, `cmd/harness/harnessapp/{cli_facade,root_command_facade}.go`, `internal/adapter/cli/usage.go`, `internal/adapter/mcp/{loop_catalog,catalog}.go`, MCP handler 파일

- [ ] 6. P1 소비자: strict PR readiness의 `loop_incomplete:` 차단 + doctor 경고

  **What to do**:
  1. `internal/core`의 `IssueOpsStrictPRReadiness`에 loop 게이트 추가: record의 repo와 같은 repo를 가진 loop 중 status가 `active` 또는 `exhausted`인 것이 있으면 `Missing`에 `loop_incomplete:<loop-id>` append. 내부 enumeration은 `looprun.ListLoopIDs()` + `ReadLoop` 사용(CLI 표면 아님). repo 비교는 abs path 정규화 후 일치.
  2. `agent-harness doctor`에 active loop 관측 추가: state root의 loop bucket을 읽어 repo별 active/exhausted loop 수를 진단 항목으로 보고(수정 없음, 보고만 — doctor 계약 유지).
  3. 테스트: 신규 `issueops_loop_gate_test.go` — (a) loop 없음 → strict-ready 유지, (b) active loop → `loop_incomplete:` 차단, (c) `loop stop --success`/`stop --reason` 후 차단 해제, (d) 다른 repo의 loop은 차단하지 않음. doctor 출력 테스트 1건.

  **Must NOT do**: pr-readiness 외 다른 phase 게이트에 loop을 끼워넣지 않는다(스코프 최소). hook에서 loop 상태를 강제하지 않는다.

  **Recommended Agent**: deep — 기존 readiness 계약에 additive 필드 추가라 회귀 민감.

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: T7 | Blocked By: T3, T5

  **References**:
  - `internal/core/issueops_pool_gate_test.go:15-60` — `pool_incomplete:` 게이트·테스트 원형(verbatim 확보됨)
  - `IssueOpsStrictPRReadiness` 구현부 — pool 차단 로직이 있는 함수(grep `pool_incomplete`)
  - `internal/core/looprun/store.go`(T3 산출물) — ListLoopIDs/ReadLoop
  - `.agent-harness/ARCHITECTURE.md:131` — strict readiness가 요구하는 기존 항목 목록

  **Acceptance Criteria**:
  - [ ] `go test ./internal/core -run 'LoopGate' -count=1` 통과
  - [ ] 기존 `go test ./internal/core -run 'PoolGate' -count=1` 무수정 통과(회귀 0)
  - [ ] `HARNESS_STATE_DIR=$(mktemp -d) ./bin/agent-harness doctor --repo . --json`에 loop 진단 필드 존재

  **QA Scenarios**:
  ```
  Scenario: active loop이 PR readiness를 차단 (happy-gate)
    Channel: bash (go test)
    Steps: go test ./internal/core -run 'StrictPRReadiness.*Loop' -count=1 -v
    Expected: active loop 존재 시 Missing에 loop_incomplete:<id>, stop 후 Ready=true assert PASS
    Evidence: .agent-harness/evidence/task-6-loop-gate.txt

  Scenario: 타 repo loop 오차단 없음 (failure/경계)
    Channel: bash (go test)
    Steps: go test ./internal/core -run 'LoopGateOtherRepo' -count=1 -v
    Expected: 다른 repo의 active loop은 Missing에 나타나지 않음
    Evidence: .agent-harness/evidence/task-6-loop-gate-scope.txt
  ```

  **Commit**: YES | `feat(issueops): block strict PR readiness on incomplete loop contracts` | Files: `internal/core/issueops_*.go`(readiness 구현 파일), `internal/core/issueops_loop_gate_test.go`, doctor 구현/테스트 파일

- [ ] 7. golden/contract 일괄 재생성 및 계약 검증

  **What to do**:
  1. TESTING.md §2 절차대로 의도된 contract 변경만 갱신:
     `go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -update -count=1`
  2. `git diff`로 golden 변경을 라인 단위 검수: `usage.golden.txt`(+loop 4줄), `mcp_tools.golden.json`(+4 loop tools), `response_contracts.golden.json`(loop 4 command/tool required-field 엔트리). **추가 엔트리 외 기존 라인 변경이 있으면 원인 규명 후 롤백**.
  3. `./bin/agent-harness contract check --json` + `go test ./cmd/harness/contractcli -count=1` 실행.
  4. self-verify의 `summary.contract` goal이 카탈로그 수 변화에 실패하지 않는지 확인, 실패 시 해당 fixture 갱신.

  **Must NOT do**: `-update`를 T4/T5/T6 이전에 실행 금지. 의도치 않은 기존 엔트리 shape 변경을 golden에 흡수 금지.

  **Recommended Agent**: quick — 기계적이지만 diff 검수 규율 필요.

  **Parallelization**: Can Parallel: NO | Wave 3 (T4·T5·T6 완료 후) | Blocks: T8, T9 | Blocked By: T4, T5, T6

  **References**:
  - `.agent-harness/TESTING.md:105-110` — golden 갱신 명령과 조건
  - `cmd/harness/testdata/{usage.golden.txt,mcp_tools.golden.json,response_contracts.golden.json}` — 대상 3종(response_contracts는 11,336줄 — diff 검수 필수)
  - `.agent-harness/AGENTS.md §10` — response-contract golden이 고정하는 범위

  **Acceptance Criteria**:
  - [ ] `go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1` (–update 없이) 통과
  - [ ] `./bin/agent-harness contract check --json` OK
  - [ ] golden diff가 추가 엔트리로만 구성됨(`git diff --stat` + 수동 검수 기록)

  **QA Scenarios**:
  ```
  Scenario: golden 재현성 (happy)
    Channel: bash
    Steps: go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1
    Expected: -update 없이 전부 PASS (재생성 결과가 결정적)
    Evidence: .agent-harness/evidence/task-7-golden.txt

  Scenario: 기존 계약 불변 (failure 감시)
    Channel: bash
    Steps: git diff cmd/harness/testdata/ | rg '^-' | rg -v '^---' | head -20
    Expected: 삭제 라인 0건(추가만) — 0건이 아니면 task FAIL로 보고
    Evidence: .agent-harness/evidence/task-7-golden-additive.txt
  ```

  **Commit**: YES | `test(contract): regenerate goldens for loop surfaces` | Files: `cmd/harness/testdata/*.golden.*`

- [ ] 8. 문서 동기화 + ADR 기록

  **What to do**:
  1. `.agent-harness/ARCHITECTURE.md`: §3 실행 모드 표에 `agent-harness loop` 행(용도: verify-until-done 루프 계약의 durable 상태; 원칙: 하네스는 검증을 실행하지 않고 기록·게이트만), §5 state 절에 loop 위치(`~/.local/state/agent-harness/loop/`)와 제공 표면(CLI 4종/MCP 4종), strict readiness 요구 항목에 loop 추가.
  2. `AGENTS.md`: §8 디렉토리 맵 `cmd/harness` 행의 명령 목록에 `loop` 추가, §9 Essential Commands에 loop 스모크 1줄.
  3. `.agent-harness/OPERATIONS.md` Core Surfaces CLI 목록 + `.agent-harness/operations/cli-and-mcp.md`에 loop/pilot 사용법.
  5. `.agent-harness/CAUTIONS.md`: 버전 스큐 1줄 — "pilot_required pool은 구버전 바이너리에서 게이트 없이 claim될 수 있다(additive 필드 무시). shared daemon과 로컬 CLI 버전을 함께 갱신한다."
  6. `.agent-harness/ADR.md`: 신규 결정 기록 — 아티클 출처, brooks TRIM 결정(4 tools/name-키/소비자 게이트, list·success_criteria 기각, goal-hash 기각, "첫 task=pilot" 기각), 스파이크 대신 소비자 게이트 포함으로 간 근거(사용자 명시 지시), 기각 대안(스케줄러, hook 강제, 토큰 계측), `state write` 컨벤션 대안이 진 이유(전이 강제 불가).
  7. TESTING.md §4 golden 목록에 loop/pilot 항목 추가.

  **Must NOT do**: README 마케팅성 서술 추가 금지. 문서와 구현 불일치 방치 금지(T7 완료 후 실제 출력 기준으로 작성).

  **Recommended Agent**: quick — 문서 편집이지만 교차 참조 정확성 필요.

  **Parallelization**: Can Parallel: NO | Wave 4 | Blocks: T9 | Blocked By: T7

  **References**:
  - `.agent-harness/ARCHITECTURE.md:62-73,95-137` — 실행 모드 표·state 절 형식
  - `.agent-harness/ADR.md:428-437,484-501` — article-insight + brooks TRIM 기록 선례 형식
  - brooks 리뷰 결과(이 계획의 Gap Analysis 절) — ADR에 옮길 기각 근거 원문

  **Acceptance Criteria**:
  - [ ] `./bin/agent-harness docs --json` 정상(문서 index 파손 없음)
  - [ ] `rg -n "loop" .agent-harness/ARCHITECTURE.md AGENTS.md .agent-harness/OPERATIONS.md | rg -i "agent-harness loop"` hit
  - [ ] ADR 엔트리에 기각 대안 ≥3건 명시

  **QA Scenarios**:
  ```
  Scenario: 문서-구현 일치 (happy)
    Channel: bash
    Steps: ./bin/agent-harness loop --help 2>&1 | head -10; rg -n 'loop start' .agent-harness/operations/cli-and-mcp.md
    Expected: 문서의 명령 문법이 실제 usage 출력과 동일 문자열
    Evidence: .agent-harness/evidence/task-8-docs-parity.txt

  Scenario: docs index 회귀 없음 (failure 감시)
    Channel: bash
    Steps: ./bin/agent-harness docs --json | head -5
    Expected: 정상 JSON, 에러 0
    Evidence: .agent-harness/evidence/task-8-docs-index.txt
  ```

  **Commit**: YES | `docs(harness): document loop contract and ADR decisions` | Files: `.agent-harness/{ARCHITECTURE,ADR,OPERATIONS,CAUTIONS,SUB_AGENT_PATTERNS,TESTING}.md`, `.agent-harness/operations/cli-and-mcp.md`, `AGENTS.md`

- [ ] 9. 전체 검증 + self-verify 게이트 + 완료 보고

  **What to do**:
  1. TESTING.md §2 전체 검증: `go mod tidy && go test ./... -count=1 && go test -race ./... -count=1 && go vet ./... && go build -o bin/agent-harness ./cmd/harness`
  2. 스모크: `./bin/agent-harness inspect --json`, `docs --json`, `daemon status --json`, T4/T5 QA 시나리오 재실행.
  3. `HARNESS_STATE_DIR=$(mktemp -d) ./bin/agent-harness self-verify --seed=100 --target-score=95 --json` — 95점 게이트. 실패 시 `lint_diagnose`로 진단 후 수정 반복(부분 검증 금지 규칙 적용: 수정 후 §2 전체부터 재실행).
  4. 완료 보고: 실행한 검증 명령과 결과, Not-tested 항목(예: GJC 실기기 hook 스모크), 변경 파일 요약, 남은 위험(버전 스큐, response_contracts 엔트리 설계 판단).

  **Must NOT do**: 부분 통과 evidence 조합 금지(T1에서 명문화한 규칙 그대로 적용). 실패를 hedge 없이 보고.

  **Recommended Agent**: quick — 실행·수집 중심, 판단은 main agent.

  **Parallelization**: Can Parallel: NO | Wave 4 | Blocks: — | Blocked By: T1, T7, T8

  **References**:
  - `.agent-harness/TESTING.md:85-110,184-195` — 검증 명령·완료 보고 기준·QA gate
  - `skills/self-verify/SKILL.md:14-35` — self-verify 명령·게이트 계약

  **Acceptance Criteria**:
  - [ ] 전체 테스트/race/vet/build 4종 exit 0
  - [ ] self-verify 모든 goal score > 95
  - [ ] 완료 보고에 Not-tested 절 포함

  **QA Scenarios**:
  ```
  Scenario: 95점 게이트 (happy)
    Channel: bash
    Steps: HARNESS_STATE_DIR=$(mktemp -d) ./bin/agent-harness self-verify --seed=100 --target-score=95 --json
    Expected: 모든 goal_scores > 95, 종료 코드 0
    Evidence: .agent-harness/evidence/task-9-self-verify.txt

  Scenario: race detector 청정 (failure 감시)
    Channel: bash
    Steps: go test -race ./internal/core/looprun -count=1
    Expected: PASS, DATA RACE 출력 0건
    Evidence: .agent-harness/evidence/task-9-race.txt
  ```

  **Commit**: NO (검증 전용; 수정 발생 시 해당 task 커밋 규칙으로 회귀)

## Final Verification Wave (MANDATORY — 모든 구현 task 후)
> ALL must APPROVE. 통합 결과를 사용자에게 보고하고 명시적 승인 후 완료.
- [ ] F1. Plan Compliance Audit — 모든 TODO가 명세대로 실행됐는가? (Must NOT Have 목록 위반 grep 포함: `rg -n "loop list|loop_list|success_criteria" cmd internal`이 0건)
- [ ] F2. Code Quality Review — `oh-my-claudecode:code-reviewer` 또는 shannon 측정으로 AI slop/불필요 추상화 부재 확인.
- [ ] F3. Real Manual QA — T1–T9의 모든 QA 시나리오 evidence가 `.agent-harness/evidence/`에 실재하고 PASS인지 재확인. 특히 T9의 self-verify 95점 게이트.
- [ ] F4. Scope Fidelity Check — 제안 4(이벤트 경계)·스케줄러·hook 자동화가 스며들지 않았는지, 기존 state 동작 회귀가 없는지 `git diff --stat`로 대조.

## Commit Strategy
task별 원자 커밋(각 task의 Commit 라인 참조). 순서: T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8. 형식은 `.agent-harness/COMMIT_POLICY.md`의 Conventional Commit subject + Lore body. golden 갱신(T7)은 배선 커밋과 분리해 리뷰 가능하게 유지. push는 사용자 지시 시에만.

## Success Criteria
- 아티클의 3개 갭이 각각 기계 강제 게이트(P1 소비자 게이트, P3 claim 게이트) 또는 단일 규범(P2)으로 착지
- 기존 계약 회귀 0: PilotRequired=false pool·기존 CLI/MCP 응답 shape 불변(golden diff는 추가 엔트리만)
- brooks 지적 사항 3건(소비자 부재, goal-hash footgun, 암묵 pilot 규칙) 모두 해소된 상태로 구현
- self-verify 95점 게이트 통과 + 완료 보고에 Not-tested 항목 명시
