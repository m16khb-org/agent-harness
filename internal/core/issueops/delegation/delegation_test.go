package delegation

import (
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestMissingPreconditionsReportsSubagentPlanGate(t *testing.T) {
	parent := model.IssueOpsRecord{
		Phase:                model.IssueOpsPhaseImplement,
		Branch:               "123-parent",
		DesignReview:         &model.IssueOpsDesignReview{Approved: true},
		CompatibilityReview:  &model.IssueOpsCompatibilityReview{Approved: true},
		DevilsAdvocateReview: &model.IssueOpsDevilsAdvocateReview{Verdict: "pass", RecordedAt: "2026-07-07T00:00:00Z"},
		ExecutionDecision:    &model.IssueOpsExecutionDecision{SubagentUse: "planned"},
	}
	missing := MissingPreconditions(parent, model.IssueOpsChildStartRequest{Branch: "123-child"})
	if len(missing) != 1 || missing[0] != "execution_decision_subagent_plan" {
		t.Fatalf("expected only subagent plan gate, got %#v", missing)
	}
	parent.ExecutionDecision.SubagentPlans = []model.IssueOpsSubAgentPlan{{Pattern: "isolated-worktree-work"}}
	if missing := MissingPreconditions(parent, model.IssueOpsChildStartRequest{Branch: "123-child"}); len(missing) != 0 {
		t.Fatalf("allowed subagent plan should satisfy helper gate, got %#v", missing)
	}
}
