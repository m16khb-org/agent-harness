package issueopscompletion

import (
	"testing"
	"time"
)

func TestApplyCompletionReleasesLeaseAndStampsDoneLedger(t *testing.T) {
	now := time.Date(2026, 8, 2, 0, 0, 0, 123456789, time.UTC)
	holder := Actor{Host: "codex", SessionID: "session", Process: &ProcessReceipt{PID: 7, StartedAt: "start", Executable: "/bin/codex"}}
	snapshot := Snapshot{
		Phase:  "pr",
		Lease:  Lease{Generation: 3, Status: "active", Holder: &holder},
		Ledger: map[string]LedgerEntry{"pr": {Phase: "pr", EnteredAt: "before"}},
	}
	command := Command{
		Generation: 3, Actor: holder, FinalHead: "0123456789012345678901234567890123456789",
		TuringReportPath: "/repo/turing.json", Verification: []string{"go test ./..."},
		RemoteArtifactURL: "https://github.com/example/repo/pull/198",
	}
	if err := ValidateActive(snapshot, command, true); err != nil {
		t.Fatal(err)
	}
	outcome := Apply(snapshot, command, command.TuringReportPath, now)
	want := now.Format(time.RFC3339Nano)
	if outcome.Phase != "done" || outcome.Lease.Status != "released" || outcome.Lease.Holder != nil {
		t.Fatalf("invalid terminal outcome: %+v", outcome)
	}
	if outcome.Completion == nil || outcome.Completion.CompletedAt != want || outcome.Lease.ReleasedAt != want {
		t.Fatalf("completion timestamps differ: %+v", outcome)
	}
	if outcome.Ledger["pr"].CompletedAt != want || outcome.Ledger["done"].EnteredAt != want {
		t.Fatalf("ledger was not stamped atomically: %+v", outcome.Ledger)
	}
}

func TestValidateActiveRejectsForeignHolder(t *testing.T) {
	holder := Actor{Host: "codex", SessionID: "holder", Process: &ProcessReceipt{PID: 7, StartedAt: "start", Executable: "/bin/codex"}}
	err := ValidateActive(Snapshot{Phase: "pr", Lease: Lease{Generation: 3, Status: "active", Holder: &holder}}, Command{
		Generation: 3, Actor: Actor{Host: "claude", SessionID: "foreign", Process: holder.Process},
	}, true)
	if err == nil || CodeOf(err) != DenyAuthority {
		t.Fatalf("foreign holder error=%v code=%q", err, CodeOf(err))
	}
}
