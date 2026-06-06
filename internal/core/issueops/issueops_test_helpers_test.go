package issueops

import (
	"agent-harness/internal/core/preflight"
	"os"
	"path/filepath"
	"testing"
)

func initIssueOpsRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	remote := t.TempDir()
	if code, _, stderr := preflight.GitCmd(remote, "init", "--bare", "-q"); code != 0 {
		t.Fatalf("git init bare failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "init", "-q", "-b", "main"); code != 0 {
		t.Fatalf("git init failed: %s", stderr)
	}
	for _, args := range [][]string{
		{"config", "user.name", "IssueOps Test"},
		{"config", "user.email", "issueops@example.test"},
		{"remote", "add", "origin", remote},
	} {
		if code, _, stderr := preflight.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	writeIssueOpsFile(t, repo, "README.md", "readme\n")
	writeIssueOpsFile(t, repo, "plans/demo.md", "plan\n")
	if code, _, stderr := preflight.GitCmd(repo, "add", "README.md", "plans/demo.md"); code != 0 {
		t.Fatalf("git add failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "commit", "-q", "-m", "initial"); code != 0 {
		t.Fatalf("git commit failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "-u", "origin", "main"); code != 0 {
		t.Fatalf("git push failed: %s", stderr)
	}
	return repo
}

func issueOpsWorktreePathForTest(repo, slug string) string {
	return filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", slug)
}

func makeIssueOpsWorktreeDirForTest(t *testing.T, repo, slug string) string {
	t.Helper()
	worktree := issueOpsWorktreePathForTest(repo, slug)
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git", "HEAD"), []byte("ref: refs/heads/"+slug+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return worktree
}

func writeIssueOpsFile(t *testing.T, repo, rel, content string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
