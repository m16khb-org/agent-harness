package mcpcli

import (
	"encoding/json"
	"fmt"
	"strings"

	"agent-harness/cmd/harness/mcpcli/argmap"
	"agent-harness/internal/adapter/provider"
	"agent-harness/internal/core"
)

// issueOpsMCPHandlers is the dispatch registry for IssueOps MCP tools. Routing
// is a single map lookup so handleIssueOpsMCPToolCall stays low-branch and each
// tool is a small handler in mcp_tool_issueops_handlers.go.
var issueOpsMCPHandlers = map[string]func(map[string]any) MCPToolOutcome{
	"issueops_start":                  handleMCPIssueOpsStart,
	"issueops_status":                 handleMCPIssueOpsStatus,
	"issueops_record_intent":          handleMCPIssueOpsRecordIntent,
	"issueops_review_design":          handleMCPIssueOpsReviewDesign,
	"issueops_link_issue":             handleMCPIssueOpsLinkIssue,
	"issueops_link_plan":              handleMCPIssueOpsLinkPlan,
	"issueops_link_worktree":          handleMCPIssueOpsLinkWorktree,
	"issueops_prepare_worktree_tools": handleMCPIssueOpsPrepareWorktreeTools,
	"issueops_link_child":             handleMCPIssueOpsLinkChild,
	"issueops_link_related":           handleMCPIssueOpsLinkRelated,
	"issueops_prepare_branch":         handleMCPIssueOpsPrepareBranch,
	"issueops_add_feedback":           handleMCPIssueOpsAddFeedback,
	"issueops_add_decision":           handleMCPIssueOpsAddDecision,
	"issueops_record_routing":         handleMCPIssueOpsRecordRouting,
	"issueops_mark_issue_updated":     handleMCPIssueOpsMarkIssueUpdated,
	"issueops_set_phase":              handleMCPIssueOpsSetPhase,
	"issueops_verify_remote_artifact": handleMCPIssueOpsVerifyRemoteArtifact,
	"issueops_remote_score":           handleMCPIssueOpsRemoteScore,
	"issueops_pr_readiness":           handleMCPIssueOpsPRReadiness,
	"issueops_cleanup_status":         handleMCPIssueOpsCleanupStatus,
	"issueops_force_release":          handleMCPIssueOpsForceRelease,
	"issueops_cleanup_stale":          handleMCPIssueOpsCleanupStale,
	"issueops_remote_create_issue":    handleMCPRemoteCreateIssue,
	"issueops_remote_create_pr":       handleMCPRemoteCreatePR,
	"issueops_remote_sync_graph":      handleMCPRemoteSyncGraph,
	"issueops_resume":                 handleMCPIssueOpsResume,
}

func handleIssueOpsMCPToolCall(call MCPToolCall) MCPToolOutcome {
	handler, ok := issueOpsMCPHandlers[call.Name]
	if !ok {
		return MCPToolOutcome{}
	}
	return handler(call.Arguments)
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
	providerName := resolveRecordProviderForMCP(record)
	if providerName == "" {
		return issueOpsMCPOutcome(nil, fmt.Errorf("cannot determine provider"), "IssueOps remote create-issue failed")
	}
	prov, err := provider.Resolve(providerName)
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-issue failed")
	}
	result, err := core.CreateRemoteIssue(core.IssueProviderCreateIssueRequest{
		Repo:      record.Repo,
		Title:     argmap.String(args, "title"),
		Body:      argmap.String(args, "body"),
		Labels:    argmap.StringSlice(args, "labels"),
		Assignees: argmap.StringSlice(args, "assignees"),
		Confirm:   argmap.Bool(args, "confirm"),
	}, prov)
	return issueOpsMCPOutcome(result, err, "IssueOps remote create-issue failed")
}

func handleMCPRemoteCreatePR(args map[string]any) MCPToolOutcome {
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), argmap.String(args, "id"))
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-pr failed: cannot read cycle")
	}
	providerName := resolveRecordProviderForMCP(record)
	if providerName == "" {
		return issueOpsMCPOutcome(nil, fmt.Errorf("cannot determine provider"), "IssueOps remote create-pr failed")
	}
	prov, err := provider.Resolve(providerName)
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-pr failed")
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
	}, prov)
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
