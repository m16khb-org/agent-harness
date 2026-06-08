package issueops

import (
	"strings"

	"agent-harness/internal/core/issueops/stringlist"
)

func IssueOpsPRReadiness(record IssueOpsRecord) IssueOpsReadiness {
	missing := issueOpsBaseImplementationMissing(record)
	if strings.TrimSpace(record.WorktreePath) == "" {
		missing = append(missing, "worktree_path")
	}
	if strings.TrimSpace(record.PlanPath) != "" && !issueOpsPlanPathExists(issueOpsPlanExistenceRoot(record), record.PlanPath) {
		missing = append(missing, "plan_exists")
	}
	if !issueOpsPlanInLinkedWorktree(record) {
		missing = append(missing, "plan_in_worktree")
	}
	if strings.TrimSpace(record.AISlopCleanAt) == "" {
		missing = append(missing, "ai_slop_clean")
	}
	if issueOpsHasUnresolvedContractFeedback(record) {
		missing = append(missing, "contract_feedback_issue_update")
	}
	missing = stringlist.UniqueSorted(missing)
	var iddWarnings []string
	if len(record.Decisions) == 0 {
		iddWarnings = append(iddWarnings, "no_decision_records")
	}
	hasNonChildLink := false
	for _, link := range record.IssueLinks {
		if link.Type != "child" {
			hasNonChildLink = true
			break
		}
	}
	if !hasNonChildLink && len(record.IssueLinks) == 0 {
		iddWarnings = append(iddWarnings, "no_issue_graph_links")
	}
	return IssueOpsReadiness{
		OK:           true,
		Ready:        len(missing) == 0,
		Missing:      missing,
		Warnings:     iddWarnings,
		IssueURL:     record.IssueURL,
		PlanPath:     record.PlanPath,
		WorktreePath: record.WorktreePath,
		Branch:       record.Branch,
	}
}

func issueOpsHasUnresolvedContractFeedback(record IssueOpsRecord) bool {
	for _, item := range record.Feedback {
		if issueOpsFeedbackRequiresIssueUpdate(item) {
			return true
		}
	}
	return false
}

func issueOpsFeedbackRequiresIssueUpdate(item IssueOpsFeedbackItem) bool {
	return strings.EqualFold(strings.TrimSpace(item.Classification), "contract_change") &&
		strings.TrimSpace(item.IssueUpdatedAt) == ""
}

func issueOpsBaseImplementationMissing(record IssueOpsRecord) []string {
	missing := issueOpsBranchEvidenceMissing(record)
	missing = append(missing, issueOpsIntentMissing(record)...)
	missing = append(missing, issueOpsDesignReviewMissing(record)...)
	if strings.TrimSpace(record.PlanPath) == "" {
		missing = append(missing, "plan_path")
	}
	return missing
}

func issueOpsPlanExistenceRoot(record IssueOpsRecord) string {
	if worktree := strings.TrimSpace(record.WorktreePath); worktree != "" {
		return worktree
	}
	return strings.TrimSpace(record.Repo)
}

func issueOpsBranchEvidenceMissing(record IssueOpsRecord) []string {
	missing := []string{}
	if strings.TrimSpace(record.IssueURL) == "" {
		missing = append(missing, "issue_url")
	}
	if strings.TrimSpace(record.Branch) == "" {
		missing = append(missing, "branch")
	}
	if record.BranchPrepare == nil {
		missing = append(missing, "branch_prepare")
	} else if !record.BranchPrepare.LinkVerified {
		missing = append(missing, "branch_link_verified")
	}
	return missing
}
