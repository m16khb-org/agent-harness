//go:build !unix

package worker

import "os"

// isPIDAlive returns true if a process with the given pid exists.
// On non-Unix platforms this is a best-effort check: os.FindProcess
// always succeeds on Windows, so dead PIDs will not be detected.
func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	_, err := os.FindProcess(pid)
	return err == nil
}
