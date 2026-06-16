package state

import (
	"fmt"
	"os"
	"path/filepath"
)

// StateUpdate reads the current state record for key, passes it to the transform
// function, and writes the result. The read-modify-write is serialized with an
// advisory lock (flock on Unix, in-process mutex on other platforms) so
// concurrent callers across processes cannot lose updates.
//
// When the key does not exist, transform receives an empty StateRecord with
// OK=false. Return an empty StateRecord from transform to skip the write.
func StateUpdate(key string, transform func(StateRecord) (StateRecord, error)) (StateResult, error) {
	key, err := NormalizeStateKey(key)
	if err != nil {
		return StateResult{OK: false, StateDir: StateDir()}, err
	}
	dir := StateDir()
	var result StateResult
	err = withStateLock(dir, key, func() error {
		current, readErr := StateRead(key)
		if readErr != nil && !os.IsNotExist(readErr) {
			return readErr
		}
		next, transformErr := transform(current.Record)
		if transformErr != nil {
			return transformErr
		}
		// Empty record returned by transform = skip write.
		if next.Key == "" && next.SchemaVersion == 0 && next.Content == "" {
			result = StateResult{OK: true, StateDir: dir}
			return nil
		}
		path, writeErr := writeStateRecord(dir, key, next)
		result = StateResult{
			OK:       writeErr == nil,
			StateDir: dir,
			Path:     path,
			Record:   next,
		}
		return writeErr
	})
	if result.OK || result.StateDir != "" {
		return result, err
	}
	return StateResult{OK: false, StateDir: dir}, err
}

// WithKeyLock runs fn while holding the same exclusive advisory lock that
// StateUpdate uses (<dir>/<key>.state-lock), so callers OUTSIDE the state package
// can serialize their own read-modify-write or append spans against concurrent
// processes (e.g. the compact-capsule RMW and the hook-failure log append).
// Like withStateLock, the lock file is a persistent inode that must NOT be
// deleted between cycles.
func WithKeyLock(dir, key string, fn func() error) error {
	return withStateLock(dir, key, fn)
}

// withStateLock holds an exclusive advisory lock on <stateDir>/<key>.state-lock
// for the full read-modify-write span. The lock file is a persistent inode that
// must NOT be deleted between lock/unlock cycles (see CAUTIONS in
// issueops_lock_unix.go for why).
func withStateLock(dir, key string, fn func() error) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	lockPath := filepath.Join(dir, key+".state-lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("state lock: %w", err)
	}
	defer f.Close()
	if err := lockFile(f); err != nil {
		return fmt.Errorf("state lock: %w", err)
	}
	defer unlockFile(f)
	return fn()
}
