package issueops

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

func TestLegacyResetActivationV1RequiresFullAtomicSeal(t *testing.T) {
	stateDir := t.TempDir()
	harnessRoot := t.TempDir()
	binDir := filepath.Join(harnessRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(binDir, "agent-harness")
	stage := filepath.Join(binDir, ".agent-harness.activate-test")
	if err := os.WriteFile(target, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stage, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	candidate, err := resetLegacyBinaryIdentityFromPathV1(stage, "test")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 22, 1, 2, 3, 0, time.UTC)
	deps := resetLegacyActivationDepsV1{
		Now: func() time.Time { return now },
		ActiveBinary: func() (resetLegacyV1BinaryIdentity, error) {
			return candidate, nil
		},
		SmokeDigest: func() (string, error) { return strings.Repeat("b", 64), nil },
	}
	begin, err := beginLegacyResetActivationV1(stateDir, LegacyResetActivationBeginRequestV1{
		TargetSchema: 1, HarnessRoot: harnessRoot, TargetBinary: target,
	}, deps)
	if err != nil || !begin.Pending || begin.Sealed {
		t.Fatalf("begin=%#v err=%v", begin, err)
	}
	control, err := sqlstore.Open(filepath.Join(stateDir, issueOpsResetV1Directory))
	if err != nil {
		t.Fatal(err)
	}
	oldActive, err := resetLegacyBinaryIdentityFromPathV1(target, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := requireLegacyResetActivationV1(control, stateDir, 1, oldActive); err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("pending activation accepted: %v", err)
	}

	if err := os.Rename(stage, target); err != nil {
		t.Fatal(err)
	}
	active, err := resetLegacyBinaryIdentityFromPathV1(target, "test")
	if err != nil {
		t.Fatal(err)
	}
	deps.ActiveBinary = func() (resetLegacyV1BinaryIdentity, error) { return active, nil }
	evidence := writeLegacyResetActivationEvidenceFixtureV1(t)

	partial := evidence[:len(evidence)-1]
	if _, err := sealLegacyResetActivationV1(stateDir, LegacyResetActivationSealRequestV1{
		TargetSchema: 1, HarnessRoot: harnessRoot, TargetBinary: target,
		CatalogSHA256: strings.Repeat("a", 64), Evidence: partial,
	}, deps); err == nil || !strings.Contains(err.Error(), "exactly four") {
		t.Fatalf("partial activation evidence accepted: %v", err)
	}
	if err := requireLegacyResetActivationV1(control, stateDir, 1, active); err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("failed seal cleared pending marker: %v", err)
	}
	if err := os.WriteFile(evidence[0].Path, []byte("changed-after-readback"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := sealLegacyResetActivationV1(stateDir, LegacyResetActivationSealRequestV1{
		TargetSchema: 1, HarnessRoot: harnessRoot, TargetBinary: target,
		CatalogSHA256: strings.Repeat("a", 64), Evidence: evidence,
	}, deps); err == nil || !strings.Contains(err.Error(), "changed after semantic readback") {
		t.Fatalf("readback TOCTOU was accepted: %v", err)
	}
	if err := os.WriteFile(evidence[0].Path, []byte("codex:mcp"), 0o600); err != nil {
		t.Fatal(err)
	}

	sealed, err := sealLegacyResetActivationV1(stateDir, LegacyResetActivationSealRequestV1{
		TargetSchema: 1, HarnessRoot: harnessRoot, TargetBinary: target,
		CatalogSHA256: strings.Repeat("a", 64), Evidence: evidence,
	}, deps)
	if err != nil || sealed.Pending || !sealed.Sealed {
		t.Fatalf("seal=%#v err=%v", sealed, err)
	}
	if err := requireLegacyResetActivationV1(control, stateDir, 1, active); err != nil {
		t.Fatalf("sealed activation rejected: %v", err)
	}
	status, err := StatusLegacyResetV1(stateDir, 1)
	if err != nil || status.Activation == nil || !status.Activation.Sealed || status.Activation.Pending {
		t.Fatalf("activation status=%#v err=%v", status.Activation, err)
	}

	if err := os.WriteFile(evidence[0].Path, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := requireLegacyResetActivationV1(control, stateDir, 1, active); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("changed host evidence accepted: %v", err)
	}
}

func TestLegacyResetActivationV1CrashBeforeSealLeavesPending(t *testing.T) {
	stateDir, harnessRoot, target, stage := newLegacyResetActivationBinaryFixtureV1(t)
	candidate, err := resetLegacyBinaryIdentityFromPathV1(stage, "test")
	if err != nil {
		t.Fatal(err)
	}
	deps := resetLegacyActivationDepsV1{
		Now:          time.Now,
		ActiveBinary: func() (resetLegacyV1BinaryIdentity, error) { return candidate, nil },
		SmokeDigest:  func() (string, error) { return strings.Repeat("b", 64), nil },
	}
	if _, err := beginLegacyResetActivationV1(stateDir, LegacyResetActivationBeginRequestV1{
		TargetSchema: 1, HarnessRoot: harnessRoot, TargetBinary: target,
	}, deps); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(stage, target); err != nil {
		t.Fatal(err)
	}
	active, err := resetLegacyBinaryIdentityFromPathV1(target, "test")
	if err != nil {
		t.Fatal(err)
	}
	crash := errors.New("crash after readback")
	deps.ActiveBinary = func() (resetLegacyV1BinaryIdentity, error) { return active, nil }
	deps.AfterStep = func(step string) error {
		if step == "activation_verified" {
			return crash
		}
		return nil
	}
	if _, err := sealLegacyResetActivationV1(stateDir, LegacyResetActivationSealRequestV1{
		TargetSchema: 1, HarnessRoot: harnessRoot, TargetBinary: target,
		CatalogSHA256: strings.Repeat("a", 64), Evidence: writeLegacyResetActivationEvidenceFixtureV1(t),
	}, deps); !errors.Is(err, crash) {
		t.Fatalf("seal crash=%v", err)
	}
	control, err := sqlstore.Open(filepath.Join(stateDir, issueOpsResetV1Directory))
	if err != nil {
		t.Fatal(err)
	}
	if err := requireLegacyResetActivationV1(control, stateDir, 1, active); err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("pre-seal crash did not retain pending marker: %v", err)
	}
}

func TestConfirmLegacyResetV1ChecksActivationInsideDeletionLock(t *testing.T) {
	stateDir := t.TempDir()
	db, err := sqlstore.Open(filepath.Join(stateDir, issueOpsLegacyDirectory))
	if err != nil {
		t.Fatal(err)
	}
	record := []byte(`{"schema_version":9,"id":"io-aaaaaaaaaaaa","phase":"done","cycle_state":"closed"}`)
	if err := db.Put("issueops", "io-aaaaaaaaaaaa", record); err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewLegacyResetV1(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	deps := resetLegacyV1TestDeps()
	activationErr := errors.New("activation receipt missing")
	deps.RequireActivation = func(*sqlstore.DB, string, int, resetLegacyV1BinaryIdentity) error { return activationErr }
	if _, err := confirmLegacyResetV1(stateDir, 1, preview.Fingerprint, deps); !errors.Is(err, activationErr) {
		t.Fatalf("confirm error=%v", err)
	}
	if _, ok, err := db.Get("issueops", "io-aaaaaaaaaaaa"); err != nil || !ok {
		t.Fatalf("activation failure deleted legacy row: ok=%v err=%v", ok, err)
	}
}

func newLegacyResetActivationBinaryFixtureV1(t *testing.T) (string, string, string, string) {
	t.Helper()
	stateDir := t.TempDir()
	harnessRoot := t.TempDir()
	binDir := filepath.Join(harnessRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(binDir, "agent-harness")
	stage := filepath.Join(binDir, ".agent-harness.activate-test")
	if err := os.WriteFile(target, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stage, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return stateDir, harnessRoot, target, stage
}

func writeLegacyResetActivationEvidenceFixtureV1(t *testing.T) []port.NativeActivationEvidence {
	t.Helper()
	root := t.TempDir()
	specs := []struct{ host, surface string }{
		{"codex", "mcp"}, {"codex", "hooks"}, {"claude", "mcp"}, {"claude", "hooks"},
	}
	result := make([]port.NativeActivationEvidence, 0, len(specs))
	for index, spec := range specs {
		path := filepath.Join(root, spec.host+"-"+spec.surface)
		if err := os.WriteFile(path, []byte(spec.host+":"+spec.surface), 0o600); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		device, inode, ok := fileIdentityV1(info)
		if !ok {
			t.Fatal("activation evidence fixture has no physical identity")
		}
		hash, err := hashFileV1(path)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, port.NativeActivationEvidence{
			Host: spec.host, Surface: spec.surface, Path: path, SemanticSHA256: strings.Repeat(string(rune('c'+index)), 64),
			SHA256: hash, Mode: uint32(info.Mode()), Size: info.Size(), Device: device, Inode: inode,
		})
	}
	return result
}
