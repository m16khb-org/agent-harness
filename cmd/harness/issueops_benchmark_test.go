package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/core"
)

func TestRunIssueOpsBenchmarkCLI(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "run", "--fixtures", filepath.Join("..", "..", "testdata", "issueops", "fixtures"), "--judge", "none", "--json"})
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("benchmark run should return JSON: %v\n%s", err, out)
	}
	if result["ok"] != true || result["fixture_count"].(float64) < 1 {
		t.Fatalf("unexpected benchmark result: %#v", result)
	}
}

func TestRunIssueOpsBenchmarkCLIAgyKeepsOneScorePerFixture(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fakeAgy := filepath.Join(t.TempDir(), "fake-agy.sh")
	if err := os.WriteFile(fakeAgy, []byte(`#!/bin/sh
cat <<'EOF'
{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"judge ok"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}
EOF
`), 0o755); err != nil {
		t.Fatal(err)
	}

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "run", "--fixtures", filepath.Join("..", "..", "testdata", "issueops", "fixtures"), "--judge", "agy", "--agy-command", fakeAgy, "--json"})
	})
	var result struct {
		OK           bool             `json:"ok"`
		FixtureCount int              `json:"fixture_count"`
		Scores       []map[string]any `json:"scores"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("benchmark run should return JSON: %v\n%s", err, out)
	}
	if !result.OK || len(result.Scores) != result.FixtureCount {
		t.Fatalf("expected one merged score per fixture: %+v", result)
	}
}

func TestRunIssueOpsBenchmarkGateCLIKeepsCandidate(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	baseline := core.IssueOpsBenchmarkRunResult{
		ID: "baseline",
		Scores: []core.IssueOpsBenchmarkScore{{
			OK:           true,
			FixtureID:    "fixture",
			AverageScore: 100,
			MinimumScore: 100,
			DimensionScores: []core.IssueOpsDimensionScore{
				{Dimension: "issue_quality", Score: 100, Evidence: "baseline"},
			},
			Passed: true,
		}},
	}
	candidateRun := baseline
	candidateRun.ID = "candidate"
	if err := core.SaveIssueOpsBenchmarkRun(stateDir, core.FinalizeIssueOpsBenchmarkRunResult(baseline)); err != nil {
		t.Fatal(err)
	}
	if err := core.SaveIssueOpsBenchmarkRun(stateDir, core.FinalizeIssueOpsBenchmarkRunResult(candidateRun)); err != nil {
		t.Fatal(err)
	}
	candidatePath := writeIssueOpsCandidateForCLITest(t, core.IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Bounded IssueOps changes should pass the gate.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	})

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "gate", "--baseline", "baseline", "--candidate", "candidate", "--candidate-file", candidatePath, "--changed-path", "skills/issueops/SKILL.md", "--json"})
	})
	var result core.IssueOpsAutoresearchGateResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("gate should return JSON: %v\n%s", err, out)
	}
	if !result.OK || !result.KeepCandidate {
		t.Fatalf("expected gate to keep candidate: %+v", result)
	}
}

func TestRunIssueOpsBenchmarkGateCLIDiscardsOutsideEditSurface(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	run := core.FinalizeIssueOpsBenchmarkRunResult(core.IssueOpsBenchmarkRunResult{
		ID: "baseline",
		Scores: []core.IssueOpsBenchmarkScore{{
			OK:           true,
			FixtureID:    "fixture",
			AverageScore: 100,
			MinimumScore: 100,
			DimensionScores: []core.IssueOpsDimensionScore{
				{Dimension: "issue_quality", Score: 100, Evidence: "baseline"},
			},
			Passed: true,
		}},
	})
	if err := core.SaveIssueOpsBenchmarkRun(stateDir, run); err != nil {
		t.Fatal(err)
	}
	candidatePath := writeIssueOpsCandidateForCLITest(t, core.IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Only skill changes are allowed.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	})

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "gate", "--baseline", "baseline", "--candidate", "baseline", "--candidate-file", candidatePath, "--changed-path", "cmd/harness/issueops.go", "--json"})
	})
	var result core.IssueOpsAutoresearchGateResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("gate should return JSON: %v\n%s", err, out)
	}
	if result.KeepCandidate || len(result.EditSurfaceViolations) != 1 {
		t.Fatalf("expected edit-surface discard: %+v", result)
	}
}

func writeIssueOpsCandidateForCLITest(t *testing.T, candidate core.IssueOpsAutoresearchCandidate) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "candidate.json")
	b, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
