package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/adapter/preflight"
	"agent-harness/internal/contract/issueops"
)

// B4 plan-before-execute gate: AdvanceIssueOpsPhase(...,"implement") must block
// on design_approval when the design review is not approved. The gate already
// exists (issueOpsBaseImplementationMissing -> issueOpsDesignReviewMissing), and
// is exercised via LinkIssueOpsPlan and the full-lifecycle test; this asserts
// the DIRECT advance-to-implement boundary specifically, and that recording an
// approved design review clears the design_approval block.
func TestAdvanceToImplementGatesOnDesignApproval(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "1-demo"
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "-b", branch); code != 0 {
		t.Fatalf("git checkout branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "-u", "origin", branch); code != 0 {
		t.Fatalf("git push branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := issueOpsWorktreePathForTest(repo, "1-demo")
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", worktree, branch); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}

	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseGrill)); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1"); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider: "github", IssueURL: "https://github.com/example/repo/issues/1",
		Branch: branch, BaseBranch: "main", LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsWorktree(stateRoot, record.ID, worktree); err != nil {
		t.Fatal(err)
	}
	setIssueOpsPlanPrepForTest(t, stateRoot, record.ID)

	// No design review recorded: implement entry is blocked because the review
	// is missing entirely.
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseImplement)); err == nil || !strings.Contains(err.Error(), "design_review") {
		t.Fatalf("implement entry with no design review must be blocked on design_review, got %v", err)
	}

	// An UNAPPROVED design review: implement entry must be blocked SPECIFICALLY on
	// design_approval (the plan-before-execute gate — a plan exists/was reviewed
	// but not approved).
	if _, err := RecordIssueOpsDesignReview(stateRoot, record.ID, issueops.IssueOpsDesignReviewRequest{
		ProblemSummary: "IssueOps must preserve the work contract",
		ProposedDesign: "Gate implementation on a reviewed design contract",
		Verification:   []string{"design review checked alternatives and risks"},
		Approved:       false,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseImplement)); err == nil || !strings.Contains(err.Error(), "design_approval") {
		t.Fatalf("implement entry with an unapproved design review must be blocked on design_approval, got %v", err)
	}

	// Recording an approved design review clears the design_approval block (other
	// readiness items such as the plan may remain, but the approval gate is gone).
	recordIssueOpsApprovedDesignForTest(t, stateRoot, record.ID)
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseImplement)); err != nil && strings.Contains(err.Error(), "design_approval") {
		t.Fatalf("approved design review must clear the design_approval block, still got %v", err)
	}
}
