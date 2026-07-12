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
	for _, want := range []string{"[dry-run]", "gh issue create", "--parent 12", "--repo acme/repo", "fallback", "sub_issues", "sub_issue_id", "--label bug", "--assignee octocat"} {
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
		Draft:      true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Preview, "pr create") || !strings.Contains(res.Preview, "--head feat/x") || !strings.Contains(res.Preview, "--draft") {
		t.Errorf("preview missing expected args: %q", res.Preview)
	}
}

func TestGitHubCreatePullRequestConfirmPassesDraftArgv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	writeFakeGh(t, binDir, `#!/bin/sh
if [ "$1 $2" = "pr create" ]; then
  printf '%s\n' "$@" > gh.argv
  printf 'create\n' >> gh.calls
  printf 'https://github.com/acme/repo/pull/16\n'
  exit 0
fi
if [ "$1 $2" = "pr view" ]; then
  printf 'view\n' >> gh.calls
  printf '{"url":"https://github.com/acme/repo/pull/16","headRefName":"feat/x","baseRefName":"main","isDraft":true,"labels":[{"name":"bug"},{"name":"extra"}],"assignees":[{"login":"octocat"},{"login":"extra"}]}'
  exit 0
fi
exit 2
`)
	t.Setenv("PATH", binDir)
	if _, err := NewProvider().CreatePullRequest(port.IssueProviderCreatePullRequestRequest{Repo: repo, Title: "PR", HeadBranch: "feat/x", BaseBranch: "main", Labels: []string{"bug"}, Assignees: []string{"octocat"}, Draft: true, Confirm: true}); err != nil {
		t.Fatal(err)
	}
	argv, err := os.ReadFile(filepath.Join(repo, "gh.argv"))
	if err != nil || !strings.Contains(string(argv), "\n--draft\n") {
		t.Fatalf("GitHub draft argv missing: %q err=%v", argv, err)
	}
	if calls, err := os.ReadFile(filepath.Join(repo, "gh.calls")); err != nil || string(calls) != "create\nview\n" {
		t.Fatalf("GitHub create/readback calls = %q err=%v", calls, err)
	}
}

func TestGitHubCreatePullRequestReadbackMismatchNeedsReconciliationWithoutRetry(t *testing.T) {
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGh(t, binDir, `#!/bin/sh
if [ "$1 $2" = "pr create" ]; then printf 'create\n' >> gh.calls; printf 'https://github.com/acme/repo/pull/16\n'; exit 0; fi
if [ "$1 $2" = "pr view" ]; then printf 'view\n' >> gh.calls; printf '{"url":"https://github.com/acme/repo/pull/16","headRefName":"feat/x","baseRefName":"wrong","isDraft":true,"labels":[],"assignees":[]}'; exit 0; fi
exit 2
`)
	t.Setenv("PATH", binDir)
	result, err := NewProvider().CreatePullRequest(port.IssueProviderCreatePullRequestRequest{Repo: repo, Title: "PR", HeadBranch: "feat/x", BaseBranch: "main", Draft: true, Confirm: true})
	if err == nil || result.URL != "https://github.com/acme/repo/pull/16" || !strings.Contains(err.Error(), result.URL) || !strings.Contains(err.Error(), "needs reconciliation") {
		t.Fatalf("GitHub mismatch result=%+v err=%v", result, err)
	}
	if calls, readErr := os.ReadFile(filepath.Join(repo, "gh.calls")); readErr != nil || string(calls) != "create\nview\n" {
		t.Fatalf("GitHub mismatch retried creation: %q err=%v", calls, readErr)
	}
}

func TestGitHubCreatePullRequestRejectsSecretCreatedURLWithoutViewOrRetry(t *testing.T) {
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGh(t, binDir, `#!/bin/sh
if [ "$1 $2" = "pr create" ]; then printf 'create\n' >> gh.calls; printf '%s\n' 'api_key=abcdefghijklmnopqrstuvwxyz123456'; exit 0; fi
printf 'view\n' >> gh.calls
exit 2
`)
	t.Setenv("PATH", binDir)
	result, err := NewProvider().CreatePullRequest(port.IssueProviderCreatePullRequestRequest{Repo: repo, Title: "PR", HeadBranch: "feat/x", BaseBranch: "main", Draft: true, Confirm: true})
	if err == nil || result.URL != "" || !strings.Contains(err.Error(), "needs reconciliation") || strings.Contains(err.Error(), "abcdefghijklmnopqrstuvwxyz123456") {
		t.Fatalf("unsafe GitHub URL result=%+v err=%v", result, err)
	}
	if calls, readErr := os.ReadFile(filepath.Join(repo, "gh.calls")); readErr != nil || string(calls) != "create\n" {
		t.Fatalf("unsafe GitHub URL reached view or retry: %q err=%v", calls, readErr)
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
  case "$*" in
    *"--parent 12"*)
      printf 'https://github.com/acme/repo/issues/34\n'
      exit 0
      ;;
  esac
  echo "missing preferred parent flag" >&2
  exit 2
fi
if [ "$1" = "api" ] && [ "$2" = "repos/acme/repo/issues/34" ]; then
  printf '{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34","labels":[{"name":"bug"}],"assignees":[{"login":"octocat"}]}'
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
		"issue create --title 하위 작업 --body details --label bug --assignee octocat --repo acme/repo --parent 12",
		"api repos/acme/repo/issues/34",
		"api repos/acme/repo/issues/12/sub_issues",
	} {
		if !strings.Contains(calls, want) {
			t.Fatalf("calls missing %q:\n%s", want, calls)
		}
	}
	if strings.Contains(calls, "sub_issue_id=987") {
		t.Fatalf("preferred --parent path must not attach with REST fallback:\n%s", calls)
	}
}

func TestGitHubCreateChildFallsBackToRESTAttachWhenParentFlagFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	logPath := filepath.Join(repo, "gh.calls")
	writeFakeGh(t, binDir, `#!/bin/sh
printf '%s\n' "$*" >> gh.calls
if [ "$1 $2" = "issue create" ]; then
  case "$*" in
    *"--parent 12"*)
      echo "unknown flag: --parent" >&2
      exit 2
      ;;
  esac
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
		"issue create --title 하위 작업 --body details --label bug --assignee octocat --repo acme/repo --parent 12",
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

func TestGitHubCloseChildDryRunDoesNotExecute(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	res, err := NewProvider().CloseChild(port.IssueProviderCloseChildRequest{
		Repo:           t.TempDir(),
		ParentIssueURL: "https://github.com/acme/repo/issues/12",
		ChildURL:       "https://github.com/acme/repo/issues/34",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK || res.Provider != "github" || res.Closed || res.HierarchyVerified {
		t.Fatalf("unexpected dry-run result: %+v", res)
	}
	for _, want := range []string{"[dry-run]", "sub_issues", "issues/34", "state=closed", "state_reason=completed"} {
		if !strings.Contains(res.Preview, want) {
			t.Fatalf("preview %q missing %q", res.Preview, want)
		}
	}
}

func TestGitHubCloseChildConfirmVerifiesHierarchyClosesAndRechecksState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	logPath := filepath.Join(repo, "gh.calls")
	writeFakeGh(t, binDir, `#!/bin/sh
printf '%s\n' "$*" >> gh.calls
if [ "$1" = "api" ] && [ "$2" = "repos/acme/repo/issues/12/sub_issues" ]; then
  printf '[{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34","state":"open"}]'
  exit 0
fi
if [ "$1 $2" = "api -X" ] && [ "$3" = "PATCH" ]; then
  printf '{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34","state":"closed"}'
  exit 0
fi
if [ "$1" = "api" ] && [ "$2" = "repos/acme/repo/issues/34" ]; then
  printf '{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34","state":"closed"}'
  exit 0
fi
echo "unexpected gh call: $*" >&2
exit 2
`)
	t.Setenv("PATH", binDir)

	got, err := NewProvider().CloseChild(port.IssueProviderCloseChildRequest{
		Repo:           repo,
		ParentIssueURL: "https://github.com/acme/repo/issues/12",
		ChildURL:       "https://github.com/acme/repo/issues/34",
		Confirm:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || !got.HierarchyVerified || !got.Closed || got.State != "closed" {
		t.Fatalf("unexpected close result: %+v", got)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	calls := strings.TrimSpace(string(log))
	for _, want := range []string{
		"api repos/acme/repo/issues/12/sub_issues",
		"api -X PATCH repos/acme/repo/issues/34 -f state=closed -f state_reason=completed",
		"api repos/acme/repo/issues/34",
	} {
		if !strings.Contains(calls, want) {
			t.Fatalf("calls missing %q:\n%s", want, calls)
		}
	}
}

func TestGitHubCloseChildRejectsHierarchyMismatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	writeFakeGh(t, binDir, `#!/bin/sh
if [ "$1" = "api" ] && [ "$2" = "repos/acme/repo/issues/12/sub_issues" ]; then
  printf '[]'
  exit 0
fi
echo "unexpected gh call: $*" >&2
exit 2
`)
	t.Setenv("PATH", binDir)

	_, err := NewProvider().CloseChild(port.IssueProviderCloseChildRequest{
		Repo:           t.TempDir(),
		ParentIssueURL: "https://github.com/acme/repo/issues/12",
		ChildURL:       "https://github.com/acme/repo/issues/34",
		Confirm:        true,
	})
	if err == nil || !strings.Contains(err.Error(), "hierarchy verification failed") {
		t.Fatalf("expected hierarchy mismatch, got %v", err)
	}
}

func TestParseGhOutput(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantURL string
	}{
		{"empty", "", ""},
		{"plain url", "https://github.com/o/r/issues/12\n", "https://github.com/o/r/issues/12"},
		// gh create never passes --json, so JSON is never emitted; any non-URL
		// output is taken verbatim as the URL (the IID/number branch is gone).
		{"json no longer parsed", `{"url":"https://github.com/o/r/pull/7","number":"7"}`, `{"url":"https://github.com/o/r/pull/7","number":"7"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseGhOutput(tc.in)
			if got.URL != tc.wantURL {
				t.Errorf("got %+v, want url=%q", got, tc.wantURL)
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
printf 'https://github.com/o/r/issues/12\n'
`)
	t.Setenv("PATH", binDir)

	got, err := runGhJSON([]string{"issue", "create", "--title", "Fix"}, repo, "issue")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://github.com/o/r/issues/12" {
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

func TestGitHubUpdateIssueBodySectionDryRun(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	res, err := NewProvider().UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest{
		IssueURL: "https://github.com/acme/repo/issues/12",
		Findings: []string{"gold-plating"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK || res.Updated {
		t.Fatalf("dry-run must not update: %+v", res)
	}
	if !strings.Contains(res.Preview, "gh issue edit") || !strings.Contains(res.Preview, "gh issue view") {
		t.Fatalf("preview missing expected commands: %q", res.Preview)
	}
}

func TestGitHubUpdateIssueBodySectionConfirmMergesIdempotently(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	logPath := filepath.Join(repo, "gh.edit")
	writeFakeGh(t, binDir, `#!/bin/sh
if [ "$1 $2" = "issue view" ]; then
  printf '{"body":"original body\\n\\n<!-- issueops:devils-advocate:start -->\\nstale-finding\\n<!-- issueops:devils-advocate:end -->\\n"}'
  exit 0
fi
if [ "$1 $2" = "issue edit" ]; then
  printf '%s' "$*" > gh.edit
  exit 0
fi
echo "unexpected gh call: $*" >&2
exit 2
`)
	t.Setenv("PATH", binDir)

	res, err := NewProvider().UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest{
		Repo:     repo,
		IssueURL: "https://github.com/acme/repo/issues/12",
		Findings: []string{"gold-plating", "schedule optimism"},
		Confirm:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !res.Updated {
		t.Fatalf("confirm should update: %+v", res)
	}
	edited, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(edited)
	if !strings.Contains(body, "original body") {
		t.Fatalf("surrounding body must round-trip: %q", body)
	}
	if strings.Contains(body, "stale-finding") {
		t.Fatalf("stale managed section must be replaced, not kept: %q", body)
	}
	if strings.Count(body, "issueops:devils-advocate:start") != 1 {
		t.Fatalf("managed section must not be duplicated: %q", body)
	}
	if !strings.Contains(body, "gold-plating") || !strings.Contains(body, "schedule optimism") {
		t.Fatalf("findings missing from edited body: %q", body)
	}
}

func writeFakeGh(t *testing.T, binDir, script string) {
	t.Helper()
	path := filepath.Join(binDir, "gh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
