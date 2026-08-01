package issueops

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"agent-harness/internal/port"
)

func TestExecutionActionReconcileUsesInjectedHandlerExactlyOnce(t *testing.T) {
	stateRoot, record, fake := pendingOrcaIntentFixture(t)
	calls := 0
	remoteCalls := 0
	want := ExecutionReconcileResult{OK: true, ID: record.ID, Reconciled: true, Code: "vertical"}
	result, err := ExecuteExecution(context.Background(), stateRoot, ExecutionActionRequest{
		Action: ExecutionActionReconcile, ID: record.ID, Confirm: true, CWD: record.Repo,
		Actor: executionActor("codex", "reconcile-handler"),
	}, ExecutionActionDependencies{
		Orca: fake, ReadIssue: executionIssueSnapshotReader,
		RemoteReconcile: func(context.Context, string, ExecutionReconcileRequest) (ExecutionReconcileResult, error) {
			remoteCalls++
			return ExecutionReconcileResult{}, nil
		},
		Reconcile: func(_ context.Context, gotRoot string, request ExecutionReconcileRequest, deps ExecutionReconcileDependencies) (ExecutionReconcileResult, error) {
			calls++
			if gotRoot != stateRoot || request.ID != record.ID || !request.Confirm || request.Snapshot == nil || request.Snapshot.Execution == nil || deps.ReadIssue == nil || deps.Orca != fake {
				t.Fatalf("root=%q request=%+v deps=%+v", gotRoot, request, deps)
			}
			return want, nil
		},
	})
	if err != nil || calls != 1 || remoteCalls != 0 {
		t.Fatalf("result=%#v calls=%d remoteCalls=%d err=%v", result, calls, remoteCalls, err)
	}
	if got, ok := result.(ExecutionReconcileResult); !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("result=%#v", result)
	}
}

func TestExecutionActionRemoteReconcileUsesPublicationHandlerWithoutLegacyOrOrcaFallback(t *testing.T) {
	stateRoot, fixture := pendingRemoteReconcileActionFixture(t)
	publicationCalls := 0
	legacyCalls := 0
	orcaCalls := 0
	want := ExecutionReconcileResult{OK: true, ID: fixture.record.ID, Reconciled: true, Code: "publication_vertical"}

	raw, err := ExecuteExecution(context.Background(), stateRoot, ExecutionActionRequest{
		Action: ExecutionActionReconcile, ID: fixture.record.ID, Confirm: true,
		Actor: fixture.actor, CWD: fixture.worktree,
	}, ExecutionActionDependencies{
		RemoteReconcile: func(_ context.Context, gotRoot string, request ExecutionReconcileRequest) (ExecutionReconcileResult, error) {
			publicationCalls++
			if gotRoot != stateRoot || request.ID != fixture.record.ID || !request.Confirm || request.Preview || request.Snapshot != nil {
				t.Fatalf("root=%q request=%+v", gotRoot, request)
			}
			return want, nil
		},
		RemotePR: RemotePullRequestDependencies{Reconcile: func(string, port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error) {
			legacyCalls++
			return port.IssueProviderReconcilePullRequestResult{}, errors.New("legacy fallback invoked")
		}},
		Reconcile: func(context.Context, string, ExecutionReconcileRequest, ExecutionReconcileDependencies) (ExecutionReconcileResult, error) {
			orcaCalls++
			return ExecutionReconcileResult{}, errors.New("Orca fallback invoked")
		},
	})
	if err != nil || publicationCalls != 1 || legacyCalls != 0 || orcaCalls != 0 {
		t.Fatalf("result=%#v publication=%d legacy=%d orca=%d err=%v", raw, publicationCalls, legacyCalls, orcaCalls, err)
	}
	if got, ok := raw.(ExecutionReconcileResult); !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("result=%#v", raw)
	}
}

func TestExecutionActionRemoteReconcileFailsClosedWithoutPublicationHandler(t *testing.T) {
	stateRoot, fixture := pendingRemoteReconcileActionFixture(t)
	legacyCalls := 0
	raw, err := ExecuteExecution(context.Background(), stateRoot, ExecutionActionRequest{
		Action: ExecutionActionReconcile, ID: fixture.record.ID, Confirm: true,
		Actor: fixture.actor, CWD: fixture.worktree,
	}, ExecutionActionDependencies{RemotePR: RemotePullRequestDependencies{
		Reconcile: func(string, port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error) {
			legacyCalls++
			return port.IssueProviderReconcilePullRequestResult{AuthoritativeZero: true}, nil
		},
	}})
	result, ok := raw.(ExecutionReconcileResult)
	if !ok || result.Code != "remote_reconcile_unavailable" || !errors.Is(err, ErrRemotePullRequestReconcileHandlerUnavailable) || legacyCalls != 0 {
		t.Fatalf("result=%#v legacyCalls=%d err=%v", raw, legacyCalls, err)
	}
}

func TestExecutionActionRemoteReconcilePreviewDoesNotCallAnyHandler(t *testing.T) {
	stateRoot, fixture := pendingRemoteReconcileActionFixture(t)
	publicationCalls := 0
	legacyCalls := 0
	orcaCalls := 0
	result, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
		ID: fixture.record.ID, Preview: true, Actor: fixture.actor, CWD: fixture.worktree,
	}, ExecutionReconcileDependencies{
		RemoteReconcile: func(context.Context, string, ExecutionReconcileRequest) (ExecutionReconcileResult, error) {
			publicationCalls++
			return ExecutionReconcileResult{}, nil
		},
		RemotePR: RemotePullRequestDependencies{Reconcile: func(string, port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error) {
			legacyCalls++
			return port.IssueProviderReconcilePullRequestResult{}, nil
		}},
		Handler: func(context.Context, string, ExecutionReconcileRequest, ExecutionReconcileDependencies) (ExecutionReconcileResult, error) {
			orcaCalls++
			return ExecutionReconcileResult{}, nil
		},
	})
	if err != nil || result.Code != "remote_reconcile_required" || result.ExternalStateInspected || publicationCalls != 0 || legacyCalls != 0 || orcaCalls != 0 {
		t.Fatalf("result=%#v publication=%d legacy=%d orca=%d err=%v", result, publicationCalls, legacyCalls, orcaCalls, err)
	}
}

func pendingRemoteReconcileActionFixture(t *testing.T) (string, remoteExecutionFixture) {
	t.Helper()
	stateRoot := t.TempDir()
	fixture := newRemoteExecutionFixture(t, stateRoot, "195-remote-action-reconcile")
	_, err := createRemotePullRequestLegacy(context.Background(), stateRoot, fixture.request("publication reconcile routing"), RemotePullRequestDependencies{
		Create: func(string, port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			return port.IssueProviderCreatePullRequestResult{}, errors.New("ambiguous provider outcome")
		},
	})
	if err == nil {
		t.Fatal("fixture create unexpectedly succeeded")
	}
	return stateRoot, fixture
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
