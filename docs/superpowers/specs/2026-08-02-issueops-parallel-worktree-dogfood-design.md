# IssueOps 병렬 격리 worktree 도그푸드 설계

## 문제

IssueOps는 record별 execution lease, canonical worktree, parent/child delegation, provider-native hierarchy, generation-fenced publication, merge 후 cleanup을 각각 검증하지만, 메인 Codex 세션이 두 delegated child를 실제 별도 agent와 별도 OS process에서 동시에 운영하는 전체 흐름은 현재 `main`에서 재현 가능한 증거로 고정돼 있지 않다. 기존 `TestStartIssueOpsChildConcurrentSiblings`은 한 Go process 안의 goroutine 경합만 다룬다.

## 목표와 완료 조건

- GitHub parent issue 하나와 `[p]` sub-issue 두 개를 만든다.
- parent는 direct-mode canonical worktree에서 계획과 통합을 소유한다.
- 각 child는 parent branch의 같은 base SHA에서 시작하고 서로 다른 lifecycle ID, generation, branch, canonical worktree, Codex process를 소유한다.
- child A는 서로 다른 OS process가 동시에 delegated child를 start해도 모든 parent ref가 보존됨을 검증한다.
- child B는 서로 다른 OS process가 동시에 done child를 accept해도 모든 verdict/evidence가 보존됨을 검증한다.
- 메인 세션은 child 결과를 검토·수락·통합하고 전체/race/build 검증, PR review/merge, child·parent 원격 issue 반영, remote/local branch·worktree·record cleanup을 완료한다.
- 수행 중의 실패와 교정, 경합 결과, cleanup receipt를 tracked 운영 보고서에 남긴다.

## 선택한 설계

### Lifecycle topology

`#221`은 umbrella parent다. 두 provider-native sub-issue는 Wave 1의 `[p]` 작업이며 상호 선행 조건이 없다. parent execution이 계획 commit을 만든 뒤 standalone `git rev-parse HEAD`로 그 SHA를 읽고, 같은 literal SHA를 두 child branch의 sealed base로 사용한다. child branch prepare는 `base_branch=221-issueops-parallel-worktree-dogfood`, 그 exact base SHA, canonical parent worktree를 함께 봉인한다. 각 child agent가 자기 actor receipt로 `issueops execution prepare --mode direct`를 호출해 canonical worktree와 generation을 획득한다. 메인 세션은 child worktree를 수정하지 않고 status/diff/commit evidence만 관찰한다.

### File ownership

- child A 전용: `internal/core/issueops/issueops_delegation_start_process_test.go`, `.agent-harness/operations/2026-08-02-issueops-parallel-worktree-child-start.md`
- child B 전용: `internal/core/issueops/issueops_delegation_accept_process_test.go`, `.agent-harness/operations/2026-08-02-issueops-parallel-worktree-child-accept.md`
- parent 전용: 설계 문서, 구현 계획, `.agent-harness/operations/2026-08-02-issueops-parallel-worktree-dogfood.md`
- production `issueops_delegation.go`는 새 테스트가 현 코드에서 실패하고 원인이 확정될 때만 parent가 범위를 다시 기록한 후 최소 수정한다.

### Cross-process test shape

각 test file은 고유한 환경변수 namespace와 고유 helper test entrypoint를 사용한다. 각 helper는 `ready/worker-0`, `ready/worker-1`처럼 worker별 marker를 쓴 뒤 공통 gate file이 생길 때까지 bounded poll한다. parent test process는 모든 ready marker를 읽은 뒤 gate file을 한 번 생성해 helper들을 release한다. start는 `TestIssueOpsDelegationStartProcessHelper`, accept는 `TestIssueOpsDelegationAcceptProcessHelper` entrypoint를 실행하며 `CombinedOutput`을 보존해 subprocess 오류 원문이 사라지지 않게 한다.

start 테스트는 네 process의 모든 child ID/branch/title이 parent `ChildCycles`에 정확히 한 번 존재하는지 확인한다. accept 테스트는 미리 done으로 만든 네 child를 네 process가 accept한 뒤 각 parent ref의 `accepted` verdict, 고유 evidence, nonempty `ValidatedAt`을 확인한다.

각 child는 test가 현재 코드에서 PASS한 뒤 자기 격리 worktree에서 대상 outer `withIssueOpsLock`만 임시 우회한다. 같은 test를 `-count=20`으로 실행해 nonzero/누락 assertion을 관찰하고 출력과 exit를 보존한다. 즉시 원본을 복원하고 `git diff -- internal/core/issueops/issueops_delegation.go`가 빈 상태임을 증명한 뒤 test-only commit을 만든다. mutation에서도 계속 PASS하면 test는 intended bug를 검출하지 못하므로 child를 완료하지 않는다.

## 두 증거 평면

- Regression plane: readiness barrier, current-code PASS, unlocked-mutation RED, 반복 PASS가 parent ref/verdict invariant를 증명한다.
- Lifecycle plane: provider hierarchy, 서로 다른 actor/generation/worktree, parent accept/integration, PR merge, cleanup residue 0이 운영 도그푸드를 증명한다.

Regression plane 실패는 code/test defect다. Lifecycle plane 실패는 regression 결과를 보존하되 사용자 목표가 미완료이므로 해당 lifecycle만 recover하고 전체 cleanup까지 계속한다. 어느 평면의 실패도 다른 평면의 성공으로 덮지 않는다.

### Integration

각 child는 자기 branch에 test와 tracked 증거 보고서를 atomic commit으로 만들고, parent branch를 base로 하는 draft PR을 생성해 readback한다. parent는 child 결과를 먼저 독립 code review한 뒤 그 PR을 merge하고 exact merge OID를 기록한다. 파일이 분리돼 있으므로 정상 경로의 text conflict는 없어야 한다. 충돌이 생기면 자동으로 한쪽을 선택하지 않고 두 diff와 의도를 비교해 resolution을 기록한다.

### Publication and cleanup

각 child의 Turing 보고서는 해당 child PR 생성 전에 child branch에 commit한다. parent는 child PR의 diff·증거를 독립 review하고 merge한 뒤 child issue completion reflection/close, child cleanup preview/apply, parent accept를 순서대로 수행한다. 두 child가 parent branch에 합쳐지면 parent Turing 보고서를 확정하고 full gate와 implementation review 이후 main 대상 draft PR을 생성·readback하며 parent execution completion을 기록한다. main merge 확인 뒤 parent issue completion reflection/close와 parent cleanup preview/apply를 수행한다. 마지막으로 `git worktree list`, local/remote branch 목록, `issueops list`, GitHub issue/PR readback을 통해 residue 0을 증명한다.

## 대안

1. goroutine 테스트 두 개만 추가: start 경합은 이미 존재하고 별도 process 경계를 검증하지 못해 기각한다.
2. 운영 보고서만 남기기: 재현 가능한 regression guard가 없어 기각한다.
3. 두 테스트를 한 worker가 순차 구현: coordination과 isolated-worktree 경합 자체를 도그푸드하지 못해 기각한다.

## 오류 처리

- ready marker timeout 또는 helper nonzero exit: 모든 marker와 stdout/stderr를 그대로 실패 메시지에 포함하고 같은 명령을 수정 없이 반복하지 않는다.
- child actor/worktree mismatch: mutation을 중단하고 execution status의 exact next command로 reconcile한다.
- child test가 현 코드에서 실패: Hopper식 원인 격리 후 production 변경 범위를 parent decision으로 기록한다.
- child integration conflict: 양쪽 commit을 보존한 채 merge를 중단하고 semantic resolution 뒤 targeted test부터 다시 수행한다.
- PR/merge/cleanup 상태가 모호함: provider와 IssueOps readback으로 reconcile하며 create/delete를 추측 재시도하지 않는다.

## 검증

- child focused: 각 새 test 이름을 `-v -run '^정확한이름$'`으로 실행해 `=== RUN`과 PASS를 확인한다.
- mutation sensitivity: 해당 outer lock을 임시 우회한 worktree에서 `-count=20`이 nonzero이고 ref/verdict 누락 assertion을 보여야 한다. 복원 뒤 production diff는 0이어야 한다.
- package repeat: 복원된 current code에서 두 cross-process test를 `-count=10`으로 반복한다.
- integration: `go test ./internal/core/issueops -count=1`.
- repository: `go test ./... -count=1`, `go test -race ./... -count=1`, `go build -o bin/agent-harness ./cmd/harness`.
- contracts: production/CLI response contract를 변경하지 않으므로 golden 갱신은 하지 않되 기존 전체 suite가 보호한다.
- cleanup: provider/local/IssueOps 네 표면 모두 residue 없음.

## Sub-agent net-positive 기록

- Pattern: `isolated-worktree-work` 및 `task-fan-out-coordination`
- Benefit: 사용자가 명시한 동일 parent 아래 실제 병렬 holder·worktree 운영을 관찰하고, 별도 subprocess barrier가 persistence 경합을 검증한다.
- Tradeoff: 두 execution 준비와 review/integration coordination 비용이 추가된다.
- Fallback: 한 child가 실패하면 다른 child 결과는 보존하고 해당 child만 reject/recover한다.
- Verification: 각 child generation/worktree/commit/test evidence와 parent accept receipt를 대조한다.
- Net-positive rationale: 두 agent가 test race를 만드는 것은 아니지만 병렬 격리 lifecycle 자체가 사용자 요청의 별도 시험 대상이므로 delegation coordination 비용은 accidental implementation speedup이 아니라 required operational experiment다.

## 승인 해석

사용자가 선택지 1인 “회귀 테스트 + 도그푸드 보고서”를 명시적으로 선택했다. 기존 goroutine coverage 발견에 따라 같은 invariant를 OS process 경계로 강화했으며 산출물과 lifecycle 범위는 유지된다.

독립 Brooks review의 최초 verdict는 `revise`였다. 미래 timestamp 동기화, mutation sensitivity 부재, post-plan child base 모호성을 지적했다. ready-marker/gate barrier, unlocked-mutation RED/restore/PASS, regression/lifecycle evidence 분리, exact post-plan parent SHA 봉인을 반영한 뒤 같은 reviewer가 `proceed`를 반환했다.
