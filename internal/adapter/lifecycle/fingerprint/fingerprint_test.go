package fingerprint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestForRoot(t *testing.T) {
	t.Run("with git dir", func(t *testing.T) {
		dir := t.TempDir()
		gitDir := filepath.Join(dir, ".git")
		os.MkdirAll(gitDir, 0o755)
		fp := ForRoot(dir)
		if fp.GitDir != gitDir {
			t.Errorf("GitDir = %q, want %q", fp.GitDir, gitDir)
		}
	})

	t.Run("with git worktree file", func(t *testing.T) {
		dir := t.TempDir()
		realGit := filepath.Join(dir, "real.git")
		os.MkdirAll(realGit, 0o755)
		os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: "+realGit+"\n"), 0o644)
		fp := ForRoot(dir)
		if fp.GitDir != "gitdir: "+realGit {
			t.Errorf("GitDir = %q, want %q", fp.GitDir, "gitdir: "+realGit)
		}
	})
}

func TestRepoID(t *testing.T) {
	fp1 := ForRoot("/tmp/a")
	fp2 := ForRoot("/tmp/b")
	id1 := RepoID(fp1)
	id2 := RepoID(fp2)
	if id1 == "" || id2 == "" {
		t.Error("expected non-empty RepoID")
	}
	if len(id1) != 24 {
		t.Errorf("expected 24-char ID, got %d", len(id1))
	}
	if id1 == id2 {
		t.Error("different fingerprints should produce different IDs")
	}
}

func TestEqual(t *testing.T) {
	fp1 := ForRoot("/tmp/a")
	fp2 := ForRoot("/tmp/a")
	fp3 := ForRoot("/tmp/b")

	if !Equal(fp1, fp2) {
		t.Error("same roots should be equal")
	}
	if Equal(fp1, fp3) {
		t.Error("different roots should not be equal")
	}
}
