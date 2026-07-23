package issueops

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

func TestExecutionOrcaCrashAfterMutationReconcilesExactlyOneCandidateWithoutDuplicate(t *testing.T) {
	tests := []struct {
		name      string
		stage     port.ExecutionOrcaIntentStage
		nextKind  string
		completed bool
	}{
		{name: "worktree", stage: port.ExecutionOrcaIntentWorktree, nextKind: "owner_launch"},
		{name: "terminal", stage: port.ExecutionOrcaIntentTerminal, nextKind: "owner_launch"},
		{name: "task", stage: port.ExecutionOrcaIntentTask, nextKind: "dispatch"},
		{name: "dispatch", stage: port.ExecutionOrcaIntentDispatch, completed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateRoot, record := executionPrepareRecord(t)
			calls := map[port.ExecutionOrcaIntentStage]int{}
			var crashedRequest port.ExecutionOrcaIntentRequest
			fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
			fake.invoke = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
				calls[request.Stage]++
				if request.Stage == test.stage {
					crashedRequest = request
					return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "injected_timeout", Invoked: true, Timeout: true}
				}
				return successfulExecutionOrcaIntentReceipt(t, request), nil
			}
			req := ExecutionPrepareRequest{
				ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
				Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model", OwnerEffort: "high",
			}
			if _, err := PrepareExecution(context.Background(), stateRoot, req, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader}); err == nil {
				t.Fatal("injected post-mutation crash must leave a pending intent")
			}
			pending, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			payload, err := readExternalOrcaIntentPayload(stateRoot, pending.Execution.Pending.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			if payload.Stage != test.stage || payload.InvocationState != orcaIntentUnknown || calls[test.stage] != 1 {
				t.Fatalf("crash receipt was not fenced at %s: payload=%#v calls=%v", test.stage, payload, calls)
			}
			db, err := sqlstore.Open(stateRoot)
			if err != nil {
				t.Fatal(err)
			}
			raw, ok, err := db.Get(externalIntentBucket, payload.OperationID)
			if err != nil || !ok {
				t.Fatalf("read raw Orca intent: ok=%t err=%v", ok, err)
			}
			if bytes.Contains(raw, []byte(`"terminal_handle"`)) || bytes.Contains(raw, []byte("terminal-1")) {
				t.Fatalf("runtime-scoped terminal handle leaked into durable intent: %s", raw)
			}

			candidate := successfulExecutionOrcaIntentReceipt(t, crashedRequest)
			fake.inspect = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
				if request.Stage != test.stage {
					t.Fatalf("reconcile inspected %s, want %s", request.Stage, test.stage)
				}
				return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{candidate}}, nil
			}
			result, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
				ID: record.ID, Confirm: true, Actor: executionActor("codex", "fresh-reconciler"), CWD: record.Repo,
			}, ExecutionReconcileDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
			if err != nil {
				t.Fatal(err)
			}
			if !result.Reconciled || calls[test.stage] != 1 {
				t.Fatalf("reconcile repeated the external mutation: result=%#v calls=%v", result, calls)
			}
			if test.completed {
				if result.Pending != nil || result.Execution.Lease.Status != model.LeaseStatusClaimable || result.Execution.Orca == nil {
					t.Fatalf("dispatch adoption did not finalize claimable authority: %#v", result)
				}
				if _, err := readExternalOrcaIntentPayload(stateRoot, payload.OperationID); err == nil {
					t.Fatal("completed dispatch left an external intent payload")
				}
			} else if result.Pending == nil || result.Pending.Kind != test.nextKind {
				t.Fatalf("reconcile must advance exactly one stage: %#v", result)
			}
		})
	}
}

func TestExecutionOrcaReconcileZeroMultipleAndTransportAmbiguityNeverMutate(t *testing.T) {
	for _, test := range []struct {
		name    string
		inspect func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error)
	}{
		{name: "authoritative zero after unknown invocation", inspect: func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
			return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{}, AuthoritativeZero: true}, nil
		}},
		{name: "multiple", inspect: func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
			candidate := successfulExecutionOrcaIntentReceipt(t, request)
			return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{candidate, candidate}}, nil
		}},
		{name: "transport", inspect: func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
			return port.ExecutionOrcaIntentInventory{}, errors.New("inventory transport unavailable")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateRoot, record := executionPrepareRecord(t)
			invokeCalls := 0
			fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
			fake.invoke = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
				invokeCalls++
				return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "timeout", Invoked: true, Timeout: true}
			}
			req := ExecutionPrepareRequest{
				ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
				Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
			}
			_, _ = PrepareExecution(context.Background(), stateRoot, req, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
			fake.inspect = test.inspect
			if _, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
				ID: record.ID, Confirm: true, Actor: executionActor("claude", "fresh"), CWD: record.Repo,
			}, ExecutionReconcileDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader}); err == nil {
				t.Fatal("ambiguous inventory must retain the intent")
			}
			if invokeCalls != 1 {
				t.Fatalf("ambiguous reconcile repeated external mutation: calls=%d", invokeCalls)
			}
			current, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil || current.Execution.Pending == nil || current.Execution.Lease.Status != model.LeaseStatusReleased {
				t.Fatalf("ambiguous reconcile changed authority: record=%#v err=%v", current.Execution, err)
			}
		})
	}
}

func TestExecutionOrcaReconcileRetriesOnlyProvenNotInvokedAndOnlyOnce(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	invokeCalls := 0
	fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
	fake.invoke = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
		invokeCalls++
		if invokeCalls <= 2 {
			return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "pre_invocation_rejected", Invoked: false}
		}
		return successfulExecutionOrcaIntentReceipt(t, request), nil
	}
	req := ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
	}
	_, _ = PrepareExecution(context.Background(), stateRoot, req, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
	fake.inspect = func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{}, AuthoritativeZero: true}, nil
	}
	reconcile := func() error {
		_, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
			ID: record.ID, Confirm: true, Actor: executionActor("codex", "fresh"), CWD: record.Repo,
		}, ExecutionReconcileDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
		return err
	}
	if err := reconcile(); err == nil {
		t.Fatal("the one permitted retry was also proven not invoked and must remain pending")
	}
	if invokeCalls != 2 {
		t.Fatalf("expected initial attempt plus one proven-not-invoked retry, got %d", invokeCalls)
	}
	if err := reconcile(); err == nil {
		t.Fatal("exhausted retry must fail closed")
	}
	if invokeCalls != 2 {
		t.Fatalf("exhausted retry invoked Orca again: %d", invokeCalls)
	}
}

func TestExecutionOrcaReceiptCASRejectsConcurrentIntentChange(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
	fake.invoke = func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
		return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "timeout", Invoked: true}
	}
	req := ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
	}
	_, _ = PrepareExecution(context.Background(), stateRoot, req, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
	fake.inspect = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		err := withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
			current, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				return err
			}
			current.Execution.Pending.Marker += "-changed"
			_, err = persistExecutionTransition(stateRoot, current, nil)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{successfulExecutionOrcaIntentReceipt(t, request)}}, nil
	}
	if _, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
		ID: record.ID, Confirm: true, Actor: executionActor("claude", "fresh"), CWD: record.Repo,
	}, ExecutionReconcileDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader}); err == nil {
		t.Fatal("receipt CAS must reject a concurrent intent identity change")
	}
	current, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil || current.Execution.Pending == nil || current.Execution.Lease.Status != model.LeaseStatusReleased {
		t.Fatalf("failed CAS changed writer authority: record=%#v err=%v", current.Execution, err)
	}
}

func successfulExecutionOrcaIntentReceipt(t *testing.T, request port.ExecutionOrcaIntentRequest) port.ExecutionOrcaIntentReceipt {
	t.Helper()
	switch request.Stage {
	case port.ExecutionOrcaIntentWorktree:
		if err := os.MkdirAll(request.Workspace.Root, 0o700); err != nil {
			t.Fatal(err)
		}
		prepared := executionOrcaWorkspaceReceipt(request.Workspace)
		return port.ExecutionOrcaIntentReceipt{Workspace: &prepared}
	case port.ExecutionOrcaIntentTerminal:
		return port.ExecutionOrcaIntentReceipt{TerminalPTYID: "pty-1", TerminalHandle: "terminal-1"}
	case port.ExecutionOrcaIntentTask:
		return port.ExecutionOrcaIntentReceipt{TaskID: "task-1"}
	case port.ExecutionOrcaIntentDispatch:
		return port.ExecutionOrcaIntentReceipt{TaskID: request.TaskID, DispatchID: "dispatch-1"}
	default:
		t.Fatalf("unsupported stage %q", request.Stage)
		return port.ExecutionOrcaIntentReceipt{}
	}
}
