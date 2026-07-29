package daemoncli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"

	"agent-harness/cmd/harness/daemoncli/daemonlock"
)

func ensureDaemonRunning() (daemonStatus, error) {
	return ensureDaemonRunningContext(context.Background())
}

func ensureDaemonRunningContext(ctx context.Context) (daemonStatus, error) {
	return ensureDaemonRunningWithDeps(daemonStartDeps{
		context:     ctx,
		checkStatus: checkDaemonStatus,
		paths:       currentDaemonPaths,
		mkdirAll:    os.MkdirAll,
		acquireLock: func(paths daemonPaths) (daemonStartLock, error) {
			return acquireDaemonLock(paths)
		},
		remove:      os.Remove,
		executable:  os.Executable,
		startDaemon: startDaemonProcess,
		wait:        waitForDaemon,
		waitContext: waitForDaemonContext,
	})
}

type daemonStartLock interface {
	Close() error
}

type daemonStartDeps struct {
	context     context.Context
	checkStatus func() daemonStatus
	paths       func() (daemonPaths, error)
	mkdirAll    func(string, os.FileMode) error
	acquireLock func(daemonPaths) (daemonStartLock, error)
	remove      func(string) error
	executable  func() (string, error)
	startDaemon func(exe string, paths daemonPaths) error
	wait        func(daemonPaths, time.Duration) (daemonStatus, error)
	waitContext func(context.Context, daemonPaths, time.Duration) (daemonStatus, error)
}

func ensureDaemonRunningWithDeps(deps daemonStartDeps) (daemonStatus, error) {
	ctx := deps.context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return daemonStatus{}, err
	}
	wait := func(paths daemonPaths, timeout time.Duration) (daemonStatus, error) {
		if deps.waitContext != nil {
			return deps.waitContext(ctx, paths, timeout)
		}
		return deps.wait(paths, timeout)
	}
	if status := deps.checkStatus(); daemonStatusIsReady(status) {
		return status, nil
	} else if daemonStatusBlocksStart(status) {
		return status, errors.New(status.Code + ": " + status.Message)
	}
	paths, err := deps.paths()
	if err != nil {
		return daemonStatus{}, err
	}
	if err := deps.mkdirAll(paths.Dir, 0o700); err != nil {
		return daemonStatus{}, err
	}
	lock, err := deps.acquireLock(paths)
	if err != nil {
		// 다른 launcher가 시작 중일 수 있다. 잠시 기다린다.
		if status, waitErr := wait(paths, daemonReadyTimeout); waitErr == nil && daemonStatusIsReady(status) {
			return status, nil
		}
		return daemonStatus{OK: false, Running: false, Paths: paths, Message: err.Error()}, err
	}
	defer func() {
		_ = lock.Close()
		_ = deps.remove(paths.Lock)
	}()
	if status := deps.checkStatus(); daemonStatusIsReady(status) {
		return status, nil
	} else if daemonStatusBlocksStart(status) {
		return status, errors.New(status.Code + ": " + status.Message)
	}
	if err := ctx.Err(); err != nil {
		return daemonStatus{}, err
	}
	exe, err := deps.executable()
	if err != nil {
		return daemonStatus{}, err
	}
	if err := deps.startDaemon(exe, paths); err != nil {
		return daemonStatus{OK: false, Paths: paths, Message: err.Error()}, err
	}
	return wait(paths, daemonReadyTimeout)
}

func startDaemonProcess(exe string, paths daemonPaths) error {
	cmd := exec.Command(exe, "daemon", "--internal")
	cmd.Env = append(os.Environ(), "HARNESS_DAEMON_DIR="+paths.Dir, "HARNESS_ROOT="+HarnessRoot())
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	return nil
}

func waitForDaemon(paths daemonPaths, timeout time.Duration) (daemonStatus, error) {
	return waitForDaemonWithDeps(paths, timeout, daemonWaitDeps{
		now:         time.Now,
		sleep:       time.Sleep,
		checkStatus: checkDaemonStatus,
	})
}

func waitForDaemonContext(ctx context.Context, paths daemonPaths, timeout time.Duration) (daemonStatus, error) {
	deadline := time.Now().Add(timeout)
	var last daemonStatus
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return last, err
		}
		last = checkDaemonStatus()
		if daemonStatusIsReady(last) {
			return last, nil
		}
		if daemonStatusBlocksStart(last) {
			return last, errors.New(last.Code + ": " + last.Message)
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return last, ctx.Err()
		}
	}
	if last.Paths.Dir == "" {
		last.Paths = paths
	}
	last.Message = "daemon did not become ready before timeout"
	return last, errors.New(last.Message)
}

type daemonWaitDeps struct {
	now         func() time.Time
	sleep       func(time.Duration)
	checkStatus func() daemonStatus
}

func waitForDaemonWithDeps(paths daemonPaths, timeout time.Duration, deps daemonWaitDeps) (daemonStatus, error) {
	deadline := deps.now().Add(timeout)
	var last daemonStatus
	for deps.now().Before(deadline) {
		last = deps.checkStatus()
		if daemonStatusIsReady(last) {
			return last, nil
		}
		if daemonStatusBlocksStart(last) {
			return last, errors.New(last.Code + ": " + last.Message)
		}
		deps.sleep(50 * time.Millisecond)
	}
	if last.Paths.Dir == "" {
		last.Paths = paths
	}
	last.Message = "daemon did not become ready before timeout"
	return last, errors.New(last.Message)
}

func acquireDaemonLock(paths daemonPaths) (*os.File, error) {
	return daemonlock.Acquire(paths.Lock, os.Getpid, processAlive)
}
