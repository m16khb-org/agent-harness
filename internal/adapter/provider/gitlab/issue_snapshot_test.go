package gitlab

import (
	"context"
	"path/filepath"
	"testing"

	"agent-harness/internal/port"
)

func TestReadIssueSnapshotUsesBoundedExactGitLabURL(t *testing.T) {
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGlab(t, binDir, `#!/bin/sh
if [ "$*" != "api projects/acme%2Frepo/issues/69 --hostname gitlab.example.com" ]; then exit 2; fi
printf '%s' '{"web_url":"https://gitlab.example.com/acme/repo/-/issues/69","description":"AC-01"}'
`)
	t.Setenv("PATH", binDir+string(filepath.ListSeparator)+t.TempDir())
	got, err := NewProvider().ReadIssueSnapshot(context.Background(), port.ExecutionIssueSnapshotRequest{
		Repo: repo, URL: "https://gitlab.example.com/acme/repo/-/issues/69",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://gitlab.example.com/acme/repo/-/issues/69" || got.Body != "AC-01" {
		t.Fatalf("snapshot = %#v", got)
	}
}
