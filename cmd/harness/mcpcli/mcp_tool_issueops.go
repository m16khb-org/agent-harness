package mcpcli

import (
	"encoding/json"
	"fmt"
	"strings"

	"agent-harness/internal/core"
)

func handleIssueOpsMCPToolCall(call MCPToolCall) MCPToolOutcome {
	switch call.Name {
	case "issueops_start":
		result, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{
			Repo:   stringArg(call.Arguments, "repo"),
			Branch: stringArg(call.Arguments, "branch"),
		})
		return issueOpsMCPOutcome(result, err, "IssueOps start failed")
	case "issueops_status":
		result, err := core.ReadIssueOps(core.IssueOpsStateRoot(), stringArg(call.Arguments, "id"))
		return issueOpsMCPOutcome(result, err, "IssueOps status failed")
	case "issueops_link_issue":
		result, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), stringArg(call.Arguments, "id"), stringArg(call.Arguments, "issue_url"))
		return issueOpsMCPOutcome(result, err, "IssueOps issue link failed")
	case "issueops_link_plan":
		result, err := core.LinkIssueOpsPlan(core.IssueOpsStateRoot(), stringArg(call.Arguments, "id"), stringArg(call.Arguments, "plan_path"))
		return issueOpsMCPOutcome(result, err, "IssueOps plan link failed")
	case "issueops_link_worktree":
		result, err := core.LinkIssueOpsWorktree(core.IssueOpsStateRoot(), stringArg(call.Arguments, "id"), stringArg(call.Arguments, "worktree_path"))
		return issueOpsMCPOutcome(result, err, "IssueOps worktree link failed")
	case "issueops_prepare_worktree_tools":
		record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), stringArg(call.Arguments, "id"))
		if err == nil {
			var result any
			result, err = PrepareIssueOpsWorktreeTools(record)
			return issueOpsMCPOutcome(result, err, "IssueOps worktree tool preparation failed")
		}
		return issueOpsMCPOutcome(nil, err, "IssueOps worktree tool preparation failed")
	case "issueops_link_child":
		if err := VerifyIssueOpsChildIssueBeforeLink(stringArg(call.Arguments, "child_url")); err != nil {
			return issueOpsMCPOutcome(nil, err, "IssueOps child link failed")
		}
		result, err := core.LinkIssueOpsChild(core.IssueOpsStateRoot(), stringArg(call.Arguments, "id"), stringArg(call.Arguments, "child_url"), stringArg(call.Arguments, "title"))
		return issueOpsMCPOutcome(result, err, "IssueOps child link failed")
	case "issueops_prepare_branch":
		result, err := core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), stringArg(call.Arguments, "id"), core.IssueOpsBranchPrepareRequest{
			Provider:        stringArg(call.Arguments, "provider"),
			IssueURL:        stringArg(call.Arguments, "issue_url"),
			Branch:          stringArg(call.Arguments, "branch"),
			BaseBranch:      stringArg(call.Arguments, "base_branch"),
			BaseSHA:         stringArg(call.Arguments, "base_sha"),
			RemoteBranchURL: stringArg(call.Arguments, "remote_branch_url"),
			LinkVerified:    boolArg(call.Arguments, "link_verified"),
		})
		return issueOpsMCPOutcome(result, err, "IssueOps branch prepare failed")
	case "issueops_add_feedback":
		result, err := core.AddIssueOpsFeedback(core.IssueOpsStateRoot(), stringArg(call.Arguments, "id"), stringArg(call.Arguments, "source"), stringArg(call.Arguments, "body"), stringArg(call.Arguments, "classification"))
		return issueOpsMCPOutcome(result, err, "IssueOps feedback failed")
	case "issueops_mark_issue_updated":
		result, err := core.MarkIssueOpsContractFeedbackIssueUpdated(core.IssueOpsStateRoot(), stringArg(call.Arguments, "id"))
		return issueOpsMCPOutcome(result, err, "IssueOps issue update mark failed")
	case "issueops_set_phase":
		phase := stringArg(call.Arguments, "phase")
		if strings.TrimSpace(phase) == "" {
			phase = stringArg(call.Arguments, "to")
		}
		result, err := core.AdvanceIssueOpsPhase(core.IssueOpsStateRoot(), stringArg(call.Arguments, "id"), phase)
		return issueOpsMCPOutcome(result, err, "IssueOps phase advance failed")
	case "issueops_verify_remote_artifact":
		result, err := verifyIssueOpsRemoteArtifactFromMCP(call.Arguments)
		return issueOpsMCPOutcome(result, err, "IssueOps remote artifact verification failed")
	case "issueops_remote_score":
		req, err := issueOpsRemoteScoringRequestFromMCP(call.Arguments)
		if err == nil {
			var result core.IssueOpsRemoteScoringResult
			result, err = core.ScoreIssueOpsRemoteCandidates(req)
			return issueOpsMCPOutcome(result, err, "IssueOps remote score failed")
		}
		return issueOpsMCPOutcome(nil, err, "IssueOps remote score failed")
	case "issueops_pr_readiness":
		record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), stringArg(call.Arguments, "id"))
		if err != nil {
			return issueOpsMCPOutcome(nil, err, "IssueOps PR readiness failed")
		}
		if boolArg(call.Arguments, "strict") {
			return mcpToolPayload(core.IssueOpsStrictPRReadiness(record))
		}
		return mcpToolPayload(core.IssueOpsPRReadiness(record))
	case "issueops_cleanup_status":
		result, err := core.IssueOpsCleanupStatusByID(core.IssueOpsStateRoot(), stringArg(call.Arguments, "id"), core.IssueOpsCleanupStatusRequest{
			Merged: IssueOpsCleanupMerged(stringArg(call.Arguments, "id"), boolArg(call.Arguments, "merged")),
		})
		return issueOpsMCPOutcome(result, err, "IssueOps cleanup status failed")
	default:
		return MCPToolOutcome{}
	}
}

func issueOpsMCPOutcome(payload any, err error, message string) MCPToolOutcome {
	if err != nil {
		return mcpToolFailure(&RPCError{Code: -32602, Message: message, Data: err.Error()})
	}
	return mcpToolPayload(payload)
}

func verifyIssueOpsRemoteArtifactFromMCP(args map[string]any) (core.IssueOpsRecord, error) {
	req := core.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  stringArg(args, "provider"),
		Kind:      stringArg(args, "kind"),
		URL:       stringArg(args, "url"),
		Labels:    stringSliceArg(args, "labels"),
		Assignees: stringSliceArg(args, "assignees"),
	}
	_, err := core.ValidateIssueOpsRemoteArtifactVerification(core.IssueOpsStateRoot(), stringArg(args, "id"), req)
	if err == nil {
		err = VerifyIssueOpsRemoteArtifactLive(req)
	}
	if err != nil {
		return core.IssueOpsRecord{}, err
	}
	return core.VerifyIssueOpsRemoteArtifact(core.IssueOpsStateRoot(), stringArg(args, "id"), req)
}

func issueOpsRemoteScoringRequestFromMCP(args map[string]any) (core.IssueOpsRemoteScoringRequest, error) {
	var req core.IssueOpsRemoteScoringRequest
	b, err := json.Marshal(args)
	if err != nil {
		return req, err
	}
	req, err = core.DecodeIssueOpsRemoteScoringRequest(b)
	if err != nil {
		return req, fmt.Errorf("invalid issueops remote scoring request: %w", err)
	}
	return req, nil
}
