package lifecycle

import (
	"strings"

	"agent-harness/internal/core/commandguard"
	"agent-harness/internal/core/lifecycle/nextactionrelay"
	"agent-harness/internal/core/remoteartifact"
	"agent-harness/internal/core/searchrouting"
)

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
		if reason := searchrouting.SearchRoutingBlockReason(result.Tool, result.Command, req.Repo); reason != "" {
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
		if reason := remoteartifact.KoreanBlockReason(req.Tool, req.Command, req.Repo); reason != "" {
			result.Decision = "block"
			result.Reason = reason
		}
	}
	if result.Decision != "block" && req.EnforceVCSLinking {
		if reason := remoteartifact.VCSIssueLinkingBlockReason(req.Tool, req.Command, req.Repo); reason != "" {
			result.Decision = "block"
			result.Reason = reason
		}
	}
	if result.Decision != "block" && req.EnforceStagedChecks {
		if decision, reason := commandguard.StagedCheckDecision(req.Tool, req.Repo, req.Command); decision != "" {
			result.Decision = decision
			result.Reason = reason
		}
	}
	if result.Decision != "block" && req.EnforceGitOpsKubectl {
		if decision, reason := commandguard.GitOpsKubectlDecision(req.Tool, req.Command); decision != "" {
			result.Decision = decision
			result.Reason = reason
		}
	}
	return result
}

func RecordStopNextActionRelay(repoRoot string, trigger NextActionJudgementTriggerResult) StopNextActionRelayResult {
	return nextactionrelay.Record(stopNextActionRelayStore(), repoRoot, trigger)
}

func ClearStopNextActionRelay(repoRoot string) StopNextActionRelayResult {
	return nextactionrelay.Clear(stopNextActionRelayStore(), repoRoot)
}

func stopNextActionRelayStore() nextactionrelay.Store {
	return nextactionrelay.Store{
		Validate: ValidateProjectLifecycleState,
		Init: func(repoRoot string, confirm bool) (ProjectLifecycleStatePlan, error) {
			return InitProjectLifecycleState(repoRoot, confirm)
		},
		WriteJSON: writeJSONAtomic,
	}
}
