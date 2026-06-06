package core

import (
	"strings"
	"testing"
)

func TestIssueOpsBenchmarkRunStorageAndJudgeMergeBranches(t *testing.T) {
	stateRoot := t.TempDir()
	result := FinalizeIssueOpsBenchmarkRunResult(IssueOpsBenchmarkRunResult{
		ID: "run-one",
		Scores: []IssueOpsBenchmarkScore{{
			OK:           true,
			FixtureID:    "fixture-one",
			AverageScore: 100,
			MinimumScore: 100,
			DimensionScores: []IssueOpsDimensionScore{
				{Dimension: "issue_quality", Score: 100, Evidence: "deterministic issue"},
				{Dimension: "plan_quality", Score: 90, Evidence: "deterministic plan"},
			},
			Passed: true,
		}},
	})

	if err := SaveIssueOpsBenchmarkRun(stateRoot, result); err != nil {
		t.Fatalf("SaveIssueOpsBenchmarkRun() error = %v", err)
	}
	loaded, err := ReadIssueOpsBenchmarkRun(stateRoot, " run-one ")
	if err != nil {
		t.Fatalf("ReadIssueOpsBenchmarkRun() error = %v", err)
	}
	if loaded.ID != result.ID || loaded.FixtureCount != 1 {
		t.Fatalf("loaded benchmark result mismatch: %+v", loaded)
	}
	if _, err := ReadIssueOpsBenchmarkRun(stateRoot, " "); err == nil {
		t.Fatalf("expected empty benchmark id to fail")
	}

	merged := MergeIssueOpsBenchmarkScoreWithJudge(result.Scores[0], IssueOpsBenchmarkScore{
		DimensionScores: []IssueOpsDimensionScore{
			{Dimension: "issue_quality", Score: 70, Evidence: "judge issue"},
		},
		JudgeFailures:    []string{"judge warning"},
		CriticalFailures: []string{"critical judge finding"},
	})
	if merged.Passed || merged.OK {
		t.Fatalf("expected judge failures to fail merged score: %+v", merged)
	}
	if merged.DimensionScores[0].Score != 70 {
		t.Fatalf("expected lower judge score to replace deterministic score: %+v", merged.DimensionScores)
	}
	if !strings.Contains(merged.DimensionScores[0].Evidence, "judge: judge issue") {
		t.Fatalf("expected judge evidence to be retained: %+v", merged.DimensionScores[0])
	}

	emptyJudge := MergeIssueOpsBenchmarkScoreWithJudge(result.Scores[0], IssueOpsBenchmarkScore{})
	if emptyJudge.Passed || len(emptyJudge.JudgeFailures) == 0 {
		t.Fatalf("expected missing judge dimensions to fail merged score: %+v", emptyJudge)
	}
}
