package mcpcli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-harness/cmd/harness/mcpcli/argmap"
	"agent-harness/internal/core"
)

func handleIssueOpsMCPToolCall(call MCPToolCall) MCPToolOutcome {
	switch call.Name {
	case "issueops_start":
		result, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{
			Repo:   argmap.String(call.Arguments, "repo"),
			Branch: argmap.String(call.Arguments, "branch"),
		})
		return issueOpsMCPOutcome(result, err, "IssueOps start failed")
	case "issueops_status":
		result, err := core.ReadIssueOps(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"))
		return issueOpsMCPOutcome(result, err, "IssueOps status failed")
	case "issueops_record_intent":
		result, err := core.RecordIssueOpsIntent(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"), core.IssueOpsIntentRecordRequest{
			RawRequest:        argmap.String(call.Arguments, "raw_request"),
			InterpretedIntent: argmap.String(call.Arguments, "interpreted_intent"),
			SuccessCriteria:   argmap.StringSlice(call.Arguments, "success_criteria"),
			Constraints:       argmap.StringSlice(call.Arguments, "constraints"),
			Ambiguities:       argmap.StringSlice(call.Arguments, "ambiguities"),
			NonGoals:          argmap.StringSlice(call.Arguments, "non_goals"),
		})
		return issueOpsMCPOutcome(result, err, "IssueOps intent record failed")
	case "issueops_review_design":
		result, err := core.RecordIssueOpsDesignReview(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"), core.IssueOpsDesignReviewRequest{
			ProblemSummary: argmap.String(call.Arguments, "problem_summary"),
			ProposedDesign: argmap.String(call.Arguments, "proposed_design"),
			RefactorPlan:   argmap.String(call.Arguments, "refactor_plan"),
			Alternatives:   argmap.StringSlice(call.Arguments, "alternatives"),
			Risks:          argmap.StringSlice(call.Arguments, "risks"),
			Verification:   argmap.StringSlice(call.Arguments, "verification"),
			OpenQuestions:  argmap.StringSlice(call.Arguments, "open_questions"),
			Approved:       argmap.Bool(call.Arguments, "approved"),
		})
		return issueOpsMCPOutcome(result, err, "IssueOps design review failed")
	case "issueops_link_issue":
		result, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"), argmap.String(call.Arguments, "issue_url"))
		return issueOpsMCPOutcome(result, err, "IssueOps issue link failed")
	case "issueops_link_plan":
		result, err := core.LinkIssueOpsPlan(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"), argmap.String(call.Arguments, "plan_path"))
		return issueOpsMCPOutcome(result, err, "IssueOps plan link failed")
	case "issueops_link_worktree":
		result, err := core.LinkIssueOpsWorktree(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"), argmap.String(call.Arguments, "worktree_path"))
		return issueOpsMCPOutcome(result, err, "IssueOps worktree link failed")
	case "issueops_prepare_worktree_tools":
		record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"))
		if err == nil {
			var result any
			result, err = PrepareIssueOpsWorktreeTools(record)
			return issueOpsMCPOutcome(result, err, "IssueOps worktree tool preparation failed")
		}
		return issueOpsMCPOutcome(nil, err, "IssueOps worktree tool preparation failed")
	case "issueops_link_child":
		if err := VerifyIssueOpsChildIssueBeforeLink(argmap.String(call.Arguments, "child_url")); err != nil {
			return issueOpsMCPOutcome(nil, err, "IssueOps child link failed")
		}
		result, err := core.LinkIssueOpsChild(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"), argmap.String(call.Arguments, "child_url"), argmap.String(call.Arguments, "title"))
		return issueOpsMCPOutcome(result, err, "IssueOps child link failed")
	case "issueops_link_related":
		result, err := core.LinkIssueOpsRelated(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"), argmap.String(call.Arguments, "type"), argmap.String(call.Arguments, "related_url"), argmap.String(call.Arguments, "title"))
		return issueOpsMCPOutcome(result, err, "IssueOps related link failed")
	case "issueops_prepare_branch":
		result, err := core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"), core.IssueOpsBranchPrepareRequest{
			Provider:        argmap.String(call.Arguments, "provider"),
			IssueURL:        argmap.String(call.Arguments, "issue_url"),
			Branch:          argmap.String(call.Arguments, "branch"),
			BaseBranch:      argmap.String(call.Arguments, "base_branch"),
			BaseSHA:         argmap.String(call.Arguments, "base_sha"),
			RemoteBranchURL: argmap.String(call.Arguments, "remote_branch_url"),
			LinkVerified:    argmap.Bool(call.Arguments, "link_verified"),
		})
		return issueOpsMCPOutcome(result, err, "IssueOps branch prepare failed")
	case "issueops_add_feedback":
		result, err := core.AddIssueOpsFeedback(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"), argmap.String(call.Arguments, "source"), argmap.String(call.Arguments, "body"), argmap.String(call.Arguments, "classification"))
		return issueOpsMCPOutcome(result, err, "IssueOps feedback failed")
	case "issueops_add_decision":
		result, err := core.AddIssueOpsDecision(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"), core.IssueOpsDecisionRecordRequest{
			Title:              argmap.String(call.Arguments, "title"),
			Body:               argmap.String(call.Arguments, "body"),
			Kind:               argmap.String(call.Arguments, "kind"),
			Rationale:          argmap.String(call.Arguments, "rationale"),
			Alternatives:       argmap.StringSlice(call.Arguments, "alternatives"),
			AffectedIssueLinks: argmap.StringSlice(call.Arguments, "affected_issue_links"),
			AffectedArtifacts:  argmap.StringSlice(call.Arguments, "affected_artifacts"),
		})
		return issueOpsMCPOutcome(result, err, "IssueOps decision record failed")
	case "issueops_mark_issue_updated":
		result, err := core.MarkIssueOpsContractFeedbackIssueUpdated(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"))
		return issueOpsMCPOutcome(result, err, "IssueOps issue update mark failed")
	case "issueops_set_phase":
		phase := argmap.String(call.Arguments, "phase")
		if strings.TrimSpace(phase) == "" {
			phase = argmap.String(call.Arguments, "to")
		}
		if argmap.Bool(call.Arguments, "force") && phase == "done" {
			result, err := core.ForceDoneIssueOps(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"))
			return issueOpsMCPOutcome(result, err, "IssueOps force-done failed")
		}
		result, err := core.AdvanceIssueOpsPhase(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"), phase)
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
		record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"))
		if err != nil {
			return issueOpsMCPOutcome(nil, err, "IssueOps PR readiness failed")
		}
		if argmap.Bool(call.Arguments, "strict") {
			return mcpToolPayload(core.IssueOpsStrictPRReadiness(record))
		}
		return mcpToolPayload(core.IssueOpsPRReadiness(record))
	case "issueops_cleanup_status":
		result, err := core.IssueOpsCleanupStatusByID(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"), core.IssueOpsCleanupStatusRequest{
			Merged: IssueOpsCleanupMerged(argmap.String(call.Arguments, "id"), argmap.Bool(call.Arguments, "merged")),
		})
		return issueOpsMCPOutcome(result, err, "IssueOps cleanup status failed")
	case "issueops_force_release":
		result, err := core.ForceReleaseIssueOps(core.IssueOpsStateRoot(), argmap.String(call.Arguments, "id"), argmap.String(call.Arguments, "reason"))
		return issueOpsMCPOutcome(result, err, "IssueOps force-release failed")
	case "issueops_cleanup_stale":
		result := core.ScanStaleIssueOpsCycles(core.IssueOpsStaleScanRequest{
			Repo:   argmap.String(call.Arguments, "repo"),
			MaxAge: time.Duration(argmap.Int(call.Arguments, "max_age", 14)) * 24 * time.Hour,
			Apply:  argmap.Bool(call.Arguments, "apply"),
		})
		if !result.OK {
			return issueOpsMCPOutcome(nil, fmt.Errorf("%s", strings.Join(result.Errors, "; ")), "IssueOps stale cleanup failed")
		}
		return mcpToolPayload(result)
	case "issueops_remote_create_issue":
		return handleMCPRemoteCreateIssue(call.Arguments)
	case "issueops_remote_create_pr":
		return handleMCPRemoteCreatePR(call.Arguments)
	case "issueops_remote_sync_graph":
		return handleMCPRemoteSyncGraph(call.Arguments)
	case "issueops_resume":
		result := core.IssueOpsResume(argmap.String(call.Arguments, "repo"))
		if argmap.Bool(call.Arguments, "bind") && result.OK && result.Bound {
			if err := core.BindIssueOpsSession(result.Repo, result.CycleID, result.Branch, result.WorktreePath); err != nil {
				return issueOpsMCPOutcome(nil, fmt.Errorf("resume bind: %w", err), "IssueOps resume bind failed")
			}
		}
		return mcpToolPayload(result)
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
		Provider:  argmap.String(args, "provider"),
		Kind:      argmap.String(args, "kind"),
		URL:       argmap.String(args, "url"),
		Labels:    argmap.StringSlice(args, "labels"),
		Assignees: argmap.StringSlice(args, "assignees"),
	}
	_, err := core.ValidateIssueOpsRemoteArtifactVerification(core.IssueOpsStateRoot(), argmap.String(args, "id"), req)
	if err == nil {
		err = VerifyIssueOpsRemoteArtifactLive(req)
	}
	if err != nil {
		return core.IssueOpsRecord{}, err
	}
	return core.VerifyIssueOpsRemoteArtifact(core.IssueOpsStateRoot(), argmap.String(args, "id"), req)
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

func handleMCPRemoteCreateIssue(args map[string]any) MCPToolOutcome {
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), argmap.String(args, "id"))
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-issue failed: cannot read cycle")
	}
	provider := resolveRecordProviderForMCP(record)
	if provider == "" {
		return issueOpsMCPOutcome(nil, fmt.Errorf("cannot determine provider"), "IssueOps remote create-issue failed")
	}
	result, err := core.CreateRemoteIssue(core.IssueProviderCreateIssueRequest{
		Repo:      record.Repo,
		Title:     argmap.String(args, "title"),
		Body:      argmap.String(args, "body"),
		Labels:    argmap.StringSlice(args, "labels"),
		Assignees: argmap.StringSlice(args, "assignees"),
		Confirm:   argmap.Bool(args, "confirm"),
	}, provider)
	return issueOpsMCPOutcome(result, err, "IssueOps remote create-issue failed")
}

func handleMCPRemoteCreatePR(args map[string]any) MCPToolOutcome {
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), argmap.String(args, "id"))
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-pr failed: cannot read cycle")
	}
	provider := resolveRecordProviderForMCP(record)
	if provider == "" {
		return issueOpsMCPOutcome(nil, fmt.Errorf("cannot determine provider"), "IssueOps remote create-pr failed")
	}
	head := argmap.String(args, "head")
	if head == "" {
		head = record.Branch
	}
	base := argmap.String(args, "base")
	if base == "" && record.BranchPrepare != nil {
		base = record.BranchPrepare.BaseBranch
	}
	result, err := core.CreateRemotePullRequest(core.IssueProviderCreatePullRequestRequest{
		Repo:       record.Repo,
		Title:      argmap.String(args, "title"),
		Body:       argmap.String(args, "body"),
		HeadBranch: head,
		BaseBranch: base,
		Labels:     argmap.StringSlice(args, "labels"),
		Assignees:  argmap.StringSlice(args, "assignees"),
		Confirm:    argmap.Bool(args, "confirm"),
	}, provider)
	return issueOpsMCPOutcome(result, err, "IssueOps remote create-pr failed")
}

func resolveRecordProviderForMCP(record core.IssueOpsRecord) string {
	if record.BranchPrepare != nil && record.BranchPrepare.Provider != "" {
		return record.BranchPrepare.Provider
	}
	if record.RemoteArtifact != nil && record.RemoteArtifact.Provider != "" {
		return record.RemoteArtifact.Provider
	}
	if record.IssueURL != "" {
		if strings.Contains(record.IssueURL, "github.com") {
			return "github"
		}
		if strings.Contains(record.IssueURL, "gitlab") {
			return "gitlab"
		}
	}
	return ""
}

func handleMCPRemoteSyncGraph(args map[string]any) MCPToolOutcome {
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), argmap.String(args, "id"))
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote sync-graph failed: cannot read cycle")
	}
	if !argmap.Bool(args, "confirm") {
		links := len(record.IssueLinks)
		return mcpToolPayload(map[string]any{
			"ok":         true,
			"synced":     false,
			"dry_run":    true,
			"link_count": links,
			"message":    fmt.Sprintf("dry-run: would sync %d issue graph links to %s", links, record.IssueURL),
		})
	}
	result, err := core.SyncRemoteIssueGraph(record)
	return issueOpsMCPOutcome(result, err, "IssueOps remote sync-graph failed")
}
