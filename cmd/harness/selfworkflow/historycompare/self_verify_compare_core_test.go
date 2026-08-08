package historycompare

import (
	"testing"

	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/internal/contract/failurecause"
)

func TestCompareSelfAugmentSummariesFromSnapshotsCoversWarningsAndGoalRegressions(t *testing.T) {
	baseline := SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          model.SelfVerificationSummaryKind,
		OK:            true,
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary: SelfAugmentSummary{
			TotalSteps:          1,
			PassedSteps:         1,
			StepLabels:          []string{"go test"},
			TerminationEligible: true,
			MinimumGoalScore:    95,
		},
	}
	candidate := baseline
	candidate.OK = true
	candidate.ElapsedMS = 100
	candidate.GeneratedAt = "2000-01-01T00:01:00Z"
	candidate.Summary.TotalSteps = 2
	candidate.Summary.StepLabels = []string{"go test", "MCP smoke"}
	candidate.Summary.TerminationEligible = false
	candidate.Summary.MinimumGoalScore = 90

	result := CompareSelfAugmentSummariesFromSnapshots("baseline", "candidate", 5, baseline, candidate)

	if !result.OK || !result.Regressed || result.ElapsedDeltaMS != 100 {
		t.Fatalf("unexpected compare result: %+v", result)
	}
	for _, want := range []string{"candidate_not_termination_eligible", "minimum_goal_score_decreased_by_5.00"} {
		if !containsString(result.Regressions, want) {
			t.Fatalf("missing regression %q in %+v", want, result.Regressions)
		}
	}
	for _, want := range []string{"baseline_elapsed_zero", "added_step_label:MCP smoke", "total_steps_delta_+1"} {
		if !containsString(result.Warnings, want) {
			t.Fatalf("missing warning %q in %+v", want, result.Warnings)
		}
	}
}
func TestCompareSelfAugmentSummariesFromSnapshotsWarnsWhenFailureCauseChanges(t *testing.T) {
	baseline := SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          model.SelfVerificationSummaryKind,
		Summary: SelfAugmentSummary{
			TotalSteps:   1,
			FailedSteps:  1,
			FailureCause: failurecause.Model,
			FailureCauseEvidence: []failurecause.Evidence{
				{Cause: failurecause.Model, Code: "invalid_tool_arguments", Source: "tool_conformance"},
			},
			TerminationEligible: false,
		},
	}
	candidate := baseline
	candidate.Summary.FailureCause = failurecause.Transport
	candidate.Summary.FailureCauseEvidence = []failurecause.Evidence{
		{Cause: failurecause.Transport, Code: "mcp_framing", Source: "tool_conformance"},
	}

	result := CompareSelfAugmentSummariesFromSnapshots("baseline", "candidate", 5, baseline, candidate)

	if result.Regressed || len(result.Regressions) != 0 {
		t.Fatalf("failure cause warning must not regress comparison: %+v", result)
	}
	if !containsString(result.Warnings, "failure_cause_changed:model->transport") {
		t.Fatalf("missing failure cause change warning in %+v", result.Warnings)
	}
}

func TestCompareSelfAugmentSummariesFromSnapshotsSkipsCauseWarningWhenOnlyOneSummaryFailed(t *testing.T) {
	baseline := SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          model.SelfVerificationSummaryKind,
		Summary: SelfAugmentSummary{
			TotalSteps:   1,
			FailedSteps:  1,
			FailureCause: failurecause.Model,
		},
	}
	candidate := baseline
	candidate.Summary.FailedSteps = 0
	candidate.Summary.FailureCause = failurecause.None

	result := CompareSelfAugmentSummariesFromSnapshots("baseline", "candidate", 5, baseline, candidate)

	if containsString(result.Warnings, "failure_cause_changed:model->none") {
		t.Fatalf("failure cause warning requires failures in both summaries: %+v", result.Warnings)
	}
}
