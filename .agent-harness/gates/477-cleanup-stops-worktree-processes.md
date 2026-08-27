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
  EVIDENCE: 2026-08-27 12:10-12:18 definitive battery on the final tree (ba5b379d + 39 changed paths): gofmt 0 files; go vet ok; go test ./... 260 ok; go test -race ./... 260 ok; golangci-lint darwin and GOOS=linux 0 findings; go build ok; docs/inspect ok:true. Dogfood: cleanup abandon --preview from an Orca PTY session and an env -i child listed the spike terminal handle and occupant receipts without requester gates; the same preview run inside a terminal bound to the target worktree reported requester_terminal_outside_worktree and requester_occupies_worktree. (The gates check runner itself timed out on this CHECK and matched 'ok' in partial output; this line is the manual proof.)
