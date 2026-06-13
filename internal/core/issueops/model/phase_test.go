package model

import "testing"

func TestIssueOpsPhaseOrderingAndClassification(t *testing.T) {
	if !KnownIssueOpsPhase(IssueOpsPhaseProblem) || !KnownIssueOpsPhase(IssueOpsPhaseDone) {
		t.Fatal("expected first and last phases to be known")
	}
	if KnownIssueOpsPhase("unknown") {
		t.Fatal("unexpected unknown phase")
	}
	if IssueOpsPhaseRank(IssueOpsPhaseProblem) != 1 || IssueOpsPhaseRank(IssueOpsPhaseDone) != len(IssueOpsPhases) {
		t.Fatalf("unexpected phase ranks: problem=%d done=%d", IssueOpsPhaseRank(IssueOpsPhaseProblem), IssueOpsPhaseRank(IssueOpsPhaseDone))
	}
	if IssueOpsPhaseRank("unknown") != 0 {
		t.Fatal("unknown phase should have rank 0")
	}
	for _, phase := range []IssueOpsPhase{IssueOpsPhaseImplement, IssueOpsPhaseAISlopClean, IssueOpsPhaseFeedback, IssueOpsPhasePR} {
		if !IssueOpsPhaseExpectsWorktree(phase) {
			t.Fatalf("%s should expect a worktree", phase)
		}
	}
	if IssueOpsPhaseExpectsWorktree(IssueOpsPhasePlan) {
		t.Fatal("plan phase should not expect a worktree")
	}
	for _, phase := range []IssueOpsPhase{IssueOpsPhaseImplement, IssueOpsPhaseAISlopClean, IssueOpsPhaseFeedback} {
		if !IssueOpsPhaseResettableOnStaleWorktree(phase) {
			t.Fatalf("%s should be resettable on stale worktree", phase)
		}
	}
	if IssueOpsPhaseResettableOnStaleWorktree(IssueOpsPhasePR) {
		t.Fatal("pr phase should not be resettable on stale worktree")
	}
}
