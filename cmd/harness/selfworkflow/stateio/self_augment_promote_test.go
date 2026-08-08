package stateio

import (
	"path/filepath"
	"testing"

	"agent-harness/cmd/harness/selfworkflow/model"
	statestore "agent-harness/internal/adapter/outbound/state"
)

func TestPromoteSelfAugmentBaseline(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	summary := model.SelfAugmentSummary{TotalRuns: 10, TotalSteps: 20, PassedSteps: 20, TerminationEligible: true, StepLabels: []string{"go test"}}
	source := SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      700,
		ElapsedMS:     1000,
		HarnessRoot:   "/tmp/harness",
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       summary,
	}
	if err := WriteSelfAugmentSnapshotRecord(dir, "candidate", source); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	dry, err := PromoteSelfAugmentBaseline("candidate", "baseline", false, false)
	if err != nil {
		t.Fatalf("promote dry-run: %v", err)
	}
	if !dry.OK || !dry.DryRun || dry.Promoted {
		t.Fatalf("unexpected dry-run promote result: %+v", dry)
	}
	if _, err := statestore.StateRead("baseline"); err == nil {
		t.Fatalf("dry-run wrote baseline")
	}
	confirmed, err := PromoteSelfAugmentBaseline("candidate", "baseline", true, false)
	if err != nil {
		t.Fatalf("promote confirm: %v", err)
	}
	if !confirmed.OK || confirmed.DryRun || !confirmed.Promoted || confirmed.Path != filepath.Join(dir, "baseline.json") {
		t.Fatalf("unexpected confirmed promote result: %+v", confirmed)
	}
	baseline, err := ReadSelfAugmentStateSnapshot("baseline")
	if err != nil {
		t.Fatalf("read promoted baseline: %v", err)
	}
	if baseline.GeneratedAt != source.GeneratedAt || baseline.Summary.TotalSteps != source.Summary.TotalSteps {
		t.Fatalf("promoted baseline drifted: %+v", baseline)
	}
}
