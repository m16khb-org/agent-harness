package issueops

import (
	"agent-harness/internal/core/issueops/readinesspaths"
	"agent-harness/internal/core/preflight"
	"strings"
)

func IssueOpsPlanReadiness(record IssueOpsRecord) IssueOpsReadiness {
	missing := issueOpsIntentMissing(record)
	if strings.TrimSpace(record.IssueURL) == "" {
		missing = append(missing, "issue_url")
	}
	return IssueOpsReadiness{
		OK:           true,
		Ready:        len(missing) == 0,
		Missing:      uniqSorted(missing),
		IssueURL:     record.IssueURL,
		PlanPath:     record.PlanPath,
		WorktreePath: record.WorktreePath,
		Branch:       record.Branch,
	}
}

func IssueOpsAISlopCleanReadiness(record IssueOpsRecord) IssueOpsReadiness {
	ready := IssueOpsImplementationReadiness(record)
	missing := append([]string{}, ready.Missing...)
	if !issueOpsHasImplementationEvidence(record) {
		missing = append(missing, "implementation_changes")
	}
	missing = uniqSorted(missing)
	ready.Missing = missing
	ready.Ready = len(missing) == 0
	return ready
}

func IssueOpsImplementationReadiness(record IssueOpsRecord) IssueOpsReadiness {
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
	return IssueOpsReadiness{
		OK:           true,
		Ready:        len(missing) == 0,
		Missing:      uniqSorted(missing),
		IssueURL:     record.IssueURL,
		PlanPath:     record.PlanPath,
		WorktreePath: record.WorktreePath,
		Branch:       record.Branch,
	}
}

func issueOpsStrictGitRoot(record IssueOpsRecord) string {
	return readinesspaths.StrictGitRoot(record)
}

func issueOpsWorktreePathValid(path string) bool {
	return readinesspaths.WorktreePathValid(path)
}

func issueOpsPlanPathExists(repo, path string) bool {
	return readinesspaths.PlanPathExists(repo, path)
}

func issueOpsPlanInLinkedWorktree(record IssueOpsRecord) bool {
	return readinesspaths.PlanInLinkedWorktree(record)
}

func issueOpsPlanPathInsideWorktree(worktree, planPath string) bool {
	return readinesspaths.PlanPathInsideWorktree(worktree, planPath)
}

func issueOpsIntentMissing(record IssueOpsRecord) []string {
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

func issueOpsDesignReviewMissing(record IssueOpsRecord) []string {
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

func issueOpsCurrentHead(record IssueOpsRecord) string {
	gitRoot := issueOpsStrictGitRoot(record)
	if gitRoot == "" {
		return ""
	}
	if code, out, _ := preflight.GitCmd(gitRoot, "rev-parse", "HEAD"); code == 0 {
		return strings.TrimSpace(out)
	}
	return ""
}
