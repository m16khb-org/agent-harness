package historycompare

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"issueops/cmd/issueops/selfworkflow/model"
	"issueops/cmd/issueops/selfworkflow/stateio"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/testsupport"
)

func TestRunSelfVerifyCompareTextAndFailOnRegression(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)
	writeSelfVerifyCLISnapshotForTest(t, dir, "baseline-cli", 1000, true, 20, 20, "2000-01-01T00:00:00Z")
	writeSelfVerifyCLISnapshotForTest(t, dir, "candidate-cli", 1300, false, 20, 19, "2000-01-01T00:01:00Z")

	out := captureStdout(t, func() error {
		return RunSelfVerifyCompare([]string{
			"--baseline-key", "baseline-cli",
			"--candidate-key", "candidate-cli",
			"--max-elapsed-regression-pct", "5",
		}, CLIDeps{})
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
	}, CLIDeps{PrintJSON: printJSONForTest})
	if err == nil || !strings.Contains(err.Error(), "summary regression detected") {
		t.Fatalf("expected fail-on-regression error, got %v", err)
	}
}

func TestRunSelfVerifyHistoryTextOutputCoversSkippedAndRetentionActions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)
	writeSelfVerifyCLISnapshotForTest(t, dir, "self-verify-old-cli", 1200, false, 20, 19, "2000-01-01T00:00:00Z")
	writeSelfVerifyCLISnapshotForTest(t, dir, "self-verify-new-cli", 900, true, 20, 20, "2000-01-02T00:00:00Z")
	if _, err := statestore.StateWrite("self-verify-note-cli", "not a summary"); err != nil {
		t.Fatalf("write non-summary state: %v", err)
	}

	planned := captureStdout(t, func() error {
		return RunSelfVerifyHistory([]string{"--prefix", "self-verify", "--retention-limit", "1"}, CLIDeps{})
	})
	if !strings.Contains(planned, "self-verify history: 2/2 entries") ||
		!strings.Contains(planned, "- self-verify-new-cli ok iterations=10 elapsed=900ms") ||
		!strings.Contains(planned, "- self-verify-old-cli fail iterations=10 elapsed=1200ms") ||
		!strings.Contains(planned, "skipped 1 non-summary records") ||
		!strings.Contains(planned, "retention: retain=1 candidates=1 planned=0") {
		t.Fatalf("unexpected planned history text:\n%s", planned)
	}

	dryRun := captureStdout(t, func() error {
		return RunSelfVerifyHistory([]string{"--prefix", "self-verify", "--retention-limit", "1", "--prune-retention"}, CLIDeps{})
	})
	if !strings.Contains(dryRun, "retention: retain=1 candidates=1 would delete=0") {
		t.Fatalf("unexpected dry-run history text:\n%s", dryRun)
	}
	if _, err := statestore.StateRead("self-verify-old-cli"); err != nil {
		t.Fatalf("dry-run deleted old summary: %v", err)
	}
}

func TestRunSelfVerifyHistoryJSONRejectsUnsafeRetentionOptions(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	err := RunSelfVerifyHistory([]string{"--confirm", "--json"}, CLIDeps{PrintJSON: printJSONForTest})
	if err == nil || !strings.Contains(err.Error(), "requires --prune-retention") {
		t.Fatalf("expected unsafe retention option error, got %v", err)
	}
}

func TestRunSelfVerifyCompareJSONOutput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)
	writeSelfVerifyCLISnapshotForTest(t, dir, "baseline-json", 1000, true, 20, 20, "2000-01-01T00:00:00Z")
	writeSelfVerifyCLISnapshotForTest(t, dir, "candidate-json", 1010, true, 20, 20, "2000-01-01T00:01:00Z")

	out := captureStdout(t, func() error {
		return RunSelfVerifyCompare([]string{"--baseline-key", "baseline-json", "--candidate-key", "candidate-json", "--json"}, CLIDeps{PrintJSON: printJSONForTest})
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
	if err := stateio.WriteSelfAugmentSnapshotRecord(dir, key, SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          model.SelfVerificationSummaryKind,
		OK:            ok,
		Iterations:    10,
		BaseSeed:      900,
		ElapsedMS:     elapsedMS,
		IssueOpsRoot:  filepath.Join(dir, "repo"),
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

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	return testsupport.CaptureStdout(t, fn)
}

func printJSONForTest(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
