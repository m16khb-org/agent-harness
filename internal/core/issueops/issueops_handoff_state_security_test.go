package issueops

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/port"
)

func TestHandoffStartPersistsRecoveryForAmbiguousSoleWriterInventory(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.dispatchedTaskListErr = errors.New("truncated dispatched inventory")

	_, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "requires recovery") {
		t.Fatalf("ambiguous inventory error = %v", err)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.PendingOperation == nil || persisted.ExecutionHandoff.PendingOperation.Kind != handoff.OperationLeaseAttestation || persisted.ExecutionHandoff.Failure == nil || persisted.ExecutionHandoff.Failure.Code != "sole_writer_inventory_ambiguous" {
		t.Fatalf("ambiguous inventory did not persist recovery: %#v", persisted.ExecutionHandoff)
	}
	if client.terminalCreates != 0 || client.taskCreates != 0 || client.dispatchCalls != 0 {
		t.Fatalf("ambiguous inventory crossed mutation boundary: %#v", client.trace)
	}
}

func TestHandoffStartPersistsRecoveryWhenBaselineTerminalInventoryIsUnreadable(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.terminalListErr = errors.New("terminal inventory was truncated")

	_, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "requires recovery") {
		t.Fatalf("ambiguous baseline terminal inventory error = %v", err)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.PendingOperation == nil || persisted.ExecutionHandoff.PendingOperation.Kind != handoff.OperationLeaseAttestation {
		t.Fatalf("ambiguous baseline inventory did not persist recovery: %#v", persisted.ExecutionHandoff)
	}
	if client.terminalCreates != 0 {
		t.Fatalf("ambiguous baseline inventory crossed terminal mutation: %#v", client.trace)
	}
}

func TestHandoffStartRejectsPreexistingWritableTerminalAsCompetingWriter(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.terminals = []port.OrcaTerminal{{
		Handle: "term_baseline", PTYID: "pty-baseline", WorktreeID: record.ExecutionHandoff.Orca.WorktreeID,
		WorktreePath: record.ExecutionHandoff.WorkerRoot, Connected: true, Writable: true,
	}}
	client.dispatchedTasks = []port.OrcaTask{{ID: "task-other", Status: "dispatched"}}
	client.dispatchByTask = map[string]port.OrcaDispatch{
		"task-other": {ID: "dispatch-other", TaskID: "task-other", AssigneeHandle: "term_baseline", Status: "dispatched"},
	}

	_, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "sole writer") {
		t.Fatalf("baseline writable terminal error = %v", err)
	}
	if client.terminalCreates != 0 || client.taskCreates != 0 || client.dispatchCalls != 0 {
		t.Fatalf("competing terminal crossed mutation boundary: terminal=%d task=%d dispatch=%d", client.terminalCreates, client.taskCreates, client.dispatchCalls)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.PendingOperation == nil || persisted.ExecutionHandoff.PendingOperation.Kind != handoff.OperationLeaseAttestation || persisted.ExecutionHandoff.Failure == nil || persisted.ExecutionHandoff.Failure.Code != "sole_writer_conflict" {
		t.Fatalf("known terminal conflict was not persisted: %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffStartRejectsDispatchedTaskAssignedToExactWorktree(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.terminals = []port.OrcaTerminal{{
		Handle: "term_other", PTYID: "pty-other", WorktreeID: record.ExecutionHandoff.Orca.WorktreeID,
		WorktreePath: record.ExecutionHandoff.WorkerRoot, Connected: false, Writable: false,
	}}
	client.dispatchedTasks = []port.OrcaTask{{ID: "task-other", Status: "dispatched"}}
	client.dispatchByTask = map[string]port.OrcaDispatch{
		"task-other": {ID: "dispatch-other", TaskID: "task-other", AssigneeHandle: "term_other", Status: "dispatched"},
	}

	_, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "dispatched task") {
		t.Fatalf("dispatched task conflict error = %v", err)
	}
	if client.terminalCreates != 0 || client.taskCreates != 0 || client.dispatchCalls != 0 {
		t.Fatalf("dispatched task conflict crossed mutation boundary: terminal=%d task=%d dispatch=%d", client.terminalCreates, client.taskCreates, client.dispatchCalls)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.PendingOperation == nil || persisted.ExecutionHandoff.PendingOperation.Kind != handoff.OperationLeaseAttestation || persisted.ExecutionHandoff.Failure == nil || persisted.ExecutionHandoff.Failure.Code != "sole_writer_conflict" {
		t.Fatalf("known dispatched-task conflict was not persisted: %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffStartRejectsDispatchedTaskWithMissingAssigneeTerminal(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.terminals = nil
	client.dispatchedTasks = []port.OrcaTask{{ID: "task-other", Status: "dispatched"}}
	client.dispatchByTask = map[string]port.OrcaDispatch{"task-other": {ID: "dispatch-other", TaskID: "task-other", AssigneeHandle: "term-vanished", Status: "dispatched"}}
	_, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "assignee terminal is absent") || client.terminalCreates != 0 || client.taskCreates != 0 || client.dispatchCalls != 0 {
		t.Fatalf("missing assignee inventory error=%v calls=%d/%d/%d", err, client.terminalCreates, client.taskCreates, client.dispatchCalls)
	}
}

func TestHandoffStartBoundsAndRedactsUntrustedSoleWriterInventory(t *testing.T) {
	secret := "api_key=abcdefghijklmnopqrstuvwxyz123456"
	for _, tc := range []struct {
		name  string
		setup func(*dispatchOrcaFake, IssueOpsRecord)
	}{
		{name: "oversized terminal handle", setup: func(client *dispatchOrcaFake, record IssueOpsRecord) {
			client.terminals = []port.OrcaTerminal{{Handle: secret + strings.Repeat("x", 512), PTYID: "pty-other", WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, WorktreePath: record.ExecutionHandoff.WorkerRoot, Connected: true}}
		}},
		{name: "oversized task id and secret status", setup: func(client *dispatchOrcaFake, record IssueOpsRecord) {
			client.dispatchedTasks = []port.OrcaTask{{ID: secret + strings.Repeat("x", 512), Status: secret}}
		}},
		{name: "secret task status", setup: func(client *dispatchOrcaFake, record IssueOpsRecord) {
			client.dispatchedTasks = []port.OrcaTask{{ID: "task-other", Status: secret}}
		}},
		{name: "oversized dispatch id", setup: func(client *dispatchOrcaFake, record IssueOpsRecord) {
			client.dispatchedTasks = []port.OrcaTask{{ID: "task-other", Status: "dispatched"}}
			client.dispatchByTask = map[string]port.OrcaDispatch{"task-other": {ID: secret + strings.Repeat("x", 512), TaskID: "task-other", AssigneeHandle: "term-other", Status: "dispatched"}}
		}},
		{name: "oversized dispatch assignee", setup: func(client *dispatchOrcaFake, record IssueOpsRecord) {
			client.dispatchedTasks = []port.OrcaTask{{ID: "task-other", Status: "dispatched"}}
			client.dispatchByTask = map[string]port.OrcaDispatch{"task-other": {ID: "dispatch-other", TaskID: "task-other", AssigneeHandle: secret + strings.Repeat("x", 512), Status: "dispatched"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			client := handoffDispatchFake(record)
			tc.setup(client, record)
			_, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
			if err == nil || strings.Contains(err.Error(), "abcdefghijklmnopqrstuvwxyz123456") || len(err.Error()) > publicationDiagnosticLimit+256 {
				t.Fatalf("untrusted inventory error was not bounded/redacted: len=%d err=%v", len(errString(err)), err)
			}
			if client.terminalCreates != 0 || client.taskCreates != 0 || client.dispatchCalls != 0 {
				t.Fatalf("untrusted inventory crossed external mutation: %#v", client)
			}
			persisted, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			failure := persisted.ExecutionHandoff.Failure
			if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.PendingOperation == nil || persisted.ExecutionHandoff.PendingOperation.Kind != handoff.OperationLeaseAttestation || failure == nil || strings.Contains(failure.Message, "abcdefghijklmnopqrstuvwxyz123456") || len(failure.Message) > publicationDiagnosticLimit {
				t.Fatalf("untrusted inventory recovery is unsafe: %#v", persisted.ExecutionHandoff)
			}
		})
	}
}

func TestHandoffStartReattestsImmediatelyBeforeDispatch(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)

	if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock()); err != nil {
		t.Fatal(err)
	}
	if client.dispatchedTaskListCalls < 2 {
		t.Fatalf("dispatched task inventory calls = %d, want initial and pre-dispatch re-attestation", client.dispatchedTaskListCalls)
	}
	lastInventory, dispatch := -1, -1
	for i, step := range client.trace {
		if step == "dispatched-task-list" {
			lastInventory = i
		}
		if step == "dispatch" {
			dispatch = i
		}
	}
	if lastInventory < 0 || dispatch != lastInventory+1 {
		t.Fatalf("dispatch was not immediately preceded by re-attestation: %v", client.trace)
	}
}

func TestHandoffRetryRequiresExactExternalQuiescence(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	record.ExecutionHandoff.State = handoff.StateClosed
	record.ExecutionHandoff.ClosedDisposition = handoff.DispositionCancelled
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminals = []port.OrcaTerminal{{
		Handle: record.ExecutionHandoff.Orca.WorkerTerminalHandle, PTYID: record.ExecutionHandoff.Orca.WorkerPTYID,
		WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, WorktreePath: record.ExecutionHandoff.WorkerRoot,
		Connected: true, Writable: true,
	}}

	_, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "retry", Confirm: true,
	}, client, IssueOpsHandoffPrepareClock{Now: handoffPrepareTestClock().Now, NewEpoch: func() (string, error) { return "epoch-2", nil }})
	if err == nil || !strings.Contains(err.Error(), "quiescence") {
		t.Fatalf("retry quiescence error = %v", err)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.ExecutionHandoff.Attempt != 1 || persisted.ExecutionHandoff.Orca.DispatchID != record.ExecutionHandoff.Orca.DispatchID {
		t.Fatalf("retry replaced live external identity: %#v", persisted.ExecutionHandoff)
	}
}
