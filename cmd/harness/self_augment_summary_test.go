package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestSummarizeSelfAugmentSuccess(t *testing.T) {
	result := SelfAugmentResult{
		Runs: []SelfAugmentIteration{
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
	summary := summarizeSelfAugment(result)
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
		Runs: []SelfAugmentIteration{
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
	summary := summarizeSelfAugment(result)
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
		Runs: []SelfAugmentIteration{
			{Iteration: 1, Seed: 10, Steps: []StepResult{{Label: "go test", OK: true}}},
			{Iteration: 2, Seed: 11, Steps: []StepResult{{Label: "go test", OK: false, Error: "boom"}}},
			{Iteration: 3, Seed: 12, Steps: []StepResult{{Label: "go test", OK: true}}},
		},
	}
	summary := summarizeSelfVerification(result, 95)
	if summary.FailureClass != "intermittent" {
		t.Fatalf("expected intermittent failure classification: %+v", summary)
	}
	if len(summary.FailureClusters) != 1 || summary.FailureClusters[0].Seeds[0] != 11 {
		t.Fatalf("unexpected failure clusters: %+v", summary.FailureClusters)
	}
}

func TestSelfVerificationCoverageReportsMissingLabels(t *testing.T) {
	coverage, gaps := selfVerificationCoverage([]string{"go test", "contract golden tests"})
	if len(coverage) == 0 || len(gaps) == 0 {
		t.Fatalf("expected coverage and gaps, got coverage=%+v gaps=%+v", coverage, gaps)
	}
	if !strings.Contains(gaps[0], "missing") {
		t.Fatalf("unexpected gap format: %+v", gaps)
	}
}

func TestSelfVerificationCoverageCompleteWhenAllLabelsPresent(t *testing.T) {
	labels := []string{}
	for _, definition := range selfVerificationCoverageDefinitions() {
		labels = append(labels, definition.Labels...)
	}
	coverage, gaps := selfVerificationCoverage(labels)
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
	contract := selfVerificationContract()
	if contract.Name != "self_verification_summary" || contract.Version < 2 || len(contract.Hash) != 64 {
		t.Fatalf("unexpected contract identity: %+v", contract)
	}
	for _, want := range []string{"goal_scores", "coverage_gaps", "slowest_steps"} {
		if !containsString(contract.RequiredFields, want) {
			t.Fatalf("contract missing required field %q: %+v", want, contract.RequiredFields)
		}
	}
	if !containsString(contract.GoalNames, "policy_security") || !containsString(contract.CoverageClaims, "secret redaction audit") {
		t.Fatalf("contract missing goals/coverage claims: %+v", contract)
	}
}

func TestForbiddenNameHitsSkipsRuntimeStateDirs(t *testing.T) {
	root := t.TempDir()
	runtimeFiles := []string{
		filepath.Join(".cache", "go-build", "log.txt"),
		filepath.Join(".claude", "hooks", ".logs", "hook-log.jsonl"),
		filepath.Join(".codex", "config.toml"),
		filepath.Join(".codegraph", "daemon.log"),
		filepath.Join(".omc", "project-memory.json"),
		filepath.Join(".omx", "state.json"),
		filepath.Join("bin", "agent-harness"),
		filepath.Join("cache", "projects.json"),
	}
	for _, rel := range runtimeFiles {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("local m"+"16kh runtime state"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sourcePath := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(sourcePath, []byte("source m"+"16kh leak"), 0o600); err != nil {
		t.Fatal(err)
	}

	hits := forbiddenNameHits(root)
	if len(hits) != 1 || hits[0] != "AGENTS.md contains m"+"16kh" {
		t.Fatalf("expected only source hit, got %+v", hits)
	}
}

func TestCachedContractGoldenStepUsesFullGoTestEvidence(t *testing.T) {
	step := cachedContractGoldenStep(StepResult{Label: "go test", Command: "go test ./... -count=1", OK: true})
	if !step.OK || step.Label != "contract golden tests" {
		t.Fatalf("unexpected cached step: %+v", step)
	}
	if step.DurationMS != 0 {
		t.Fatalf("cached step should not report subprocess duration: %+v", step)
	}
	if !strings.Contains(step.Command, "covered by go test") || !strings.Contains(step.Stdout, "full go test suite") {
		t.Fatalf("cached step did not explain evidence source: %+v", step)
	}
}

func TestSaveSelfAugmentSummary(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	result := SelfAugmentResult{
		OK:          true,
		Iterations:  10,
		BaseSeed:    300,
		ElapsedMS:   1234,
		HarnessRoot: "/tmp/harness",
		Runs: []SelfAugmentIteration{
			{
				Iteration: 1,
				Seed:      300,
				Steps: []StepResult{
					{Label: "go test", OK: true, DurationMS: 25},
				},
			},
		},
	}
	result.Summary = summarizeSelfAugment(result)
	if err := saveSelfAugmentSummary(&result, "self-verify-test"); err != nil {
		t.Fatalf("saveSelfAugmentSummary: %v", err)
	}
	if result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("missing state checkpoint: %+v", result.StateCheckpoint)
	}
	if result.StateCheckpoint.Key != "self-verify-test" || result.StateCheckpoint.Path != filepath.Join(dir, "self-verify-test.json") {
		t.Fatalf("unexpected checkpoint metadata: %+v", result.StateCheckpoint)
	}
	state, err := core.StateRead("self-verify-test")
	if err != nil {
		t.Fatalf("StateRead: %v", err)
	}
	var snapshot SelfAugmentStateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		t.Fatalf("unmarshal saved snapshot: %v", err)
	}
	if snapshot.Kind != "self_verification_summary" || !snapshot.OK || snapshot.Summary.TotalSteps != 1 || snapshot.Summary.PassedSteps != 1 {
		t.Fatalf("unexpected saved snapshot: %+v", snapshot)
	}
}
