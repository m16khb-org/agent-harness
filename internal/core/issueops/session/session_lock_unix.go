//go:build unix

package session

import (
	"os"

	"golang.org/x/sys/unix"
)

// withFileLock holds an exclusive advisory lock on lockPath for the full
// read-modify-write span. unix.Flock auto-releases on fd close / process death
// (no stale-lock deadlock) and is honored across processes — so concurrent
// multi-session CLI/daemon invocations on the same host serialize per repo.
//
// The lock file must NOT be deleted between lock/unlock cycles: flock locks are
// associated with the open file description (the inode), so deleting and
// recreating the lock file creates a new inode, breaking mutual exclusion
// across contenders that opened different inodes. This mirrors the invariant in
// internal/core/issueops/issueops_lock_unix.go.
func withFileLock(lockPath string, fn func() error) error {
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
