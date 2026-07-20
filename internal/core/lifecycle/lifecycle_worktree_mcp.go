package lifecycle

import (
	"strings"

	"agent-harness/internal/core/issueops"
)

func mcpWorktreeRootBlockReason(req HookToolUseLifecycleRequest) string {
	expected := expectedIssueOpsWorktreesForMCPGuard(req)
	if len(expected) == 0 {
		return ""
	}
	primary := expected[0]
	if requestIsProvenSourceOnly(req, sourceRootForExpectedWorktree(req, primary)) {
		return ""
	}
	tool := strings.ToLower(strings.TrimSpace(req.Tool))
	if strings.Contains(tool, "filesystem") || strings.Contains(tool, "serena") {
		return "source-root-bound MCP tool is not allowed during IssueOps worktree implementation; use native absolute-path file tools, rg rooted at the IssueOps worktree, or git -C " + primary
	}
	return ""
}

func expectedIssueOpsWorktreesForMCPGuard(req HookToolUseLifecycleRequest) []string {
	if expected := cleanAbsPath(req.ExpectedWorktree); expected != "" {
		return []string{expected}
	}
	// Check session binding first — the most reliable per-repo signal across
	// session restarts, since HARNESS_EXPECTED_WORKTREE is ephemeral.
	if sessionWorktree := expectedWorktreeFromSessionBinding(req.Repo); sessionWorktree != "" {
		return []string{sessionWorktree}
	}
	if rec, ok := ActiveIssueOpsCycleForBranch(req.Repo, gitBranchFromHead(req.Repo)); ok && IssueOpsPhaseExpectsWorktree(rec.Phase) {
		if expected := cleanAbsPath(rec.WorktreePath); expected != "" {
			return []string{expected}
		}
	}
	expected := []string{}
	for _, rec := range ActiveIssueOpsLinkedWorktreeCyclesForRepo(req.Repo) {
		if !IssueOpsPhaseExpectsWorktree(rec.Phase) {
			continue
		}
		if worktree := cleanAbsPath(rec.WorktreePath); worktree != "" {
			expected = append(expected, worktree)
		}
	}
	return expected
}

// expectedWorktreeFromSessionBinding reads the session-to-cycle binding for a
// repo and returns the expected worktree path, or empty when unbound or the
// worktree field is not set.
func expectedWorktreeFromSessionBinding(repo string) string {
	b, ok := branchMatchedIssueOpsSessionBinding(repo)
	if !ok {
		return ""
	}
	return cleanAbsPath(b.ExpectedWorktree)
}

func branchMatchedIssueOpsSessionBinding(repo string) (issueops.SessionBinding, bool) {
	repo = cleanAbsPath(repo)
	if repo == "" {
		return issueops.SessionBinding{}, false
	}
	branch := gitBranchFromHead(repo)
	primary, primaryErr := readIssueOpsSession(repo)
	bindings, err := listIssueOpsSessionBindings(repo)
	if err == nil {
		for _, binding := range bindings {
			if primaryErr == nil && primary.CycleID != "" && binding.CycleID == primary.CycleID {
				continue
			}
			if sessionBindingMatchesBranch(binding, branch) {
				return binding, true
			}
		}
		if primaryErr == nil && sessionBindingMatchesBranch(primary, branch) {
			return primary, true
		}
	}
	binding, err := readIssueOpsSession(repo)
	if err != nil || binding.CycleID == "" {
		return issueops.SessionBinding{}, false
	}
	if !sessionBindingMatchesBranch(binding, branch) {
		return issueops.SessionBinding{}, false
	}
	return binding, true
}

func issueOpsSessionBindingForMirrorGuard(repo string) (issueops.SessionBinding, bool) {
	repo = cleanAbsPath(repo)
	if repo == "" {
		return issueops.SessionBinding{}, false
	}
	branch := gitBranchFromHead(repo)
	primary, primaryErr := readIssueOpsSession(repo)
	bindings, err := listIssueOpsSessionBindings(repo)
	if err == nil {
		for _, binding := range bindings {
			if primaryErr == nil && primary.CycleID != "" && binding.CycleID == primary.CycleID {
				continue
			}
			if sessionBindingMatchesBranch(binding, branch) {
				return binding, true
			}
		}
	}
	if primaryErr == nil && strings.TrimSpace(primary.CycleID) != "" {
		return primary, true
	}
	return issueops.SessionBinding{}, false
}

func sessionBindingMatchesBranch(binding issueops.SessionBinding, branch string) bool {
	if strings.TrimSpace(binding.CycleID) == "" {
		return false
	}
	bindingBranch := strings.TrimSpace(binding.Branch)
	return bindingBranch == "" || bindingBranch == strings.TrimSpace(branch)
}

func pathEqualsAny(path string, candidates []string) bool {
	p := cleanAbsPath(path)
	for _, candidate := range candidates {
		if p != "" && p == cleanAbsPath(candidate) {
			return true
		}
	}
	return false
}
