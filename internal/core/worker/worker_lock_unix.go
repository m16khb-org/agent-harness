//go:build unix

package worker

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// withWorkerJobLock holds an exclusive advisory lock on
// <workerDir>/<jobID>.lock for the full read-modify-write span. unix.Flock
// auto-releases on fd close / process death (no stale-lock deadlock), and is
// honored across processes — so concurrent multi-session CLI/daemon invocations
// on the same host serialize per job.
//
// The lock file must NOT be deleted between lock/unlock cycles: flock locks are
// associated with the open file description (the inode), so deleting and
// recreating the lock file creates a new inode, breaking mutual exclusion across
// contenders that opened different inodes.
func withWorkerJobLock(dir, jobID string, fn func() error) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	lockPath := filepath.Join(dir, jobID+".lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return fn()
}
