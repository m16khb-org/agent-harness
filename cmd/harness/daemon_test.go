package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
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

func TestDaemonPathsUseStateDirFallback(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_DAEMON_DIR", "")
	t.Setenv("HARNESS_STATE_DIR", stateDir)

	paths, err := currentDaemonPaths()
	if err != nil {
		t.Fatal(err)
	}

	wantDir := filepath.Join(stateDir, "daemon")
	if paths.Dir != wantDir || paths.Socket != filepath.Join(wantDir, "agent-harness.sock") {
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
	t.Setenv("HARNESS_DAEMON_DIR", dir)
	paths, err := currentDaemonPaths()
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", paths.Socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := os.WriteFile(paths.PID, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	status := checkDaemonStatus()

	if !status.OK || !status.Running || status.PID != os.Getpid() || status.Message != "daemon is reachable" {
		t.Fatalf("unexpected reachable daemon status: %#v", status)
	}
}

func TestCheckDaemonStatusReportsLivePIDWithoutSocket(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_DAEMON_DIR", dir)
	paths, err := currentDaemonPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.PID, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	status := checkDaemonStatus()

	if !status.OK || status.Running || status.PID != os.Getpid() || status.Message != "daemon pid exists but socket is not reachable" {
		t.Fatalf("unexpected pid-only daemon status: %#v", status)
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
			return daemonStatus{OK: true, Running: calls == 2, Paths: paths, Message: "daemon is reachable"}
		},
	})

	if err != nil {
		t.Fatalf("expected ready daemon status, got error: %v", err)
	}
	if !status.Running || status.Message != "daemon is reachable" || calls != 2 {
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

func TestEnsureDaemonRunningWithDepsReturnsExistingStatus(t *testing.T) {
	want := daemonStatus{OK: true, Running: true, PID: 42, Message: "daemon is reachable"}

	status, err := ensureDaemonRunningWithDeps(daemonStartDeps{
		checkStatus: func() daemonStatus {
			return want
		},
	})

	if err != nil {
		t.Fatalf("expected existing daemon status, got %v", err)
	}
	if status != want {
		t.Fatalf("unexpected daemon status: %#v", status)
	}
}

func TestEnsureDaemonRunningWithDepsWaitsWhenLockIsBusy(t *testing.T) {
	paths := daemonPaths{Dir: t.TempDir()}
	lockErr := errors.New("lock busy")
	waitStatus := daemonStatus{OK: true, Running: true, PID: 77, Paths: paths}

	status, err := ensureDaemonRunningWithDeps(daemonStartDeps{
		checkStatus: func() daemonStatus {
			return daemonStatus{OK: true, Running: false}
		},
		paths: func() (daemonPaths, error) {
			return paths, nil
		},
		mkdirAll: func(string, os.FileMode) error {
			return nil
		},
		acquireLock: func(daemonPaths) (daemonStartLock, error) {
			return nil, lockErr
		},
		wait: func(got daemonPaths, timeout time.Duration) (daemonStatus, error) {
			if got != paths || timeout != daemonReadyTimeout {
				t.Fatalf("unexpected wait inputs: %#v %s", got, timeout)
			}
			return waitStatus, nil
		},
	})

	if err != nil {
		t.Fatalf("expected launcher wait success, got %v", err)
	}
	if status != waitStatus {
		t.Fatalf("unexpected wait status: %#v", status)
	}
}

func TestEnsureDaemonRunningWithDepsStartsDaemonAndCleansLock(t *testing.T) {
	paths := daemonPaths{Dir: t.TempDir(), Lock: filepath.Join(t.TempDir(), "agent-harness.lock")}
	lock := &daemonStartFakeLock{}
	checks := 0
	started := false
	removedLock := false
	waitStatus := daemonStatus{OK: true, Running: true, PID: 88, Paths: paths}

	status, err := ensureDaemonRunningWithDeps(daemonStartDeps{
		checkStatus: func() daemonStatus {
			checks++
			return daemonStatus{OK: true, Running: false}
		},
		paths: func() (daemonPaths, error) {
			return paths, nil
		},
		mkdirAll: func(string, os.FileMode) error {
			return nil
		},
		acquireLock: func(daemonPaths) (daemonStartLock, error) {
			return lock, nil
		},
		remove: func(path string) error {
			if path == paths.Lock {
				removedLock = true
			}
			return nil
		},
		executable: func() (string, error) {
			return "agent-harness", nil
		},
		startDaemon: func(exe string, got daemonPaths) error {
			if exe != "agent-harness" || got != paths {
				t.Fatalf("unexpected start inputs: %s %#v", exe, got)
			}
			started = true
			return nil
		},
		wait: func(daemonPaths, time.Duration) (daemonStatus, error) {
			return waitStatus, nil
		},
	})

	if err != nil {
		t.Fatalf("expected daemon start success, got %v", err)
	}
	if status != waitStatus || !started || !lock.closed || checks != 2 || !removedLock {
		t.Fatalf("unexpected start state: status=%#v started=%v lock=%v checks=%d removedLock=%v", status, started, lock.closed, checks, removedLock)
	}
}

func TestEnsureDaemonRunningWithDepsReturnsStartErrorWithPaths(t *testing.T) {
	paths := daemonPaths{Dir: t.TempDir(), Lock: filepath.Join(t.TempDir(), "agent-harness.lock")}
	startErr := errors.New("start failed")
	lock := &daemonStartFakeLock{}

	status, err := ensureDaemonRunningWithDeps(daemonStartDeps{
		checkStatus: func() daemonStatus {
			return daemonStatus{OK: true, Running: false}
		},
		paths: func() (daemonPaths, error) {
			return paths, nil
		},
		mkdirAll: func(string, os.FileMode) error {
			return nil
		},
		acquireLock: func(daemonPaths) (daemonStartLock, error) {
			return lock, nil
		},
		remove: func(string) error {
			return nil
		},
		executable: func() (string, error) {
			return "agent-harness", nil
		},
		startDaemon: func(string, daemonPaths) error {
			return startErr
		},
	})

	if !errors.Is(err, startErr) {
		t.Fatalf("expected start error, got status=%#v err=%v", status, err)
	}
	if status.OK || status.Paths != paths || status.Message != "start failed" || !lock.closed {
		t.Fatalf("unexpected failed start status: %#v lockClosed=%v", status, lock.closed)
	}
}

func TestStartDaemonProcessReturnsStartError(t *testing.T) {
	err := startDaemonProcess(filepath.Join(t.TempDir(), "missing-agent-harness"), daemonPaths{Dir: t.TempDir()})

	if err == nil {
		t.Fatal("expected missing executable to fail")
	}
}

type daemonStartFakeLock struct {
	closed bool
}

func (l *daemonStartFakeLock) Close() error {
	l.closed = true
	return nil
}
