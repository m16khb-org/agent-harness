package lifecycle

import "strings"

func BuildLifecyclePreToolUseDecision(req HookToolUseLifecycleRequest) HookPreToolUseDecisionResult {
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "pre-tool-use"
	}
	result := HookPreToolUseDecisionResult{
		OK:       true,
		Decision: "allow",
		Tool:     strings.TrimSpace(req.Tool),
		Paths:    append([]string{}, req.Paths...),
		Command:  strings.TrimSpace(req.Command),
		Source:   source,
	}
	if req.EnforceSearchRouting {
		if reason := searchRoutingBlockReason(result.Tool, result.Command, req.Repo); reason != "" {
			result.Decision = "block"
			result.Reason = reason
		}
	}
	if result.Decision != "block" && req.EnforceWorktree {
		if reason := mcpWorktreeRootBlockReason(req); reason != "" {
			result.Decision = "block"
			result.Reason = reason
		}
	}
	if result.Decision != "block" && req.EnforceWorktree {
		if reason := worktreeGuardBlockReason(req); reason != "" {
			result.Decision = "block"
			result.Reason = reason
		}
	}
	if result.Decision != "block" && req.EnforceKoreanRemote {
		if reason := koreanRemoteArtifactBlockReason(req); reason != "" {
			result.Decision = "block"
			result.Reason = reason
		}
	}
	if result.Decision != "block" && req.EnforceVCSLinking {
		if reason := vcsIssueLinkingBlockReason(req); reason != "" {
			result.Decision = "block"
			result.Reason = reason
		}
	}
	if result.Decision != "block" && req.EnforceStagedChecks {
		if decision, reason := stagedCheckDecision(req); decision != "" {
			result.Decision = decision
			result.Reason = reason
		}
	}
	if result.Decision != "block" && req.EnforceGitOpsKubectl {
		if decision, reason := gitOpsKubectlDecision(req.Tool, req.Command); decision != "" {
			result.Decision = decision
			result.Reason = reason
		}
	}
	return result
}
