# Incident → Hook 매핑 (Hashimoto 원칙)

품질향상 프로그램 v2 **B2**. 운영 원칙: *관찰된 실패(incident)는 가능한 한 결정적 hook/테스트로 전환해, "다시는 일어날 수 없게" 강제한다.* `CAUTIONS.md`는 실수를 **기록**하지만, 기록만으로는 재발을 막지 못한다. 이 표는 각 incident가 **어떤 결정적 강제장치(hook flag 또는 Go 테스트)로 전환됐는지** 매핑한다.

- 전환됨(✅): 그 실수를 결정적으로 차단/탐지하는 hook 또는 테스트가 존재한다(증거 = 파일/플래그).
- 미전환(○): 아직 문서 규약만 있고 기계 강제장치가 없다 → 후속 전환 후보.

최종 갱신: **2026-07-02**

| # | Incident (출처) | 관찰된 실패 | 결정적 강제장치 (hook/test) | 상태 |
|---|------------------|-------------|------------------------------|------|
| 1 | CAUTIONS §15 IssueOps worktree edits | issue 작업이 worktree 밖(소스 repo)에서 편집 | PreToolUse `--enforce-worktree` (`internal/adapter/lifecycle/lifecycle_state.go`; install: claude/codex `install_hooks.go`) | ✅ hook |
| 2 | CAUTIONS §16 decision replies must have numbered choices | 결정 응답이 번호 선택지/추천 1개 없이 종료 | Stop `--enforce-numbered-next-actions` (`internal/core/nextaction/`) | ✅ hook |
| 3 | CAUTIONS §4 Secret leakage | hook 실패 로그에 secret 원문 기록 | hook-failure-log 리댁션, `TestRunHookRecordsFailureEvent`가 "leaked secret"을 assert (`cmd/harness/hookcli/hook_post_tool_use_test.go`) | ✅ test |
| 4 | CAUTIONS §14 Codex vs Claude hook rendering drift | host별 hook 명령/JSON 형식 drift | native 설치 계약 golden `TestNativeInstallAdapterContractMatrix` (`internal/adapter/testdata/native_install_contract_matrix.golden.json`) | ✅ golden test |
| 5 | CAUTIONS §7 MCP schema drift / §18 audit flags | CLI/MCP 응답 계약·docs 인덱스 drift | response-contract golden `TestResponseContractsGolden` (`cmd/harness/harnessapp/response_contract_golden_test.go`) | ✅ golden test |
| 6 | CAUTIONS §10 자기 검증/자가 증강 drift | augment 후보를 모델 자기비판으로 "검증"(외부 신호 없이) | `qualitycatalog.VerifyWithGrounded` + 양 카탈로그 grounding 테스트 (B1, 2026-06-15) | ✅ test |
| 7 | (신규 2026-06-15) 편집 후 미포맷 Go | edit가 gofmt 미통과 Go를 남김 | PostToolUse 결정적 lint-as-gate `internal/adapter/hook/lintgate.go` + `TestRunHookPostToolUse*` (B3) | ✅ hook+test |
| 8 | (신규 2026-06-15) untracked .md가 docs-index golden을 drift | llm-wiki 훅의 untracked 연구문서가 self-verify golden을 간헐 flake | hermetic docs-index(`ListDocs` git-tracked 필터) + `TestListDocsExcludesUntrackedInGitRepo` (env-fix) | ✅ test |
| 9 | CAUTIONS §19 git identity before contributor-sensitive pushes | 잘못된 git identity로 기여자-민감 push | preflight git 점검(`internal/adapter/preflight/git.go`) — 단, push 차단까지의 강제는 부분적 | ○ 부분 |
| 10 | CAUTIONS §5 Worker lifecycle 문제 (stuck running) | 프로세스 크래시 후 worker가 running으로 고착 | `MaybeDetectStuckWorkerJobs` SessionStart 자동 트리거(W1, A2) | ✅ hook |
| 11 | plan-before-execute (S2 원칙) | 승인된 설계 검토 없이 implement(구현) 시작 | issueops implement 진입 게이트: `IssueOpsImplementationReadiness`→`issueOpsDesignReviewMissing`가 `design_review`/`design_approval` 요구(`AdvanceIssueOpsPhase` implement 차단); `TestAdvanceToImplementGatesOnDesignApproval`(B4) | ✅ gate+test |
| 12 | CP1 command policy override | 첫 workspace root의 policy catalog가 같은 프로세스의 다른 repo 평가로 새는 문제 | `.agent-harness/policy.json` per-evaluation load + two-root isolation tests (`internal/adapter/policy/policy_catalog_test.go`, `cmd/harness/policycli/policy_cli_test.go`, `cmd/harness/mcpcli/mcp_tool_policy_state_test.go`) | ✅ test |
| 13 | IssueOps record format drift | legacy/future `IssueOpsRecord` JSON을 같은 방식으로 읽어 상태를 잘못 해석 | `schema_version` normalization/future-version rejection (`internal/core/issueops/issueops_schema_version_test.go`) | ✅ test |
| 14 | docs-index content churn | 문서 본문/목록 변화가 response contract golden을 과민하게 깨뜨림 | docs-index projection golden (`cmd/harness/harnessapp/response_contract_docs_projection_test.go`) keeps schema/count/required docs while CLI/MCP schema remains fixed | ✅ golden test |

## 종결 판정 (B2 수용기준)

- **incident→hook 매핑 테이블 1개**: 본 문서. ✅
- **기존 CAUTIONS 항목 ≥3건이 hook/테스트로 전환됐는지 표기**: §15·§16·§4·§14·§7·§10·§5·CP1·IssueOps schema·docs-index 등 **10건 이상 ✅ 전환** 표기(증거 파일 명시). 미전환(○) §19와 Slack List schema live-write test는 후속 후보로 유지. ✅

## 갱신 규약

- 새 CAUTIONS 항목 추가 시: 가능하면 동시에 결정적 hook/테스트로 전환하고 본 표에 1행 추가한다(✅). 즉시 전환이 불가하면 ○로 기록해 후속 후보로 남긴다.
- 강제장치(파일/플래그/테스트명)가 바뀌면 해당 행의 증거 참조를 갱신한다.
- 외부 ref: Hashimoto "every mistake → engineer it can never happen again"; 하네스의 hook 아키텍처가 이를 실현.
