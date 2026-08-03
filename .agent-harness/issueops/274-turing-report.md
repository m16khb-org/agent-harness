# Issue #274 Turing evidence

## Goal

#270 replacement finalize의 durable artifact identity 누락과 실제 #261의
holderless `task=failed`, `dispatch=dispatched` 교착을 최소 경계로 수정한다.
실제 #261은 Orca `reseed → resume → claim`까지 통과해야 완료다.

## Acceptance ledger

| ID | 상태 | 증거 |
|---|---|---|
| T1 | pass | finalize가 새 generation의 artifact identity version, issue/context/prompt digest, binding lease generation을 claimable transition과 함께 저장한다. durable reread와 immediate resume-artifact verification regression이 통과했다. |
| T2 | pass | changed-runtime holderless + terminal 부재 + `TaskLive=false` + task `completed|failed`에서만 stale `dispatch=dispatched`를 core resume/replacement와 outbound reseed inventory가 허용한다. |
| T3 | pass | live task, nonterminal task, ghost/live terminal, active holder negative matrix가 core/outbound에서 통과했다. 독립 리뷰에서 발견한 same-runtime nonterminal 조기 허용도 finalization/reseed 전용 경계에서 차단하고 재리뷰 PASS를 받았다. |
| T4 | blocked | #274 feature binary의 #261 read-only preview가 Orca relay handshake 뒤 `No owning Orca client is connected to the relay`로 실패했다. record mutation은 수행하지 않았다. |
| T5 | pending | #261 Orca reseed/resume/claim 성공 뒤 #270 PR #273과 #274 PR evidence를 갱신한다. |

## RED → GREEN

RED:

```text
TestExecutionReplacementRecoversDeadOwnerAfterOrcaRuntimeRollover
finalize did not persist the resealed generation identity

TestValidateReseedRuntimeRolloverAcceptsSettledTaskWithStaleDispatch
Orca runtime rollover owner is not quiescent: ... task_status=failed dispatch_status=dispatched
```

GREEN:

```text
go test ./internal/core/issueops ./internal/adapter/outbound/issueopslease \
  -run 'ExecutionReplacementRuntimeRolloverSafetyBoundaries|ExecutionReplacementRecoversDeadOwnerAfterOrcaRuntimeRollover|ValidateReseedRuntimeRollover' \
  -count=1
ok agent-harness/internal/core/issueops
ok agent-harness/internal/adapter/outbound/issueopslease
```

독립 리뷰 RED → GREEN:

```text
RED: same runtime + TaskLive=false + task/dispatch=dispatched가
     finalization과 outbound reseed를 통과
GREEN: finalization quiescence와 outbound reseed 경계에서만 fail-closed
       (shared same-runtime ResumeExistingBinding validator는 보존)
review_274_holderless_reseed: PASS
```

## Verification

```text
go test ./internal/core/issueops ./internal/application/issueopslease ./internal/adapter/outbound/issueopslease ./internal/adapter/orca -count=1
PASS

go test ./... -count=1
PASS, exit 0

go test ./internal/core/webfetch \
  -run TestRunBenchmarkLiveRunsBaselineComparatorAsBlackBox -count=5 -v
PASS, 5/5

go test -race ./... -count=1
PASS, exit 0

go vet ./...
PASS, exit 0

go test ./cmd/harness/contractgolden -run Golden -count=1
PASS

go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
PASS

go build -o bin/agent-harness ./cmd/harness
PASS, exit 0

git diff --check
PASS, exit 0
```

`go test ./...`와 `go test -race ./...`를 동시에 실행한 최초 검증에서는
일반 테스트의 webfetch baseline comparator 하위 프로세스 한 건이 5초에
`signal: killed`로 종료됐다. 같은 테스트의 격리 5회는 모두 약 0.15초에
통과했고, race 전체도 exit 0, 일반 전체 테스트의 단독 재실행도 exit 0으로
통과해 병렬 자원 경합으로 판정했다.

## Independent review

- initial verdict: REVISE — same-runtime nonterminal inventory가 runtime equality
  early return으로 finalization/reseed를 통과
- fix: shared resume validator는 유지하고 finalization quiescence와 outbound
  reseed inventory 경계에만 same-runtime live/nonterminal fail-closed 추가
- final verdict: PASS
- reviewer regression checks: gopls, scoped tests, existing-binding resume tests,
  related packages, vet, diff check, secret/legacy/fallback risk scan 모두 통과

## Dogfood observations

- canonical issue: <https://github.com/m16khb/agent-harness/issues/274>
- path-identity follow-up: <https://github.com/m16khb/agent-harness/issues/275>
- Orca CLI executable: `/Users/m16khb/.orca-relay/bin/orca`
- relay handshake succeeds, but no owning Orca client is attached
- post-review feature binary로 #261 reseed inventory를 재호출해 같은
  `Handshake OK ... No owning Orca client is connected to the relay`를 확인했다
- 호출 전후 #261은 generation 2, claimable, binding/digest가 동일해 record
  mutation이 없었다
- a pre-execution non-canonical worktree plus immutable absolute plan path could
  not be atomically relinked; the broken lifecycle was abandoned through the
  supported fingerprinted cleanup and recreated with the same deterministic ID
- #261 preview remains mutation-free until an owning Orca client reconnects

## Publication boundary

Do not mark #274 complete and do not merge #273 until T4 and T5 pass. The repair
implementation may be reviewed and published as a draft PR while the live Orca
acceptance remains explicit and incomplete.
