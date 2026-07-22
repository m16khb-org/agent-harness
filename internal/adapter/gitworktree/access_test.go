package gitworktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

func TestProbeAccessReturnsHostSpecificRelaunchWithoutWorktreeMutation(t *testing.T) {
	repo := initAccessRepo(t)
	base := repo + ".worktrees"
	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(base, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(base, 0o700) })
	req := port.ExecutionWorkspaceRequest{
		LifecycleID: "io-69", SourceRoot: repo, Root: filepath.Join(base, "69-access"),
		Branch: "69-access", BaseBranch: "main", BaseHead: preflight.GitOut(repo, "rev-parse", "HEAD"), Confirm: true,
	}
	for _, host := range []string{"codex", "claude"} {
		got, err := New().ProbeAccess(context.Background(), req, host)
		if err != nil {
			t.Fatal(err)
		}
		if got.Allowed || got.Code != "canonical_worktree_base_inaccessible" || !strings.HasPrefix(got.RelaunchCommand, host+" ") {
			t.Fatalf("unexpected %s access result: %#v", host, got)
		}
	}
	if err := os.Chmod(base, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(req.Root); !os.IsNotExist(err) {
		t.Fatalf("access failure created a partial worktree: %v", err)
	}
}

func initAccessRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"config", "user.email", "test@example.invalid"}, {"config", "user.name", "Test"}} {
		if code, _, stderr := preflight.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v: %s", args, stderr)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := preflight.GitCmd(repo, "add", "README.md"); code != 0 {
		t.Fatal(stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "commit", "-q", "-m", "fixture"); code != 0 {
		t.Fatal(stderr)
	}
	return repo
}
