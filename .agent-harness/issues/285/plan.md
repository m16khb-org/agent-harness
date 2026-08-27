---
title: "#285 cleanup status/finish parity"
issue: "https://github.com/m16khb/agent-harness/issues/285"
lifecycle: "io-af174a3584fd"
branch: "285-cleanup-status-finish-parity"
base_branch: "228-clean-break-hexagonal-architecture"
base_sha: "edbeeb8616bc29f11eb43253e819462e25bf061b"
worktree: "/Users/m16khb/Workspace/agent-harness.worktrees/285-cleanup-status-finish-parity"
status: approved-design-execution-plan
---

# 목표

`cleanup status --merged`가 동일 시점에 구성한 `CleanupFinishRequest`를 `CleanupFinish(..., Apply=false)`에 전달하고 그 결과를 기계적으로 투영하게 해, `cleanup finish --preview`와 readiness/missing 판단이 어긋나지 않게 한다.

# 불변식과 범위

- `cleanupstatus.ForRecord`의 early phase/no-artifact/no-`--merged` 구조 진단과 zero provider call을 보존한다.
- finish-eligible status에 explicit `--merged`가 있을 때만 remote artifact/issue를 읽는다.
- merge head와 base는 `VerifyMergedHead` 한 readback에서 얻는다. 실패·unmerged·base drift는 fail-closed한다.
- `CleanupFinishRequest`는 finish CLI와 같은 필드로 만들고 `CleanupFinish` preview를 유일 readiness oracle로 호출한다.
- `IssueOpsCleanupStatus` JSON schema는 바꾸지 않는다. `OK`, `ID`, `Ready`, `Merged`, `Missing`, `WorktreePath`, `Branch`, `RemoteArtifactURL`을 전부 명시적으로 투영하고 process holder는 `Warnings`로 옮긴다. status 조회가 성공한 ordinary blocker는 기존 계약대로 `OK=true`, `Ready=false`이며 choices는 기존 3-choice helper만 사용한다.
- ordinary blocked preview는 result ID가 요청 ID와 같고 `Missing`이 비어 있지 않을 때만 `OK=true`, `Ready=false` status로 정규화한다. provider/record/merge/issue/cwd/malformed 오류는 error로 유지한다.
- finish gate는 제거·완화하지 않는다. #283과 #291은 비목표다. live dogfood는 #248을 닫거나 cleanup하지 않는다.

# 구현 순서

## Task 1 — 기존/early 경계 characterization

Files:

- `cmd/harness/issueopscli/issueops_cleanup_cli_test.go`
- 필요하면 `internal/core/issueops/cleanupstatus/cleanup_status_test.go`

RED/verification:

- early phase, no remote artifact, no `--merged`에서 injected provider/merge reader 호출 수가 0임을 고정한다.
- 기존 missing/choices projection이 유지됨을 고정한다.
- 테스트 이름을 `-v -run`으로 실제 실행해 baseline/RED를 구분한다.

## Task 2 — finish preview parity orchestration

Files:

- `cmd/harness/issueopscli/feedbackcleanup/feedback_cleanup.go`
- 필요한 최소 test seam이 현재 `Deps`로 부족할 때만 같은 package의 작은 helper/dependency field

RED:

- explicit merged + VerifyMergedHead unmerged/readback failure는 error다.
- merged base가 prepared base와 다르면 `base_branch_drifted`가 status에 보인다.
- open issue는 `issue_closed`, `ok=true`, `ready=false`를 보고하고 cleanup 추천 choice를 만들지 않는다.
- closed issue는 finish preview와 `Ready`/`Missing` 및 공개 field projection이 동일하다.
- process holder는 차단 사유가 아니라 apply가 종료할 대상이다(#477). status는 `Ready`를 유지하되 `pid:command:started_at` 문자열과 "apply가 프로세스 N개와 Orca 터미널 M개를 종료합니다" warning으로 투영한다. 관측 실패(`workspace_processes_observable`)와 요청자 점유(`requester_occupies_worktree`)만 missing으로 남는다.
- fake git/process dependency로 실제 repo/process를 건드리지 않는다.

GREEN:

1. `cleanupstatus.ForRecord`로 structural applicability를 먼저 계산한다.
2. early/no-artifact/no-`--merged`면 즉시 기존 결과를 반환한다.
3. finish-eligible explicit merged면 `VerifyMergedHead`와 linked issue snapshot을 읽는다.
4. finish CLI와 같은 request를 만들어 `CleanupFinish(... Apply=false)`를 호출한다.
5. finish result를 schema-stable status로 전부 투영하고 기존 choice helper를 적용한다.
6. 동일 ID + non-empty Missing의 ordinary blocked preview만 status result로 정규화한다.

## Task 3 — 회귀·live dogfood·quality gate

Focused verification:

```bash
go test -v ./cmd/harness/issueopscli -run 'CleanupStatus|CleanupFinish' -count=1
go test -v ./internal/core/issueops/... -run 'CleanupStatus|CleanupFinish' -count=1
```

Full verification:

```bash
go test ./... -count=1
go test -race ./... -count=1
go build -o bin/agent-harness ./cmd/harness
```

Live read-only dogfood after building the feature binary:

```bash
agent-harness issueops cleanup status --id io-d37060eb87f6 --merged --json
agent-harness issueops cleanup finish --id io-d37060eb87f6 --provider github --preview --json
```

Expected: both report `issue_closed`; status is `ready=false` and does not recommend cleanup. Do not close #248 and do not apply cleanup.

# Acceptance mapping

| AC | Binary evidence |
|---|---|
| AC-01 | early/no-artifact/no-`--merged` tests pass with zero provider calls |
| AC-02 | unmerged/readback/base drift negative tests pass fail-closed |
| AC-03 | open issue test returns `issue_closed`, `ready=false`, blocked choice |
| AC-04 | closed issue projection table and process warning tests pass |
| AC-05 | live `io-d37060eb87f6` status/finish outputs both contain `issue_closed`; #248 remains open |
| AC-06 | contract/golden and full/race/build pass with no schema or finish-gate removal |

# Compatibility review inputs

- Backward compatibility: no JSON fields are added/removed; non-applicable status paths keep current local-only behavior. Only currently false-positive `ready=true` merged cleanup changes to truthful blocked status.
- Side effects: explicit merged finish-eligible status performs the same provider issue/merge reads as finish preview and local injected git/process observations; early/no-merged paths remain offline.
- Rollback: revert the single behavior commit; no schema/data migration or persisted-state rewrite is introduced.
- Verification: focused RED/GREEN tests, full/race/build, contract golden if affected, and live read-only dogfood.
- Blockers: none. Brooks PASS is already recorded; approved design has no open questions.

# Completion and cleanup boundary

- Commit one atomic behavior+tests intent using Conventional subject + Lore body, push without force, and open a draft PR into `228-clean-break-hexagonal-architecture`.
- Record Orca implementation review PASS against the final diff fingerprint before publication.
- `execution complete` records done/released only after verified draft PR and committed Turing report.
- Merge, issue close, remote branch deletion, worktree/local branch/record cleanup are separate typed post-merge steps. Execute only after remote readback gates pass.

# Sub-agent decision

Implementation and TDD stay with the main owner because the change is cross-cutting across one CLI flow and its shared finish oracle. A fresh planner-grade adversarial reviewer is net-positive only at the mandatory implementation-review gate (`devils-advocate-review`); no implementation fan-out is planned.
