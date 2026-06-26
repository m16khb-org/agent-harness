package issueops

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueOpsImplementationReadinessRequiresCompatibilityReview(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "example")
	worktree := makeIssueOpsWorktreeDirForTest(t, repo, "1-demo")
	record := IssueOpsRecord{
		OK:                true,
		Repo:              repo,
		Branch:            "1-demo",
		IssueURL:          "https://github.com/example/repo/issues/1",
		PlanPath:          filepath.Join(worktree, "plans/demo.md"),
		WorktreePath:      worktree,
		Intent:            issueOpsIntentContractForTest(),
		DesignReview:      issueOpsDesignReviewForTest(),
		ExecutionDecision: issueOpsExecutionDecisionForTest(),
		BranchPrepare:     &IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/example/repo/issues/1", Branch: "1-demo", BaseBranch: "main", LinkVerified: true},
		WorktreeTools: &IssueOpsWorktreeToolPreparation{
			OK:                   true,
			WorktreePath:         worktree,
			CodeGraphProjectPath: worktree,
			CodeGraphChecked:     true,
			CodeGraphReady:       true,
			PreparedAt:           "2026-06-26T00:00:00Z",
		},
	}
	writeIssueOpsFile(t, worktree, "plans/demo.md", "plan\n")

	ready := IssueOpsImplementationReadiness(record)
	if ready.Ready || !containsString(ready.Missing, "compatibility_review") {
		t.Fatalf("implementation readiness should require compatibility_review, got %+v", ready)
	}
	record.CompatibilityReview = issueOpsCompatibilityReviewForTest()
	ready = IssueOpsImplementationReadiness(record)
	if !ready.Ready || len(ready.Missing) != 0 {
		t.Fatalf("compatibility review should satisfy the last implementation gate, got %+v", ready)
	}
}

func TestIssueOpsPhaseImplementRequiresCompatibilityReviewPhase(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "123-compatibility-review"
	worktree := makeIssueOpsWorktreeDirForTest(t, repo, branch)

	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/123")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{Provider: "github", IssueURL: record.IssueURL, Branch: branch, BaseBranch: "main", LinkVerified: true})
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
	record, err = RecordIssueOpsWorktreeTools(stateRoot, record.ID, IssueOpsWorktreeToolPreparation{
		OK:                   true,
		WorktreePath:         worktree,
		CodeGraphProjectPath: worktree,
		CodeGraphChecked:     true,
		CodeGraphReady:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsExecutionDecisionForTest(t, stateRoot, record.ID)

	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseImplement)); err == nil || !strings.Contains(err.Error(), "compatibility_review") {
		t.Fatalf("implement phase should require compatibility_review, got %v", err)
	}
	record, err = RecordIssueOpsCompatibilityReview(stateRoot, record.ID, IssueOpsCompatibilityReviewRequest{
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
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseImplement))
	if err != nil {
		t.Fatal(err)
	}
	if record.Phase != IssueOpsPhaseImplement {
		t.Fatalf("compatibility review should allow implement phase, got %+v", record)
	}
}
