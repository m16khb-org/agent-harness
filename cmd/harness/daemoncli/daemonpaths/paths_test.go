package daemonpaths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCurrentUsesDaemonDirEnv(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "daemon")
	t.Setenv("HARNESS_DAEMON_DIR", dir)
	t.Setenv("HARNESS_STATE_DIR", "")

	paths, err := Current()
	if err != nil {
		t.Fatalf("Current returned error: %v", err)
	}
	if paths.Dir != filepath.Clean(dir) {
		t.Fatalf("Dir = %q, want %q", paths.Dir, filepath.Clean(dir))
	}
	if paths.Socket != filepath.Join(paths.Dir, "agent-harness.sock") ||
		paths.PID != filepath.Join(paths.Dir, "agent-harness.pid") ||
		paths.Lock != filepath.Join(paths.Dir, "agent-harness.lock") ||
		paths.Log != filepath.Join(paths.Dir, "agent-harness.log") {
		t.Fatalf("unexpected derived paths: %#v", paths)
	}
}

func TestCurrentFallsBackToStateDir(t *testing.T) {
	state := t.TempDir()
	t.Setenv("HARNESS_DAEMON_DIR", " ")
	t.Setenv("HARNESS_STATE_DIR", state)

	paths, err := Current()
	if err != nil {
		t.Fatalf("Current returned error: %v", err)
	}
	if paths.Dir != filepath.Join(state, "daemon") {
		t.Fatalf("Dir = %q, want state daemon dir", paths.Dir)
	}
}

func TestReadPIDAndProcessAliveBoundaries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pid")
	if ReadPID(path) != 0 {
		t.Fatal("missing pid file should return 0")
	}
	if err := os.WriteFile(path, []byte("not-a-pid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if ReadPID(path) != 0 {
		t.Fatal("invalid pid file should return 0")
	}
	if err := os.WriteFile(path, []byte("123\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if ReadPID(path) != 123 {
		t.Fatalf("expected pid 123, got %d", ReadPID(path))
	}
	if ProcessAlive(0) {
		t.Fatal("pid 0 should not be alive")
	}
	if !ProcessAlive(os.Getpid()) {
		t.Fatal("current process should be alive")
	}
}
