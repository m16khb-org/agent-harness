// Package session manages lightweight session-to-cycle bindings for
// multi-session IssueOps continuity. Each repo may have at most one active
// binding pointing to the IssueOps cycle the agent is currently working on.
//
// The binding is used by:
//   - The PreToolUse worktree guard (falls back to branch-based discovery when absent)
//   - The resume command (restores expected worktree context)
//   - Parallel cycle edit guard (identifies which specific worktree to enforce)
//
// Bindings are persisted under the IssueOps state root as
// <stateRoot>/issueops-session-<sha256(repo)>.json.
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Binding associates the current agent session with a specific IssueOps cycle.
type Binding struct {
	CycleID          string `json:"cycle_id"`
	Repo             string `json:"repo"`
	Branch           string `json:"branch"`
	ExpectedWorktree string `json:"expected_worktree,omitempty"`
	BoundAt          string `json:"bound_at"`
}

// Store abstracts the filesystem for testing.
type Store struct {
	StateRoot func() string
}

// Bind records a session-to-cycle binding. It overwrites any existing binding
// for the same repo. Pass an empty cycleID to unbind. The write runs under the
// per-repo session lock so concurrent cycles racing on the shared per-repo
// binding file cannot interleave their read-modify-write spans.
func Bind(store Store, repo, cycleID, branch, expectedWorktree string) error {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return fmt.Errorf("repo is required")
	}
	return withSessionLock(store, repo, func() error {
		return writeBindingForKey(store, repo, bindingKey(repo), cycleID, branch, expectedWorktree)
	})
}

// writeBinding performs the actual binding-file mutation. Callers must hold the
// per-repo session lock and pass an already-trimmed, non-empty repo.
func writeBinding(store Store, repo, cycleID, branch, expectedWorktree string) error {
	return writeBindingForKey(store, repo, bindingKey(repo), cycleID, branch, expectedWorktree)
}

func writeBindingForKey(store Store, repo, key, cycleID, branch, expectedWorktree string) error {
	dir := store.StateRoot()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := bindingPath(dir, key)

	if cycleID == "" {
		// Unbind: remove the file.
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
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
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".issueops-session-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
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
	path := bindingPath(store.StateRoot(), key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Binding{}, nil
		}
		return Binding{}, err
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
// points at cycleID. The read-compare-delete runs atomically under the per-repo
// session lock so closing one cycle never drops a binding that a concurrent
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

// UnbindScopedForCycle removes the scoped binding for cycleID only when the file
// still points at that same cycle. The compare-and-delete runs under the shared
// per-repo session lock.
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

	dir := store.StateRoot()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return bindings, nil
		}
		return nil, err
	}
	prefix := bindingKey(repo) + "-"
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".json") {
			continue
		}
		names = append(names, strings.TrimSuffix(name, ".json"))
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

// withSessionLock serializes all binding-file mutations for a repo under a
// per-repo advisory lock held on <stateRoot>/<bindingKey>.lock. The per-cycle
// IssueOps lock cannot serialize two different cycles racing on the shared
// per-repo binding file, so this lock guards bind vs cross-cycle unbind.
func withSessionLock(store Store, repo string, fn func() error) error {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return fmt.Errorf("repo is required")
	}
	dir := store.StateRoot()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return withFileLock(lockPath(dir, bindingKey(repo)), fn)
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

func bindingPath(dir, key string) string {
	return filepath.Join(dir, key+".json")
}

func lockPath(dir, key string) string {
	return filepath.Join(dir, key+".lock")
}
