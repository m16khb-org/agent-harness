package daemonlock

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireCreatesLockWithCurrentPID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.lock")
	f, err := Acquire(path, func() int { return 42 }, func(int) bool { return true })
	if err != nil {
		t.Fatalf("Acquire returned error: %v", err)
	}
	defer f.Close()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	if string(b) != "42\n" {
		t.Fatalf("unexpected lock content %q", string(b))
	}
}

func TestAcquireRemovesStaleLockForDeadProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.lock")
	if err := os.WriteFile(path, []byte("42\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := Acquire(path, func() int { return 99 }, func(pid int) bool { return pid != 42 })
	if err != nil {
		t.Fatalf("Acquire returned error: %v", err)
	}
	defer f.Close()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	if string(b) != "99\n" {
		t.Fatalf("expected refreshed lock content, got %q", string(b))
	}
}

func TestAcquireRejectsFreshLiveLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.lock")
	if err := os.WriteFile(path, []byte("42\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := Acquire(path, func() int { return 99 }, func(int) bool { return true })
	if err == nil {
		f.Close()
		t.Fatal("expected lock acquisition error")
	}
}

func TestStaleLockBoundaries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.lock")
	if !stale(path, time.Second, func(int) bool { return true }) {
		t.Fatal("missing lock should be stale")
	}
	if err := os.WriteFile(path, []byte("bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if stale(path, time.Hour, func(int) bool { return false }) {
		t.Fatal("invalid fresh pid should not be stale")
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if !stale(path, time.Second, func(int) bool { return true }) {
		t.Fatal("old lock should be stale")
	}
}
