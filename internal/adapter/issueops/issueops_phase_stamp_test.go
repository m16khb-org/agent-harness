package issueops

import (
	"testing"

	"agent-harness/internal/contract/issueops"
)

// A forward transition stamps the ledger: the phase being left is recorded
// complete (completed_at + artifacts), the phase being entered gets entered_at.
// Real (observed) timestamps, unlike derived entries.
func TestAdvanceStampsPhaseLedgerOnTransition(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	rec, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "1-stamp"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	id := rec.ID
	recordIssueOpsIntentForTest(t, stateRoot, id)

	grilled, err := AdvanceIssueOpsPhase(stateRoot, id, string(IssueOpsPhaseGrill))
	if err != nil {
		t.Fatalf("advance to grill: %v", err)
	}
	problemEntry, ok := grilled.PhaseLedger[IssueOpsPhaseProblem]
	if !ok || problemEntry.CompletedAt == "" {
		t.Fatalf("leaving problem should stamp its completion: %#v", grilled.PhaseLedger)
	}
	if len(problemEntry.Artifacts) == 0 {
		t.Fatalf("completed problem entry should record artifacts: %#v", problemEntry)
	}
	grillEntry, ok := grilled.PhaseLedger[IssueOpsPhaseGrill]
	if !ok || grillEntry.EnteredAt == "" {
		t.Fatalf("entering grill should stamp entered_at: %#v", grilled.PhaseLedger)
	}
	if grillEntry.CompletedAt != "" {
		t.Fatalf("just-entered grill should not be complete yet: %#v", grillEntry)
	}
}
