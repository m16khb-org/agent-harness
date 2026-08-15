package cleanupstatus

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	model "agent-harness/internal/contract/issueops"
)

func TestByIDReadsRecordAndReportsMissingEvidence(t *testing.T) {
	store := newCleanupStatusTestStore(model.IssueOpsRecord{
		ID:     "io-123456789abc",
		Repo:   t.TempDir(),
		Branch: "1-cleanup",
		Phase:  model.IssueOpsPhasePR,
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{
			Provider:  "github",
			Kind:      "pr",
			URL:       "https://github.com/example/repo/pull/1",
			Labels:    []string{"issueops"},
			Assignees: []string{"sample"},
		},
	})

	status, err := ByID(store.issueOpsStore(), t.TempDir(), "io-123456789abc", model.IssueOpsCleanupStatusRequest{Merged: false})
	if err != nil {
		t.Fatal(err)
	}
	if !status.OK || status.ID != "io-123456789abc" || status.Ready {
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

func TestByIDReturnsReadError(t *testing.T) {
	store := newCleanupStatusTestStore(model.IssueOpsRecord{})
	status, err := ByID(store.issueOpsStore(), t.TempDir(), "io-123456789abc", model.IssueOpsCleanupStatusRequest{})
	if err == nil {
		t.Fatal("ByID(missing) error = nil, want error")
	}
	if status.OK || status.ID != "io-123456789abc" {
		t.Fatalf("missing status = %+v, want ok=false with id", status)
	}
}

func TestRemoteArtifactMissingFieldsAreSorted(t *testing.T) {
	missing := RemoteArtifactMissing(model.IssueOpsRecord{RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{
		Labels:    []string{" ", ""},
		Assignees: nil,
	}})
	for _, want := range []string{"remote_artifact_assignees", "remote_artifact_kind", "remote_artifact_labels", "remote_artifact_provider", "remote_artifact_url"} {
		if !containsString(missing, want) {
			t.Fatalf("missing %q in %#v", want, missing)
		}
	}
}

func TestForRecordRequiresLinkedChildCloseEvidence(t *testing.T) {
	record := completeCleanupRecord(t)
	record.IssueLinks = []model.IssueOpsIssueLink{{
		Type:      "child",
		URL:       "https://github.com/example/repo/issues/2",
		Provider:  "github",
		CreatedAt: "2026-06-17T00:00:00Z",
	}}

	status := ForRecord(record, model.IssueOpsCleanupStatusRequest{Merged: true})
	if status.Ready || !containsString(status.Missing, "child_tasks_closed") {
		t.Fatalf("linked child without close evidence should block cleanup: %#v", status)
	}

	record.IssueLinks[0].ClosedAt = "2026-06-17T00:01:00Z"
	record.IssueLinks[0].CloseVerifiedAt = "2026-06-17T00:01:30Z"
	record.IssueLinks[0].CloseReason = "completed"
	status = ForRecord(record, model.IssueOpsCleanupStatusRequest{Merged: true})
	if containsString(status.Missing, "child_tasks_closed") {
		t.Fatalf("verified child close evidence should unblock child cleanup: %#v", status)
	}
}

func TestForRecordCoversWorktreeAndGitBranches(t *testing.T) {
	record := completeCleanupRecord(t)
	record.WorktreePath = filepath.Join(t.TempDir(), "missing")
	status := ForRecord(record, model.IssueOpsCleanupStatusRequest{Merged: true})
	if status.Ready || !containsString(status.Missing, "worktree_exists") {
		t.Fatalf("missing worktree status = %#v", status)
	}

	repo := newCleanupGitRepo(t)
	record.WorktreePath = repo
	record.Branch = "1234-cleanup"
	status = ForRecord(record, model.IssueOpsCleanupStatusRequest{Merged: true})
	if status.Ready || !containsString(status.Missing, "remote_branch_check_unavailable") {
		t.Fatalf("repo without remote status = %#v", status)
	}

	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("dirty"), 0o600); err != nil {
		t.Fatal(err)
	}
	status = ForRecord(record, model.IssueOpsCleanupStatusRequest{Merged: true})
	if !containsString(status.Missing, "worktree_clean") {
		t.Fatalf("dirty worktree status = %#v", status)
	}

	record.Branch = "different"
	status = ForRecord(record, model.IssueOpsCleanupStatusRequest{Merged: true})
	if !containsString(status.Missing, "branch_match") {
		t.Fatalf("branch mismatch status = %#v", status)
	}
	if worktreePathValid("bad\x00path") {
		t.Fatal("NUL path should be invalid")
	}
}

type cleanupStatusTestStore struct {
	record model.IssueOpsRecord
}

func newCleanupStatusTestStore(record model.IssueOpsRecord) *cleanupStatusTestStore {
	return &cleanupStatusTestStore{record: record}
}

func (s *cleanupStatusTestStore) issueOpsStore() Store {
	return Store{
		Read: func(_ string, id string) (model.IssueOpsRecord, error) {
			if s.record.ID != id {
				return model.IssueOpsRecord{}, fmt.Errorf("missing record")
			}
			return s.record, nil
		},
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func completeCleanupRecord(t *testing.T) model.IssueOpsRecord {
	t.Helper()
	return model.IssueOpsRecord{
		ID:     "io-cleanup",
		Phase:  model.IssueOpsPhasePR,
		Branch: "1234-cleanup",
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{
			Provider:  "github",
			Kind:      "pr",
			URL:       "https://github.com/example/repo/pull/1",
			Labels:    []string{"issueops"},
			Assignees: []string{"sample"},
		},
	}
}

func newCleanupGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runCleanupGit(t, repo, "init", "-b", "1234-cleanup")
	runCleanupGit(t, repo, "config", "user.name", "Test User")
	runCleanupGit(t, repo, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("readme"), 0o600); err != nil {
		t.Fatal(err)
	}
	runCleanupGit(t, repo, "add", "README.md")
	runCleanupGit(t, repo, "commit", "-m", "base")
	return repo
}

func runCleanupGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
