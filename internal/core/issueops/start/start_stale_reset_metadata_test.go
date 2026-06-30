package start

import (
	"testing"

	"agent-harness/internal/core/issueops/model"
)

// TestStartResetPreservesAnalysisMetadata verifies that a stale-worktree reset
// carries the non-worktree analysis/audit metadata into the fresh problem-phase
// record. Intent/DomainReview/DesignReview/Decisions/PlanPrep/CompatibilityReview/
// ExecutionDecision/RoutingTrace live in the state JSON (not the deleted
// worktree) and describe the problem/plan that carries into re-work, so a
// transient os.Stat worktree-miss must not irreversibly destroy them. Before the
// fix the reset struct dropped these fields, so they came back nil/empty.
func TestStartResetPreservesAnalysisMetadata(t *testing.T) {
	s := newFakeStartStore()
	s.valid = func(string) bool { return false } // worktree gone

	s.records["io-fixed"] = model.IssueOpsRecord{
		OK: true, ID: "io-fixed", Repo: "/repo", Branch: "1-x",
		Phase:        model.IssueOpsPhaseImplement,
		WorktreePath: "/gone/worktree",
		CreatedAt:    "2026-01-01T00:00:00Z",
		Intent: &model.IssueOpsIntentContract{
			RawRequest:        "fix the bug",
			InterpretedIntent: "repair X",
			SuccessCriteria:   []string{"tests pass"},
			RecordedAt:        "2026-01-01T00:01:00Z",
		},
		DomainReview: &model.IssueOpsDomainReview{
			ModelFit:   "fits",
			ReviewedAt: "2026-01-01T00:02:00Z",
		},
		DesignReview: &model.IssueOpsDesignReview{
			ProblemSummary: "summary",
			ProposedDesign: "design",
			Verification:   []string{"verify"},
			Approved:       true,
			ReviewedAt:     "2026-01-01T00:03:00Z",
		},
		Decisions: []model.IssueOpsDecision{
			{Title: "use Y", Body: "because", Kind: "design", CreatedAt: "2026-01-01T00:04:00Z"},
		},
		PlanPrep: &model.IssueOpsPlanPrep{
			PriorDecisions: model.IssueOpsPlanPrepItem{Status: "ok"},
			RecordedAt:     "2026-01-01T00:05:00Z",
		},
		CompatibilityReview: &model.IssueOpsCompatibilityReview{
			RollbackPlan: "revert",
			Approved:     true,
			ReviewedAt:   "2026-01-01T00:06:00Z",
		},
		ExecutionDecision: &model.IssueOpsExecutionDecision{
			SubagentUse: "none",
			RecordedAt:  "2026-01-01T00:07:00Z",
		},
		RoutingTrace: []model.SkillRoutingEntry{
			{Phase: "implement", Skill: "executor", At: "2026-01-01T00:08:00Z"},
		},
	}

	got, err := Start(s.store(), "/state", model.IssueOpsStartRequest{Repo: "/repo", Branch: "1-x"})
	if err != nil {
		t.Fatal(err)
	}

	// The stale reset must have fired.
	if got.Phase != model.IssueOpsPhaseProblem {
		t.Fatalf("reset must move phase to problem, got %q", got.Phase)
	}
	if got.StaleResetPriorPhase != string(model.IssueOpsPhaseImplement) {
		t.Fatalf("StaleResetPriorPhase must record the prior implement phase, got %q", got.StaleResetPriorPhase)
	}

	// Analysis/audit metadata must survive the reset.
	if got.Intent == nil || got.Intent.InterpretedIntent != "repair X" {
		t.Fatalf("Intent must survive reset, got %+v", got.Intent)
	}
	if got.DomainReview == nil || got.DomainReview.ModelFit != "fits" {
		t.Fatalf("DomainReview must survive reset, got %+v", got.DomainReview)
	}
	if got.DesignReview == nil || got.DesignReview.ProposedDesign != "design" {
		t.Fatalf("DesignReview must survive reset, got %+v", got.DesignReview)
	}
	if got.DesignReview.Approved {
		t.Fatalf("DesignReview.Approved must be cleared on reset so re-work re-earns the design gate")
	}
	if len(got.Decisions) != 1 || got.Decisions[0].Title != "use Y" {
		t.Fatalf("Decisions must survive reset, got %+v", got.Decisions)
	}
	if got.PlanPrep == nil || got.PlanPrep.PriorDecisions.Status != "ok" {
		t.Fatalf("PlanPrep must survive reset, got %+v", got.PlanPrep)
	}
	if got.CompatibilityReview == nil || got.CompatibilityReview.RollbackPlan != "revert" {
		t.Fatalf("CompatibilityReview must survive reset, got %+v", got.CompatibilityReview)
	}
	if got.CompatibilityReview.Approved {
		t.Fatalf("CompatibilityReview.Approved must be cleared on reset so re-work re-earns the compatibility gate")
	}
	if got.ExecutionDecision == nil || got.ExecutionDecision.SubagentUse != "none" {
		t.Fatalf("ExecutionDecision must survive reset, got %+v", got.ExecutionDecision)
	}
	if len(got.RoutingTrace) != 1 || got.RoutingTrace[0].Skill != "executor" {
		t.Fatalf("RoutingTrace must survive reset, got %+v", got.RoutingTrace)
	}

	// Worktree-specific state must NOT carry over as live state, but the orphan
	// breadcrumb must be stamped for audit.
	if got.WorktreePath != "" {
		t.Fatalf("WorktreePath must be cleared on reset, got %q", got.WorktreePath)
	}
	if got.OrphanWorktreePath != "/gone/worktree" {
		t.Fatalf("orphan worktree breadcrumb must be stamped, got %q", got.OrphanWorktreePath)
	}
}
