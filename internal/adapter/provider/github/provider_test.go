package github

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func TestGitHubProviderName(t *testing.T) {
	if got := NewProvider().Name(); got != "github" {
		t.Fatalf("Name()=%q, want github", got)
	}
}

func TestGitHubCreateIssueRequiresTitle(t *testing.T) {
	_, err := NewProvider().CreateIssue(port.IssueProviderCreateIssueRequest{Title: "  "})
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestGitHubCreateIssueDryRunDoesNotExecute(t *testing.T) {
	res, err := NewProvider().CreateIssue(port.IssueProviderCreateIssueRequest{
		Title:  "Fix bug",
		Body:   "details",
		Labels: []string{"bug"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK {
		t.Fatal("expected OK dry-run result")
	}
	if !strings.HasPrefix(res.Preview, "[dry-run]") {
		t.Errorf("expected dry-run preview, got %q", res.Preview)
	}
	if !strings.Contains(res.Preview, "issue create") || !strings.Contains(res.Preview, "--label bug") {
		t.Errorf("preview missing expected args: %q", res.Preview)
	}
	if res.URL != "" || res.Number != "" {
		t.Error("dry-run must not populate URL/Number")
	}
}

func TestGitHubCreatePullRequestRequiresBranches(t *testing.T) {
	_, err := NewProvider().CreatePullRequest(port.IssueProviderCreatePullRequestRequest{Title: "PR"})
	if err == nil {
		t.Fatal("expected error for missing head/base branches")
	}
}

func TestGitHubCreatePullRequestDryRun(t *testing.T) {
	res, err := NewProvider().CreatePullRequest(port.IssueProviderCreatePullRequestRequest{
		Title:      "Add feature",
		HeadBranch: "feat/x",
		BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Preview, "pr create") || !strings.Contains(res.Preview, "--head feat/x") {
		t.Errorf("preview missing expected args: %q", res.Preview)
	}
}

func TestParseGhOutput(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantURL    string
		wantNumber string
	}{
		{"empty", "", "", ""},
		{"plain url", "https://github.com/o/r/issues/12\n", "https://github.com/o/r/issues/12", ""},
		{"json", `{"url":"https://github.com/o/r/pull/7","number":"7"}`, "https://github.com/o/r/pull/7", "7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGhOutput(tc.in, "issue")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.URL != tc.wantURL || got.Number != tc.wantNumber {
				t.Errorf("got %+v, want url=%q number=%q", got, tc.wantURL, tc.wantNumber)
			}
		})
	}
}

func TestRunGhJSONReportsMissingCLI(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := runGhJSON([]string{"issue", "create"}, "", "issue")
	if err == nil || !strings.Contains(err.Error(), "gh CLI is not installed") {
		t.Fatalf("error=%v, want missing gh CLI", err)
	}
}

func TestRunGhJSONExecutesInRepoAndParsesOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	logPath := filepath.Join(repo, "gh.args")
	writeFakeGh(t, binDir, `#!/bin/sh
printf '%s\n' "$PWD|$*" > gh.args
printf '{"url":"https://github.com/o/r/issues/12","number":"12"}'
`)
	t.Setenv("PATH", binDir)

	got, err := runGhJSON([]string{"issue", "create", "--title", "Fix"}, repo, "issue")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://github.com/o/r/issues/12" || got.Number != "12" {
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

func TestRunGhJSONReportsExitStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	writeFakeGh(t, binDir, `#!/bin/sh
echo "provider rejected request" >&2
exit 2
`)
	t.Setenv("PATH", binDir)

	_, err := runGhJSON([]string{"pr", "create"}, "", "pr")
	if err == nil || !strings.Contains(err.Error(), "gh pr create failed: provider rejected request") {
		t.Fatalf("error=%v, want stderr failure", err)
	}
}

func writeFakeGh(t *testing.T, binDir, script string) {
	t.Helper()
	path := filepath.Join(binDir, "gh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
