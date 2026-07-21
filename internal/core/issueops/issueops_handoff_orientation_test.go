package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
)

func TestOwnershipClaimEntersOrientationWithoutWriteLease(t *testing.T) {
	_, record, _ := ownershipOrientingRecord(t)
	if record.ExecutionHandoff.State != handoff.StateOwnerOrienting || record.ExecutionHandoff.OwnerSession == nil || record.Phase == IssueOpsPhaseImplement {
		t.Fatalf("ownership claim must enter orientation without implementation lease: %#v", record)
	}
}

func TestOwnershipOrientationRequiresExactIssuePlanAndContext(t *testing.T) {
	stateRoot, record, claim := ownershipOrientingRecord(t)
	packet, err := handoff.BuildContext(record, handoff.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	req := IssueOpsHandoffAcknowledgeRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256,
		Host: claim.Host, SessionID: claim.SessionID, AgentID: claim.AgentID, CWD: claim.CWD,
		IssueURL: record.IssueURL, PlanSHA256: packet.PlanSHA256, Understanding: "I will implement only the sealed cycle.", ScopeConfirmation: "worker root only",
	}
	wrongPlan := req
	wrongPlan.PlanSHA256 = strings.Repeat("0", 64)
	if _, err := AcknowledgeIssueOpsHandoffContext(stateRoot, wrongPlan); err == nil {
		t.Fatal("wrong plan SHA must be rejected")
	}
	wrongContext := req
	wrongContext.ContextSHA256 = strings.Repeat("0", 64)
	if _, err := AcknowledgeIssueOpsHandoffContext(stateRoot, wrongContext); err == nil {
		t.Fatal("wrong context fence must be rejected")
	}
	if _, err := AcknowledgeIssueOpsHandoffContext(stateRoot, req); err != nil {
		t.Fatal(err)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateOwnerActive || persisted.ExecutionHandoff.Orientation == nil {
		t.Fatalf("ownership acknowledgement did not grant active owner: %#v", persisted.ExecutionHandoff)
	}
	if repeated, err := AcknowledgeIssueOpsHandoffContext(stateRoot, req); err != nil || repeated.ExecutionHandoff.Orientation.RecordedAt != persisted.ExecutionHandoff.Orientation.RecordedAt {
		t.Fatalf("identical acknowledgement must be idempotent: record=%#v err=%v", repeated.ExecutionHandoff, err)
	}

	req.Understanding = "conflicting acknowledgement"
	if _, err := AcknowledgeIssueOpsHandoffContext(stateRoot, req); err == nil {
		t.Fatal("conflicting acknowledgement must fail")
	}
}

func ownershipOrientingRecord(t *testing.T) (string, IssueOpsRecord, IssueOpsHandoffClaimRequest) {
	t.Helper()
	stateRoot, dispatched, actor := ownershipActiveRecorderRecord(t)
	dispatched.ExecutionHandoff.State = handoff.StateOwnershipDispatched
	dispatched.ExecutionHandoff.OwnerSession = nil
	dispatched.ExecutionHandoff.Orientation = nil
	if _, err := WriteIssueOps(stateRoot, dispatched); err != nil {
		t.Fatal(err)
	}
	claim := IssueOpsHandoffClaimRequest{
		ID: dispatched.ID, Attempt: dispatched.ExecutionHandoff.Attempt, OwnershipEpoch: dispatched.ExecutionHandoff.OwnershipEpoch,
		ContextSHA256: dispatched.ExecutionHandoff.ContextSHA256, Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID,
		CWD: dispatched.WorktreePath, OrcaWorktreeID: dispatched.ExecutionHandoff.Orca.WorktreeID,
	}
	claimed, err := ClaimIssueOpsHandoff(stateRoot, claim)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, claimed, claim
}
