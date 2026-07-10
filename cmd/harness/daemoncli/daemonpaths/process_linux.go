//go:build linux

package daemonpaths

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func InspectProcess(pid int) (ProcessIdentity, error) {
	if pid <= 0 {
		return ProcessIdentity{}, fmt.Errorf("pid must be positive")
	}
	procDir := filepath.Join("/proc", strconv.Itoa(pid))
	executable, err := os.Readlink(filepath.Join(procDir, "exe"))
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("read process executable: %w", err)
	}
	executable = strings.TrimSuffix(executable, " (deleted)")
	executable, err = canonicalExecutable(executable)
	if err != nil {
		return ProcessIdentity{}, err
	}
	stat, err := os.ReadFile(filepath.Join(procDir, "stat"))
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("read process start time: %w", err)
	}
	closing := strings.LastIndexByte(string(stat), ')')
	if closing < 0 {
		return ProcessIdentity{}, fmt.Errorf("invalid process stat")
	}
	fields := strings.Fields(string(stat)[closing+1:])
	const startTimeIndex = 19 // field 22 after removing pid and parenthesized comm
	if len(fields) <= startTimeIndex || fields[startTimeIndex] == "" {
		return ProcessIdentity{}, fmt.Errorf("process stat start time is missing")
	}
	return ProcessIdentity{StartTime: "linux:" + fields[startTimeIndex], Executable: executable}, nil
}
