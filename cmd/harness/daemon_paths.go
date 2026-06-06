package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func currentDaemonPaths() (daemonPaths, error) {
	dir := strings.TrimSpace(os.Getenv("HARNESS_DAEMON_DIR"))
	if dir == "" {
		if state := strings.TrimSpace(os.Getenv("HARNESS_STATE_DIR")); state != "" {
			dir = filepath.Join(state, "daemon")
		} else if home, err := os.UserHomeDir(); err == nil && home != "" {
			dir = filepath.Join(home, ".local", "state", "agent-harness", "daemon")
		} else {
			return daemonPaths{}, fmt.Errorf("cannot resolve daemon directory")
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return daemonPaths{}, err
	}
	abs = filepath.Clean(abs)
	return daemonPaths{
		Dir:    abs,
		Socket: filepath.Join(abs, "agent-harness.sock"),
		PID:    filepath.Join(abs, "agent-harness.pid"),
		Lock:   filepath.Join(abs, "agent-harness.lock"),
		Log:    filepath.Join(abs, "agent-harness.log"),
	}, nil
}

func readDaemonPID(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0
	}
	return pid
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
