package issueops

// Frozen legacy oracle from exact base
// 667e5d15b0773e2550cfbf5bc2780506e9eb2896.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

const legacyPublicationOperationID = "0123456789abcdef0123456789abcdef"

func TestBeginRemotePullRequestIntentWithOperationIDRejectsNonCanonicalID(t *testing.T) {
	tests := []struct {
		name        string
		operationID string
	}{
		{name: "too short", operationID: "0123456789abcdef0123456789abcde"},
		{name: "uppercase", operationID: "0123456789abcdef0123456789abcdeF"},
		{name: "non hexadecimal", operationID: "0123456789abcdef0123456789abcdeg"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateRoot, record, actor, providerReq := legacyRemotePublicationV1Fixture(t)
			_, _, err := beginRemotePullRequestIntentWithOperationID(
				stateRoot,
				record,
				actor,
				record.Execution.Workspace.Root,
				record.Execution.Lease.Generation,
				providerReq,
				"github",
				"pr",
				test.operationID,
				func() time.Time { return time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC) },
			)
			if err == nil || !strings.Contains(err.Error(), "32 lowercase hexadecimal") {
				t.Fatalf("non-canonical operation ID error = %v", err)
			}
		})
	}
}

func TestLegacyRemotePublicationV1Goldens(t *testing.T) {
	stateRoot, record, actor, providerReq := legacyRemotePublicationV1Fixture(t)
	fixedNow := func() time.Time {
		return time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	}

	var intentRaw []byte
	_, err := legacyCreateRemotePullRequestWithOperationID(
		stateRoot,
		record,
		actor,
		record.Execution.Workspace.Root,
		record.Execution.Lease.Generation,
		providerReq,
		"github",
		"pr",
		legacyPublicationOperationID,
		RemotePullRequestDependencies{
			Create: func(string, port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
				intentRaw = readLegacyPublicationRawRow(t, stateRoot, externalIntentBucket, legacyPublicationOperationID)
				return port.IssueProviderCreatePullRequestResult{}, errors.New("provider outcome is ambiguous")
			},
			Now: fixedNow,
		},
	)
	if err == nil {
		t.Fatal("legacy create returned nil for an ambiguous provider outcome")
	}
	failureRecordRaw := readLegacyPublicationRawRow(t, stateRoot, issueOpsBucket, record.ID)

	assertLegacyPublicationGolden(t, "intent.golden.json", intentRaw)
	assertLegacyPublicationGolden(t, "failure_record.golden.json", failureRecordRaw)
}

// legacyCreateRemotePullRequestWithOperationID is the exact-base create sequence
// after public request preparation. It remains test-only and never calls the new
// publication vertical.
func legacyCreateRemotePullRequestWithOperationID(
	stateRoot string,
	record IssueOpsRecord,
	actor model.NativeActor,
	cwd string,
	expectedGeneration uint64,
	providerReq port.IssueProviderCreatePullRequestRequest,
	provider string,
	kind string,
	operationID string,
	deps RemotePullRequestDependencies,
) (port.IssueProviderCreatePullRequestResult, error) {
	pending, payload, err := beginRemotePullRequestIntentWithOperationID(
		stateRoot,
		record,
		actor,
		cwd,
		expectedGeneration,
		providerReq,
		provider,
		kind,
		operationID,
		deps.Now,
	)
	if err != nil {
		return port.IssueProviderCreatePullRequestResult{}, err
	}
	result, callErr := deps.Create(payload.Provider, payload.Request)
	if callErr != nil {
		invocation := remoteInvocationUnknown
		var typed *port.IssueProviderCreateError
		if errors.As(callErr, &typed) && !typed.Invoked {
			invocation = remoteInvocationNotInvoked
		}
		_ = recordRemotePullRequestFailure(stateRoot, pending.ID, payload.OperationID, invocation, payload.RetryCount, result.URL, callErr, deps.Now)
		return result, fmt.Errorf("remote create outcome requires execution reconcile; creation was not retried: %w", callErr)
	}
	if strings.TrimSpace(result.URL) == "" {
		err = fmt.Errorf("provider create returned no canonical URL")
		_ = recordRemotePullRequestFailure(stateRoot, pending.ID, payload.OperationID, remoteInvocationUnknown, payload.RetryCount, "", err, deps.Now)
		return result, err
	}
	if err := verifyRemotePullRequestResult(record, payload, result.URL, deps.Verify); err != nil {
		_ = recordRemotePullRequestFailure(stateRoot, pending.ID, payload.OperationID, remoteInvocationUnknown, payload.RetryCount, result.URL, err, deps.Now)
		return result, fmt.Errorf("provider returned a URL but durable verification requires execution reconcile: %w", err)
	}
	if _, err := finishRemotePullRequestIntent(stateRoot, pending.ID, payload, result.URL, true, deps.Now); err != nil {
		_ = recordRemotePullRequestFailure(stateRoot, pending.ID, payload.OperationID, remoteInvocationUnknown, payload.RetryCount, result.URL, err, deps.Now)
		return result, fmt.Errorf("provider succeeded but durable receipt requires execution reconcile: %w", err)
	}
	return result, nil
}

func legacyRemotePublicationV1Fixture(t *testing.T) (string, IssueOpsRecord, model.NativeActor, port.IssueProviderCreatePullRequestRequest) {
	t.Helper()
	stateRoot := t.TempDir()
	process := model.NativeProcessReceipt{
		PID:        4242,
		StartedAt:  "2026-08-01T00:00:00Z",
		Executable: "/usr/local/bin/codex",
	}
	actor := model.NativeActor{
		Host:            "codex",
		SessionID:       "publication-oracle-session",
		AgentID:         "publication-oracle-agent",
		SessionProcess:  &process,
		ProcessAncestry: []model.NativeProcessReceipt{process},
	}
	record := IssueOpsRecord{
		SchemaVersion: model.IssueOpsSchemaVersion,
		ID:            "io-publication-oracle",
		Repo:          "/fixture/agent-harness",
		Branch:        "195-publication-oracle",
		Phase:         model.IssueOpsPhasePR,
		WorktreePath:  "/fixture/agent-harness.worktrees/195-publication-oracle",
		Execution: &model.Execution{
			Mode: model.ExecutionModeDirect,
			Workspace: model.Workspace{
				SourceRoot: "/fixture/agent-harness",
				Root:       "/fixture/agent-harness.worktrees/195-publication-oracle",
				Branch:     "195-publication-oracle",
				BaseHead:   "667e5d15b0773e2550cfbf5bc2780506e9eb2896",
				Driver:     "git",
				LinkedAt:   "2026-08-01T00:00:00Z",
			},
			Lease: model.WriteLease{
				Generation: 1,
				Status:     model.LeaseStatusActive,
				Holder:     &actor,
				ClaimedAt:  "2026-08-01T00:00:00Z",
			},
		},
		CreatedAt: "2026-08-01T00:00:00Z",
		UpdatedAt: "2026-08-01T00:00:00Z",
	}
	persisted, err := writeIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, persisted, actor, port.IssueProviderCreatePullRequestRequest{
		Repo:            "/fixture/agent-harness.worktrees/195-publication-oracle",
		ProjectKey:      "github.com/example/agent-harness",
		Title:           "Freeze publication schema v1 bytes",
		Body:            "Deterministic publication oracle.",
		HeadBranch:      "195-publication-oracle",
		BaseBranch:      "117-hexagonal-architecture-migration",
		Labels:          []string{"enhancement"},
		Assignees:       []string{"maintainer"},
		Draft:           true,
		ExpectedHeadSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Confirm:         true,
		Host:            actor.Host,
		SessionID:       actor.SessionID,
		AgentID:         actor.AgentID,
		CWD:             record.Execution.Workspace.Root,
	}
}

func readLegacyPublicationRawRow(t *testing.T, stateRoot, bucket, id string) []byte {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok, err := db.Get(bucket, id)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("raw row %s/%s is missing", bucket, id)
	}
	return append([]byte(nil), raw...)
}

func assertLegacyPublicationGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("testdata", "remote_publication_v1", name))
	if err != nil {
		t.Fatal(err)
	}
	want = bytes.TrimSuffix(want, []byte("\n"))
	if !bytes.Equal(got, want) {
		t.Errorf("%s byte drift\nwant=%x\ngot=%x", name, want, got)
	}
}

type legacyRemotePublicationTransition struct {
	recordRaw []byte
	intentRaw []byte
	record    IssueOpsRecord
}

func runLegacyRemotePublicationTransition(t *testing.T, transition string) legacyRemotePublicationTransition {
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
	pending, payload, err := beginRemotePullRequestIntentWithOperationID(
		stateRoot, record, actor, record.Execution.Workspace.Root, record.Execution.Lease.Generation,
		providerReq, "github", "pr", legacyPublicationOperationID, fixedNow,
	)
	if err != nil {
		t.Fatal(err)
	}

	switch transition {
	case "begin":
	case "failure":
		err = recordRemotePullRequestFailure(stateRoot, pending.ID, payload.OperationID, remoteInvocationUnknown, payload.RetryCount, "", errors.New("provider outcome is ambiguous"), fixedNow)
	case "retry":
		if err = recordRemotePullRequestFailure(stateRoot, pending.ID, payload.OperationID, remoteInvocationNotInvoked, payload.RetryCount, "", errors.New("provider was not invoked"), fixedNow); err == nil {
			payload, err = readExternalRemotePRPayload(stateRoot, payload.OperationID)
		}
		if err == nil {
			_, err = markRemotePullRequestRetry(stateRoot, pending.ID, payload)
		}
	case "adopt":
		_, err = finishRemotePullRequestIntent(stateRoot, pending.ID, payload, "https://github.com/example/agent-harness/pull/7", false, fixedNow)
	case "terminal-not-invoked":
		if err = recordRemotePullRequestFailure(stateRoot, pending.ID, payload.OperationID, remoteInvocationNotInvoked, payload.RetryCount, "", errors.New("provider was not invoked"), fixedNow); err == nil {
			payload, err = readExternalRemotePRPayload(stateRoot, payload.OperationID)
		}
		if err == nil {
			payload, err = markRemotePullRequestRetry(stateRoot, pending.ID, payload)
		}
		if err == nil {
			_, err = finishRemotePullRequestPreInvocationFailure(stateRoot, pending.ID, payload, errors.New("provider retry was not invoked"), fixedNow)
		}
	default:
		t.Fatalf("unknown legacy transition %q", transition)
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

func readLegacyPublicationRawRowIfPresent(t *testing.T, stateRoot, bucket, id string) []byte {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok, err := db.Get(bucket, id)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		return nil
	}
	return append([]byte(nil), raw...)
}
