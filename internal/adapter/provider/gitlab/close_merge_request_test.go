package gitlab

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func TestGitLabCloseMergeRequestPreviewKeepsHostname(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	res, err := NewProvider().ClosePullRequest(port.IssueProviderClosePullRequestRequest{
		ArtifactURL: "https://gitlab.corp.example.com/acme/repo/-/merge_requests/7", Kind: "mr",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK || res.Closed || !strings.Contains(res.Preview, "state_event=close") ||
		!strings.Contains(res.Preview, "--hostname gitlab.corp.example.com") ||
		!strings.Contains(res.Preview, "merge_requests/7") {
		t.Fatalf("preview must keep the self-hosted hostname and MR endpoint: %+v", res)
	}
}

func TestGitLabCloseMergeRequestRejectsMalformedURL(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, raw := range []string{"", "https://gitlab.example.com/acme/repo/-/issues/7"} {
		if _, err := NewProvider().ClosePullRequest(port.IssueProviderClosePullRequestRequest{
			ArtifactURL: raw, Kind: "mr",
		}); err == nil {
			t.Fatalf("malformed artifact url %q must be rejected", raw)
		}
	}
}

func TestGitLabCloseMergeRequestLeavesAMergedRequestAlone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	writeFakeGlab(t, binDir, `#!/bin/sh
case "$*" in
*"--method PUT"*) echo "must not close a merged merge request" >&2; exit 9 ;;
*) echo '{"state":"merged"}' ;;
esac
`)
	t.Setenv("PATH", binDir)

	res, err := NewProvider().ClosePullRequest(port.IssueProviderClosePullRequestRequest{
		ArtifactURL: "https://gitlab.example.com/acme/repo/-/merge_requests/7", Kind: "mr", Confirm: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Merged || res.Closed || res.State != "merged" {
		t.Fatalf("a merged merge request must be reported, not closed: %+v", res)
	}
}

func TestGitLabCloseMergeRequestClosesAndVerifies(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	marker := binDir + "/closed.marker"
	writeFakeGlab(t, binDir, `#!/bin/sh
case "$*" in
*"--method PUT"*) : > `+marker+`; echo '{"state":"closed"}' ;;
*) if [ -f `+marker+` ]; then echo '{"state":"closed"}'; else echo '{"state":"opened"}'; fi ;;
esac
`)
	t.Setenv("PATH", binDir)

	res, err := NewProvider().ClosePullRequest(port.IssueProviderClosePullRequestRequest{
		ArtifactURL: "https://gitlab.example.com/acme/repo/-/merge_requests/7", Kind: "mr", Confirm: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Closed || res.AlreadyClosed || res.State != "closed" {
		t.Fatalf("an open merge request must be closed and verified: %+v", res)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("the close mutation must have run: %v", err)
	}
}

func TestGitLabCloseMergeRequestAlreadyClosedShortCircuits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	writeFakeGlab(t, binDir, `#!/bin/sh
case "$*" in
*"--method PUT"*) echo "must not mutate an already closed merge request" >&2; exit 9 ;;
*) echo '{"state":"closed"}' ;;
esac
`)
	t.Setenv("PATH", binDir)

	res, err := NewProvider().ClosePullRequest(port.IssueProviderClosePullRequestRequest{
		ArtifactURL: "https://gitlab.example.com/acme/repo/-/merge_requests/7", Kind: "mr", Confirm: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Closed || !res.AlreadyClosed {
		t.Fatalf("an already closed merge request must short-circuit idempotently: %+v", res)
	}
}
