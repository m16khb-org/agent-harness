package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func guardRepoWithCycle(t *testing.T, branch string, phase issueopscontract.IssueOpsPhase) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := StartIssueOps(IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: branch}); err != nil {
		t.Fatal(err)
	}
	if phase != IssueOpsPhaseProblem {
		setIssueOpsPhaseForTest(t, repo, branch, phase)
	}
	return repo
}

func setIssueOpsPhaseForTest(t *testing.T, repo, branch string, phase issueopscontract.IssueOpsPhase) {
	t.Helper()
	id := newIssueOpsID(repo, branch)
	record, err := ReadIssueOps(IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = phase
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
}

func markIssueOpsPRPhaseForTest(t *testing.T, repo, branch string) {
	t.Helper()
	id := newIssueOpsID(repo, branch)
	record, err := ReadIssueOps(IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = IssueOpsPhasePR
	record.RemoteArtifact = &issueopscontract.IssueOpsRemoteArtifactVerification{
		Provider:   "github",
		Kind:       "pr",
		URL:        "https://github.com/example/repo/pull/1",
		Labels:     []string{"issueops"},
		Assignees:  []string{"sample"},
		VerifiedAt: "2026-06-05T00:00:00Z",
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
}

func issueOpsGuardWorktreePathForTest(repo, slug string) string {
	return filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", slug)
}

func makeIssueOpsGuardWorktreeForTest(t *testing.T, repo, slug string) string {
	t.Helper()
	worktree := issueOpsGuardWorktreePathForTest(repo, slug)
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git", "HEAD"), []byte("ref: refs/heads/"+slug+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return worktree
}

type linkedIssueOpsWorktreeForTest struct {
	id   string
	path string
}

func linkIssueOpsWorktreeForGuardTest(t *testing.T, repo, branch string) linkedIssueOpsWorktreeForTest {
	t.Helper()
	record, err := StartIssueOps(IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	issueURL := "https://github.com/example/repo/issues/" + strings.SplitN(branch, "-", 2)[0]
	if _, err := LinkIssueOpsIssue(IssueOpsStateRoot(), record.ID, issueURL); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIssueOpsBranch(IssueOpsStateRoot(), record.ID, issueopscontract.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     issueURL,
		Branch:       branch,
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	worktree := makeIssueOpsGuardWorktreeForTest(t, repo, branch)
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), record.ID, worktree); err != nil {
		t.Fatal(err)
	}
	setIssueOpsPhaseForTest(t, repo, branch, IssueOpsPhaseImplement)
	return linkedIssueOpsWorktreeForTest{id: record.ID, path: worktree}
}
