package commitsuggest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/externalllm"
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

func TestSuggestCommitUsesZAIAPIForWorkingTreeDiff(t *testing.T) {
	repo := initCommitSuggestRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Mock Z.AI API
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"commit_message\":\"test(core): cover commit suggest\\n\\nLore:\\n- Intent: Cover commit suggest.\\n- Why: Characterization.\\n- Changes:\\n  - Use fake Z.AI.\\n- Verify: go test.\\n- Risk: Low.\"}"}}]}`)
	}))
	defer ts.Close()

	// Override the externalllm package baseURL
	origBaseURL := externalllm.SetBaseURL(ts.URL)
	defer externalllm.SetBaseURL(origBaseURL)

	t.Setenv("Z_AI_API_KEY", "test-key")

	result, err := SuggestCommit(CommitSuggestRequest{
		RepoRoot: repo,
		Model:    "glm-5-turbo",
		Timeout:  10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Executed {
		t.Fatalf("suggest result = %+v", result)
	}
	if !strings.HasPrefix(result.CommitMessage, "test(core): cover commit suggest") {
		t.Fatalf("commit message = %q", result.CommitMessage)
	}
	if result.Model != "glm-5-turbo" {
		t.Fatalf("model = %q", result.Model)
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
