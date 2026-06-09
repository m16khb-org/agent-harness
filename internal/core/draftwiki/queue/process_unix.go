//go:build unix

package queue

import (
	"syscall"
)

// processAlive reports whether a process with the given PID exists.
// Uses kill(pid, 0) which checks existence without sending a signal.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, syscall.Signal(0))
	return err == nil || err == syscall.EPERM
}
