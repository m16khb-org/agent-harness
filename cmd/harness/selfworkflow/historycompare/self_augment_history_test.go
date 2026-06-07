package historycompare

import (
	"testing"

	"agent-harness/cmd/harness/selfworkflow/stateio"
	"agent-harness/internal/core"
)

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
	if err := stateio.WriteSelfAugmentSnapshotRecord(dir, "self-verify-old", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      800,
		ElapsedMS:     1200,
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write old snapshot: %v", err)
	}
	if err := stateio.WriteSelfAugmentSnapshotRecord(dir, "self-verify-new", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      801,
		ElapsedMS:     1000,
		GeneratedAt:   "2000-01-02T00:00:00Z",
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write new snapshot: %v", err)
	}
	if err := stateio.WriteSelfAugmentSnapshotRecord(dir, "other-summary", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      802,
		ElapsedMS:     900,
		GeneratedAt:   "2000-01-03T00:00:00Z",
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write other snapshot: %v", err)
	}
	if _, err := core.StateWrite("self-verify-note", "not a summary"); err != nil {
		t.Fatalf("write non-summary state: %v", err)
	}

	limited, err := SelfAugmentHistory("self-verify", 1)
	if err != nil {
		t.Fatalf("history limited: %v", err)
	}
	if !limited.OK || limited.TotalMatches != 2 || limited.Returned != 1 || limited.Entries[0].Key != "self-verify-new" {
		t.Fatalf("unexpected limited history: %+v", limited)
	}
	if !historySkippedKey(limited.Skipped, "self-verify-note") {
		t.Fatalf("expected non-summary key to be skipped: %+v", limited.Skipped)
	}
	if limited.Retention != nil {
		t.Fatalf("retention should be omitted when no retention limit is requested: %+v", limited.Retention)
	}

	retentionPlan, err := SelfAugmentHistory("self-verify", 0, SelfAugmentHistoryRetentionOptions{Limit: 1})
	if err != nil {
		t.Fatalf("history retention plan: %v", err)
	}
	if retentionPlan.Retention == nil || !retentionPlan.Retention.Enabled || retentionPlan.Retention.Limit != 1 {
		t.Fatalf("retention plan missing: %+v", retentionPlan.Retention)
	}
	if retentionPlan.Retention.TotalMatches != 2 ||
		!containsString(retentionPlan.Retention.RetainedKeys, "self-verify-new") ||
		!containsString(retentionPlan.Retention.CandidateKeys, "self-verify-old") ||
		len(retentionPlan.Retention.DeletedKeys) != 0 {
		t.Fatalf("unexpected retention plan: %+v", retentionPlan.Retention)
	}
	if !containsString(retentionPlan.Warnings, "history_retention_candidates:1") {
		t.Fatalf("retention plan should warn about prune candidates: %+v", retentionPlan.Warnings)
	}

	retentionDryRun, err := SelfAugmentHistory("self-verify", 0, SelfAugmentHistoryRetentionOptions{Limit: 1, PruneRequested: true})
	if err != nil {
		t.Fatalf("history retention dry-run: %v", err)
	}
	if retentionDryRun.Retention == nil || !retentionDryRun.Retention.DryRun || retentionDryRun.Retention.Confirm || len(retentionDryRun.Retention.DeletedKeys) != 0 {
		t.Fatalf("unexpected retention dry-run: %+v", retentionDryRun.Retention)
	}
	if _, err := core.StateRead("self-verify-old"); err != nil {
		t.Fatalf("retention dry-run deleted old summary: %v", err)
	}

	retentionConfirmed, err := SelfAugmentHistory("self-verify", 0, SelfAugmentHistoryRetentionOptions{Limit: 1, PruneRequested: true, Confirm: true})
	if err != nil {
		t.Fatalf("history retention confirm: %v", err)
	}
	if retentionConfirmed.Retention == nil || retentionConfirmed.Retention.DryRun || !retentionConfirmed.Retention.Confirm || !containsString(retentionConfirmed.Retention.DeletedKeys, "self-verify-old") {
		t.Fatalf("unexpected retention confirm: %+v", retentionConfirmed.Retention)
	}
	if _, err := core.StateRead("self-verify-old"); err == nil {
		t.Fatalf("retention confirm left old summary in state")
	}

	all, err := SelfAugmentHistory("", 0)
	if err != nil {
		t.Fatalf("history all: %v", err)
	}
	if all.TotalMatches != 2 || all.Returned != 2 || all.Entries[0].Key != "other-summary" {
		t.Fatalf("unexpected all history ordering/counts: %+v", all)
	}
}

func TestSelfAugmentHistoryRetentionRejectsUnsafeOptions(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if _, err := SelfAugmentHistory("self-verify", 0, SelfAugmentHistoryRetentionOptions{Limit: -1}); err == nil {
		t.Fatalf("negative retention limit was accepted")
	}
	if _, err := SelfAugmentHistory("self-verify", 0, SelfAugmentHistoryRetentionOptions{Confirm: true}); err == nil {
		t.Fatalf("confirm without prune-retention was accepted")
	}
	if _, err := SelfAugmentHistory("self-verify", 0, SelfAugmentHistoryRetentionOptions{PruneRequested: true}); err == nil {
		t.Fatalf("prune-retention without positive retention limit was accepted")
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
