package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func TestResponseContractsGolden(t *testing.T) {
	stubIssueOpsChildIssueVerifier(t, nil)
	stateDir := t.TempDir()
	workspaceDir := t.TempDir()
	gitRepoDir := makeGitRepoForContract(t)
	homeDir := t.TempDir()
	auditLog := filepath.Join(t.TempDir(), "audit.jsonl")
	workerDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	t.Setenv("HARNESS_AUDIT_LOG", auditLog)
	t.Setenv("HARNESS_WORKER_DIR", workerDir)
	t.Setenv("HOME", homeDir)
	t.Setenv("CODEX_HOME", filepath.Join(homeDir, ".codex"))

	replacements := map[string]string{
		stateDir:      "$STATE_DIR",
		workspaceDir:  "$WORKSPACE",
		gitRepoDir:    "$GIT_REPO",
		harnessRoot(): "$HARNESS_ROOT",
		homeDir:       "$HOME",
		auditLog:      "$AUDIT_LOG",
		workerDir:     "$WORKER_DIR",
	}
	addEvalSymlinkReplacement(t, replacements, stateDir, "$STATE_DIR")
	addEvalSymlinkReplacement(t, replacements, workspaceDir, "$WORKSPACE")
	addEvalSymlinkReplacement(t, replacements, gitRepoDir, "$GIT_REPO")
	addEvalSymlinkReplacement(t, replacements, harnessRoot(), "$HARNESS_ROOT")
	addEvalSymlinkReplacement(t, replacements, workerDir, "$WORKER_DIR")
	addEvalSymlinkReplacement(t, replacements, filepath.Dir(auditLog), "$AUDIT_DIR")

	snapshot := map[string]any{
		"cli": buildCLIResponseContractSnapshot(t, replacements, stateDir, workspaceDir, gitRepoDir),
		"mcp": buildMCPResponseContractSnapshot(t, replacements, workspaceDir, gitRepoDir),
	}
	assertJSONGolden(t, "response_contracts.golden.json", snapshot)
}

func assertJSONGolden(t *testing.T, name string, value any) {
	t.Helper()
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, name, append(b, '\n'))
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run go test ./cmd/harness -run Golden -update)", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, string(got), string(want))
	}
}
