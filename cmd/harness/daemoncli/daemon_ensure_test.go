package daemoncli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureDaemonRunningWithDepsStopsBeforeWorkWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ensureDaemonRunningWithDeps(daemonStartDeps{
		context: ctx,
		checkStatus: func() daemonStatus {
			t.Fatal("canceled ensure must not inspect or start the daemon")
			return daemonStatus{}
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled ensure error = %v", err)
	}
}

func TestEnsureDaemonRunningWithDepsReturnsExistingStatus(t *testing.T) {
	want := daemonStatus{OK: true, Running: true, Reachable: true, IdentityVerified: true, PID: 42, Code: daemonStatusReady, Instance: &daemonInstance{PID: 42}, Message: "daemon is reachable and identity verified"}

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
	waitStatus := daemonStatus{OK: true, Running: true, Reachable: true, IdentityVerified: true, PID: 77, Code: daemonStatusReady, Paths: paths, Instance: &daemonInstance{PID: 77}}

	status, err := ensureDaemonRunningWithDeps(daemonStartDeps{
		checkStatus: func() daemonStatus {
			return daemonStatus{OK: true, Code: daemonStatusStopped}
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
	waitStatus := daemonStatus{OK: true, Running: true, Reachable: true, IdentityVerified: true, PID: 88, Code: daemonStatusReady, Paths: paths, Instance: &daemonInstance{PID: 88}}

	status, err := ensureDaemonRunningWithDeps(daemonStartDeps{
		checkStatus: func() daemonStatus {
			checks++
			return daemonStatus{OK: true, PID: 999999, Code: daemonStatusStopped, Instance: &daemonInstance{PID: 999999}}
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
			return daemonStatus{OK: true, Code: daemonStatusStopped}
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
