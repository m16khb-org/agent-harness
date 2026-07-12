package daemonpaths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type Paths struct {
	Dir    string `json:"dir"`
	Socket string `json:"socket"`
	PID    string `json:"pid_file"`
	Lock   string `json:"lock_file"`
	Log    string `json:"log_file"`
}

func Current() (Paths, error) {
	dir := strings.TrimSpace(os.Getenv("HARNESS_DAEMON_DIR"))
	if dir == "" {
		if state := strings.TrimSpace(os.Getenv("HARNESS_STATE_DIR")); state != "" {
			dir = filepath.Join(state, "daemon")
		} else if home, err := os.UserHomeDir(); err == nil && home != "" {
			dir = filepath.Join(home, ".local", "state", "agent-harness", "daemon")
		} else {
			return Paths{}, fmt.Errorf("cannot resolve daemon directory")
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Paths{}, err
	}
	abs = filepath.Clean(abs)
	return Paths{
		Dir:    abs,
		Socket: filepath.Join(abs, "agent-harness.sock"),
		PID:    filepath.Join(abs, "agent-harness.pid"),
		Lock:   filepath.Join(abs, "agent-harness.lock"),
		Log:    filepath.Join(abs, "agent-harness.log"),
	}, nil
}

func ReadPID(path string) int {
	record, _, err := ReadInstance(path)
	if err != nil {
		return 0
	}
	return record.PID
}

func ProcessAlive(pid int) bool {
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
