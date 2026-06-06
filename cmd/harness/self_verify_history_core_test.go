package main

import (
	"testing"
)

func TestSelfAugmentHistoryCoversInvalidTimestampSchemaSkipAndNilSlices(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	if err := writeSelfAugmentSnapshotRecord(dir, "self-verify-invalid-time", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          selfVerificationSummaryKind,
		OK:            true,
		GeneratedAt:   "not-a-time",
		Summary:       SelfAugmentSummary{TotalRuns: 1, TotalSteps: 1},
	}); err != nil {
		t.Fatalf("write invalid time: %v", err)
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "self-verify-bad-schema", SelfAugmentStateSnapshot{
		SchemaVersion: 2,
		Kind:          selfVerificationSummaryKind,
		Summary:       SelfAugmentSummary{TotalRuns: 1},
	}); err != nil {
		t.Fatalf("write bad schema: %v", err)
	}

	result, err := selfAugmentHistory("self-verify", 0)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if !containsString(result.Warnings, "invalid_generated_at:self-verify-invalid-time") {
		t.Fatalf("missing invalid timestamp warning: %+v", result.Warnings)
	}
	if !historySkippedKey(result.Skipped, "self-verify-bad-schema") {
		t.Fatalf("missing bad schema skip: %+v", result.Skipped)
	}
	if len(result.Entries) != 1 || result.Entries[0].StepLabels == nil || result.Entries[0].SlowestSteps == nil {
		t.Fatalf("expected one entry with non-nil slices: %+v", result.Entries)
	}
}

func TestParseSelfAugmentTimestampCoversEmptyInvalidAndRFC3339Fallback(t *testing.T) {
	if _, ok := parseSelfAugmentTimestamp(""); ok {
		t.Fatal("empty timestamp parsed")
	}
	if _, ok := parseSelfAugmentTimestamp("not-a-time"); ok {
		t.Fatal("invalid timestamp parsed")
	}
	parsed, ok := parseSelfAugmentTimestamp("2000-01-01T00:00:00Z")
	if !ok || parsed.Year() != 2000 {
		t.Fatalf("expected RFC3339 timestamp to parse, parsed=%v ok=%v", parsed, ok)
	}
}
