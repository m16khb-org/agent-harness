package lifecycle

import (
	"strings"

	"agent-harness/internal/core/commandguard"
	"agent-harness/internal/core/lifecycle/liveapproval"
	"agent-harness/internal/core/lifecycle/nextactionrelay"
	"agent-harness/internal/core/remoteartifact"
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
	if reason := linkedWorktreeDecisionGateReason(req); reason != "" {
		result.Decision = "block"
		result.Reason = reason
		return result
	}
	if reason := unsafeOrcaMailboxInjectReason(req); reason != "" {
		result.Decision = "block"
		result.Reason = reason
		return result
	}
	if reason := invalidOrcaOrchestrationMessageTypeReason(req); reason != "" {
		result.Decision = "block"
		result.Reason = reason
		return result
	}
	handoffHandled, handoffReason := handoffOwnershipBlockReason(req)
	if handoffReason != "" {
		result.Decision = "block"
		result.Reason = handoffReason
	}
	if !handoffHandled && req.EnforceWorktree {
		if reason := mcpWorktreeRootBlockReason(req); reason != "" {
			result.Decision = "block"
			result.Reason = reason
		}
	}
	if !handoffHandled && result.Decision != "block" && req.EnforceWorktree {
		if reason := worktreeGuardBlockReason(req); reason != "" {
			result.Decision = "block"
			result.Reason = reason
		}
	}
	if !handoffHandled && result.Decision != "block" && req.EnforceWorktree {
		if decision, reason := sourceCheckoutMirrorEditAskReason(req); decision != "" {
			result.Decision = decision
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
	if result.Decision != "block" && req.EnforceVCSLinking {
		if reason := issueOpsPRTargetBranchBlockReason(req); reason != "" {
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
			if decision == "ask" && strings.EqualFold(strings.TrimSpace(req.Host), "codex") {
				approval := liveapproval.Evaluate(liveApprovalStore(), liveapproval.Request{
					Host:      req.Host,
					SessionID: req.SessionID,
					RepoRoot:  req.Repo,
					CWD:       req.CWD,
					Tool:      req.Tool,
					Command:   req.Command,
				})
				switch {
				case approval.Allowed:
					result.Decision = "allow"
					result.Reason = ""
				case approval.Token != "":
					result.Decision = "ask"
					result.Reason = approval.Reason
				default:
					result.Decision = "block"
					result.Reason = approval.Reason
				}
			} else {
				result.Decision = decision
				result.Reason = reason
			}
		}
	}
	return result
}

func ApproveCodexKubectlLiveAccess(repo, host, sessionID, prompt string) liveapproval.Result {
	return liveapproval.Approve(liveApprovalStore(), liveapproval.ApprovalRequest{
		Host:      host,
		SessionID: sessionID,
		RepoRoot:  repo,
		Prompt:    prompt,
	})
}

func RecordStopNextActionRelay(repoRoot string, trigger NextActionJudgementTriggerResult) StopNextActionRelayResult {
	return nextactionrelay.Record(stopNextActionRelayStore(), repoRoot, trigger)
}

func ReadStopNextActionRelay(repoRoot string) (StopNextActionRelayRecord, bool) {
	return nextactionrelay.Read(stopNextActionRelayStore(), repoRoot)
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
