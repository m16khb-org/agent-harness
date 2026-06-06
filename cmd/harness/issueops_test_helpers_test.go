package main

import (
	"os"
	"path/filepath"
	"testing"

	"agent-harness/cmd/harness/issueopscli"
)

func makeIssueOpsCLIRepoForTest(t *testing.T, name string) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	return repo
}

func makeIssueOpsCLIWorktreeForTest(t *testing.T, repo, slug string) string {
	t.Helper()
	worktree := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", slug)
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git", "HEAD"), []byte("ref: refs/heads/"+slug+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return worktree
}

func stubIssueOpsChildIssueVerifier(t *testing.T, verifier func(string) error) {
	t.Helper()
	previous := issueopscli.SetChildIssueVerifier(verifier)
	t.Cleanup(func() {
		issueopscli.SetChildIssueVerifier(previous)
	})
}
