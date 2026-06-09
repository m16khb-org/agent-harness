//go:build !unix

package issueops

import (
	"os"
	"sync"
)

var (
	issueOpsLocks   = map[string]*sync.Mutex{}
	issueOpsLocksMu sync.Mutex
)

// withIssueOpsLock holds an exclusive per-id mutex for the full read-modify-write
// span. On !unix platforms this is an in-process sync.Mutex fallback; concurrent
// multi-session invocations on the same host may still race. The primary target
// platforms (darwin/linux) use the unix build tag with flock.
func withIssueOpsLock(stateRoot, id string, fn func() error) error {
	id, err := normalizeIssueOpsID(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		return err
	}
	key := stateRoot + "\x00" + id
	issueOpsLocksMu.Lock()
	mu, ok := issueOpsLocks[key]
	if !ok {
		mu = &sync.Mutex{}
		issueOpsLocks[key] = mu
	}
	issueOpsLocksMu.Unlock()
	mu.Lock()
	defer mu.Unlock()
	return fn()
}
