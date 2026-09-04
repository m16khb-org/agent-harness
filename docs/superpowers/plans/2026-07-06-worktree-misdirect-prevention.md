# Worktree 오적용(main checkout misdirect) 방지 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** IssueOps 사이클이 격리 워크트리를 보유한 상태에서, 에이전트의 기본 cwd(메인 체크아웃) 때문에 패치가 워크트리 대신 메인 체크아웃에 조용히 적용되는 사고(2026-07-06 sample service #2519 사례)를 재발 방지한다.

**Architecture:** 가드는 `internal/core/lifecycle`(worktree guard), 훅 CLI는 `cmd/issueops/hookcli`. 개선은 기존 차단 정책을 바꾸지 않고 (1) PostToolUse 사후 감지 경고, (2) PreToolUse ask 승격, (3) 세션/프롬프트 힌트, (4) env 인체공학의 4개 레이어를 추가한다. CAUTIONS §21의 교착 방지 결정은 유지한다.

**Tech Stack:** Go 표준 라이브러리. 테스트는 기존 `runHookCapture`/`ISSUEOPS_STATE_DIR` 격리 패턴.

## 사고 원인 분석 (2026-07-06 조사, 증거 기반)

**사고**: Codex 에이전트가 sample service #2519 사이클(worktree `/Users/sample/workspace/service-api.worktrees/2519-test-quality-comprehensive`) 작업 중, 기본 cwd가 메인 체크아웃이라 패치가 메인 체크아웃에 적용됨. 차단도 경고도 없었고 에이전트가 나중에 스스로 발견.

**검증된 인과 사슬** (모두 소스로 확인):
1. **의도 신호 부재**: 강한 가드(`expectedWorktreeGuardBlockReason`)는 `ISSUEOPS_EXPECTED_WORKTREE` env 또는 `--expected-worktree` 플래그가 있어야 작동하는데, 이 env는 세션에 설정되어 있지 않았다. `resolveExpectedWorktree`(`hook_pre_tool_use.go:74-82`)는 **의도적으로** 세션 바인딩을 읽지 않는다(브랜치 가드 없이 읽으면 같은 repo의 무관한 작업을 차단하므로 — 주석에 문서화됨).
2. **바인딩 브랜치 게이트**: repo 단위 세션 바인딩(`issueops-session-*.json`)은 존재하지만 `expectedWorktreeFromSessionBinding`(`lifecycle_worktree_mcp.go:60-73`)은 현재 브랜치==바인딩 브랜치일 때만 적용 — 메인 체크아웃은 다른 브랜치이므로 무시됨. 그리고 이 fallback은 MCP 가드 전용이며 파일 편집 가드에는 연결되지 않음.
3. **의도된 escape hatch가 오적용을 통과시킴**: fallback 가드 `sourceCheckoutWorktreeGuardBlockReason`(`lifecycle_worktree_guard.go:36-46`)은 "현재 브랜치에 활성 사이클 없음 → 소스 체크아웃 편집 허용" — 다른 브랜치의 stuck 사이클이 repo 전체 편집을 교착시키지 않기 위한 **의도된 설계**(코드 주석 + CAUTIONS §21). 메인 체크아웃이 비-사이클 브랜치였으므로 편집 허용.
4. **사후 감지 없음**: PostToolUse는 이 시나리오를 감지하지 않아 조용히 통과. 발견은 에이전트의 우연.

**기각된 가설**: (a) "Codex에 훅이 없다" — 기각: `~/.codex/hooks.json`의 PreToolUse에 `--enforce-worktree` 배선 확인. (b) "가드 버그" — 기각: 가드는 문서화된 설계대로 동작했고, 설계의 사각지대(비-사이클 브랜치 소스 편집 허용 + 의도 신호 부재)가 원인. (c) "상대 경로 해석 버그" — 기각: `resolveHookTargetPath`는 repo 기준으로 올바르게 해석했고, repo 자체가 잘못된 대상(메인 체크아웃)이었다.

**핵심 통찰**: 이 사고는 "잘못된 편집을 차단 못 한 것"이 아니라 **"의도(워크트리 작업)와 실행 위치(메인 체크아웃)의 불일치를 표현할 신호가 없는 것"**이다. 하드 차단으로 풀면 CAUTIONS §21의 교착 함정을 재도입하므로, 감지·확인·상기 레이어로 푼다.

## 설계 결정 (구현자는 재논의하지 않는다)

1. **기존 allow 정책 유지**: 비-사이클 브랜치의 소스 체크아웃 편집은 계속 허용(교착 방지). 하드 block 추가 금지.
2. **PostToolUse 사후 감지(핵심)**: mutating 편집이 소스 체크아웃에 적용됐고, 같은 repo에 implement/ai-slop-clean phase의 linked worktree 사이클이 있으면 additionalContext 경고를 주입한다. 경고는 사이클 ID와 워크트리 경로를 명시한다. PostToolUse는 additionalContext 주입이 가능하다(Stop과 다름).
3. **PreToolUse ask 승격(제한적)**: repo 세션 바인딩이 implement-phase 사이클을 가리키고, mutating target이 소스 체크아웃 내부이며, 같은 상대 경로 파일이 바인딩된 워크트리에도 존재하면 decision을 `ask`로 승격한다(block 아님 — 병렬 main 작업의 오탐은 사용자가 승인으로 통과). 파일 존재 확인은 `os.Stat` 1회/target — 핫패스 예산 내. git/remote 호출 추가 금지(CAUTIONS §21).
4. **힌트 상기**: SessionStart와 UserPromptSubmit에서 repo에 활성 linked-worktree 사이클이 있으면 `expected worktree: <path> — 편집 전 cwd/절대경로 확인` 한 줄을 주입한다.
5. **env 인체공학**: `issueops resume`/`prepare-worktree-tools` 출력에 `export ISSUEOPS_EXPECTED_WORKTREE=<path>` 지시를 포함해 강한 가드를 켜기 쉽게 한다. 자동 설정은 하지 않는다(다른 세션 오차단 위험).
6. 모든 신규 경고/ask 문구는 **작동하는 escape**를 함께 안내한다: 워크트리 경로로 이동 또는 `issueops force-release --id <id> --reason <why>` (CAUTIONS §21의 non-working-escape 함정 방지).

## Out of Scope

- 세션 ID 단위 바인딩(호스트별 session_id 파싱은 hookinput에 없음 — 별도 조사 후 후속 계획).
- 가드의 하드 차단 정책 변경, stale 사이클 분류 로직 변경.
- Codex apply_patch의 상대 경로 자체를 금지하는 정책.

## File Structure

| 파일 | 변경 |
|---|---|
| `internal/core/lifecycle/lifecycle_worktree_guard.go` | ask 승격 판정 `sourceCheckoutMirrorEditAskReason` 추가 |
| `internal/core/lifecycle/lifecycle_state.go` | PreToolUse 파이프라인에 ask 판정 연결 |
| `internal/core/lifecycle/lifecycle_worktree_misdirect.go` (신규) | PostToolUse 감지 `SourceCheckoutMisdirectWarning` |
| `cmd/issueops/hookcli/hook_lifecycle.go` | PostToolUse 경고 주입 |
| `cmd/issueops/hookcli/hook_session.go` 또는 session-start 담당 파일 | 세션 시작 힌트 (grep으로 확인) |
| `internal/core/hookprompt/hook_prompt.go` | UserPromptSubmit 워크트리 상기 힌트 |
| `internal/core/issueops/` resume/prepare 응답 | env 지시 추가 |

---

### Task 1: PostToolUse 사후 감지 경고 (silent failure 제거 — 최우선)

**Files:**
- Create: `internal/core/lifecycle/lifecycle_worktree_misdirect.go`
- Modify: `cmd/issueops/hookcli/hook_lifecycle.go` (runHookPostToolUse)
- Test: `internal/core/lifecycle/lifecycle_worktree_misdirect_test.go`, `cmd/issueops/hookcli/hook_lifecycle_test.go`(있으면; 없으면 신규)

**Produces:** `SourceCheckoutMisdirectWarning(req HookToolUseLifecycleRequest) string` — 경고 필요 시 사이클 ID/워크트리 경로를 담은 한 줄, 아니면 "".

- [ ] **Step 1**: 실패 테스트 작성 — 시나리오: `ISSUEOPS_STATE_DIR` 격리, 임시 repo에 implement-phase 사이클 + linked worktree 기록(`issueops.WriteIssueOps` 테스트 헬퍼 패턴은 `lifecycle_worktree_guard_state_test.go` 참조), `req.Tool="apply_patch"`, `req.Paths=[<repo>/src/a.ts]` → 경고 문자열에 사이클 ID·워크트리 경로·"의도한 대상인지 확인" 포함. 반대 케이스: linked 사이클 없음 → "", target이 워크트리 내부 → "", 도구가 비-mutating → "".
- [ ] **Step 2**: 실패 확인 (`go test ./internal/core/lifecycle/ -run Misdirect -v`).
- [ ] **Step 3**: 구현 — 판정 순서: `toolUseMayMutateLifecycleFiles` → `worktreeGuardEditTargets` → target이 `cleanAbsPath(req.Repo)` 내부이고 `IsInsideWorktreesPath` 아님 → `ActiveIssueOpsLinkedWorktreeCyclesForRepo`에서 `IssueOpsPhaseExpectsWorktree(phase)`인 사이클 존재 → 경고. 경고 문구는 escape 포함: `"편집이 소스 체크아웃 <repo>에 적용되었습니다. 활성 IssueOps 사이클 <id>가 워크트리 <path>를 보유 중입니다. 이 편집이 사이클 작업이면 워크트리에서 다시 적용하고 소스 체크아웃 변경을 되돌리세요; 무관한 작업이면 무시하세요."`
- [ ] **Step 4**: `runHookPostToolUse`에서 경고를 additionalContext로 주입 — 기존 PostToolUse 출력 조립부를 읽고(`ho.FormatContext` 사용 여부 grep) 같은 채널에 덧붙인다. Codex/Claude 두 호스트 계약 테스트로 JSON 스키마 유효성 확인.
- [ ] **Step 5**: `go test ./internal/core/lifecycle/... ./cmd/issueops/hookcli/... -count=1` 통과 → 커밋 `feat(lifecycle): warn when edits land in source checkout during worktree cycles`.

---

### Task 2: PreToolUse ask 승격 (미러 파일 편집 확인)

**Files:**
- Modify: `internal/core/lifecycle/lifecycle_worktree_guard.go`, `internal/core/lifecycle/lifecycle_state.go`
- Test: `internal/core/lifecycle/lifecycle_worktree_guard_test.go` 계열

**Produces:** `sourceCheckoutMirrorEditAskReason(req) (decision, reason string)` — decision `"ask"` 또는 `""`.

- [ ] **Step 1**: 실패 테스트 — repo 세션 바인딩(cycle, branch, expected worktree)을 `session.Bind` 테스트 스토어로 기록, 워크트리에 `src/a.ts` 실재, req target `<repo>/src/a.ts` → decision `ask` + reason에 워크트리 경로와 escape. 반례: 워크트리에 같은 상대 경로 파일 없음(신규 파일, main 전용 파일) → allow / 바인딩 없음 → allow / 바인딩 사이클 phase가 implement 계열 아님 → allow.
- [ ] **Step 2**: 실패 확인.
- [ ] **Step 3**: 구현 — `BuildLifecyclePreToolUseDecision`(`lifecycle_state.go:12-74`)의 EnforceWorktree 블록 뒤, block이 아닐 때만 ask 판정 실행. 판정: `readIssueOpsSession(repo)` → 바인딩 사이클 레코드 read → `IssueOpsPhaseExpectsWorktree` → 각 target의 repo-상대 경로를 `filepath.Rel`로 구해 워크트리에서 `os.Stat` — 존재하면 ask. **branch 게이트를 걸지 않는 것이 이 태스크의 핵심**(오적용은 언제나 다른 브랜치에서 발생) — 대신 미러 파일 존재 조건이 오탐을 줄인다. `HookPreToolUseDecisionResult.Decision`에 `"ask"`를 넣었을 때 두 호스트 어댑터가 유효한 permission decision JSON을 내는지 계약 테스트로 고정(기존 `commandguard.StagedCheckDecision`이 decision 문자열을 반환하는 선례를 grep으로 확인해 같은 경로를 쓴다).
- [ ] **Step 4**: 전체 테스트 → 커밋 `feat(lifecycle): ask before mirror-file edits in source checkout`.

---

### Task 3: 세션/프롬프트 워크트리 상기 힌트

**Files:**
- Modify: `internal/core/hookprompt/hook_prompt.go` (UserPromptSubmit 힌트), session-start 컨텍스트 조립부(grep `session-start`로 확인)
- Test: `internal/core/hookprompt/hook_prompt_test.go`

- [ ] **Step 1**: 실패 테스트 — repo에 linked worktree 사이클 존재 시 `BuildUserPromptMCPHints` 결과에 `worktree: <path> 편집 전 cwd/절대경로 확인` 힌트 포함; 사이클 없으면 미포함.
- [ ] **Step 2**: 구현 — 힌트는 `ActiveIssueOpsLinkedWorktreeCyclesForRepo` 1회 조회(이미 상태 파일 read 수준, 핫패스 예산 확인: 기존 호출부가 PreToolUse에서 매번 수행하는 것과 동일 비용). 사이클 2개 이상이면 개수만 표기하고 대표 1개 + "외 N개".
- [ ] **Step 3**: 테스트 통과 → 커밋 `feat(hookprompt): remind expected worktree on prompt and session start`.

---

### Task 4: env 인체공학 + 문서화

**Files:**
- Modify: `issueops resume`/`prepare-worktree-tools` 응답 조립부 (`grep -rn "prepare_worktree_tools\|issueops_resume" internal/core/issueops/`로 확인)
- Modify: `.issueops/CAUTIONS.md` (신규 섹션), `.issueops/AGENT_WORKFLOW.md`

- [ ] **Step 1**: resume/prepare 응답에 `guidance: export ISSUEOPS_EXPECTED_WORKTREE=<worktree>` 필드 추가 + 테스트.
- [ ] **Step 2**: CAUTIONS에 이번 사고를 기록: "워크트리 사이클 중 기본 cwd 오적용 — 가드는 비-사이클 브랜치 소스 편집을 의도적으로 허용하므로(§21 교착 방지), 워크트리 작업 세션은 ISSUEOPS_EXPECTED_WORKTREE를 설정하고, 패치는 절대 경로 또는 `git -C <worktree>`로 적용한다. 감지 레이어: PostToolUse 경고 / PreToolUse ask / 프롬프트 힌트."
- [ ] **Step 3**: 커밋 `docs(cautions): record worktree misdirect incident and guards`.

---

### Task 5: 통합 검증

- [ ] `go build ./... && go test ./... -count=1` 클린.
- [ ] 재현 시나리오 E2E 테스트: 임시 repo + worktree + 사이클 기록 후 (a) PreToolUse: 미러 파일 편집 → ask, (b) PostToolUse: 소스 체크아웃 편집 → 경고 주입, (c) 비-미러 신규 파일 main 편집 → allow·무경고 (교착 방지 회귀 가드).
- [ ] `go build -o bin/issueops ./cmd/issueops` 재빌드.
- [ ] 커밋 `test(hooks): e2e guard coverage for worktree misdirect scenario`.

## Self-Review

- 인과 사슬의 각 고리(신호 부재→브랜치 게이트→escape hatch→사후 감지 부재)에 대응하는 태스크: 신호(T4), 게이트 보완(T2), escape hatch 유지+감지(T1), 상기(T3).
- CAUTIONS §21 준수: 하드 block 추가 없음, 핫패스에 git/remote 호출 없음(os.Stat만), 모든 문구에 작동하는 escape 포함.
- 미확인 지점은 각 태스크에 grep 확인 단계로 명시(PostToolUse 출력 채널, ask decision 어댑터 지원, session-start 조립부, resume 응답 구조).
