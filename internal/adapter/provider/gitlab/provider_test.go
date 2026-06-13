package gitlab

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func TestGitLabProviderName(t *testing.T) {
	if got := NewProvider().Name(); got != "gitlab" {
		t.Fatalf("Name()=%q, want gitlab", got)
	}
}

func TestGitLabCreateIssueRequiresTitle(t *testing.T) {
	_, err := NewProvider().CreateIssue(port.IssueProviderCreateIssueRequest{Title: ""})
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestGitLabCreateIssueDryRun(t *testing.T) {
	res, err := NewProvider().CreateIssue(port.IssueProviderCreateIssueRequest{
		Title: "Fix bug",
		Body:  "details",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(res.Preview, "[dry-run]") {
		t.Errorf("expected dry-run preview, got %q", res.Preview)
	}
	if !strings.Contains(res.Preview, "issue create") || !strings.Contains(res.Preview, "--description details") {
		t.Errorf("preview missing expected args: %q", res.Preview)
	}
}

func TestGitLabCreateMRRequiresBranches(t *testing.T) {
	_, err := NewProvider().CreatePullRequest(port.IssueProviderCreatePullRequestRequest{Title: "MR"})
	if err == nil {
		t.Fatal("expected error for missing source/target branches")
	}
}

func TestGitLabCreateMRDryRun(t *testing.T) {
	res, err := NewProvider().CreatePullRequest(port.IssueProviderCreatePullRequestRequest{
		Title:      "Add feature",
		HeadBranch: "feat/x",
		BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Preview, "mr create") || !strings.Contains(res.Preview, "--source-branch feat/x") || !strings.Contains(res.Preview, "--target-branch main") {
		t.Errorf("preview missing expected args: %q", res.Preview)
	}
}

func TestParseGlabOutput(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantURL    string
		wantNumber string
	}{
		{"empty", "", "", ""},
		{"plain url", "creating...\nhttps://gitlab.com/g/p/-/issues/9\n", "https://gitlab.com/g/p/-/issues/9", ""},
		{"json with iid", `{"web_url":"https://gitlab.com/g/p/-/issues/9","iid":9}`, "https://gitlab.com/g/p/-/issues/9", "9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url, number := parseGlabOutput(tc.in, "issue")
			if url != tc.wantURL || number != tc.wantNumber {
				t.Errorf("got url=%q number=%q, want url=%q number=%q", url, number, tc.wantURL, tc.wantNumber)
			}
		})
	}
}

func TestRunGlabJSONReportsMissingCLI(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := runGlabJSON([]string{"issue", "create"}, "", "issue")
	if err == nil || !strings.Contains(err.Error(), "glab CLI is not installed") {
		t.Fatalf("error=%v, want missing glab CLI", err)
	}

	_, mrErr := runGlabMRJSON([]string{"mr", "create"}, "")
	if mrErr == nil || !strings.Contains(mrErr.Error(), "glab CLI is not installed") {
		t.Fatalf("mr error=%v, want missing glab CLI", mrErr)
	}
}

func TestRunGlabJSONExecutesInRepoAndParsesOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	logPath := filepath.Join(repo, "glab.args")
	writeFakeGlab(t, binDir, `#!/bin/sh
printf '%s\n' "$PWD|$*" > glab.args
printf '{"web_url":"https://gitlab.com/g/p/-/issues/9","iid":9}'
`)
	t.Setenv("PATH", binDir)

	got, err := runGlabJSON([]string{"issue", "create", "--title", "Fix"}, repo, "issue")
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.URL != "https://gitlab.com/g/p/-/issues/9" || got.Number != "9" {
		t.Fatalf("result=%+v", got)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := repo + "|issue create --title Fix"; strings.TrimSpace(string(log)) != want {
		t.Fatalf("logged command=%q, want %q", strings.TrimSpace(string(log)), want)
	}
}

func TestRunGlabMRJSONExecutesAndReportsStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	writeFakeGlab(t, binDir, `#!/bin/sh
if [ "$1" = "mr" ]; then
  printf '{"web_url":"https://gitlab.com/g/p/-/merge_requests/4","iid":4}'
  exit 0
fi
echo "provider rejected request" >&2
exit 2
`)
	t.Setenv("PATH", binDir)

	mr, err := runGlabMRJSON([]string{"mr", "create"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !mr.OK || mr.URL != "https://gitlab.com/g/p/-/merge_requests/4" || mr.Number != "4" {
		t.Fatalf("mr result=%+v", mr)
	}

	_, issueErr := runGlabJSON([]string{"issue", "create"}, "", "issue")
	if issueErr == nil || !strings.Contains(issueErr.Error(), "glab issue create failed: provider rejected request") {
		t.Fatalf("issue error=%v, want stderr failure", issueErr)
	}
}

func writeFakeGlab(t *testing.T, binDir, script string) {
	t.Helper()
	path := filepath.Join(binDir, "glab")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
