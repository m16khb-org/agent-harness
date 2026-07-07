//go:build !unix

package workpool

import "sync"

var (
	workpoolFileLocks   = map[string]*sync.Mutex{}
	workpoolFileLocksMu sync.Mutex
)

func withFileLock(lockPath string, fn func() error) error {
	workpoolFileLocksMu.Lock()
	mu, ok := workpoolFileLocks[lockPath]
	if !ok {
		mu = &sync.Mutex{}
		workpoolFileLocks[lockPath] = mu
	}
	workpoolFileLocksMu.Unlock()
	mu.Lock()
	defer mu.Unlock()
	return fn()
}
