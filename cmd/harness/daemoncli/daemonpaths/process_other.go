//go:build !darwin && !linux

package daemonpaths

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func InspectProcess(pid int) (ProcessIdentity, error) {
	if pid <= 0 {
		return ProcessIdentity{}, fmt.Errorf("pid must be positive")
	}
	startOut, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "lstart=").Output()
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("read process start time: %w", err)
	}
	exeOut, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("read process executable: %w", err)
	}
	start := strings.TrimSpace(string(startOut))
	if start == "" {
		return ProcessIdentity{}, fmt.Errorf("process start time is empty")
	}
	executable, err := canonicalExecutable(string(exeOut))
	if err != nil {
		return ProcessIdentity{}, err
	}
	return ProcessIdentity{StartTime: start, Executable: executable}, nil
}
