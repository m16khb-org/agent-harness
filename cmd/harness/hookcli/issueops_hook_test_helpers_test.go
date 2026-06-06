package hookcli

import (
	"testing"

	"agent-harness/internal/core"
)

func recordIssueOpsHookIntentForTest(t *testing.T, id string) {
	t.Helper()
	if _, err := core.RecordIssueOpsIntent(core.IssueOpsStateRoot(), id, core.IssueOpsIntentRecordRequest{
		RawRequest:        "refactor issueops flow",
		InterpretedIntent: "keep intent and design evidence before implementation",
		SuccessCriteria:   []string{"intent is recorded", "design is reviewed"},
	}); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsHookDesignForTest(t *testing.T, id string) {
	t.Helper()
	if _, err := core.RecordIssueOpsDesignReview(core.IssueOpsStateRoot(), id, core.IssueOpsDesignReviewRequest{
		ProblemSummary: "IssueOps must preserve the work contract",
		ProposedDesign: "Gate implementation on a reviewed design contract",
		Verification:   []string{"go test ./cmd/harness/hookcli"},
		Approved:       true,
	}); err != nil {
		t.Fatal(err)
	}
}
