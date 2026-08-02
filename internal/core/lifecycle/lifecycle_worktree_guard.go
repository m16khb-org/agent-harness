package lifecycle

import lifecyclecontract "agent-harness/internal/contract/lifecycle"

func worktreeGuardBlockReason(req lifecyclecontract.HookToolUseLifecycleRequest) string {
	if !toolUseMayMutateLifecycleFiles(req.Tool, req.Command) {
		return ""
	}
	expected := cleanAbsPath(req.ExpectedWorktree)
	if expected == "" {
		return ""
	}
	targets := worktreeGuardEditTargets(req)
	if len(targets) == 0 {
		return ""
	}
	for _, target := range targets {
		if !pathWithin(target, expected) {
			return "mutating tool target is outside expected IssueOps worktree; set cwd/target path to the isolated worktree before editing"
		}
	}
	return ""
}
