package augmentcatalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirContainsTermIgnoresTestOnlySignals(t *testing.T) {
	root := t.TempDir()
	relDir := filepath.Join("cmd", "harness")
	writeFileForRepoSignalTest(t, filepath.Join(root, relDir, "signal_test.go"), "package main\nconst marker = \"production-only-signal\"\n")

	if dirContainsTerm(root, relDir, "production-only-signal") {
		t.Fatalf("test-only source was accepted as production repo signal")
	}

	writeFileForRepoSignalTest(t, filepath.Join(root, relDir, "signal.go"), "package main\nconst marker = \"production-only-signal\"\n")
	if !dirContainsTerm(root, relDir, "production-only-signal") {
		t.Fatalf("production source was not accepted as repo signal")
	}
}

func TestCollectSelfAugmentRepoSignalsFindsMCPAdapterCatalogInContractCLI(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "adapter", "mcp", "catalog.go"), "package mcp\nfunc AdapterOwnedTools() {}\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, "cmd", "harness", "contractcli", "contract.go"), "package contractcli\nconst marker = \"mcpadapter.AdapterOwnedTools\"\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasMCPAdapterCatalog {
		t.Fatalf("contractcli MCP adapter catalog signal was not detected: %+v", signals)
	}
}

func writeFileForRepoSignalTest(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
