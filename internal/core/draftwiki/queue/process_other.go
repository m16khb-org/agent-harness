//go:build !unix

package queue

// processAlive is a best-effort check on non-Unix platforms.
// It always returns true (assumes alive) on platforms where
// signal-based process checking is not available.
func processAlive(pid int) bool {
	return true
}
