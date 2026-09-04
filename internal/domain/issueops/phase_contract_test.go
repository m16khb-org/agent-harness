package issueops_test

import (
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

func TestIssueOpsPhaseOrderingAndClassification(t *testing.T) {
	if !issueopsdomain.KnownIssueOpsPhase(issueopscontract.IssueOpsPhaseProblem) || !issueopsdomain.KnownIssueOpsPhase(issueopscontract.IssueOpsPhaseDone) {
		t.Fatal("expected first and last phases to be known")
	}
	if issueopsdomain.KnownIssueOpsPhase("unknown") {
		t.Fatal("unexpected unknown phase")
	}
	if issueopsdomain.IssueOpsPhaseRank(issueopscontract.IssueOpsPhaseProblem) != 1 || issueopsdomain.IssueOpsPhaseRank(issueopscontract.IssueOpsPhaseDone) != len(issueopscontract.IssueOpsPhases) {
		t.Fatalf("unexpected phase ranks: problem=%d done=%d", issueopsdomain.IssueOpsPhaseRank(issueopscontract.IssueOpsPhaseProblem), issueopsdomain.IssueOpsPhaseRank(issueopscontract.IssueOpsPhaseDone))
	}
	if issueopsdomain.IssueOpsPhaseRank("unknown") != 0 {
		t.Fatal("unknown phase should have rank 0")
	}
	if !issueopsdomain.KnownIssueOpsPhase(issueopscontract.IssueOpsPhaseCompatibilityReview) {
		t.Fatal("compatibility-review phase should be known")
	}
	if issueopsdomain.IssueOpsPhaseRank(issueopscontract.IssueOpsPhaseCompatibilityReview) <= issueopsdomain.IssueOpsPhaseRank(issueopscontract.IssueOpsPhasePlan) ||
		issueopsdomain.IssueOpsPhaseRank(issueopscontract.IssueOpsPhaseCompatibilityReview) >= issueopsdomain.IssueOpsPhaseRank(issueopscontract.IssueOpsPhaseImplement) {
		t.Fatalf("compatibility-review should sit between plan and implement, got rank %d", issueopsdomain.IssueOpsPhaseRank(issueopscontract.IssueOpsPhaseCompatibilityReview))
	}
	for _, phase := range []issueopscontract.IssueOpsPhase{issueopscontract.IssueOpsPhaseImplement, issueopscontract.IssueOpsPhaseAISlopClean, issueopscontract.IssueOpsPhaseFeedback} {
		if !issueopsdomain.IssueOpsPhaseResettableOnStaleWorktree(phase) {
			t.Fatalf("%s should be resettable on stale worktree", phase)
		}
	}
	if issueopsdomain.IssueOpsPhaseResettableOnStaleWorktree(issueopscontract.IssueOpsPhasePR) {
		t.Fatal("pr phase should not be resettable on stale worktree")
	}
}
