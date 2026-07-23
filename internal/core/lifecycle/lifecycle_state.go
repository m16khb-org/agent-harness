package lifecycle

import (
	"fmt"
	"strings"

	"agent-harness/internal/core/commandguard"
	"agent-harness/internal/core/lifecycle/liveapproval"
	"agent-harness/internal/core/lifecycle/nextactionrelay"
	"agent-harness/internal/core/lifecycle/worktreeguard"
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
	observation := executionObservation(req)
	typedExecutionControl := executionTypedControlPlane(req)
	if !observation && !typedExecutionControl {
		if req.EnforceWorktree {
			if reason := directBranchCreationBlockReason(req); reason != "" {
				result.Decision = "block"
				result.Reason = reason
			}
		}
		executionHandled := false
		if result.Decision != "block" {
			var executionReason string
			executionHandled, executionReason, result.Deny = executionMutationDecision(req)
			if executionReason != "" {
				result.Decision = "block"
				result.Reason = executionReason
			}
		}
		if !executionHandled && result.Decision != "block" && req.EnforceWorktree {
			if reason := worktreeGuardBlockReason(req); reason != "" {
				result.Decision = "block"
				result.Reason = reason
			}
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
		evaluation := commandguard.EvaluateGitOpsKubectl(req.Tool, req.Command)
		if evaluation.Decision != "" {
			result.Decision = evaluation.Decision
			result.Reason = evaluation.Reason
			if evaluation.Decision == "ask" && strings.EqualFold(strings.TrimSpace(req.Host), "codex") {
				switch evaluation.LiveAccess {
				case commandguard.KubectlLiveAccessPortForward:
					applyCodexLiveApproval(&result, liveapproval.Evaluate(liveApprovalStore(), liveapproval.Request{
						Host:      req.Host,
						SessionID: req.SessionID,
						RepoRoot:  req.Repo,
						CWD:       req.CWD,
						Tool:      req.Tool,
						Command:   req.Command,
					}))
				case commandguard.KubectlLiveAccessReadOnlyExec:
					applyCodexLiveApproval(&result, liveapproval.EvaluateReadOnlyExec(liveApprovalStore(), liveapproval.ReadOnlyExecRequest{
						Host:      req.Host,
						SessionID: req.SessionID,
						RepoRoot:  req.Repo,
						CWD:       req.CWD,
						Tool:      req.Tool,
						Command:   req.Command,
						Context:   evaluation.ExecScope.Context,
						Namespace: evaluation.ExecScope.Namespace,
					}))
				case commandguard.KubectlLiveAccessUnsafeExec:
					result.Decision = "block"
					result.Reason = "kubectl exec is blocked: Codex session approval is limited to explicit-context and explicit-namespace DNS, resolver, and Linkerd metrics diagnostics."
				default:
					result.Decision = "block"
					result.Reason = "kubectl live-access approval unavailable: the request could not be classified safely."
				}
			}
		}
	}
	return result
}

func directBranchCreationBlockReason(req HookToolUseLifecycleRequest) string {
	creation := worktreeguard.LocalIssueOpsBranchCreation(req.Command)
	if strings.TrimSpace(creation.Branch) != "" {
		if strings.TrimSpace(creation.SourceRef) == "" {
			return worktreeguard.IssueOpsBranchCreationSourceReason(creation.Branch)
		}
		return fmt.Sprintf("branch %s must be started through IssueOps; direct Git branch creation is blocked, use `agent-harness issueops execution prepare --id ID --mode auto ...`", creation.Branch)
	}
	if worktreeguard.DirectGitWorktreeMutation(req.Command) {
		return "direct Git worktree mutation is blocked; use `agent-harness issueops execution prepare --id ID --mode auto ...` so IssueOps creates the canonical isolated worktree"
	}
	return ""
}

func applyCodexLiveApproval(result *HookPreToolUseDecisionResult, approval liveapproval.Result) {
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
