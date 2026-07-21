package issueops

import (
	"context"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

func TestHandoffSoleWriterAllowsExactOwnedDispatch(t *testing.T) {
	record, client := soleWriterOwnedDispatchFixture()

	if err := attestHandoffSoleWriter(context.Background(), record, client, "term_owner"); err != nil {
		t.Fatalf("exact owned dispatch rejected: %v", err)
	}
}

func TestHandoffSoleWriterAllowsCanonicalLazygitSidecar(t *testing.T) {
	record, client := soleWriterOwnedDispatchFixture()
	client.terminals = append(client.terminals, port.OrcaTerminal{
		Handle:       "term_lazygit",
		PTYID:        "pty_lazygit",
		WorktreeID:   "worktree_owner",
		WorktreePath: record.ExecutionHandoff.WorkerRoot,
		TabID:        "pty:pty_lazygit",
		LeafID:       "pty:pty_lazygit",
		Title:        "lazygit",
		Connected:    true,
		Writable:     true,
	})

	if err := attestHandoffSoleWriter(context.Background(), record, client, "term_owner"); err != nil {
		t.Fatalf("canonical Orca lazygit sidecar rejected: %v", err)
	}
}

func TestHandoffSoleWriterRejectsWritableNonSidecarTerminal(t *testing.T) {
	record, client := soleWriterOwnedDispatchFixture()
	client.terminals = append(client.terminals, port.OrcaTerminal{
		Handle:       "term_shell",
		PTYID:        "pty_shell",
		WorktreeID:   "worktree_owner",
		WorktreePath: record.ExecutionHandoff.WorkerRoot,
		TabID:        "tab_shell",
		LeafID:       "leaf_shell",
		Title:        "shell",
		Connected:    true,
		Writable:     true,
	})

	err := attestHandoffSoleWriter(context.Background(), record, client, "term_owner")
	if err == nil || !strings.Contains(err.Error(), "competing connected or writable terminal") {
		t.Fatalf("writable non-sidecar terminal accepted: %v", err)
	}
}

func TestReconcilePublicationOwnerTerminalIdentityPreservesMailbox(t *testing.T) {
	record, client := soleWriterOwnedDispatchFixture()
	live := client.terminals[0]
	live.Handle = "term_reissued"

	updated, err := reconcilePublicationOwnerTerminalIdentity(record, live)
	if err != nil {
		t.Fatalf("reconcile reissued owner terminal: %v", err)
	}
	if updated.ExecutionHandoff.Orca.WorkerTerminalHandle != "term_reissued" {
		t.Fatalf("live terminal handle = %q", updated.ExecutionHandoff.Orca.WorkerTerminalHandle)
	}
	if updated.ExecutionHandoff.Orca.WorkerMailboxHandle != "term_owner" {
		t.Fatalf("sealed mailbox handle changed = %q", updated.ExecutionHandoff.Orca.WorkerMailboxHandle)
	}
	client.terminals[0] = live
	if err := attestHandoffSoleWriter(context.Background(), updated, client, live.Handle); err != nil {
		t.Fatalf("reissued stable owner terminal rejected: %v", err)
	}
}

func TestHandoffSoleWriterRejectsOwnedDispatchFromNonOwnerTerminal(t *testing.T) {
	record, client := soleWriterOwnedDispatchFixture()

	err := attestHandoffSoleWriter(context.Background(), record, client, "term_other")
	if err == nil || !strings.Contains(err.Error(), "sole writer attestation found") {
		t.Fatalf("non-owner terminal accepted the owned dispatch: %v", err)
	}
}

func TestExactOwnedHandoffDispatchRequiresEverySealedIdentity(t *testing.T) {
	record, client := soleWriterOwnedDispatchFixture()
	h := record.ExecutionHandoff
	task := client.tasks[0]
	dispatch := client.dispatches[task.ID]
	if !exactOwnedHandoffDispatch(h, task, dispatch, "term_owner") {
		t.Fatal("exact sealed owner dispatch was rejected")
	}

	for _, tt := range []struct {
		name          string
		task          port.OrcaTask
		dispatch      port.OrcaDispatch
		allowedHandle string
	}{
		{name: "non-owner terminal", task: task, dispatch: dispatch, allowedHandle: "term_other"},
		{name: "foreign task", task: port.OrcaTask{ID: "task_other"}, dispatch: dispatch, allowedHandle: "term_owner"},
		{name: "foreign dispatch", task: task, dispatch: port.OrcaDispatch{ID: "dispatch_other", AssigneeHandle: "term_owner"}, allowedHandle: "term_owner"},
		{name: "foreign assignee", task: task, dispatch: port.OrcaDispatch{ID: "dispatch_owner", AssigneeHandle: "term_other"}, allowedHandle: "term_owner"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if exactOwnedHandoffDispatch(h, tt.task, tt.dispatch, tt.allowedHandle) {
				t.Fatal("mismatched dispatch identity was accepted")
			}
		})
	}
}

func TestHandoffSoleWriterRejectsCompetingDispatchOnOwnedTerminal(t *testing.T) {
	for _, tt := range []struct {
		name       string
		taskID     string
		dispatchID string
	}{
		{name: "different task", taskID: "task_other", dispatchID: "dispatch_other"},
		{name: "different dispatch", taskID: "task_owner", dispatchID: "dispatch_other"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			record, client := soleWriterOwnedDispatchFixture()
			client.tasks = []port.OrcaTask{{ID: tt.taskID, Status: "dispatched"}}
			client.dispatches = map[string]port.OrcaDispatch{tt.taskID: {
				ID: tt.dispatchID, TaskID: tt.taskID, AssigneeHandle: "term_owner", Status: "dispatched",
			}}

			err := attestHandoffSoleWriter(context.Background(), record, client, "term_owner")
			if err == nil || !strings.Contains(err.Error(), "dispatched task assigned to the exact worktree") {
				t.Fatalf("competing dispatch accepted: %v", err)
			}
		})
	}
}

func soleWriterOwnedDispatchFixture() (IssueOpsRecord, *soleWriterOrcaFake) {
	const workerRoot = "/repo.worktrees/51-p0-safety-critical-fixes"
	record := IssueOpsRecord{ExecutionHandoff: &model.IssueOpsExecutionHandoff{
		WorkerRoot: workerRoot,
		Orca: &model.IssueOpsOrcaIdentity{
			WorktreeID:           "worktree_owner",
			WorkerPTYID:          "pty_owner",
			WorkerTerminalHandle: "term_owner",
			WorkerMailboxHandle:  "term_owner",
			WorkerTabID:          "tab_owner",
			WorkerLeafID:         "leaf_owner",
			TaskID:               "task_owner",
			DispatchID:           "dispatch_owner",
		},
	}}
	client := &soleWriterOrcaFake{
		terminals: []port.OrcaTerminal{{
			Handle: "term_owner", PTYID: "pty_owner", WorktreeID: "worktree_owner", WorktreePath: workerRoot, TabID: "tab_owner", LeafID: "leaf_owner", Connected: true, Writable: true,
		}},
		tasks: []port.OrcaTask{{ID: "task_owner", Status: "dispatched"}},
		dispatches: map[string]port.OrcaDispatch{"task_owner": {
			ID: "dispatch_owner", TaskID: "task_owner", AssigneeHandle: "term_owner", Status: "dispatched",
		}},
	}
	return record, client
}

type soleWriterOrcaFake struct {
	terminals  []port.OrcaTerminal
	tasks      []port.OrcaTask
	dispatches map[string]port.OrcaDispatch
}

func (f *soleWriterOrcaFake) ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error) {
	return nil, nil
}

func (f *soleWriterOrcaFake) ListTerminals(_ context.Context, _ string) ([]port.OrcaTerminal, error) {
	return append([]port.OrcaTerminal(nil), f.terminals...), nil
}

func (f *soleWriterOrcaFake) CreateTerminal(context.Context, port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error) {
	return port.OrcaTerminal{}, nil
}

func (f *soleWriterOrcaFake) RefreshTerminal(context.Context, string, string) (port.OrcaTerminal, error) {
	return port.OrcaTerminal{}, nil
}

func (f *soleWriterOrcaFake) ListTasks(context.Context) ([]port.OrcaTask, error) {
	return nil, nil
}

func (f *soleWriterOrcaFake) ListDispatchedTasks(context.Context) ([]port.OrcaTask, error) {
	return append([]port.OrcaTask(nil), f.tasks...), nil
}

func (f *soleWriterOrcaFake) CreateTask(context.Context, port.OrcaCreateTaskRequest) (port.OrcaTask, error) {
	return port.OrcaTask{}, nil
}

func (f *soleWriterOrcaFake) Dispatch(context.Context, port.OrcaDispatchRequest) (port.OrcaDispatch, error) {
	return port.OrcaDispatch{}, nil
}

func (f *soleWriterOrcaFake) ShowDispatch(_ context.Context, taskID string) (port.OrcaDispatch, error) {
	return f.dispatches[taskID], nil
}

func (f *soleWriterOrcaFake) ShowDispatchFrom(context.Context, string, string) (port.OrcaDispatch, error) {
	return port.OrcaDispatch{}, nil
}
