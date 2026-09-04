package issueops

import (
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
)

func TestIssueOpsImplementationReadinessRequiresCompatibilityReview(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "example")
	worktree := makeIssueOpsWorktreeDirForTest(t, repo, "1-demo")
	record := issueops.IssueOpsRecord{
		OK:            true,
		Repo:          repo,
		Branch:        "1-demo",
		IssueURL:      "https://github.com/example/repo/issues/1",
		PlanPath:      filepath.Join(worktree, "plans/demo.md"),
		WorktreePath:  worktree,
		Intent:        issueOpsIntentContractForTest(),
		DesignReview:  issueOpsDesignReviewForTest(),
		BranchPrepare: &issueops.IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/example/repo/issues/1", Branch: "1-demo", BaseBranch: "main", LinkVerified: true},
		Execution:     issueOpsExecutionForTest(repo, worktree, "1-demo"),
	}
	writeIssueOpsFile(t, worktree, "plans/demo.md", "plan\n")

	ready := IssueOpsImplementationReadiness(record)
	if ready.Ready || !containsString(ready.Missing, "compatibility_review") {
		t.Fatalf("implementation readiness should require compatibility_review, got %+v", ready)
	}
	record.CompatibilityReview = issueOpsCompatibilityReviewForTest()
	ready = IssueOpsImplementationReadiness(record)
	if ready.Ready || !containsString(ready.Missing, "devils_advocate_review") {
		t.Fatalf("compatibility review alone should still require devils_advocate_review, got %+v", ready)
	}
	record.DevilsAdvocateReview = issueOpsDevilsAdvocateReviewForTest()
	record.DevilsAdvocateReview.ReviewedPlanDigest = digestExecutionOwnerBytes([]byte("plan\n"))
	ready = IssueOpsImplementationReadiness(record)
	if !ready.Ready || len(ready.Missing) != 0 {
		t.Fatalf("compatibility + devils-advocate review should satisfy the last implementation gate, got %+v", ready)
	}
}

func TestIssueOpsPhaseImplementRequiresCompatibilityReviewPhase(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "123-compatibility-review"
	worktree := makeIssueOpsWorktreeDirForTest(t, repo, branch)

	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/123")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{Provider: "github", IssueURL: record.IssueURL, Branch: branch, BaseBranch: "main", LinkVerified: true})
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
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseImplement)); err == nil || !strings.Contains(err.Error(), "compatibility_review") {
		t.Fatalf("implement phase should require compatibility_review, got %v", err)
	}
	record, err = RecordIssueOpsCompatibilityReview(stateRoot, record.ID, issueops.IssueOpsCompatibilityReviewRequest{
		BackwardCompatibility: []string{"existing IssueOps JSON records remain readable"},
		SideEffects:           []string{"phase order changes are limited to IssueOps lifecycle transitions"},
		RollbackPlan:          "Revert the phase and readiness gate if host integration breaks.",
		Verification:          []string{"compatibility review checked backward compatibility and side effects", "go test ./internal/core/issueops"},
		Approved:              true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Phase != IssueOpsPhaseCompatibilityReview {
		t.Fatalf("compatibility review should persist the compatibility-review phase, got %+v", record)
	}
	if _, err := RecordIssueOpsDevilsAdvocateReview(stateRoot, record.ID, issueops.IssueOpsDevilsAdvocateReviewRequest{Verdict: "pass", ReviewerContext: "subagent", Findings: []string{"attacked gate 3"}}); err != nil {
		t.Fatal(err)
	}
	record = recordIssueOpsPreparedExecutionForTest(t, stateRoot, record.ID, worktree)
	if record.Phase != IssueOpsPhaseImplement {
		t.Fatalf("compatibility and devils-advocate review should allow implement phase, got %+v", record)
	}
}
