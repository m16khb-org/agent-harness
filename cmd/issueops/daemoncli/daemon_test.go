package daemoncli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"issueops/cmd/issueops/daemoncli/daemonpaths"
)

func TestDaemonPathsUseOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_DAEMON_DIR", dir)
	paths, err := currentDaemonPaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.Dir != dir || filepath.Base(paths.Socket) != "issueops.sock" || filepath.Base(paths.PID) != "issueops.pid" {
		t.Fatalf("unexpected daemon paths: %+v", paths)
	}
}

func TestDaemonPathsUseStateDirFallback(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_DAEMON_DIR", "")
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)

	paths, err := currentDaemonPaths()
	if err != nil {
		t.Fatal(err)
	}

	wantDir := filepath.Join(stateDir, "daemon")
	if paths.Dir != wantDir || paths.Socket != filepath.Join(wantDir, "issueops.sock") {
		t.Fatalf("unexpected state fallback daemon paths: %+v", paths)
	}
}

func TestCheckDaemonStatusReportsReachableSocket(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "ahd-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	t.Setenv("ISSUEOPS_DAEMON_DIR", dir)
	paths, err := currentDaemonPaths()
	if err != nil {
		t.Fatal(err)
	}
	instance := startVerifiedDaemonTestSocket(t, paths)

	status := checkDaemonStatus()

	if !status.OK || !status.Running || !status.Reachable || !status.IdentityVerified || status.PID != os.Getpid() || status.Code != daemonStatusReady || status.Instance == nil || *status.Instance != instance {
		t.Fatalf("unexpected reachable daemon status: %#v", status)
	}
}

func TestCheckDaemonStatusReportsLivePIDWithoutSocket(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_DAEMON_DIR", dir)
	paths, err := currentDaemonPaths()
	if err != nil {
		t.Fatal(err)
	}
	process, err := daemonpaths.InspectProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	instance := daemonInstance{PID: os.Getpid(), ProcessStartTime: process.StartTime, Executable: process.Executable, InstanceNonce: "nonce-a", BuildSHA: "build-a", ProtocolVersion: daemonProtocolVersion, Generation: "generation-a"}
	if err := daemonpaths.WriteInstance(paths.PID, instance); err != nil {
		t.Fatal(err)
	}

	status := checkDaemonStatus()

	if status.OK || !status.Running || status.Reachable || status.IdentityVerified || status.PID != os.Getpid() || status.Code != daemonStatusSocketUnreachable || status.Message != "daemon pid exists but socket is not reachable" {
		t.Fatalf("unexpected pid-only daemon status: %#v", status)
	}
}

func TestAcquireDaemonLockRemovesStaleLock(t *testing.T) {
	dir := t.TempDir()
	paths := daemonPaths{Dir: dir, Lock: filepath.Join(dir, "issueops.lock")}
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
	paths := daemonPaths{Dir: dir, Lock: filepath.Join(dir, "issueops.lock")}
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

func TestWaitForDaemonWithDepsReportsReadyStatus(t *testing.T) {
	paths := daemonPaths{Dir: t.TempDir()}
	calls := 0

	status, err := waitForDaemonWithDeps(paths, time.Second, daemonWaitDeps{
		now: func() time.Time {
			return time.Unix(100, 0).Add(time.Duration(calls) * time.Millisecond)
		},
		sleep: func(time.Duration) {},
		checkStatus: func() daemonStatus {
			calls++
			if calls == 2 {
				return daemonStatus{OK: true, Running: true, Reachable: true, IdentityVerified: true, PID: 42, Code: daemonStatusReady, Paths: paths, Instance: &daemonInstance{PID: 42}, Message: "daemon is reachable and identity verified"}
			}
			return daemonStatus{OK: true, Code: daemonStatusStopped, Paths: paths, Message: "daemon is not running"}
		},
	})

	if err != nil {
		t.Fatalf("expected ready daemon status, got error: %v", err)
	}
	if !status.Running || !status.IdentityVerified || status.Code != daemonStatusReady || calls != 2 {
		t.Fatalf("unexpected ready daemon status: %#v calls=%d", status, calls)
	}
}

func TestWaitForDaemonWithDepsReturnsTimeoutWithFallbackPaths(t *testing.T) {
	paths := daemonPaths{Dir: t.TempDir()}
	now := time.Unix(100, 0)

	status, err := waitForDaemonWithDeps(paths, 3*time.Millisecond, daemonWaitDeps{
		now: func() time.Time {
			current := now
			now = now.Add(time.Millisecond)
			return current
		},
		sleep: func(time.Duration) {},
		checkStatus: func() daemonStatus {
			return daemonStatus{OK: true, Running: false}
		},
	})

	if err == nil || err.Error() != "daemon did not become ready before timeout" {
		t.Fatalf("expected timeout error, got status=%#v err=%v", status, err)
	}
	if status.Running || status.Paths.Dir != paths.Dir || status.Message != "daemon did not become ready before timeout" {
		t.Fatalf("unexpected timeout daemon status: %#v", status)
	}
}

func TestWaitForDaemonReturnsTimeoutWithoutStartingProcess(t *testing.T) {
	paths := daemonPaths{Dir: t.TempDir()}

	status, err := waitForDaemon(paths, time.Nanosecond)

	if err == nil || err.Error() != "daemon did not become ready before timeout" {
		t.Fatalf("expected timeout error, got status=%#v err=%v", status, err)
	}
	if status.Running || status.Paths.Dir != paths.Dir {
		t.Fatalf("unexpected daemon wait status: %#v", status)
	}
}
