package issueops

import (
	"context"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/port"
)

func TestHandoffCleanupRequiresApprovalAndPersistsOrderedIdempotentReceipts(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	record.ExecutionHandoff.State = handoff.StateClosed
	record.ExecutionHandoff.ClosedDisposition = handoff.DispositionWorkerFailed
	record.ExecutionHandoff.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "worker-1"}
	record.ExecutionHandoff.Result = validFailedHandoffResultForTest(record)
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminals = []port.OrcaTerminal{{
		Handle: record.ExecutionHandoff.Orca.WorkerTerminalHandle, PTYID: record.ExecutionHandoff.Orca.WorkerPTYID,
		WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, WorktreePath: record.ExecutionHandoff.WorkerRoot,
	}}
	client.dispatch = port.OrcaDispatch{
		ID: record.ExecutionHandoff.Orca.DispatchID, TaskID: record.ExecutionHandoff.Orca.TaskID,
		AssigneeHandle: record.ExecutionHandoff.Orca.WorkerMailboxHandle, Status: "failed",
	}
	client.worktrees = []port.OrcaWorktree{{
		ID: record.ExecutionHandoff.Orca.WorktreeID, InstanceID: record.ExecutionHandoff.Orca.WorktreeInstanceID,
		RepoID: record.ExecutionHandoff.Orca.RepoID, Path: record.ExecutionHandoff.WorkerRoot,
	}}

	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: "task_terminal",
	}, client, handoffPrepareTestClock()); err == nil {
		t.Fatal("cleanup receipt must require prior disposition approval")
	}
	approved, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "approve-cleanup", Confirm: true, CleanupDisposition: "remove", Reason: "discard failed worker resources",
	}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if approved.Cleanup == nil || approved.Cleanup.Disposition != "remove" || approved.Cleanup.ApprovedAt == "" {
		t.Fatalf("cleanup approval = %#v", approved.Cleanup)
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: "terminal_quiescent",
	}, client, handoffPrepareTestClock()); err == nil {
		t.Fatal("out-of-order cleanup receipt was accepted")
	}

	for _, step := range []string{"task_terminal", "task_terminal", "terminal_quiescent"} {
		if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
			ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: step,
		}, client, handoffPrepareTestClock()); err != nil {
			t.Fatalf("record cleanup %s: %v", step, err)
		}
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.Cleanup == nil || len(persisted.ExecutionHandoff.Cleanup.Receipts) != 2 {
		t.Fatalf("duplicate cleanup was not idempotent: %#v", persisted.ExecutionHandoff.Cleanup)
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: "worktree_removed",
	}, client, handoffPrepareTestClock()); err == nil {
		t.Fatal("existing worktree produced a removal receipt")
	}
	client.worktrees = nil
	completed, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: "worktree_removed",
	}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if completed.Cleanup == nil || len(completed.Cleanup.Receipts) != 3 || completed.Cleanup.Receipts[2].Step != "worktree_removed" {
		t.Fatalf("ordered cleanup completion = %#v", completed.Cleanup)
	}
}

func TestAcceptedHandoffCannotApproveCleanupDisposition(t *testing.T) {
	stateRoot, record := acceptedPublicationHandoff(t, "github")
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "approve-cleanup", Confirm: true, CleanupDisposition: "remove", Reason: "bypass publication boundary",
	}, nil, handoffPrepareTestClock()); err == nil {
		t.Fatal("accepted handoff approved destructive cleanup")
	}
}

func TestHandoffCleanupRecordsTerminalTaskWithStaleDispatchProjection(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	record.ExecutionHandoff.State = handoff.StateClosed
	record.ExecutionHandoff.ClosedDisposition = handoff.DispositionCancelled
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminals = nil
	client.tasks = []port.OrcaTask{{ID: record.ExecutionHandoff.Orca.TaskID, Status: "failed", HasResult: true}}
	client.dispatch = port.OrcaDispatch{
		ID: record.ExecutionHandoff.Orca.DispatchID, TaskID: record.ExecutionHandoff.Orca.TaskID,
		AssigneeHandle: record.ExecutionHandoff.Orca.WorkerMailboxHandle, Status: "dispatched",
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "approve-cleanup", Confirm: true, CleanupDisposition: "retry", Reason: "preserve committed checkpoint",
	}, client, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	got, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: "task_terminal",
	}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.Cleanup == nil || len(got.Cleanup.Receipts) != 1 || got.Cleanup.Receipts[0].Step != "task_terminal" {
		t.Fatalf("stale dispatch cleanup receipt = %#v", got.Cleanup)
	}
}

func TestHandoffCleanupAllowsTasklessPreDispatchCancellation(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	record.ExecutionHandoff.State = handoff.StateClosed
	record.ExecutionHandoff.ClosedDisposition = handoff.DispositionCancelled
	record.ExecutionHandoff.Orca.TaskID = ""
	record.ExecutionHandoff.Orca.DispatchID = ""
	record.ExecutionHandoff.Orca.WorkerMailboxHandle = ""
	record.ExecutionHandoff.DeliveryMode = ""
	record.ExecutionHandoff.DispatchedAt = ""
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminals = []port.OrcaTerminal{{
		Handle: record.ExecutionHandoff.Orca.WorkerTerminalHandle, PTYID: record.ExecutionHandoff.Orca.WorkerPTYID,
		WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, WorktreePath: record.ExecutionHandoff.WorkerRoot,
	}}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "approve-cleanup", Confirm: true, CleanupDisposition: "remove", Reason: "remove cancelled pre-dispatch resources",
	}, client, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}

	partial, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	partial.ExecutionHandoff.Orca.TaskID = "task-only"
	if _, err := WriteIssueOps(stateRoot, partial); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: "task_terminal",
	}, client, handoffPrepareTestClock()); err == nil {
		t.Fatal("partial task identity authorized pre-dispatch cleanup")
	}
	partial.ExecutionHandoff.Orca.TaskID = ""
	partial.ExecutionHandoff.Orca.DispatchID = "dispatch-only"
	partial.ExecutionHandoff.Orca.WorkerMailboxHandle = "mailbox-only"
	if _, err := WriteIssueOps(stateRoot, partial); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
		ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: "task_terminal",
	}, client, handoffPrepareTestClock()); err == nil {
		t.Fatal("partial dispatch identity authorized pre-dispatch cleanup")
	}
	partial.ExecutionHandoff.Orca.DispatchID = ""
	partial.ExecutionHandoff.Orca.WorkerMailboxHandle = ""
	if _, err := WriteIssueOps(stateRoot, partial); err != nil {
		t.Fatal(err)
	}

	for _, step := range []string{"task_terminal", "terminal_quiescent"} {
		if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
			ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: step,
		}, client, handoffPrepareTestClock()); err != nil {
			t.Fatalf("record taskless cleanup %s: %v", step, err)
		}
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := persisted.ExecutionHandoff.Cleanup.Receipts; len(got) != 2 || got[0].TaskID != "" || got[0].DispatchID != "" || got[1].Step != "terminal_quiescent" {
		t.Fatalf("taskless cleanup receipts = %#v", got)
	}
}

func TestHandoffCleanupRetriesTerminallessPreDispatchCancellation(t *testing.T) {
	stateRoot, record, client := dispatchedHandoffRecord(t)
	record.ExecutionHandoff.State = handoff.StateClosed
	record.ExecutionHandoff.ClosedDisposition = handoff.DispositionCancelled
	record.ExecutionHandoff.DeliveryMode = ""
	record.ExecutionHandoff.DispatchedAt = ""
	record.ExecutionHandoff.Orca.TaskID = ""
	record.ExecutionHandoff.Orca.DispatchID = ""
	record.ExecutionHandoff.Orca.WorkerMailboxHandle = ""
	record.ExecutionHandoff.Orca.WorkerPTYID = ""
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = ""
	record.ExecutionHandoff.Orca.WorkerTabID = ""
	record.ExecutionHandoff.Orca.WorkerLeafID = ""
	client.terminals = nil
	client.dispatchedTasks = nil
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}

	for _, req := range []IssueOpsHandoffRecoverRequest{
		{ID: record.ID, Action: "approve-cleanup", Confirm: true, CleanupDisposition: "retry", Reason: "retry terminalless pre-dispatch handoff"},
		{ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: "task_terminal"},
		{ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: "terminal_quiescent"},
		{ID: record.ID, Action: "retry", Confirm: true},
	} {
		if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, req, client, handoffPrepareTestClock()); err != nil {
			t.Fatalf("%s terminalless pre-dispatch recovery: %v", req.Action, err)
		}
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateCoordinatorPreparing || persisted.ExecutionHandoff.Attempt != 2 || persisted.ExecutionHandoff.Orca.WorkerPTYID != "" || persisted.ExecutionHandoff.Orca.WorkerTerminalHandle != "" {
		t.Fatalf("terminalless retry = %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffCleanupQuiescenceRejectsPossibleWritersDispatchesAndReissuedWorktree(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	record.ExecutionHandoff.State = handoff.StateClosed
	record.ExecutionHandoff.ClosedDisposition = handoff.DispositionCancelled
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.dispatch = port.OrcaDispatch{ID: record.ExecutionHandoff.Orca.DispatchID, TaskID: record.ExecutionHandoff.Orca.TaskID, AssigneeHandle: record.ExecutionHandoff.Orca.WorkerMailboxHandle, Status: "cancelled"}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "approve-cleanup", Confirm: true, CleanupDisposition: "remove", Reason: "remove cancelled resources"}, client, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: "task_terminal"}, client, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}

	client.terminals = []port.OrcaTerminal{{Handle: "term-sibling", PTYID: "pty-sibling", WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, WorktreePath: record.ExecutionHandoff.WorkerRoot, Writable: true}}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: "terminal_quiescent"}, client, handoffPrepareTestClock()); err == nil {
		t.Fatal("writable-only exact-worktree terminal authorized cleanup receipt")
	}
	client.terminals = nil
	client.dispatchedTasks = []port.OrcaTask{{ID: "task-reissued", Status: "dispatched"}}
	client.dispatchByTask = map[string]port.OrcaDispatch{"task-reissued": {ID: "dispatch-reissued", TaskID: "task-reissued", AssigneeHandle: record.ExecutionHandoff.Orca.WorkerTerminalHandle, Status: "dispatched"}}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: "terminal_quiescent"}, client, handoffPrepareTestClock()); err == nil {
		t.Fatal("dispatched exact-worktree assignment authorized cleanup receipt")
	}
	client.dispatchedTasks = nil
	client.dispatchByTask = nil
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: "terminal_quiescent"}, client, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	client.worktrees = []port.OrcaWorktree{{ID: "wt-unknown", InstanceID: "instance-unknown"}}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: "worktree_removed"}, client, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), "complete canonical classification") {
		t.Fatalf("incomplete worktree row proved removal: %v", err)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.ExecutionHandoff.Cleanup.Receipts) != 2 {
		t.Fatalf("incomplete worktree row persisted receipt: %#v", persisted.ExecutionHandoff.Cleanup)
	}
	client.worktrees = []port.OrcaWorktree{{ID: "wt-reissued", InstanceID: "instance-reissued", RepoID: record.ExecutionHandoff.Orca.RepoID, Path: record.ExecutionHandoff.WorkerRoot, Branch: "refs/heads/" + record.Branch}}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "record-cleanup", Confirm: true, CleanupStep: "worktree_removed"}, client, handoffPrepareTestClock()); err == nil {
		t.Fatal("reissued canonical-path/branch worktree authorized removal receipt")
	}
}
