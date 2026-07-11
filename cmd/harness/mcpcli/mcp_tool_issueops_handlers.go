package mcpcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-harness/cmd/harness/mcpcli/argmap"
	"agent-harness/internal/adapter/orca"
	"agent-harness/internal/adapter/provider"
	"agent-harness/internal/core"
)

// This file holds one handler per IssueOps MCP tool. handleIssueOpsMCPToolCall
// (mcp_tool_issueops.go) dispatches through the issueOpsMCPHandlers registry so
// routing is a single map lookup instead of a high-branch switch, and each tool
// stays a small, independently testable function.

func handleMCPIssueOpsStart(args map[string]any) MCPToolOutcome {
	result, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{
		Repo:   argmap.String(args, "repo"),
		Branch: argmap.String(args, "branch"),
	})
	return issueOpsMCPOutcome(result, err, "IssueOps start failed")
}

func handleMCPIssueOpsStatus(args map[string]any) MCPToolOutcome {
	result, err := core.IssueOpsStatus(core.IssueOpsStateRoot(), argmap.String(args, "id"))
	return issueOpsMCPOutcome(result, err, "IssueOps status failed")
}

func handleMCPIssueOpsRecordIntent(args map[string]any) MCPToolOutcome {
	result, err := core.RecordIssueOpsIntent(core.IssueOpsStateRoot(), argmap.String(args, "id"), core.IssueOpsIntentRecordRequest{
		RawRequest:        argmap.String(args, "raw_request"),
		InterpretedIntent: argmap.String(args, "interpreted_intent"),
		SuccessCriteria:   argmap.StringSlice(args, "success_criteria"),
		Constraints:       argmap.StringSlice(args, "constraints"),
		Ambiguities:       argmap.StringSlice(args, "ambiguities"),
		NonGoals:          argmap.StringSlice(args, "non_goals"),
		IntentClass:       argmap.String(args, "intent_class"),
	})
	return issueOpsMCPOutcome(result, err, "IssueOps intent record failed")
}

func handleMCPIssueOpsPlanPrepRecord(args map[string]any) MCPToolOutcome {
	result, err := core.RecordIssueOpsPlanPrep(core.IssueOpsStateRoot(), argmap.String(args, "id"), core.IssueOpsPlanPrepRequest{
		PriorDecisions: core.IssueOpsPlanPrepItemRequest{Evidence: argmap.StringSlice(args, "decisions_evidence"), WaiveReason: argmap.String(args, "decisions_waive")},
		RelatedIssues:  core.IssueOpsPlanPrepItemRequest{Evidence: argmap.StringSlice(args, "related_score_ref"), WaiveReason: argmap.String(args, "related_waive")},
		WebResearch:    core.IssueOpsPlanPrepItemRequest{Evidence: argmap.StringSlice(args, "web_research_evidence"), WaiveReason: argmap.String(args, "web_research_waive")},
	})
	return issueOpsMCPOutcome(result, err, "IssueOps plan-prep record failed")
}

func handleMCPIssueOpsReviewDesign(args map[string]any) MCPToolOutcome {
	result, err := core.RecordIssueOpsDesignReview(core.IssueOpsStateRoot(), argmap.String(args, "id"), core.IssueOpsDesignReviewRequest{
		ProblemSummary: argmap.String(args, "problem_summary"),
		ProposedDesign: argmap.String(args, "proposed_design"),
		RefactorPlan:   argmap.String(args, "refactor_plan"),
		Alternatives:   argmap.StringSlice(args, "alternatives"),
		Risks:          argmap.StringSlice(args, "risks"),
		Verification:   argmap.StringSlice(args, "verification"),
		OpenQuestions:  argmap.StringSlice(args, "open_questions"),
		Approved:       argmap.Bool(args, "approved"),
	})
	return issueOpsMCPOutcome(result, err, "IssueOps design review failed")
}

func handleMCPIssueOpsLinkIssue(args map[string]any) MCPToolOutcome {
	result, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), argmap.String(args, "id"), argmap.String(args, "issue_url"))
	return issueOpsMCPOutcome(result, err, "IssueOps issue link failed")
}

func handleMCPIssueOpsLinkPlan(args map[string]any) MCPToolOutcome {
	result, err := core.LinkIssueOpsPlan(core.IssueOpsStateRoot(), argmap.String(args, "id"), argmap.String(args, "plan_path"))
	return issueOpsMCPOutcome(result, err, "IssueOps plan link failed")
}

func handleMCPIssueOpsLinkWorktree(args map[string]any) MCPToolOutcome {
	result, err := core.LinkIssueOpsWorktree(core.IssueOpsStateRoot(), argmap.String(args, "id"), argmap.String(args, "worktree_path"))
	return issueOpsMCPOutcome(result, err, "IssueOps worktree link failed")
}

func handleMCPIssueOpsPrepareWorktreeTools(args map[string]any) MCPToolOutcome {
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), argmap.String(args, "id"))
	if err == nil {
		var result any
		result, err = PrepareIssueOpsWorktreeTools(record)
		if err == nil {
			if prepared, ok := result.(interface {
				IssueOpsWorktreeToolPreparation() core.IssueOpsWorktreeToolPreparation
			}); ok {
				var updated core.IssueOpsRecord
				updated, err = core.RecordIssueOpsWorktreeTools(core.IssueOpsStateRoot(), record.ID, prepared.IssueOpsWorktreeToolPreparation())
				if err == nil && updated.WorktreeTools != nil {
					result = *updated.WorktreeTools
				}
			}
		}
		return issueOpsMCPOutcome(result, err, "IssueOps worktree tool preparation failed")
	}
	return issueOpsMCPOutcome(nil, err, "IssueOps worktree tool preparation failed")
}

func handleMCPIssueOpsRecordExecutionDecision(args map[string]any) MCPToolOutcome {
	plans, err := issueOpsSubagentPlansFromMCP(args["subagent_plans"])
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps execution decision record failed")
	}
	result, err := core.RecordIssueOpsExecutionDecision(core.IssueOpsStateRoot(), argmap.String(args, "id"), core.IssueOpsExecutionDecisionRecordRequest{
		AutoProceed:       argmap.StringSlice(args, "auto_proceed"),
		HookBlocked:       argmap.StringSlice(args, "hook_blocked"),
		HumanGates:        argmap.StringSlice(args, "human_gates"),
		SubagentUse:       argmap.String(args, "subagent_use"),
		SubagentRationale: argmap.String(args, "subagent_rationale"),
		SubagentPlans:     plans,
	})
	return issueOpsMCPOutcome(result, err, "IssueOps execution decision record failed")
}

func handleMCPIssueOpsRecordCompatibilityReview(args map[string]any) MCPToolOutcome {
	result, err := core.RecordIssueOpsCompatibilityReview(core.IssueOpsStateRoot(), argmap.String(args, "id"), core.IssueOpsCompatibilityReviewRequest{
		BackwardCompatibility: argmap.StringSlice(args, "backward_compatibility"),
		SideEffects:           argmap.StringSlice(args, "side_effects"),
		RollbackPlan:          argmap.String(args, "rollback_plan"),
		Verification:          argmap.StringSlice(args, "verification"),
		Blockers:              argmap.StringSlice(args, "blockers"),
		Approved:              argmap.Bool(args, "approved"),
	})
	return issueOpsMCPOutcome(result, err, "IssueOps compatibility review record failed")
}

func handleMCPIssueOpsRecordDevilsAdvocateReview(args map[string]any) MCPToolOutcome {
	result, err := core.RecordIssueOpsDevilsAdvocateReview(core.IssueOpsStateRoot(), argmap.String(args, "id"), core.IssueOpsDevilsAdvocateReviewRequest{
		Verdict:         argmap.String(args, "verdict"),
		Findings:        argmap.StringSlice(args, "findings"),
		Waived:          argmap.Bool(args, "waived"),
		WaiverRationale: argmap.String(args, "waiver_rationale"),
	})
	return issueOpsMCPOutcome(result, err, "IssueOps devils-advocate review record failed")
}

func handleMCPIssueOpsRecordDomainReview(args map[string]any) MCPToolOutcome {
	result, err := core.RecordIssueOpsDomainReview(core.IssueOpsStateRoot(), argmap.String(args, "id"), core.IssueOpsDomainReviewRequest{
		Terminology:       argmap.StringSlice(args, "terminology"),
		ModelFit:          argmap.String(args, "model_fit"),
		Risks:             argmap.StringSlice(args, "risks"),
		OpenUncertainties: argmap.StringSlice(args, "open_uncertainties"),
	})
	return issueOpsMCPOutcome(result, err, "IssueOps domain review record failed")
}

func handleMCPIssueOpsRecordAISlopCleanEvidence(args map[string]any) MCPToolOutcome {
	result, err := core.RecordIssueOpsAISlopCleanEvidence(core.IssueOpsStateRoot(), argmap.String(args, "id"), argmap.StringSlice(args, "categories"), argmap.StringSlice(args, "verification"))
	return issueOpsMCPOutcome(result, err, "IssueOps ai-slop-clean evidence record failed")
}

func handleMCPIssueOpsResolveFeedback(args map[string]any) MCPToolOutcome {
	result, err := core.ResolveIssueOpsFeedback(core.IssueOpsStateRoot(), argmap.String(args, "id"), argmap.Int(args, "index", -1), argmap.String(args, "resolution"))
	return issueOpsMCPOutcome(result, err, "IssueOps resolve feedback failed")
}

func handleMCPIssueOpsRegressForReplan(args map[string]any) MCPToolOutcome {
	result, err := core.RegressIssueOpsForReplan(core.IssueOpsStateRoot(), argmap.String(args, "id"), argmap.String(args, "reason"))
	return issueOpsMCPOutcome(result, err, "IssueOps regress for replan failed")
}

func issueOpsSubagentPlansFromMCP(raw any) ([]core.IssueOpsSubAgentPlan, error) {
	if raw == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var plans []core.IssueOpsSubAgentPlan
	if err := decoder.Decode(&plans); err != nil {
		return nil, err
	}
	return plans, nil
}

func handleMCPIssueOpsLinkChild(args map[string]any) MCPToolOutcome {
	if err := VerifyIssueOpsChildIssueBeforeLink(argmap.String(args, "child_url")); err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps child link failed")
	}
	result, err := core.LinkIssueOpsChild(core.IssueOpsStateRoot(), argmap.String(args, "id"), argmap.String(args, "child_url"), argmap.String(args, "title"))
	return issueOpsMCPOutcome(result, err, "IssueOps child link failed")
}

func handleMCPIssueOpsLinkRelated(args map[string]any) MCPToolOutcome {
	result, err := core.LinkIssueOpsRelated(core.IssueOpsStateRoot(), argmap.String(args, "id"), argmap.String(args, "type"), argmap.String(args, "related_url"), argmap.String(args, "title"))
	return issueOpsMCPOutcome(result, err, "IssueOps related link failed")
}

func handleMCPIssueOpsChildStart(args map[string]any) MCPToolOutcome {
	result, err := core.StartIssueOpsChild(core.IssueOpsStateRoot(), core.IssueOpsChildStartRequest{
		ParentID:           argmap.String(args, "parent"),
		Branch:             argmap.String(args, "branch"),
		Title:              argmap.String(args, "title"),
		TaskScope:          argmap.String(args, "scope"),
		AcceptanceCriteria: argmap.StringSlice(args, "acceptance"),
		ChildIssueURL:      argmap.String(args, "child_issue_url"),
	})
	return issueOpsMCPOutcome(result, err, "IssueOps child start failed")
}

func handleMCPIssueOpsChildStatus(args map[string]any) MCPToolOutcome {
	result, err := core.IssueOpsChildStatus(core.IssueOpsStateRoot(), argmap.String(args, "parent"), argmap.Bool(args, "repair"))
	return issueOpsMCPOutcome(result, err, "IssueOps child status failed")
}

func handleMCPIssueOpsChildAccept(args map[string]any) MCPToolOutcome {
	result, err := core.AcceptIssueOpsChild(core.IssueOpsStateRoot(), argmap.String(args, "parent"), argmap.String(args, "child"), argmap.StringSlice(args, "evidence"))
	return issueOpsMCPOutcome(result, err, "IssueOps child accept failed")
}

func handleMCPIssueOpsChildReject(args map[string]any) MCPToolOutcome {
	result, err := core.RejectIssueOpsChild(core.IssueOpsStateRoot(), argmap.String(args, "parent"), argmap.String(args, "child"), argmap.String(args, "reason"), nil)
	return issueOpsMCPOutcome(result, err, "IssueOps child reject failed")
}

func handleMCPIssueOpsChildDrop(args map[string]any) MCPToolOutcome {
	result, err := core.DropIssueOpsChild(core.IssueOpsStateRoot(), argmap.String(args, "parent"), argmap.String(args, "child"), argmap.String(args, "reason"))
	return issueOpsMCPOutcome(result, err, "IssueOps child drop failed")
}

func handleMCPIssueOpsPrepareBranch(args map[string]any) MCPToolOutcome {
	result, err := core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), argmap.String(args, "id"), core.IssueOpsBranchPrepareRequest{
		Provider:        argmap.String(args, "provider"),
		IssueURL:        argmap.String(args, "issue_url"),
		Branch:          argmap.String(args, "branch"),
		BaseBranch:      argmap.String(args, "base_branch"),
		BaseSHA:         argmap.String(args, "base_sha"),
		RemoteBranchURL: argmap.String(args, "remote_branch_url"),
		LinkVerified:    argmap.Bool(args, "link_verified"),
	})
	return issueOpsMCPOutcome(result, err, "IssueOps branch prepare failed")
}

func handleMCPIssueOpsAddFeedback(args map[string]any) MCPToolOutcome {
	result, err := core.AddIssueOpsFeedback(core.IssueOpsStateRoot(), argmap.String(args, "id"), argmap.String(args, "source"), argmap.String(args, "body"), argmap.String(args, "classification"))
	return issueOpsMCPOutcome(result, err, "IssueOps feedback failed")
}

func handleMCPIssueOpsAddDecision(args map[string]any) MCPToolOutcome {
	result, err := core.AddIssueOpsDecision(core.IssueOpsStateRoot(), argmap.String(args, "id"), core.IssueOpsDecisionRecordRequest{
		Title:              argmap.String(args, "title"),
		Body:               argmap.String(args, "body"),
		Kind:               argmap.String(args, "kind"),
		Rationale:          argmap.String(args, "rationale"),
		Alternatives:       argmap.StringSlice(args, "alternatives"),
		AffectedIssueLinks: argmap.StringSlice(args, "affected_issue_links"),
		AffectedArtifacts:  argmap.StringSlice(args, "affected_artifacts"),
	})
	return issueOpsMCPOutcome(result, err, "IssueOps decision record failed")
}

func handleMCPIssueOpsRecordRouting(args map[string]any) MCPToolOutcome {
	result, err := core.RecordIssueOpsRouting(core.IssueOpsStateRoot(), argmap.String(args, "id"), argmap.String(args, "phase"), argmap.String(args, "skill"))
	return issueOpsMCPOutcome(result, err, "IssueOps routing record failed")
}

func handleMCPIssueOpsMarkIssueUpdated(args map[string]any) MCPToolOutcome {
	result, err := core.MarkIssueOpsContractFeedbackIssueUpdated(core.IssueOpsStateRoot(), argmap.String(args, "id"))
	return issueOpsMCPOutcome(result, err, "IssueOps issue update mark failed")
}

func handleMCPIssueOpsSetPhase(args map[string]any) MCPToolOutcome {
	phase := argmap.String(args, "phase")
	if strings.TrimSpace(phase) == "" {
		phase = argmap.String(args, "to")
	}
	if argmap.Bool(args, "force") && phase == "done" {
		result, err := core.ForceDoneIssueOps(core.IssueOpsStateRoot(), argmap.String(args, "id"))
		return issueOpsMCPOutcome(result, err, "IssueOps force-done failed")
	}
	result, err := core.AdvanceIssueOpsPhase(core.IssueOpsStateRoot(), argmap.String(args, "id"), phase)
	return issueOpsMCPOutcome(result, err, "IssueOps phase advance failed")
}

func handleMCPIssueOpsVerifyRemoteArtifact(args map[string]any) MCPToolOutcome {
	result, err := verifyIssueOpsRemoteArtifactFromMCP(args)
	return issueOpsMCPOutcome(result, err, "IssueOps remote artifact verification failed")
}

func handleMCPIssueOpsRemoteScore(args map[string]any) MCPToolOutcome {
	req, err := issueOpsRemoteScoringRequestFromMCP(args)
	if err == nil {
		var result core.IssueOpsRemoteScoringResult
		result, err = core.ScoreIssueOpsRemoteCandidates(req)
		return issueOpsMCPOutcome(result, err, "IssueOps remote score failed")
	}
	return issueOpsMCPOutcome(nil, err, "IssueOps remote score failed")
}

func handleMCPIssueOpsPRReadiness(args map[string]any) MCPToolOutcome {
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), argmap.String(args, "id"))
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps PR readiness failed")
	}
	if argmap.Bool(args, "strict") {
		return mcpToolPayload(core.IssueOpsStrictPRReadinessWithState(core.IssueOpsStateRoot(), record))
	}
	return mcpToolPayload(core.IssueOpsPRReadiness(record))
}

func handleMCPIssueOpsCleanupStatus(args map[string]any) MCPToolOutcome {
	result, err := core.IssueOpsCleanupStatusByID(core.IssueOpsStateRoot(), argmap.String(args, "id"), core.IssueOpsCleanupStatusRequest{
		Merged: IssueOpsCleanupMerged(argmap.String(args, "id"), argmap.Bool(args, "merged")),
	})
	return issueOpsMCPOutcome(result, err, "IssueOps cleanup status failed")
}

func handleMCPIssueOpsCleanupCloseChildren(args map[string]any) MCPToolOutcome {
	result, err := core.CloseIssueOpsChildren(core.IssueOpsStateRoot(), argmap.String(args, "id"), core.IssueOpsCloseChildrenRequest{
		Merged:  IssueOpsCleanupMerged(argmap.String(args, "id"), argmap.Bool(args, "merged")),
		Confirm: argmap.Bool(args, "confirm"),
	}, provider.Resolve)
	return issueOpsMCPOutcome(result, err, "IssueOps cleanup close-children failed")
}

func handleMCPIssueOpsForceRelease(args map[string]any) MCPToolOutcome {
	result, err := core.ForceReleaseIssueOps(core.IssueOpsStateRoot(), argmap.String(args, "id"), argmap.String(args, "reason"))
	return issueOpsMCPOutcome(result, err, "IssueOps force-release failed")
}

func handleMCPIssueOpsCleanupStale(args map[string]any) MCPToolOutcome {
	// prune_done mirrors the CLI --prune-done flag (default 720h): it lets the
	// MCP cleanup-stale tool prune done cycles past the retention window, which
	// only takes effect together with apply.
	pruneDoneAge, err := time.ParseDuration(argmap.StringDefault(args, "prune_done", "720h"))
	if err != nil {
		return issueOpsMCPOutcome(nil, fmt.Errorf("invalid prune_done duration: %w", err), "IssueOps stale cleanup failed")
	}
	if pruneDoneAge < 0 {
		return issueOpsMCPOutcome(nil, fmt.Errorf("prune_done must be non-negative, got %s", pruneDoneAge), "IssueOps stale cleanup failed")
	}
	result := core.ScanStaleIssueOpsCycles(core.IssueOpsStaleScanRequest{
		Repo:         argmap.String(args, "repo"),
		MaxAge:       time.Duration(argmap.Int(args, "max_age", 14)) * 24 * time.Hour,
		Apply:        argmap.Bool(args, "apply"),
		PruneDoneAge: pruneDoneAge,
	})
	if !result.OK {
		return issueOpsMCPOutcome(nil, fmt.Errorf("%s", strings.Join(result.Errors, "; ")), "IssueOps stale cleanup failed")
	}
	return mcpToolPayload(result)
}

func handleMCPIssueOpsResume(args map[string]any) MCPToolOutcome {
	result := core.IssueOpsResume(argmap.String(args, "repo"), argmap.String(args, "id"))
	if argmap.Bool(args, "bind") && result.OK && result.CycleID != "" {
		if result.ExecutionHandoff != nil {
			return issueOpsMCPOutcome(nil, fmt.Errorf("resume bind is read-only and refused for a supervised handoff; use issueops_handoff action=claim"), "IssueOps resume bind failed")
		}
		if err := core.BindIssueOpsSessionForCycle(result.Repo, result.CycleID); err != nil {
			return issueOpsMCPOutcome(nil, fmt.Errorf("resume bind: %w", err), "IssueOps resume bind failed")
		}
	}
	return mcpToolPayload(result)
}

func handleMCPIssueOpsHeartbeat(args map[string]any) MCPToolOutcome {
	record, err := core.RecordIssueOpsHeartbeatWithRequest(core.IssueOpsStateRoot(), core.IssueOpsHeartbeatRequest{
		ID: argmap.String(args, "id"), Attempt: argmap.Int(args, "attempt", 0), OwnershipEpoch: argmap.String(args, "ownership_epoch"),
		ContextSHA256: argmap.String(args, "context_sha256"), Host: argmap.String(args, "host"), SessionID: argmap.String(args, "session_id"), AgentID: argmap.String(args, "agent_id"),
	})
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps heartbeat failed")
	}
	return mcpToolPayload(record)
}

func handleMCPIssueOpsHandoff(args map[string]any) MCPToolOutcome {
	id := argmap.String(args, "id")
	switch argmap.String(args, "action") {
	case "start":
		result, err := core.StartIssueOpsHandoff(context.Background(), core.IssueOpsStateRoot(), core.IssueOpsHandoffStartRequest{ID: id, Confirm: argmap.Bool(args, "confirm")}, orca.New(), core.IssueOpsHandoffStartClock{})
		return issueOpsMCPOutcome(result, err, "IssueOps handoff start failed")
	case "claim":
		result, err := core.ClaimIssueOpsHandoff(core.IssueOpsStateRoot(), core.IssueOpsHandoffClaimRequest{
			ID: id, Attempt: argmap.Int(args, "attempt", 0), OwnershipEpoch: argmap.String(args, "ownership_epoch"), ContextSHA256: argmap.String(args, "context_sha256"),
			Host: argmap.String(args, "host"), SessionID: argmap.String(args, "session_id"), AgentID: argmap.String(args, "agent_id"), CWD: argmap.String(args, "cwd"), OrcaWorktreeID: argmap.String(args, "orca_worktree_id"),
		})
		return issueOpsMCPOutcome(result, err, "IssueOps handoff claim failed")
	case "finish":
		result, err := core.FinishIssueOpsHandoff(core.IssueOpsStateRoot(), core.IssueOpsHandoffFinishRequest{
			ID: id, Attempt: argmap.Int(args, "attempt", 0), OwnershipEpoch: argmap.String(args, "ownership_epoch"), ContextSHA256: argmap.String(args, "context_sha256"),
			Host: argmap.String(args, "host"), SessionID: argmap.String(args, "session_id"), AgentID: argmap.String(args, "agent_id"), Outcome: argmap.String(args, "outcome"),
			FinalHead: argmap.String(args, "final_head"), ChangedFiles: argmap.StringSlice(args, "changed_files"), TuringReportPath: argmap.String(args, "turing_report_path"),
			Verification: argmap.StringSlice(args, "verification"), CleanupReceipts: argmap.StringSlice(args, "cleanup_receipts"), EvidenceDigest: argmap.String(args, "evidence_digest"),
			TaskID: argmap.String(args, "task_id"), DispatchID: argmap.String(args, "dispatch_id"),
		})
		return issueOpsMCPOutcome(result, err, "IssueOps handoff finish failed")
	case "accept":
		result, err := core.AcceptIssueOpsHandoff(core.IssueOpsStateRoot(), core.IssueOpsHandoffAcceptRequest{
			ID: id, Attempt: argmap.Int(args, "attempt", 0), OwnershipEpoch: argmap.String(args, "ownership_epoch"), ContextSHA256: argmap.String(args, "context_sha256"), FinalHead: argmap.String(args, "final_head"),
		})
		return issueOpsMCPOutcome(result, err, "IssueOps handoff accept failed")
	case "recover":
		result, err := core.RecoverIssueOpsHandoff(context.Background(), core.IssueOpsStateRoot(), core.IssueOpsHandoffRecoverRequest{ID: id, Action: argmap.String(args, "recovery_action"), Confirm: argmap.Bool(args, "confirm")}, orca.New(), core.IssueOpsHandoffPrepareClock{})
		return issueOpsMCPOutcome(result, err, "IssueOps handoff recover failed")
	default:
		return issueOpsMCPOutcome(nil, fmt.Errorf("handoff action must be start, claim, finish, accept, or recover"), "IssueOps handoff failed")
	}
}
