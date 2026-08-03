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
	if outcome.Completion == nil || outcome.Completion.Generation != command.Generation || outcome.Completion.CompletedAt != want || outcome.Lease.ReleasedAt != want {
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

func TestApplyCompletionClearsCompletedReseedStaleNotesFromPRAndDone(t *testing.T) {
	now := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	holder := Actor{Host: "codex", SessionID: "session", Process: &ProcessReceipt{PID: 7, StartedAt: "start", Executable: "/bin/codex"}}
	stale := "stale: completed execution reseed (4 -> 5)"
	snapshot := Snapshot{
		Phase: "pr",
		Lease: Lease{Generation: 5, Status: "active", Holder: &holder},
		Ledger: map[string]LedgerEntry{
			"pr":   {Phase: "pr", EnteredAt: "old-pr", Notes: []string{"keep pr", stale}},
			"done": {Phase: "done", EnteredAt: "old-done", Notes: []string{stale, "keep done"}},
		},
	}
	command := Command{Generation: 5, Actor: holder, FinalHead: "ff27b34520e4e253d8ebfd523e4e4352bf93e8d8", TuringReportPath: "/repo/turing.json", Verification: []string{"new verification"}, RemoteArtifactURL: "https://github.com/example/repo/pull/304"}
	outcome := Apply(snapshot, command, command.TuringReportPath, now)
	if got := outcome.Ledger["pr"].Notes; len(got) != 1 || got[0] != "keep pr" {
		t.Fatalf("pr notes=%v", got)
	}
	if got := outcome.Ledger["done"].Notes; len(got) != 1 || got[0] != "keep done" {
		t.Fatalf("done notes=%v", got)
	}
	if outcome.Ledger["done"].EnteredAt != "old-done" {
		t.Fatalf("done entered_at changed=%q", outcome.Ledger["done"].EnteredAt)
	}
}
