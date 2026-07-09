package basiccli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"agent-harness/internal/core"
	"agent-harness/internal/testsupport"
)

func init() {
	root := testHarnessRoot()
	Configure(Deps{
		HarnessRoot:   func() string { return root },
		ResolveTarget: testResolveTarget,
		Version:       "0.1.0",
		InspectHarness: func(repo string) core.InspectInfo {
			target := testResolveTarget(repo)
			home, _ := os.UserHomeDir()
			return core.InspectHarness(root, target, home, "0.1.0", "atomic-commit-push")
		},
	})
}

func testHarnessRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

func testResolveTarget(target string) string {
	if target != "" {
		return target
	}
	if projectDir := os.Getenv("CLAUDE_PROJECT_DIR"); projectDir != "" {
		return projectDir
	}
	if pwd := os.Getenv("PWD"); pwd != "" {
		return pwd
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func captureStatusVerifyStdout(t *testing.T, fn func() error) string {
	t.Helper()
	return testsupport.CaptureStdout(t, fn)
}

func runStatusVerifyTestCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, string(output))
	}
}

func captureTraceGuardPolicyStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	return testsupport.CaptureStdoutAndError(t, fn)
}

func writeFileForCLITest(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
