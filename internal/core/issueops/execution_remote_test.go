package issueops

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

type remoteExecutionFixture struct {
	claimableExecutionFixture
	actor model.NativeActor
}

func TestRemotePullRequestPersistsIntentBeforeSingleProviderCallWithActiveLease(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "69-remote-intent")
	record := fixture.record
	record.IssueURL = "https://github.com/example/agent-harness/issues/69"
	record.Phase = model.IssueOpsPhasePR
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	actor := executionActor("codex", "remote-intent-session")
	if _, err := claimExecution(stateRoot, ExecutionClaimRequest{
		ID: record.ID, Generation: 1, Actor: actor, CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	}); err != nil {
		t.Fatal(err)
	}

	wantTitle := "IssueOps v1 검증"
	wantBody := "## 요약\n\nIssueOps v1 구현과 검증 결과를 기록합니다."
	providerCalls := 0
	result, err := CreateRemotePullRequest(context.Background(), stateRoot, RemotePullRequestRequest{
		ID: record.ID, Provider: "github", Title: wantTitle, Body: wantBody, Head: record.Branch, Base: "main",
		Labels: []string{"enhancement"}, Assignees: []string{"maintainer"}, Confirm: true,
		ExpectedGeneration: 1, Actor: actor, CWD: fixture.worktree,
	}, RemotePullRequestDependencies{Create: func(provider string, request port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
		providerCalls++
		pending, readErr := ReadIssueOps(stateRoot, record.ID)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if pending.Execution == nil || pending.Execution.Pending == nil {
			t.Fatal("provider was invoked before the external intent became durable")
		}
		if provider != "github" || !request.Confirm || !request.Draft || request.Title != wantTitle ||
			!strings.HasPrefix(request.Body, wantBody+"\n\n<!-- agent-harness:issueops-v1 operation=") || !strings.HasSuffix(request.Body, " -->") ||
			request.HeadBranch != record.Branch || request.BaseBranch != "main" ||
			strings.Join(request.Labels, ",") != "enhancement" || strings.Join(request.Assignees, ",") != "maintainer" {
			t.Fatalf("provider request = %#v provider=%q", request, provider)
		}
		return port.IssueProviderCreatePullRequestResult{OK: true, URL: "https://github.com/example/agent-harness/pull/1", Number: "1"}, nil
	}})
	if err != nil {
		t.Fatalf("create remote pull request: %v", err)
	}
	if providerCalls != 1 || result.URL == "" {
		t.Fatalf("provider calls=%d result=%#v", providerCalls, result)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution.Pending != nil || persisted.RemoteArtifact == nil || persisted.RemoteArtifact.TargetBranch != "main" ||
		strings.Join(persisted.RemoteArtifact.Labels, ",") != "enhancement" || strings.Join(persisted.RemoteArtifact.Assignees, ",") != "maintainer" {
		t.Fatalf("remote receipt was not committed atomically: %#v", persisted)
	}
}

func TestRemotePullRequestAuthoritativeZeroRetriesOnlyProvenNotInvokedOnce(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newRemoteExecutionFixture(t, stateRoot, "69-remote-zero-retry")
	initialCalls := 0
	_, err := CreateRemotePullRequest(context.Background(), stateRoot, fixture.request("retry once"), RemotePullRequestDependencies{
		Create: func(string, port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			initialCalls++
			return port.IssueProviderCreatePullRequestResult{}, &port.IssueProviderCreateError{Invoked: false, Err: errors.New("preflight rejected")}
		},
	})
	if err == nil || initialCalls != 1 {
		t.Fatalf("initial create err=%v calls=%d", err, initialCalls)
	}

	retryCalls := 0
	result, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
		ID: fixture.record.ID, Confirm: true, Actor: fixture.actor, CWD: fixture.worktree,
	}, ExecutionReconcileDependencies{RemotePR: RemotePullRequestDependencies{
		Reconcile: func(string, port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error) {
			return port.IssueProviderReconcilePullRequestResult{AuthoritativeZero: true}, nil
		},
		Create: func(string, port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			retryCalls++
			return port.IssueProviderCreatePullRequestResult{OK: true, URL: "https://github.com/example/agent-harness/pull/3", Number: "3"}, nil
		},
	}})
	if err != nil {
		t.Fatalf("reconcile proven zero: %v", err)
	}
	if retryCalls != 1 || !result.Reconciled || result.Code != "remote_reconcile_retry_succeeded" {
		t.Fatalf("retry calls=%d result=%#v", retryCalls, result)
	}
	if persisted, readErr := ReadIssueOps(stateRoot, fixture.record.ID); readErr != nil || persisted.Execution.Pending != nil || persisted.RemoteArtifact == nil {
		t.Fatalf("retry receipt persisted=%#v err=%v", persisted, readErr)
	}
}

func TestRemotePullRequestZeroUnprovenAndMultipleCandidatesRetainIntentWithoutRetry(t *testing.T) {
	for _, tc := range []struct {
		name      string
		inventory port.IssueProviderReconcilePullRequestResult
		wantCode  string
	}{
		{name: "authoritative-zero-unproven", inventory: port.IssueProviderReconcilePullRequestResult{AuthoritativeZero: true}, wantCode: "remote_reconcile_zero_unproven"},
		{name: "multiple", inventory: port.IssueProviderReconcilePullRequestResult{Candidates: []port.IssueProviderReconcilePullRequestCandidate{{URL: "one"}, {URL: "two"}}}, wantCode: "remote_reconcile_multiple"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			fixture := newRemoteExecutionFixture(t, stateRoot, "69-remote-"+tc.name)
			_, err := CreateRemotePullRequest(context.Background(), stateRoot, fixture.request(tc.name), RemotePullRequestDependencies{
				Create: func(string, port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
					return port.IssueProviderCreatePullRequestResult{}, errors.New("ambiguous transport")
				},
			})
			if err == nil {
				t.Fatal("ambiguous create unexpectedly succeeded")
			}
			retryCalls := 0
			result, reconcileErr := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
				ID: fixture.record.ID, Confirm: true, Actor: fixture.actor, CWD: fixture.worktree,
			}, ExecutionReconcileDependencies{RemotePR: RemotePullRequestDependencies{
				Reconcile: func(string, port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error) {
					return tc.inventory, nil
				},
				Create: func(string, port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
					retryCalls++
					return port.IssueProviderCreatePullRequestResult{}, nil
				},
			}})
			if reconcileErr == nil || result.Code != tc.wantCode || retryCalls != 0 {
				t.Fatalf("result=%#v err=%v retryCalls=%d", result, reconcileErr, retryCalls)
			}
			persisted, readErr := ReadIssueOps(stateRoot, fixture.record.ID)
			if readErr != nil || persisted.Execution.Pending == nil || persisted.RemoteArtifact != nil {
				t.Fatalf("intent not retained: %#v err=%v", persisted, readErr)
			}
		})
	}
}

func TestRemotePullRequestOneExactCandidateIsAdopted(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newRemoteExecutionFixture(t, stateRoot, "69-remote-adopt")
	_, err := CreateRemotePullRequest(context.Background(), stateRoot, fixture.request("adopt exact"), RemotePullRequestDependencies{
		Create: func(string, port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			return port.IssueProviderCreatePullRequestResult{}, errors.New("ambiguous transport")
		},
	})
	if err == nil {
		t.Fatal("ambiguous create unexpectedly succeeded")
	}
	pending, err := ReadIssueOps(stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := readExternalRemotePRPayload(stateRoot, pending.Execution.Pending.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	expected := remotePullRequestReconcileRequest(payload)
	candidate := port.IssueProviderReconcilePullRequestCandidate{
		URL: "https://github.com/example/agent-harness/pull/4", ProjectKey: expected.ProjectKey, SourceProjectKey: expected.ProjectKey,
		HeadBranch: expected.HeadBranch, BaseBranch: expected.BaseBranch, HeadSHA: expected.ExpectedHeadSHA,
		Title: expected.Title, BodySHA256: expected.BodySHA256, Labels: expected.Labels, Assignees: expected.Assignees, Draft: expected.Draft,
	}
	result, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
		ID: fixture.record.ID, Confirm: true, Actor: fixture.actor, CWD: fixture.worktree,
	}, ExecutionReconcileDependencies{RemotePR: RemotePullRequestDependencies{
		Reconcile: func(string, port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error) {
			return port.IssueProviderReconcilePullRequestResult{Candidates: []port.IssueProviderReconcilePullRequestCandidate{candidate}}, nil
		},
	}})
	if err != nil || !result.Reconciled || result.Code != "remote_reconcile_adopted" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestRemotePullRequestBlocksGenerationReplacementWhileIntentIsPending(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "69-remote-race")
	record := fixture.record
	record.IssueURL = "https://github.com/example/agent-harness/issues/69"
	record.Phase = model.IssueOpsPhasePR
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	owner := executionActor("codex", "remote-race-owner")
	if _, err := claimExecution(stateRoot, ExecutionClaimRequest{
		ID: record.ID, Generation: 1, Actor: owner, CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	}); err != nil {
		t.Fatal(err)
	}

	providerEntered := make(chan struct{})
	providerReturn := make(chan struct{})
	createErr := make(chan error, 1)
	go func() {
		_, err := CreateRemotePullRequest(context.Background(), stateRoot, RemotePullRequestRequest{
			ID: record.ID, Provider: "github", Title: "IssueOps v1 race", Head: record.Branch, Base: "main",
			Labels: []string{"enhancement"}, Assignees: []string{"maintainer"}, Confirm: true,
			ExpectedGeneration: 1, Actor: owner, CWD: fixture.worktree,
		}, RemotePullRequestDependencies{Create: func(string, port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			close(providerEntered)
			<-providerReturn
			return port.IssueProviderCreatePullRequestResult{OK: true, URL: "https://github.com/example/agent-harness/pull/2", Number: "2"}, nil
		}})
		createErr <- err
	}()
	<-providerEntered

	replacer := executionActor("claude", "remote-race-replacer")
	preview, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1, Actor: replacer, CWD: record.Repo,
	})
	if err != nil {
		t.Fatalf("read-only replacement preview should remain available: %v", err)
	}
	_, err = ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "owner became unavailable",
		Actor: replacer, CWD: record.Repo, Confirm: true,
	})
	if err == nil || !strings.Contains(err.Error(), "pending external intent") {
		t.Fatalf("replacement must not change generation during a remote external intent: %v", err)
	}
	close(providerReturn)
	if err := <-createErr; err != nil {
		t.Fatalf("same-generation provider receipt must finish after replacement was blocked: %v", err)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.RemoteArtifact == nil || persisted.Execution.Pending != nil || persisted.Execution.Lease.Generation != 1 || persisted.Execution.Lease.Status != model.LeaseStatusActive {
		t.Fatalf("pending replacement changed or lost the same-generation receipt: %#v", persisted)
	}
}

func newRemoteExecutionFixture(t *testing.T, stateRoot, branch string) remoteExecutionFixture {
	t.Helper()
	fixture := newClaimableExecutionFixture(t, stateRoot, branch)
	record := fixture.record
	record.IssueURL = "https://github.com/example/agent-harness/issues/69"
	record.Phase = model.IssueOpsPhasePR
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	actor := executionActor("codex", branch+"-session")
	if _, err := claimExecution(stateRoot, ExecutionClaimRequest{
		ID: record.ID, Generation: 1, Actor: actor, CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	}); err != nil {
		t.Fatal(err)
	}
	fixture.record = record
	return remoteExecutionFixture{claimableExecutionFixture: fixture, actor: actor}
}

func (fixture remoteExecutionFixture) request(title string) RemotePullRequestRequest {
	return RemotePullRequestRequest{
		ID: fixture.record.ID, Provider: "github", Title: title, Head: fixture.record.Branch, Base: "main",
		Labels: []string{"enhancement"}, Assignees: []string{"maintainer"}, ExpectedGeneration: 1,
		Actor: fixture.actor, CWD: fixture.worktree, Confirm: true,
	}
}
