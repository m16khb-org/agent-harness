package core

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueOpsCleanupStatusRequiresMergedCleanWorktreeAndDeletedRemoteBranch(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "2-cleanup"
	if code, _, stderr := GitCmd(repo, "checkout", "-q", "-b", branch); code != 0 {
		t.Fatalf("git checkout branch failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(repo, "push", "-q", "-u", "origin", branch); code != 0 {
		t.Fatalf("git push branch failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := issueOpsWorktreePathForTest(repo, branch)
	if code, _, stderr := GitCmd(repo, "worktree", "add", "-q", worktree, branch); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/2")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     record.IssueURL,
		Branch:       branch,
		BaseBranch:   "main",
		LinkVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsWorktree(stateRoot, record.ID, worktree)
	if err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, worktree, "plans/demo.md", "plan\n")
	record, err = LinkIssueOpsPlan(stateRoot, record.ID, filepath.Join(worktree, "plans/demo.md"))
	if err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, worktree, "internal/demo.go", "package demo\n")
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean))
	if err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := GitCmd(worktree, "add", "internal/demo.go", "plans/demo.md"); code != 0 {
		t.Fatalf("git add failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(worktree, "commit", "-q", "-m", "feat: implement cleanup"); code != 0 {
		t.Fatalf("git commit failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(worktree, "push", "-q"); code != 0 {
		t.Fatalf("git push failed: %s", stderr)
	}
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhasePR))
	if err != nil {
		t.Fatal(err)
	}
	record, err = VerifyIssueOpsRemoteArtifact(stateRoot, record.ID, IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/2",
		Labels:    []string{"bug"},
		Assignees: []string{"habin"},
	})
	if err != nil {
		t.Fatal(err)
	}
	notMerged := IssueOpsCleanupStatusForRecord(record, IssueOpsCleanupStatusRequest{})
	if notMerged.Ready || !containsString(notMerged.Missing, "remote_artifact_merged") {
		t.Fatalf("cleanup should require explicit merged evidence, got %+v", notMerged)
	}
	if len(notMerged.Choices) != 3 || strings.Contains(notMerged.Choices[0], "정리 진행") || !strings.Contains(notMerged.Choices[0], "차단 해소") {
		t.Fatalf("cleanup must not recommend deletion before readiness, got %+v", notMerged.Choices)
	}
	remoteBranchPresent := IssueOpsCleanupStatusForRecord(record, IssueOpsCleanupStatusRequest{Merged: true})
	if remoteBranchPresent.Ready || !containsString(remoteBranchPresent.Missing, "remote_branch_present") {
		t.Fatalf("cleanup should report remote source branch before local deletion, got %+v", remoteBranchPresent)
	}
	if len(remoteBranchPresent.Choices) != 3 || strings.Contains(remoteBranchPresent.Choices[0], "정리 진행") || !strings.Contains(remoteBranchPresent.Choices[0], "차단 해소") {
		t.Fatalf("cleanup must not recommend deletion while source branch still exists, got %+v", remoteBranchPresent.Choices)
	}
	if code, _, stderr := GitCmd(worktree, "push", "-q", "origin", "--delete", branch); code != 0 {
		t.Fatalf("git push delete branch failed: %s", stderr)
	}
	ready := IssueOpsCleanupStatusForRecord(record, IssueOpsCleanupStatusRequest{Merged: true})
	if !ready.Ready || len(ready.Missing) != 0 || len(ready.Choices) != 3 {
		t.Fatalf("clean merged worktree with deleted remote branch should be cleanup-ready, got %+v", ready)
	}
	if !strings.Contains(ready.Choices[0], "정리 진행") || !strings.Contains(ready.Choices[0], "(추천)") {
		t.Fatalf("cleanup-ready status should recommend cleanup, got %+v", ready.Choices)
	}
	writeIssueOpsFile(t, worktree, "DIRTY.md", "dirty\n")
	dirty := IssueOpsCleanupStatusForRecord(record, IssueOpsCleanupStatusRequest{Merged: true})
	if dirty.Ready || !containsString(dirty.Missing, "worktree_dirty") {
		t.Fatalf("dirty worktree should block cleanup, got %+v", dirty)
	}
}

func TestIssueOpsCleanupStatusBlocksWhenRemoteBranchCheckUnavailable(t *testing.T) {
	repo := initIssueOpsRepo(t)
	branch := "2-cleanup"
	if code, _, stderr := GitCmd(repo, "checkout", "-q", "-b", branch); code != 0 {
		t.Fatalf("git checkout branch failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := issueOpsWorktreePathForTest(repo, branch)
	if code, _, stderr := GitCmd(repo, "worktree", "add", "-q", worktree, branch); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(worktree, "remote", "remove", "origin"); code != 0 {
		t.Fatalf("git remote remove failed: %s", stderr)
	}
	record := IssueOpsRecord{
		ID:           "io-cleanup",
		Branch:       branch,
		Phase:        IssueOpsPhasePR,
		WorktreePath: worktree,
		RemoteArtifact: &IssueOpsRemoteArtifactVerification{
			Provider:  "github",
			Kind:      "pr",
			URL:       "https://github.com/example/repo/pull/2",
			Labels:    []string{"bug"},
			Assignees: []string{"habin"},
		},
	}
	status := IssueOpsCleanupStatusForRecord(record, IssueOpsCleanupStatusRequest{Merged: true})
	if status.Ready || !containsString(status.Missing, "remote_branch_check_unavailable") {
		t.Fatalf("cleanup should block when remote branch check is unavailable, got %+v", status)
	}
}
