package handoff

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestIssueOpsExecutionHandoffTransitionTable(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-handoff", Repo: "/repo", Branch: "16-demo"}
	prepared, err := Prepare(record, PrepareRequest{
		Attempt:         1,
		OwnershipEpoch:  "epoch-1",
		CoordinatorRoot: "/repo",
		WorkerRoot:      "/repo.worktrees/16-demo",
		Agent:           "codex",
		Now:             "2026-07-11T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := prepared.ExecutionHandoff.State; got != StateCoordinatorPreparing {
		t.Fatalf("prepared state = %q, want %q", got, StateCoordinatorPreparing)
	}

	contextSHA := strings.Repeat("a", 64)
	prepared, err = SetContext(prepared, Fence{Attempt: 1, OwnershipEpoch: "epoch-1"}, 1, contextSHA, "2026-07-11T00:01:00Z")
	if err != nil {
		t.Fatal(err)
	}
	dispatched, err := Dispatch(prepared, Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: contextSHA}, model.IssueOpsOrcaIdentity{
		RuntimeID:   "runtime-1",
		WorktreeID:  "worktree-1",
		WorkerPTYID: "pty-1",
		TaskID:      "task-1",
		DispatchID:  "dispatch-1",
	}, "2026-07-11T00:02:00Z")
	if err != nil {
		t.Fatal(err)
	}

	worker := model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "agent-1"}
	claimed, err := Claim(dispatched, ClaimRequest{
		Fence:      Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: contextSHA},
		Worker:     worker,
		WorkerRoot: "/repo.worktrees/16-demo",
		Now:        "2026-07-11T00:03:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := claimed.ExecutionHandoff.State; got != StateClaimed {
		t.Fatalf("claimed state = %q, want %q", got, StateClaimed)
	}

	result := model.IssueOpsExecutionHandoffResult{
		Outcome:          OutcomeCompleted,
		FinalHead:        strings.Repeat("b", 40),
		ChangedFiles:     []string{"internal/core/issueops/example.go"},
		TuringReportPath: ".agent-harness/research/report.md",
		Verification:     []string{"go test ./...: pass"},
		CleanupReceipts:  []string{"cleanup: no runtime resources spawned"},
	}
	submitted, err := Finish(claimed, FinishRequest{
		Fence:  Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: contextSHA},
		Worker: worker,
		Result: result,
		Now:    "2026-07-11T00:04:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := submitted.ExecutionHandoff.State; got != StateSubmitted {
		t.Fatalf("submitted state = %q, want %q", got, StateSubmitted)
	}

	accepted, err := Accept(submitted, AcceptRequest{
		Fence:     Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: contextSHA},
		FinalHead: result.FinalHead,
		Now:       "2026-07-11T00:05:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := accepted.ExecutionHandoff.State; got != StateClosed {
		t.Fatalf("accepted state = %q, want %q", got, StateClosed)
	}
	if got := accepted.ExecutionHandoff.ClosedDisposition; got != DispositionAccepted {
		t.Fatalf("accepted disposition = %q, want %q", got, DispositionAccepted)
	}
}

func TestIssueOpsExecutionHandoffRejectsStaleAttemptEpochAndContext(t *testing.T) {
	record := dispatchedRecordForTest(t)
	tests := []struct {
		name  string
		fence Fence
	}{
		{name: "attempt", fence: Fence{Attempt: 2, OwnershipEpoch: "epoch-1", ContextSHA256: strings.Repeat("a", 64)}},
		{name: "epoch", fence: Fence{Attempt: 1, OwnershipEpoch: "epoch-stale", ContextSHA256: strings.Repeat("a", 64)}},
		{name: "context", fence: Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: strings.Repeat("c", 64)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Claim(record, ClaimRequest{
				Fence:      tt.fence,
				Worker:     model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1"},
				WorkerRoot: "/repo.worktrees/16-demo",
			})
			if err == nil || !strings.Contains(err.Error(), "stale handoff") {
				t.Fatalf("Claim() error = %v, want stale handoff", err)
			}
		})
	}
}

func TestIssueOpsExecutionHandoffPendingOperationSurvivesRoundTrip(t *testing.T) {
	record, err := Prepare(model.IssueOpsRecord{ID: "io-handoff"}, PrepareRequest{
		Attempt: 1, OwnershipEpoch: "epoch-1", CoordinatorRoot: "/repo", WorkerRoot: "/worker",
	})
	if err != nil {
		t.Fatal(err)
	}
	pending := model.IssueOpsExecutionHandoffPendingOperation{
		Kind:           OperationTerminalCreate,
		StartedAt:      "2026-07-11T00:00:00Z",
		BaselinePTYIDs: []string{"pty-a", "pty-b"},
	}
	updated, err := BeginOperation(record, Fence{Attempt: 1, OwnershipEpoch: "epoch-1"}, pending)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip := cloneRecord(t, updated)
	if !reflect.DeepEqual(roundTrip.ExecutionHandoff.PendingOperation, &pending) {
		t.Fatalf("pending operation round trip = %#v, want %#v", roundTrip.ExecutionHandoff.PendingOperation, pending)
	}
}

func dispatchedRecordForTest(t *testing.T) model.IssueOpsRecord {
	t.Helper()
	record, err := Prepare(model.IssueOpsRecord{ID: "io-handoff"}, PrepareRequest{
		Attempt: 1, OwnershipEpoch: "epoch-1", CoordinatorRoot: "/repo", WorkerRoot: "/repo.worktrees/16-demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	sha := strings.Repeat("a", 64)
	record, err = SetContext(record, Fence{Attempt: 1, OwnershipEpoch: "epoch-1"}, 1, sha, "")
	if err != nil {
		t.Fatal(err)
	}
	record, err = Dispatch(record, Fence{Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: sha}, model.IssueOpsOrcaIdentity{}, "")
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func cloneRecord(t *testing.T, record model.IssueOpsRecord) model.IssueOpsRecord {
	t.Helper()
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var cloned model.IssueOpsRecord
	if err := json.Unmarshal(raw, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}
