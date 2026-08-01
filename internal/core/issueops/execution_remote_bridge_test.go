package issueops

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

func TestRemotePublicationBridgePreservesLegacyTransitionBytes(t *testing.T) {
	for _, transition := range []string{"begin", "failure", "retry", "adopt", "terminal-not-invoked"} {
		t.Run(transition, func(t *testing.T) {
			legacy := runLegacyRemotePublicationTransition(t, transition)
			vertical := runRemotePublicationBridgeTransition(t, transition)

			if !bytes.Equal(legacy.intentRaw, vertical.intentRaw) {
				t.Fatalf("external intent bytes drift\nlegacy=%x\nvertical=%x", legacy.intentRaw, vertical.intentRaw)
			}
			if !bytes.Equal(legacy.recordRaw, vertical.recordRaw) {
				t.Fatalf("record bytes drift\nlegacy=%x\nvertical=%x", legacy.recordRaw, vertical.recordRaw)
			}
			if !reflect.DeepEqual(legacy.record.Execution.Pending, vertical.record.Execution.Pending) ||
				!reflect.DeepEqual(legacy.record.Execution.Failure, vertical.record.Execution.Failure) ||
				!reflect.DeepEqual(legacy.record.RemoteArtifact, vertical.record.RemoteArtifact) {
				t.Fatalf("decoded transition drift\nlegacy=%#v\nvertical=%#v", legacy.record, vertical.record)
			}

			switch transition {
			case "begin":
				assertRemotePublicationBridgeGolden(t, "intent.golden.json", vertical.intentRaw)
			case "failure":
				assertRemotePublicationBridgeGolden(t, "failure_record.golden.json", vertical.recordRaw)
			}
		})
	}
}

func TestPrepareRemotePublicationPreservesPreview(t *testing.T) {
	stateRoot, record, _, _ := legacyRemotePublicationV1Fixture(t)
	record.IssueURL = "https://github.com/example/agent-harness/issues/195"
	record.BranchPrepare = &IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch,
		BaseBranch: "117-hexagonal-architecture-migration", LinkVerified: true,
	}
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}

	prepared, err := PrepareRemotePublication(context.Background(), stateRoot, RemotePullRequestRequest{
		ID: record.ID, Provider: "github", Title: "Publication preview", Body: "Preview body.",
		Head: record.Branch, Base: record.BranchPrepare.BaseBranch,
		Labels: []string{" enhancement "}, Assignees: []string{" maintainer "}, Confirm: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Record.ID != record.ID || prepared.Provider != "github" || prepared.Kind != "pr" {
		t.Fatalf("prepared identity = %#v", prepared)
	}
	want := port.IssueProviderCreatePullRequestRequest{
		Repo: record.Repo, ProjectKey: "github.com/example/agent-harness",
		Title: "Publication preview", Body: "Preview body.", HeadBranch: record.Branch,
		BaseBranch: record.BranchPrepare.BaseBranch, Labels: []string{"enhancement"},
		Assignees: []string{"maintainer"}, Draft: true,
	}
	if !reflect.DeepEqual(prepared.Request, want) {
		t.Fatalf("prepared request = %#v, want %#v", prepared.Request, want)
	}
}

func TestRemotePublicationBridgeVerificationUsesDurableIntent(t *testing.T) {
	stateRoot, record, actor, providerReq := legacyRemotePublicationV1Fixture(t)
	record.IssueURL = "https://github.com/example/agent-harness/issues/195"
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	state, err := BeginRemotePublicationIntent(
		context.Background(), stateRoot, record, actor, record.Execution.Workspace.Root,
		record.Execution.Lease.Generation, providerReq, "github", "pr",
		RemotePublicationBridgeDependencies{
			Now:            func() time.Time { return time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC) },
			NewOperationID: func() (string, error) { return legacyPublicationOperationID, nil },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := remotePublicationPayload(state)
	if err != nil {
		t.Fatal(err)
	}
	expected := remotePullRequestReconcileRequest(payload)
	candidate := port.IssueProviderReconcilePullRequestCandidate{
		URL: "https://github.com/example/agent-harness/pull/7", ProjectKey: expected.ProjectKey,
		SourceProjectKey: expected.ProjectKey, HeadBranch: expected.HeadBranch,
		BaseBranch: expected.BaseBranch, HeadSHA: expected.ExpectedHeadSHA,
		Title: expected.Title, BodySHA256: expected.BodySHA256, Labels: expected.Labels,
		Assignees: expected.Assignees, Draft: expected.Draft,
	}
	if err := VerifyRemotePublicationCandidate(context.Background(), stateRoot, state, candidate); err != nil {
		t.Fatalf("exact durable candidate rejected: %v", err)
	}
	candidate.HeadSHA = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err := VerifyRemotePublicationCandidate(context.Background(), stateRoot, state, candidate); err == nil || err.Error() != "remote reconcile candidate does not match the exact durable intent" {
		t.Fatalf("mismatched candidate error = %v", err)
	}

	var verified model.IssueOpsRemoteArtifactVerificationRequest
	if err := VerifyRemotePublicationLive(
		context.Background(), stateRoot, state, "https://github.com/example/agent-harness/pull/7",
		func(request model.IssueOpsRemoteArtifactVerificationRequest) error {
			verified = request
			return nil
		},
	); err != nil {
		t.Fatal(err)
	}
	wantVerification := model.IssueOpsRemoteArtifactVerificationRequest{
		Provider: "github", Kind: "pr", URL: "https://github.com/example/agent-harness/pull/7",
		Labels: providerReq.Labels, Assignees: providerReq.Assignees, TargetBranch: providerReq.BaseBranch,
	}
	if !reflect.DeepEqual(verified, wantVerification) {
		t.Fatalf("verification request = %#v, want %#v", verified, wantVerification)
	}
}

func runRemotePublicationBridgeTransition(t *testing.T, transition string) legacyRemotePublicationTransition {
	t.Helper()
	stateRoot, record, actor, providerReq := legacyRemotePublicationV1Fixture(t)
	if transition == "adopt" {
		record.IssueURL = "https://github.com/example/agent-harness/issues/195"
	}
	var err error
	record, err = writeIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	fixedNow := func() time.Time { return time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC) }
	state, err := BeginRemotePublicationIntent(
		context.Background(), stateRoot, record, actor, record.Execution.Workspace.Root,
		record.Execution.Lease.Generation, providerReq, "github", "pr",
		RemotePublicationBridgeDependencies{
			Now: fixedNow,
			NewOperationID: func() (string, error) {
				return legacyPublicationOperationID, nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	switch transition {
	case "begin":
	case "failure":
		err = RecordRemotePublicationFailure(context.Background(), stateRoot, state, remoteInvocationUnknown, "", errors.New("provider outcome is ambiguous"), fixedNow)
	case "retry":
		if err = RecordRemotePublicationFailure(context.Background(), stateRoot, state, remoteInvocationNotInvoked, "", errors.New("provider was not invoked"), fixedNow); err == nil {
			state, err = LoadRemotePublicationIntent(context.Background(), stateRoot, record.ID)
		}
		if err == nil {
			state, err = MarkRemotePublicationRetry(context.Background(), stateRoot, state)
		}
	case "adopt":
		_, err = CompleteRemotePublication(context.Background(), stateRoot, state, "https://github.com/example/agent-harness/pull/7", false, fixedNow)
	case "terminal-not-invoked":
		if err = RecordRemotePublicationFailure(context.Background(), stateRoot, state, remoteInvocationNotInvoked, "", errors.New("provider was not invoked"), fixedNow); err == nil {
			state, err = LoadRemotePublicationIntent(context.Background(), stateRoot, record.ID)
		}
		if err == nil {
			state, err = MarkRemotePublicationRetry(context.Background(), stateRoot, state)
		}
		if err == nil {
			_, err = CompleteRemotePublicationNotInvoked(context.Background(), stateRoot, state, errors.New("provider retry was not invoked"), fixedNow)
		}
	default:
		t.Fatalf("unknown bridge transition %q", transition)
	}
	if err != nil {
		t.Fatal(err)
	}

	latest, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	return legacyRemotePublicationTransition{
		recordRaw: readLegacyPublicationRawRow(t, stateRoot, issueOpsBucket, record.ID),
		intentRaw: readLegacyPublicationRawRowIfPresent(t, stateRoot, externalIntentBucket, legacyPublicationOperationID),
		record:    latest,
	}
}

func assertRemotePublicationBridgeGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("testdata", "remote_publication_v1", name))
	if err != nil {
		t.Fatal(err)
	}
	want = bytes.TrimSuffix(want, []byte("\n"))
	if !bytes.Equal(got, want) {
		t.Fatalf("%s byte drift\nwant=%x\ngot=%x", name, want, got)
	}
}
