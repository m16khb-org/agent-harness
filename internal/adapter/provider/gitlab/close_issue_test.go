package gitlab

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func TestGitLabCloseIssuePreviewKeepsHostname(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	res, err := NewProvider().CloseIssue(port.IssueProviderCloseIssueRequest{
		IssueURL: "https://gitlab.corp.example.com/acme/repo/-/issues/7",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK || res.Closed || !strings.Contains(res.Preview, "state_event=close") || !strings.Contains(res.Preview, "--hostname gitlab.corp.example.com") {
		t.Fatalf("preview must keep the self-hosted hostname: %+v", res)
	}
}

func TestGitLabCloseIssueClosesAndVerifiesWithHostname(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	log := filepath.Join(binDir, "calls.log")
	closedMarker := filepath.Join(binDir, "closed.marker")
	// 1번째 조회: opened → close PUT(마커 생성) → 2번째 조회: closed.
	writeFakeGlab(t, binDir, `#!/bin/sh
echo "$*" >> `+log+`
case "$*" in
*"--method PUT"*) : > `+closedMarker+`; echo '{"state":"closed"}' ;;
*) if [ -f `+closedMarker+` ]; then echo '{"state":"closed"}'; else echo '{"state":"opened"}'; fi ;;
esac
`)
	t.Setenv("PATH", binDir)

	res, err := NewProvider().CloseIssue(port.IssueProviderCloseIssueRequest{
		IssueURL: "https://gitlab.corp.example.com/acme/repo/-/issues/7", Confirm: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Closed || res.AlreadyClosed || !strings.EqualFold(res.State, "closed") {
		t.Fatalf("close must be verified by readback: %+v", res)
	}
	logged, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(logged)), "\n") {
		if !strings.Contains(line, "--hostname gitlab.corp.example.com") {
			t.Fatalf("every glab call must carry the self-hosted hostname: %q", line)
		}
	}
	if !strings.Contains(string(logged), "state_event=close") {
		t.Fatalf("close mutation must use state_event=close: %s", logged)
	}
}

// 회귀(2026-08-26): GitLab 18.10+(관측 19.2.4-ee)은 일반 이슈의 web_url을
// /-/work_items/:iid로 돌려주고 레코드는 그 값을 그대로 봉인한다. close-issue는 그 URL을 issues/:iid REST
// endpoint로 해석해야 한다 — 경로 표식은 identity가 아니다.
func TestGitLabCloseIssueAcceptsWorkItemsIssueURL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	log := filepath.Join(binDir, "calls.log")
	closedMarker := filepath.Join(binDir, "closed.marker")
	// fail-closed fake: issues/105 endpoint 이외의 호출은 실패한다.
	writeFakeGlab(t, binDir, `#!/bin/sh
echo "$*" >> `+log+`
case "$*" in
"api projects/acme%2Frepo/issues/105 --hostname gitlab.example.com --method PUT -f state_event=close") : > `+closedMarker+`; echo '{"state":"closed"}' ;;
"api projects/acme%2Frepo/issues/105 --hostname gitlab.example.com") if [ -f `+closedMarker+` ]; then echo '{"state":"closed"}'; else echo '{"state":"opened"}'; fi ;;
*) echo "unexpected glab call: $*" >&2; exit 1 ;;
esac
`)
	t.Setenv("PATH", binDir)
	issueURL := "https://gitlab.example.com/acme/repo/-/work_items/105"

	preview, err := NewProvider().CloseIssue(port.IssueProviderCloseIssueRequest{IssueURL: issueURL})
	if err != nil {
		t.Fatalf("preview must accept the work_items alias: %v", err)
	}
	if !preview.OK || preview.Closed || !strings.Contains(preview.Preview, "projects/acme%2Frepo/issues/105") {
		t.Fatalf("preview must resolve the alias on the issues endpoint: %+v", preview)
	}

	res, err := NewProvider().CloseIssue(port.IssueProviderCloseIssueRequest{IssueURL: issueURL, Confirm: true})
	if err != nil {
		t.Fatalf("confirm must accept the work_items alias: %v", err)
	}
	if !res.Closed || res.AlreadyClosed || !strings.EqualFold(res.State, "closed") || res.IssueURL != issueURL {
		t.Fatalf("close must be verified by readback on the issues endpoint: %+v", res)
	}
}
