package gitlab

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"issueops/internal/port"
)

// GitHub 쪽과 같은 계약이다: preview는 무엇을 할지 보여주는 것에 더해 자식의
// 현재 원격 상태를 관측한다. cleanup close-children이 부모 머지 증거 없이
// 정리해도 되는지 판정하는 근거다(#129).
func TestGitLabCloseChildPreviewObservesChildState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	repo := t.TempDir()
	logPath := filepath.Join(repo, "glab.calls")
	writeFakeGlab(t, binDir, `#!/bin/sh
printf '%s\n' "$*" >> glab.calls
case "$*" in
  *parentIid*)
    printf '{"data":{"project":{"issue":{"id":"gid://gitlab/WorkItem/12"}}}}'
    exit 0
    ;;
  *children*)
    printf '{"data":{"workItem":{"widgets":[{"type":"HIERARCHY","children":{"nodes":[{"id":"gid://gitlab/WorkItem/34","iid":"34","webUrl":"https://gitlab.example.com/acme/repo/-/work_items/34","state":"CLOSED"}]}}]}}}'
    exit 0
    ;;
esac
echo "unexpected glab call: $*" >&2
exit 2
`)
	t.Setenv("PATH", binDir)

	got, err := NewProvider().CloseChild(port.IssueProviderCloseChildRequest{
		Repo:           repo,
		ParentIssueURL: "https://gitlab.example.com/acme/repo/-/issues/12",
		ChildURL:       "https://gitlab.example.com/acme/repo/-/work_items/34",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Closed {
		t.Fatalf("preview must not report a close it did not perform: %+v", got)
	}
	if !strings.EqualFold(got.State, "closed") || !got.AlreadyClosed || !got.HierarchyVerified {
		t.Fatalf("preview must report the observed remote state: %+v", got)
	}
	if got.Preview == "" {
		t.Fatal("preview text must still describe the pending mutation")
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(log), "workItemUpdate") {
		t.Fatalf("preview must not mutate the remote work item: %s", log)
	}
}

// glab이 없으면 상태를 비워 두고 성공을 돌려준다. core가 미상을 통과 근거로
// 인정하지 않으므로 여기서 오류를 내면 dry-run만 깨진다.
func TestGitLabCloseChildPreviewLeavesStateUnobservedWithoutGlab(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got, err := NewProvider().CloseChild(port.IssueProviderCloseChildRequest{
		Repo:           t.TempDir(),
		ParentIssueURL: "https://gitlab.example.com/acme/repo/-/issues/12",
		ChildURL:       "https://gitlab.example.com/acme/repo/-/work_items/34",
	})
	if err != nil {
		t.Fatalf("an unavailable glab must not fail the dry-run: %v", err)
	}
	if !got.OK || got.State != "" || got.AlreadyClosed || got.HierarchyVerified {
		t.Fatalf("unobserved preview must report no state: %+v", got)
	}
}
