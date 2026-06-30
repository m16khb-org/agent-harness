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

func TestGitLabCreateChildDryRunDoesNotExecute(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	res, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{
		Repo:           t.TempDir(),
		ParentIssueURL: "https://gitlab.example.com/acme/repo/-/issues/12",
		Title:          "하위 작업",
		Body:           "details",
		Labels:         []string{"bug"},
		Assignees:      []string{"habin"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK || res.Provider != "gitlab" {
		t.Fatalf("result=%+v, want ok gitlab dry-run", res)
	}
	if res.ChildURL != "" || res.ChildNumber != "" || res.HierarchyVerified {
		t.Fatalf("dry-run must not populate remote result fields: %+v", res)
	}
	for _, want := range []string{"[dry-run]", "gitlab.example.com", "workItemCreate", "Task", "workItemHierarchyAddChildrenItems", "verify", "bug", "habin"} {
		if !strings.Contains(res.Preview, want) {
			t.Fatalf("preview %q missing %q", res.Preview, want)
		}
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

func TestGitLabCreateChildConfirmCreatesAttachesAndVerifies(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	logPath := filepath.Join(repo, "glab.calls")
	writeFakeGlab(t, binDir, `#!/bin/sh
printf '%s\n' "$*" >> glab.calls
case "$*" in
  *taskType*)
    printf '{"data":{"namespace":{"workItemTypes":{"nodes":[{"id":"gid://gitlab/WorkItems::Type/2","name":"Task"}]}}}}'
    exit 0
    ;;
  *labelLookup*)
    printf '{"data":{"project":{"labels":{"nodes":[{"id":"gid://gitlab/ProjectLabel/7","title":"bug"}]}}}}'
    exit 0
    ;;
  *userLookup*)
    printf '{"data":{"user":{"id":"gid://gitlab/User/9","username":"habin"}}}'
    exit 0
    ;;
  *workItemCreate*)
    printf '{"data":{"workItemCreate":{"workItem":{"id":"gid://gitlab/WorkItem/34","iid":"34","webUrl":"https://gitlab.com/acme/repo/-/work_items/34","labels":{"nodes":[{"title":"bug"}]},"assignees":{"nodes":[{"username":"habin"}]}}}}}'
    exit 0
    ;;
  *parentIid*)
    printf '{"data":{"project":{"issue":{"id":"gid://gitlab/WorkItem/12"}}}}'
    exit 0
    ;;
  *workItemHierarchyAddChildrenItems*)
    printf '{"data":{"workItemHierarchyAddChildrenItems":{"workItem":{"id":"gid://gitlab/WorkItem/12"},"errors":[]}}}'
    exit 0
    ;;
  *children*)
    printf '{"data":{"workItem":{"widgets":[{"type":"HIERARCHY","children":{"nodes":[{"id":"gid://gitlab/WorkItem/34","iid":"34","webUrl":"https://gitlab.com/acme/repo/-/work_items/34"}]}}]}}}'
    exit 0
    ;;
  *childVerify*)
    printf '{"data":{"workItem":{"iid":"34","webUrl":"https://gitlab.com/acme/repo/-/work_items/34","labels":{"nodes":[{"title":"bug"}]},"assignees":{"nodes":[{"username":"habin"}]}}}}'
    exit 0
    ;;
esac
echo "unexpected glab call: $*" >&2
exit 2
`)
	t.Setenv("PATH", binDir)

	got, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{
		Repo:           repo,
		ParentIssueURL: "https://gitlab.example.com/acme/repo/-/issues/12",
		Title:          "하위 작업",
		Body:           "details",
		Labels:         []string{"bug"},
		Assignees:      []string{"habin"},
		Confirm:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.Provider != "gitlab" || got.ChildURL != "https://gitlab.com/acme/repo/-/work_items/34" || got.ChildNumber != "34" || !got.HierarchyVerified {
		t.Fatalf("result=%+v", got)
	}
	if strings.Join(got.Labels, ",") != "bug" || strings.Join(got.Assignees, ",") != "habin" {
		t.Fatalf("verification labels/assignees not reflected: %+v", got)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	calls := strings.TrimSpace(string(log))
	for _, want := range []string{"--hostname gitlab.example.com", "taskType", "labelLookup", "userLookup", "workItemCreate", "parentIid", "workItemHierarchyAddChildrenItems", "children", "childVerify"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("calls missing %q:\n%s", want, calls)
		}
	}
}

func TestGitLabCreateChildFailureAfterCreateIncludesChildURL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	writeFakeGlab(t, binDir, `#!/bin/sh
case "$*" in
  *taskType*)
    printf '{"data":{"namespace":{"workItemTypes":{"nodes":[{"id":"gid://gitlab/WorkItems::Type/2","name":"Task"}]}}}}'
    exit 0
    ;;
  *labelLookup*)
    printf '{"data":{"project":{"labels":{"nodes":[{"id":"gid://gitlab/ProjectLabel/7","title":"bug"}]}}}}'
    exit 0
    ;;
  *userLookup*)
    printf '{"data":{"user":{"id":"gid://gitlab/User/9","username":"habin"}}}'
    exit 0
    ;;
  *workItemCreate*)
    printf '{"data":{"workItemCreate":{"workItem":{"id":"gid://gitlab/WorkItem/34","iid":"34","webUrl":"https://gitlab.example.com/acme/repo/-/work_items/34","labels":{"nodes":[{"title":"bug"}]},"assignees":{"nodes":[{"username":"habin"}]}}}}}'
    exit 0
    ;;
esac
echo "parent lookup failed" >&2
exit 2
`)
	t.Setenv("PATH", binDir)

	_, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{
		Repo:           repo,
		ParentIssueURL: "https://gitlab.example.com/acme/repo/-/issues/12",
		Title:          "하위 작업",
		Labels:         []string{"bug"},
		Assignees:      []string{"habin"},
		Confirm:        true,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "https://gitlab.example.com/acme/repo/-/work_items/34") {
		t.Fatalf("error=%q, want created child URL for cleanup", err)
	}
}

func TestGitLabCreateChildGraphQLFailureDoesNotFallbackToIssue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	logPath := filepath.Join(repo, "glab.calls")
	writeFakeGlab(t, binDir, `#!/bin/sh
printf '%s\n' "$*" >> glab.calls
echo "GraphQL: field not available" >&2
exit 2
`)
	t.Setenv("PATH", binDir)

	_, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{
		Repo:           repo,
		ParentIssueURL: "https://gitlab.com/acme/repo/-/issues/12",
		Title:          "하위 작업",
		Confirm:        true,
	})
	if err == nil || !strings.Contains(err.Error(), "glab graphql") {
		t.Fatalf("error=%v, want graphql failure", err)
	}
	log, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.Contains(string(log), "issue create") {
		t.Fatalf("must not fall back to sibling issue create, calls:\n%s", string(log))
	}
}

func TestGitLabCloseChildDryRunDoesNotExecute(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	res, err := NewProvider().CloseChild(port.IssueProviderCloseChildRequest{
		Repo:           t.TempDir(),
		ParentIssueURL: "https://gitlab.example.com/acme/repo/-/issues/12",
		ChildURL:       "https://gitlab.example.com/acme/repo/-/work_items/34",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK || res.Provider != "gitlab" || res.Closed || res.HierarchyVerified {
		t.Fatalf("unexpected dry-run result: %+v", res)
	}
	for _, want := range []string{"[dry-run]", "gitlab.example.com", "children", "workItemUpdate", "stateEvent: CLOSE", "work_items/34"} {
		if !strings.Contains(res.Preview, want) {
			t.Fatalf("preview %q missing %q", res.Preview, want)
		}
	}
}

func TestGitLabCloseChildConfirmVerifiesHierarchyClosesAndRechecksState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	logPath := filepath.Join(repo, "glab.calls")
	writeFakeGlab(t, binDir, `#!/bin/sh
printf '%s\n' "$*" >> glab.calls
case "$*" in
  *parentIid*)
    printf '{"data":{"project":{"issue":{"id":"gid://gitlab/WorkItem/12"}}}}'
    exit 0
    ;;
  *children*)
    printf '{"data":{"workItem":{"widgets":[{"type":"HIERARCHY","children":{"nodes":[{"id":"gid://gitlab/WorkItem/34","iid":"34","webUrl":"https://gitlab.example.com/acme/repo/-/work_items/34","state":"OPEN"}]}}]}}}'
    exit 0
    ;;
  *workItemUpdate*)
    printf '{"data":{"workItemUpdate":{"workItem":{"id":"gid://gitlab/WorkItem/34","iid":"34","webUrl":"https://gitlab.example.com/acme/repo/-/work_items/34","state":"CLOSED"},"errors":[]}}}'
    exit 0
    ;;
  *childCloseVerify*)
    printf '{"data":{"workItem":{"id":"gid://gitlab/WorkItem/34","iid":"34","webUrl":"https://gitlab.example.com/acme/repo/-/work_items/34","state":"CLOSED"}}}'
    exit 0
    ;;
esac
echo "unexpected glab call: $*" >&2
exit 2
`)
	t.Setenv("PATH", binDir)

	got, err := NewProvider().CloseChild(port.IssueProviderCloseChildRequest{
		Repo:           repo,
		ParentIssueURL: "https://gitlab.example.com/acme/repo/-/issues/12",
		ChildURL:       "https://gitlab.example.com/acme/repo/-/work_items/34",
		Confirm:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || !got.HierarchyVerified || !got.Closed || got.State != "CLOSED" {
		t.Fatalf("unexpected close result: %+v", got)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	calls := strings.TrimSpace(string(log))
	for _, want := range []string{"--hostname gitlab.example.com", "parentIid", "children", "workItemUpdate", "childCloseVerify"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("calls missing %q:\n%s", want, calls)
		}
	}
}

func TestGitLabCloseChildAlreadyClosedSkipsMutation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	logPath := filepath.Join(repo, "glab.calls")
	writeFakeGlab(t, binDir, `#!/bin/sh
printf '%s\n' "$*" >> glab.calls
case "$*" in
  *parentIid*)
    printf '{"data":{"project":{"issue":{"id":"gid://gitlab/WorkItem/12"}}}}'
    exit 0
    ;;
  *children*)
    printf '{"data":{"workItem":{"widgets":[{"type":"HIERARCHY","children":{"nodes":[{"id":"gid://gitlab/WorkItem/34","iid":"34","webUrl":"https://gitlab.example.com/acme/repo/-/work_items/34","state":"CLOSED"}]}}]}}}'
    exit 0
    ;;
  *childCloseVerify*)
    printf '{"data":{"workItem":{"id":"gid://gitlab/WorkItem/34","iid":"34","webUrl":"https://gitlab.example.com/acme/repo/-/work_items/34","state":"CLOSED"}}}'
    exit 0
    ;;
esac
echo "unexpected glab call: $*" >&2
exit 2
`)
	t.Setenv("PATH", binDir)

	got, err := NewProvider().CloseChild(port.IssueProviderCloseChildRequest{
		Repo:           repo,
		ParentIssueURL: "https://gitlab.example.com/acme/repo/-/issues/12",
		ChildURL:       "https://gitlab.example.com/acme/repo/-/work_items/34",
		Confirm:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.AlreadyClosed || !got.Closed {
		t.Fatalf("already closed child should succeed: %+v", got)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(log), "workItemUpdate") {
		t.Fatalf("already closed child should not be mutated:\n%s", log)
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

func TestParseGitLabIssueURLAcceptsSelfHostedCustomDomain(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantHost    string
		wantProject string
		wantIID     string
	}{
		{"gitlab.example.com", "https://gitlab.example.com/acme/repo/-/issues/12", "gitlab.example.com", "acme/repo", "12"},
		{"custom domain without gitlab substring", "https://code.company.com/group/proj/-/issues/5", "code.company.com", "group/proj", "5"},
		{"internal host with subgroups", "https://git.internal/group/subgroup/proj/-/issues/9", "git.internal", "group/subgroup/proj", "9"},
		{"scheme-less self-hosted (back-compat)", "gitlab.example.com/acme/repo/-/issues/12", "gitlab.example.com", "acme/repo", "12"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host, project, iid, err := parseGitLabIssueURL(tc.raw)
			if err != nil {
				t.Fatalf("parseGitLabIssueURL(%q) error: %v", tc.raw, err)
			}
			if host != tc.wantHost || project != tc.wantProject || iid != tc.wantIID {
				t.Fatalf("got host=%q project=%q iid=%q, want host=%q project=%q iid=%q", host, project, iid, tc.wantHost, tc.wantProject, tc.wantIID)
			}
		})
	}
}

func TestParseGitLabIssueURLRejectsNonIssue(t *testing.T) {
	if _, _, _, err := parseGitLabIssueURL("https://code.company.com/group/proj/-/work_items/5"); err == nil {
		t.Fatal("work item URL must not parse as a parent issue URL")
	}
	if _, _, _, err := parseGitLabIssueURL("https://github.com/acme/repo/issues/5"); err == nil {
		t.Fatal("GitHub issue URL must be rejected by the GitLab issue parser")
	}
}

func TestParseGitLabWorkItemURLAcceptsSelfHostedCustomDomain(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantHost    string
		wantProject string
		wantIID     string
	}{
		{"gitlab.example.com", "https://gitlab.example.com/acme/repo/-/work_items/34", "gitlab.example.com", "acme/repo", "34"},
		{"custom domain without gitlab substring", "https://code.company.com/group/proj/-/work_items/7", "code.company.com", "group/proj", "7"},
		{"internal host with subgroups", "https://git.internal/group/subgroup/proj/-/work_items/3", "git.internal", "group/subgroup/proj", "3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host, project, iid, err := parseGitLabWorkItemURL(tc.raw)
			if err != nil {
				t.Fatalf("parseGitLabWorkItemURL(%q) error: %v", tc.raw, err)
			}
			if host != tc.wantHost || project != tc.wantProject || iid != tc.wantIID {
				t.Fatalf("got host=%q project=%q iid=%q, want host=%q project=%q iid=%q", host, project, iid, tc.wantHost, tc.wantProject, tc.wantIID)
			}
		})
	}
}

func TestParseGitLabWorkItemURLRejectsNonWorkItem(t *testing.T) {
	if _, _, _, err := parseGitLabWorkItemURL("https://code.company.com/group/proj/-/issues/5"); err == nil {
		t.Fatal("issue URL must not parse as a work item URL")
	}
}
