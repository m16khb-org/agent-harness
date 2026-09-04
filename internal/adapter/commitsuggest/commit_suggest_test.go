package commitsuggest

import (
	commitsuggestcontract "issueops/internal/contract/commitsuggest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/preflight"
)

func TestSuggestCommitReturnsNoopWhenDiffIsEmpty(t *testing.T) {
	repo := initCommitSuggestRepo(t)
	result, err := SuggestCommit(commitsuggestcontract.CommitSuggestRequest{
		RepoRoot: repo,
		Staged:   false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Executed || result.RepoRoot != repo {
		t.Fatalf("empty diff result = %+v", result)
	}
}

func TestSuggestCommitRendersPromptForWorkingTreeDiff(t *testing.T) {
	repo := initCommitSuggestRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := SuggestCommit(commitsuggestcontract.CommitSuggestRequest{
		RepoRoot: repo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Executed {
		t.Fatalf("suggest result = %+v", result)
	}
	for _, want := range []string{"Git Diff", "commit_message", "changed"} {
		if !strings.Contains(result.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, result.Prompt)
		}
	}
}

func initCommitSuggestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.name", "Commit Suggest Test"},
		{"config", "user.email", "commit@example.test"},
	} {
		if code, _, stderr := preflight.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("initial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := preflight.GitCmd(repo, "add", "README.md"); code != 0 {
		t.Fatalf("git add failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "commit", "-q", "-m", "initial"); code != 0 {
		t.Fatalf("git commit failed: %s", stderr)
	}
	return repo
}
