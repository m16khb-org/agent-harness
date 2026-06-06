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

func TestFinalizeAndCompareIssueOpsBenchmarkRunsSummarizesScoresAndRegressions(t *testing.T) {
	baseline := FinalizeIssueOpsBenchmarkRunResult(IssueOpsBenchmarkRunResult{
		ID: "baseline",
		Scores: []IssueOpsBenchmarkScore{{
			FixtureID:    "fixture-one",
			AverageScore: 90,
			MinimumScore: 90,
			DimensionScores: []IssueOpsDimensionScore{
				{Dimension: "issue_quality", Score: 90},
				{Dimension: "plan_quality", Score: 95},
			},
			Passed: true,
		}},
	})
	candidate := FinalizeIssueOpsBenchmarkRunResult(IssueOpsBenchmarkRunResult{
		ID: "candidate",
		Scores: []IssueOpsBenchmarkScore{{
			FixtureID:    "fixture-one",
			AverageScore: 95,
			MinimumScore: 85,
			DimensionScores: []IssueOpsDimensionScore{
				{Dimension: "issue_quality", Score: 85},
				{Dimension: "plan_quality", Score: 100},
			},
			Passed: true,
		}},
	})
	if !baseline.OK || baseline.FixtureCount != 1 || baseline.CriticalFailureCount != 0 || baseline.AverageScore != 90 || baseline.MinimumScore != 90 {
		t.Fatalf("baseline summary mismatch: %+v", baseline)
	}
	compare := CompareIssueOpsBenchmarkRuns(baseline, candidate)
	if compare.OK || compare.Improved || compare.AverageScoreDelta != 5 || compare.MinimumScoreDelta != -5 {
		t.Fatalf("candidate with minimum regression should not pass compare: %+v", compare)
	}
	if len(compare.Regressions) == 0 {
		t.Fatalf("expected dimension regression to be reported: %+v", compare)
	}

	empty := FinalizeIssueOpsBenchmarkRunResult(IssueOpsBenchmarkRunResult{ID: "empty"})
	if !empty.OK || empty.AverageScore != 0 || empty.MinimumScore != 0 || empty.FixtureCount != 0 {
		t.Fatalf("empty benchmark should summarize to zero without synthetic failures: %+v", empty)
	}
}
