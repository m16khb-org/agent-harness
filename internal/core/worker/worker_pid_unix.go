//go:build unix

package worker

import "syscall"

// isPIDAlive returns true if a process with the given pid exists.
func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
