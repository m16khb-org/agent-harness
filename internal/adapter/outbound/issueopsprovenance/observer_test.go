package issueopsprovenance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	provenanceport "issueops/internal/port/issueopsprovenance"
)

var _ provenanceport.Observer = (*executableObserver)(nil)

func TestExecutableObserverFailsWithoutSyntheticEvidence(t *testing.T) {
	observer := executableObserver{
		executable: func() (string, error) { return "", errors.New("executable unavailable") },
	}
	got, err := observer.Observe(context.Background())
	if err == nil {
		t.Fatal("executable observation failure must be returned")
	}
	if got != (provenanceport.Receipt{}) {
		t.Fatalf("failure must not synthesize provenance: %+v", got)
	}
}

func TestExecutableObserverCanonicalizesAndHashesExecutable(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "issueops-real")
	content := []byte("current binary fixture")
	if err := os.WriteFile(executable, content, 0o755); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(dir, "issueops")
	if err := os.Symlink(executable, linked); err != nil {
		t.Fatal(err)
	}

	observer := executableObserver{executable: func() (string, error) { return linked, nil }}
	got, err := observer.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	canonicalExecutable, err := filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256(content)
	if got.ExecutablePath != canonicalExecutable {
		t.Fatalf("canonical executable = %q, want %q", got.ExecutablePath, canonicalExecutable)
	}
	if got.ExecutableSHA256 != hex.EncodeToString(wantHash[:]) {
		t.Fatalf("executable sha256 = %q, want %q", got.ExecutableSHA256, hex.EncodeToString(wantHash[:]))
	}
}
