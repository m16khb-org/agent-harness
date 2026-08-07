package lifecycle

import (
	"fmt"
	"strings"

	issueopscontract "agent-harness/internal/contract/issueops"
	lifecyclecontract "agent-harness/internal/contract/lifecycle"

	"agent-harness/internal/adapter/remoteartifact"
)

func issueOpsPRTargetBranchBlockReason(req lifecyclecontract.HookToolUseLifecycleRequest) string {
	branches, ok := remoteartifact.PullRequestBranchInfoFromCommand(req.Tool, req.Command, req.Repo)
	if !ok {
		return ""
	}
	record, ok := issueOpsRecordForPRTargetGuard(req, branches.HeadBranch)
	if !ok || record.BranchPrepare == nil {
		return ""
	}
	expected := strings.TrimSpace(record.BranchPrepare.BaseBranch)
	if expected == "" {
		return ""
	}
	actual := strings.TrimSpace(branches.BaseBranch)
	if actual == "" {
		return fmt.Sprintf("IssueOps %s create must specify a target branch matching branch_prepare.base_branch %q for branch %s", strings.ToUpper(branches.Kind), expected, record.Branch)
	}
	if actual != expected {
		return fmt.Sprintf("IssueOps %s create target branch %q must match branch_prepare.base_branch %q for branch %s", strings.ToUpper(branches.Kind), actual, expected, record.Branch)
	}
	return ""
}

func issueOpsRecordForPRTargetGuard(req lifecyclecontract.HookToolUseLifecycleRequest, headBranch string) (issueopscontract.IssueOpsRecord, bool) {
	repo := strings.TrimSpace(req.Repo)
	if repo == "" {
		return issueopscontract.IssueOpsRecord{}, false
	}
	headBranch = strings.TrimSpace(headBranch)
	if headBranch != "" {
		if record, ok := ActiveIssueOpsCycleForBranch(repo, headBranch); ok {
			return record, true
		}
	}
	if headBranch == "" {
		if branch := gitBranchFromHead(repo); branch != "" {
			if record, ok := ActiveIssueOpsCycleForBranch(repo, branch); ok {
				return record, true
			}
		}
	}
	for _, record := range ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo) {
		if headBranch == "" || strings.TrimSpace(record.Branch) == headBranch {
			return record, true
		}
	}
	return issueopscontract.IssueOpsRecord{}, false
}
