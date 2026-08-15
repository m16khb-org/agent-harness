package issueopscli

import (
	issueopscore "agent-harness/internal/adapter/issueops"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunIssueOpsLinkChildRecordsGitLabWorkItemAfterIssuesTaskFallback(t *testing.T) {
	bin := t.TempDir()
	writeIssueOpsLinkChildFakeCommand(t, filepath.Join(bin, "glab"), `#!/bin/sh
case "$2" in
  projects/sample-group%2Fplatform-group%2Fservice-api/work_items/2490)
    echo "glab: HTTP 404" >&2
    exit 1
    ;;
  projects/sample-group%2Fplatform-group%2Fservice-api/issues/2490)
    printf '{"iid":2490,"type":"TASK","issue_type":"task","web_url":"https://gitlab.example.test/sample-group/platform-group/service-api/-/work_items/2490"}'
    exit 0
    ;;
  *)
    echo "unexpected endpoint: $2" >&2
    exit 1
    ;;
esac
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "gitlab-work-item-link-child")

	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "2490-gitlab-work-item-link-child", "--json"})
	})
	var started struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(start), &started); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, start)
	}
	if started.ID == "" {
		t.Fatalf("start did not return id: %s", start)
	}
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{
			"link-issue",
			"--id", started.ID,
			"--issue-url", "https://gitlab.example.test/sample-group/platform-group/service-api/-/issues/2435",
			"--json",
		})
	})

	childURL := "https://gitlab.example.test/sample-group/platform-group/service-api/-/work_items/2490"
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{
			"link-child",
			"--id", started.ID,
			"--child-url", childURL,
			"--title", "[grpc-ai] Vertex partial instruction/persona cache 누락 방지",
			"--json",
		})
	})

	record, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), started.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, link := range record.IssueLinks {
		if link.Type == "child" && link.URL == childURL && strings.Contains(link.Title, "Vertex partial") {
			found = true
		}
	}
	if !found {
		t.Fatalf("linked child work item was not persisted: %+v", record.IssueLinks)
	}
}

func writeIssueOpsLinkChildFakeCommand(t *testing.T, path string, script string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
