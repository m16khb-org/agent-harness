package stateio

import (
	"strings"
	"testing"

	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/internal/adapter/core"
)

func writePromoteGateSnapshotForTest(t *testing.T, dir, key string, ok, terminationEligible bool) {
	t.Helper()
	snapshot := SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            ok,
		Iterations:    1,
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       model.SelfAugmentSummary{TotalRuns: 1, TotalSteps: 2, PassedSteps: 2, TerminationEligible: terminationEligible},
	}
	if err := WriteSelfAugmentSnapshotRecord(dir, key, snapshot); err != nil {
		t.Fatal(err)
	}
}

// SA-P selected gate (measured by SV-B): promote --confirm wrote ANY snapshot
// into the baseline without checking it passed, so one failed run could
// silently poison every future compare. Confirmed promotion now refuses
// failing sources unless explicitly overridden.
func TestPromoteRefusesFailedSourceWithoutOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	writePromoteGateSnapshotForTest(t, dir, "candidate", false, false)

	_, err := PromoteSelfAugmentBaseline("candidate", "baseline", true, false)
	if err == nil || !strings.Contains(err.Error(), "did not pass") {
		t.Fatalf("confirmed promote of a failing snapshot must refuse, got err=%v", err)
	}
	if _, err := core.StateRead("baseline"); err == nil {
		t.Fatal("refused promote must not write the baseline")
	}
}

func TestPromoteDryRunReportsSourcePassed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	writePromoteGateSnapshotForTest(t, dir, "candidate", false, false)

	dry, err := PromoteSelfAugmentBaseline("candidate", "baseline", false, false)
	if err != nil || !dry.OK || !dry.DryRun {
		t.Fatalf("dry-run must stay diagnostic: %+v err=%v", dry, err)
	}
	if dry.SourcePassed {
		t.Fatalf("dry-run must report source_passed=false for a failing snapshot: %+v", dry)
	}
}

func TestPromoteAllowsFailedSourceWithExplicitOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	writePromoteGateSnapshotForTest(t, dir, "candidate", false, false)

	confirmed, err := PromoteSelfAugmentBaseline("candidate", "baseline", true, true)
	if err != nil || !confirmed.Promoted || confirmed.SourcePassed {
		t.Fatalf("explicit override must promote and still report source_passed=false: %+v err=%v", confirmed, err)
	}
}

func TestPromotePassingSourceStillPromotes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	writePromoteGateSnapshotForTest(t, dir, "candidate", true, true)

	confirmed, err := PromoteSelfAugmentBaseline("candidate", "baseline", true, false)
	if err != nil || !confirmed.Promoted || !confirmed.SourcePassed {
		t.Fatalf("passing snapshot must promote without override: %+v err=%v", confirmed, err)
	}
}
