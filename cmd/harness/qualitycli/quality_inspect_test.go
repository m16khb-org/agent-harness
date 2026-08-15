package qualitycli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

func TestParseCoverageFindsBarePackageOutput(t *testing.T) {
	output := "agent-harness/internal/domain/issueopsinventory\t\tcoverage: 0.0% of statements\n"

	got := parseCoveragePackages(output, 60)

	if len(got) != 1 || got[0].Package != "agent-harness/internal/domain/issueopsinventory" || got[0].Coverage != 0 {
		t.Fatalf("parseCoveragePackages() = %+v", got)
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
	const coverageOutput = "agent-harness/sample coverage: 55.0% of statements\n"
	executeGoTestCoverage = func(context.Context, string) (string, error) {
		calls++
		return coverageOutput, nil
	}
	cacheDir := t.TempDir()
	coverageCacheBase = func() (string, error) {
		return cacheDir, nil
	}

	first, firstErr := runGoTestCoverage(root)
	second, secondErr := runGoTestCoverage(root)
	if firstErr != nil || secondErr != nil || first != coverageOutput || second != first || calls != 1 {
		t.Fatalf("first=%q/%v second=%q/%v calls=%d", first, firstErr, second, secondErr, calls)
	}
	cachePath, cacheErr := coverageCachePath(root)
	if cacheErr != nil {
		t.Fatal(cacheErr)
	}
	cached, readErr := os.ReadFile(cachePath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if bytes.Contains(cached, []byte(`"output"`)) || !bytes.Contains(cached, []byte(`"packages"`)) {
		t.Fatalf("coverage cache must persist parsed package data only: %s", cached)
	}
	if err := os.WriteFile(source, []byte("package sample\n\nconst changed = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runGoTestCoverage(root); err != nil || calls != 2 {
		t.Fatalf("changed source did not invalidate coverage cache: calls=%d err=%v", calls, err)
	}
}

func TestBoundedQualityBufferCapsCapturedBytes(t *testing.T) {
	buffer := newBoundedQualityBuffer(4)
	if written, err := buffer.Write([]byte("abcdef")); err != nil || written != 6 {
		t.Fatalf("write = %d, %v", written, err)
	}
	if got := buffer.String(); got != "abcd" || !buffer.Truncated() {
		t.Fatalf("bounded output = %q truncated=%v", got, buffer.Truncated())
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
		"quality status: collection=ok health=needs_attention gate=report_only",
		"self-augment open: 2",
		"self-verify open: 1",
		"low coverage packages: 1",
		"pioneer coverage: benchmark=12/12 reproduction=12/12",
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

func TestRunInspectWithDepsReturnsBlockedErrorAfterCollectionFailure(t *testing.T) {
	deps := qualityDepsForTest("now")
	deps.Coverage = func(string) (string, error) {
		return "", errors.New("coverage unavailable")
	}

	err := RunInspectWithDeps([]string{"--repo", t.TempDir(), "--json"}, deps)

	if !errors.Is(err, ErrQualityGateBlocked) {
		t.Fatalf("error = %v, want ErrQualityGateBlocked", err)
	}
}

func TestInspectDepsWithDefaultsFillsMissingDependencies(t *testing.T) {
	deps := (InspectDeps{}).withDefaults()
	if deps.Now == nil || deps.Coverage == nil || deps.SelfAugmentOpenCount == nil || deps.SelfVerifyOpenCount == nil || deps.PioneerCoverage == nil {
		t.Fatalf("withDefaults left nil dependency: %+v", deps)
	}
}

func TestCollectPioneerCoverageKeepsNamesakeDenominatorSeparate(t *testing.T) {
	root := t.TempDir()
	writeQualityTestFile(t, filepath.Join(root, "testdata", "issueops", "fixtures", "pioneer-turing.json"), "{}")
	writeQualityTestFile(t, filepath.Join(root, "testdata", "issueops", "fixtures", "pioneer-issueops.json"), "{}")
	writeQualityTestFile(t, filepath.Join(root, "testdata", "pioneer-holdouts", "turing", "TASK.md"), "task")

	coverage, err := collectPioneerCoverage(root)

	if err != nil {
		t.Fatal(err)
	}
	if coverage.Expected != 12 || coverage.BenchmarkObserved != 1 || coverage.ReproductionObserved != 1 {
		t.Fatalf("coverage = %+v", coverage)
	}
	if slices.Contains(coverage.BenchmarkMissing, "turing") || len(coverage.BenchmarkMissing) != 11 {
		t.Fatalf("benchmark missing = %v", coverage.BenchmarkMissing)
	}
}

func TestCollectPioneerCoverageRejectsSymlinkedFixtures(t *testing.T) {
	root := t.TempDir()
	external := filepath.Join(t.TempDir(), "fixture.json")
	writeQualityTestFile(t, external, "{}")
	fixture := filepath.Join(root, "testdata", "issueops", "fixtures", "pioneer-turing.json")
	if err := os.MkdirAll(filepath.Dir(fixture), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, fixture); err != nil {
		t.Fatal(err)
	}

	coverage, err := collectPioneerCoverage(root)

	if err != nil {
		t.Fatal(err)
	}
	if coverage.BenchmarkObserved != 0 || !slices.Contains(coverage.BenchmarkMissing, "turing") {
		t.Fatalf("symlinked fixture counted as coverage: %+v", coverage)
	}
}

func TestCollectPioneerEvaluationManifestValidatesThreeAxesAndHashes(t *testing.T) {
	root := t.TempDir()
	skill := "sample-skill"
	files := map[string]string{
		"primary":     "TASK.md",
		"boundary":    "BOUNDARY.md",
		"operational": "OPERATIONAL.md",
	}
	cases := make([]map[string]any, 0, len(files))
	runs := make([]map[string]any, 0, len(files))
	evidencePath := filepath.ToSlash(filepath.Join("evidence-records", skill+".json"))
	evidenceBody := []byte(`{"schema_version":1,"skill":"sample-skill"}`)
	writeQualityTestFile(
		t,
		filepath.Join(root, "testdata", "pioneer-holdouts", evidencePath),
		string(evidenceBody),
	)
	evidenceDigest := sha256.Sum256(evidenceBody)
	evidenceSHA256 := fmt.Sprintf("%x", evidenceDigest[:])
	for axis, filename := range files {
		body := []byte("# " + axis + "\n")
		path := filepath.Join(root, "testdata", "pioneer-holdouts", skill, filename)
		writeQualityTestFile(t, path, string(body))
		digest := sha256.Sum256(body)
		taskID := "st-" + axis
		cases = append(cases, map[string]any{
			"skill":                    skill,
			"axis":                     axis,
			"case_path":                filepath.ToSlash(filepath.Join(skill, filename)),
			"case_sha256":              fmt.Sprintf("%x", digest[:]),
			"task_id":                  taskID,
			"verdict":                  "pass",
			"hidden_holdout":           false,
			"evidence_path":            evidencePath,
			"evidence_sha256":          evidenceSHA256,
			"deterministic_assertions": []string{"fixture-hash", "semantic-contract"},
			"semantic_grade":           "meets_case_contract",
			"host_capability":          "available",
		})
		runs = append(runs, map[string]any{
			"task_id": taskID, "axes": []string{axis}, "status": "completed",
			"host": "omo", "model": "test-model",
			"receipt_sha256":   fmt.Sprintf("%x", sha256.Sum256([]byte(taskID))),
			"receipt_bytes":    len(taskID),
			"execution_method": "fresh_context_child_task",
			"artifact_kind":    "bounded_final_response_receipt",
			"evidence_path":    evidencePath,
			"evidence_sha256":  evidenceSHA256,
		})
	}
	manifest, err := json.Marshal(map[string]any{
		"schema_version": 2,
		"provenance": map[string]any{
			"host": "omo", "execution_count": 3, "case_count": 3,
			"receipt_algorithm": "sha256", "receipt_source": "test receipt",
			"answers_committed": false, "hidden_holdouts": false,
			"evidence_record_count": 1, "evidence_record_algorithm": "sha256",
			"semantic_grading": "case-contract assertions",
		},
		"runs":  runs,
		"cases": cases,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeQualityTestFile(
		t,
		filepath.Join(root, "testdata", "pioneer-holdouts", "evaluation-manifest.json"),
		string(manifest),
	)

	counts, err := collectPioneerEvaluationManifest(root, []string{skill})
	if err != nil {
		t.Fatal(err)
	}
	if counts.observed != 3 || counts.passed != 3 || counts.blocked != 0 || counts.hidden != 0 {
		t.Fatalf("counts = %+v", counts)
	}

	cases[0]["case_sha256"] = strings.Repeat("0", 64)
	manifest, err = json.Marshal(map[string]any{
		"schema_version": 2,
		"provenance": map[string]any{
			"host": "omo", "execution_count": 3, "case_count": 3,
			"receipt_algorithm": "sha256", "receipt_source": "test receipt",
			"answers_committed": false, "hidden_holdouts": false,
			"evidence_record_count": 1, "evidence_record_algorithm": "sha256",
			"semantic_grading": "case-contract assertions",
		},
		"runs": runs, "cases": cases,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeQualityTestFile(
		t,
		filepath.Join(root, "testdata", "pioneer-holdouts", "evaluation-manifest.json"),
		string(manifest),
	)
	if _, err := collectPioneerEvaluationManifest(root, []string{skill}); err == nil {
		t.Fatal("hash mismatch must fail closed")
	}
}

func TestCoverageCommandTimeoutIncludesColdCacheHeadroom(t *testing.T) {
	if coverageCommandTimeout < 10*time.Minute {
		t.Fatalf("coverage command timeout = %s, want at least 10m", coverageCommandTimeout)
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
		CodeSNR: func(string) (SNRResult, error) {
			started <- "snr"
			<-release
			return SNRResult{}, nil
		},
	}
	result := make(chan InspectResult, 1)
	go func() {
		result <- Inspect(t.TempDir(), deps)
	}()

	<-started
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("independent quality signals did not start concurrently")
	}
	close(release)
	if got := <-result; !got.OK {
		t.Fatalf("inspect result = %+v", got)
	}
}

func TestInspectSeparatesCollectionHealthAndGateStatus(t *testing.T) {
	deps := qualityDepsForTest("now")
	deps.PioneerCoverage = func(string) (PioneerCoverage, error) {
		return PioneerCoverage{
			Expected:             12,
			BenchmarkObserved:    9,
			BenchmarkMissing:     []string{"boehm", "brooks", "engelbart"},
			ReproductionObserved: 9,
			ReproductionMissing:  []string{"boehm", "brooks", "engelbart"},
		}, nil
	}

	result := Inspect(t.TempDir(), deps)

	if !result.OK || result.CollectionStatus != CollectionStatusOK {
		t.Fatalf("collection status = ok=%v status=%q", result.OK, result.CollectionStatus)
	}
	if result.HealthStatus != HealthStatusNeedsAttention || result.GateStatus != GateStatusReportOnly {
		t.Fatalf("quality statuses = health=%q gate=%q", result.HealthStatus, result.GateStatus)
	}
	if result.PioneerCoverage.Expected != 12 || len(result.PioneerCoverage.BenchmarkMissing) != 3 {
		t.Fatalf("pioneer coverage = %+v", result.PioneerCoverage)
	}
	finding := findQualityFinding(result.Findings, "pioneer-skill-coverage")
	if finding == nil || finding.Blocking || len(finding.Evidence) == 0 || finding.VerifyCommand == "" {
		t.Fatalf("pioneer finding = %+v", finding)
	}
}

func TestInspectCollectorErrorBlocksWithoutClaimingHealth(t *testing.T) {
	deps := qualityDepsForTest("now")
	deps.Coverage = func(string) (string, error) {
		return "", errors.New("coverage failed")
	}
	deps.PioneerCoverage = func(string) (PioneerCoverage, error) {
		return PioneerCoverage{Expected: 12}, errors.New("pioneer scan failed")
	}

	result := Inspect(t.TempDir(), deps)

	if result.OK || result.CollectionStatus != CollectionStatusError {
		t.Fatalf("collection status = ok=%v status=%q", result.OK, result.CollectionStatus)
	}
	if result.HealthStatus != HealthStatusUnknown || result.GateStatus != GateStatusBlock {
		t.Fatalf("quality statuses = health=%q gate=%q", result.HealthStatus, result.GateStatus)
	}
	finding := findQualityFinding(result.Findings, "quality-collector-error")
	if finding == nil || !finding.Blocking || finding.Severity != "p0" {
		t.Fatalf("collector finding = %+v", finding)
	}
}

func TestInspectMarksFailedSNRCollectorSignalAsError(t *testing.T) {
	deps := qualityDepsForTest("now")
	deps.CodeSNR = func(string) (SNRResult, error) {
		return SNRResult{}, errors.New("walk failed")
	}

	result := Inspect(t.TempDir(), deps)

	if result.CollectionStatus != CollectionStatusError || result.GateStatus != GateStatusBlock {
		t.Fatalf("quality statuses = %+v", result)
	}
	for _, signal := range result.Signals {
		if signal.ID == "code-snr" && signal.Status != "error" {
			t.Fatalf("code-snr signal = %+v", signal)
		}
	}
}

func TestInspectP0AuditItemBlocksGate(t *testing.T) {
	root := t.TempDir()
	writeQualityTestFile(t, filepath.Join(root, ".agent-harness", "PROJECT_AUDIT.md"), `
| ID | Area | Title | Priority | Size |
| --- | --- | --- | --- | --- |
| P0-1 | Runtime | Active release blocker | P0 | Small |
`)

	result := Inspect(root, qualityDepsForTest("now"))

	if result.GateStatus != GateStatusBlock {
		t.Fatalf("gate status = %q, want block", result.GateStatus)
	}
	finding := findQualityFinding(result.Findings, "project-audit-items")
	if finding == nil || !finding.Blocking || finding.Severity != "p0" {
		t.Fatalf("audit finding = %+v", finding)
	}
}

func TestRunInspectBlocksOnBaselineWriteFailureAndRegression(t *testing.T) {
	tests := []struct {
		name string
		deps func() InspectDeps
		args []string
	}{
		{
			name: "write failure",
			deps: func() InspectDeps {
				deps := qualityDepsForTest("now")
				deps.SaveSNRBaseline = func(string, float64) error { return errors.New("store unavailable") }
				return deps
			},
			args: []string{"--save-baseline", "--json"},
		},
		{
			name: "regression",
			deps: func() InspectDeps {
				deps := qualityDepsForTest("now")
				deps.CodeSNR = func(string) (SNRResult, error) { return SNRResult{Ratio: 0.50}, nil }
				deps.ReadSNRBaseline = func(string) (float64, bool, error) { return 0.75, true, nil }
				return deps
			},
			args: []string{"--trend", "--json"},
		},
		{
			name: "baseline read failure",
			deps: func() InspectDeps {
				deps := qualityDepsForTest("now")
				deps.ReadSNRBaseline = func(string) (float64, bool, error) {
					return 0, false, errors.New("corrupt baseline")
				}
				return deps
			},
			args: []string{"--trend", "--json"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"--repo", t.TempDir()}, test.args...)
			_ = captureQualityStdout(t, func() error {
				err := RunInspectWithDeps(args, test.deps())
				if !errors.Is(err, ErrQualityGateBlocked) {
					t.Fatalf("error = %v, want ErrQualityGateBlocked", err)
				}
				return nil
			})
		})
	}
}

func TestRunInspectDoesNotOverwriteBaselineWhenSNRCollectionFails(t *testing.T) {
	deps := qualityDepsForTest("now")
	deps.CodeSNR = func(string) (SNRResult, error) {
		return SNRResult{}, errors.New("walk failed")
	}
	writes := 0
	deps.SaveSNRBaseline = func(string, float64) error {
		writes++
		return nil
	}

	output := captureQualityStdout(t, func() error {
		err := RunInspectWithDeps(
			[]string{"--repo", t.TempDir(), "--save-baseline", "--json"},
			deps,
		)
		if !errors.Is(err, ErrQualityGateBlocked) {
			t.Fatalf("error = %v, want ErrQualityGateBlocked", err)
		}
		return nil
	})

	if writes != 0 || !strings.Contains(output, "code-snr signal is unavailable") {
		t.Fatalf("writes=%d output=%s", writes, output)
	}
}

func findQualityFinding(findings []Finding, id string) *Finding {
	for index := range findings {
		if findings[index].ID == id {
			return &findings[index]
		}
	}
	return nil
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
		PioneerCoverage: func(string) (PioneerCoverage, error) {
			return PioneerCoverage{Expected: 12, BenchmarkObserved: 12, ReproductionObserved: 12}, nil
		},
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
