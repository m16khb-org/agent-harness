package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/preflight"
)

func completedOwnershipCleanupRecord(t *testing.T) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot, record, owner := ownershipActiveRecorderRecord(t)
	finalHead := strings.TrimSpace(preflight.GitOut(record.ExecutionHandoff.WorkerRoot, "rev-parse", "refs/heads/"+record.Branch))
	record.Phase = model.IssueOpsPhasePR
	record.ExecutionHandoff.PublishReceipt = &model.IssueOpsExecutionHandoffPublishReceipt{FinalHead: finalHead}
	record.RemoteArtifact = &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/17", TargetBranch: record.BranchPrepare.BaseBranch, VerifiedAt: "2026-07-20T00:00:00Z"}
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	completed, err := CompleteIssueOpsOwnershipTransfer(stateRoot, IssueOpsHandoffFinishRequest{ID: record.ID, Attempt: record.ExecutionHandoff.Attempt, OwnershipEpoch: record.ExecutionHandoff.OwnershipEpoch, ContextSHA256: record.ExecutionHandoff.ContextSHA256, Host: owner.Host, SessionID: owner.SessionID, AgentID: owner.AgentID, CWD: owner.CWD, FinalHead: finalHead, TuringReportPath: "plans/test.md", Verification: []string{"go test ./..."}})
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, completed
}

func TestOwnershipCleanupPreviewIsReadOnlyAndFreshSourceMayApprove(t *testing.T) {
	stateRoot, record := completedOwnershipCleanupRecord(t)
	request := IssueOpsOwnershipCleanupPreviewRequest{ID: record.ID, Host: "codex", Session: "fresh-source", AgentID: "source-agent", CWD: record.Repo}
	preview, err := PreviewIssueOpsOwnershipCleanup(stateRoot, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Choices) != 3 {
		t.Fatalf("cleanup choices = %#v", preview.Choices)
	}
	unchanged, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil || unchanged.ExecutionHandoff.State != handoff.StateCleanupPendingHumanDecision || unchanged.ExecutionHandoff.Cleanup != nil {
		t.Fatalf("preview mutated cleanup authority: %#v err=%v", unchanged.ExecutionHandoff, err)
	}
	approved, err := ApproveIssueOpsOwnershipCleanup(stateRoot, IssueOpsOwnershipCleanupApproveRequest{IssueOpsOwnershipCleanupPreviewRequest: request, InventoryFingerprint: preview.InventoryFingerprint, Disposition: "close-owner", Reason: "human requested retained workspace", Confirm: true})
	if err != nil {
		t.Fatal(err)
	}
	if approved.ExecutionHandoff.State != handoff.StateCleanupExecuting || approved.ExecutionHandoff.Cleanup == nil || approved.ExecutionHandoff.Cleanup.ApprovedBySession == nil || approved.ExecutionHandoff.Cleanup.ApprovedBySession.SessionID != "fresh-source" {
		t.Fatalf("human approval did not atomically select source cleanup session: %#v", approved.ExecutionHandoff)
	}
}

func TestOwnershipCleanupRejectsCompletedOwnerAndStalePreview(t *testing.T) {
	stateRoot, record := completedOwnershipCleanupRecord(t)
	owner := record.ExecutionHandoff.OwnerSession
	if _, err := PreviewIssueOpsOwnershipCleanup(stateRoot, IssueOpsOwnershipCleanupPreviewRequest{ID: record.ID, Host: owner.Host, Session: owner.SessionID, AgentID: owner.AgentID, CWD: record.Repo}); err == nil || !strings.Contains(err.Error(), "completed owner") {
		t.Fatalf("completed owner preview error = %v", err)
	}
	request := IssueOpsOwnershipCleanupPreviewRequest{ID: record.ID, Host: "codex", Session: "fresh-source", AgentID: "source-agent", CWD: record.Repo}
	preview, err := PreviewIssueOpsOwnershipCleanup(stateRoot, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApproveIssueOpsOwnershipCleanup(stateRoot, IssueOpsOwnershipCleanupApproveRequest{IssueOpsOwnershipCleanupPreviewRequest: request, InventoryFingerprint: strings.Repeat("0", 64), Disposition: "remove-local", Reason: "human requested local removal", Confirm: true}); err == nil || !strings.Contains(err.Error(), "preview again") {
		t.Fatalf("stale preview approval error = %v", err)
	}
	if preview.InventoryFingerprint == "" {
		t.Fatal("preview fingerprint is empty")
	}
}

func TestOwnershipCleanupRecordsOnlyApprovedOrderedReceipts(t *testing.T) {
	stateRoot, record := completedOwnershipCleanupRecord(t)
	candidate := IssueOpsOwnershipCleanupPreviewRequest{ID: record.ID, Host: "codex", Session: "fresh-source", AgentID: "source-agent", CWD: record.Repo}
	preview, err := PreviewIssueOpsOwnershipCleanup(stateRoot, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApproveIssueOpsOwnershipCleanup(stateRoot, IssueOpsOwnershipCleanupApproveRequest{IssueOpsOwnershipCleanupPreviewRequest: candidate, InventoryFingerprint: preview.InventoryFingerprint, Disposition: "remove-local", Reason: "human requested local resource removal", Confirm: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordIssueOpsOwnershipCleanup(t.Context(), stateRoot, IssueOpsOwnershipCleanupRecordRequest{IssueOpsOwnershipCleanupPreviewRequest: candidate, Step: "task_terminal"}, nil); err == nil || !strings.Contains(err.Error(), "out of order") {
		t.Fatalf("unordered receipt error = %v", err)
	}
	updated, err := RecordIssueOpsOwnershipCleanup(t.Context(), stateRoot, IssueOpsOwnershipCleanupRecordRequest{IssueOpsOwnershipCleanupPreviewRequest: candidate, Step: "remote_head_safe"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ExecutionHandoff.State != handoff.StateCleanupExecuting || len(updated.ExecutionHandoff.Cleanup.Receipts) != 1 || updated.ExecutionHandoff.Cleanup.Receipts[0].Step != "remote_head_safe" {
		t.Fatalf("first human-approved cleanup receipt = %#v", updated.ExecutionHandoff)
	}
	other := candidate
	other.Session = "other-source"
	if _, err := RecordIssueOpsOwnershipCleanup(t.Context(), stateRoot, IssueOpsOwnershipCleanupRecordRequest{IssueOpsOwnershipCleanupPreviewRequest: other, Step: "task_terminal"}, nil); err == nil || !strings.Contains(err.Error(), "human-approved") {
		t.Fatalf("unapproved source receipt error = %v", err)
	}
}
