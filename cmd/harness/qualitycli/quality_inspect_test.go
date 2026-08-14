package qualitycli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/testsupport"
)

func TestParseCoverageFindsPackagesBelowThreshold(t *testing.T) {
	output := "ok  \tagent-harness/internal/adapter/commandguard\t0.011s\tcoverage: 54.3% of statements\n" +
		"ok  \tagent-harness/internal/adapter/outbound/state\t0.012s\tcoverage: 81.0% of statements\n" +
		"?   \tagent-harness/internal/adapter/empty\t[no test files]\n"

	got := parseCoveragePackages(output, 60)
	if len(got) != 1 {
		t.Fatalf("low coverage packages=%#v, want one", got)
	}
	if got[0].Package != "agent-harness/internal/adapter/commandguard" || got[0].Coverage != 54.3 {
		t.Fatalf("unexpected low coverage package: %+v", got[0])
	}
}
func TestRunGoTestCoverageCachesExactRepositoryFingerprint(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "sample.go")
	if err := os.WriteFile(source, []byte("package sample\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init"},
		{"add", "sample.go"},
		{"-c", "user.name=coverage-test", "-c", "user.email=coverage@example.invalid", "commit", "-m", "baseline"},
	} {
		command := exec.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, output, err)
		}
	}
	previousExecute := executeGoTestCoverage
	previousCacheBase := coverageCacheBase
	defer func() {
		executeGoTestCoverage = previousExecute
		coverageCacheBase = previousCacheBase
	}()
	calls := 0
	executeGoTestCoverage = func(context.Context, string) (string, error) {
		calls++
		return "coverage-run", nil
	}
	cacheDir := t.TempDir()
	coverageCacheBase = func() (string, error) {
		return cacheDir, nil
	}

	first, firstErr := runGoTestCoverage(root)
	second, secondErr := runGoTestCoverage(root)
	if firstErr != nil || secondErr != nil || first != "coverage-run" || second != first || calls != 1 {
		t.Fatalf("first=%q/%v second=%q/%v calls=%d", first, firstErr, second, secondErr, calls)
	}
	if err := os.WriteFile(source, []byte("package sample\n\nconst changed = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runGoTestCoverage(root); err != nil || calls != 2 {
		t.Fatalf("changed source did not invalidate coverage cache: calls=%d err=%v", calls, err)
	}
}
func TestRunRoutesQualityCommands(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing subcommand", wantErr: "usage: quality inspect"},
		{name: "unknown subcommand", args: []string{"unknown"}, wantErr: `unknown quality command "unknown"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Run(tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error=%v, want substring %q", err, tc.wantErr)
			}
		})
	}

	out := captureQualityStdout(t, func() error { return Run([]string{"help"}) })
	if !strings.Contains(out, "agent-harness quality inspect") {
		t.Fatalf("help output=%q", out)
	}
}
func TestRunInspectWithDepsPrintsTextAndJSON(t *testing.T) {
	root := t.TempDir()
	deps := qualityDepsForTest("2026-06-13T00:00:00Z")
	textOut := captureQualityStdout(t, func() error {
		return RunInspectWithDeps([]string{"--repo", root}, deps)
	})
	for _, want := range []string{
		"quality inspect: ok=true repo=" + root,
		"self-augment open: 2",
		"self-verify open: 1",
		"low coverage packages: 1",
	} {
		if !strings.Contains(textOut, want) {
			t.Fatalf("text output missing %q:\n%s", want, textOut)
		}
	}

	jsonOut := captureQualityStdout(t, func() error {
		return RunInspectWithDeps([]string{"--json", root}, deps)
	})
	var result InspectResult
	if err := json.Unmarshal([]byte(jsonOut), &result); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, jsonOut)
	}
	if result.GeneratedAt != "2026-06-13T00:00:00Z" || result.Summary.SelfAugmentOpenCandidates != 2 || result.Summary.SelfVerifyOpenCandidates != 1 {
		t.Fatalf("unexpected JSON result: %+v", result)
	}
}

func TestRunInspectWithDepsValidatesFlags(t *testing.T) {
	err := RunInspectWithDeps([]string{"--unknown"}, qualityDepsForTest("now"))
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("error=%v, want flag error", err)
	}
}

func TestInspectDepsWithDefaultsFillsMissingDependencies(t *testing.T) {
	deps := (InspectDeps{}).withDefaults()
	if deps.Now == nil || deps.Coverage == nil || deps.SelfAugmentOpenCount == nil || deps.SelfVerifyOpenCount == nil {
		t.Fatalf("withDefaults left nil dependency: %+v", deps)
	}
}

func TestInspectRunsIndependentSignalsConcurrently(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	deps := InspectDeps{
		Now: func() string { return "now" },
		Coverage: func(string) (string, error) {
			started <- "coverage"
			<-release
			return "", nil
		},
		SelfAugmentOpenCount: func(string) (int, error) { return 0, nil },
		SelfVerifyOpenCount:  func(string) (int, error) { return 0, nil },
		Candidates:           func(string) []QualityCandidate { return nil },
		CodeSNR: func(string) SNRResult {
			started <- "snr"
			<-release
			return SNRResult{}
		},
	}
	result := make(chan InspectResult, 1)
	go func() {
		result <- Inspect(t.TempDir(), deps)
	}()

	<-started
	select {
	case <-started:
	case <-time.After(250 * time.Millisecond):
		close(release)
		t.Fatal("independent quality signals did not start concurrently")
	}
	close(release)
	if got := <-result; !got.OK {
		t.Fatalf("inspect result = %+v", got)
	}
}

func writeQualityTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func qualityDepsForTest(now string) InspectDeps {
	return InspectDeps{
		Now: func() string { return now },
		Coverage: func(string) (string, error) {
			return "ok  \tagent-harness/internal/adapter/example\t0.011s\tcoverage: 54.3% of statements\n", nil
		},
		SelfAugmentOpenCount: func(string) (int, error) { return 2, nil },
		SelfVerifyOpenCount:  func(string) (int, error) { return 1, nil },
	}
}

func captureQualityStdout(t *testing.T, fn func() error) string {
	t.Helper()
	return testsupport.CaptureStdout(t, fn)
}

func TestInspectQualityUsesInjectedSignals(t *testing.T) {
	root := t.TempDir()
	writeQualityTestFile(t, filepath.Join(root, "sample.go"), `package sample

func branchy(v int) int {
	if v > 0 {
		v++
	}
	if v > 1 {
		v++
	}
	if v > 2 {
		v++
	}
	if v > 3 {
		v++
	}
	if v > 4 {
		v++
	}
	if v > 5 {
		v++
	}
	if v > 6 {
		v++
	}
	return v
}
`)
	writeQualityTestFile(t, filepath.Join(root, ".agent-harness", "PROJECT_AUDIT.md"), `
| D1 | Daemon | No connection limit | P1 | Small |
| X1 | Docs | Low priority docs polish | P3 | Small |
`)

	result := Inspect(root, InspectDeps{
		Now: func() string { return "2026-06-13T00:00:00Z" },
		Coverage: func(string) (string, error) {
			return "ok  \tagent-harness/internal/adapter/commandguard\t0.011s\tcoverage: 54.3% of statements\n", nil
		},
		SelfAugmentOpenCount: func(string) (int, error) { return 10, nil },
		SelfVerifyOpenCount:  func(string) (int, error) { return 0, nil },
	})

	if !result.OK {
		t.Fatalf("quality inspect not ok: %+v", result)
	}
	if result.GeneratedAt != "2026-06-13T00:00:00Z" || result.HarnessRoot != root {
		t.Fatalf("identity fields mismatch: %+v", result)
	}
	if result.Summary.SelfAugmentOpenCandidates != 10 || result.Summary.LowCoveragePackages != 1 || result.Summary.BranchCandidateFunctions != 1 || result.Summary.AuditP1P2Items != 1 {
		t.Fatalf("summary did not reflect injected/scanned signals: %+v", result.Summary)
	}
	if len(result.Signals) == 0 {
		t.Fatalf("expected quality signals")
	}
	if len(result.Candidates) == 0 {
		t.Fatalf("expected quality candidates")
	}
	for _, candidate := range result.Candidates {
		if candidate.ID == "daemon-connection-limit" || candidate.ID == "worker-stuck-running-detection" || candidate.ID == "state-write-locking" || candidate.ID == "draftwiki-stale-lock" {
			t.Fatalf("resolved audit candidate should not be listed by quality inspect: %+v", candidate)
		}
		if candidate.ID == "" || candidate.Status == "" || len(candidate.VerifyWith) == 0 || len(candidate.Evidence) == 0 {
			t.Fatalf("candidate missing required quality fields: %+v", candidate)
		}
	}
}

func TestInspectQualityCandidatesCanUseProjectedStatuses(t *testing.T) {
	root := t.TempDir()

	result := Inspect(root, InspectDeps{
		Now: func() string { return "2026-06-13T00:00:00Z" },
		Coverage: func(string) (string, error) {
			return "ok  \tagent-harness/internal/adapter/commandguard\t0.011s\tcoverage: 74.3% of statements\n", nil
		},
		SelfAugmentOpenCount: func(string) (int, error) { return 8, nil },
		SelfVerifyOpenCount:  func(string) (int, error) { return 0, nil },
		Candidates: func(string) []QualityCandidate {
			return []QualityCandidate{
				{ID: "quality-signal-harvester", Status: "already_satisfied", Score: 0, VerifyWith: []string{"agent-harness quality inspect --json"}, Evidence: []string{"quality inspect CLI"}},
				{ID: "coverage-issueops-linking", Status: "open", Score: 77.4, VerifyWith: []string{"go test ./internal/adapter/issueops/linking -count=1"}, Evidence: []string{"PROJECT_AUDIT"}},
			}
		},
	})

	if got := len(result.Candidates); got != 2 {
		t.Fatalf("candidate count=%d, want injected candidate count", got)
	}
	if result.Candidates[0].ID != "quality-signal-harvester" || result.Candidates[0].Status != "already_satisfied" || result.Candidates[0].Score != 0 {
		t.Fatalf("quality candidate status was not preserved: %+v", result.Candidates[0])
	}
	if result.Summary.CandidateCount != 2 || result.Summary.SelfAugmentOpenCandidates != 8 {
		t.Fatalf("summary did not match injected candidate/open counts: %+v", result.Summary)
	}
}
