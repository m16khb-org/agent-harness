package issueops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStartIssueOpsStoresAbsoluteRepoWhenRelativePathProvided(t *testing.T) {
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: ".", Branch: "12-demo"})
	if err != nil {
		t.Fatal(err)
	}

	if record.Repo != repo {
		t.Fatalf("relative repo should be stored as absolute path, got %q want %q", record.Repo, repo)
	}
	if record.ID != newIssueOpsID(repo, "12-demo") {
		t.Fatalf("relative repo should use absolute path identity, got %q want %q", record.ID, newIssueOpsID(repo, "12-demo"))
	}
}

func TestStartIssueOpsResumesLegacyRelativeRepoRecord(t *testing.T) {
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)
	legacy := IssueOpsRecord{
		OK:     true,
		ID:     newIssueOpsID(".", "13-demo"),
		Repo:   ".",
		Branch: "13-demo",
		Phase:  IssueOpsPhasePlan,
	}
	if _, err := WriteIssueOps(stateRoot, legacy); err != nil {
		t.Fatal(err)
	}

	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: ".", Branch: "13-demo"})
	if err != nil {
		t.Fatal(err)
	}

	if record.ID != legacy.ID || record.Repo != legacy.Repo || record.Phase != legacy.Phase {
		t.Fatalf("start should resume legacy relative repo record, got %+v want %+v", record, legacy)
	}
}
