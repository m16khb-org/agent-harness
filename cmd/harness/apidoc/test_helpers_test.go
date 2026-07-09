package apidoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"agent-harness/internal/testsupport"
)

func captureStatusVerifyStdout(t *testing.T, fn func() error) string {
	t.Helper()
	out, err := captureAPIDocStdout(t, fn)
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	return out
}

func captureTraceGuardPolicyStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	return captureAPIDocStdout(t, fn)
}

func captureAPIDocStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	return testsupport.CaptureStdoutAndError(t, fn)
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

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func writeFileForWrapperTest(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
