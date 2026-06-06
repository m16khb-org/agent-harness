package lifecycle

import "testing"

func recordIssueOpsLifecycleIntentForTest(t *testing.T, id string) {
	t.Helper()
	if _, err := RecordIssueOpsIntent(IssueOpsStateRoot(), id, IssueOpsIntentRecordRequest{
		RawRequest:        "refactor issueops flow",
		InterpretedIntent: "keep intent and design evidence before implementation",
		SuccessCriteria:   []string{"intent is recorded", "design is reviewed"},
	}); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsLifecycleDesignForTest(t *testing.T, id string) {
	t.Helper()
	if _, err := RecordIssueOpsDesignReview(IssueOpsStateRoot(), id, IssueOpsDesignReviewRequest{
		ProblemSummary: "IssueOps must preserve the work contract",
		ProposedDesign: "Gate implementation on a reviewed design contract",
		Verification:   []string{"go test ./internal/core/lifecycle"},
		Approved:       true,
	}); err != nil {
		t.Fatal(err)
	}
}
