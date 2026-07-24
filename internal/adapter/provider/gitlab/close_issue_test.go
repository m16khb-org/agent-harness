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
