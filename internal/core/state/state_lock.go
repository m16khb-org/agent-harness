package state

import (
	"os"
)

// StateUpdate reads the current state record for key, passes it to the transform
// function, and writes the result. The read-modify-write runs under the state
// directory's sqlstore span (in-process mutex + cross-process sqlite write
// lock) so concurrent callers across processes cannot lose updates.
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

// WithKeyLock runs fn while holding the same exclusive span that StateUpdate
// uses for dir, so callers OUTSIDE the state package can serialize their own
// read-modify-write or append spans against concurrent processes (e.g. the
// compact-capsule RMW and the hook-failure log append). All spans on one
// directory serialize together and must not nest.
func WithKeyLock(dir, key string, fn func() error) error {
	return withStateLock(dir, key, fn)
}

// withStateLock holds the state directory's sqlstore span for the full
// read-modify-write span. The span serializes in-process via a per-directory
// mutex and cross-process via a held BEGIN IMMEDIATE transaction on the
// directory's lock database, which is released automatically on process death.
func withStateLock(dir, key string, fn func() error) error {
	if _, err := NormalizeStateKey(key); err != nil {
		return err
	}
	db, err := openStateDB(dir)
	if err != nil {
		return err
	}
	return db.WithSpan(fn)
}
