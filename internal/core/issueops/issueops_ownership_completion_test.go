package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/preflight"
)

func TestOwnerCompletionWaitsForHumanCleanupDecision(t *testing.T) {
	stateRoot, record, owner := ownershipActiveRecorderRecord(t)
	finalHead := strings.TrimSpace(preflight.GitOut(record.ExecutionHandoff.WorkerRoot, "rev-parse", "refs/heads/"+record.Branch))
	record.Phase = model.IssueOpsPhasePR
	record.ExecutionHandoff.PublishReceipt = &model.IssueOpsExecutionHandoffPublishReceipt{FinalHead: finalHead}
	record.RemoteArtifact = &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/17", TargetBranch: record.BranchPrepare.BaseBranch, VerifiedAt: "2026-07-20T00:00:00Z"}
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}

	completed, err := CompleteIssueOpsHandoff(stateRoot, IssueOpsHandoffCompleteRequest{
		ID: record.ID, Attempt: record.ExecutionHandoff.Attempt, OwnershipEpoch: record.ExecutionHandoff.OwnershipEpoch, ContextSHA256: record.ExecutionHandoff.ContextSHA256,
		Host: owner.Host, SessionID: owner.SessionID, AgentID: owner.AgentID, CWD: owner.CWD, FinalHead: finalHead, TuringReportPath: "plans/test.md", Verification: []string{"go test ./..."},
	})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Phase != model.IssueOpsPhaseDone || completed.ExecutionHandoff.State != handoff.StateCleanupPendingHumanDecision || completed.ExecutionHandoff.Completion == nil || completed.ExecutionHandoff.Cleanup != nil || completed.ExecutionHandoff.WorkerDoneProjection == nil {
		t.Fatalf("owner completion must await human cleanup decision: %#v", completed.ExecutionHandoff)
	}
}

func TestNonClosedCompletionRejectsForceRelease(t *testing.T) {
	stateRoot, record, owner := ownershipActiveRecorderRecord(t)
	finalHead := strings.TrimSpace(preflight.GitOut(record.ExecutionHandoff.WorkerRoot, "rev-parse", "refs/heads/"+record.Branch))
	record.Phase = model.IssueOpsPhasePR
	record.ExecutionHandoff.PublishReceipt = &model.IssueOpsExecutionHandoffPublishReceipt{FinalHead: finalHead}
	record.RemoteArtifact = &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/17", TargetBranch: record.BranchPrepare.BaseBranch, VerifiedAt: "2026-07-20T00:00:00Z"}
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	if _, err := CompleteIssueOpsHandoff(stateRoot, IssueOpsHandoffCompleteRequest{ID: record.ID, Attempt: record.ExecutionHandoff.Attempt, OwnershipEpoch: record.ExecutionHandoff.OwnershipEpoch, ContextSHA256: record.ExecutionHandoff.ContextSHA256, Host: owner.Host, SessionID: owner.SessionID, AgentID: owner.AgentID, CWD: owner.CWD, FinalHead: finalHead, TuringReportPath: "plans/test.md", Verification: []string{"go test ./..."}}); err != nil {
		t.Fatal(err)
	}
	if _, err := ForceReleaseIssueOps(stateRoot, record.ID, "manual release attempt"); err == nil || !strings.Contains(err.Error(), "ownership-transfer") {
		t.Fatalf("force release must preserve human cleanup authority: %v", err)
	}
}

func TestSourceCannotCompleteOwnerHandoff(t *testing.T) {
	stateRoot, record, owner := ownershipActiveRecorderRecord(t)
	finalHead := strings.TrimSpace(preflight.GitOut(record.ExecutionHandoff.WorkerRoot, "rev-parse", "refs/heads/"+record.Branch))
	record.Phase = model.IssueOpsPhasePR
	record.ExecutionHandoff.PublishReceipt = &model.IssueOpsExecutionHandoffPublishReceipt{FinalHead: finalHead}
	record.RemoteArtifact = &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/17", TargetBranch: record.BranchPrepare.BaseBranch, VerifiedAt: "2026-07-20T00:00:00Z"}
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	if _, err := CompleteIssueOpsHandoff(stateRoot, IssueOpsHandoffCompleteRequest{
		ID: record.ID, Attempt: record.ExecutionHandoff.Attempt, OwnershipEpoch: record.ExecutionHandoff.OwnershipEpoch, ContextSHA256: record.ExecutionHandoff.ContextSHA256,
		Host: owner.Host, SessionID: "source-session", AgentID: owner.AgentID, CWD: record.Repo, FinalHead: finalHead, TuringReportPath: "plans/test.md", Verification: []string{"go test ./..."},
	}); err == nil || !strings.Contains(err.Error(), "ownership transfer") {
		t.Fatalf("source completion error = %v", err)
	}
}
