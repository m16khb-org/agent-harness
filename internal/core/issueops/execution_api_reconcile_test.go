package issueops

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestExecutionActionReconcileUsesInjectedHandlerExactlyOnce(t *testing.T) {
	stateRoot, record, fake := pendingOrcaIntentFixture(t)
	calls := 0
	want := ExecutionReconcileResult{OK: true, ID: record.ID, Reconciled: true, Code: "vertical"}
	result, err := ExecuteExecution(context.Background(), stateRoot, ExecutionActionRequest{
		Action: ExecutionActionReconcile, ID: record.ID, Confirm: true, CWD: record.Repo,
		Actor: executionActor("codex", "reconcile-handler"),
	}, ExecutionActionDependencies{
		Orca: fake, ReadIssue: executionIssueSnapshotReader,
		Reconcile: func(_ context.Context, gotRoot string, request ExecutionReconcileRequest, deps ExecutionReconcileDependencies) (ExecutionReconcileResult, error) {
			calls++
			if gotRoot != stateRoot || request.ID != record.ID || !request.Confirm || request.Snapshot == nil || request.Snapshot.Execution == nil || deps.ReadIssue == nil || deps.Orca != fake {
				t.Fatalf("root=%q request=%+v deps=%+v", gotRoot, request, deps)
			}
			return want, nil
		},
	})
	if err != nil || calls != 1 {
		t.Fatalf("result=%#v calls=%d err=%v", result, calls, err)
	}
	if got, ok := result.(ExecutionReconcileResult); !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("result=%#v", result)
	}
}

func TestExecutionActionReconcileFailsClosedWithoutInjectedHandler(t *testing.T) {
	stateRoot, record, fake := pendingOrcaIntentFixture(t)
	result, err := ExecuteExecution(context.Background(), stateRoot, ExecutionActionRequest{
		Action: ExecutionActionReconcile, ID: record.ID, Confirm: true, CWD: record.Repo,
		Actor: executionActor("codex", "reconcile-handler"),
	}, ExecutionActionDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
	if !errors.Is(err, ErrReconcileHandlerUnavailable) {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestExecutionActionReconcilePreviewDoesNotCallInjectedHandler(t *testing.T) {
	stateRoot, record, fake := pendingOrcaIntentFixture(t)
	calls := 0
	result, err := ExecuteExecution(context.Background(), stateRoot, ExecutionActionRequest{
		Action: ExecutionActionReconcile, ID: record.ID, Preview: true, CWD: record.Repo,
		Actor: executionActor("codex", "reconcile-handler"),
	}, ExecutionActionDependencies{Orca: fake, Reconcile: func(context.Context, string, ExecutionReconcileRequest, ExecutionReconcileDependencies) (ExecutionReconcileResult, error) {
		calls++
		return ExecutionReconcileResult{}, nil
	}})
	if err != nil || calls != 0 {
		t.Fatalf("result=%#v calls=%d err=%v", result, calls, err)
	}
	got := result.(ExecutionReconcileResult)
	if got.Code != "orca_reconcile_required" || got.ExternalStateInspected {
		t.Fatalf("result=%#v", got)
	}
}
