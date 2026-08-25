package issueops

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIssueOpsStartLockIDTreatsLinkedWorktreeAsSourceCheckout(t *testing.T) {
	source := filepath.Join(t.TempDir(), "repo")
	runIdentityGit(t, "", "init", "-b", "main", source)
	runIdentityGit(t, source, "config", "user.email", "issueops@example.test")
	runIdentityGit(t, source, "config", "user.name", "IssueOps Test")
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runIdentityGit(t, source, "add", "README.md")
	runIdentityGit(t, source, "commit", "-m", "fixture")

	worktree := filepath.Join(filepath.Dir(source), filepath.Base(source)+".worktrees", "69-v1")
	runIdentityGit(t, source, "worktree", "add", "-b", "69-v1", worktree)

	sourceID := issueOpsStartLockID(source, "69-v1")
	worktreeID := issueOpsStartLockID(worktree, "69-v1")
	if worktreeID != sourceID {
		t.Fatalf("worktree lock id=%q, want source lock id=%q", worktreeID, sourceID)
	}
}

func runIdentityGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	if dir != "" {
		command.Dir = dir
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
