---
name: 2026-08-28-lease-quiescence-lsof
description: Caution record for a solved false case or recurring risk.
---

# lease quiescence 테스트가 시스템 전역 lsof 프로브에 묶여 전체 스위트에서 확률적으로 깨졌다

- Date: 2026-08-28
- Kind: `caution`
- Source: Claude Code session 2026-08-28 — systematic-debugging + TDD on full-suite flaky lease test
- Summary: executionQuiescenceFingerprint는 워크스페이스 점유를 판정하려고 시스템 전역 lsof를 3초 상한으로 실행하는데 ExecutionReplaceDependencies에 그 관측을 대체할 시임이 없어, lease 상태 기계만 검증하는 테스트가 호스트의 열린 파일 목록에 묶여 전체 스위트 부하에서 context deadline exceeded로 깨졌다.
- Context: go test ./... 전체 실행에서 TestExecutionReplacementRecoversDeadOwnerAfterOrcaRuntimeRollover가 `execution_lease_rollover_test.go:83: preview dead owner rollover finalization: context deadline exceeded`로 실패했다. 단독 재실행과 2차 전체 스위트에서는 통과해 확률적 실패였다. 테스트는 context.Background()를 넘기므로 데드라인은 테스트가 아니라 프로브 내부의 nativeProcessProbeTimeout(3초)에서 온 것이고, ctx.Err()가 wrap 없이 그대로 올라와 어느 프로브가 터졌는지 메시지만으로는 구분되지 않았다. 실패가 finalize preview에서만 나고 앞선 preview/revoke는 통과한 점이 quiescence 경로 고유의 프로브(inspectWorkspaceProcesses)를 가리켰다. 이는 .issueops/testing/concurrency-and-race.md의 결정성 계약("테스트는 deterministic해야 한다 … local machine state에 의존하지 않는다") 위반이다. 같은 노출이 TestFinalizeReleasesInsteadOfClaimableWhenTheWorkspaceIsGone에도 있었다.
- Resolution: 타임아웃을 늘리는 증상 완화 대신 결정성 결여를 고쳤다. ExecutionReplaceDependencies에 비공개 시임 inspectWorkspace(executionWorkspaceProcessInspector)를 추가하고 deps.workspaceInspector()가 nil이면 기존 inspectWorkspaceProcesses를 반환하므로 프로덕션 경로의 동작은 그대로다. lease 상태 기계만 검증하는 두 테스트는 테스트 헬퍼 quiescentWorkspaceInspector()를 주입해 시스템 lsof에서 분리했고, 시임이 실제로 쓰이는지 고정하는 TestExecutionFinalizePreviewUsesInjectedWorkspaceInspector를 추가했다. finalize 경로가 lsof에 도달하지 않는 서브테스트(TaskLive로 먼저 막히는 same-runtime 케이스)는 건드리지 않았다.
- Evidence:
  - lsof -nPw -Fpcfna 소요 측정 — 유휴 0.32~0.42초, go test ./... 부하 중 40회 샘플링 최대 1.17초(상한 3초)
  - 인과 실험 — inspectWorkspaceProcesses의 lsof 상한만 time.Millisecond로 굶자 라인 번호까지 동일한 재현: execution_lease_rollover_test.go:83: preview dead owner rollover finalization: context deadline exceeded
  - RED — 시임 필드를 추가하고 배선하지 않은 상태에서 신규 테스트 실패: injected workspace occupancy did not block finalization: <nil>
  - GREEN — 배선 후 go test ./internal/adapter/issueops/ -count=1 통과(191s)
  - 검증 — lsof 상한 1ms로 굶긴 상태에서 세 테스트 -count=3 모두 통과(수정 전엔 확정 실패하던 조건)
  - go test ./... -count=1 2회 연속 실패 0건, gofmt/vet 깨끗, issueops docs --json / inspect --json OK
