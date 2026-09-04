package mcpcli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func mustMarshalMCPTest(t *testing.T, value any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func makeGitRepoForContract(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitForContract(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# contract fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/contract\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForContract(t, dir, "add", "README.md", "go.mod")
	runGitForContract(t, dir,
		"-c", "user.name=Contract Test",
		"-c", "user.email=contract@example.invalid",
		"commit", "-q",
		"-m", "docs(contract): add fixture",
		"-m", "Lore:\n- Intent: Normalize preflight contract.\n- Why: Response golden should cover git DTOs.\n- Changes:\n  - Add fixture README.\n- Verify: go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden\n- Risk: Low",
	)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("TOKEN=fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runGitForContract(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}
