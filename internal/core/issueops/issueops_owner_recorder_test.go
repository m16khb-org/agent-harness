package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
)

func TestProtocolV2OwnerRequiredForPostTransferRecorders(t *testing.T) {
	stateRoot, record, owner := ownershipActiveRecorderRecord(t)

	if _, err := AdvanceIssueOpsPhaseWithActor(stateRoot, record.ID, string(record.Phase), owner); err != nil {
		t.Fatalf("exact owner phase recorder: %v", err)
	}
	if _, err := RecordIssueOpsAISlopCleanEvidenceWithActor(stateRoot, record.ID, []string{"minimal-diff"}, []string{"go test ./..."}, owner); err != nil {
		t.Fatalf("exact owner evidence recorder: %v", err)
	}
	if _, err := AddIssueOpsFeedbackWithActor(stateRoot, record.ID, "review", "update contract", "contract_change", owner); err != nil {
		t.Fatalf("exact owner feedback add: %v", err)
	}
	if _, err := ResolveIssueOpsFeedbackWithActor(stateRoot, record.ID, 0, "valid-defect", owner); err != nil {
		t.Fatalf("exact owner feedback resolve: %v", err)
	}
	if _, err := MarkIssueOpsContractFeedbackIssueUpdatedWithActor(stateRoot, record.ID, owner); err != nil {
		t.Fatalf("exact owner issue-update marker: %v", err)
	}

	for _, actor := range []IssueOpsActor{
		{Host: owner.Host, SessionID: "source-session", AgentID: owner.AgentID, CWD: record.Repo},
		{Host: owner.Host, SessionID: "other-session", AgentID: owner.AgentID, CWD: owner.CWD},
		{},
	} {
		if _, err := AdvanceIssueOpsPhaseWithActor(stateRoot, record.ID, string(record.Phase), actor); err == nil || !strings.Contains(err.Error(), "ownership transfer") {
			t.Fatalf("non-owner phase recorder must fail closed: actor=%#v err=%v", actor, err)
		}
	}
}

func ownershipActiveRecorderRecord(t *testing.T) (string, IssueOpsRecord, IssueOpsActor) {
	t.Helper()
	stateRoot, record, claim := ownershipOrientingRecord(t)
	packet, err := handoff.BuildContext(record, handoff.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	active, err := AcknowledgeIssueOpsHandoffContext(stateRoot, IssueOpsHandoffAcknowledgeRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256,
		Host: claim.Host, SessionID: claim.SessionID, AgentID: claim.AgentID, CWD: claim.CWD,
		IssueURL: record.IssueURL, PlanSHA256: packet.PlanSHA256, Understanding: "sealed cycle only", ScopeConfirmation: "worker root only",
	})
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, active, IssueOpsActor{Host: claim.Host, SessionID: claim.SessionID, AgentID: claim.AgentID, CWD: claim.CWD}
}
