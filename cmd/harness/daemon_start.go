package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func ensureDaemonRunning() (daemonStatus, error) {
	if status := checkDaemonStatus(); status.Running {
		return status, nil
	}
	paths, err := currentDaemonPaths()
	if err != nil {
		return daemonStatus{}, err
	}
	if err := os.MkdirAll(paths.Dir, 0o700); err != nil {
		return daemonStatus{}, err
	}
	lock, err := acquireDaemonLock(paths)
	if err != nil {
		// Another launcher may be starting it. Wait briefly.
		if status, waitErr := waitForDaemon(paths, daemonReadyTimeout); waitErr == nil && status.Running {
			return status, nil
		}
		return daemonStatus{OK: false, Running: false, Paths: paths, Message: err.Error()}, err
	}
	defer func() {
		_ = lock.Close()
		_ = os.Remove(paths.Lock)
	}()
	if status := checkDaemonStatus(); status.Running {
		return status, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return daemonStatus{}, err
	}
	cmd := exec.Command(exe, "daemon", "--internal")
	cmd.Env = append(os.Environ(), "HARNESS_DAEMON_DIR="+paths.Dir, "HARNESS_ROOT="+harnessRoot())
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return daemonStatus{OK: false, Paths: paths, Message: err.Error()}, err
	}
	_ = cmd.Process.Release()
	return waitForDaemon(paths, daemonReadyTimeout)
}

func waitForDaemon(paths daemonPaths, timeout time.Duration) (daemonStatus, error) {
	return waitForDaemonWithDeps(paths, timeout, daemonWaitDeps{
		now:         time.Now,
		sleep:       time.Sleep,
		checkStatus: checkDaemonStatus,
	})
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
		if last.Running {
			return last, nil
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
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(paths.Lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = f.WriteString(strconv.Itoa(os.Getpid()) + "\n")
			return f, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if isStaleDaemonLock(paths.Lock, 30*time.Second) {
			_ = os.Remove(paths.Lock)
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("cannot acquire daemon lock %s", paths.Lock)
}

func isStaleDaemonLock(path string, maxAge time.Duration) bool {
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	if time.Since(info.ModTime()) > maxAge {
		return true
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return false
	}
	return !processAlive(pid)
}
