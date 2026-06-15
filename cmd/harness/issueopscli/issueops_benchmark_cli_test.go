package issueopscli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunIssueOpsBenchmarkCompareAndGateTextBranches(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	baseline := benchmarkRunForCLITest("baseline", 80, "baseline evidence")
	candidate := benchmarkRunForCLITest("candidate", 95, "candidate evidence")
	if err := core.SaveIssueOpsBenchmarkRun(stateDir, baseline); err != nil {
		t.Fatal(err)
	}
	if err := core.SaveIssueOpsBenchmarkRun(stateDir, candidate); err != nil {
		t.Fatal(err)
	}
	candidatePath := writeIssueOpsCandidateForCLITest(t, core.IssueOpsAutoresearchCandidate{
		ID:               "issueops-benchmark-cli",
		Hypothesis:       "Benchmark CLI text output should be stable.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"cmd/harness/**"},
	})

	compareOut := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "compare", "--baseline", "baseline", "--candidate", "candidate"})
	})
	for _, want := range []string{"improved=true", "average_delta=15.00", "minimum_delta=15.00"} {
		if !strings.Contains(compareOut, want) {
			t.Fatalf("benchmark compare text missing %q:\n%s", want, compareOut)
		}
	}

	gateOut := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "gate", "--baseline", "baseline", "--candidate", "candidate", "--candidate-file", candidatePath, "--changed-path", "cmd/harness/issueops.go"})
	})
	for _, want := range []string{"keep_candidate=true", "ok=true", "discard_reasons=0"} {
		if !strings.Contains(gateOut, want) {
			t.Fatalf("benchmark gate text missing %q:\n%s", want, gateOut)
		}
	}
}

func TestRunIssueOpsBenchmarkUsageAndErrorBranches(t *testing.T) {
	usage := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "help"})
	})
	if !strings.Contains(usage, "Usage: agent-harness issueops benchmark run|compare|gate|reliability [--json]") {
		t.Fatalf("benchmark usage missing command summary:\n%s", usage)
	}
	if err := runIssueOps([]string{"benchmark", "unknown"}); err == nil || !strings.Contains(err.Error(), "unknown issueops benchmark subcommand") {
		t.Fatalf("benchmark unknown subcommand error = %v", err)
	}
	if err := runIssueOps([]string{"benchmark", "run", "--fixtures", filepath.Join("..", "..", "..", "testdata", "issueops", "fixtures"), "--judge", "bogus"}); err == nil || !strings.Contains(err.Error(), `unsupported issueops benchmark judge "bogus"`) {
		t.Fatalf("benchmark unsupported judge error = %v", err)
	}
	if err := runIssueOps([]string{"benchmark", "gate", "--baseline", "missing", "--candidate", "missing"}); err == nil || !strings.Contains(err.Error(), "candidate-file is required") {
		t.Fatalf("benchmark missing candidate-file error = %v", err)
	}

	badCandidate := filepath.Join(t.TempDir(), "candidate.json")
	if err := os.WriteFile(badCandidate, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runIssueOps([]string{"benchmark", "gate", "--baseline", "missing", "--candidate", "missing", "--candidate-file", badCandidate}); err == nil || !strings.Contains(err.Error(), "parse candidate file") {
		t.Fatalf("benchmark invalid candidate-file error = %v", err)
	}
}

func benchmarkRunForCLITest(id string, score float64, evidence string) core.IssueOpsBenchmarkRunResult {
	return core.FinalizeIssueOpsBenchmarkRunResult(core.IssueOpsBenchmarkRunResult{
		ID: id,
		Scores: []core.IssueOpsBenchmarkScore{{
			OK:           true,
			FixtureID:    "fixture",
			AverageScore: score,
			MinimumScore: score,
			DimensionScores: []core.IssueOpsDimensionScore{
				{Dimension: "issue_quality", Score: score, Evidence: evidence},
			},
			Passed: true,
		}},
	})
}
