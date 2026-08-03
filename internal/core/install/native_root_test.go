package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveStableNativeRoot(t *testing.T) {
	t.Run("normal checkout remains canonical", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "source")
		mustMkdir(t, filepath.Join(root, ".git"))

		got, err := ResolveStableNativeRoot(root)
		if err != nil {
			t.Fatal(err)
		}
		if got != root {
			t.Fatalf("stable root = %q, want %q", got, root)
		}
	})

	t.Run("linked worktree resolves absolute gitdir and relative commondir", func(t *testing.T) {
		base := t.TempDir()
		source := filepath.Join(base, "source")
		worktree := filepath.Join(base, "source.worktrees", "completed")
		gitdir := filepath.Join(source, ".git", "worktrees", "completed")
		mustMkdir(t, filepath.Join(source, ".git"))
		mustMkdir(t, worktree)
		mustMkdir(t, gitdir)
		mustWrite(t, filepath.Join(worktree, ".git"), "gitdir: "+gitdir+"\n")
		mustWrite(t, filepath.Join(gitdir, "commondir"), "../..\n")

		got, err := ResolveStableNativeRoot(worktree)
		if err != nil {
			t.Fatal(err)
		}
		if got != source {
			t.Fatalf("stable root = %q, want %q", got, source)
		}
	})

	t.Run("linked worktree resolves relative gitdir", func(t *testing.T) {
		base := t.TempDir()
		source := filepath.Join(base, "source")
		worktree := filepath.Join(base, "worktrees", "relative")
		gitdir := filepath.Join(source, ".git", "worktrees", "relative")
		mustMkdir(t, filepath.Join(source, ".git"))
		mustMkdir(t, worktree)
		mustMkdir(t, gitdir)
		relativeGitdir, err := filepath.Rel(worktree, gitdir)
		if err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(worktree, ".git"), "gitdir: "+relativeGitdir+"\n")
		mustWrite(t, filepath.Join(gitdir, "commondir"), "../..\n")

		got, err := ResolveStableNativeRoot(worktree)
		if err != nil {
			t.Fatal(err)
		}
		if got != source {
			t.Fatalf("stable root = %q, want %q", got, source)
		}
	})

	t.Run("malformed worktree metadata fails closed", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "worktree")
		mustMkdir(t, root)
		mustWrite(t, filepath.Join(root, ".git"), "not-a-gitdir\n")

		_, err := ResolveStableNativeRoot(root)
		if err == nil || !strings.Contains(err.Error(), "gitdir") {
			t.Fatalf("error = %v, want gitdir diagnostic", err)
		}
	})
}

func TestValidateStableNativeRuntime(t *testing.T) {
	root := filepath.Join(t.TempDir(), "source")
	mustMkdir(t, filepath.Join(root, ".git"))
	canonical := filepath.Join(root, "bin", "agent-harness")
	if err := ValidateStableNativeRuntime(root, canonical); err != nil {
		t.Fatalf("canonical runtime rejected: %v", err)
	}

	external := filepath.Join(filepath.Dir(root), "source.worktrees", "completed", "bin", "agent-harness")
	if err := ValidateStableNativeRuntime(root, external); err == nil || !strings.Contains(err.Error(), canonical) {
		t.Fatalf("error = %v, want expected canonical runtime %q", err, canonical)
	}
}

func TestDefaultNativeInstallRequestMapsLinkedWorktree(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source")
	worktree := filepath.Join(base, "source.worktrees", "completed")
	gitdir := filepath.Join(source, ".git", "worktrees", "completed")
	mustMkdir(t, filepath.Join(source, ".git"))
	mustMkdir(t, worktree)
	mustMkdir(t, gitdir)
	mustWrite(t, filepath.Join(worktree, ".git"), "gitdir: "+gitdir+"\n")
	mustWrite(t, filepath.Join(gitdir, "commondir"), "../..\n")

	req := DefaultNativeInstallRequest(worktree, filepath.Join(base, "home"), "", "")
	if req.Root != source {
		t.Fatalf("root = %q, want %q", req.Root, source)
	}
	wantBin := filepath.Join(source, "bin", "agent-harness")
	if req.BinPath != wantBin {
		t.Fatalf("bin path = %q, want %q", req.BinPath, wantBin)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}
