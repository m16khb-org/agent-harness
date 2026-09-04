# #285 Turing 검증 보고서 — cleanup status/finish readiness parity

## 범위

- lifecycle: `io-af174a3584fd`
- branch: `285-cleanup-status-finish-parity`
- sealed base: `edbeeb8616bc29f11eb43253e819462e25bf061b`
- target: `228-clean-break-hexagonal-architecture`
- 구현 범위: finish-eligible `cleanup status --merged`가 `CleanupFinish` preview를 유일 readiness oracle로 사용하고 기존 status schema에 결과를 투영한다.
- 비목표: #250 status/resume actor receipt, #292 executable canonicalization, #293 record-backed abandon cleanup, #283 superseded sub-PR cleanup, #291 remote branch delete idempotency.

## TDD 증거

### RED

새 CLI 회귀 테스트를 먼저 추가한 뒤 다음 focused command를 실행했다.

```text
go test ./cmd/issueops/issueopscli/feedbackcleanup -run 'TestRunCleanupStatus' -count=1
```

예상한 이유로 compile RED가 발생했다.

```text
deps.CleanupFinishGit undefined
deps.InspectCleanupProcesses undefined
```

이는 status가 finish oracle의 Git/process 관측을 주입받아 검증할 seam과 orchestration을 아직 갖지 않았음을 고정했다.

### GREEN

최소 구현 후 같은 focused suite가 PASS했다. ordinary blocker의 stable status 의미를 재검토한 뒤 `OK=true`, `Ready=false`로 수정하고 다음 명령을 새로 실행해 PASS를 확인했다.

```text
go test ./cmd/issueops/issueopscli/feedbackcleanup -run 'TestRunCleanupStatus' -count=1
```

covered cases:

- early phase, no artifact, no `--merged`: provider/merge reader call count 0
- unmerged 및 merge readback 실패: command error로 fail-closed
- provider resolve 및 issue snapshot 실패: blocked status로 강등하지 않고 command error 유지
- open issue, base drift, workspace holder: `OK=true`, `Ready=false`
- closed issue: finish preview와 ready/missing parity
- process holder: `workspace_processes_quiescent` missing 및 holder warning 투영
- injected finish Git/process dependency 사용

## Acceptance evidence

| AC | 증거 | 결과 |
|---|---|---|
| AC-01 | `TestRunCleanupStatusSkipsRemoteObservationUntilFinishEligible` | PASS — early/no-artifact/no-merged provider call 0 |
| AC-02 | `TestRunCleanupStatusFailsClosedOnMergedReadbackErrors`, base drift table case | PASS — readback error 유지, drift missing 투영 |
| AC-03 | open issue table case | PASS — `ok=true`, `ready=false`, `missing=[issue_closed]`, cleanup 진행 비추천 |
| AC-04 | closed issue/process-holder table cases | PASS — finish readiness 및 공개 필드/warning projection |
| AC-05 | live `io-d37060eb87f6` read-only dogfood | PASS — `ok=true`, `ready=false`, sole missing `issue_closed`; #248 close/cleanup 미실행 |
| AC-06 | full/race/build 및 schema diff | PASS — DTO field 변경과 finish gate 제거 없음 |

## 최종 검증

다른 repository-wide test/race process가 0임을 확인한 뒤 full과 race를 각각 단독 순차 실행했다.

```text
go test ./... -count=1
```

- PASS, exit 0
- `internal/core/webfetch` PASS (5.729s)

```text
go test -race ./... -count=1
```

- PASS, exit 0
- `internal/core/webfetch` PASS (5.964s)

```text
go build -o bin/issueops ./cmd/issueops
git diff --check
```

- 모두 PASS, exit 0

검증 직렬화 전에는 다른 lifecycle의 full/race와 겹쳐 `internal/core/webfetch.TestRunBenchmarkLiveRunsBaselineComparatorAsBlackBox` child가 정확히 5초에 종료된 사례가 있었다. #285에서 timeout을 수정하지 않았고, 관련 #237 owner의 독립 수정도 가져오지 않았다. 다른 test process가 없는 standalone 재실행으로 최종 full/race 증거를 교체했다.

## Live dogfood

최종 feature binary를 PATH의 `issueops` token으로 실행했다.

```text
issueops cleanup status --id io-d37060eb87f6 --merged --json
issueops cleanup finish --id io-d37060eb87f6 --provider github --preview --json
```

핵심 readback:

```json
{
  "ok": true,
  "ready": false,
  "id": "io-d37060eb87f6",
  "merged": true,
  "missing": ["issue_closed"]
}
```

finish preview도 `missing=["issue_closed"]`를 반환하고 expected not-ready error로 종료했다. 두 명령은 read-only였고 issue #248을 닫거나 cleanup apply를 실행하지 않았다.

## Review evidence

- fresh read-only code reviewer: critical 0, important 0, minor 0, verdict PASS
- reviewer focused/full checks: PASS
- final implementation review는 이 보고서를 포함한 최종 diff fingerprint에 대해 별도로 봉인한다.

## Turing evidence block

- Criteria: AC-01..AC-06
- Evidence: named RED/GREEN tests, standalone full/race, build, diff-check, live provider readback
- Cleanup: temporary test/build artifacts는 tracked diff에 포함하지 않음; post-merge resource cleanup은 typed gates 뒤에만 수행
- Verdict: PASS pending final fingerprint-bound implementation review and publication
