package issueops

import (
	"context"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/port"
)

func TestHandoffCancelClosesBeforeCleanup(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	updated, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "cancel", Confirm: true}, nil, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != handoff.StateClosed || updated.Disposition != handoff.DispositionCancelled {
		t.Fatalf("cancel must close durably: %#v", updated)
	}
}

func TestHandoffRetryUsesNewAttemptAndEpoch(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "cancel", Confirm: true}, nil, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "retry", Confirm: true}, nil, IssueOpsHandoffPrepareClock{Now: handoffPrepareTestClock().Now, NewEpoch: func() (string, error) { return "epoch-2", nil }})
	if err != nil {
		t.Fatal(err)
	}
	if got.Attempt != 2 || got.State != handoff.StateCoordinatorPreparing {
		t.Fatalf("retry did not create new fenced attempt: %#v", got)
	}
	persisted, _ := ReadIssueOps(stateRoot, record.ID)
	if persisted.ExecutionHandoff.OwnershipEpoch != "epoch-2" {
		t.Fatalf("epoch not replaced: %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffRecoverExactOneOnlyAndNeverAdvances(t *testing.T) {
	tests := []struct {
		name string
		rows []port.OrcaTerminal
		ok   bool
	}{
		{name: "zero"},
		{name: "one", rows: []port.OrcaTerminal{{Handle: "term-new", PTYID: "pty-new", WorktreeID: "wt-1", Connected: true, Writable: true}}, ok: true},
		{name: "multiple", rows: []port.OrcaTerminal{{Handle: "term-1", PTYID: "pty-1", WorktreeID: "wt-1"}, {Handle: "term-2", PTYID: "pty-2", WorktreeID: "wt-1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			record.ExecutionHandoff.State = handoff.StateRecoveryRequired
			record.ExecutionHandoff.PendingOperation = &IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTerminalCreate, BaselinePTYIDs: []string{"old"}}
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake()
			client.terminals = tt.rows
			got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "reconcile"}, client, handoffPrepareTestClock())
			if tt.ok && (err != nil || got.State != handoff.StateCoordinatorPreparing || got.NextCommand == "") {
				t.Fatalf("expected reconciled status only, got %#v err=%v", got, err)
			}
			if tt.ok && (client.taskCreates != 0 || client.dispatchCalls != 0) {
				t.Fatal("reconcile advanced to a next operation")
			}
			if !tt.ok && err == nil {
				t.Fatalf("expected fail closed, got %#v", got)
			}
		})
	}
}
