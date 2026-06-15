package benchmark

import (
	"fmt"
	"testing"
)

func TestValidateJudgeProvenanceFailsClosed(t *testing.T) {
	read := func(stateRoot, id string) (IssueOpsBenchmarkRunResult, error) {
		if id == "prior-run" {
			return IssueOpsBenchmarkRunResult{ID: id}, nil
		}
		return IssueOpsBenchmarkRunResult{}, fmt.Errorf("not found")
	}
	const scored = "scored-run"

	for name, judge := range map[string]IssueOpsJudgeMap{
		"missing source_run_id": {Provenance: "recorded judge"},
		"missing provenance":    {SourceRunID: "prior-run"},
		"self-attributed":       {SourceRunID: scored, Provenance: "recorded judge"},
		"unresolvable source":   {SourceRunID: "ghost", Provenance: "recorded judge"},
	} {
		if err := validateJudgeProvenance(judge, scored, "", read); err == nil {
			t.Fatalf("%s must fail closed", name)
		}
	}

	valid := IssueOpsJudgeMap{SourceRunID: "prior-run", Provenance: "recorded fresh-context judge"}
	if err := validateJudgeProvenance(valid, scored, "", read); err != nil {
		t.Fatalf("distinct + resolvable source must pass: %v", err)
	}
}

func TestJudgeDownwardOverrideRate(t *testing.T) {
	deterministic := IssueOpsBenchmarkScore{DimensionScores: []IssueOpsDimensionScore{
		{Dimension: "a", Score: 100},
		{Dimension: "b", Score: 100},
		{Dimension: "c", Score: 0, NotApplicable: true}, // N/A: excluded
		{Dimension: "d", Score: 100},                    // judge does not score: excluded
	}}
	judge := IssueOpsBenchmarkScore{DimensionScores: []IssueOpsDimensionScore{
		{Dimension: "a", Score: 100}, // not lowered
		{Dimension: "b", Score: 60},  // lowered
		{Dimension: "c", Score: 100}, // N/A in deterministic: excluded
	}}
	rate, comparable := JudgeDownwardOverrideRate(deterministic, judge)
	if comparable != 2 {
		t.Fatalf("comparable = %d, want 2 (a,b; c is N/A, d unscored)", comparable)
	}
	if rate != 0.5 {
		t.Fatalf("downward-override rate = %v, want 0.5 (1 of 2 lowered)", rate)
	}

	if r, c := JudgeDownwardOverrideRate(IssueOpsBenchmarkScore{}, IssueOpsBenchmarkScore{}); r != 0 || c != 0 {
		t.Fatalf("no comparable dimensions = %v/%d, want 0/0", r, c)
	}
}
