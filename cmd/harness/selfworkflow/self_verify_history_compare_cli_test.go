package selfworkflow

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunSelfVerifyCompareTextAndFailOnRegression(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	writeSelfVerifyCLISnapshotForTest(t, dir, "baseline-cli", 1000, true, 20, 20, "2000-01-01T00:00:00Z")
	writeSelfVerifyCLISnapshotForTest(t, dir, "candidate-cli", 1300, false, 20, 19, "2000-01-01T00:01:00Z")

	out := captureStatusVerifyStdout(t, func() error {
		return RunSelfVerifyCompare([]string{
			"--baseline-key", "baseline-cli",
			"--candidate-key", "candidate-cli",
			"--max-elapsed-regression-pct", "5",
		})
	})
	if !strings.Contains(out, "self-verify compare regressed") ||
		!strings.Contains(out, "candidate_not_ok") ||
		!strings.Contains(out, "elapsed_ms_increased_by_30.00_pct") ||
		!strings.Contains(out, "failed_steps_increased_by_1") {
		t.Fatalf("unexpected compare text output:\n%s", out)
	}

	err := RunSelfVerifyCompare([]string{
		"--baseline-key", "baseline-cli",
		"--candidate-key", "candidate-cli",
		"--max-elapsed-regression-pct", "5",
		"--fail-on-regression",
		"--json",
	})
	if err == nil || !strings.Contains(err.Error(), "summary regression detected") {
		t.Fatalf("expected fail-on-regression error, got %v", err)
	}
}

func TestRunSelfVerifyHistoryTextOutputCoversSkippedAndRetentionActions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	writeSelfVerifyCLISnapshotForTest(t, dir, "self-verify-old-cli", 1200, false, 20, 19, "2000-01-01T00:00:00Z")
	writeSelfVerifyCLISnapshotForTest(t, dir, "self-verify-new-cli", 900, true, 20, 20, "2000-01-02T00:00:00Z")
	if _, err := core.StateWrite("self-verify-note-cli", "not a summary"); err != nil {
		t.Fatalf("write non-summary state: %v", err)
	}

	planned := captureStatusVerifyStdout(t, func() error {
		return RunSelfVerifyHistory([]string{"--prefix", "self-verify", "--retention-limit", "1"})
	})
	if !strings.Contains(planned, "self-verify history: 2/2 entries") ||
		!strings.Contains(planned, "- self-verify-new-cli ok iterations=10 elapsed=900ms") ||
		!strings.Contains(planned, "- self-verify-old-cli fail iterations=10 elapsed=1200ms") ||
		!strings.Contains(planned, "skipped 1 non-summary records") ||
		!strings.Contains(planned, "retention: retain=1 candidates=1 planned=0") {
		t.Fatalf("unexpected planned history text:\n%s", planned)
	}

	dryRun := captureStatusVerifyStdout(t, func() error {
		return RunSelfVerifyHistory([]string{"--prefix", "self-verify", "--retention-limit", "1", "--prune-retention"})
	})
	if !strings.Contains(dryRun, "retention: retain=1 candidates=1 would delete=0") {
		t.Fatalf("unexpected dry-run history text:\n%s", dryRun)
	}
	if _, err := core.StateRead("self-verify-old-cli"); err != nil {
		t.Fatalf("dry-run deleted old summary: %v", err)
	}
}

func TestRunSelfVerifyHistoryJSONRejectsUnsafeRetentionOptions(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	err := RunSelfVerifyHistory([]string{"--confirm", "--json"})
	if err == nil || !strings.Contains(err.Error(), "requires --prune-retention") {
		t.Fatalf("expected unsafe retention option error, got %v", err)
	}
}

func TestRunSelfVerifyCompareJSONOutput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	writeSelfVerifyCLISnapshotForTest(t, dir, "baseline-json", 1000, true, 20, 20, "2000-01-01T00:00:00Z")
	writeSelfVerifyCLISnapshotForTest(t, dir, "candidate-json", 1010, true, 20, 20, "2000-01-01T00:01:00Z")

	out := captureStatusVerifyStdout(t, func() error {
		return RunSelfVerifyCompare([]string{"--baseline-key", "baseline-json", "--candidate-key", "candidate-json", "--json"})
	})
	var result SelfAugmentCompareResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode compare JSON: %v\n%s", err, out)
	}
	if !result.OK || result.Regressed || result.BaselineKey != "baseline-json" || result.CandidateKey != "candidate-json" {
		t.Fatalf("unexpected compare JSON result: %+v", result)
	}
}

func writeSelfVerifyCLISnapshotForTest(t *testing.T, dir, key string, elapsedMS int64, ok bool, totalSteps, passedSteps int, generatedAt string) {
	t.Helper()
	if err := WriteSelfAugmentSnapshotRecord(dir, key, SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          selfVerificationSummaryKind,
		OK:            ok,
		Iterations:    10,
		BaseSeed:      900,
		ElapsedMS:     elapsedMS,
		HarnessRoot:   filepath.Join(dir, "repo"),
		GeneratedAt:   generatedAt,
		Summary: SelfAugmentSummary{
			TotalRuns:   10,
			TotalSteps:  totalSteps,
			PassedSteps: passedSteps,
			FailedSteps: totalSteps - passedSteps,
			StepLabels:  []string{"go test", "MCP smoke"},
		},
	}); err != nil {
		t.Fatalf("write snapshot %q: %v", key, err)
	}
}
