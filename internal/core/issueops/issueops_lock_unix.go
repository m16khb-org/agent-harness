//go:build unix

package issueops

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// withIssueOpsLock holds an exclusive advisory lock on <stateRoot>/<id>.lock for
// the full read-modify-write span. unix.Flock auto-releases on fd close /
// process death (no stale-lock deadlock), and is honored across processes — so
// concurrent multi-session CLI/daemon invocations on the same host serialize per
// id.
//
// The lock file must NOT be deleted between lock/unlock cycles: flock locks are
// associated with the open file description (the inode), so deleting and
// recreating the lock file creates a new inode, breaking mutual exclusion across
// contenders that opened different inodes. Orphaned .lock files (with no matching
// .json cycle file) are cleaned by the off-hot-path stale scan instead.
func withIssueOpsLock(stateRoot, id string, fn func() error) error {
	id, err := normalizeIssueOpsID(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		return err
	}
	lockPath := filepath.Join(stateRoot, id+".lock")
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
