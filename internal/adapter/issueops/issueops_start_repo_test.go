package issueops

import (
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/contract/issueops"
)

func TestStartIssueOpsStoresAbsoluteRepoWhenRelativePathProvided(t *testing.T) {
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: ".", Branch: "12-demo"})
	if err != nil {
		t.Fatal(err)
	}
	if record.Repo != repo || record.ID != newIssueOpsID(repo, "12-demo") {
		t.Fatalf("relative repo was not normalized: %+v", record)
	}
}
