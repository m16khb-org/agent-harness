package main

import (
	"path/filepath"
	"testing"
)

func TestDirContainsTermIgnoresTestOnlySignals(t *testing.T) {
	root := t.TempDir()
	relDir := filepath.Join("cmd", "harness")
	writeFileForWrapperTest(t, filepath.Join(root, relDir, "signal_test.go"), "package main\nconst marker = \"production-only-signal\"\n")

	if dirContainsTerm(root, relDir, "production-only-signal") {
		t.Fatalf("test-only source was accepted as production repo signal")
	}

	writeFileForWrapperTest(t, filepath.Join(root, relDir, "signal.go"), "package main\nconst marker = \"production-only-signal\"\n")
	if !dirContainsTerm(root, relDir, "production-only-signal") {
		t.Fatalf("production source was not accepted as repo signal")
	}
}
