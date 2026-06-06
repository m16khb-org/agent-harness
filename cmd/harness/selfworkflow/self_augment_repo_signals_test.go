package selfworkflow

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

func TestCollectSelfAugmentRepoSignalsFindsMCPAdapterCatalogInContractCLI(t *testing.T) {
	root := t.TempDir()
	writeFileForWrapperTest(t, filepath.Join(root, "internal", "adapter", "mcp", "catalog.go"), "package mcp\nfunc AdapterOwnedTools() {}\n")
	writeFileForWrapperTest(t, filepath.Join(root, "cmd", "harness", "contractcli", "contract.go"), "package contractcli\nconst marker = \"mcpadapter.AdapterOwnedTools\"\n")

	signals := collectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasMCPAdapterCatalog {
		t.Fatalf("contractcli MCP adapter catalog signal was not detected: %+v", signals)
	}
}
