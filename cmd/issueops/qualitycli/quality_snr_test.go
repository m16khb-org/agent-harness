package qualitycli

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestComputeCodeSNR(t *testing.T) {
	dir := t.TempDir()
	// signal: "package x", "func F() int {", "return 1" (3)
	// noise: blank, "// comment", "}" structural-only (3)
	src := "package x\n\n// comment\nfunc F() int {\n\treturn 1\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	// _test.go and testdata/ must be excluded from the measurement.
	if err := os.WriteFile(filepath.Join(dir, "x_test.go"), []byte("package x\nfunc T() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snr, err := computeCodeSNR(dir)
	if err != nil {
		t.Fatal(err)
	}
	if snr.SignalLines != 3 || snr.NoiseLines != 3 || snr.TotalLines != 6 {
		t.Fatalf("unexpected SNR counts: %+v", snr)
	}
	if snr.Ratio != 0.5 {
		t.Fatalf("ratio = %v, want 0.5", snr.Ratio)
	}
}

func TestSNRBaselineRoundTrip(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	configureTestStateStore(t)
	repository := t.TempDir()
	if _, ok, err := readSNRBaseline(repository); ok || err != nil {
		t.Fatalf("expected no baseline before any save, ok=%v err=%v", ok, err)
	}
	if err := saveSNRBaseline(repository, 0.7143); err != nil {
		t.Fatalf("saveSNRBaseline: %v", err)
	}
	v, ok, err := readSNRBaseline(repository)
	if err != nil || !ok || v != 0.7143 {
		t.Fatalf("readSNRBaseline = %v, %v, %v; want 0.7143, true, nil", v, ok, err)
	}
}

func TestSNRBaselineRejectsNonFiniteAndCrossRepositoryValues(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	configureTestStateStore(t)
	first := t.TempDir()
	second := t.TempDir()
	if err := saveSNRBaseline(first, math.NaN()); err == nil {
		t.Fatal("NaN baseline must be rejected")
	}
	if err := saveSNRBaseline(first, 0.75); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := readSNRBaseline(second); ok || err != nil {
		t.Fatalf("cross-repository baseline leaked: ok=%v err=%v", ok, err)
	}
	repository, key, err := snrBaselineIdentity(first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hostDeps.StateWrite(
		key,
		`{"schema_version":1,"repository":"`+repository+`","ratio":NaN}`,
	); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := readSNRBaseline(first); ok || err == nil {
		t.Fatalf("corrupt baseline must fail closed: ok=%v err=%v", ok, err)
	}
}
