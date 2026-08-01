package issueops

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	completionoutbound "agent-harness/internal/adapter/outbound/issueopscompletion"
	completionapp "agent-harness/internal/application/issueopscompletion"
	completioncontract "agent-harness/internal/contract/issueopscompletion"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/core/sqlstore"
)

func TestExecutionCompletionVerticalDifferentialSuccessAndRetry(t *testing.T) {
	legacyRoot := t.TempDir()
	verticalRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, legacyRoot, "198-completion-differential")
	prepareExecutionCompletionFixture(t, legacyRoot, &fixture)
	actor := executionActor("codex", "completion-differential")
	if _, err := claimViaVertical(legacyRoot, ExecutionClaimRequest{ID: fixture.record.ID, Generation: 1, Actor: actor, CWD: fixture.worktree, TokenFile: fixture.tokenPath}); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(fixture.worktree, ".agent-harness", "turing", "issue198-report.json")
	if err := os.MkdirAll(filepath.Dir(report), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, []byte(`{"status":"pass"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	copyCompletionState(t, legacyRoot, verticalRoot, fixture.record.ID, actor)

	request := ExecutionCompleteRequest{
		ID: fixture.record.ID, Generation: 1, Actor: actor, CWD: fixture.worktree,
		FinalHead: preflight.GitOut(fixture.worktree, "rev-parse", "HEAD"), TuringReportPath: report,
		Verification:      []string{"go test ./... -count=1", "go test -race ./... -count=1"},
		RemoteArtifactURL: "https://github.com/example/agent-harness/pull/69", Confirm: true,
	}
	fixed := time.Date(2026, 8, 2, 2, 3, 4, 5, time.UTC)
	legacyClock := &sequenceCompletionClock{values: []time.Time{fixed, fixed.Add(time.Nanosecond)}}
	legacy, err := completeExecutionWithClock(legacyRoot, request, ExecutionCompleteDeps{}, legacyClock.Now)
	if err != nil {
		t.Fatal(err)
	}
	verticalDB, err := sqlstore.Open(verticalRoot)
	if err != nil {
		t.Fatal(err)
	}
	verticalClock := &sequenceCompletionClock{values: []time.Time{fixed, fixed.Add(time.Nanosecond)}}
	service := completionapp.NewService(completionoutbound.NewRepository(verticalDB), completionoutbound.NewEnvironment(differentialArtifactVerifier), verticalClock, differentialProcessInspector, nil)
	vertical, err := service.Complete(context.Background(), differentialCompletionRequest(request))
	if err != nil {
		t.Fatal(err)
	}

	assertCompletionDifferential(t, legacyRoot, verticalRoot, fixture.record.ID, actor, legacy, vertical)
	if legacyClock.calls != 2 || verticalClock.calls != 2 {
		t.Fatalf("clock calls differ: legacy=%d vertical=%d", legacyClock.calls, verticalClock.calls)
	}
	legacyBeforeRetry, _, _ := releaseDifferentialSnapshot(t, legacyRoot, fixture.record.ID, leaseHolderIndexKey(actor))
	verticalBeforeRetry, _, _ := releaseDifferentialSnapshot(t, verticalRoot, fixture.record.ID, leaseHolderIndexKey(actor))
	if _, err := completeExecutionWithClock(legacyRoot, request, ExecutionCompleteDeps{}, func() time.Time { return fixed.Add(time.Hour) }); err != nil {
		t.Fatalf("legacy retry: %v", err)
	}
	if _, err := service.Complete(context.Background(), differentialCompletionRequest(request)); err != nil {
		t.Fatalf("vertical retry: %v", err)
	}
	legacyAfterRetry, _, _ := releaseDifferentialSnapshot(t, legacyRoot, fixture.record.ID, leaseHolderIndexKey(actor))
	verticalAfterRetry, _, _ := releaseDifferentialSnapshot(t, verticalRoot, fixture.record.ID, leaseHolderIndexKey(actor))
	if !bytes.Equal(legacyBeforeRetry, legacyAfterRetry) || !bytes.Equal(verticalBeforeRetry, verticalAfterRetry) {
		t.Fatal("identical retry changed persisted record bytes")
	}
}

func TestExecutionCompletionVerticalDifferentialDenialMatrix(t *testing.T) {
	tests := []struct {
		name          string
		mutateRecord  func(*IssueOpsRecord)
		mutateRequest func(*ExecutionCompleteRequest, *testing.T)
	}{
		{name: "confirmation", mutateRequest: func(request *ExecutionCompleteRequest, _ *testing.T) { request.Confirm = false }},
		{name: "verification", mutateRequest: func(request *ExecutionCompleteRequest, _ *testing.T) { request.Verification = nil }},
		{name: "remote url", mutateRequest: func(request *ExecutionCompleteRequest, _ *testing.T) { request.RemoteArtifactURL = "http://invalid" }},
		{name: "phase", mutateRecord: func(record *IssueOpsRecord) { record.Phase = IssueOpsPhaseFeedback }},
		{name: "artifact", mutateRecord: func(record *IssueOpsRecord) { record.RemoteArtifact = nil }},
		{name: "artifact canonicality", mutateRecord: func(record *IssueOpsRecord) { record.RemoteArtifact.Provider = " GitHub " }},
		{name: "target branch", mutateRecord: func(record *IssueOpsRecord) { record.RemoteArtifact.TargetBranch = "release" }},
		{name: "holder", mutateRequest: func(request *ExecutionCompleteRequest, _ *testing.T) { request.Actor.SessionID = "foreign-session" }},
		{name: "generation", mutateRequest: func(request *ExecutionCompleteRequest, _ *testing.T) { request.Generation = 2 }},
		{name: "cwd", mutateRequest: func(request *ExecutionCompleteRequest, t *testing.T) { request.CWD = t.TempDir() }},
		{name: "head", mutateRequest: func(request *ExecutionCompleteRequest, _ *testing.T) { request.FinalHead = strings.Repeat("0", 40) }},
		{name: "report", mutateRequest: func(request *ExecutionCompleteRequest, t *testing.T) {
			request.TuringReportPath = filepath.Join(t.TempDir(), "missing.json")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			legacyRoot, verticalRoot := t.TempDir(), t.TempDir()
			fixture := newClaimableExecutionFixture(t, legacyRoot, "198-completion-denial-"+strings.ReplaceAll(test.name, " ", "-"))
			prepareExecutionCompletionFixture(t, legacyRoot, &fixture)
			actor := executionActor("codex", "completion-denial-"+test.name)
			if _, err := claimViaVertical(legacyRoot, ExecutionClaimRequest{ID: fixture.record.ID, Generation: 1, Actor: actor, CWD: fixture.worktree, TokenFile: fixture.tokenPath}); err != nil {
				t.Fatal(err)
			}
			if test.mutateRecord != nil {
				record, err := ReadIssueOps(legacyRoot, fixture.record.ID)
				if err != nil {
					t.Fatal(err)
				}
				test.mutateRecord(&record)
				if _, err := writeIssueOps(legacyRoot, record); err != nil {
					t.Fatal(err)
				}
			}
			report := filepath.Join(fixture.worktree, "completion-denial-report.json")
			if err := os.WriteFile(report, []byte(`{"status":"pass"}`), 0o644); err != nil {
				t.Fatal(err)
			}
			request := ExecutionCompleteRequest{ID: fixture.record.ID, Generation: 1, Actor: actor, CWD: fixture.worktree, FinalHead: preflight.GitOut(fixture.worktree, "rev-parse", "HEAD"), TuringReportPath: report, Verification: []string{"go test ./..."}, RemoteArtifactURL: "https://github.com/example/agent-harness/pull/69", Confirm: true}
			if test.mutateRequest != nil {
				test.mutateRequest(&request, t)
			}
			copyCompletionState(t, legacyRoot, verticalRoot, fixture.record.ID, actor)
			before, beforeIndex, beforeIndexExists := releaseDifferentialSnapshot(t, legacyRoot, fixture.record.ID, leaseHolderIndexKey(actor))
			fixed := time.Date(2026, 8, 2, 3, 4, 5, 6, time.UTC)
			_, legacyErr := completeExecutionWithClock(legacyRoot, request, ExecutionCompleteDeps{}, func() time.Time { return fixed })
			verticalDB, err := sqlstore.Open(verticalRoot)
			if err != nil {
				t.Fatal(err)
			}
			service := completionapp.NewService(completionoutbound.NewRepository(verticalDB), completionoutbound.NewEnvironment(differentialArtifactVerifier), differentialCompletionClock{fixed}, differentialProcessInspector, nil)
			_, verticalErr := service.Complete(context.Background(), differentialCompletionRequest(request))
			if legacyErr == nil || verticalErr == nil || legacyErr.Error() != verticalErr.Error() {
				t.Fatalf("errors differ: legacy=%v vertical=%v", legacyErr, verticalErr)
			}
			legacyAfter, legacyIndex, legacyIndexExists := releaseDifferentialSnapshot(t, legacyRoot, fixture.record.ID, leaseHolderIndexKey(actor))
			verticalAfter, verticalIndex, verticalIndexExists := releaseDifferentialSnapshot(t, verticalRoot, fixture.record.ID, leaseHolderIndexKey(actor))
			if !bytes.Equal(before, legacyAfter) || !bytes.Equal(before, verticalAfter) || !bytes.Equal(beforeIndex, legacyIndex) || !bytes.Equal(beforeIndex, verticalIndex) || beforeIndexExists != legacyIndexExists || beforeIndexExists != verticalIndexExists {
				t.Fatal("denied completion changed record or holder index")
			}
		})
	}
}

func copyCompletionState(t *testing.T, sourceRoot, targetRoot, id string, actor NativeActor) {
	t.Helper()
	record, index, indexExists := releaseDifferentialSnapshot(t, sourceRoot, id, leaseHolderIndexKey(actor))
	target, err := sqlstore.Open(targetRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := target.Put(issueOpsBucket, id, record); err != nil {
		t.Fatal(err)
	}
	if indexExists {
		if err := target.Put(leaseHolderBucket, leaseHolderIndexKey(actor), index); err != nil {
			t.Fatal(err)
		}
	}
}

func differentialCompletionRequest(request ExecutionCompleteRequest) completionapp.Request {
	actor := completioncontract.Actor{Host: request.Actor.Host, SessionID: request.Actor.SessionID, AgentID: request.Actor.AgentID}
	if request.Actor.SessionProcess != nil {
		actor.Process = &completioncontract.ProcessReceipt{PID: request.Actor.SessionProcess.PID, StartedAt: request.Actor.SessionProcess.StartedAt, Executable: request.Actor.SessionProcess.Executable}
	}
	ancestry := make([]completioncontract.ProcessReceipt, 0, len(request.Actor.ProcessAncestry))
	for _, receipt := range request.Actor.ProcessAncestry {
		ancestry = append(ancestry, completioncontract.ProcessReceipt{PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable})
	}
	return completionapp.Request{ID: request.ID, Generation: request.Generation, Actor: actor, Ancestry: ancestry, CWD: request.CWD, FinalHead: request.FinalHead, TuringReportPath: request.TuringReportPath, Verification: append([]string(nil), request.Verification...), RemoteArtifactURL: request.RemoteArtifactURL, Confirm: request.Confirm}
}

func differentialArtifactVerifier(record completioncontract.RecordSnapshot, requestedURL string) error {
	coreRecord := IssueOpsRecord{
		Phase: IssueOpsPhase(record.Phase), IssueURL: record.IssueURL,
		BranchPrepare: &IssueOpsBranchPrepare{BaseBranch: record.BaseBranch},
	}
	if record.Artifact != nil {
		coreRecord.RemoteArtifact = &IssueOpsRemoteArtifactVerification{
			Provider: record.Artifact.Provider, Kind: record.Artifact.Kind, URL: record.Artifact.URL,
			Labels: append([]string(nil), record.Artifact.Labels...), Assignees: append([]string(nil), record.Artifact.Assignees...),
			VerifiedAt: record.Artifact.VerifiedAt, TargetBranch: record.Artifact.TargetBranch,
		}
	}
	return ValidateExecutionCompletionArtifact(coreRecord, requestedURL)
}

func differentialProcessInspector(_ context.Context, receipt completioncontract.ProcessReceipt) (string, completioncontract.ProcessReceipt, error) {
	status, observed, err := InspectNativeProcessReceipt(NativeProcessReceipt{PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable})
	return status, completioncontract.ProcessReceipt{PID: observed.PID, StartedAt: observed.StartedAt, Executable: observed.Executable}, err
}

func assertCompletionDifferential(t *testing.T, legacyRoot, verticalRoot, id string, actor NativeActor, legacy ExecutionResult, vertical completionapp.Result) {
	t.Helper()
	legacyRecord, _, legacyIndex := releaseDifferentialSnapshot(t, legacyRoot, id, leaseHolderIndexKey(actor))
	verticalRecord, _, verticalIndex := releaseDifferentialSnapshot(t, verticalRoot, id, leaseHolderIndexKey(actor))
	if !bytes.Equal(legacyRecord, verticalRecord) {
		t.Fatalf("persisted bytes differ\nlegacy=%s\nvertical=%s", legacyRecord, verticalRecord)
	}
	if legacyIndex || verticalIndex {
		t.Fatalf("holder index remains: legacy=%t vertical=%t", legacyIndex, verticalIndex)
	}
	legacyExecution, err := json.Marshal(legacy.Execution)
	if err != nil {
		t.Fatal(err)
	}
	verticalExecution, err := json.Marshal(vertical.Execution)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(legacyExecution, verticalExecution) || legacy.OK != vertical.OK || legacy.ID != vertical.ID {
		t.Fatalf("result differs\nlegacy=%s\nvertical=%s", legacyExecution, verticalExecution)
	}
}

type differentialCompletionClock struct{ at time.Time }

func (c differentialCompletionClock) Now() time.Time { return c.at }

type sequenceCompletionClock struct {
	values []time.Time
	calls  int
}

func (c *sequenceCompletionClock) Now() time.Time {
	value := c.values[c.calls]
	c.calls++
	return value
}
