package issueopscli

import (
	issueopscore "agent-harness/internal/adapter/issueops"
	"encoding/json"
	"testing"
)

func TestRunIssueOpsBenchmarkGateCLIKeepsCandidate(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	baseline := issueopscore.IssueOpsBenchmarkRunResult{
		ID: "baseline",
		Scores: []issueopscore.IssueOpsBenchmarkScore{{
			OK:           true,
			FixtureID:    "fixture",
			AverageScore: 100,
			MinimumScore: 100,
			DimensionScores: []issueopscore.IssueOpsDimensionScore{
				{Dimension: "issue_quality", Score: 100, Evidence: "baseline"},
			},
			Passed: true,
		}},
	}
	candidateRun := baseline
	candidateRun.ID = "candidate"
	if err := issueopscore.SaveIssueOpsBenchmarkRun(stateDir, issueopscore.FinalizeIssueOpsBenchmarkRunResult(baseline)); err != nil {
		t.Fatal(err)
	}
	if err := issueopscore.SaveIssueOpsBenchmarkRun(stateDir, issueopscore.FinalizeIssueOpsBenchmarkRunResult(candidateRun)); err != nil {
		t.Fatal(err)
	}
	candidatePath := writeIssueOpsCandidateForCLITest(t, issueopscore.IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Bounded IssueOps changes should pass the gate.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	})

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "gate", "--baseline", "baseline", "--candidate", "candidate", "--candidate-file", candidatePath, "--changed-path", "skills/issueops/SKILL.md", "--json"})
	})
	var result issueopscore.IssueOpsAutoresearchGateResult
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
	run := issueopscore.FinalizeIssueOpsBenchmarkRunResult(issueopscore.IssueOpsBenchmarkRunResult{
		ID: "baseline",
		Scores: []issueopscore.IssueOpsBenchmarkScore{{
			OK:           true,
			FixtureID:    "fixture",
			AverageScore: 100,
			MinimumScore: 100,
			DimensionScores: []issueopscore.IssueOpsDimensionScore{
				{Dimension: "issue_quality", Score: 100, Evidence: "baseline"},
			},
			Passed: true,
		}},
	})
	if err := issueopscore.SaveIssueOpsBenchmarkRun(stateDir, run); err != nil {
		t.Fatal(err)
	}
	candidatePath := writeIssueOpsCandidateForCLITest(t, issueopscore.IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Only skill changes are allowed.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	})

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "gate", "--baseline", "baseline", "--candidate", "baseline", "--candidate-file", candidatePath, "--changed-path", "cmd/harness/issueops.go", "--json"})
	})
	var result issueopscore.IssueOpsAutoresearchGateResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("gate should return JSON: %v\n%s", err, out)
	}
	if result.KeepCandidate || len(result.EditSurfaceViolations) != 1 {
		t.Fatalf("expected edit-surface discard: %+v", result)
	}
}
