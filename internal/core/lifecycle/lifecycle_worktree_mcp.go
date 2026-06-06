package lifecycle

import "strings"

func mcpWorktreeRootBlockReason(req HookToolUseLifecycleRequest) string {
	expected := expectedIssueOpsWorktreesForMCPGuard(req)
	if len(expected) == 0 {
		return ""
	}
	primary := expected[0]
	tool := strings.ToLower(strings.TrimSpace(req.Tool))
	switch {
	case isCodeGraphTool(tool):
		projectPath := cleanAbsPath(req.ProjectPath)
		if projectPath == "" {
			return "CodeGraph in an IssueOps worktree must set projectPath to the expected IssueOps worktree: " + primary
		}
		if !pathEqualsAny(projectPath, expected) {
			return "CodeGraph projectPath is outside the expected IssueOps worktree; set projectPath to " + primary
		}
	case strings.Contains(tool, "filesystem") || strings.Contains(tool, "serena"):
		return "source-root-bound MCP tool is not allowed during IssueOps worktree implementation; use native absolute-path file tools, rg rooted at the IssueOps worktree, git -C, or CodeGraph with projectPath " + primary
	}
	return ""
}

func expectedIssueOpsWorktreesForMCPGuard(req HookToolUseLifecycleRequest) []string {
	if expected := cleanAbsPath(req.ExpectedWorktree); expected != "" {
		return []string{expected}
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

func pathEqualsAny(path string, candidates []string) bool {
	p := cleanAbsPath(path)
	for _, candidate := range candidates {
		if p != "" && p == cleanAbsPath(candidate) {
			return true
		}
	}
	return false
}
