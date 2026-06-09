package queue

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// staleLockMaxAge is the duration after which a lock file is considered stale
// regardless of whether the PID is alive. This covers the case where a PID is
// reused by an unrelated process.
const staleLockMaxAge = 5 * time.Minute

func AcquireLock(projectStateDir string) (func(), bool, error) {
	if err := os.MkdirAll(projectStateDir, 0o700); err != nil {
		return nil, false, err
	}
	path := filepath.Join(projectStateDir, LockFile)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if os.IsExist(err) {
		// Lock file exists — check if it is stale before giving up.
		if isStale(path) {
			_ = os.Remove(path)
			// Retry once with a fresh O_EXCL.
			f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		}
		if os.IsExist(err) {
			return func() {}, false, nil
		}
	}
	if err != nil {
		return nil, false, err
	}
	_, writeErr := fmt.Fprintf(f, "%d %s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if writeErr != nil {
			return nil, false, writeErr
		}
		return nil, false, closeErr
	}
	return func() { _ = os.Remove(path) }, true, nil
}

// isStale reports whether the lock file at path is stale — its holding
// process is no longer alive or the lock is older than staleLockMaxAge.
func isStale(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	if time.Since(info.ModTime()) > staleLockMaxAge {
		return true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return true
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil || pid <= 0 {
		return true
	}
	return !processAlive(pid)
}
