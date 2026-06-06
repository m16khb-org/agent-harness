package issueops

import (
	"agent-harness/internal/core/preflight"
	"path/filepath"
	"testing"
)

func TestIssueOpsRefreshesAISlopCleanEvidenceFromFeedback(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "13-refresh-cleanup"
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "-b", branch); code != 0 {
		t.Fatalf("git checkout branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "-u", "origin", branch); code != 0 {
		t.Fatalf("git push branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := issueOpsWorktreePathForTest(repo, "refresh-cleanup")
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", worktree, branch); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}

	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/13")
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
	recordIssueOpsApprovedDesignForTest(t, stateRoot, record.ID)
	writeIssueOpsFile(t, worktree, "plans/demo.md", "plan\n")
	record, err = LinkIssueOpsPlan(stateRoot, record.ID, filepath.Join(worktree, "plans/demo.md"))
	if err != nil {
		t.Fatal(err)
	}

	writeIssueOpsFile(t, worktree, "internal/demo.go", "package demo\nconst Value = 1\n")
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean))
	if err != nil {
		t.Fatal(err)
	}
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseFeedback))
	if err != nil {
		t.Fatal(err)
	}
	originalFingerprint := record.AISlopCleanFingerprint

	writeIssueOpsFile(t, worktree, "internal/demo.go", "package demo\nconst Value = 2\n")
	refreshed, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean))
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Phase != IssueOpsPhaseFeedback {
		t.Fatalf("refresh should preserve feedback phase, got %+v", refreshed)
	}
	if refreshed.AISlopCleanFingerprint == "" || refreshed.AISlopCleanFingerprint == originalFingerprint {
		t.Fatalf("refresh should update ai-slop-clean fingerprint: before=%q after=%q", originalFingerprint, refreshed.AISlopCleanFingerprint)
	}
	if ready := IssueOpsStrictPRReadiness(refreshed); ready.Ready || containsString(ready.Missing, "ai_slop_clean_stale") {
		t.Fatalf("refreshed cleanup evidence should clear stale gate while preserving other gates: %+v", ready)
	}
}
