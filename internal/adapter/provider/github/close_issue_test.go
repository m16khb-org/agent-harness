package github

import (
	"runtime"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func TestGitHubCloseIssuePreview(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	res, err := NewProvider().CloseIssue(port.IssueProviderCloseIssueRequest{
		IssueURL: "https://github.com/acme/repo/issues/12",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK || res.Closed || !strings.Contains(res.Preview, "gh issue close") || !strings.Contains(res.Preview, "--reason completed") {
		t.Fatalf("preview must describe the close without mutating: %+v", res)
	}
}

func TestGitHubCloseIssueAlreadyClosedShortCircuits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	writeFakeGh(t, binDir, `#!/bin/sh
case "$1 $2" in
"issue view") echo '{"state":"CLOSED"}' ;;
"issue close") echo "must not mutate an already closed issue" >&2; exit 9 ;;
esac
`)
	t.Setenv("PATH", binDir)

	res, err := NewProvider().CloseIssue(port.IssueProviderCloseIssueRequest{
		IssueURL: "https://github.com/acme/repo/issues/12", Confirm: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Closed || !res.AlreadyClosed || res.State != "CLOSED" {
		t.Fatalf("already-closed issue must short-circuit idempotently: %+v", res)
	}
}

func TestGitHubCloseIssueVerifiesReadbackState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	// close는 성공하지만 readback이 여전히 OPEN → 검증 실패로 에러여야 한다.
	writeFakeGh(t, binDir, `#!/bin/sh
case "$1 $2" in
"issue view") echo '{"state":"OPEN"}' ;;
"issue close") exit 0 ;;
esac
`)
	t.Setenv("PATH", binDir)

	res, err := NewProvider().CloseIssue(port.IssueProviderCloseIssueRequest{
		IssueURL: "https://github.com/acme/repo/issues/12", Confirm: true,
	})
	if err == nil || !strings.Contains(err.Error(), "not verified") {
		t.Fatalf("unverified close must fail: %v %+v", err, res)
	}
}
