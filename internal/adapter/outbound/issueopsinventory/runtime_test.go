package issueopsinventory

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCleanPathNormalizesLinkedWorktreeToSourceCheckout(t *testing.T) {
	source := filepath.Join(t.TempDir(), "repo")
	runGit(t, "", "init", "-b", "main", source)
	runGit(t, source, "config", "user.email", "issueops@example.test")
	runGit(t, source, "config", "user.name", "IssueOps Test")
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, source, "add", "README.md")
	runGit(t, source, "commit", "-m", "fixture")

	worktree := filepath.Join(filepath.Dir(source), filepath.Base(source)+".worktrees", "69-v1")
	runGit(t, source, "worktree", "add", "-b", "69-v1", worktree)

	paths := CleanPath{}
	if got := paths.Normalize(source); got != filepath.Clean(source) {
		t.Fatalf("source checkout normalized to %q, want lexical path %q", got, source)
	}
	if got, want := paths.Normalize(worktree), paths.Normalize(source); got != want {
		t.Fatalf("linked worktree normalized to %q, want source checkout %q", got, want)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	if dir != "" {
		command.Dir = dir
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
