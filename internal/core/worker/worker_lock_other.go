//go:build !unix

package worker

import (
	"os"
	"sync"
)

var (
	workerJobLocks   = map[string]*sync.Mutex{}
	workerJobLocksMu sync.Mutex
)

// withWorkerJobLock holds an exclusive per-id mutex for the full
// read-modify-write span. On !unix platforms this is an in-process sync.Mutex
// fallback; concurrent multi-session invocations on the same host may still
// race. The primary target platforms (darwin/linux) use the unix build tag with
// flock.
func withWorkerJobLock(dir, jobID string, fn func() error) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	key := dir + "\x00" + jobID
	workerJobLocksMu.Lock()
	mu, ok := workerJobLocks[key]
	if !ok {
		mu = &sync.Mutex{}
		workerJobLocks[key] = mu
	}
	workerJobLocksMu.Unlock()
	mu.Lock()
	defer mu.Unlock()
	return fn()
}
