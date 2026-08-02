package issueops

import (
	"path/filepath"
	"testing"

	"agent-harness/internal/contract/issueops"
)

func baseIntentRecord(class string) issueops.IssueOpsRecord {
	return issueops.IssueOpsRecord{
		IssueURL: "https://github.com/o/r/issues/1",
		Intent: &issueops.IssueOpsIntentContract{
			RawRequest:        "raw user ask",
			InterpretedIntent: "agent reframed interpretation that differs",
			SuccessCriteria:   []string{"gate works"},
			IntentClass:       class,
		},
	}
}

func planPrepHasMissing(missing []string, key string) bool {
	for _, m := range missing {
		if m == key {
			return true
		}
	}
	return false
}

func TestPlanReadinessRequiresPlanPrepForNonTrivial(t *testing.T) {
	ready := IssueOpsPlanReadiness(baseIntentRecord("standard"))
	for _, key := range []string{"plan_prep_decisions", "plan_prep_related_issues", "plan_prep_web_research", "plan_prep_codebase_survey"} {
		if !planPrepHasMissing(ready.Missing, key) {
			t.Fatalf("standard cycle without plan_prep should miss %s: %#v", key, ready.Missing)
		}
	}
	if ready.Ready {
		t.Fatal("standard cycle without plan_prep should not be ready")
	}
}

func TestPlanReadinessSkipsPlanPrepForTrivial(t *testing.T) {
	ready := IssueOpsPlanReadiness(baseIntentRecord("trivial"))
	if !ready.Ready {
		t.Fatalf("trivial cycle should be ready without plan_prep: %#v", ready.Missing)
	}
}

func TestPlanReadinessAcceptsEvidenceAndWaive(t *testing.T) {
	rec := baseIntentRecord("standard")
	rec.PlanPrep = &issueops.IssueOpsPlanPrep{
		PriorDecisions: issueops.IssueOpsPlanPrepItem{Status: "evidence", Evidence: []string{".agent-harness/ADR.md#gate"}},
		RelatedIssues:  issueops.IssueOpsPlanPrepItem{Status: "evidence", Evidence: []string{"remote score: selected=#12(0.81), threshold=0.70"}},
		WebResearch:    issueops.IssueOpsPlanPrepItem{Status: "waived", WaiveReason: "순수 내부 리팩토링이라 외부 근거 불필요"},
		CodebaseSurvey: issueops.IssueOpsPlanPrepItem{Status: "evidence", Evidence: []string{"rg PlanPrep: internal/core/issueops/issueops_readiness.go, model/types.go, intentdesign/plan_prep.go"}},
	}
	ready := IssueOpsPlanReadiness(rec)
	if !ready.Ready {
		t.Fatalf("evidence+waive should satisfy plan-prep gate: %#v", ready.Missing)
	}
}

func TestPlanReadinessRequiresCodebaseSurvey(t *testing.T) {
	rec := baseIntentRecord("standard")
	rec.PlanPrep = &issueops.IssueOpsPlanPrep{
		PriorDecisions: issueops.IssueOpsPlanPrepItem{Status: "evidence", Evidence: []string{"adr"}},
		RelatedIssues:  issueops.IssueOpsPlanPrepItem{Status: "waived", WaiveReason: "n/a"},
		WebResearch:    issueops.IssueOpsPlanPrepItem{Status: "waived", WaiveReason: "n/a"},
	}
	ready := IssueOpsPlanReadiness(rec)
	if !planPrepHasMissing(ready.Missing, "plan_prep_codebase_survey") {
		t.Fatalf("plan_prep without codebase survey must be missing plan_prep_codebase_survey: %#v", ready.Missing)
	}
}

func TestPlanReadinessRejectsEmptyStatusItem(t *testing.T) {
	rec := baseIntentRecord("standard")
	rec.PlanPrep = &issueops.IssueOpsPlanPrep{
		PriorDecisions: issueops.IssueOpsPlanPrepItem{Status: "evidence", Evidence: []string{"adr"}},
		RelatedIssues:  issueops.IssueOpsPlanPrepItem{Status: "waived", WaiveReason: "n/a"},
		WebResearch:    issueops.IssueOpsPlanPrepItem{Status: "evidence"}, // evidence missing
	}
	ready := IssueOpsPlanReadiness(rec)
	if !planPrepHasMissing(ready.Missing, "plan_prep_web_research") {
		t.Fatalf("web research with empty evidence must be missing: %#v", ready.Missing)
	}
	if planPrepHasMissing(ready.Missing, "plan_prep_decisions") || planPrepHasMissing(ready.Missing, "plan_prep_related_issues") {
		t.Fatalf("valid items must not be missing: %#v", ready.Missing)
	}
}

func TestImplementationReadinessRequiresExecutionLease(t *testing.T) {
	repo := t.TempDir()
	worktree := makeIssueOpsWorktreeDirForTest(t, repo, "1-demo")
	planPath := filepath.Join(worktree, "plans/demo.md")
	writeIssueOpsFile(t, worktree, "plans/demo.md", "plan\n")

	rec := baseIntentRecord("trivial")
	rec.Repo = repo
	rec.Branch = "1-demo"
	rec.BranchPrepare = &issueops.IssueOpsBranchPrepare{
		Provider:     "github",
		IssueURL:     rec.IssueURL,
		Branch:       "1-demo",
		BaseBranch:   "main",
		LinkVerified: true,
	}
	rec.DesignReview = &issueops.IssueOpsDesignReview{
		ProblemSummary: "prepare worktree tools",
		ProposedDesign: "record worktree tool preparation before implementation",
		RefactorPlan:   "durable gate only",
		Alternatives:   []string{"prompt-only reminder"},
		Risks:          []string{"stale worktree tool state"},
		Verification:   []string{"design review checked alternatives and risks"},
		Approved:       true,
	}
	rec.WorktreePath = worktree
	rec.PlanPath = planPath

	ready := IssueOpsImplementationReadiness(rec)
	if ready.Ready {
		t.Fatalf("implementation should not be ready before execution is prepared: %+v", ready)
	}
	if !planPrepHasMissing(ready.Missing, "execution") {
		t.Fatalf("implementation readiness should require execution state: %#v", ready.Missing)
	}
}
