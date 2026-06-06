package main

import (
	"strings"
	"testing"
)

func TestCompareSelfAugmentSummariesFromSnapshotsCoversWarningsAndGoalRegressions(t *testing.T) {
	baseline := SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          selfVerificationSummaryKind,
		OK:            true,
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary: SelfAugmentSummary{
			TotalSteps:          1,
			PassedSteps:         1,
			StepLabels:          []string{"go test"},
			TerminationEligible: true,
			MinimumGoalScore:    95,
		},
	}
	candidate := baseline
	candidate.OK = true
	candidate.ElapsedMS = 100
	candidate.GeneratedAt = "2000-01-01T00:01:00Z"
	candidate.Summary.TotalSteps = 2
	candidate.Summary.StepLabels = []string{"go test", "MCP smoke"}
	candidate.Summary.TerminationEligible = false
	candidate.Summary.MinimumGoalScore = 90

	result := compareSelfAugmentSummariesFromSnapshots("baseline", "candidate", 5, baseline, candidate)

	if !result.OK || !result.Regressed || result.ElapsedDeltaMS != 100 {
		t.Fatalf("unexpected compare result: %+v", result)
	}
	for _, want := range []string{"candidate_not_termination_eligible", "minimum_goal_score_decreased_by_5.00"} {
		if !containsString(result.Regressions, want) {
			t.Fatalf("missing regression %q in %+v", want, result.Regressions)
		}
	}
	for _, want := range []string{"baseline_elapsed_zero", "added_step_label:MCP smoke", "total_steps_delta_+1"} {
		if !containsString(result.Warnings, want) {
			t.Fatalf("missing warning %q in %+v", want, result.Warnings)
		}
	}
}

func TestReadSelfAugmentStateSnapshotRejectsBadKindAndSchemaAndAcceptsLegacyKind(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	summary := SelfAugmentSummary{TotalRuns: 1, TotalSteps: 1, PassedSteps: 1}

	if err := writeSelfAugmentSnapshotRecord(dir, "bad-kind", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "other",
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write bad kind: %v", err)
	}
	if _, err := readSelfAugmentStateSnapshot("bad-kind"); err == nil || !strings.Contains(err.Error(), "contains kind") {
		t.Fatalf("expected bad kind error, got %v", err)
	}

	if err := writeSelfAugmentSnapshotRecord(dir, "bad-schema", SelfAugmentStateSnapshot{
		SchemaVersion: 2,
		Kind:          selfVerificationSummaryKind,
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write bad schema: %v", err)
	}
	if _, err := readSelfAugmentStateSnapshot("bad-schema"); err == nil || !strings.Contains(err.Error(), "unsupported self-verification summary schema") {
		t.Fatalf("expected bad schema error, got %v", err)
	}

	if err := writeSelfAugmentSnapshotRecord(dir, "legacy", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          legacySelfAugmentSummaryKind,
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write legacy kind: %v", err)
	}
	if snapshot, err := readSelfAugmentStateSnapshot("legacy"); err != nil || snapshot.Kind != legacySelfAugmentSummaryKind {
		t.Fatalf("expected legacy kind to read, snapshot=%+v err=%v", snapshot, err)
	}
}
