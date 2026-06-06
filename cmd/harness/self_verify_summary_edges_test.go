package main

import (
	"strings"
	"testing"
)

func TestSelfVerifyStepRerunCommandCoversOperationalLabels(t *testing.T) {
	tests := map[string]string{
		"go build":              "go build -o bin/agent-harness ./cmd/harness",
		"command audit smoke":   "policy audit",
		"candidate export":      "self-verify candidates",
		"step budget baseline":  "self-verify compare",
		"daemon resilience":     "daemon start",
		"redaction audit":       "go test ./cmd/harness -run Test -count=1",
		"unknown helper branch": "",
	}
	for label, want := range tests {
		t.Run(label, func(t *testing.T) {
			got, ok := selfVerifyStepRerunCommand(label)
			if want == "" {
				if ok || got != "" {
					t.Fatalf("expected no rerun command for %q, got %q ok=%v", label, got, ok)
				}
				return
			}
			if !ok || !strings.Contains(got, want) {
				t.Fatalf("selfVerifyStepRerunCommand(%q)=%q ok=%v, want substring %q", label, got, ok, want)
			}
		})
	}
}

func TestClassifySelfVerificationFailureCoversDeterministicMixedAndUnknown(t *testing.T) {
	deterministic := SelfAugmentResult{
		Runs: []SelfAugmentIteration{
			{Iteration: 1, Seed: 10, Steps: []StepResult{{Label: "go test", OK: false}}},
			{Iteration: 2, Seed: 11, Steps: []StepResult{{Label: "go test", OK: false}}},
		},
	}
	class, reason, clusters := classifySelfVerificationFailure(deterministic, SelfAugmentSummary{TotalRuns: 2, FailedSteps: 2})
	if class != "deterministic" || !strings.Contains(reason, "same step") || len(clusters) != 1 || clusters[0].Count != 2 {
		t.Fatalf("unexpected deterministic classification: class=%q reason=%q clusters=%+v", class, reason, clusters)
	}

	mixed := SelfAugmentResult{
		Runs: []SelfAugmentIteration{
			{Iteration: 1, Seed: 20, Steps: []StepResult{{Label: "go test", OK: false}}},
			{Iteration: 2, Seed: 21, Steps: []StepResult{{Label: "go build", OK: false}}},
		},
	}
	class, reason, clusters = classifySelfVerificationFailure(mixed, SelfAugmentSummary{TotalRuns: 2, FailedSteps: 2})
	if class != "mixed" || !strings.Contains(reason, "multiple failure steps") || len(clusters) != 2 {
		t.Fatalf("unexpected mixed classification: class=%q reason=%q clusters=%+v", class, reason, clusters)
	}

	class, reason, clusters = classifySelfVerificationFailure(SelfAugmentResult{}, SelfAugmentSummary{FailedSteps: 1})
	if class != "unknown" || !strings.Contains(reason, "no failed step details") || clusters != nil {
		t.Fatalf("unexpected unknown classification: class=%q reason=%q clusters=%+v", class, reason, clusters)
	}
}

func TestSummarizeSelfVerificationMarksGoalFailureWhenLabelsMissing(t *testing.T) {
	result := SelfAugmentResult{
		OK:         true,
		Iterations: 1,
		Runs: []SelfAugmentIteration{
			{Iteration: 1, Seed: 100, Steps: []StepResult{{Label: "go test", OK: true}}},
		},
	}
	summary := summarizeSelfVerification(result, 95)
	if summary.TerminationEligible {
		t.Fatalf("missing labels must prevent termination: %+v", summary)
	}
	if summary.MinimumGoalScore >= 100 {
		t.Fatalf("missing labels should lower minimum goal score: %+v", summary.GoalScores)
	}
}
