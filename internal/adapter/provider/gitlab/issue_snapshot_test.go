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

func TestReadIssueSnapshotTreatsWorkItemAndIssueURLsAsSameIdentity(t *testing.T) {
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGlab(t, binDir, `#!/bin/sh
if [ "$*" != "api projects/acme%2Frepo/issues/69 --hostname gitlab.example.com" ]; then exit 2; fi
printf '%s' '{"web_url":"https://gitlab.example.com/acme/repo/-/issues/69","description":"AC-01"}'
`)
	t.Setenv("PATH", binDir+string(filepath.ListSeparator)+t.TempDir())
	got, err := NewProvider().ReadIssueSnapshot(context.Background(), port.ExecutionIssueSnapshotRequest{
		Repo: repo, URL: "https://gitlab.example.com/acme/repo/-/work_items/69",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://gitlab.example.com/acme/repo/-/work_items/69" || got.Body != "AC-01" {
		t.Fatalf("snapshot = %#v", got)
	}
}

func TestGitLabIssueSnapshotIdentityRequiresSameAuthorityProjectAndIID(t *testing.T) {
	base := "https://gitlab.example.com:8443/acme/repo/-/work_items/69"
	for name, candidate := range map[string]string{
		"authority": "https://gitlab.example.com/acme/repo/-/issues/69",
		"project":   "https://gitlab.example.com:8443/acme/other/-/issues/69",
		"iid":       "https://gitlab.example.com:8443/acme/repo/-/issues/70",
	} {
		t.Run(name, func(t *testing.T) {
			if sameGitLabIssueSnapshotIdentity(base, candidate) {
				t.Fatalf("%s drift was accepted: %q", name, candidate)
			}
		})
	}
	if !sameGitLabIssueSnapshotIdentity(base, "https://gitlab.example.com:8443/acme/repo/-/issues/69") {
		t.Fatal("same authority/project/iid must accept issues and work_items aliases")
	}
}
