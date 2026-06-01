package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestServeMCPStreamListsHarnessTools(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"harness://commit-policy"}}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	var diag bytes.Buffer
	if err := serveMCPStream(strings.NewReader(input), &out, &diag); err != nil {
		t.Fatal(err)
	}
	lines := splitLines(out.String())
	if len(lines) != 3 {
		t.Fatalf("expected 3 responses, got %d: %s", len(lines), out.String())
	}
	for _, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("invalid json %q: %v", line, err)
		}
		if _, ok := obj["result"]; !ok {
			t.Fatalf("missing result: %s", line)
		}
	}
	if !strings.Contains(out.String(), "atomic_commit_preflight") || !strings.Contains(out.String(), "Lore") {
		t.Fatalf("missing harness tools/context in output:\n%s", out.String())
	}
}
func TestDaemonPathsUseOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_DAEMON_DIR", dir)
	paths, err := currentDaemonPaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.Dir != dir || filepath.Base(paths.Socket) != "agent-harness.sock" || filepath.Base(paths.PID) != "agent-harness.pid" {
		t.Fatalf("unexpected daemon paths: %+v", paths)
	}
}

func TestAcquireDaemonLockRemovesStaleLock(t *testing.T) {
	dir := t.TempDir()
	paths := daemonPaths{Dir: dir, Lock: filepath.Join(dir, "agent-harness.lock")}
	if err := os.WriteFile(paths.Lock, []byte("999999\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Minute)
	if err := os.Chtimes(paths.Lock, old, old); err != nil {
		t.Fatal(err)
	}
	lock, err := acquireDaemonLock(paths)
	if err != nil {
		t.Fatalf("acquireDaemonLock should recover stale lock: %v", err)
	}
	defer lock.Close()
	b, err := os.ReadFile(paths.Lock)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), strconv.Itoa(os.Getpid())) {
		t.Fatalf("lock file was not replaced with current pid: %q", string(b))
	}
}

func TestAcquireDaemonLockRejectsFreshLiveLock(t *testing.T) {
	dir := t.TempDir()
	paths := daemonPaths{Dir: dir, Lock: filepath.Join(dir, "agent-harness.lock")}
	if err := os.WriteFile(paths.Lock, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lock, err := acquireDaemonLock(paths)
	if err == nil {
		_ = lock.Close()
		t.Fatal("expected fresh live daemon lock to be rejected")
	}
	b, readErr := os.ReadFile(paths.Lock)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(b), strconv.Itoa(os.Getpid())) {
		t.Fatalf("fresh lock was unexpectedly replaced: %q", string(b))
	}
}
