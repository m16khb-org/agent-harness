package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueOpsLifecycle(t *testing.T) {
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	if record.ID == "" || record.Phase != IssueOpsPhaseProblem || record.Repo != repo || record.Branch != "1-demo" {
		t.Fatalf("unexpected start record: %+v", record)
	}
	if ready := IssueOpsPRReadiness(record); ready.Ready {
		t.Fatalf("new cycle should not be PR-ready: %+v", ready)
	}

	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	if record.Phase != IssueOpsPhasePlan || record.IssueURL == "" {
		t.Fatalf("issue link should move to plan phase: %+v", record)
	}

	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     record.IssueURL,
		Branch:       "1-demo",
		BaseBranch:   "main",
		LinkVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	worktree := makeIssueOpsWorktreeDirForTest(t, repo, "1-demo")
	record, err = LinkIssueOpsWorktree(stateRoot, record.ID, worktree)
	if err != nil {
		t.Fatal(err)
	}
	if record.WorktreePath != worktree {
		t.Fatalf("worktree path should be persisted: %+v", record)
	}
	recordIssueOpsApprovedDesignForTest(t, stateRoot, record.ID)
	writeIssueOpsFile(t, worktree, "docs/superpowers/plans/demo.md", "plan\n")
	record, err = LinkIssueOpsPlan(stateRoot, record.ID, "docs/superpowers/plans/demo.md")
	if err != nil {
		t.Fatal(err)
	}
	if record.Phase != IssueOpsPhaseImplement || record.PlanPath != filepath.Join(worktree, "docs/superpowers/plans/demo.md") {
		t.Fatalf("plan link should move to implement phase: %+v", record)
	}

	record, err = AddIssueOpsFeedback(stateRoot, record.ID, "user", "tighten acceptance criteria", "")
	if err != nil {
		t.Fatal(err)
	}
	if record.Phase != IssueOpsPhaseImplement || len(record.Feedback) != 1 || record.Feedback[0].Source != "user" {
		t.Fatalf("early feedback should be persisted without entering feedback phase: %+v", record)
	}

	reloaded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.ID != record.ID || reloaded.IssueURL != record.IssueURL || reloaded.PlanPath != record.PlanPath || len(reloaded.Feedback) != 1 {
		t.Fatalf("reloaded record mismatch: %+v vs %+v", reloaded, record)
	}
	if ready := IssueOpsPRReadiness(reloaded); ready.Ready || !containsString(ready.Missing, "ai_slop_clean") {
		t.Fatalf("cycle with issue and plan still needs ai-slop-clean before PR drafting: %+v", ready)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean)); err == nil || !strings.Contains(err.Error(), "implementation_changes") {
		t.Fatalf("ai-slop-clean should require implementation changes, got %v", err)
	}
	writeIssueOpsFile(t, worktree, "internal/demo.go", "package demo\n")
	reloaded, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean))
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.AISlopCleanAt == "" {
		t.Fatalf("ai-slop-clean phase should record completion time: %+v", reloaded)
	}
	reloaded, err = AddIssueOpsFeedback(stateRoot, record.ID, "user", "cleanup passed", "")
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Phase != IssueOpsPhaseFeedback || len(reloaded.Feedback) != 2 {
		t.Fatalf("feedback after ai-slop-clean should enter feedback phase: %+v", reloaded)
	}
	if ready := IssueOpsPRReadiness(reloaded); !ready.Ready || len(ready.Missing) != 0 {
		t.Fatalf("cycle with issue, plan, and ai-slop-clean should be PR-ready for drafting: %+v", ready)
	}
}

func TestStartIssueOpsTrimsRepoResumesExistingRecordAndValidatesBranch(t *testing.T) {
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}

	first, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "  " + repo + "  ", Branch: "12-demo"})
	if err != nil {
		t.Fatal(err)
	}
	if !first.OK || first.Repo != repo || first.Branch != "12-demo" || first.Phase != IssueOpsPhaseProblem {
		t.Fatalf("unexpected first start record: %+v", first)
	}
	if first.CreatedAt == "" || first.UpdatedAt == "" || first.ID != newIssueOpsID(repo, "12-demo") {
		t.Fatalf("start should set deterministic identity and timestamps: %+v", first)
	}

	resumed, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "12-demo"})
	if err != nil {
		t.Fatal(err)
	}
	if resumed.ID != first.ID || resumed.CreatedAt != first.CreatedAt || resumed.UpdatedAt != first.UpdatedAt {
		t.Fatalf("second start should resume existing record, first=%+v resumed=%+v", first, resumed)
	}

	if _, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "feature/no-issue"}); err == nil || !strings.Contains(err.Error(), "issue number") {
		t.Fatalf("branch without issue number should be rejected, got %v", err)
	}
}

func TestIssueOpsContractChangeFeedbackBlocksPRUntilIssueUpdateRecorded(t *testing.T) {
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     record.IssueURL,
		Branch:       "1-demo",
		BaseBranch:   "main",
		LinkVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	worktree := makeIssueOpsWorktreeDirForTest(t, repo, "1-demo")
	record, err = LinkIssueOpsWorktree(stateRoot, record.ID, worktree)
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsApprovedDesignForTest(t, stateRoot, record.ID)
	writeIssueOpsFile(t, worktree, "docs/superpowers/plans/demo.md", "plan\n")
	record, err = LinkIssueOpsPlan(stateRoot, record.ID, "docs/superpowers/plans/demo.md")
	if err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, worktree, "internal/demo.go", "package demo\n")
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean))
	if err != nil {
		t.Fatal(err)
	}
	record, err = AddIssueOpsFeedback(stateRoot, record.ID, "review", "acceptance criteria changed", "contract_change")
	if err != nil {
		t.Fatal(err)
	}
	if ready := IssueOpsPRReadiness(record); ready.Ready || !containsString(ready.Missing, "contract_feedback_issue_update") {
		t.Fatalf("contract_change feedback should block PR until issue update is recorded: %+v", ready)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhasePR)); err == nil || !strings.Contains(err.Error(), "contract_feedback_issue_update") {
		t.Fatalf("pr phase should be blocked by unresolved contract feedback, got %v", err)
	}
	record, err = MarkIssueOpsContractFeedbackIssueUpdated(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if record.Feedback[0].IssueUpdatedAt == "" {
		t.Fatalf("issue update timestamp should be recorded: %+v", record.Feedback)
	}
	if ready := IssueOpsPRReadiness(record); !ready.Ready || containsString(ready.Missing, "contract_feedback_issue_update") {
		t.Fatalf("recorded issue update should unblock PR readiness: %+v", ready)
	}
	record.Phase = IssueOpsPhasePR
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	record, err = AddIssueOpsFeedback(stateRoot, record.ID, "review", "post-pr review feedback", "defect")
	if err != nil {
		t.Fatal(err)
	}
	if record.Phase != IssueOpsPhaseFeedback || len(record.Feedback) != 2 {
		t.Fatalf("post-pr feedback should be recorded and return to feedback phase: %+v", record)
	}
	record.Phase = IssueOpsPhaseDone
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	if _, err := AddIssueOpsFeedback(stateRoot, record.ID, "review", "late feedback", "defect"); err == nil || !strings.Contains(err.Error(), "after done phase") {
		t.Fatalf("done phase should reject new feedback, got %v", err)
	}
}

func TestIssueOpsFeedbackRecordsClassification(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = AddIssueOpsFeedback(stateRoot, record.ID, "review", "scope change請求", "CONTRACT_CHANGE")
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Feedback) != 1 || record.Feedback[0].Classification != "contract_change" {
		t.Fatalf("expected classification persisted, got %+v", record.Feedback)
	}
	// classification is optional and defaults to empty.
	record, err = AddIssueOpsFeedback(stateRoot, record.ID, "ci", "flaky test", "")
	if err != nil {
		t.Fatal(err)
	}
	if record.Feedback[1].Classification != "" {
		t.Fatalf("expected empty classification default, got %+v", record.Feedback[1])
	}
	if _, err := AddIssueOpsFeedback(stateRoot, record.ID, "review", "looks odd", "typo"); err == nil || !strings.Contains(err.Error(), "unknown issueops feedback classification") {
		t.Fatalf("expected unknown classification rejection, got %v", err)
	}
}

func TestIssueOpsConcurrentFeedbackNoLostUpdate(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	record, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: "/repo/concurrent"})
	if err != nil {
		t.Fatal(err)
	}
	id := record.ID

	const N = 50
	errs := make(chan error, N)
	for i := range N {
		go func(idx int) {
			_, e := AddIssueOpsFeedback(IssueOpsStateRoot(), id, "test", "feedback-"+string(rune('0'+idx%10)), "")
			errs <- e
		}(i)
	}
	failed := 0
	for range N {
		if e := <-errs; e != nil {
			failed++
			t.Logf("feedback error: %v", e)
		}
	}
	if failed > 0 {
		t.Fatalf("%d of %d concurrent feedback appends failed", failed, N)
	}
	after, err := ReadIssueOps(IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Feedback) != N {
		t.Fatalf("lost update: expected %d feedback items, got %d", N, len(after.Feedback))
	}
}

func TestIssueOpsLockFileDoesNotPolluteEnumerator(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	repo := t.TempDir()
	record, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "42-enum-lock-test"})
	if err != nil {
		t.Fatal(err)
	}
	before := ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo)

	// Simulate a leftover .lock file next to the .json record
	lockPath := filepath.Join(IssueOpsStateRoot(), record.ID+".lock")
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	after := ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo)

	if len(before) != len(after) {
		t.Fatalf(".lock file pollutes enumerator: before=%d after=%d", len(before), len(after))
	}
}

func TestIssueOpsSequentialLockNoDeadlock(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	record, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: "/repo/seq-lock"})
	if err != nil {
		t.Fatal(err)
	}
	id := record.ID
	// Two sequential lock acquisitions must not deadlock.
	for range 2 {
		if _, err := AddIssueOpsFeedback(IssueOpsStateRoot(), id, "test", "feedback", ""); err != nil {
			t.Fatal(err)
		}
	}
}

func TestForceReleaseStampsOrphanWorktreePath(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	repo := t.TempDir()
	worktreeDir := makeIssueOpsWorktreeDirForTest(t, repo, "13-fr-orphan")
	record, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "13-fr-orphan"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(IssueOpsStateRoot(), record.ID, "https://github.com/example/repo/issues/13")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(IssueOpsStateRoot(), record.ID, IssueOpsBranchPrepareRequest{
		Provider: "github", IssueURL: record.IssueURL,
		Branch: "13-fr-orphan", BaseBranch: "main", LinkVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsWorktree(IssueOpsStateRoot(), record.ID, worktreeDir)
	if err != nil {
		t.Fatal(err)
	}
	if record.WorktreePath != worktreeDir {
		t.Fatalf("WorktreePath not set: %+v", record)
	}

	released, err := ForceReleaseIssueOps(IssueOpsStateRoot(), record.ID, "test orphan stamp")
	if err != nil {
		t.Fatal(err)
	}
	if released.OrphanWorktreePath != worktreeDir {
		t.Fatalf("force-release must stamp orphan worktree path: want %q, got %q", worktreeDir, released.OrphanWorktreePath)
	}
	if released.Phase != IssueOpsPhaseDone {
		t.Fatalf("force-release must advance to done, got %q", released.Phase)
	}
}

func TestForceReleaseDoesNotSyncDeleteWorktree(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	repo := t.TempDir()
	worktreeDir := makeIssueOpsWorktreeDirForTest(t, repo, "14-fr-nodelete")
	// Also create a real file to detect deletion
	sentinel := filepath.Join(worktreeDir, "uncommitted.txt")
	if err := os.WriteFile(sentinel, []byte("precious data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	record, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "14-fr-nodelete"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(IssueOpsStateRoot(), record.ID, "https://github.com/example/repo/issues/14")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(IssueOpsStateRoot(), record.ID, IssueOpsBranchPrepareRequest{
		Provider: "github", IssueURL: record.IssueURL,
		Branch: "14-fr-nodelete", BaseBranch: "main", LinkVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsWorktree(IssueOpsStateRoot(), record.ID, worktreeDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ForceReleaseIssueOps(IssueOpsStateRoot(), record.ID, "test no sync delete")
	if err != nil {
		t.Fatal(err)
	}
	// Verify the directory is still present and the sentinel file is intact.
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("force-release must NOT sync-delete worktree dir: sentinel file missing: %v", err)
	}
}

func TestForceReleaseRejectsShortReason(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	repo := t.TempDir()
	worktreeDir := makeIssueOpsWorktreeDirForTest(t, repo, "15-fr-short-reason")
	record, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "15-fr-short-reason"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(IssueOpsStateRoot(), record.ID, "https://github.com/example/repo/issues/15")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(IssueOpsStateRoot(), record.ID, IssueOpsBranchPrepareRequest{
		Provider: "github", IssueURL: record.IssueURL,
		Branch: "15-fr-short-reason", BaseBranch: "main", LinkVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsWorktree(IssueOpsStateRoot(), record.ID, worktreeDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ForceReleaseIssueOps(IssueOpsStateRoot(), record.ID, "short")
	if err == nil {
		t.Fatalf("force-release with short reason must be rejected")
	}
	if !strings.Contains(err.Error(), "at least 10 characters") {
		t.Fatalf("expected 'at least 10 characters' error, got: %v", err)
	}
}

