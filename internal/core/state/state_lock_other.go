//go:build !unix

package state

import (
	"os"
	"sync"
)

var (
	stateLocks   = map[string]*sync.Mutex{}
	stateLocksMu sync.Mutex
)

func lockFile(f *os.File) error {
	// On non-Unix, use an in-process mutex keyed on the file path.
	// This serializes within a single process but does NOT protect
	// across processes. The primary target platforms (darwin/linux)
	// use the unix build tag with flock.
	key := f.Name()
	stateLocksMu.Lock()
	mu, ok := stateLocks[key]
	if !ok {
		mu = &sync.Mutex{}
		stateLocks[key] = mu
	}
	stateLocksMu.Unlock()
	mu.Lock()
	return nil
}

func unlockFile(f *os.File) error {
	key := f.Name()
	stateLocksMu.Lock()
	mu, ok := stateLocks[key]
	stateLocksMu.Unlock()
	if ok {
		mu.Unlock()
	}
	return nil
}
