package qualitycli

import (
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
	snr := computeCodeSNR(dir)
	if snr.SignalLines != 3 || snr.NoiseLines != 3 || snr.TotalLines != 6 {
		t.Fatalf("unexpected SNR counts: %+v", snr)
	}
	if snr.Ratio != 0.5 {
		t.Fatalf("ratio = %v, want 0.5", snr.Ratio)
	}
}

func TestSNRBaselineRoundTrip(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if _, ok := readSNRBaseline(); ok {
		t.Fatal("expected no baseline before any save")
	}
	if err := saveSNRBaseline(0.7143); err != nil {
		t.Fatalf("saveSNRBaseline: %v", err)
	}
	v, ok := readSNRBaseline()
	if !ok || v != 0.7143 {
		t.Fatalf("readSNRBaseline = %v, %v; want 0.7143, true", v, ok)
	}
}
