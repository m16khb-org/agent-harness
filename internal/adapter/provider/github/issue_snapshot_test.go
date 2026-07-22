package github

import (
	"context"
	"path/filepath"
	"testing"

	"agent-harness/internal/port"
)

func TestReadIssueSnapshotUsesBoundedExactGitHubURL(t *testing.T) {
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGh(t, binDir, `#!/bin/sh
if [ "$*" != "issue view https://github.com/acme/repo/issues/69 --json url,body" ]; then exit 2; fi
printf '%s' '{"url":"https://github.com/acme/repo/issues/69","body":"AC-01"}'
`)
	t.Setenv("PATH", binDir+string(filepath.ListSeparator)+t.TempDir())
	got, err := NewProvider().ReadIssueSnapshot(context.Background(), port.ExecutionIssueSnapshotRequest{
		Repo: repo, URL: "https://github.com/acme/repo/issues/69",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://github.com/acme/repo/issues/69" || got.Body != "AC-01" {
		t.Fatalf("snapshot = %#v", got)
	}
}
