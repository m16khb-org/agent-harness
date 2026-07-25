package github

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

// preview는 무엇을 할지 보여주는 것에 더해 지금 상태가 무엇인지도 관측한다.
// cleanup close-children이 부모 머지 증거 없이 정리해도 되는지 판정하려면 자식이
// 원격에서 이미 닫혔는지를 알아야 한다(#129).
func TestGitHubCloseChildPreviewObservesChildState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	logPath := filepath.Join(repo, "gh.calls")
	writeFakeGh(t, binDir, `#!/bin/sh
printf '%s\n' "$*" >> gh.calls
if [ "$1" = "api" ] && [ "$2" = "repos/acme/repo/issues/12/sub_issues" ]; then
  printf '[{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34","state":"closed"}]'
  exit 0
fi
echo "unexpected gh call: $*" >&2
exit 2
`)
	t.Setenv("PATH", binDir)

	got, err := NewProvider().CloseChild(port.IssueProviderCloseChildRequest{
		Repo:           repo,
		ParentIssueURL: "https://github.com/acme/repo/issues/12",
		ChildURL:       "https://github.com/acme/repo/issues/34",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Closed {
		t.Fatalf("preview must not report a close it did not perform: %+v", got)
	}
	if got.State != "closed" || !got.AlreadyClosed || !got.HierarchyVerified {
		t.Fatalf("preview must report the observed remote state: %+v", got)
	}
	if got.Preview == "" {
		t.Fatal("preview text must still describe the pending mutation")
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(log), "PATCH") {
		t.Fatalf("preview must not mutate the remote issue: %s", log)
	}
}

// 관측할 수 없으면 상태를 비워 둔다. core는 미상을 통과 근거로 인정하지 않으므로
// 여기서 오류를 내면 gh 부재 환경의 dry-run이 통째로 깨질 뿐이다.
func TestGitHubCloseChildPreviewLeavesStateUnobservedWithoutGh(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got, err := NewProvider().CloseChild(port.IssueProviderCloseChildRequest{
		Repo:           t.TempDir(),
		ParentIssueURL: "https://github.com/acme/repo/issues/12",
		ChildURL:       "https://github.com/acme/repo/issues/34",
	})
	if err != nil {
		t.Fatalf("an unavailable gh must not fail the dry-run: %v", err)
	}
	if !got.OK || got.State != "" || got.AlreadyClosed || got.HierarchyVerified {
		t.Fatalf("unobserved preview must report no state: %+v", got)
	}
}

// 자식이 부모의 sub-issue가 아니면 preview는 계층 검증 실패로 남긴다.
func TestGitHubCloseChildPreviewReportsHierarchyMissing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	writeFakeGh(t, binDir, `#!/bin/sh
if [ "$1" = "api" ] && [ "$2" = "repos/acme/repo/issues/12/sub_issues" ]; then
  printf '[]'
  exit 0
fi
echo "unexpected gh call: $*" >&2
exit 2
`)
	t.Setenv("PATH", binDir)

	got, err := NewProvider().CloseChild(port.IssueProviderCloseChildRequest{
		Repo:           t.TempDir(),
		ParentIssueURL: "https://github.com/acme/repo/issues/12",
		ChildURL:       "https://github.com/acme/repo/issues/34",
	})
	if err != nil {
		t.Fatalf("preview must report rather than fail: %v", err)
	}
	if got.HierarchyVerified || got.State != "" {
		t.Fatalf("a child outside the parent hierarchy must not be reported as observed: %+v", got)
	}
}
