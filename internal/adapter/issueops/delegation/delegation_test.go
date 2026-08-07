package delegation

import (
	"testing"

	model "agent-harness/internal/contract/issueops"
)

func TestMissingPreconditionsAcceptsReviewedParent(t *testing.T) {
	parent := model.IssueOpsRecord{
		Phase:                model.IssueOpsPhaseImplement,
		Branch:               "123-parent",
		DesignReview:         &model.IssueOpsDesignReview{Approved: true},
		CompatibilityReview:  &model.IssueOpsCompatibilityReview{Approved: true},
		DevilsAdvocateReview: &model.IssueOpsDevilsAdvocateReview{Verdict: "pass", RecordedAt: "2026-07-07T00:00:00Z"},
	}
	if missing := MissingPreconditions(parent, model.IssueOpsChildStartRequest{Branch: "123-child"}); len(missing) != 0 {
		t.Fatalf("reviewed parent should satisfy delegation gate, got %#v", missing)
	}
}
