package issueopscli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRemoteArtifactGateSmokeScript(t *testing.T) {
	repoRoot := findRepoRootForScriptTest(t)
	scriptPath := filepath.Join(repoRoot, "scripts", "remote-artifact-gate-smoke.sh")
	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("stat smoke script: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("smoke script is not executable: mode=%v", info.Mode())
	}

	harnessBin := filepath.Join(t.TempDir(), "agent-harness")
	build := exec.Command("go", "build", "-o", harnessBin, "./cmd/harness")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build harness for smoke script: %v\n%s", err, string(output))
	}

	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "HARNESS_BIN="+harnessBin)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("remote artifact gate smoke script failed: %v\n%s", err, string(output))
	}
}

func findRepoRootForScriptTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root from %s", dir)
		}
		dir = parent
	}
}
