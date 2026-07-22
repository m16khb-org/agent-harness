package issueops

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

func TestExecutionV1OrcaPersistsIntentBeforeExternalMutationAndCASReceipt(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
	fake.prepare = func(workspace port.ExecutionWorkspaceRequest, request port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
		pending, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			return port.ExecutionOrcaWorkspaceReceipt{}, err
		}
		if pending.Execution == nil || pending.Execution.Pending == nil || pending.Execution.Pending.Kind != "worktree_create" {
			t.Fatalf("external mutation ran before its durable intent: %#v", pending.Execution)
		}
		if request.Marker != pending.Execution.Pending.Marker || request.Marker == "" {
			t.Fatalf("adapter marker must equal the durable intent marker: request=%q pending=%#v", request.Marker, pending.Execution.Pending)
		}
		if err := os.MkdirAll(workspace.Root, 0o755); err != nil {
			return port.ExecutionOrcaWorkspaceReceipt{}, err
		}
		return executionOrcaWorkspaceReceipt(workspace), nil
	}

	got, err := PrepareExecutionV1(context.Background(), stateRoot, ExecutionPrepareRequestV1{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActorV1("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model", OwnerEffort: "high",
	}, ExecutionPrepareDependenciesV1{Orca: fake, ReadIssue: executionIssueSnapshotReaderV1})
	if err != nil {
		t.Fatal(err)
	}
	if fake.prepareCalls != 1 || got.Execution == nil || got.Execution.Pending != nil || got.Execution.Orca == nil {
		t.Fatalf("Orca receipt was not CAS-persisted exactly once: calls=%d result=%#v", fake.prepareCalls, got)
	}
	if got.Execution.Lease.Status != model.LeaseStatusClaimable || got.ClaimTokenPath == "" {
		t.Fatalf("verified dispatch must produce one claimable lease: %#v", got)
	}
	if got.Execution.Orca.OwnerModel != "caller-model" || got.Execution.Orca.OwnerEffort != "high" {
		t.Fatalf("caller owner profile was not preserved: %#v", got.Execution.Orca)
	}
}

func TestExecutionV1OrcaAmbiguityNeverFallsBackOrRepeatsExternalMutation(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	direct := &executionDirectCountingFake{}
	fake := &executionOrcaFake{
		probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true},
		prepare: func(port.ExecutionWorkspaceRequest, port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
			return port.ExecutionOrcaWorkspaceReceipt{}, errors.New("ambiguous external outcome")
		},
	}
	req := ExecutionPrepareRequestV1{
		ID: record.ID, Mode: ExecutionModeAuto, CWD: record.Repo, Confirm: true,
		Actor: executionActorV1("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
	}
	if _, err := PrepareExecutionV1(context.Background(), stateRoot, req, ExecutionPrepareDependenciesV1{Direct: direct, Orca: fake, ReadIssue: executionIssueSnapshotReaderV1}); err == nil {
		t.Fatal("ambiguous Orca outcome must require reconcile")
	}
	pending, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Execution == nil || pending.Execution.Mode != model.ExecutionModeOrca || pending.Execution.Pending == nil {
		t.Fatalf("ambiguous outcome must remain Orca with a durable pending intent: %#v", pending.Execution)
	}
	if _, err := PrepareExecutionV1(context.Background(), stateRoot, req, ExecutionPrepareDependenciesV1{Direct: direct, Orca: fake, ReadIssue: executionIssueSnapshotReaderV1}); err == nil {
		t.Fatal("second prepare must direct the caller to reconcile")
	}
	if fake.prepareCalls != 1 || direct.calls != 0 {
		t.Fatalf("external ambiguity repeated mutation or fell back: orca=%d direct=%d", fake.prepareCalls, direct.calls)
	}
}

func TestExecutionV1ConcurrentOrcaPrepareInvokesDriverOnce(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	entered, release := make(chan struct{}), make(chan struct{})
	fake := &executionOrcaFake{
		probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true},
		prepare: func(workspace port.ExecutionWorkspaceRequest, _ port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
			close(entered)
			<-release
			return port.ExecutionOrcaWorkspaceReceipt{}, errors.New("interrupted")
		},
	}
	req := ExecutionPrepareRequestV1{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActorV1("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
	}
	var firstErr error
	done := make(chan struct{})
	go func() {
		_, firstErr = PrepareExecutionV1(context.Background(), stateRoot, req, ExecutionPrepareDependenciesV1{Orca: fake, ReadIssue: executionIssueSnapshotReaderV1})
		close(done)
	}()
	<-entered
	if _, err := PrepareExecutionV1(context.Background(), stateRoot, req, ExecutionPrepareDependenciesV1{Orca: fake, ReadIssue: executionIssueSnapshotReaderV1}); err == nil {
		t.Fatal("concurrent retry must observe pending intent and stop")
	}
	close(release)
	<-done
	if firstErr == nil || fake.prepareCalls != 1 {
		t.Fatalf("driver must be invoked once: calls=%d firstErr=%v", fake.prepareCalls, firstErr)
	}
}

type executionOrcaFake struct {
	mu           sync.Mutex
	probe        port.ExecutionOrcaProbeResult
	prepare      func(port.ExecutionWorkspaceRequest, port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error)
	launch       func(port.ExecutionOrcaWorkspaceReceipt, port.ExecutionOrcaProbeRequest, port.ExecutionOrcaLaunchRequest) (port.ExecutionOrcaReceipt, error)
	inspect      func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error)
	invoke       func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error)
	prepareCalls int
	launchCalls  int
}

func (f *executionOrcaFake) Probe(context.Context, port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	return f.probe, nil
}

func (f *executionOrcaFake) InspectIntent(_ context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	if f.inspect != nil {
		return f.inspect(request)
	}
	return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{}, AuthoritativeZero: true}, nil
}

func (f *executionOrcaFake) InvokeIntent(_ context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	if f.invoke != nil {
		return f.invoke(request)
	}
	switch request.Stage {
	case port.ExecutionOrcaIntentWorktree:
		f.mu.Lock()
		f.prepareCalls++
		f.mu.Unlock()
		var prepared port.ExecutionOrcaWorkspaceReceipt
		var err error
		if f.prepare != nil {
			prepared, err = f.prepare(request.Workspace, request.Probe)
		} else {
			prepared = executionOrcaWorkspaceReceipt(request.Workspace)
		}
		if err != nil {
			return port.ExecutionOrcaIntentReceipt{}, err
		}
		return port.ExecutionOrcaIntentReceipt{Workspace: &prepared}, nil
	case port.ExecutionOrcaIntentTerminal:
		return port.ExecutionOrcaIntentReceipt{TerminalPTYID: "pty-1", TerminalHandle: "terminal-1"}, nil
	case port.ExecutionOrcaIntentTask:
		return port.ExecutionOrcaIntentReceipt{TaskID: "task-1"}, nil
	case port.ExecutionOrcaIntentDispatch:
		f.mu.Lock()
		f.launchCalls++
		f.mu.Unlock()
		if f.launch != nil {
			receipt, err := f.launch(*request.Prepared, request.Probe, *request.Launch)
			if err != nil {
				return port.ExecutionOrcaIntentReceipt{}, err
			}
			return port.ExecutionOrcaIntentReceipt{TaskID: receipt.TaskID, DispatchID: receipt.DispatchID}, nil
		}
		return port.ExecutionOrcaIntentReceipt{TaskID: request.TaskID, DispatchID: "dispatch-1"}, nil
	default:
		return port.ExecutionOrcaIntentReceipt{}, errors.New("unsupported fake Orca intent stage")
	}
}

func executionOrcaWorkspaceReceipt(workspace port.ExecutionWorkspaceRequest) port.ExecutionOrcaWorkspaceReceipt {
	return port.ExecutionOrcaWorkspaceReceipt{
		Workspace: port.ExecutionWorkspaceReceipt{
			SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch,
			BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
		},
		RuntimeID: "runtime-1", RepoID: "repo-1", WorktreeID: "worktree-1", WorktreeInstanceID: "instance-1",
	}
}

func executionOrcaReceipt(prepared port.ExecutionOrcaWorkspaceReceipt) port.ExecutionOrcaReceipt {
	return port.ExecutionOrcaReceipt{
		Workspace: prepared.Workspace,
		RuntimeID: prepared.RuntimeID, RepoID: prepared.RepoID, WorktreeID: prepared.WorktreeID, WorktreeInstanceID: prepared.WorktreeInstanceID,
		TaskID: "task-1", DispatchID: "dispatch-1", TerminalPTYID: "pty-1",
	}
}

func executionIssueSnapshotReaderV1(_ context.Context, _ string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
	body := "## acceptance criteria\n\n- [ ] AC-01: first\n- [ ] AC-23: last\n\n## 검증 명령\n\n```bash\ngo test ./... -count=1\ngo vet ./...\n```\n"
	return port.ExecutionIssueSnapshot{URL: request.URL, Body: body}, nil
}

type executionDirectCountingFake struct{ calls int }

func (f *executionDirectCountingFake) Prepare(context.Context, port.ExecutionWorkspaceRequest) (port.ExecutionWorkspaceReceipt, error) {
	f.calls++
	return port.ExecutionWorkspaceReceipt{}, nil
}
