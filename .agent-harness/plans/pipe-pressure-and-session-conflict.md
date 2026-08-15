# 파이프 압력 테스트 행 + 멀티 세션 충돌 해결

## TL;DR
> **Summary**: (문제 1) macOS 파이프 KVA 고갈 시 stdout-capture 테스트가 행하는 문제를 — 공유 캡처 헬퍼(동시 reader)로 전 패키지를 면역화하고, doctor에 파이프 용량 진단을 추가하고, 누수 호스트 재시작 runbook을 명문화해 — 3층으로 해결한다. (문제 2) 다른 세션과의 동시 작업 충돌은 — 문서 변경 즉시 커밋 + 이후 구현은 격리 worktree 브랜치 + 착수/커밋 전 충돌 체크 절차로 해결한다.
> **Deliverables**: `internal/testsupport` 공유 캡처 헬퍼 + 취약 헬퍼 전수 교체, doctor `pipe_capacity` 체크, CAUTIONS runbook 보강, docs 커밋 1건, worktree 격리 작업 절차, ADR 기록.
> **Effort**: Medium (1–2일)
> **Parallel**: YES — 3 waves
> **Critical Path**: T1(커밋 분리) → T2(worktree 생성) → T3(공유 헬퍼) → T4(전수 교체) → T7(검증)

## Context

### Original Request
사용자: "두 문제를 해결하기 위한 구체적이고 실행가능한 계획 수립" — 두 문제는 직전 진단에서 확정된 (1) 파이프 KVA 고갈로 인한 stdout-capture CLI 테스트 무기한 행, (2) 같은 repo에서 다른 세션(codex PID 91044 계열 + updatecli 수정 중인 세션)과의 동시 작업 충돌.

### 진단 근거 (요약)
- 시스템 파이프 fd 14,402개, codex 1개 프로세스가 3,112개 점유 → 신규 파이프 100/100이 512B 버퍼로 강등(실측) → write-then-read 캡처 헬퍼가 512B 초과 JSON에서 데드락 → go test 600s FAIL. 상세: `.agent-harness/CAUTIONS.md` 2026-07-09 파이프 KVA 항목.
- 표준 수정 패턴 실재: `cmd/harness/harnessapp/response_contract_runners_test.go:66-85` — `io.ReadAll(r)`을 goroutine으로 **fn 실행 전에** 시작해 파이프 버퍼 크기 의존을 제거(6ee897d).
- 취약 write-then-read 헬퍼 전수(grep `os.Pipe()` + fn 후 `io.ReadAll`): `cmd/harness/{loopcli,statecli,policycli,projectcli,basiccli,qualitycli,commandstep,issueopscli(2),hookcli(hook_user_prompt, vcsissue/hook_capture, hook_post_tool_use),mcpcli,daemoncli,selfworkflow/{historycompare,candidatescmd},...}` — 고아로 발견된 `.test` 바이너리 목록(statecli/statuscli/workercli/candidatescmd/historycompare/promotecmd/verifycmd)과 일치.
- 다른 세션이 `cmd/harness/updatecli/update_bootstrap_mcp.go`(+테스트)를 수정 중 — updatecli/mcp proxy 영역은 이 계획의 범위에서 제외한다.

### Gap Analysis
- **중복 vs 공유**: 패키지마다 헬퍼를 개별 수정하면 (a) 15곳 중복, (b) 미래 신규 패키지가 다시 취약 패턴을 복붙할 위험 → 공유 non-test 패키지 `internal/testsupport`로 단일화. Go 테스트는 non-test 패키지를 import할 수 있으므로 성립.
- **동시 수정 충돌**: mcpcli/daemoncli 캡처 헬퍼는 다른 세션의 작업 영역(mcp proxy)과 파일이 겹칠 수 있음 → 전수 교체 task에서 **updatecli는 제외**, mcpcli/daemoncli는 착수 시점에 `git status` 재확인 후 겹치면 보류 목록으로 이월.
- **검증 자체가 행 위험**: 파이프 압력이 해소되지 않으면 교체 후 검증도 행할 수 있음 → 교체 자체가 행을 없애므로 순서를 "교체 먼저, 전체 검증은 교체 후"로 배치. 교체 전 상태에서의 대조 실험(행 재현)은 이미 완료된 증거를 사용하고 재실험하지 않는다.
- **doctor 체크의 부작용**: os.Pipe 1개 생성/즉시 close는 무해·ms 수준. 임계값은 실측 근거로 8192(정상 16384의 절반) 미만 경고.
- **worktree와 golden**: golden 재생성은 worktree 안에서 실행해도 `cmd/harness/testdata` 경로가 worktree-로컬이므로 안전.

## Work Objectives

### Core Objective
파이프 압력이 어떤 상태여도 이 저장소의 테스트가 행하지 않게 만들고(면역화), 압력 자체는 진단 가능하게 하며(doctor), 멀티 세션 환경에서 안전하게 구현 작업을 진행할 절차를 확립한다.

### Definition of Done (검증 명령 포함)
- 파이프 강등 상태를 모사한 단위 테스트(공유 헬퍼가 8KB+ 출력에서 블록 없이 동작)가 통과
- `rg -l 'os\.Pipe\(\)' cmd --type go` 결과 중 updatecli/제외 목록 외 전부가 `testsupport.CaptureStdout` 사용으로 전환 (`rg -n 'io.ReadAll\(r\)' cmd/harness/*cli* | wc -l` 감소분으로 확인)
- `HARNESS_STATE_DIR=$(mktemp -d) ./bin/agent-harness doctor --repo . --json`에 `pipe_capacity` 진단 필드 존재, 강등 시 warning
- worktree 브랜치에서 `go test ./cmd/harness/... -count=1` 전체 통과 (600s 행 0건)
- main 작업 트리는 docs 커밋 외 무변경(다른 세션 파일 불간섭)

### Must Have
- 공유 헬퍼는 6ee897d 패턴 그대로: reader goroutine을 fn 실행 **전** 시작, w.Close 후 join, os.Stdout 복원 순서 보존
- 교체는 기계적 치환(동작 동일) — 각 패키지의 assertion/에러 메시지 유지
- 모든 구현 작업은 격리 worktree 브랜치에서, main tree는 읽기 전용으로 취급
- 착수/커밋 전 충돌 체크: `git -C <main> status --short`로 다른 세션 수정 파일 목록을 뜨고 이번 변경 파일과 교집합 0 확인

### Must NOT Have (guardrails)
- `cmd/harness/updatecli/**` 수정 금지(다른 세션 작업 영역)
- codex 세션(PID 91044) 종료/재시작을 계획이 자동 실행 금지 — 사용자 액션 항목으로만
- mcp proxy 수명주기/churn 수정 금지(다른 세션이 6ee897d 계열로 작업 중 — 범위 밖, 필요 시 별도 사이클)
- stress/실커널 KVA 고갈을 유발하는 테스트 금지(머신 전역 상태를 건드리는 테스트는 비결정적) — 헬퍼 테스트는 큰 출력으로 로직만 검증
- 캡처 헬퍼 외 테스트 로직 리팩토링 금지(surgical)

## 사용자 액션 (계획 밖, 병행 권장)
- codex PID 91044 세션 재시작(또는 종료)으로 파이프 fd 3,112개 회수 — 실행 후 `lsof -n | rg -c PIPE` 재측정으로 총량 하락 확인. 이 액션 없이도 본 계획의 면역화로 테스트는 통과하게 되지만, 머신 전역의 파이프 성능 저하는 남는다.

## Verification Strategy
> 구현+테스트 동일 task. Evidence: `.agent-harness/evidence/pipefix-*.txt`. 전체 스위트는 T4 완료 후에만 실행(그 전에는 행 위험).

## Execution Strategy
- **Wave 1**: T1(docs 커밋) → T2(worktree 준비) — 순차, 이후 전부 worktree에서
- **Wave 2**: T3(공유 헬퍼+테스트) 후 T4(전수 교체) ∥ T5(doctor 체크) ∥ T6(CAUTIONS/ADR 보강)
- **Wave 3**: T7(전체 검증 + 충돌 체크 + 병합 준비)

### Dependency Matrix

| Task | Depends On | Blocks | Can Parallelize With |
|------|-----------|--------|---------------------|
| T1 | — | T2 | — |
| T2 | T1 | T3–T7 | — |
| T3 | T2 | T4 | T5, T6 |
| T4 | T3 | T7 | T5, T6 |
| T5 | T2 | T7 | T3, T4, T6 |
| T6 | T2 | T7 | T3, T4, T5 |
| T7 | T4, T5, T6 | — | — |

## TODOs

- [ ] 1. 미커밋 문서 변경을 즉시 원자 커밋 (main tree, 충돌 창 최소화)

  **What to do**: main 작업 트리에서 `git add .agent-harness/CAUTIONS.md .agent-harness/plans/loop-article-gaps-123.md .agent-harness/plans/phase-aware-pioneer-hints.md .agent-harness/plans/pipe-pressure-and-session-conflict.md` 후 `docs(cautions): record macOS pipe-KVA hang diagnosis and follow-up plans` Conventional Commit + Lore body로 커밋. **updatecli 2개 파일은 절대 stage하지 않는다**(다른 세션 소유). atomic-commit-push 스킬 절차(preflight, 개별 파일 지정 add) 준수.

  **Must NOT do**: `git add -A`/`git add .` 금지. push는 사용자 지시 시.
  **Recommended Agent**: quick
  **References**: `.agent-harness/COMMIT_POLICY.md`, `skills/atomic-commit-push/SKILL.md`
  **Acceptance Criteria**: 커밋 후 `git status --short`에 updatecli 2개 파일만 남음
  **QA**:
  ```
  Scenario: 선택적 스테이징 (happy)
    Channel: bash — git show --stat HEAD
    Expected: 문서 4개 파일만 포함, updatecli 부재
    Evidence: .agent-harness/evidence/pipefix-commit-docs.txt
  Scenario: 타 세션 파일 불간섭 (failure 감시)
    Channel: bash — git status --short
    Expected: ' M cmd/harness/updatecli/...' 2줄이 그대로 남아 있음
    Evidence: 동일 파일
  ```
  **Commit**: YES (이 task 자체가 커밋) | Files: 문서 4종

- [ ] 2. 격리 worktree 브랜치 생성 및 작업 규약 고정

  **What to do**: `git worktree add ../agent-harness-pipefix -b fix/pipe-capture-immunity` (T1 커밋 이후 시점 기준). 이후 T3–T7 전부 이 worktree에서 실행. 착수 시 `git -C /Users/sample/workspace/agent-harness status --short` 스냅샷을 evidence로 남기고, T7 병합 준비 때 재확인해 다른 세션 변경과 파일 교집합 0을 검증한다. 빌드 산출물은 worktree-로컬 `bin/`을 사용(main의 `bin/agent-harness`는 hook이 사용 중이므로 덮어쓰지 않음).

  **Must NOT do**: main tree에서의 코드 편집·golden 재생성·`go build -o bin/agent-harness` 금지.
  **Recommended Agent**: quick
  **References**: `git worktree` 표준 절차; `.agent-harness/AGENT_WORKFLOW.md`의 worktree 계약
  **Acceptance Criteria**: `git worktree list`에 신규 항목; worktree에서 `go build ./...` 성공
  **QA**:
  ```
  Scenario: 격리 확인 (happy)
    Channel: bash — cd ../agent-harness-pipefix && git status --short
    Expected: clean (updatecli 수정은 main tree에만 존재)
    Evidence: .agent-harness/evidence/pipefix-worktree.txt
  Scenario: main 불간섭 (failure 감시)
    Channel: bash — worktree 작업 후 git -C main status
    Expected: main 변경 없음(다른 세션 파일 제외)
    Evidence: 동일 파일
  ```
  **Commit**: NO

- [ ] 3. 공유 캡처 헬퍼 `internal/testsupport` 신설

  **What to do**: `internal/testsupport/capture.go` — `func CaptureStdout(t *testing.T, fn func() error) string`: os.Pipe 생성 → **reader goroutine 먼저 시작**(`io.ReadAll(r)` → chan) → `os.Stdout = w` → fn() → `w.Close()` → stdout 복원 → reader join → 에러 처리(6ee897d의 response_contract_runners_test.go:66-85 순서 그대로). stderr 변형이 필요한 호출부가 있으면 `CaptureStderr`도 동일 패턴으로. 테스트 `capture_test.go`: (a) 64KB 출력(파이프 버퍼 초과)을 5초 데드라인 내 캡처 — 강등 상태 모사 없이도 write-then-read라면 반드시 실패할 크기, (b) fn 에러 전파, (c) stdout 복원 확인.

  **Must NOT do**: production 코드에서 이 패키지 import 금지(테스트 전용 계약을 doc comment로 명시).
  **Recommended Agent**: deep
  **References**: `cmd/harness/harnessapp/response_contract_runners_test.go:66-85`(정확한 복제 원형)
  **Acceptance Criteria**: `go test ./internal/testsupport -count=1 -timeout 30s` 통과(64KB 케이스 포함)
  **QA**:
  ```
  Scenario: 대용량 출력 무블록 (happy)
    Channel: bash — go test ./internal/testsupport -run CaptureLarge -v -timeout 30s
    Expected: 64KB 캡처 PASS, 30s 타임아웃 미발동
    Evidence: .agent-harness/evidence/pipefix-helper-large.txt
  Scenario: fn 에러 전파 (failure)
    Channel: bash — go test ./internal/testsupport -run CaptureError -v
    Expected: fn 에러가 t.Fatalf 경로로 표면화됨을 서브테스트로 검증
    Evidence: .agent-harness/evidence/pipefix-helper-error.txt
  ```
  **Commit**: YES | `test(testsupport): add pipe-buffer-safe stdout capture helper` | Files: `internal/testsupport/{capture.go,capture_test.go}`

- [ ] 4. 취약 캡처 헬퍼 전수 교체

  **What to do**: grep 목록의 write-then-read 헬퍼를 `testsupport.CaptureStdout` 호출로 교체. 대상(현 시점 확인분): `cmd/harness/{loopcli,statecli,policycli(2),projectcli(2),basiccli,qualitycli,commandstep,hookcli(hook_user_prompt/vcsissue/hook_post_tool_use),selfworkflow/{historycompare,candidatescmd},issueopscli(2)}` + 착수 시 `rg -n 'io.ReadAll' $(rg -ln 'os\.Pipe\(\)' --type go)`로 재수집해 신규 발생분 포함. **mcpcli/daemoncli는 착수 시 main tree의 다른 세션 변경과 겹치는지 확인 후, 겹치면 보류 목록으로 이월하고 계획 파일에 기록**. 각 파일은 로컬 헬퍼 함수 삭제 + import 추가의 기계적 diff로 제한. 패키지별 targeted test 즉시 실행.

  **Must NOT do**: updatecli 제외. harnessapp(이미 수정됨)은 testsupport로 옮기지 않음(동작 동일한 중복이지만 다른 세션 활동 영역 — 이월 항목으로만 기록). assertion 로직 변경 금지.
  **Recommended Agent**: deep (12+ 파일 기계 교체, 패키지별 검증)
  **References**: T3 헬퍼; 취약형 원형 `cmd/harness/loopcli/loop_cli_test.go:88-110`
  **Acceptance Criteria**: 교체 패키지 각각 `go test <pkg> -count=1 -timeout 120s` 통과; `rg -c 'os\.Pipe\(\)' cmd/harness/loopcli ...` 교체 대상에서 0
  **QA**:
  ```
  Scenario: 행 재발 방지 회귀 (happy — 핵심)
    Channel: bash — go test ./cmd/harness/loopcli -count=1 -timeout 120s
    Expected: 두 패키지 ok, 120s 내 완료 (교체 전에는 파이프 강등 시 600s 행이던 케이스)
    Evidence: .agent-harness/evidence/pipefix-sweep-core.txt
  Scenario: 잔존 취약 패턴 없음 (failure 감시)
    Channel: bash — 제외 목록 외에서 rg -l 'os\.Pipe\(\)' cmd --type go 결과와 교체 완료 목록 대조
    Expected: 차집합은 보류 목록(문서화된 이월분)과 정확히 일치
    Evidence: .agent-harness/evidence/pipefix-sweep-audit.txt
  ```
  **Commit**: YES | `test(cli): replace write-then-read stdout captures with pipe-safe helper` | Files: 교체된 *_test.go 전부

- [ ] 5. doctor `pipe_capacity` 진단 체크 추가

  **What to do**: `internal/core/doctor/checks.go`에 체크 추가 — os.Pipe 1개 생성, write end nonblocking으로 1KB 단위 write 반복해 실효 용량 측정, 즉시 양단 close. 결과 필드 `pipe_capacity_bytes`; `< 8192`면 warning("system pipe buffer degraded — long-lived host process may be leaking pipes; see CAUTIONS 2026-07-09"). 결정성: 용량 값 자체는 환경 의존이므로 golden에는 필드 존재만 고정(값은 normalize). 테스트: 체크가 필드를 채우고 close 후 fd 누수 없음(lsof 검사 대신 close 에러 검사).

  **Must NOT do**: 파이프 다수 생성 금지(진단이 압력을 가중하면 안 됨 — 1개만). 실패 시 doctor 전체를 fail시키지 않음(warning 전용).
  **Recommended Agent**: quick
  **References**: `internal/core/doctor/checks.go:15`(2026-07-09 loop 체크 추가 diff가 최근 선례), `internal/core/doctor/doctor_test.go`
  **Acceptance Criteria**: `go test ./internal/core/doctor -count=1` 통과; doctor --json 출력에 필드 존재
  **QA**:
  ```
  Scenario: 진단 필드 (happy)
    Channel: bash — HARNESS_STATE_DIR=$(mktemp -d) ./bin/agent-harness doctor --repo . --json | rg pipe_capacity
    Expected: pipe_capacity_bytes 숫자 필드 존재 (현 머신에선 512 관측 예상 → warning 동반)
    Evidence: .agent-harness/evidence/pipefix-doctor.txt
  Scenario: 정상 머신 무경고 (경계)
    Channel: bash — 단위 테스트에서 임계값 주입으로 8192 이상 케이스 검증
    Expected: warning 부재
    Evidence: .agent-harness/evidence/pipefix-doctor-ok.txt
  ```
  **Commit**: YES | `feat(doctor): detect degraded system pipe capacity` | Files: `internal/core/doctor/{checks.go,doctor_test.go}` (+response contract golden 영향 시 T7에서 갱신)

- [ ] 6. CAUTIONS runbook 보강 + ADR 기록

  **What to do**: (1) CAUTIONS 2026-07-09 파이프 항목의 Resolution에 "재발 방지 완료 상태" 갱신 — 공유 헬퍼 경로(`internal/testsupport`)와 doctor 체크를 참조하고, 신규 캡처 테스트는 반드시 testsupport를 쓰라는 규칙 추가. TESTING.md §3에도 같은 규칙 1줄. (2) ADR 신규 엔트리 — 결정: 테스트 면역화(공유 헬퍼) + 관측(doctor) + 사용자 runbook(호스트 재시작) 3층 대응; 기각: 커널 튜닝(권한/이식성), proxy churn 동시 수정(다른 세션 영역), 스트레스 테스트(비결정성), harnessapp 헬퍼 즉시 통합(활동 영역 충돌 — 이월).

  **Recommended Agent**: quick
  **References**: `.agent-harness/CAUTIONS.md` 2026-07-09 항목, `.agent-harness/ADR.md:610` 형식
  **Acceptance Criteria**: `./bin/agent-harness docs --json` 정상; ADR에 기각 대안 3건+
  **QA**:
  ```
  Scenario: 규칙 명문화 (happy)
    Channel: bash — rg -n 'testsupport' .agent-harness/CAUTIONS.md .agent-harness/TESTING.md
    Expected: 두 문서 모두 hit
    Evidence: .agent-harness/evidence/pipefix-docs.txt
  Scenario: docs index 회귀 없음 (failure 감시)
    Channel: bash — ./bin/agent-harness docs --json | head -3
    Expected: ok true
    Evidence: 동일 파일
  ```
  **Commit**: YES | `docs(harness): record pipe-capture immunity policy and ADR` | Files: CAUTIONS/TESTING/ADR

- [ ] 7. 전체 검증 + 충돌 체크 + 병합 준비

  **What to do**: worktree에서 (1) `go test ./... -count=1 -timeout 600s` — 이제 파이프 압력과 무관하게 행 0건이어야 함, (2) `go test -race ./internal/testsupport ./internal/core/doctor -count=1`, (3) doctor golden 영향 시 `-update` 후 additive diff 검수, (4) main tree `git status` 재확인 — 다른 세션 변경 파일과 이번 브랜치 변경 파일 교집합 0 검증(교집합 발생 시 병합 보류·보고), (5) `git rebase origin/main`(또는 main) 후 테스트 재실행, (6) 병합/PR 여부는 사용자에게 보고 후 결정. 완료 보고에 Not-tested(예: 파이프 정상 머신에서의 doctor 경고 부재는 단위 테스트로만 검증) 명시.

  **Recommended Agent**: quick
  **References**: `.agent-harness/TESTING.md` §2/§5, T2의 충돌 체크 규약
  **Acceptance Criteria**: 전체 테스트 600s 내 완료·행 0건; 교집합 0 evidence; rebase 후 재통과
  **QA**:
  ```
  Scenario: 압력 하 전체 스위트 (happy — 최종 증명)
    Channel: bash — 파이프 강등 상태(현 머신 그대로)에서 go test ./... -count=1
    Expected: 모든 패키지 ok, 개별 600s 타임아웃 0건 — 강등 상태에서의 통과가 면역화의 직접 증거
    Evidence: .agent-harness/evidence/pipefix-full-suite.txt
  Scenario: 세션 충돌 0 (failure 감시)
    Channel: bash — comm -12 <(브랜치 변경 파일 sort) <(main 타 세션 변경 파일 sort)
    Expected: 출력 없음
    Evidence: .agent-harness/evidence/pipefix-conflict-check.txt
  ```
  **Commit**: NO (검증 전용; golden 갱신 발생 시 별도 커밋)

## Final Verification Wave
- [ ] F1. 계획 준수: Must NOT Have grep(updatecli 무변경, mcp proxy 코드 무변경, 커널 튜닝 없음)
- [ ] F2. 품질: 교체 diff가 기계적 치환인지 리뷰(assertion 변경 0), testsupport 문서 주석 존재
- [ ] F3. QA evidence 전건 실재 + 특히 pipefix-full-suite.txt의 행 0건
- [ ] F4. 스코프: main tree 불간섭 최종 확인, 이월 목록(mcpcli/daemoncli/harnessapp 통합 여부) 문서화 확인

## Commit Strategy
T1(main, docs) → 이후 worktree 브랜치에서 T3→T4→T5→T6 순 원자 커밋. Conventional Commit + Lore. 병합/push는 T7 보고 후 사용자 결정.

## Success Criteria
- **문제 1**: 현재의 강등된 머신 상태 그대로 전체 테스트가 행 없이 통과(면역화 직접 증명) + doctor가 강등을 경고 + 재발 방지 규칙 명문화
- **문제 2**: main tree에 docs 커밋 외 무간섭, 모든 구현이 격리 브랜치에서 진행, 충돌 체크 evidence 확보
- 사용자 액션(codex 재시작)은 성능 회복용으로 분리되어 계획 성공의 전제조건이 아님
