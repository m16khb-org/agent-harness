package issueopspreparation

import (
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
)

func TestPrepareContractClonesMutableAuthority(t *testing.T) {
	process := &leasecontract.ProcessReceipt{PID: 42, StartedAt: "start", Executable: "/bin/codex"}
	holder := &leasecontract.Actor{Host: "codex", SessionID: "session", SessionProcess: process}
	execution := &leasecontract.Execution{
		Mode: "direct", Lease: leasecontract.Lease{Generation: 1, Status: "active", Holder: holder},
		Completion: &leasecontract.Completion{Verification: []string{"go test ./..."}},
		CompletionHistory: []leasecontract.CompletionHistoryEntry{{
			Generation: 1, Completion: leasecontract.Completion{Verification: []string{"old verification"}},
		}},
		SyncBaseResolution: &leasecontract.SyncBaseResolution{
			Generation: 1, CompletionGeneration: 1, BaseOID: "base", Actor: *holder,
			ConflictFiles: []string{"internal/a.go"}, StartedAt: "start",
		},
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
	resultClone.Execution.CompletionHistory[0].Completion.Verification[0] = "changed"
	snapshotClone.RecordRaw[0] = 'X'
	snapshotClone.Record.BranchPrepare[0] = 'X'
	snapshotClone.Record.Execution.SyncBaseResolution.Actor.SessionProcess.PID = 9
	snapshotClone.Record.Execution.SyncBaseResolution.ConflictFiles[0] = "internal/changed.go"
	snapshotClone.Record.Execution.SyncBaseEvents[0].Mode = "finalize"
	snapshotClone.RootConflict.LifecycleID = "changed"

	if command.Actor.SessionProcess.PID != 42 || result.Execution.Lease.Holder.SessionID != "session" ||
		result.Execution.Completion.Verification[0] != "go test ./..." || result.Execution.CompletionHistory[0].Completion.Verification[0] != "old verification" || string(snapshot.RecordRaw) != "record" ||
		string(snapshot.Record.BranchPrepare) != `{"provider":"github"}` || snapshot.Record.Execution.SyncBaseResolution.Actor.SessionProcess.PID != 42 ||
		snapshot.Record.Execution.SyncBaseResolution.ConflictFiles[0] != "internal/a.go" || snapshot.Record.Execution.SyncBaseEvents[0].Mode != "apply" ||
		snapshot.RootConflict.LifecycleID != "io-other" {
		t.Fatal("a preparation clone mutated its source")
	}
}

func TestImplementerDefaults(t *testing.T) {
	tests := []struct {
		host, model, effort string
		ok                  bool
	}{
		{host: "codex", model: "gpt-5.6-terra", effort: "xhigh", ok: true},
		{host: "claude", model: "claude-sonnet-5", effort: "high", ok: true},
		{host: "omo", model: "openai-codex/gpt-5.6-sol", effort: "max", ok: true},
		{host: "unknown"},
	}
	for _, test := range tests {
		model, effort, ok := ImplementerDefaults(test.host)
		if model != test.model || effort != test.effort || ok != test.ok {
			t.Fatalf("host=%s defaults=(%q,%q,%v)", test.host, model, effort, ok)
		}
	}
}
