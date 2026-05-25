package main

import (
	"encoding/json"
	"path/filepath"
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
	if err := saveSelfAugmentSummary(&result, "self-augment-test"); err != nil {
		t.Fatalf("saveSelfAugmentSummary: %v", err)
	}
	if result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("missing state checkpoint: %+v", result.StateCheckpoint)
	}
	if result.StateCheckpoint.Key != "self-augment-test" || result.StateCheckpoint.Path != filepath.Join(dir, "self-augment-test.json") {
		t.Fatalf("unexpected checkpoint metadata: %+v", result.StateCheckpoint)
	}
	state, err := core.StateRead("self-augment-test")
	if err != nil {
		t.Fatalf("StateRead: %v", err)
	}
	var snapshot SelfAugmentStateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		t.Fatalf("unmarshal saved snapshot: %v", err)
	}
	if snapshot.Kind != "self_augment_summary" || !snapshot.OK || snapshot.Summary.TotalSteps != 1 || snapshot.Summary.PassedSteps != 1 {
		t.Fatalf("unexpected saved snapshot: %+v", snapshot)
	}
}

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
		Kind:          "self_augment_summary",
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
		Kind:          "self_augment_summary",
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
		Kind:          "self_augment_summary",
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
		Kind:          "self_augment_summary",
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

func TestPromoteSelfAugmentBaseline(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	summary := SelfAugmentSummary{TotalRuns: 10, TotalSteps: 20, PassedSteps: 20, StepLabels: []string{"go test"}}
	source := SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_augment_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      700,
		ElapsedMS:     1000,
		HarnessRoot:   "/tmp/harness",
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       summary,
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "candidate", source); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	dry, err := promoteSelfAugmentBaseline("candidate", "baseline", false)
	if err != nil {
		t.Fatalf("promote dry-run: %v", err)
	}
	if !dry.OK || !dry.DryRun || dry.Promoted {
		t.Fatalf("unexpected dry-run promote result: %+v", dry)
	}
	if _, err := core.StateRead("baseline"); err == nil {
		t.Fatalf("dry-run wrote baseline")
	}
	confirmed, err := promoteSelfAugmentBaseline("candidate", "baseline", true)
	if err != nil {
		t.Fatalf("promote confirm: %v", err)
	}
	if !confirmed.OK || confirmed.DryRun || !confirmed.Promoted || confirmed.Path != filepath.Join(dir, "baseline.json") {
		t.Fatalf("unexpected confirmed promote result: %+v", confirmed)
	}
	baseline, err := readSelfAugmentStateSnapshot("baseline")
	if err != nil {
		t.Fatalf("read promoted baseline: %v", err)
	}
	if baseline.GeneratedAt != source.GeneratedAt || baseline.Summary.TotalSteps != source.Summary.TotalSteps {
		t.Fatalf("promoted baseline drifted: %+v", baseline)
	}
	compared, err := compareSelfAugmentSummaries("baseline", "candidate", 0)
	if err != nil {
		t.Fatalf("compare promoted: %v", err)
	}
	if compared.Regressed || compared.ElapsedDeltaMS != 0 {
		t.Fatalf("promoted baseline should compare cleanly: %+v", compared)
	}
}

func TestSelfAugmentHistory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	summary := SelfAugmentSummary{
		TotalRuns:   10,
		TotalSteps:  20,
		PassedSteps: 20,
		StepLabels:  []string{"go test", "MCP smoke"},
		SlowestSteps: []SelfAugmentSlowStep{
			{Iteration: 1, Seed: 800, Label: "go test", DurationMS: 1000},
		},
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "self-augment-old", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_augment_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      800,
		ElapsedMS:     1200,
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write old snapshot: %v", err)
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "self-augment-new", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_augment_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      801,
		ElapsedMS:     1000,
		GeneratedAt:   "2000-01-02T00:00:00Z",
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write new snapshot: %v", err)
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "other-summary", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_augment_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      802,
		ElapsedMS:     900,
		GeneratedAt:   "2000-01-03T00:00:00Z",
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write other snapshot: %v", err)
	}
	if _, err := core.StateWrite("self-augment-note", "not a summary"); err != nil {
		t.Fatalf("write non-summary state: %v", err)
	}

	limited, err := selfAugmentHistory("self-augment", 1)
	if err != nil {
		t.Fatalf("history limited: %v", err)
	}
	if !limited.OK || limited.TotalMatches != 2 || limited.Returned != 1 || limited.Entries[0].Key != "self-augment-new" {
		t.Fatalf("unexpected limited history: %+v", limited)
	}
	if !historySkippedKey(limited.Skipped, "self-augment-note") {
		t.Fatalf("expected non-summary key to be skipped: %+v", limited.Skipped)
	}

	all, err := selfAugmentHistory("", 0)
	if err != nil {
		t.Fatalf("history all: %v", err)
	}
	if all.TotalMatches != 3 || all.Returned != 3 || all.Entries[0].Key != "other-summary" {
		t.Fatalf("unexpected all history ordering/counts: %+v", all)
	}
}

func historySkippedKey(skipped []SelfAugmentHistorySkipped, key string) bool {
	for _, item := range skipped {
		if item.Key == key {
			return true
		}
	}
	return false
}
