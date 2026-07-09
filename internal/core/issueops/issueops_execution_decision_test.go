package issueops

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueOpsImplementationReadinessRequiresExecutionDecision(t *testing.T) {
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
		ExecutionDecision: nil,
		BranchPrepare:     &IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/example/repo/issues/1", Branch: "1-demo", BaseBranch: "main", LinkVerified: true},
		WorktreeTools: &IssueOpsWorktreeToolPreparation{
			OK:           true,
			WorktreePath: worktree,
			PreparedAt:   "2026-06-23T00:00:00Z",
		},
	}
	writeIssueOpsFile(t, worktree, "plans/demo.md", "plan\n")

	ready := IssueOpsImplementationReadiness(record)
	if ready.Ready || !containsString(ready.Missing, "execution_decision") {
		t.Fatalf("implementation readiness should require execution_decision, got %+v", ready)
	}
	record.ExecutionDecision = issueOpsExecutionDecisionForTest()
	ready = IssueOpsImplementationReadiness(record)
	if ready.Ready || !containsString(ready.Missing, "compatibility_review") {
		t.Fatalf("execution decision should leave compatibility_review as the remaining implementation gate, got %+v", ready)
	}
	record.CompatibilityReview = issueOpsCompatibilityReviewForTest()
	ready = IssueOpsImplementationReadiness(record)
	if ready.Ready || !containsString(ready.Missing, "devils_advocate_review") {
		t.Fatalf("compatibility review should leave devils_advocate_review as the remaining implementation gate, got %+v", ready)
	}
	record.DevilsAdvocateReview = issueOpsDevilsAdvocateReviewForTest()
	ready = IssueOpsImplementationReadiness(record)
	if !ready.Ready || len(ready.Missing) != 0 {
		t.Fatalf("execution decision, compatibility, and devils-advocate reviews should satisfy implementation gates, got %+v", ready)
	}
}

func TestIssueOpsPhaseImplementRequiresExecutionDecision(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "123-execution-decision"
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
		OK:           true,
		WorktreePath: worktree,
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Phase == IssueOpsPhaseImplement {
		t.Fatalf("worktree tools must not auto-advance without execution decision: %+v", record)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseImplement)); err == nil || !strings.Contains(err.Error(), "execution_decision") {
		t.Fatalf("implement phase should require execution_decision, got %v", err)
	}
	recordIssueOpsExecutionDecisionForTest(t, stateRoot, record.ID)
	recordIssueOpsCompatibilityReviewForTest(t, stateRoot, record.ID)
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseImplement))
	if err != nil {
		t.Fatal(err)
	}
	if record.Phase != IssueOpsPhaseImplement {
		t.Fatalf("execution decision should allow implement phase, got %+v", record)
	}
}

func TestRecordIssueOpsExecutionDecisionValidatesSubagentPolicy(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: "123-subagents"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RecordIssueOpsExecutionDecision(stateRoot, record.ID, IssueOpsExecutionDecisionRecordRequest{
		AutoProceed: []string{"implementation after gates"},
		HookBlocked: []string{"hook remains a fact relay"},
		HumanGates:  []string{"unclear safety judgement"},
		SubagentUse: "planned",
		SubagentPlans: []IssueOpsSubAgentPlan{{
			Objective:            "review the diff from a fresh context",
			Pattern:              "devils-advocate-review",
			Benefit:              "fresh_review",
			Tradeoffs:            []string{"cannot steer the reviewer mid-run", "adds one extra model call"},
			NetPositiveRationale: "fresh-context review is worth the overhead because the main agent authored the diff",
			Scope:                "changed IssueOps files only",
			Verification:         "report findings with file and line evidence",
			Fallback:             "main agent reviews directly if no reviewer is available",
		}},
	})
	if err != nil {
		t.Fatalf("valid planned sub-agent decision should persist: %v", err)
	}
	reloaded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.ExecutionDecision == nil || reloaded.ExecutionDecision.SubagentUse != "planned" || len(reloaded.ExecutionDecision.SubagentPlans) != 1 {
		t.Fatalf("execution decision not persisted: %+v", reloaded.ExecutionDecision)
	}

	if _, err := RecordIssueOpsExecutionDecision(stateRoot, record.ID, IssueOpsExecutionDecisionRecordRequest{
		AutoProceed: []string{"implementation after gates"},
		HookBlocked: []string{"hook remains a fact relay"},
		HumanGates:  []string{"unclear safety judgement"},
		SubagentUse: "planned",
		SubagentPlans: []IssueOpsSubAgentPlan{{
			Objective:            "review the diff",
			Pattern:              "invented-pattern",
			Benefit:              "fresh_review",
			Tradeoffs:            []string{"cannot steer the reviewer mid-run"},
			NetPositiveRationale: "fresh-context review outweighs overhead",
			Scope:                "changed files",
			Verification:         "report findings",
			Fallback:             "main agent reviews directly",
		}},
	}); err == nil || !strings.Contains(err.Error(), "invalid subagent pattern") {
		t.Fatalf("invalid pattern should fail closed, got %v", err)
	}
	if _, err := RecordIssueOpsExecutionDecision(stateRoot, record.ID, IssueOpsExecutionDecisionRecordRequest{
		AutoProceed: []string{"implementation after gates"},
		HookBlocked: []string{"hook remains a fact relay"},
		HumanGates:  []string{"unclear safety judgement"},
		SubagentUse: "planned",
		SubagentPlans: []IssueOpsSubAgentPlan{{
			Objective:            "review the diff",
			Pattern:              "devils-advocate-review",
			Benefit:              "fresh_review",
			Scope:                "changed files",
			Verification:         "report findings",
			Fallback:             "main agent reviews directly",
			NetPositiveRationale: "fresh-context review outweighs overhead",
		}},
	}); err == nil || !strings.Contains(err.Error(), "tradeoffs") {
		t.Fatalf("planned sub-agent decision should require tradeoffs, got %v", err)
	}
	if _, err := RecordIssueOpsExecutionDecision(stateRoot, record.ID, IssueOpsExecutionDecisionRecordRequest{
		AutoProceed:       []string{"implementation after gates"},
		HookBlocked:       []string{"hook remains a fact relay"},
		HumanGates:        []string{"unclear safety judgement"},
		SubagentUse:       "none",
		SubagentRationale: "token=ghp_123456789012345678901234567890123456",
	}); err == nil || !strings.Contains(err.Error(), "secrets") {
		t.Fatalf("secret-like decision text should fail closed, got %v", err)
	}
}
