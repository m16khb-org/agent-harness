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

func TestGitHubCreateChildDryRunDoesNotExecute(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	res, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{
		Repo:           t.TempDir(),
		ParentIssueURL: "https://github.com/acme/repo/issues/12",
		Title:          "하위 작업",
		Body:           "details",
		Labels:         []string{"bug"},
		Assignees:      []string{"octocat"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK || res.Provider != "github" {
		t.Fatalf("result=%+v, want ok github dry-run", res)
	}
	if res.ChildURL != "" || res.ChildNumber != "" || res.HierarchyVerified {
		t.Fatalf("dry-run must not populate remote result fields: %+v", res)
	}
	for _, want := range []string{"[dry-run]", "gh issue create", "--repo acme/repo", "sub_issues", "sub_issue_id", "--label bug", "--assignee octocat"} {
		if !strings.Contains(res.Preview, want) {
			t.Fatalf("preview %q missing %q", res.Preview, want)
		}
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

func TestGitHubCreateChildConfirmCreatesAttachesAndVerifies(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	logPath := filepath.Join(repo, "gh.calls")
	writeFakeGh(t, binDir, `#!/bin/sh
printf '%s\n' "$*" >> gh.calls
if [ "$1 $2" = "issue create" ]; then
  printf 'https://github.com/acme/repo/issues/34\n'
  exit 0
fi
if [ "$1" = "api" ] && [ "$2" = "repos/acme/repo/issues/34" ]; then
  printf '{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34","labels":[{"name":"bug"}],"assignees":[{"login":"octocat"}]}'
  exit 0
fi
if [ "$1 $2" = "api -X" ] && [ "$3" = "POST" ]; then
  printf '{"ok":true}'
  exit 0
fi
if [ "$1" = "api" ] && [ "$2" = "repos/acme/repo/issues/12/sub_issues" ]; then
  printf '[{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34"}]'
  exit 0
fi
echo "unexpected gh call: $*" >&2
exit 2
`)
	t.Setenv("PATH", binDir)

	got, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{
		Repo:           repo,
		ParentIssueURL: "https://github.com/acme/repo/issues/12",
		Title:          "하위 작업",
		Body:           "details",
		Labels:         []string{"bug"},
		Assignees:      []string{"octocat"},
		Confirm:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.Provider != "github" || got.ChildURL != "https://github.com/acme/repo/issues/34" || got.ChildNumber != "34" || !got.HierarchyVerified {
		t.Fatalf("result=%+v", got)
	}
	if strings.Join(got.Labels, ",") != "bug" || strings.Join(got.Assignees, ",") != "octocat" {
		t.Fatalf("verification labels/assignees not reflected: %+v", got)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	calls := strings.TrimSpace(string(log))
	for _, want := range []string{
		"issue create --title 하위 작업 --body details --label bug --assignee octocat --repo acme/repo",
		"api repos/acme/repo/issues/34",
		"api -X POST repos/acme/repo/issues/12/sub_issues -f sub_issue_id=987",
		"api repos/acme/repo/issues/12/sub_issues",
	} {
		if !strings.Contains(calls, want) {
			t.Fatalf("calls missing %q:\n%s", want, calls)
		}
	}
}

func TestGitHubCreateChildFailureAfterCreateIncludesChildURL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	writeFakeGh(t, binDir, `#!/bin/sh
if [ "$1 $2" = "issue create" ]; then
  printf 'https://github.com/acme/repo/issues/34\n'
  exit 0
fi
echo "lookup failed" >&2
exit 2
`)
	t.Setenv("PATH", binDir)

	_, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{
		Repo:           repo,
		ParentIssueURL: "https://github.com/acme/repo/issues/12",
		Title:          "하위 작업",
		Labels:         []string{"bug"},
		Assignees:      []string{"octocat"},
		Confirm:        true,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "https://github.com/acme/repo/issues/34") {
		t.Fatalf("error=%q, want created child URL for cleanup", err)
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
