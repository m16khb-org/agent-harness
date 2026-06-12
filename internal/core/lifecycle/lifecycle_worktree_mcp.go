package lifecycle

import (
	"strings"

	"agent-harness/internal/core/searchrouting"
)

func mcpWorktreeRootBlockReason(req HookToolUseLifecycleRequest) string {
	expected := expectedIssueOpsWorktreesForMCPGuard(req)
	if len(expected) == 0 {
		return ""
	}
	primary := expected[0]
	tool := strings.ToLower(strings.TrimSpace(req.Tool))
	switch {
	case searchrouting.IsCodeGraphTool(tool):
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
	b, err := readIssueOpsSession(repo)
	if err != nil || b.CycleID == "" {
		return ""
	}
	// The binding is repo-scoped, not session-scoped: apply it only when the
	// session is actually on the bound branch, so a binding written by one
	// cycle never blocks unrelated work (e.g. main-branch source edits) in
	// another session of the same repo.
	if b.Branch != "" && b.Branch != gitBranchFromHead(repo) {
		return ""
	}
	return cleanAbsPath(b.ExpectedWorktree)
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
