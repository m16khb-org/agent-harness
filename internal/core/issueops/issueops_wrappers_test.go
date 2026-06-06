package issueops

import (
	"strings"
	"testing"
)

func TestActiveIssueOpsLinkedWorktreeCycleForRepoReturnsFirstActiveRecord(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := initIssueOpsRepo(t)
	record, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "1-active"})
	if err != nil {
		t.Fatal(err)
	}
	issueURL := "https://github.com/example/repo/issues/1"
	if _, err := LinkIssueOpsIssue(IssueOpsStateRoot(), record.ID, issueURL); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIssueOpsBranch(IssueOpsStateRoot(), record.ID, IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     issueURL,
		Branch:       "1-active",
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	worktree := makeIssueOpsWorktreeDirForTest(t, repo, "1-active")
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), record.ID, worktree); err != nil {
		t.Fatal(err)
	}

	got, ok := ActiveIssueOpsLinkedWorktreeCycleForRepo(repo)
	if !ok {
		t.Fatal("ActiveIssueOpsLinkedWorktreeCycleForRepo() ok = false, want true")
	}
	if got.ID != record.ID || got.WorktreePath != worktree {
		t.Fatalf("ActiveIssueOpsLinkedWorktreeCycleForRepo() = %+v, want id %s worktree %s", got, record.ID, worktree)
	}
}

func TestActiveIssueOpsLinkedWorktreeCycleForRepoRejectsMissingRepo(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if got, ok := ActiveIssueOpsLinkedWorktreeCycleForRepo("   "); ok || got.ID != "" {
		t.Fatalf("ActiveIssueOpsLinkedWorktreeCycleForRepo(blank) = %+v, %v; want empty false", got, ok)
	}
}

func TestIssueOpsCleanupStatusByIDReadsRecordAndReportsMissingEvidence(t *testing.T) {
	stateRoot := t.TempDir()
	record := IssueOpsRecord{
		ID:     "io-123456789abc",
		Repo:   t.TempDir(),
		Branch: "1-cleanup",
		Phase:  IssueOpsPhasePR,
		RemoteArtifact: &IssueOpsRemoteArtifactVerification{
			Provider:  "github",
			Kind:      "pr",
			URL:       "https://github.com/example/repo/pull/1",
			Labels:    []string{"issueops"},
			Assignees: []string{"habin"},
		},
	}
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}

	status, err := IssueOpsCleanupStatusByID(stateRoot, record.ID, IssueOpsCleanupStatusRequest{Merged: false})
	if err != nil {
		t.Fatal(err)
	}
	if !status.OK || status.ID != record.ID || status.Ready {
		t.Fatalf("unexpected cleanup status: %+v", status)
	}
	for _, want := range []string{"remote_artifact_merged", "worktree_path"} {
		if !containsString(status.Missing, want) {
			t.Fatalf("cleanup status missing %q not found in %+v", want, status.Missing)
		}
	}
	if len(status.Choices) != 3 || !strings.Contains(status.Choices[0], "차단 해소") {
		t.Fatalf("blocked cleanup choices should guide remediation: %+v", status.Choices)
	}
}

func TestIssueOpsCleanupStatusByIDReturnsReadError(t *testing.T) {
	status, err := IssueOpsCleanupStatusByID(t.TempDir(), "io-123456789abc", IssueOpsCleanupStatusRequest{})
	if err == nil {
		t.Fatal("IssueOpsCleanupStatusByID(missing) error = nil, want error")
	}
	if status.OK || status.ID != "io-123456789abc" {
		t.Fatalf("missing status = %+v, want ok=false with id", status)
	}
}
