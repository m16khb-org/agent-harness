package remoteverify

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyIssueOpsChildIssueLiveRoutesGithubAndUnknownHosts(t *testing.T) {
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "gh.log")
	writeFakeCommand(t, filepath.Join(bin, "gh"), `#!/bin/sh
printf '%s\n' "$*" > "$HARNESS_FAKE_GH_LOG"
`)
	t.Setenv("HARNESS_FAKE_GH_LOG", logPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := VerifyChildIssueLive(" https://github.com/example/repo/issues/7 "); err != nil {
		t.Fatalf("verify GitHub child issue: %v", err)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(log)), "issue view https://github.com/example/repo/issues/7 --json url,state,title"; got != want {
		t.Fatalf("gh args = %q, want %q", got, want)
	}

	if err := VerifyChildIssueLive("https://example.com/issues/7"); err != nil {
		t.Fatalf("unknown host should not require live verification: %v", err)
	}
}

func TestVerifyIssueOpsChildIssueLiveRoutesGitLabIssueURL(t *testing.T) {
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "glab.log")
	writeFakeCommand(t, filepath.Join(bin, "glab"), `#!/bin/sh
printf '%s\n' "$*" > "$HARNESS_FAKE_GLAB_LOG"
`)
	t.Setenv("HARNESS_FAKE_GLAB_LOG", logPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := VerifyChildIssueLive("https://gitlab.example.com/group/project/-/issues/42"); err != nil {
		t.Fatalf("verify GitLab child issue: %v", err)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "api projects/group%2Fproject/issues/42 --hostname gitlab.example.com"
	if got := strings.TrimSpace(string(log)); got != want {
		t.Fatalf("glab args = %q, want %q", got, want)
	}
}

func TestVerifyIssueOpsChildIssueLiveRoutesGitLabWorkItemURL(t *testing.T) {
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "glab.log")
	writeFakeCommand(t, filepath.Join(bin, "glab"), `#!/bin/sh
printf '%s\n' "$*" > "$HARNESS_FAKE_GLAB_LOG"
`)
	t.Setenv("HARNESS_FAKE_GLAB_LOG", logPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := VerifyChildIssueLive("https://gitlab.example.com/group/project/-/work_items/42"); err != nil {
		t.Fatalf("verify GitLab child work item: %v", err)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "api projects/group%2Fproject/work_items/42 --hostname gitlab.example.com"
	if got := strings.TrimSpace(string(log)); got != want {
		t.Fatalf("glab args = %q, want %q", got, want)
	}
}

func TestVerifyIssueOpsChildIssueLiveAcceptsGitLabWorkItemWhenIssuesEndpointReturnsTask(t *testing.T) {
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "glab.log")
	writeFakeCommand(t, filepath.Join(bin, "glab"), `#!/bin/sh
printf '%s\n' "$*" >> "$HARNESS_FAKE_GLAB_LOG"
case "$2" in
  projects/sample-group%2Fplatform-group%2Fservice-api/work_items/2490)
    echo "glab: HTTP 404" >&2
    exit 1
    ;;
  projects/sample-group%2Fplatform-group%2Fservice-api/issues/2490)
    printf '{"iid":2490,"type":"TASK","issue_type":"task","web_url":"https://gitlab.example.test/sample-group/platform-group/service-api/-/work_items/2490"}'
    exit 0
    ;;
  *)
    echo "unexpected endpoint: $2" >&2
    exit 1
    ;;
esac
`)
	t.Setenv("HARNESS_FAKE_GLAB_LOG", logPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := VerifyChildIssueLive("https://gitlab.example.test/sample-group/platform-group/service-api/-/work_items/2490")
	if err != nil {
		t.Fatalf("verify GitLab child work item through issues fallback: %v", err)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(log))
	want := strings.Join([]string{
		"api projects/sample-group%2Fplatform-group%2Fservice-api/work_items/2490 --hostname gitlab.example.test",
		"api projects/sample-group%2Fplatform-group%2Fservice-api/issues/2490 --hostname gitlab.example.test",
	}, "\n")
	if got != want {
		t.Fatalf("glab calls = %q, want %q", got, want)
	}
}

func TestVerifyGitLabIssueLiveRejectsInvalidIssuePath(t *testing.T) {
	parsed, err := url.Parse("https://gitlab.example.com/group/project")
	if err != nil {
		t.Fatal(err)
	}

	err = VerifyGitLabIssueLive(parsed)
	if err == nil || !strings.Contains(err.Error(), "child_url must be a GitLab issue or work item URL") {
		t.Fatalf("expected invalid GitLab issue URL error, got %v", err)
	}
}

func TestVerifyGitHubIssueLiveReportsCLIStderr(t *testing.T) {
	bin := t.TempDir()
	writeFakeCommand(t, filepath.Join(bin, "gh"), `#!/bin/sh
echo "issue not found" >&2
exit 1
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := VerifyGitHubIssueLive("https://github.com/example/repo/issues/404")
	if err == nil || !strings.Contains(err.Error(), "verify GitHub child issue through gh failed: issue not found") {
		t.Fatalf("expected gh stderr in error, got %v", err)
	}
}

func writeFakeCommand(t *testing.T, path string, script string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
