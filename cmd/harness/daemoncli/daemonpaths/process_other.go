//go:build !darwin && !linux

package daemonpaths

import (
	"fmt"
)

func InspectProcess(pid int) (ProcessIdentity, error) {
	if pid <= 0 {
		return ProcessIdentity{}, fmt.Errorf("pid must be positive")
	}
	startOut, err := processFieldWithCLocale(pid, "lstart=")
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("read process start time: %w", err)
	}
	exeOut, err := processFieldWithCLocale(pid, "comm=")
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("read process executable: %w", err)
	}
	start, err := canonicalProcessStartTime(string(startOut))
	if err != nil {
		return ProcessIdentity{}, err
	}
	executable, err := canonicalExecutable(string(exeOut))
	if err != nil {
		return ProcessIdentity{}, err
	}
	return ProcessIdentity{StartTime: start, Executable: executable}, nil
}
