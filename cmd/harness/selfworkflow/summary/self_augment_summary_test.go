package summary

import (
	"strings"
	"testing"

	"agent-harness/cmd/harness/selfworkflow/model"
)

func TestSummarizeSelfAugmentSuccess(t *testing.T) {
	result := SelfAugmentResult{
		Runs: []model.SelfAugmentIteration{
			{
				Iteration: 1,
				Seed:      100,
				Steps: []StepResult{
					{Label: "fast", OK: true, DurationMS: 10},
					{Label: "slow", OK: true, DurationMS: 50},
				},
			},
			{
				Iteration: 2,
				Seed:      101,
				Steps: []StepResult{
					{Label: "fast", OK: true, DurationMS: 15},
					{Label: "slow", OK: true, DurationMS: 40},
				},
			},
		},
	}
	summary := SummarizeSelfAugment(result)
	if summary.TotalRuns != 2 || summary.TotalSteps != 4 || summary.PassedSteps != 4 || summary.FailedSteps != 0 {
		t.Fatalf("unexpected summary counts: %+v", summary)
	}
	if len(summary.StepLabels) != 2 || summary.StepLabels[0] != "fast" || summary.StepLabels[1] != "slow" {
		t.Fatalf("unexpected step labels: %+v", summary.StepLabels)
	}
	if len(summary.SlowestSteps) != 4 || summary.SlowestSteps[0].Label != "slow" || summary.SlowestSteps[0].DurationMS != 50 {
		t.Fatalf("unexpected slowest steps: %+v", summary.SlowestSteps)
	}
}

func TestSummarizeSelfAugmentFailure(t *testing.T) {
	result := SelfAugmentResult{
		Runs: []model.SelfAugmentIteration{
			{
				Iteration: 3,
				Seed:      202,
				Steps: []StepResult{
					{Label: "go test", OK: true, DurationMS: 100},
					{Label: "MCP smoke", OK: false, DurationMS: 25, Error: "boom"},
				},
			},
		},
	}
	summary := SummarizeSelfAugment(result)
	if summary.TotalRuns != 1 || summary.TotalSteps != 2 || summary.PassedSteps != 1 || summary.FailedSteps != 1 {
		t.Fatalf("unexpected summary counts: %+v", summary)
	}
	if summary.FailedIteration != 3 || summary.FailedSeed != 202 || summary.FailedStep != "MCP smoke" {
		t.Fatalf("unexpected failure pointer: %+v", summary)
	}
	if len(summary.RerunCommands) == 0 || !strings.Contains(summary.RerunCommands[len(summary.RerunCommands)-1], "--progress=jsonl") {
		t.Fatalf("expected rerun commands with progress heartbeat: %+v", summary.RerunCommands)
	}
	if summary.FailureClass != "single_failure_observation" || len(summary.FailureClusters) != 1 {
		t.Fatalf("expected single failure classification: %+v", summary)
	}
}

func TestSummarizeSelfVerificationClassifiesIntermittentFailure(t *testing.T) {
	result := SelfAugmentResult{
		Iterations: 3,
		BaseSeed:   10,
		Runs: []model.SelfAugmentIteration{
			{Iteration: 1, Seed: 10, Steps: []StepResult{{Label: "go test", OK: true}}},
			{Iteration: 2, Seed: 11, Steps: []StepResult{{Label: "go test", OK: false, Error: "boom"}}},
			{Iteration: 3, Seed: 12, Steps: []StepResult{{Label: "go test", OK: true}}},
		},
	}
	summary := SummarizeSelfVerification(result, 95)
	if summary.FailureClass != "intermittent" {
		t.Fatalf("expected intermittent failure classification: %+v", summary)
	}
	if len(summary.FailureClusters) != 1 || summary.FailureClusters[0].Seeds[0] != 11 {
		t.Fatalf("unexpected failure clusters: %+v", summary.FailureClusters)
	}
}

func TestSelfVerificationCoverageReportsMissingLabels(t *testing.T) {
	coverage, gaps := SelfVerificationCoverageForLabels([]string{"go test", "contract golden tests"})
	if len(coverage) == 0 || len(gaps) == 0 {
		t.Fatalf("expected coverage and gaps, got coverage=%+v gaps=%+v", coverage, gaps)
	}
	if !strings.Contains(gaps[0], "missing") {
		t.Fatalf("unexpected gap format: %+v", gaps)
	}
}

func TestSelfVerificationCoverageCompleteWhenAllLabelsPresent(t *testing.T) {
	labels := []string{}
	for _, definition := range SelfVerificationCoverageDefinitions() {
		labels = append(labels, definition.Labels...)
	}
	coverage, gaps := SelfVerificationCoverageForLabels(labels)
	if len(gaps) != 0 {
		t.Fatalf("expected no coverage gaps, got %+v", gaps)
	}
	for _, item := range coverage {
		if !item.Covered || len(item.MissingLabels) != 0 {
			t.Fatalf("unexpected uncovered item: %+v", item)
		}
	}
}

func TestSelfVerificationContractIncludesSummaryExtensions(t *testing.T) {
	contract := SelfVerificationContractValue()
	if contract.Name != "self_verification_summary" || contract.Version != 4 || len(contract.Hash) != 64 {
		t.Fatalf("unexpected contract identity: %+v", contract)
	}
	for _, want := range []string{"goal_scores", "coverage_gaps", "slowest_steps", "failure_cause", "failure_cause_reason", "failure_cause_evidence"} {
		if !containsString(contract.RequiredFields, want) {
			t.Fatalf("contract missing required field %q: %+v", want, contract.RequiredFields)
		}
	}
	if !containsString(contract.GoalNames, "policy_security") || !containsString(contract.CoverageClaims, "secret redaction audit") || !containsString(contract.CoverageClaims, "tool-call schema conformance") {
		t.Fatalf("contract missing goals/coverage claims: %+v", contract)
	}
}

func TestClassifySelfVerificationFailureCoversDeterministicMixedAndUnknown(t *testing.T) {
	deterministic := SelfAugmentResult{
		Runs: []model.SelfAugmentIteration{
			{Iteration: 1, Seed: 10, Steps: []StepResult{{Label: "go test", OK: false}}},
			{Iteration: 2, Seed: 11, Steps: []StepResult{{Label: "go test", OK: false}}},
		},
	}
	class, reason, clusters := ClassifySelfVerificationFailure(deterministic, SelfAugmentSummary{TotalRuns: 2, FailedSteps: 2})
	if class != "deterministic" || !strings.Contains(reason, "same step") || len(clusters) != 1 || clusters[0].Count != 2 {
		t.Fatalf("unexpected deterministic classification: class=%q reason=%q clusters=%+v", class, reason, clusters)
	}

	mixed := SelfAugmentResult{
		Runs: []model.SelfAugmentIteration{
			{Iteration: 1, Seed: 20, Steps: []StepResult{{Label: "go test", OK: false}}},
			{Iteration: 2, Seed: 21, Steps: []StepResult{{Label: "go build", OK: false}}},
		},
	}
	class, reason, clusters = ClassifySelfVerificationFailure(mixed, SelfAugmentSummary{TotalRuns: 2, FailedSteps: 2})
	if class != "mixed" || !strings.Contains(reason, "multiple failure steps") || len(clusters) != 2 {
		t.Fatalf("unexpected mixed classification: class=%q reason=%q clusters=%+v", class, reason, clusters)
	}

	class, reason, clusters = ClassifySelfVerificationFailure(SelfAugmentResult{}, SelfAugmentSummary{FailedSteps: 1})
	if class != "unknown" || !strings.Contains(reason, "no failed step details") || clusters != nil {
		t.Fatalf("unexpected unknown classification: class=%q reason=%q clusters=%+v", class, reason, clusters)
	}
}

func TestSummarizeSelfVerificationMarksGoalFailureWhenLabelsMissing(t *testing.T) {
	result := SelfAugmentResult{
		OK:         true,
		Iterations: 1,
		Runs: []model.SelfAugmentIteration{
			{Iteration: 1, Seed: 100, Steps: []StepResult{{Label: "go test", OK: true}}},
		},
	}
	summary := SummarizeSelfVerification(result, 95)
	if summary.TerminationEligible {
		t.Fatalf("missing labels must prevent termination: %+v", summary)
	}
	if summary.MinimumGoalScore >= 100 {
		t.Fatalf("missing labels should lower minimum goal score: %+v", summary.GoalScores)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
