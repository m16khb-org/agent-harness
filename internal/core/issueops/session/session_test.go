package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionBindAndRead(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}

	// Initially empty.
	b, err := Read(store, "/repo/example")
	if err != nil {
		t.Fatal(err)
	}
	if b.CycleID != "" {
		t.Errorf("expected empty binding, got %q", b.CycleID)
	}

	// Bind.
	if err := Bind(store, "/repo/example", "io-abc123", "1-feat", "/repo.worktrees/1-feat"); err != nil {
		t.Fatal(err)
	}

	// Read back.
	b, err = Read(store, "/repo/example")
	if err != nil {
		t.Fatal(err)
	}
	if b.CycleID != "io-abc123" {
		t.Errorf("expected io-abc123, got %q", b.CycleID)
	}
	if b.Branch != "1-feat" {
		t.Errorf("expected 1-feat, got %q", b.Branch)
	}
	if b.ExpectedWorktree != "/repo.worktrees/1-feat" {
		t.Errorf("expected /repo.worktrees/1-feat, got %q", b.ExpectedWorktree)
	}
	if b.BoundAt == "" {
		t.Error("expected bound_at")
	}

	// Overwrite.
	if err := Bind(store, "/repo/example", "io-xyz789", "2-fix", "/repo.worktrees/2-fix"); err != nil {
		t.Fatal(err)
	}
	b, err = Read(store, "/repo/example")
	if err != nil {
		t.Fatal(err)
	}
	if b.CycleID != "io-xyz789" {
		t.Errorf("expected io-xyz789, got %q", b.CycleID)
	}
}

func TestSessionUnbind(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}

	if err := Bind(store, "/repo/example", "io-abc123", "1-feat", "/worktree"); err != nil {
		t.Fatal(err)
	}
	if err := Unbind(store, "/repo/example"); err != nil {
		t.Fatal(err)
	}
	b, err := Read(store, "/repo/example")
	if err != nil {
		t.Fatal(err)
	}
	if b.CycleID != "" {
		t.Errorf("expected empty after unbind, got %q", b.CycleID)
	}

	// Unbind when nothing bound is a no-op.
	if err := Unbind(store, "/repo/example"); err != nil {
		t.Fatal(err)
	}
}

func TestSessionDifferentRepos(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}

	if err := Bind(store, "/repo/a", "io-aaa", "1-a", "/wa"); err != nil {
		t.Fatal(err)
	}
	if err := Bind(store, "/repo/b", "io-bbb", "2-b", "/wb"); err != nil {
		t.Fatal(err)
	}

	// Repo A.
	b, err := Read(store, "/repo/a")
	if err != nil {
		t.Fatal(err)
	}
	if b.CycleID != "io-aaa" {
		t.Errorf("repo a: expected io-aaa, got %q", b.CycleID)
	}

	// Repo B.
	b, err = Read(store, "/repo/b")
	if err != nil {
		t.Fatal(err)
	}
	if b.CycleID != "io-bbb" {
		t.Errorf("repo b: expected io-bbb, got %q", b.CycleID)
	}

	// Unbind A should not affect B.
	if err := Unbind(store, "/repo/a"); err != nil {
		t.Fatal(err)
	}
	b, err = Read(store, "/repo/b")
	if err != nil {
		t.Fatal(err)
	}
	if b.CycleID != "io-bbb" {
		t.Errorf("repo b after unbind a: expected io-bbb, got %q", b.CycleID)
	}
}

func TestSessionExpectedWorktree(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}

	// No binding, no cycle fallback.
	got := ExpectedWorktree(store, "/repo/x", nil)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	// No binding, fallback to cycle worktree.
	got = ExpectedWorktree(store, "/repo/x", func() string { return "/fallback-worktree" })
	if got != "/fallback-worktree" {
		t.Errorf("expected fallback, got %q", got)
	}

	// Binding with worktree takes precedence over fallback.
	if err := Bind(store, "/repo/x", "io-123", "1-x", "/bound-worktree"); err != nil {
		t.Fatal(err)
	}
	got = ExpectedWorktree(store, "/repo/x", func() string { return "/fallback-worktree" })
	if got != "/bound-worktree" {
		t.Errorf("expected bound worktree, got %q", got)
	}

	// Binding without worktree falls back.
	if err := Bind(store, "/repo/x", "io-456", "2-y", ""); err != nil {
		t.Fatal(err)
	}
	got = ExpectedWorktree(store, "/repo/x", func() string { return "/fallback-worktree" })
	if got != "/fallback-worktree" {
		t.Errorf("expected fallback when bound without worktree, got %q", got)
	}
}

func TestSessionActiveCycleID(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}

	if got := ActiveCycleID(store, "/repo/x"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	if err := Bind(store, "/repo/x", "io-abc", "1-x", "/wx"); err != nil {
		t.Fatal(err)
	}
	if got := ActiveCycleID(store, "/repo/x"); got != "io-abc" {
		t.Errorf("expected io-abc, got %q", got)
	}
}

func TestSessionFilePermissions(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}

	if err := Bind(store, "/repo/p", "io-123", "1-p", "/wp"); err != nil {
		t.Fatal(err)
	}

	// Find the binding file.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("expected 0600, got %#o for %s", info.Mode().Perm(), e.Name())
		}
	}

	// Content is valid JSON.
	b, err := Read(store, "/repo/p")
	if err != nil {
		t.Fatal(err)
	}
	if b.Repo != "/repo/p" {
		t.Errorf("expected /repo/p, got %q", b.Repo)
	}
}

func TestSessionTrimWhitespace(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}

	if err := Bind(store, "  /repo/trim  ", "  io-trim  ", "  1-t  ", "  /wt  "); err != nil {
		t.Fatal(err)
	}

	b, err := Read(store, "/repo/trim")
	if err != nil {
		t.Fatal(err)
	}
	if b.CycleID != "io-trim" {
		t.Errorf("expected io-trim, got %q", b.CycleID)
	}
	if b.Branch != "1-t" {
		t.Errorf("expected 1-t, got %q", b.Branch)
	}
}

func TestSessionEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	store := Store{StateRoot: func() string { return dir }}

	if err := Bind(store, "", "io-123", "1-x", "/wx"); err == nil {
		t.Error("expected error for empty repo")
	}
	if _, err := Read(store, ""); err == nil {
		t.Error("expected error for empty repo")
	}
}

func TestSessionStateRootNotExist(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent", "sub")
	store := Store{StateRoot: func() string { return dir }}

	// Bind creates the directory.
	if err := Bind(store, "/repo/x", "io-123", "1-x", "/wx"); err != nil {
		t.Fatal(err)
	}

	// Read from non-existent dir returns empty binding, not error.
	emptyDir := filepath.Join(t.TempDir(), "empty-nope")
	emptyStore := Store{StateRoot: func() string { return emptyDir }}
	b, err := Read(emptyStore, "/repo/x")
	if err != nil {
		t.Fatal(err)
	}
	if b.CycleID != "" {
		t.Errorf("expected empty, got %q", b.CycleID)
	}
}
