package qualitycli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCoverageFindsPackagesBelowThreshold(t *testing.T) {
	output := "ok  \tagent-harness/internal/core/commandguard\t0.011s\tcoverage: 54.3% of statements\n" +
		"ok  \tagent-harness/internal/core/state\t0.012s\tcoverage: 81.0% of statements\n" +
		"?   \tagent-harness/internal/core/empty\t[no test files]\n"

	got := parseCoveragePackages(output, 60)
	if len(got) != 1 {
		t.Fatalf("low coverage packages=%#v, want one", got)
	}
	if got[0].Package != "agent-harness/internal/core/commandguard" || got[0].Coverage != 54.3 {
		t.Fatalf("unexpected low coverage package: %+v", got[0])
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
			return "ok  \tagent-harness/internal/core/commandguard\t0.011s\tcoverage: 54.3% of statements\n", nil
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
	if len(result.Candidates) < 10 {
		t.Fatalf("expected at least 10 quality candidates, got %d", len(result.Candidates))
	}
	for _, candidate := range result.Candidates {
		if candidate.ID == "" || candidate.Status != "open" || candidate.Score <= 0 || len(candidate.VerifyWith) == 0 || len(candidate.Evidence) == 0 {
			t.Fatalf("candidate missing required quality fields: %+v", candidate)
		}
	}
}
