package main

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

	if err := verifyIssueOpsChildIssueLive(" https://github.com/example/repo/issues/7 "); err != nil {
		t.Fatalf("verify GitHub child issue: %v", err)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(log)), "issue view https://github.com/example/repo/issues/7 --json url,state,title"; got != want {
		t.Fatalf("gh args = %q, want %q", got, want)
	}

	if err := verifyIssueOpsChildIssueLive("https://example.com/issues/7"); err != nil {
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

	if err := verifyIssueOpsChildIssueLive("https://gitlab.example.com/group/project/-/issues/42"); err != nil {
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

func TestVerifyGitLabIssueLiveRejectsInvalidIssuePath(t *testing.T) {
	parsed, err := url.Parse("https://gitlab.example.com/group/project")
	if err != nil {
		t.Fatal(err)
	}

	err = verifyGitLabIssueLive(parsed)
	if err == nil || !strings.Contains(err.Error(), "child_url must be a GitLab issue URL") {
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

	err := verifyGitHubIssueLive("https://github.com/example/repo/issues/404")
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
