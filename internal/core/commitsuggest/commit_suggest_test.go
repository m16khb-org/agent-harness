package commitsuggest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/preflight"
)

func TestSuggestCommitReturnsNoopWhenDiffIsEmpty(t *testing.T) {
	repo := initCommitSuggestRepo(t)
	result, err := SuggestCommit(CommitSuggestRequest{
		RepoRoot: repo,
		Staged:   false,
		Timeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Executed || result.RepoRoot != repo {
		t.Fatalf("empty diff result = %+v", result)
	}
}

func TestSuggestCommitUsesFakeAgyForWorkingTreeDiff(t *testing.T) {
	repo := initCommitSuggestRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	agy := writeCommitSuggestFakeAgy(t, `{"commit_message":"test(core): cover commit suggest\n\nLore:\n- Intent: Cover commit suggest.\n- Why: Characterization.\n- Changes:\n  - Use fake agy.\n- Verify: go test.\n- Risk: Low."}`)

	result, err := SuggestCommit(CommitSuggestRequest{
		RepoRoot:   repo,
		AgyCommand: agy,
		AgyModel:   "fake-model",
		Timeout:    10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Executed {
		t.Fatalf("suggest result = %+v", result)
	}
	if result.AgyCommand != agy || result.AgyModel != "fake-model" {
		t.Fatalf("agy metadata = %+v", result)
	}
	if !strings.HasPrefix(result.CommitMessage, "test(core): cover commit suggest") {
		t.Fatalf("commit message = %q", result.CommitMessage)
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

func writeCommitSuggestFakeAgy(t *testing.T, output string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "agy")
	if runtime.GOOS == "windows" {
		path += ".bat"
	}
	script := "#!/bin/sh\nprintf '%s\\n' " + quoteCommitSuggestShell(output) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func quoteCommitSuggestShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
