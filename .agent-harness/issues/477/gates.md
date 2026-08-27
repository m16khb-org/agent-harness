# Gates: 477 cleanup stops worktree processes

- [x] G1: preview lists occupants with receipts and no quiescent block (finish and abandon)
  CHECK: go test ./internal/adapter/issueops -run PreviewListsOccupants -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/issueops	1.842s
- [x] G2: preview fingerprint binds occupant receipts and orca terminal handles
  CHECK: go test ./internal/adapter/issueops -run FingerprintBindsOccupant -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/issueops	0.382s
- [x] G3: apply stops orca terminals then signals only current occupants HUP+TERM then KILL
  CHECK: go test ./internal/adapter/issueops -run StopsOccupants -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/issueops	1.601s
- [x] G4: remaining occupants fail closed with workspace_processes_stop and the step is codec-admitted
  CHECK: go test ./internal/adapter/issueops ./internal/contract/issueops -run WorkspaceProcessesStop -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/issueops	2.081s | ok  	agent-harness/internal/contract/issueops	0.239s
- [x] G5: requester occupancy, requester terminal, unresolved env, and source-checkout gates refuse
  CHECK: go test ./internal/adapter/issueops -run RequesterGates -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/issueops	0.444s
- [x] G6: orca_runtime_ready for bound cycles, selector_not_found means no terminals, abandon gate refuses TaskLive, and TerminalLive unless apply ①′ can reach the terminal
  CHECK: go test ./internal/adapter/issueops -run OrcaTerminalsGates -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/issueops	0.498s
- [x] G7: results report stopped processes and terminals with audit line, and cleanup status projects occupants as warnings
  CHECK: go test ./internal/adapter/issueops ./cmd/harness/issueopscli/feedbackcleanup -run StoppedProcesses -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/issueops	0.378s | ok  	agent-harness/cmd/harness/issueopscli/feedbackcleanup	0.540s
- [x] G8: full battery green (gofmt, vet, test, race, lint, build) and dogfood evidence recorded
  CHECK: go test ./... -count=1
  EXPECT: ok
  EVIDENCE: 2026-08-27 16:10-16:45 최종 트리(9655b334 + 미커밋 변경, daemoncli 백로그 수정 포함) 배터리를 CHECK로 직접 실행: gofmt -l 0 files; go vet ./... ok; go test ./... -count=1 → 260 packages ok (exit 0); go test -race (issueops, orca, contract/issueops, daemoncli, feedbackcleanup) 전부 ok; golangci-lint darwin·GOOS=linux 0 findings; go build -o bin/agent-harness ok; TestResponseContractsGolden ok(골든 무드리프트); 문서 검사기 ok:true(318 docs, violations 0). full-suite를 흔들던 TestRunDaemonAcceptLoopHealthProbeBypassesFullMCPAdmission은 flake가 아니라 커널 unix 백로그(kern.ipc.somaxconn=128) < maxConnections(256)로 129번째 connect가 ECONNREFUSED로 거절되던 부하 종속 결함이었고(GOMAXPROCS=1에서 20/20 RED → 수정 후 20/20 GREEN), dial과 세션 시작 확인을 번갈아 수행하도록 고쳐 제거했다 ([2026-08-27 lesson](../../cautions/lessons/2026-08-27-daemon-accept-loop-burst-dial-backlog.md)). Dogfood(구현 세션 실측): disposable Orca 워크트리에서 preview→apply→부재 확인 2회, 두 번째는 첫 apply가 fail-closed된 뒤 레코드·워크트리가 보존되고 재-preview로 회복되는 경로까지 확인; `orca terminal close --terminal <handle> --json`이 handle 일치·`ptyKilled=true` receipt를 돌려주고, 가시 탭 없는 background 워크트리 터미널은 PTY 종료 뒤에도 `runtime_error`/`tab_not_found`로 경합하는 것을 실측; 요청자 터미널에서 실행한 preview는 `requester_terminal_outside_worktree`와 `requester_occupies_worktree`로 거부됨.
