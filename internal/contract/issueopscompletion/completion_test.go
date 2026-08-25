package issueopscompletion

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExecutionCompletionJSONRoundTrip(t *testing.T) {
	completion := Completion{
		Generation:        7,
		FinalHead:         "abc1234",
		TuringReportPath:  "docs/turing/io-9.md",
		Verification:      []string{"go test ./...", "agent-harness self-verify --json"},
		RemoteArtifactURL: "https://example.com/pr/9",
		CompletedAt:       "2026-08-25T00:00:00Z",
	}
	raw, err := json.Marshal(completion)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded Completion
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded.Generation != completion.Generation || decoded.FinalHead != completion.FinalHead ||
		decoded.TuringReportPath != completion.TuringReportPath ||
		decoded.RemoteArtifactURL != completion.RemoteArtifactURL ||
		decoded.CompletedAt != completion.CompletedAt ||
		len(decoded.Verification) != len(completion.Verification) ||
		(len(decoded.Verification) > 0 && decoded.Verification[0] != completion.Verification[0]) {
		t.Fatalf("round trip mismatch: %+v vs %+v", decoded, completion)
	}
}

func TestCommandAndLedgerJSONRoundTrip(t *testing.T) {
	command := Command{
		Generation:        3,
		Actor:             Actor{Host: "codex", SessionID: "s-1", AgentID: "a-1"},
		FinalHead:         "def5678",
		Verification:      []string{"go vet ./..."},
		RemoteArtifactURL: "https://example.com/mr/3",
	}
	raw, err := json.Marshal(command)
	if err != nil {
		t.Fatalf("marshal command failed: %v", err)
	}
	var decodedCommand Command
	if err := json.Unmarshal(raw, &decodedCommand); err != nil {
		t.Fatalf("unmarshal command failed: %v", err)
	}
	if decodedCommand.Generation != command.Generation || decodedCommand.Actor.Host != "codex" {
		t.Fatalf("command round trip mismatch: %+v", decodedCommand)
	}

	ledger := LedgerEntry{
		Phase:       "implement",
		EnteredAt:   "2026-08-25T01:00:00Z",
		CompletedAt: "2026-08-25T02:00:00Z",
		Artifacts:   []string{"plan"},
		Missing:     []string{},
		Notes:       []string{"ok"},
	}
	rawLedger, err := json.Marshal(ledger)
	if err != nil {
		t.Fatalf("marshal ledger failed: %v", err)
	}
	var decodedLedger LedgerEntry
	if err := json.Unmarshal(rawLedger, &decodedLedger); err != nil {
		t.Fatalf("unmarshal ledger failed: %v", err)
	}
	if decodedLedger.Phase != ledger.Phase || len(decodedLedger.Artifacts) != 1 || decodedLedger.Artifacts[0] != "plan" {
		t.Fatalf("ledger round trip mismatch: %+v", decodedLedger)
	}
}

func TestProcessReceiptRoundTrip(t *testing.T) {
	actor := Actor{
		Host:      "claude",
		SessionID: "sess-7",
		AgentID:   "agent-2",
		Process:   &ProcessReceipt{PID: 4242, StartedAt: "2026-08-25T03:00:00Z", Executable: "/usr/local/bin/claude"},
	}
	raw, err := json.Marshal(actor)
	if err != nil {
		t.Fatalf("marshal actor failed: %v", err)
	}
	var decoded Actor
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal actor failed: %v", err)
	}
	if decoded.Process == nil || decoded.Process.PID != 4242 || decoded.Process.Executable != "/usr/local/bin/claude" {
		t.Fatalf("process receipt round trip mismatch: %+v", decoded.Process)
	}
}

func TestErrExecutionNotPreparedAlias(t *testing.T) {
	if ErrExecutionNotPrepared == nil {
		t.Fatal("ErrExecutionNotPrepared must alias the lease contract error")
	}
	if strings.TrimSpace(ErrExecutionNotPrepared.Error()) == "" {
		t.Fatal("ErrExecutionNotPrepared message must not be empty")
	}
}

func TestRecordSnapshotCloneDeepCopiesMutableState(t *testing.T) {
	snapshot := RecordSnapshot{
		ID:       "io-9",
		Prepared: true,
		Phase:    "implement",
		Lease: Lease{
			Generation: 3,
			Status:     "claimed",
			Holder: &Actor{
				Host:      "codex",
				SessionID: "s-1",
				Process:   &ProcessReceipt{PID: 11, Executable: "/bin/codex"},
			},
		},
		Completion: &Completion{Generation: 3, Verification: []string{"go test ./..."}},
		Artifact: &RemoteArtifact{
			Provider:  "github",
			Kind:      "pull_request",
			URL:       "https://example.com/pr/9",
			Labels:    []string{"ready"},
			Assignees: []string{"alice"},
		},
		Orca: &OrcaBinding{RunID: "run-1", TaskID: "task-1"},
		Ledger: map[string]LedgerEntry{
			"plan": {Phase: "plan", Artifacts: []string{"plan"}, Missing: []string{}, Notes: []string{"ok"}},
		},
	}

	clone := snapshot.Clone()

	clone.Lease.Holder.Host = "claude"
	clone.Lease.Holder.Process.PID = 99
	clone.Completion.Verification[0] = "mutated"
	clone.Artifact.Labels[0] = "mutated"
	clone.Artifact.Assignees[0] = "bob"
	clone.Orca.RunID = "run-2"
	clone.Ledger["feedback"] = LedgerEntry{Phase: "feedback", Notes: []string{"new"}}
	clone.Ledger["plan"].Notes[0] = "mutated"

	if snapshot.Lease.Holder.Host != "codex" || snapshot.Lease.Holder.Process.PID != 11 {
		t.Fatalf("lease holder leaked through clone: %+v", snapshot.Lease.Holder)
	}
	if snapshot.Completion.Verification[0] != "go test ./..." {
		t.Fatalf("completion verification leaked: %v", snapshot.Completion.Verification)
	}
	if snapshot.Artifact.Labels[0] != "ready" || snapshot.Artifact.Assignees[0] != "alice" {
		t.Fatalf("artifact slices leaked: %+v", snapshot.Artifact)
	}
	if snapshot.Orca.RunID != "run-1" {
		t.Fatalf("orca binding leaked: %+v", snapshot.Orca)
	}
	if _, exists := snapshot.Ledger["feedback"]; exists {
		t.Fatal("ledger map is shared with clone")
	}
	if snapshot.Ledger["plan"].Notes[0] != "ok" {
		t.Fatalf("ledger entry slice leaked: %+v", snapshot.Ledger["plan"])
	}
}

func TestRecordSnapshotCloneHandlesEmptyProjection(t *testing.T) {
	var snapshot RecordSnapshot
	clone := snapshot.Clone()
	if clone.ID != "" || clone.Lease.Holder != nil || clone.Completion != nil || clone.Orca != nil {
		t.Fatalf("empty clone drifted: %+v", clone)
	}
	if clone.Ledger == nil {
		t.Fatal("clone ledger must be initialized to an empty map")
	}
}
