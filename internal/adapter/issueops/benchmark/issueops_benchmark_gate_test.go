package benchmark

import (
	issueopscontract "agent-harness/internal/contract/issueops"
	"strings"
	"testing"
)

func TestRunAndCompareIssueOpsBenchmark(t *testing.T) {
	dir := t.TempDir()
	fixtures := []issueopscontract.IssueOpsBenchmarkFixture{
		{ID: "fixture", Title: "Fixture", UserPrompt: "prompt", RepoContext: "ctx", CriticalFailures: []string{"works in source repo"}},
	}

	baseline, err := RunIssueOpsBenchmark(IssueOpsBenchmarkRunRequest{
		StateRoot: dir,
		Fixtures:  fixtures,
		Artifacts: map[string]issueopscontract.IssueOpsBenchmarkArtifact{
			"fixture": {},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := RunIssueOpsBenchmark(IssueOpsBenchmarkRunRequest{
		StateRoot: dir,
		Fixtures:  fixtures,
		Artifacts: map[string]issueopscontract.IssueOpsBenchmarkArtifact{
			"fixture": completeBenchmarkArtifactForTest(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	compare := CompareIssueOpsBenchmarkRuns(baseline, candidate)
	if !compare.Improved || compare.AverageScoreDelta <= 0 {
		t.Fatalf("expected candidate improvement: %+v", compare)
	}
}

func TestEvaluateIssueOpsAutoresearchGateKeepsPassingCandidate(t *testing.T) {
	candidate := IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "A candidate with bounded files and no benchmark regression should be kept.",
		TargetDimensions: []string{"issue_quality", "plan_quality"},
		EditSurface:      []string{"skills/issueops/**", "internal/core/issueops_benchmark.go"},
		KeepCriteria:     "no regressions and no critical failures",
		DiscardCriteria:  "discard on benchmark regression or edit-surface violation",
	}
	baseline := issueOpsBenchmarkRunForGateTest("baseline", 100, 100, 0)
	next := issueOpsBenchmarkRunForGateTest("candidate", 100, 100, 0)

	result := EvaluateIssueOpsAutoresearchGate(IssueOpsAutoresearchGateRequest{
		Candidate:    candidate,
		BaselineRun:  baseline,
		CandidateRun: next,
		ChangedPaths: []string{"skills/issueops/SKILL.md", "internal/core/issueops_benchmark.go"},
	})

	if !result.OK || !result.KeepCandidate || len(result.DiscardReasons) != 0 {
		t.Fatalf("expected gate to keep candidate: %+v", result)
	}
}

func TestEvaluateIssueOpsAutoresearchGateRejectsEditSurfaceViolation(t *testing.T) {
	candidate := IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "A candidate cannot touch files outside the declared surface.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	}

	result := EvaluateIssueOpsAutoresearchGate(IssueOpsAutoresearchGateRequest{
		Candidate:    candidate,
		BaselineRun:  issueOpsBenchmarkRunForGateTest("baseline", 100, 100, 0),
		CandidateRun: issueOpsBenchmarkRunForGateTest("candidate", 100, 100, 0),
		ChangedPaths: []string{"cmd/harness/issueops.go"},
	})

	if result.KeepCandidate || len(result.EditSurfaceViolations) != 1 || !containsFold(strings.Join(result.DiscardReasons, "\n"), "edit surface") {
		t.Fatalf("expected edit surface discard: %+v", result)
	}
}

func TestEvaluateIssueOpsAutoresearchGateRejectsTargetRegression(t *testing.T) {
	candidate := IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Target dimensions cannot regress.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	}
	baseline := issueOpsBenchmarkRunWithDimensionForGateTest("baseline", "issue_quality", 100)
	next := issueOpsBenchmarkRunWithDimensionForGateTest("candidate", "issue_quality", 90)

	result := EvaluateIssueOpsAutoresearchGate(IssueOpsAutoresearchGateRequest{
		Candidate:    candidate,
		BaselineRun:  baseline,
		CandidateRun: next,
		ChangedPaths: []string{"skills/issueops/SKILL.md"},
	})

	if result.KeepCandidate || len(result.TargetDimensionRegressions) != 1 {
		t.Fatalf("expected target dimension regression discard: %+v", result)
	}
}

func TestEvaluateIssueOpsAutoresearchGateRejectsUnknownTargetDimension(t *testing.T) {
	candidate := IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Target dimensions must be known benchmark dimensions.",
		TargetDimensions: []string{"issue_qualit"},
		EditSurface:      []string{"skills/issueops/**"},
	}

	result := EvaluateIssueOpsAutoresearchGate(IssueOpsAutoresearchGateRequest{
		Candidate:    candidate,
		BaselineRun:  issueOpsBenchmarkRunForGateTest("baseline", 100, 100, 0),
		CandidateRun: issueOpsBenchmarkRunForGateTest("candidate", 100, 100, 0),
		ChangedPaths: []string{"skills/issueops/SKILL.md"},
	})

	if result.KeepCandidate || !containsFold(strings.Join(result.DiscardReasons, "\n"), "invalid target dimension") {
		t.Fatalf("expected unknown target dimension discard: %+v", result)
	}
}

func TestEvaluateIssueOpsAutoresearchGateRejectsNonPassingCandidateRun(t *testing.T) {
	candidate := IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Candidate benchmark must pass.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	}

	result := EvaluateIssueOpsAutoresearchGate(IssueOpsAutoresearchGateRequest{
		Candidate:    candidate,
		BaselineRun:  issueOpsBenchmarkRunForGateTest("baseline", 100, 100, 0),
		CandidateRun: issueOpsBenchmarkRunForGateTest("candidate", 90, 90, 1),
		ChangedPaths: []string{"skills/issueops/SKILL.md"},
	})

	if result.KeepCandidate || !containsFold(strings.Join(result.DiscardReasons, "\n"), "candidate benchmark did not pass") {
		t.Fatalf("expected non-passing benchmark discard: %+v", result)
	}
}

func issueOpsBenchmarkRunForGateTest(id string, average, minimum float64, criticalFailures int) IssueOpsBenchmarkRunResult {
	score := IssueOpsBenchmarkScore{
		OK:           criticalFailures == 0 && minimum >= 100,
		FixtureID:    "fixture",
		AverageScore: average,
		MinimumScore: minimum,
		DimensionScores: []IssueOpsDimensionScore{
			{Dimension: "issue_quality", Score: minimum, Evidence: "gate test"},
			{Dimension: "plan_quality", Score: minimum, Evidence: "gate test"},
		},
		Passed: criticalFailures == 0 && minimum >= 100,
	}
	for i := 0; i < criticalFailures; i++ {
		score.CriticalFailures = append(score.CriticalFailures, "critical failure")
	}
	return FinalizeIssueOpsBenchmarkRunResult(IssueOpsBenchmarkRunResult{ID: id, Scores: []IssueOpsBenchmarkScore{score}})
}

func issueOpsBenchmarkRunWithDimensionForGateTest(id, dimension string, scoreValue float64) IssueOpsBenchmarkRunResult {
	score := IssueOpsBenchmarkScore{
		OK:           scoreValue >= 100,
		FixtureID:    "fixture",
		AverageScore: scoreValue,
		MinimumScore: scoreValue,
		DimensionScores: []IssueOpsDimensionScore{
			{Dimension: dimension, Score: scoreValue, Evidence: "gate test"},
		},
		Passed: scoreValue >= 100,
	}
	return FinalizeIssueOpsBenchmarkRunResult(IssueOpsBenchmarkRunResult{ID: id, Scores: []IssueOpsBenchmarkScore{score}})
}
