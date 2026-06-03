package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStableExecutablePathRemapsCellarToBinSymlink(t *testing.T) {
	prefix := t.TempDir() // stand-in for /opt/homebrew
	cellarBin := filepath.Join(prefix, "Cellar", "agent-harness", "0.1.0", "bin")
	if err := os.MkdirAll(cellarBin, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(cellarBin, "agent-harness")
	if err := os.WriteFile(exe, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	stable := filepath.Join(prefix, "bin", "agent-harness")
	if err := os.MkdirAll(filepath.Dir(stable), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(exe, stable); err != nil {
		t.Fatal(err)
	}
	if got := stableExecutablePath(exe); got != stable {
		t.Fatalf("expected stable bin symlink %q, got %q", stable, got)
	}
}

func TestStableExecutablePathLeavesNonCellarUnchanged(t *testing.T) {
	exe := "/Users/dev/agent-harness/bin/agent-harness"
	if got := stableExecutablePath(exe); got != exe {
		t.Fatalf("non-Cellar path must be unchanged, got %q", got)
	}
}

func TestStableExecutablePathKeepsCellarWhenNoBinSymlink(t *testing.T) {
	prefix := t.TempDir()
	cellarBin := filepath.Join(prefix, "Cellar", "agent-harness", "0.1.0", "bin")
	if err := os.MkdirAll(cellarBin, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(cellarBin, "agent-harness")
	if err := os.WriteFile(exe, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	// No prefix/bin symlink created -> fall back to the original path.
	if got := stableExecutablePath(exe); got != exe {
		t.Fatalf("missing bin symlink should keep original path, got %q", got)
	}
}
