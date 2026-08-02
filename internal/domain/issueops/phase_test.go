package issueops_test

import (
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
	issueopsdomain "agent-harness/internal/domain/issueops"
)

func TestPhaseDecisionsPreserveCurrentOrder(t *testing.T) {
	if !issueopsdomain.KnownIssueOpsPhase(issueopscontract.IssueOpsPhaseImplement) || issueopsdomain.IssueOpsPhaseRank(issueopscontract.IssueOpsPhaseImplement) != 5 {
		t.Fatal("implement phase identity drift")
	}
	if !issueopsdomain.IssueOpsPhaseExpectsWorktree(issueopscontract.IssueOpsPhasePR) || issueopsdomain.IssueOpsPhaseResettableOnStaleWorktree(issueopscontract.IssueOpsPhasePR) {
		t.Fatal("pr worktree decision drift")
	}
	if issueopsdomain.KnownIssueOpsPhase("unknown") || issueopsdomain.IssueOpsPhaseRank("unknown") != 0 {
		t.Fatal("unknown phase accepted")
	}
}
