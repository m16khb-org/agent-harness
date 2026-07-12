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
		Draft:      true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Preview, "mr create") || !strings.Contains(res.Preview, "--source-branch feat/x") || !strings.Contains(res.Preview, "--target-branch main") || !strings.Contains(res.Preview, "--draft") || !strings.Contains(res.Preview, "--yes") {
		t.Errorf("preview missing expected args: %q", res.Preview)
	}
	if strings.Contains(res.Preview, "--push") || strings.Contains(res.Preview, "--fill") {
		t.Errorf("preview gained forbidden implicit mutation args: %q", res.Preview)
	}
}

func TestGitLabCreateMRConfirmPassesDraftArgv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	writeFakeGlab(t, binDir, `#!/bin/sh
if [ "$1 $2" = "mr create" ]; then
  printf '%s\n' "$@" > glab.argv
  printf 'create\n' >> glab.calls
  printf 'https://gitlab.com/acme/repo/-/merge_requests/16\n'
  exit 0
fi
if [ "$1" = "api" ]; then
  printf 'view\n' >> glab.calls
	printf '%s\n' "$@" > glab.api.argv
  printf '{"web_url":"https://gitlab.com/acme/repo/-/merge_requests/16","source_branch":"feat/x","target_branch":"main","draft":true,"labels":["bug","extra"],"assignees":[{"username":"habin"},{"username":"extra"}]}'
  exit 0
fi
exit 2
`)
	t.Setenv("PATH", binDir)
	if _, err := NewProvider().CreatePullRequest(port.IssueProviderCreatePullRequestRequest{Repo: repo, Title: "MR", HeadBranch: "feat/x", BaseBranch: "main", Labels: []string{"bug"}, Assignees: []string{"habin"}, Draft: true, Confirm: true}); err != nil {
		t.Fatal(err)
	}
	argv, err := os.ReadFile(filepath.Join(repo, "glab.argv"))
	if err != nil || !strings.Contains(string(argv), "\n--draft\n") || !strings.Contains(string(argv), "\n--yes\n") || strings.Contains(string(argv), "\n--push\n") || strings.Contains(string(argv), "\n--fill\n") {
		t.Fatalf("GitLab draft argv missing: %q err=%v", argv, err)
	}
	if calls, err := os.ReadFile(filepath.Join(repo, "glab.calls")); err != nil || string(calls) != "create\nview\n" {
		t.Fatalf("GitLab create/readback calls = %q err=%v", calls, err)
	}
	apiArgv, err := os.ReadFile(filepath.Join(repo, "glab.api.argv"))
	wantAPI := "api\nprojects/acme%2Frepo/merge_requests/16\n--hostname\ngitlab.com\n"
	if err != nil || string(apiArgv) != wantAPI {
		t.Fatalf("GitLab readback argv = %q, want %q, err=%v", apiArgv, wantAPI, err)
	}
}

func TestGitLabCreateMRReadbackMismatchNeedsReconciliationWithoutRetry(t *testing.T) {
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGlab(t, binDir, `#!/bin/sh
if [ "$1 $2" = "mr create" ]; then printf 'create\n' >> glab.calls; printf 'https://gitlab.com/acme/repo/-/merge_requests/16\n'; exit 0; fi
if [ "$1" = "api" ]; then printf 'view\n' >> glab.calls; printf '{"web_url":"https://gitlab.com/acme/repo/-/merge_requests/16","source_branch":"feat/x","target_branch":"wrong","draft":true,"labels":[],"assignees":[]}'; exit 0; fi
exit 2
`)
	t.Setenv("PATH", binDir)
	result, err := NewProvider().CreatePullRequest(port.IssueProviderCreatePullRequestRequest{Repo: repo, Title: "MR", HeadBranch: "feat/x", BaseBranch: "main", Draft: true, Confirm: true})
	if err == nil || result.URL != "https://gitlab.com/acme/repo/-/merge_requests/16" || !strings.Contains(err.Error(), result.URL) || !strings.Contains(err.Error(), "needs reconciliation") {
		t.Fatalf("GitLab mismatch result=%+v err=%v", result, err)
	}
	if calls, readErr := os.ReadFile(filepath.Join(repo, "glab.calls")); readErr != nil || string(calls) != "create\nview\n" {
		t.Fatalf("GitLab mismatch retried creation: %q err=%v", calls, readErr)
	}
}

func TestGitLabCreateMRRejectsSecretCreatedURLWithoutAPIOrRetry(t *testing.T) {
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGlab(t, binDir, `#!/bin/sh
if [ "$1 $2" = "mr create" ]; then printf 'create\n' >> glab.calls; printf '%s\n' 'api_key=abcdefghijklmnopqrstuvwxyz123456'; exit 0; fi
printf 'api\n' >> glab.calls
exit 2
`)
	t.Setenv("PATH", binDir)
	result, err := NewProvider().CreatePullRequest(port.IssueProviderCreatePullRequestRequest{Repo: repo, Title: "MR", HeadBranch: "feat/x", BaseBranch: "main", Draft: true, Confirm: true})
	if err == nil || result.URL != "" || !strings.Contains(err.Error(), "needs reconciliation") || strings.Contains(err.Error(), "abcdefghijklmnopqrstuvwxyz123456") {
		t.Fatalf("unsafe GitLab URL result=%+v err=%v", result, err)
	}
	if calls, readErr := os.ReadFile(filepath.Join(repo, "glab.calls")); readErr != nil || string(calls) != "create\n" {
		t.Fatalf("unsafe GitLab URL reached API or retry: %q err=%v", calls, readErr)
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
		// glab create never emits JSON, so a JSON line is not an https line and is
		// ignored; the IID/number branch was removed, so number is always empty.
		{"json no longer parsed", `{"web_url":"https://gitlab.com/g/p/-/issues/9","iid":9}`, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url, number := parseGlabOutput(tc.in)
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
	if mrErr == nil || !strings.Contains(mrErr.Error(), "was not invoked") {
		t.Fatalf("mr error=%v, want pre-invocation failure", mrErr)
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
printf 'https://gitlab.com/g/p/-/issues/9\n'
`)
	t.Setenv("PATH", binDir)

	got, err := runGlabJSON([]string{"issue", "create", "--title", "Fix"}, repo, "issue")
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.URL != "https://gitlab.com/g/p/-/issues/9" || got.Number != "" {
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
  printf 'https://gitlab.com/g/p/-/merge_requests/4\n'
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
	if !mr.OK || mr.URL != "https://gitlab.com/g/p/-/merge_requests/4" || mr.Number != "" {
		t.Fatalf("mr result=%+v", mr)
	}

	_, issueErr := runGlabJSON([]string{"issue", "create"}, "", "issue")
	if issueErr == nil || !strings.Contains(issueErr.Error(), "glab issue create failed: provider rejected request") {
		t.Fatalf("issue error=%v, want stderr failure", issueErr)
	}
}

func TestGitLabUpdateIssueBodySectionDryRun(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	res, err := NewProvider().UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest{
		IssueURL: "https://gitlab.example.com/acme/repo/-/issues/12",
		Findings: []string{"gold-plating"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK || res.Updated {
		t.Fatalf("dry-run must not update: %+v", res)
	}
	if !strings.Contains(res.Preview, "glab api projects/acme%2Frepo/issues/12") || !strings.Contains(res.Preview, "PUT") {
		t.Fatalf("preview missing expected command: %q", res.Preview)
	}
}

func TestGitLabUpdateIssueBodySectionConfirmMergesIdempotently(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	logPath := filepath.Join(repo, "glab.put")
	writeFakeGlab(t, binDir, `#!/bin/sh
case "$*" in
  *"--method PUT"*)
    printf '%s' "$*" > glab.put
    printf '{"web_url":"https://gitlab.example.com/acme/repo/-/issues/12"}'
    exit 0
    ;;
esac
printf '{"description":"original body\\n\\n<!-- issueops:devils-advocate:start -->\\nstale-finding\\n<!-- issueops:devils-advocate:end -->\\n","web_url":"https://gitlab.example.com/acme/repo/-/issues/12"}'
exit 0
`)
	t.Setenv("PATH", binDir)

	res, err := NewProvider().UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest{
		Repo:     repo,
		IssueURL: "https://gitlab.example.com/acme/repo/-/issues/12",
		Findings: []string{"gold-plating", "schedule optimism"},
		Confirm:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !res.Updated {
		t.Fatalf("confirm should update: %+v", res)
	}
	put, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(put)
	if !strings.Contains(body, "--method PUT") || !strings.Contains(body, "original body") {
		t.Fatalf("PUT must round-trip surrounding body: %q", body)
	}
	if strings.Contains(body, "stale-finding") {
		t.Fatalf("stale managed section must be replaced: %q", body)
	}
	if strings.Count(body, "issueops:devils-advocate:start") != 1 {
		t.Fatalf("managed section must not be duplicated: %q", body)
	}
	if !strings.Contains(body, "gold-plating") || !strings.Contains(body, "schedule optimism") {
		t.Fatalf("findings missing from PUT description: %q", body)
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
