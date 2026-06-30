//go:build !unix

package session

import "sync"

var (
	sessionFileLocks   = map[string]*sync.Mutex{}
	sessionFileLocksMu sync.Mutex
)

// withFileLock holds an exclusive per-path mutex for the full read-modify-write
// span. On !unix platforms this is an in-process sync.Mutex fallback; concurrent
// multi-session invocations on the same host may still race. The primary target
// platforms (darwin/linux) use the unix build tag with flock. This mirrors the
// fallback in internal/core/issueops/issueops_lock_other.go.
func withFileLock(lockPath string, fn func() error) error {
	sessionFileLocksMu.Lock()
	mu, ok := sessionFileLocks[lockPath]
	if !ok {
		mu = &sync.Mutex{}
		sessionFileLocks[lockPath] = mu
	}
	sessionFileLocksMu.Unlock()
	mu.Lock()
	defer mu.Unlock()
	return fn()
}
