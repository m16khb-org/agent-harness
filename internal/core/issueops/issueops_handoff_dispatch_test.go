package issueops

import (
	"context"
	"errors"
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
	client := handoffDispatchFake()
	_, err := StartIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffStartRequest{ID: record.ID, Confirm: true}, client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "worktree_tools_prepared") {
		t.Fatalf("expected readiness error, got %v", err)
	}
	if len(client.trace) != 0 {
		t.Fatalf("readiness failure called Orca: %v", client.trace)
	}
}

func TestHandoffStartPersistsStableContextBeforeMutation(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake()
	client.beforeTerminalCreate = func() {
		persisted, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			t.Fatal(err)
		}
		if persisted.ExecutionHandoff.ContextVersion != handoff.ContextVersion || len(persisted.ExecutionHandoff.ContextSHA256) != 64 || persisted.ExecutionHandoff.PendingOperation == nil {
			t.Fatalf("context and pending operation must precede mutation: %#v", persisted.ExecutionHandoff)
		}
	}
	got, err := StartIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffStartRequest{ID: record.ID, Confirm: true}, client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ContextSHA256) != 64 {
		t.Fatalf("missing stable context hash: %#v", got)
	}
}

func TestHandoffStartCreatesTerminalTaskDispatchExactlyOnce(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake()
	req := IssueOpsHandoffStartRequest{ID: record.ID, Confirm: true}
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
			client := handoffDispatchFake()
			tt.fail(client)
			req := IssueOpsHandoffStartRequest{ID: record.ID, Confirm: true}
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

func TestHandoffStartTerminalDeltaRequiresExactlyOne(t *testing.T) {
	tests := []struct {
		name string
		rows []port.OrcaTerminal
		ok   bool
	}{
		{name: "zero", rows: []port.OrcaTerminal{{PTYID: "old"}}},
		{name: "one", rows: []port.OrcaTerminal{{PTYID: "old"}, {PTYID: "new", Handle: "term-new", WorktreeID: "wt-1"}}, ok: true},
		{name: "multiple", rows: []port.OrcaTerminal{{PTYID: "new-1"}, {PTYID: "new-2"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReconcileIssueOpsHandoffTerminal([]string{"old"}, "wt-1", tt.rows)
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
	rows := []port.OrcaTask{{ID: "old", Title: "old"}, {ID: "new", Title: marker}}
	got, err := ReconcileIssueOpsHandoffTask([]string{"old"}, marker, rows)
	if err != nil || got.ID != "new" {
		t.Fatalf("got %#v err=%v", got, err)
	}
	if _, err := ReconcileIssueOpsHandoffTask(nil, marker, append(rows, port.OrcaTask{ID: "another", Title: marker})); err == nil {
		t.Fatal("multiple marker candidates must fail closed")
	}
}

func TestHandoffStartDispatchRecoveryRequiresPersistedTask(t *testing.T) {
	client := handoffDispatchFake()
	if _, err := ReconcileIssueOpsHandoffDispatch(context.Background(), "", client); err == nil {
		t.Fatal("missing persisted task must fail")
	}
	got, err := ReconcileIssueOpsHandoffDispatch(context.Background(), "task-1", client)
	if err != nil || got.ID != "dispatch-1" {
		t.Fatalf("got %#v err=%v", got, err)
	}
}

type dispatchOrcaFake struct {
	terminals            []port.OrcaTerminal
	tasks                []port.OrcaTask
	terminal             port.OrcaTerminal
	task                 port.OrcaTask
	dispatch             port.OrcaDispatch
	terminalErr          error
	taskErr              error
	dispatchErr          error
	beforeTerminalCreate func()
	terminalCreates      int
	taskCreates          int
	dispatchCalls        int
	trace                []string
}

func handoffDispatchFake() *dispatchOrcaFake {
	return &dispatchOrcaFake{
		terminal: port.OrcaTerminal{Handle: "term-1", PTYID: "pty-1", WorktreeID: "wt-1", Connected: true, Writable: true},
		task:     port.OrcaTask{ID: "task-1", Status: "pending"},
		dispatch: port.OrcaDispatch{ID: "dispatch-1", TaskID: "task-1", AssigneeHandle: "term-1", Status: "dispatched", Injected: true},
	}
}

func (f *dispatchOrcaFake) ListTerminals(context.Context, string) ([]port.OrcaTerminal, error) {
	f.trace = append(f.trace, "terminal-list")
	return append([]port.OrcaTerminal(nil), f.terminals...), nil
}

func (f *dispatchOrcaFake) CreateTerminal(context.Context, port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error) {
	f.trace = append(f.trace, "terminal-create")
	f.terminalCreates++
	if f.beforeTerminalCreate != nil {
		f.beforeTerminalCreate()
	}
	return f.terminal, f.terminalErr
}

func (f *dispatchOrcaFake) ListTasks(context.Context) ([]port.OrcaTask, error) {
	f.trace = append(f.trace, "task-list")
	return append([]port.OrcaTask(nil), f.tasks...), nil
}

func (f *dispatchOrcaFake) CreateTask(context.Context, port.OrcaCreateTaskRequest) (port.OrcaTask, error) {
	f.trace = append(f.trace, "task-create")
	f.taskCreates++
	return f.task, f.taskErr
}

func (f *dispatchOrcaFake) Dispatch(context.Context, port.OrcaDispatchRequest) (port.OrcaDispatch, error) {
	f.trace = append(f.trace, "dispatch")
	f.dispatchCalls++
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
			Orca: &IssueOpsOrcaIdentity{RuntimeID: "runtime-1", WorktreeID: "wt-1", WorktreeInstanceID: "inst-1", WorktreePath: worktree},
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
