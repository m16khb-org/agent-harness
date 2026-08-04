package lifecycle

import (
	"fmt"
	"strings"

	lifecyclecontract "agent-harness/internal/contract/lifecycle"

	"agent-harness/internal/core/commandguard"
	"agent-harness/internal/core/lifecycle/liveapproval"
	"agent-harness/internal/core/lifecycle/nextactionrelay"
	"agent-harness/internal/core/lifecycle/worktreeguard"
	"agent-harness/internal/core/remoteartifact"
)

func BuildLifecyclePreToolUseDecision(req lifecyclecontract.HookToolUseLifecycleRequest) lifecyclecontract.HookPreToolUseDecisionResult {
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "pre-tool-use"
	}
	result := lifecyclecontract.HookPreToolUseDecisionResult{
		OK:       true,
		Decision: "allow",
		Tool:     strings.TrimSpace(req.Tool),
		Paths:    append([]string{}, req.Paths...),
		Command:  strings.TrimSpace(req.Command),
		Source:   source,
	}
	if reason, deny := generatedIssueOpsExecutableBlock(req); reason != "" {
		result.Decision = "block"
		result.Reason = reason
		result.Deny = deny
		return result
	}
	observation := executionObservation(req)
	if !observation && childHostSmokeInvocation(req) {
		reason := "child host smoke command does not match the exact released delegation contract"
		result.Decision = "block"
		result.Reason = reason
		result.Deny = &lifecyclecontract.IssueOpsDenyReason{Code: "unsafe_mutation", Reason: reason}
		return result
	}
	typedExecutionControl := executionTypedControlPlane(req)
	if !observation && typedExecutionControl {
		if reason, deny := executionTypedPreLinkBlock(req); reason != "" {
			result.Decision = "block"
			result.Reason = reason
			result.Deny = deny
			if result.Deny != nil && result.Deny.Reason == "" {
				result.Deny.Reason = reason
			}
		}
	}
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
			// 구조화된 deny는 host hook 출력에서 result.Reason을 대체한다
			// (hookDenyReason). 사유를 deny 안에 함께 실어야 코드만 보고
			// 추측 재시도하는 일이 없다(이슈 #154).
			if result.Deny != nil && result.Deny.Reason == "" {
				result.Deny.Reason = executionReason
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
	if result.Decision != "block" && req.EnforceVCSLinking {
		if reason := sealedIssueEditBlockReason(req); reason != "" {
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

func directBranchCreationBlockReason(req lifecyclecontract.HookToolUseLifecycleRequest) string {
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

func applyCodexLiveApproval(result *lifecyclecontract.HookPreToolUseDecisionResult, approval liveapproval.Result) {
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

func RecordStopNextActionRelay(repoRoot string, trigger NextActionJudgementTriggerResult) lifecyclecontract.StopNextActionRelayResult {
	return nextactionrelay.Record(stopNextActionRelayStore(), repoRoot, trigger)
}

func ReadStopNextActionRelay(repoRoot string) (lifecyclecontract.StopNextActionRelayRecord, bool) {
	return nextactionrelay.Read(stopNextActionRelayStore(), repoRoot)
}

func ClearStopNextActionRelay(repoRoot string) lifecyclecontract.StopNextActionRelayResult {
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
