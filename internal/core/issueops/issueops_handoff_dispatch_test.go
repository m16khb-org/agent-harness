package issueops

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/port"
)

func TestHandoffStartRequiresPreDispatchReadiness(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.WorktreeTools = nil
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	_, err := StartIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffStartRequest{ID: record.ID, Confirm: true}, client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "worktree_tools_prepared") {
		t.Fatalf("expected readiness error, got %v", err)
	}
	if len(client.trace) != 0 {
		t.Fatalf("readiness failure called Orca: %v", client.trace)
	}
}

func TestHandoffStartRequiresExplicitCodexHookTrustBypassAttestation(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	preview, err := StartIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffStartRequest{ID: record.ID}, client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Preview || !preview.CodexHookTrustBypassRequired || preview.CodexHookTrustBypassAttested {
		t.Fatalf("Codex preview must expose the unattested startup requirement: %#v", preview)
	}
	if len(client.trace) != 0 {
		t.Fatalf("preview invoked Orca: %v", client.trace)
	}

	_, err = StartIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffStartRequest{ID: record.ID, Confirm: true}, client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "--allow-codex-hook-trust-bypass") {
		t.Fatalf("confirmed unattested Codex start error = %v", err)
	}
	if len(client.trace) != 0 {
		t.Fatalf("missing attestation must fail before terminal/task/dispatch calls: %v", client.trace)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.ExecutionHandoff.ContextSHA256 != "" || persisted.ExecutionHandoff.PendingOperation != nil {
		t.Fatalf("missing attestation persisted dispatch state: %#v", persisted.ExecutionHandoff)
	}

	attestedRequest := IssueOpsHandoffStartRequest{ID: record.ID, Context: handoff.ContextOptions{AllowCodexHookTrustBypass: true}}
	reviewed, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedRequest, client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if !reviewed.Preview || !reviewed.CodexHookTrustBypassRequired || !reviewed.CodexHookTrustBypassAttested || len(reviewed.ContextSHA256) != 64 {
		t.Fatalf("attested no-confirm preview must expose the reviewed context hash: %#v", reviewed)
	}
	if len(client.trace) != 0 {
		t.Fatalf("attested preview invoked Orca: %v", client.trace)
	}
	attestedRequest.Confirm = true
	started, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedRequest, client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if started.ContextSHA256 != reviewed.ContextSHA256 || !started.CodexHookTrustBypassRequired || !started.CodexHookTrustBypassAttested || len(client.terminalRequests) != 1 || !client.terminalRequests[0].AllowCodexHookTrustBypass {
		t.Fatalf("attested Codex start did not preserve launch authority: result=%#v requests=%#v", started, client.terminalRequests)
	}
}

func TestHandoffStartLeavesClaudeAndGJCStartupUnchanged(t *testing.T) {
	for _, agent := range []string{"claude", "gjc"} {
		t.Run(agent, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			record.ExecutionHandoff.Agent = agent
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			started, err := StartIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffStartRequest{ID: record.ID, Confirm: true}, client, handoffStartTestClock())
			if err != nil {
				t.Fatal(err)
			}
			if started.CodexHookTrustBypassRequired || started.CodexHookTrustBypassAttested || len(client.terminalRequests) != 1 || client.terminalRequests[0].AllowCodexHookTrustBypass {
				t.Fatalf("%s startup changed under the Codex-only attestation: result=%#v terminal=%#v", agent, started, client.terminalRequests)
			}
		})
	}
}

func TestHandoffStartAdvancedStateRepeatsNeverMutateExternalSystems(t *testing.T) {
	for _, state := range []string{handoff.StateClaimed, handoff.StateSubmitted, handoff.StateClosed} {
		t.Run(state, func(t *testing.T) {
			stateRoot, record, _ := dispatchedHandoffRecord(t)
			record.ExecutionHandoff.State = state
			switch state {
			case handoff.StateClaimed:
				record.ExecutionHandoff.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1"}
			case handoff.StateSubmitted:
				record.ExecutionHandoff.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1"}
				record.ExecutionHandoff.Result = validCompletedHandoffResultForTest(record)
			case handoff.StateClosed:
				record.ExecutionHandoff.ClosedDisposition = handoff.DispositionAccepted
				record.ExecutionHandoff.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1"}
				record.ExecutionHandoff.Result = validCompletedHandoffResultForTest(record)
			}
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			got, err := StartIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffStartRequest{ID: record.ID, Confirm: true}, client, handoffStartTestClock())
			if err != nil || got.State != state {
				t.Fatalf("advanced-state start repeat = %#v err=%v", got, err)
			}
			if len(client.trace) != 0 {
				t.Fatalf("advanced-state start repeat invoked Orca: %v", client.trace)
			}
		})
	}
}

func TestHandoffStartPersistsStableContextBeforeMutation(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.beforeTerminalCreate = func() {
		persisted, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			t.Fatal(err)
		}
		if persisted.ExecutionHandoff.ContextVersion != handoff.ContextVersion || len(persisted.ExecutionHandoff.ContextSHA256) != 64 || persisted.ExecutionHandoff.PendingOperation == nil {
			t.Fatalf("context and pending operation must precede mutation: %#v", persisted.ExecutionHandoff)
		}
	}
	got, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ContextSHA256) != 64 {
		t.Fatalf("missing stable context hash: %#v", got)
	}
}

func TestHandoffStartLateCreateErrorCannotReopenCancelledAttempt(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	var cancelErr error
	client.beforeTerminalCreate = func() {
		_, cancelErr = RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
			ID: record.ID, Action: "cancel", Confirm: true,
		}, nil, handoffPrepareTestClock())
	}
	client.terminalErr = errors.New("terminal create returned after coordinator cancel")
	if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(record.ID), client, handoffStartTestClock()); err == nil {
		t.Fatal("late terminal create error must still be reported")
	}
	if cancelErr == nil {
		t.Fatal("coordinator cancel must reject an unresolved terminal-create journal")
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.ClosedDisposition != "" || persisted.ExecutionHandoff.PendingOperation == nil {
		t.Fatalf("late error must preserve the unresolved recovery journal: %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffStartCreatesTerminalTaskDispatchExactlyOnce(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	req := attestedCodexStart(record.ID)
	first, err := StartIssueOpsHandoff(context.Background(), stateRoot, req, client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	second, err := StartIssueOpsHandoff(context.Background(), stateRoot, req, client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if client.terminalCreates != 1 || client.taskCreates != 1 || client.dispatchCalls != 1 {
		t.Fatalf("expected one of each mutation, got terminal=%d task=%d dispatch=%d trace=%v", client.terminalCreates, client.taskCreates, client.dispatchCalls, client.trace)
	}
	if first.State != handoff.StateDispatched || second.State != handoff.StateDispatched || first.Orca == nil || first.Orca.DispatchID != "dispatch-1" {
		t.Fatalf("unexpected results: first=%#v second=%#v", first, second)
	}
}

func TestHandoffStartResolvesOptionalPTYFromExactTerminalDelta(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.terminals = []port.OrcaTerminal{
		{Handle: "term-old", PTYID: "pty-old", WorktreeID: "wt-1", WorktreePath: record.WorktreePath, Connected: true, Writable: true},
	}
	client.terminal = port.OrcaTerminal{Handle: "term-create", WorktreeID: "wt-1"}
	client.terminalsAfterCreate = []port.OrcaTerminal{
		{Handle: "term-old", PTYID: "pty-old", WorktreeID: "wt-1", WorktreePath: record.WorktreePath, Connected: true, Writable: true},
		{Handle: "term-live", PTYID: "pty-new", WorktreeID: "wt-1", WorktreePath: record.WorktreePath, Connected: true, Writable: true},
	}
	client.dispatch.AssigneeHandle = "term-live"

	got, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateDispatched || got.Orca == nil || got.Orca.WorkerPTYID != "pty-new" || got.Orca.WorkerMailboxHandle != "term-live" {
		t.Fatalf("partial create identity did not resolve from PTY delta: %#v", got)
	}
	if client.terminalCreates != 1 || client.terminalListCalls != 2 {
		t.Fatalf("terminal create/list calls = %d/%d, want 1/2; trace=%v", client.terminalCreates, client.terminalListCalls, client.trace)
	}
}

func TestHandoffStartRejectsCreatePTYThatDiffersFromDelta(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.terminal = port.OrcaTerminal{Handle: "term-create", PTYID: "pty-returned", WorktreeID: "wt-1"}
	client.terminalsAfterCreate = []port.OrcaTerminal{
		{Handle: "term-live", PTYID: "pty-listed", WorktreeID: "wt-1", WorktreePath: record.WorktreePath, Connected: true, Writable: true},
	}

	_, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(record.ID), client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "created terminal PTY") {
		t.Fatalf("StartIssueOpsHandoff() error = %v, want create/list PTY mismatch", err)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.PendingOperation == nil || client.terminalCreates != 1 || client.taskCreates != 0 {
		t.Fatalf("PTY mismatch did not preserve recovery: handoff=%#v mutations=%d/%d", persisted.ExecutionHandoff, client.terminalCreates, client.taskCreates)
	}
}

func TestHandoffStartCrashMatrixNeverRepeatsCreate(t *testing.T) {
	tests := []struct {
		name string
		fail func(*dispatchOrcaFake)
	}{
		{name: "terminal-after-invoke", fail: func(f *dispatchOrcaFake) { f.terminalErr = &port.OrcaError{Code: "timeout", Invoked: true} }},
		{name: "task-after-invoke", fail: func(f *dispatchOrcaFake) { f.taskErr = &port.OrcaError{Code: "timeout", Invoked: true} }},
		{name: "dispatch-after-invoke", fail: func(f *dispatchOrcaFake) { f.dispatchErr = &port.OrcaError{Code: "timeout", Invoked: true} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			client := handoffDispatchFake(record)
			tt.fail(client)
			req := attestedCodexStart(record.ID)
			if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, req, client, handoffStartTestClock()); err == nil {
				t.Fatal("expected ambiguous mutation error")
			}
			counts := []int{client.terminalCreates, client.taskCreates, client.dispatchCalls}
			got, err := StartIssueOpsHandoff(context.Background(), stateRoot, req, client, handoffStartTestClock())
			if err != nil {
				t.Fatalf("repeat should return recovery status: %v", err)
			}
			if got.State != handoff.StateRecoveryRequired || client.terminalCreates != counts[0] || client.taskCreates != counts[1] || client.dispatchCalls != counts[2] {
				t.Fatalf("repeat performed mutation: result=%#v counts=%v now=%d/%d/%d", got, counts, client.terminalCreates, client.taskCreates, client.dispatchCalls)
			}
		})
	}
}

func TestHandoffStartDefinitivePreInvocationFailuresClearOnlyTheirJournal(t *testing.T) {
	tests := []struct {
		name  string
		fail  func(*dispatchOrcaFake)
		clear func(*dispatchOrcaFake)
	}{
		{name: "terminal command_start_failed", fail: func(f *dispatchOrcaFake) {
			f.terminalErr = &port.OrcaError{Code: "command_start_failed", Invoked: false}
		}, clear: func(f *dispatchOrcaFake) { f.terminalErr = nil }},
		{name: "task command_start_failed", fail: func(f *dispatchOrcaFake) { f.taskErr = &port.OrcaError{Code: "command_start_failed", Invoked: false} }, clear: func(f *dispatchOrcaFake) { f.taskErr = nil }},
		{name: "dispatch command_start_failed", fail: func(f *dispatchOrcaFake) {
			f.dispatchErr = &port.OrcaError{Code: "command_start_failed", Invoked: false}
		}, clear: func(f *dispatchOrcaFake) { f.dispatchErr = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			client := handoffDispatchFake(record)
			tt.fail(client)
			req := attestedCodexStart(record.ID)
			if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, req, client, handoffStartTestClock()); err == nil || !strings.Contains(err.Error(), "safe to retry") {
				t.Fatalf("definitive failure must return retry guidance: %v", err)
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionHandoff.State != handoff.StateCoordinatorPreparing || persisted.ExecutionHandoff.PendingOperation != nil {
				t.Fatalf("definitive non-invocation left recovery journal: %#v", persisted.ExecutionHandoff)
			}
			tt.clear(client)
			if got, err := StartIssueOpsHandoff(context.Background(), stateRoot, req, client, handoffStartTestClock()); err != nil || got.State != handoff.StateDispatched {
				t.Fatalf("safe explicit retry did not complete: %#v err=%v", got, err)
			}
		})
	}
}

func TestHandoffStartRejectsFullTerminalAndTaskBaselinesBeforeCreate(t *testing.T) {
	t.Run("terminal", func(t *testing.T) {
		stateRoot, record := handoffDispatchRecord(t)
		client := handoffDispatchFake(record)
		client.terminals = make([]port.OrcaTerminal, 0, handoff.MaxBaselineIDs)
		for i := 0; i < handoff.MaxBaselineIDs; i++ {
			client.terminals = append(client.terminals, port.OrcaTerminal{PTYID: fmt.Sprintf("pty-%03d", i)})
		}
		if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(record.ID), client, handoffStartTestClock()); err == nil || !strings.Contains(err.Error(), "headroom") {
			t.Fatalf("terminal headroom failure = %v", err)
		}
		if client.terminalCreates != 0 {
			t.Fatalf("terminal create invoked without delta headroom: %d", client.terminalCreates)
		}
	})
	t.Run("task", func(t *testing.T) {
		stateRoot, record := handoffDispatchRecord(t)
		record.ExecutionHandoff.Orca.WorkerPTYID = "pty-1"
		record.ExecutionHandoff.Orca.WorkerMailboxHandle = "term-1"
		if _, err := WriteIssueOps(stateRoot, record); err != nil {
			t.Fatal(err)
		}
		client := handoffDispatchFake(record)
		client.tasks = make([]port.OrcaTask, 0, handoff.MaxBaselineIDs)
		for i := 0; i < handoff.MaxBaselineIDs; i++ {
			client.tasks = append(client.tasks, port.OrcaTask{ID: fmt.Sprintf("task-%03d", i)})
		}
		if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(record.ID), client, handoffStartTestClock()); err == nil || !strings.Contains(err.Error(), "headroom") {
			t.Fatalf("task headroom failure = %v", err)
		}
		if client.taskCreates != 0 {
			t.Fatalf("task create invoked without delta headroom: %d", client.taskCreates)
		}
	})
}

func TestHandoffStartContinuesAfterTerminalCreateReconcileWithoutDuplicate(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationTerminalCreate, BaselinePTYIDs: []string{"pty-old"},
	})
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminals = []port.OrcaTerminal{
		{Handle: "term-stale", PTYID: "pty-1", WorktreeID: "wt-1", WorktreePath: record.WorktreePath, Connected: true, Writable: true},
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "reconcile"}, client, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	client.refreshedTerminal = port.OrcaTerminal{Handle: "term-live", PTYID: "pty-1", WorktreeID: "wt-1", WorktreePath: record.WorktreePath, Connected: true, Writable: true}
	client.dispatch.AssigneeHandle = "term-live"

	got, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateDispatched || client.terminalCreates != 0 || client.terminalRefreshes != 1 || client.taskCreates != 1 || client.dispatchCalls != 1 {
		t.Fatalf("reconciled terminal was replayed: result=%#v create/refresh/task/dispatch=%d/%d/%d/%d trace=%v", got, client.terminalCreates, client.terminalRefreshes, client.taskCreates, client.dispatchCalls, client.trace)
	}
	if len(client.dispatchRequests) != 1 || client.dispatchRequests[0].ToHandle != "term-live" || got.Orca == nil || got.Orca.WorkerMailboxHandle != "term-live" {
		t.Fatalf("dispatch did not use the refreshed mailbox: result=%#v requests=%#v", got, client.dispatchRequests)
	}
}

func TestHandoffStartContinuesAfterTaskCreateReconcileWithoutDuplicate(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.ExecutionHandoff.Orca.TerminalBaselinePTYIDs = []string{"pty-old"}
	record.ExecutionHandoff.Orca.WorkerPTYID = "pty-1"
	record.ExecutionHandoff.Orca.WorkerMailboxHandle = "term-1"
	setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationTaskCreate, BaselineTaskIDs: []string{"task-old"},
	})
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.tasks = []port.OrcaTask{
		{ID: "task-1", Title: mustHandoffTaskTitle(t, record), DisplayName: mustHandoffTaskDisplay(t, record), Status: "ready"},
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "reconcile"}, client, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}

	got, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateDispatched || client.terminalCreates != 0 || client.taskCreates != 0 || client.dispatchCalls != 1 {
		t.Fatalf("reconciled terminal or task was replayed: result=%#v mutations=%d/%d/%d trace=%v", got, client.terminalCreates, client.taskCreates, client.dispatchCalls, client.trace)
	}
}

func TestHandoffStartTerminalDeltaRequiresExactlyOne(t *testing.T) {
	tests := []struct {
		name string
		rows []port.OrcaTerminal
		ok   bool
	}{
		{name: "zero", rows: []port.OrcaTerminal{{PTYID: "old"}}},
		{name: "one", rows: []port.OrcaTerminal{{PTYID: "old"}, {PTYID: "new", Handle: "term-new", WorktreeID: "wt-1", WorktreePath: "/worker", Connected: true, Writable: true}}, ok: true},
		{name: "disconnected", rows: []port.OrcaTerminal{{PTYID: "new", Handle: "term-new", WorktreeID: "wt-1"}}},
		{name: "multiple", rows: []port.OrcaTerminal{{PTYID: "new-1"}, {PTYID: "new-2"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReconcileIssueOpsHandoffTerminal([]string{"old"}, "wt-1", "/worker", tt.rows)
			if tt.ok && (err != nil || got.PTYID != "new") {
				t.Fatalf("got %#v err=%v", got, err)
			}
			if !tt.ok && err == nil {
				t.Fatalf("expected fail closed, got %#v", got)
			}
		})
	}
}

func TestHandoffStartTaskMarkerRequiresExactlyOne(t *testing.T) {
	marker := issueOpsHandoffMarker("io-demo", "epoch-1", 1)
	rows := []port.OrcaTask{{ID: "old", Title: "old"}, {ID: "new", Title: marker, DisplayName: "16-demo", Status: "ready"}}
	got, err := ReconcileIssueOpsHandoffTask([]string{"old"}, marker, "16-demo", rows)
	if err != nil || got.ID != "new" {
		t.Fatalf("got %#v err=%v", got, err)
	}
	if _, err := ReconcileIssueOpsHandoffTask(nil, marker, "16-demo", append(rows, port.OrcaTask{ID: "another", Title: marker, DisplayName: "16-demo", Status: "ready"})); err == nil {
		t.Fatal("multiple marker candidates must fail closed")
	}
}

func TestHandoffStartDispatchRecoveryRequiresPersistedTask(t *testing.T) {
	client := handoffDispatchFake()
	if _, err := ReconcileIssueOpsHandoffDispatch(context.Background(), "", "term-1", client); err == nil {
		t.Fatal("missing persisted task must fail")
	}
	got, err := ReconcileIssueOpsHandoffDispatch(context.Background(), "task-1", "term-1", client)
	if err != nil || got.ID != "dispatch-1" {
		t.Fatalf("got %#v err=%v", got, err)
	}
}

type dispatchOrcaFake struct {
	terminals            []port.OrcaTerminal
	terminalsAfterCreate []port.OrcaTerminal
	tasks                []port.OrcaTask
	terminal             port.OrcaTerminal
	refreshedTerminal    port.OrcaTerminal
	task                 port.OrcaTask
	dispatch             port.OrcaDispatch
	terminalErr          error
	taskErr              error
	dispatchErr          error
	beforeTerminalCreate func()
	terminalCreates      int
	terminalListCalls    int
	terminalRefreshes    int
	taskCreates          int
	dispatchCalls        int
	dispatchRequests     []port.OrcaDispatchRequest
	terminalRequests     []port.OrcaCreateTerminalRequest
	trace                []string
}

func handoffDispatchFake(records ...IssueOpsRecord) *dispatchOrcaFake {
	workerRoot := ""
	if len(records) > 0 && records[0].ExecutionHandoff != nil {
		workerRoot = records[0].ExecutionHandoff.WorkerRoot
	}
	fake := &dispatchOrcaFake{
		terminal: port.OrcaTerminal{Handle: "term-1", PTYID: "pty-1", WorktreeID: "wt-1", WorktreePath: workerRoot, Connected: true, Writable: true},
		task:     port.OrcaTask{ID: "task-1", Status: "ready"},
		dispatch: port.OrcaDispatch{ID: "dispatch-1", TaskID: "task-1", AssigneeHandle: "term-1", Status: "dispatched", Injected: true},
	}
	fake.terminalsAfterCreate = []port.OrcaTerminal{fake.terminal}
	return fake
}

func attestedCodexStart(id string) IssueOpsHandoffStartRequest {
	return IssueOpsHandoffStartRequest{
		ID: id, Confirm: true, Context: handoff.ContextOptions{AllowCodexHookTrustBypass: true},
	}
}

func mustHandoffTaskTitle(t *testing.T, record IssueOpsRecord) string {
	t.Helper()
	title, _, err := issueOpsHandoffTaskIdentity(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt)
	if err != nil {
		t.Fatal(err)
	}
	return title
}

func mustHandoffTaskDisplay(t *testing.T, record IssueOpsRecord) string {
	t.Helper()
	_, display, err := issueOpsHandoffTaskIdentity(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt)
	if err != nil {
		t.Fatal(err)
	}
	return display
}

func (f *dispatchOrcaFake) ListTerminals(context.Context, string) ([]port.OrcaTerminal, error) {
	f.trace = append(f.trace, "terminal-list")
	f.terminalListCalls++
	return append([]port.OrcaTerminal(nil), f.terminals...), nil
}

func (f *dispatchOrcaFake) CreateTerminal(_ context.Context, req port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error) {
	f.trace = append(f.trace, "terminal-create")
	f.terminalCreates++
	f.terminalRequests = append(f.terminalRequests, req)
	if f.beforeTerminalCreate != nil {
		f.beforeTerminalCreate()
	}
	if f.terminalsAfterCreate != nil && !externalMutationNotInvoked(f.terminalErr) {
		f.terminals = append([]port.OrcaTerminal(nil), f.terminalsAfterCreate...)
	}
	return f.terminal, f.terminalErr
}

func (f *dispatchOrcaFake) RefreshTerminal(context.Context, string, string) (port.OrcaTerminal, error) {
	f.trace = append(f.trace, "terminal-refresh")
	f.terminalRefreshes++
	if f.refreshedTerminal.PTYID != "" {
		return f.refreshedTerminal, nil
	}
	return f.terminal, nil
}

func (f *dispatchOrcaFake) ListTasks(context.Context) ([]port.OrcaTask, error) {
	f.trace = append(f.trace, "task-list")
	return append([]port.OrcaTask(nil), f.tasks...), nil
}

func (f *dispatchOrcaFake) CreateTask(_ context.Context, req port.OrcaCreateTaskRequest) (port.OrcaTask, error) {
	f.trace = append(f.trace, "task-create")
	f.taskCreates++
	task := f.task
	if task.Title == "" {
		task.Title = req.Title
	}
	if task.DisplayName == "" {
		task.DisplayName = req.DisplayName
	}
	return task, f.taskErr
}

func (f *dispatchOrcaFake) Dispatch(_ context.Context, req port.OrcaDispatchRequest) (port.OrcaDispatch, error) {
	f.trace = append(f.trace, "dispatch")
	f.dispatchCalls++
	f.dispatchRequests = append(f.dispatchRequests, req)
	return f.dispatch, f.dispatchErr
}

func (f *dispatchOrcaFake) ShowDispatch(context.Context, string) (port.OrcaDispatch, error) {
	f.trace = append(f.trace, "dispatch-show")
	return f.dispatch, nil
}

func (f *dispatchOrcaFake) SendTerminal(context.Context, string, string) error {
	return errors.New("unexpected compatibility delivery")
}

func handoffDispatchRecord(t *testing.T) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	worktree := makeIssueOpsWorktreeDirForTest(t, repo, "16-demo")
	plan := filepath.Join(worktree, "plans", "plan.md")
	writeIssueOpsFile(t, worktree, "plans/plan.md", "# plan\n")
	record := IssueOpsRecord{
		SchemaVersion:       IssueOpsCurrentSchemaVersion,
		ID:                  NewIssueOpsID(repo, "16-demo"),
		Repo:                repo,
		Branch:              "16-demo",
		IssueURL:            "https://github.com/acme/repo/issues/16",
		Phase:               IssueOpsPhaseCompatibilityReview,
		PlanPath:            plan,
		WorktreePath:        worktree,
		Intent:              issueOpsIntentContractForTest(),
		DesignReview:        issueOpsDesignReviewForTest(),
		ExecutionDecision:   issueOpsExecutionDecisionForTest(),
		CompatibilityReview: issueOpsCompatibilityReviewForTest(),
		DevilsAdvocateReview: &IssueOpsDevilsAdvocateReview{
			Verdict: "pass", RecordedAt: "2026-07-11T00:00:00Z",
		},
		BranchPrepare: &IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/acme/repo/issues/16", Branch: "16-demo", BaseBranch: "main", BaseSHA: strings.Repeat("a", 40), LinkVerified: true},
		WorktreeTools: &IssueOpsWorktreeToolPreparation{OK: true, WorktreePath: worktree, PreparedAt: "2026-07-11T00:00:00Z"},
		ExecutionHandoff: &IssueOpsExecutionHandoff{
			ProtocolVersion: handoff.ProtocolVersion, State: handoff.StateCoordinatorPreparing, Attempt: 1, OwnershipEpoch: "epoch-1", Driver: "orca", Agent: "codex", CoordinatorRoot: repo, WorkerRoot: worktree,
			AttemptBaseHead: strings.Repeat("a", 40),
			Orca: &IssueOpsOrcaIdentity{RuntimeID: "runtime-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo",
				WorktreeID: "wt-1", WorktreeInstanceID: "inst-1", WorktreePath: worktree},
		},
		CreatedAt: "2026-07-11T00:00:00Z", UpdatedAt: "2026-07-11T00:00:00Z",
	}
	got, err := WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, got
}

func handoffStartTestClock() IssueOpsHandoffStartClock {
	return IssueOpsHandoffStartClock{Now: func() time.Time { return time.Date(2026, 7, 11, 2, 3, 4, 0, time.UTC) }}
}

func validCompletedHandoffResultForTest(record IssueOpsRecord) *IssueOpsExecutionHandoffResult {
	return &IssueOpsExecutionHandoffResult{
		Outcome: handoff.OutcomeCompleted, FinalHead: strings.Repeat("b", 40),
		ChangedFiles: []string{".agent-harness/research/report.md"}, TuringReportPath: ".agent-harness/research/report.md",
		Verification: []string{"focused verification passed"}, CleanupReceipts: []string{"worker resources handed off"},
		TaskID: record.ExecutionHandoff.Orca.TaskID, DispatchID: record.ExecutionHandoff.Orca.DispatchID,
	}
}

func validFailedHandoffResultForTest(record IssueOpsRecord) *IssueOpsExecutionHandoffResult {
	return &IssueOpsExecutionHandoffResult{
		Outcome: handoff.OutcomeFailed, Verification: []string{"failure reproduced"}, CleanupReceipts: []string{"worker resources stopped"},
		TaskID: record.ExecutionHandoff.Orca.TaskID, DispatchID: record.ExecutionHandoff.Orca.DispatchID,
	}
}

func setRecoveryRequiredForTest(record *IssueOpsRecord, pending IssueOpsExecutionHandoffPendingOperation) {
	if pending.StartedAt == "" {
		pending.StartedAt = "2026-07-11T00:00:01Z"
	}
	record.ExecutionHandoff.State = handoff.StateRecoveryRequired
	record.ExecutionHandoff.PendingOperation = &pending
	record.ExecutionHandoff.Failure = &IssueOpsExecutionHandoffFailure{
		Code: "test_recovery", Message: "reconcile the exact pending operation", At: pending.StartedAt,
	}
}
