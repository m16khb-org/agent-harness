// Package session manages lightweight session-to-cycle bindings for
// multi-session IssueOps continuity. Each repo may have at most one active
// binding pointing to the IssueOps cycle the agent is currently working on.
//
// The binding is used by:
//   - The PreToolUse worktree guard (falls back to branch-based discovery when absent)
//   - The resume command (restores expected worktree context)
//   - Parallel cycle edit guard (identifies which specific worktree to enforce)
//
// Bindings are persisted in the state root's sqlstore database under the
// "session" bucket, keyed by issueops-session-<sha256(repo)> (plus a
// -<cycleID> suffix for scoped per-cycle bindings).
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"agent-harness/internal/core/sqlstore"
)

// sessionBucket is the sqlstore bucket holding one row per binding key.
const sessionBucket = "session"

// Binding associates the current agent session with a specific IssueOps cycle.
type Binding struct {
	CycleID          string `json:"cycle_id"`
	Repo             string `json:"repo"`
	Branch           string `json:"branch"`
	ExpectedWorktree string `json:"expected_worktree,omitempty"`
	BoundAt          string `json:"bound_at"`
}

// Store abstracts the state root for testing.
type Store struct {
	StateRoot func() string
}

func openDB(store Store) (*sqlstore.DB, error) {
	return sqlstore.Open(store.StateRoot())
}

// Bind records a session-to-cycle binding. It overwrites any existing binding
// for the same repo. Pass an empty cycleID to unbind. The write runs under the
// state root's span lock so concurrent cycles racing on the shared per-repo
// binding cannot interleave their read-modify-write spans.
func Bind(store Store, repo, cycleID, branch, expectedWorktree string) error {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return fmt.Errorf("repo is required")
	}
	return withSessionLock(store, repo, func() error {
		return writeBindingForKey(store, repo, bindingKey(repo), cycleID, branch, expectedWorktree)
	})
}

// writeBinding performs the actual binding mutation. Callers must hold the
// session span lock and pass an already-trimmed, non-empty repo.
func writeBinding(store Store, repo, cycleID, branch, expectedWorktree string) error {
	return writeBindingForKey(store, repo, bindingKey(repo), cycleID, branch, expectedWorktree)
}

func writeBindingForKey(store Store, repo, key, cycleID, branch, expectedWorktree string) error {
	db, err := openDB(store)
	if err != nil {
		return err
	}

	if cycleID == "" {
		// Unbind: remove the record.
		return db.Delete(sessionBucket, key)
	}

	b := Binding{
		CycleID:          strings.TrimSpace(cycleID),
		Repo:             repo,
		Branch:           strings.TrimSpace(branch),
		ExpectedWorktree: strings.TrimSpace(expectedWorktree),
		BoundAt:          time.Now().UTC().Format(time.RFC3339Nano),
	}

	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return db.Put(sessionBucket, key, data)
}

// Read returns the current session-to-cycle binding for a repo, or an empty
// Binding when none exists.
func Read(store Store, repo string) (Binding, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return Binding{}, fmt.Errorf("repo is required")
	}
	return readBindingForKey(store, repo, bindingKey(repo))
}

func readBindingForKey(store Store, repo, key string) (Binding, error) {
	db, err := openDB(store)
	if err != nil {
		return Binding{}, err
	}
	data, ok, err := db.Get(sessionBucket, key)
	if err != nil {
		return Binding{}, err
	}
	if !ok {
		return Binding{}, nil
	}
	var b Binding
	if err := json.Unmarshal(data, &b); err != nil {
		return Binding{}, err
	}
	return b, nil
}

// ExpectedWorktree returns the expected worktree path from the session binding,
// or falls back to the cycle record's worktree path. Returns empty string when
// neither source has a worktree.
func ExpectedWorktree(store Store, repo string, cycleWorktree func() string) string {
	b, err := Read(store, repo)
	if err == nil && b.ExpectedWorktree != "" {
		return b.ExpectedWorktree
	}
	if cycleWorktree != nil {
		return cycleWorktree()
	}
	return ""
}

// ActiveCycleID returns the bound cycle ID for a repo, or empty when unbound.
func ActiveCycleID(store Store, repo string) string {
	b, err := Read(store, repo)
	if err != nil {
		return ""
	}
	return b.CycleID
}

// Unbind removes the session binding for a repo.
func Unbind(store Store, repo string) error {
	return Bind(store, repo, "", "", "")
}

// UnbindForCycle removes the session binding for a repo only when it still
// points at cycleID. The read-compare-delete runs atomically under the session
// span lock so closing one cycle never drops a binding that a concurrent
// cycle wrote between the read and the delete (TOCTOU).
func UnbindForCycle(store Store, repo, cycleID string) error {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return fmt.Errorf("repo is required")
	}
	cycleID = strings.TrimSpace(cycleID)
	return withSessionLock(store, repo, func() error {
		b, err := Read(store, repo)
		if err != nil {
			return err
		}
		if b.CycleID != cycleID {
			return nil
		}
		return writeBinding(store, repo, "", "", "")
	})
}

// BindScoped records a per-cycle binding without overwriting the repo's primary
// binding. Scoped bindings share the primary repo lock so primary/scoped
// mutations stay linearized.
func BindScoped(store Store, repo, cycleID, branch, expectedWorktree string) error {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return fmt.Errorf("repo is required")
	}
	cycleID = strings.TrimSpace(cycleID)
	if err := validateScopedCycleID(cycleID); err != nil {
		return err
	}
	return withSessionLock(store, repo, func() error {
		return writeBindingForKey(store, repo, scopedBindingKey(repo, cycleID), cycleID, branch, expectedWorktree)
	})
}

// ReadScoped returns the per-cycle binding for repo/cycleID, or an empty Binding
// when none exists.
func ReadScoped(store Store, repo, cycleID string) (Binding, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return Binding{}, fmt.Errorf("repo is required")
	}
	cycleID = strings.TrimSpace(cycleID)
	if err := validateScopedCycleID(cycleID); err != nil {
		return Binding{}, err
	}
	return readBindingForKey(store, repo, scopedBindingKey(repo, cycleID))
}

// UnbindScopedForCycle removes the scoped binding for cycleID only when the
// record still points at that same cycle. The compare-and-delete runs under the
// shared session span lock.
func UnbindScopedForCycle(store Store, repo, cycleID string) error {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return fmt.Errorf("repo is required")
	}
	cycleID = strings.TrimSpace(cycleID)
	if err := validateScopedCycleID(cycleID); err != nil {
		return err
	}
	return withSessionLock(store, repo, func() error {
		key := scopedBindingKey(repo, cycleID)
		b, err := readBindingForKey(store, repo, key)
		if err != nil {
			return err
		}
		if b.CycleID != cycleID {
			return nil
		}
		return writeBindingForKey(store, repo, key, "", "", "")
	})
}

// ListBindings returns the primary repo binding, if present, followed by scoped
// per-cycle bindings for the same repo.
func ListBindings(store Store, repo string) ([]Binding, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return nil, fmt.Errorf("repo is required")
	}
	var bindings []Binding
	if primary, err := Read(store, repo); err != nil {
		return nil, err
	} else if primary.CycleID != "" {
		bindings = append(bindings, primary)
	}

	db, err := openDB(store)
	if err != nil {
		return nil, err
	}
	keys, err := db.List(sessionBucket)
	if err != nil {
		return nil, err
	}
	prefix := bindingKey(repo) + "-"
	names := make([]string, 0, len(keys))
	for _, key := range keys {
		if strings.HasPrefix(key, prefix) {
			names = append(names, key)
		}
	}
	sort.Strings(names)
	for _, key := range names {
		b, err := readBindingForKey(store, repo, key)
		if err != nil {
			return nil, err
		}
		if b.CycleID != "" {
			bindings = append(bindings, b)
		}
	}
	return bindings, nil
}

// withSessionLock serializes all binding mutations for a repo under the state
// root's span lock. The per-cycle IssueOps lock cannot serialize two different
// cycles racing on the shared per-repo binding, so this lock guards bind vs
// cross-cycle unbind. It must never be entered while already inside another
// span on the same state root (spans do not nest).
func withSessionLock(store Store, repo string, fn func() error) error {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return fmt.Errorf("repo is required")
	}
	db, err := openDB(store)
	if err != nil {
		return err
	}
	return db.WithSpan(fn)
}

// StaleBinding pairs a session binding's sqlstore key with its cycle ID for
// stale cleanup.
type StaleBinding struct {
	Key     string `json:"key"`
	CycleID string `json:"cycle_id"`
}

// FindStaleBindings lists every session binding for repo whose cycle is not
// live (isCycleLive returns false). It scans both the primary binding key and
// scoped per-cycle keys. Returns entries keyed by their sqlstore key so the
// caller can pass them to PruneStaleBindings for deletion.
func FindStaleBindings(store Store, repo string, isCycleLive func(cycleID string) bool) ([]StaleBinding, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return nil, fmt.Errorf("repo is required")
	}
	db, err := openDB(store)
	if err != nil {
		return nil, err
	}
	primaryKey := bindingKey(repo)
	prefix := primaryKey + "-"
	keys, err := db.List(sessionBucket)
	if err != nil {
		return nil, err
	}
	var stale []StaleBinding
	for _, key := range keys {
		if key != primaryKey && !strings.HasPrefix(key, prefix) {
			continue
		}
		b, err := readBindingForKey(store, repo, key)
		if err != nil {
			return nil, err
		}
		if b.CycleID == "" {
			continue
		}
		if !isCycleLive(b.CycleID) {
			stale = append(stale, StaleBinding{Key: key, CycleID: b.CycleID})
		}
	}
	return stale, nil
}

// PruneStaleBindings deletes stale bindings after a TOCTOU re-check: if a
// binding was re-bound to a live cycle between the scan and the delete, it is
// left untouched. Returns the number of bindings actually deleted.
func PruneStaleBindings(store Store, repo string, stale []StaleBinding, isCycleLive func(cycleID string) bool) (int, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return 0, fmt.Errorf("repo is required")
	}
	pruned := 0
	for _, entry := range stale {
		deleted := false
		err := withSessionLock(store, repo, func() error {
			fresh, err := readBindingForKey(store, repo, entry.Key)
			if err != nil {
				return err
			}
			if fresh.CycleID != entry.CycleID {
				return nil
			}
			if isCycleLive(fresh.CycleID) {
				return nil
			}
			deleted = true
			return writeBindingForKey(store, repo, entry.Key, "", "", "")
		})
		if err != nil {
			return pruned, err
		}
		if deleted {
			pruned++
		}
	}
	return pruned, nil
}

func bindingKey(repo string) string {
	sum := sha256.Sum256([]byte(repo))
	return "issueops-session-" + hex.EncodeToString(sum[:])[:16]
}

func scopedBindingKey(repo, cycleID string) string {
	return bindingKey(repo) + "-" + cycleID
}

func validateScopedCycleID(cycleID string) error {
	if len(cycleID) != len("io-000000000000") || !strings.HasPrefix(cycleID, "io-") {
		return fmt.Errorf("cycle_id must match io-[0-9a-f]{12}")
	}
	for _, r := range cycleID[3:] {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return fmt.Errorf("cycle_id must match io-[0-9a-f]{12}")
		}
	}
	return nil
}
