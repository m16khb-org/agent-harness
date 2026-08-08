package issueops

import (
	"strings"

	"agent-harness/internal/adapter/issueops/implementation"
	"agent-harness/internal/adapter/issueops/intentdesign"
	"agent-harness/internal/adapter/issueops/readinesspaths"
	"agent-harness/internal/adapter/preflight"
	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/domain/stringlist"
)

func IssueOpsPlanReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	missing := issueOpsIntentMissing(record)
	if strings.TrimSpace(record.IssueURL) == "" {
		missing = append(missing, "issue_url")
	}
	if planPrepGateApplies(record) {
		missing = append(missing, planPrepMissing(record.PlanPrep)...)
	}
	return issueops.IssueOpsReadiness{
		OK:           true,
		Ready:        len(missing) == 0,
		Missing:      stringlist.UniqueSorted(missing),
		IssueURL:     record.IssueURL,
		PlanPath:     record.PlanPath,
		WorktreePath: record.WorktreePath,
		Branch:       record.Branch,
	}
}

// planPrepGateApplies reports whether the plan-prep evidence gate is active.
// It activates only once an intent contract exists (so intent_contract is the
// first missing key for an empty cycle) and the intent class is not trivial.
func planPrepGateApplies(record issueops.IssueOpsRecord) bool {
	if record.Intent == nil {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(record.Intent.IntentClass), "trivial")
}

func planPrepMissing(pp *issueops.IssueOpsPlanPrep) []string {
	if pp == nil {
		return []string{"plan_prep_decisions", "plan_prep_related_issues", "plan_prep_web_research", "plan_prep_codebase_survey"}
	}
	missing := []string{}
	if !planPrepItemValid(pp.PriorDecisions) {
		missing = append(missing, "plan_prep_decisions")
	}
	if !planPrepItemValid(pp.RelatedIssues) {
		missing = append(missing, "plan_prep_related_issues")
	}
	if !planPrepItemValid(pp.WebResearch) {
		missing = append(missing, "plan_prep_web_research")
	}
	if !planPrepItemValid(pp.CodebaseSurvey) {
		missing = append(missing, "plan_prep_codebase_survey")
	}
	return missing
}

func planPrepItemValid(item issueops.IssueOpsPlanPrepItem) bool {
	switch strings.TrimSpace(item.Status) {
	case "evidence":
		return len(cleanIssueOpsTextValues(item.Evidence)) > 0
	case "waived":
		return strings.TrimSpace(item.WaiveReason) != ""
	default:
		return false
	}
}

func IssueOpsAISlopCleanReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	ready := IssueOpsImplementationReadiness(record)
	missing := append([]string{}, ready.Missing...)
	if !implementation.HasEvidence(record) {
		missing = append(missing, "implementation_changes")
	}
	missing = stringlist.UniqueSorted(missing)
	ready.Missing = missing
	ready.Ready = len(missing) == 0
	return ready
}

func IssueOpsCompatibilityReviewReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	missing := issueOpsBaseImplementationMissing(record)
	if path := strings.TrimSpace(record.WorktreePath); path == "" {
		missing = append(missing, "worktree_path")
	} else if !issueOpsWorktreePathValid(path) {
		missing = append(missing, "worktree_exists")
	}
	if strings.TrimSpace(record.PlanPath) != "" && !issueOpsPlanPathExists(issueOpsPlanExistenceRoot(record), record.PlanPath) {
		missing = append(missing, "plan_exists")
	}
	if !issueOpsPlanInLinkedWorktree(record) {
		missing = append(missing, "plan_in_worktree")
	}
	return issueops.IssueOpsReadiness{
		OK:           true,
		Ready:        len(missing) == 0,
		Missing:      stringlist.UniqueSorted(missing),
		IssueURL:     record.IssueURL,
		PlanPath:     record.PlanPath,
		WorktreePath: record.WorktreePath,
		Branch:       record.Branch,
	}
}

func IssueOpsImplementationReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	missing := issueOpsBaseImplementationMissing(record)
	if path := strings.TrimSpace(record.WorktreePath); path == "" {
		missing = append(missing, "worktree_path")
	} else if !issueOpsWorktreePathValid(path) {
		missing = append(missing, "worktree_exists")
	}
	if strings.TrimSpace(record.PlanPath) != "" && !issueOpsPlanPathExists(issueOpsPlanExistenceRoot(record), record.PlanPath) {
		missing = append(missing, "plan_exists")
	}
	if !issueOpsPlanInLinkedWorktree(record) {
		missing = append(missing, "plan_in_worktree")
	}
	missing = append(missing, issueOpsCompatibilityReviewMissing(record)...)
	missing = append(missing, issueOpsDevilsAdvocateReviewMissing(record)...)
	if record.Execution == nil {
		missing = append(missing, "execution")
	} else {
		if err := issueops.ValidateExecution(*record.Execution); err != nil {
			missing = append(missing, "execution_valid")
		}
		if !samePath(record.WorktreePath, record.Execution.Workspace.Root) {
			missing = append(missing, "execution_worktree_match")
		}
		if record.Execution.Lease.Status != issueops.LeaseStatusActive || record.Execution.Lease.Holder == nil {
			missing = append(missing, "execution_write_lease")
		}
	}
	return issueops.IssueOpsReadiness{
		OK:           true,
		Ready:        len(missing) == 0,
		Missing:      stringlist.UniqueSorted(missing),
		IssueURL:     record.IssueURL,
		PlanPath:     record.PlanPath,
		WorktreePath: record.WorktreePath,
		Branch:       record.Branch,
	}
}

// issueOpsDevilsAdvocateReviewMissing is the fail-closed implement-entry gate for
// the brooks devil's advocate: a review must be recorded, and a stop/revise
// verdict must be explicitly waived, before implementation can begin.
func issueOpsDevilsAdvocateReviewMissing(record issueops.IssueOpsRecord) []string {
	review := record.DevilsAdvocateReview
	if review == nil || strings.TrimSpace(review.RecordedAt) == "" {
		return []string{"devils_advocate_review"}
	}
	if (review.Verdict == "stop" || review.Verdict == "revise") && !review.Waived {
		return []string{"devils_advocate_review"}
	}
	return nil
}

func issueOpsCompatibilityReviewMissing(record issueops.IssueOpsRecord) []string {
	review := record.CompatibilityReview
	if review == nil {
		return []string{"compatibility_review"}
	}
	missing := []string{}
	if len(cleanIssueOpsTextValues(review.BackwardCompatibility)) == 0 {
		missing = append(missing, "backward_compatibility")
	}
	if len(cleanIssueOpsTextValues(review.SideEffects)) == 0 {
		missing = append(missing, "side_effects")
	}
	if strings.TrimSpace(review.RollbackPlan) == "" {
		missing = append(missing, "rollback_plan")
	}
	if len(cleanIssueOpsTextValues(review.Verification)) == 0 {
		missing = append(missing, "compatibility_verification")
	}
	if len(cleanIssueOpsTextValues(review.Blockers)) > 0 {
		missing = append(missing, "compatibility_blockers")
	}
	if !review.Approved {
		missing = append(missing, "compatibility_approval")
	}
	return missing
}

func issueOpsStrictGitRoot(record issueops.IssueOpsRecord) string {
	return readinesspaths.StrictGitRoot(record)
}

func issueOpsWorktreePathValid(path string) bool {
	return readinesspaths.WorktreePathValid(path)
}

func issueOpsPlanPathExists(repo, path string) bool {
	return readinesspaths.PlanPathExists(repo, path)
}

func issueOpsPlanInLinkedWorktree(record issueops.IssueOpsRecord) bool {
	return readinesspaths.PlanInLinkedWorktree(record)
}

func issueOpsPlanPathInsideWorktree(worktree, planPath string) bool {
	return readinesspaths.PlanPathInsideWorktree(worktree, planPath)
}

func issueOpsIntentMissing(record issueops.IssueOpsRecord) []string {
	if record.Intent == nil {
		return []string{"intent_contract"}
	}
	missing := []string{}
	if strings.TrimSpace(record.Intent.RawRequest) == "" {
		missing = append(missing, "raw_request")
	}
	if strings.TrimSpace(record.Intent.InterpretedIntent) == "" {
		missing = append(missing, "interpreted_intent")
	}
	if len(cleanIssueOpsTextValues(record.Intent.SuccessCriteria)) == 0 {
		missing = append(missing, "success_criteria")
	}
	return missing
}

func issueOpsDesignReviewMissing(record issueops.IssueOpsRecord) []string {
	if record.DesignReview == nil {
		return []string{"design_review"}
	}
	missing := []string{}
	if strings.TrimSpace(record.DesignReview.ProblemSummary) == "" {
		missing = append(missing, "problem_summary")
	}
	if strings.TrimSpace(record.DesignReview.ProposedDesign) == "" {
		missing = append(missing, "proposed_design")
	}
	if len(cleanIssueOpsTextValues(record.DesignReview.Verification)) == 0 {
		missing = append(missing, "design_verification")
	}
	if record.DesignReview.Approved && !intentdesign.HasDesignReviewEvidence(record.DesignReview.Verification) {
		missing = append(missing, "design_review_evidence")
	}
	if !record.DesignReview.Approved {
		missing = append(missing, "design_approval")
	}
	if len(cleanIssueOpsTextValues(record.DesignReview.OpenQuestions)) > 0 {
		missing = append(missing, "design_open_questions")
	}
	if record.DesignReview.Approved && strings.TrimSpace(record.DesignReview.RefactorPlan) == "" {
		missing = append(missing, "refactor_plan")
	}
	if record.DesignReview.Approved && len(cleanIssueOpsTextValues(record.DesignReview.Alternatives)) == 0 {
		missing = append(missing, "alternatives")
	}
	if record.DesignReview.Approved && len(cleanIssueOpsTextValues(record.DesignReview.Risks)) == 0 {
		missing = append(missing, "risks")
	}
	return missing
}

func issueOpsCurrentHead(record issueops.IssueOpsRecord) string {
	gitRoot := issueOpsStrictGitRoot(record)
	if gitRoot == "" {
		return ""
	}
	if code, out, _ := preflight.GitCmd(gitRoot, "rev-parse", "HEAD"); code == 0 {
		return strings.TrimSpace(out)
	}
	return ""
}
