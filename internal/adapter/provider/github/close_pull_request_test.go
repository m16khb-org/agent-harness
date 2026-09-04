package github

import (
	"runtime"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func TestGitHubClosePullRequestPreview(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	res, err := NewProvider().ClosePullRequest(port.IssueProviderClosePullRequestRequest{
		ArtifactURL: "https://github.com/acme/repo/pull/12", Kind: "pr",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK || res.Closed || !strings.Contains(res.Preview, "gh pr close") ||
		!strings.Contains(res.Preview, "gh pr view") {
		t.Fatalf("preview must describe the close without mutating: %+v", res)
	}
}

func TestGitHubClosePullRequestRejectsMalformedURL(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, raw := range []string{"", "https://github.com/acme/repo/issues/12", "https://github.com/acme/repo/pull/0"} {
		if _, err := NewProvider().ClosePullRequest(port.IssueProviderClosePullRequestRequest{
			ArtifactURL: raw, Kind: "pr",
		}); err == nil {
			t.Fatalf("malformed artifact url %q must be rejected", raw)
		}
	}
}

// 머지된 PR은 닫지 않는다. abandon 게이트가 미머지만 통과시키지만, provider가
// 최종 판단자이므로 여기서도 mutation 없이 사실만 돌려준다.
func TestGitHubClosePullRequestLeavesAMergedPullRequestAlone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	writeFakeGh(t, binDir, `#!/bin/sh
case "$1 $2" in
"pr view") echo '{"state":"MERGED"}' ;;
"pr close") echo "must not close a merged pull request" >&2; exit 9 ;;
esac
`)
	t.Setenv("PATH", binDir)

	res, err := NewProvider().ClosePullRequest(port.IssueProviderClosePullRequestRequest{
		ArtifactURL: "https://github.com/acme/repo/pull/12", Kind: "pr", Confirm: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Merged || res.Closed || res.State != "MERGED" {
		t.Fatalf("a merged pull request must be reported, not closed: %+v", res)
	}
}

func TestGitHubClosePullRequestAlreadyClosedShortCircuits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	writeFakeGh(t, binDir, `#!/bin/sh
case "$1 $2" in
"pr view") echo '{"state":"CLOSED"}' ;;
"pr close") echo "must not mutate an already closed pull request" >&2; exit 9 ;;
esac
`)
	t.Setenv("PATH", binDir)

	res, err := NewProvider().ClosePullRequest(port.IssueProviderClosePullRequestRequest{
		ArtifactURL: "https://github.com/acme/repo/pull/12", Kind: "pr", Confirm: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Closed || !res.AlreadyClosed || res.State != "CLOSED" {
		t.Fatalf("an already closed pull request must short-circuit idempotently: %+v", res)
	}
}

func TestGitHubClosePullRequestVerifiesReadbackState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	// close는 성공하지만 readback이 여전히 OPEN이면 검증 실패로 에러여야 한다.
	writeFakeGh(t, binDir, `#!/bin/sh
case "$1 $2" in
"pr view") echo '{"state":"OPEN"}' ;;
"pr close") exit 0 ;;
esac
`)
	t.Setenv("PATH", binDir)

	if _, err := NewProvider().ClosePullRequest(port.IssueProviderClosePullRequestRequest{
		ArtifactURL: "https://github.com/acme/repo/pull/12", Kind: "pr", Confirm: true,
	}); err == nil {
		t.Fatal("an unverified close must fail closed")
	}
}

// CloseIssue의 reason은 abandon이 "not planned"로 닫기 위해 필요하다. 빈 값은
// 기존 동작(completed)을 그대로 유지한다.
func TestGitHubCloseIssueReasonDefaultsToCompleted(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	res, err := NewProvider().CloseIssue(port.IssueProviderCloseIssueRequest{
		IssueURL: "https://github.com/acme/repo/issues/12",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Preview, "--reason completed") {
		t.Fatalf("an empty reason must keep the completed default: %+v", res)
	}
	notPlanned, err := NewProvider().CloseIssue(port.IssueProviderCloseIssueRequest{
		IssueURL: "https://github.com/acme/repo/issues/12", Reason: "not_planned",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(notPlanned.Preview, `--reason "not planned"`) {
		t.Fatalf("not_planned must render the gh reason: %+v", notPlanned)
	}
}

func TestGitHubCloseIssueRejectsUnknownReason(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := NewProvider().CloseIssue(port.IssueProviderCloseIssueRequest{
		IssueURL: "https://github.com/acme/repo/issues/12", Reason: "abandoned",
	}); err == nil {
		t.Fatal("an unknown close reason must be rejected")
	}
}
