package main

import (
	"testing"
)

func TestCompareSelfAugmentSummaries(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	baseSummary := SelfAugmentSummary{
		TotalRuns:   10,
		TotalSteps:  20,
		PassedSteps: 20,
		StepLabels:  []string{"go test", "MCP smoke"},
		SlowestSteps: []SelfAugmentSlowStep{
			{Iteration: 1, Seed: 400, Label: "go test", DurationMS: 1000},
		},
	}
	candidateSummary := baseSummary
	if err := writeSelfAugmentSnapshotRecord(dir, "baseline", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      400,
		ElapsedMS:     1000,
		HarnessRoot:   "/tmp/harness",
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       baseSummary,
	}); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "candidate", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      400,
		ElapsedMS:     1100,
		HarnessRoot:   "/tmp/harness",
		GeneratedAt:   "2000-01-01T00:01:00Z",
		Summary:       candidateSummary,
	}); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	okResult, err := compareSelfAugmentSummaries("baseline", "candidate", 20)
	if err != nil {
		t.Fatalf("compare ok: %v", err)
	}
	if !okResult.OK || okResult.Regressed || okResult.ElapsedDeltaMS != 100 || okResult.FailedStepsDelta != 0 {
		t.Fatalf("unexpected non-regression result: %+v", okResult)
	}
	regressed, err := compareSelfAugmentSummaries("baseline", "candidate", 5)
	if err != nil {
		t.Fatalf("compare regression: %v", err)
	}
	if !regressed.OK || !regressed.Regressed || len(regressed.Regressions) == 0 {
		t.Fatalf("expected regression: %+v", regressed)
	}
}

func TestCompareSelfAugmentSummariesDetectsFailedStepRegression(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	baseline := SelfAugmentSummary{TotalRuns: 10, TotalSteps: 20, PassedSteps: 20, StepLabels: []string{"go test", "MCP smoke"}}
	candidate := SelfAugmentSummary{TotalRuns: 10, TotalSteps: 20, PassedSteps: 19, FailedSteps: 1, StepLabels: []string{"go test"}}
	if err := writeSelfAugmentSnapshotRecord(dir, "baseline", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      500,
		ElapsedMS:     1000,
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       baseline,
	}); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "candidate", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            false,
		Iterations:    10,
		BaseSeed:      500,
		ElapsedMS:     900,
		GeneratedAt:   "2000-01-01T00:01:00Z",
		Summary:       candidate,
	}); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	result, err := compareSelfAugmentSummaries("baseline", "candidate", 20)
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if !result.Regressed || !containsString(result.MissingStepLabels, "MCP smoke") || result.FailedStepsDelta != 1 {
		t.Fatalf("expected failed-step and missing-label regression: %+v", result)
	}
}

func TestCompareSelfAugmentSummariesDetectsSlowStepRegression(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	baseline := SelfAugmentSummary{
		TotalRuns:   10,
		TotalSteps:  20,
		PassedSteps: 20,
		StepLabels:  []string{"go test", "MCP smoke"},
		SlowestSteps: []SelfAugmentSlowStep{
			{Iteration: 1, Seed: 600, Label: "go test", DurationMS: 1000},
			{Iteration: 1, Seed: 600, Label: "MCP smoke", DurationMS: 100},
		},
	}
	candidate := baseline
	candidate.SlowestSteps = []SelfAugmentSlowStep{
		{Iteration: 1, Seed: 600, Label: "go test", DurationMS: 1400},
		{Iteration: 1, Seed: 600, Label: "MCP smoke", DurationMS: 100},
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "baseline", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      600,
		ElapsedMS:     1000,
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       baseline,
	}); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "candidate", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      600,
		ElapsedMS:     1000,
		GeneratedAt:   "2000-01-01T00:01:00Z",
		Summary:       candidate,
	}); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	result, err := compareSelfAugmentSummaries("baseline", "candidate", 20)
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if !result.Regressed || len(result.SlowStepRegressions) != 1 {
		t.Fatalf("expected slow-step regression: %+v", result)
	}
	regression := result.SlowStepRegressions[0]
	if regression.Label != "go test" || regression.DeltaMS != 400 || regression.DeltaPct != 40 {
		t.Fatalf("unexpected slow-step regression detail: %+v", regression)
	}
	if !containsString(result.Regressions, "slow_step:go test_increased_by_40.00_pct") {
		t.Fatalf("missing slow-step regression marker: %+v", result.Regressions)
	}
}

func TestCompareSelfAugmentSummariesDetectsStepBudgetRegressionBeyondSlowestTopFive(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	baseline := SelfAugmentSummary{
		TotalRuns:   10,
		TotalSteps:  60,
		PassedSteps: 60,
		StepLabels:  []string{"go test", "MCP smoke", "docs index smoke"},
		SlowestSteps: []SelfAugmentSlowStep{
			{Iteration: 1, Seed: 601, Label: "go test", DurationMS: 2000},
			{Iteration: 1, Seed: 601, Label: "MCP smoke", DurationMS: 1500},
		},
		StepDurationStats: []SelfAugmentStepDurationStat{
			{Label: "MCP smoke", Count: 10, MinDurationMS: 1200, MaxDurationMS: 1500, AverageDurationMS: 1400, P95DurationMS: 1500},
			{Label: "docs index smoke", Count: 10, MinDurationMS: 90, MaxDurationMS: 100, AverageDurationMS: 95, P95DurationMS: 100},
			{Label: "go test", Count: 10, MinDurationMS: 1800, MaxDurationMS: 2000, AverageDurationMS: 1900, P95DurationMS: 2000},
		},
	}
	candidate := baseline
	candidate.StepDurationStats = []SelfAugmentStepDurationStat{
		{Label: "MCP smoke", Count: 10, MinDurationMS: 1200, MaxDurationMS: 1500, AverageDurationMS: 1400, P95DurationMS: 1500},
		{Label: "docs index smoke", Count: 10, MinDurationMS: 90, MaxDurationMS: 130, AverageDurationMS: 105, P95DurationMS: 130},
		{Label: "go test", Count: 10, MinDurationMS: 1800, MaxDurationMS: 2000, AverageDurationMS: 1900, P95DurationMS: 2000},
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "baseline", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      601,
		ElapsedMS:     1000,
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       baseline,
	}); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "candidate", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      601,
		ElapsedMS:     1000,
		GeneratedAt:   "2000-01-01T00:01:00Z",
		Summary:       candidate,
	}); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	result, err := compareSelfAugmentSummaries("baseline", "candidate", 5)
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if !result.Regressed || len(result.StepBudgetRegressions) != 1 || len(result.SlowStepRegressions) != 0 {
		t.Fatalf("expected budget-only regression: %+v", result)
	}
	regression := result.StepBudgetRegressions[0]
	if regression.Label != "docs index smoke" || regression.Metric != "p95_duration_ms" || regression.DeltaMS != 30 || regression.DeltaPct != 30 {
		t.Fatalf("unexpected step-budget regression detail: %+v", regression)
	}
	if !containsString(result.Regressions, "step_budget:docs index smoke_p95_increased_by_30.00_pct") {
		t.Fatalf("missing step-budget regression marker: %+v", result.Regressions)
	}
}

func TestCompareStepBudgetRegressionsIgnoresTinyAbsoluteNoise(t *testing.T) {
	baseline := []SelfAugmentStepDurationStat{{Label: "state roundtrip", Count: 10, P95DurationMS: 76}}
	candidate := []SelfAugmentStepDurationStat{{Label: "state roundtrip", Count: 10, P95DurationMS: 83}}
	regressions := compareStepBudgetRegressions(baseline, candidate, 5)
	if len(regressions) != 0 {
		t.Fatalf("expected tiny p95 delta to be ignored, got %+v", regressions)
	}
}
