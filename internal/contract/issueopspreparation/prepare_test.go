package issueopspreparation

import (
	"testing"

	leasecontract "agent-harness/internal/contract/issueopslease"
)

func TestPrepareContractClonesMutableAuthority(t *testing.T) {
	process := &leasecontract.ProcessReceipt{PID: 42, StartedAt: "start", Executable: "/bin/codex"}
	holder := &leasecontract.Actor{Host: "codex", SessionID: "session", SessionProcess: process}
	execution := &leasecontract.Execution{
		Mode: "direct", Lease: leasecontract.Lease{Generation: 1, Status: "active", Holder: holder},
		Completion:     &leasecontract.Completion{Verification: []string{"go test ./..."}},
		SyncBaseEvents: []leasecontract.SyncBaseEvent{{Mode: "apply", BaseBranch: "main"}},
	}
	command := Command{ID: "io-prepare", Actor: *holder}
	result := Result{ID: command.ID, Execution: execution}
	snapshot := Snapshot{
		Record:    leasecontract.Record{ID: command.ID, BranchPrepare: []byte(`{"provider":"github"}`), Execution: execution},
		RecordRaw: []byte("record"), CanonicalRoot: "/repo.worktrees/prepare",
		RootConflict: &RootClaim{LifecycleID: "io-other", Root: "/repo.worktrees/prepare"},
	}

	commandClone := command.Clone()
	resultClone := result.Clone()
	snapshotClone := snapshot.Clone()
	commandClone.Actor.SessionProcess.PID = 7
	resultClone.Execution.Lease.Holder.SessionID = "changed"
	resultClone.Execution.Completion.Verification[0] = "changed"
	snapshotClone.RecordRaw[0] = 'X'
	snapshotClone.Record.BranchPrepare[0] = 'X'
	snapshotClone.Record.Execution.SyncBaseEvents[0].Mode = "finalize"
	snapshotClone.RootConflict.LifecycleID = "changed"

	if command.Actor.SessionProcess.PID != 42 || result.Execution.Lease.Holder.SessionID != "session" ||
		result.Execution.Completion.Verification[0] != "go test ./..." || string(snapshot.RecordRaw) != "record" ||
		string(snapshot.Record.BranchPrepare) != `{"provider":"github"}` || snapshot.Record.Execution.SyncBaseEvents[0].Mode != "apply" ||
		snapshot.RootConflict.LifecycleID != "io-other" {
		t.Fatal("a preparation clone mutated its source")
	}
}
