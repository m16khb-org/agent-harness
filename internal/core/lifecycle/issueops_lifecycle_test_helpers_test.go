package lifecycle

import (
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func recordIssueOpsLifecycleIntentForTest(t *testing.T, id string) {
	t.Helper()
	if _, err := RecordIssueOpsIntent(IssueOpsStateRoot(), id, issueopscontract.IssueOpsIntentRecordRequest{
		RawRequest:        "refactor issueops flow",
		InterpretedIntent: "keep intent and design evidence before implementation",
		SuccessCriteria:   []string{"intent is recorded", "design is reviewed"},
	}); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsLifecycleDesignForTest(t *testing.T, id string) {
	t.Helper()
	if _, err := RecordIssueOpsDesignReview(IssueOpsStateRoot(), id, issueopscontract.IssueOpsDesignReviewRequest{
		ProblemSummary: "IssueOps must preserve the work contract",
		ProposedDesign: "Gate implementation on a reviewed design contract",
		RefactorPlan:   "Keep lifecycle guard behavior aligned with IssueOps cycle state",
		Alternatives:   []string{"allow source edits without linked-cycle evidence"},
		Risks:          []string{"worktree guard fixtures must model approved design evidence"},
		Verification:   []string{"design review checked worktree guard risks", "go test ./internal/core/lifecycle"},
		Approved:       true,
	}); err != nil {
		t.Fatal(err)
	}
}
