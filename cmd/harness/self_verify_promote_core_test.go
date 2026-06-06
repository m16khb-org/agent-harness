package main

import (
	"strings"
	"testing"
)

func TestPromoteSelfAugmentBaselineRejectsMissingKeysAndBadSource(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	result, err := promoteSelfAugmentBaseline("", "baseline", false)
	if err == nil || !strings.Contains(err.Error(), "from-key is required") {
		t.Fatalf("expected missing from-key error, result=%+v err=%v", result, err)
	}
	if result.OK || !result.DryRun || result.FromKey != "" || result.BaselineKey != "baseline" {
		t.Fatalf("unexpected missing from-key result: %+v", result)
	}

	result, err = promoteSelfAugmentBaseline("candidate", "", true)
	if err == nil || !strings.Contains(err.Error(), "baseline-key is required") {
		t.Fatalf("expected missing baseline-key error, result=%+v err=%v", result, err)
	}
	if result.OK || result.DryRun || result.FromKey != "candidate" || result.BaselineKey != "" {
		t.Fatalf("unexpected missing baseline-key result: %+v", result)
	}

	result, err = promoteSelfAugmentBaseline("missing-source", "baseline", false)
	if err == nil || !strings.Contains(err.Error(), "read source summary") {
		t.Fatalf("expected wrapped source read error, result=%+v err=%v", result, err)
	}
	if result.OK || !result.DryRun || result.SnapshotGeneratedAt != "" {
		t.Fatalf("unexpected missing source result: %+v", result)
	}
}

func TestPromoteSelfAugmentBaselinePropagatesDestinationWriteError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	source := SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          selfVerificationSummaryKind,
		OK:            true,
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       SelfAugmentSummary{TotalRuns: 1, TotalSteps: 1, PassedSteps: 1},
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "candidate", source); err != nil {
		t.Fatalf("write candidate: %v", err)
	}

	result, err := promoteSelfAugmentBaseline("candidate", "!bad-key", true)
	if err == nil || !strings.Contains(err.Error(), "invalid state key") {
		t.Fatalf("expected invalid destination key error, result=%+v err=%v", result, err)
	}
	if result.OK || result.Promoted || result.Path != "" || result.Bytes != 0 || result.SnapshotGeneratedAt != source.GeneratedAt {
		t.Fatalf("unexpected invalid destination result: %+v", result)
	}
}
