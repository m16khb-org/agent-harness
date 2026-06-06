package main

import "testing"

func TestPlannedSelfVerifyStepsPreservesExecutionOrder(t *testing.T) {
	var goTestStep StepResult

	steps := plannedSelfVerifySteps("/repo", "/tmp/agent-harness", 100, &goTestStep)

	got := make([]string, 0, len(steps))
	for _, step := range steps {
		got = append(got, step.Label)
	}
	want := []string{
		"harness invariants",
		"go test",
		"contract golden tests",
		"risk QA tier",
		"go build",
		"inspect smoke",
		"docs index smoke",
		"candidate export",
		"step budget baseline",
		"install dry-run smoke",
		"command policy smoke",
		"command audit smoke",
		"contract check",
		"worker lifecycle smoke",
		"MCP smoke",
		"state roundtrip",
		"parallel isolation",
		"daemon resilience",
		"preflight fuzz",
		"native integration",
		"redaction audit",
		"QA gate",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d steps, got %d: %v", len(want), len(got), got)
	}
	for i, label := range want {
		if got[i] != label {
			t.Fatalf("step %d label = %q, want %q; all labels: %v", i, got[i], label, got)
		}
	}
}

func TestPlannedSelfVerifyStepsUsesCachedContractGoldenAfterGoTest(t *testing.T) {
	goTestStep := StepResult{Label: "go test", OK: true}

	steps := plannedSelfVerifySteps("/repo", "/tmp/agent-harness", 100, &goTestStep)
	got := steps[2].Run()

	if !got.OK || got.Label != "contract golden tests" || got.Command != "covered by go test ./... -count=1" {
		t.Fatalf("expected cached contract golden result, got %#v", got)
	}
}
