package lintdiagnose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnoseCommandRejectsEmptyAgyDiagnosis(t *testing.T) {
	root := t.TempDir()
	fakeAgy := filepath.Join(root, "fake-agy.sh")
	if err := os.WriteFile(fakeAgy, []byte("#!/bin/sh\nprintf '%s' '{\"diagnosis\":\"\"}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := DiagnoseCommand(LintDiagnoseRequest{
		RepoRoot:    root,
		CommandArgv: []string{"/bin/sh", "-c", "echo lint failed >&2; exit 2"},
		AgyCommand:  fakeAgy,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Failed {
		t.Fatalf("expected failed command result: %+v", result)
	}
	if !strings.Contains(result.Diagnosis, "missing diagnosis") {
		t.Fatalf("expected parsing diagnostic for empty diagnosis, got %q", result.Diagnosis)
	}
}
