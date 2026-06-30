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
	"issueops_start":                         handleMCPIssueOpsStart,
	"issueops_status":                        handleMCPIssueOpsStatus,
	"issueops_record_intent":                 handleMCPIssueOpsRecordIntent,
	"issueops_plan_prep_record":              handleMCPIssueOpsPlanPrepRecord,
	"issueops_review_design":                 handleMCPIssueOpsReviewDesign,
	"issueops_link_issue":                    handleMCPIssueOpsLinkIssue,
	"issueops_link_plan":                     handleMCPIssueOpsLinkPlan,
	"issueops_link_worktree":                 handleMCPIssueOpsLinkWorktree,
	"issueops_prepare_worktree_tools":        handleMCPIssueOpsPrepareWorktreeTools,
	"issueops_record_execution_decision":     handleMCPIssueOpsRecordExecutionDecision,
	"issueops_record_compatibility_review":   handleMCPIssueOpsRecordCompatibilityReview,
	"issueops_record_domain_review":          handleMCPIssueOpsRecordDomainReview,
	"issueops_record_ai_slop_clean_evidence": handleMCPIssueOpsRecordAISlopCleanEvidence,
	"issueops_resolve_feedback":              handleMCPIssueOpsResolveFeedback,
	"issueops_regress_for_replan":            handleMCPIssueOpsRegressForReplan,
	"issueops_link_child":                    handleMCPIssueOpsLinkChild,
	"issueops_link_related":                  handleMCPIssueOpsLinkRelated,
	"issueops_prepare_branch":                handleMCPIssueOpsPrepareBranch,
	"issueops_add_feedback":                  handleMCPIssueOpsAddFeedback,
	"issueops_add_decision":                  handleMCPIssueOpsAddDecision,
	"issueops_record_routing":                handleMCPIssueOpsRecordRouting,
	"issueops_mark_issue_updated":            handleMCPIssueOpsMarkIssueUpdated,
	"issueops_set_phase":                     handleMCPIssueOpsSetPhase,
	"issueops_verify_remote_artifact":        handleMCPIssueOpsVerifyRemoteArtifact,
	"issueops_remote_score":                  handleMCPIssueOpsRemoteScore,
	"issueops_pr_readiness":                  handleMCPIssueOpsPRReadiness,
	"issueops_cleanup_status":                handleMCPIssueOpsCleanupStatus,
	"issueops_cleanup_close_children":        handleMCPIssueOpsCleanupCloseChildren,
	"issueops_force_release":                 handleMCPIssueOpsForceRelease,
	"issueops_cleanup_stale":                 handleMCPIssueOpsCleanupStale,
	"issueops_remote_render_template":        handleMCPRemoteRenderTemplate,
	"issueops_remote_create_issue":           handleMCPRemoteCreateIssue,
	"issueops_remote_create_child":           handleMCPRemoteCreateChild,
	"issueops_remote_create_pr":              handleMCPRemoteCreatePR,
	"issueops_remote_sync_graph":             handleMCPRemoteSyncGraph,
	"issueops_resume":                        handleMCPIssueOpsResume,
}

func handleIssueOpsMCPToolCall(call MCPToolCall) MCPToolOutcome {
	handler, ok := issueOpsMCPHandlers[call.Name]
	if !ok {
		return MCPToolOutcome{}
	}
	return handler(call.Arguments)
}

// issueOpsMCPOutcome returns IssueOps tool-level FAILURES (cycle-not-found,
// validation, disk/lock, live-verify) as a normalized error tool result
// mirroring the CLI's {ok:false,error:...} body, instead of collapsing every
// error into a -32602 "Invalid params" JSON-RPC protocol error. message carries
// the operation context the CLI error string already implies.
func issueOpsMCPOutcome(payload any, err error, message string) MCPToolOutcome {
	if err != nil {
		return mcpToolErrorPayload(map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("%s: %s", message, err.Error()),
		})
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

func handleMCPRemoteRenderTemplate(args map[string]any) MCPToolOutcome {
	result := core.RenderIssueOpsTemplate(core.IssueOpsTemplateInput{
		Kind:         core.IssueOpsArtifactKind(argmap.String(args, "kind")),
		Template:     core.IssueOpsTemplateKind(argmap.String(args, "template")),
		Provider:     argmap.String(args, "provider"),
		Title:        argmap.String(args, "title"),
		Fields:       templateFieldsFromMCP(args),
		ScoreSummary: scoreSummaryFromMCP(args),
	})
	return mcpToolPayload(result)
}

func handleMCPRemoteCreateIssue(args map[string]any) MCPToolOutcome {
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), argmap.String(args, "id"))
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-issue failed: cannot read cycle")
	}
	providerName := firstNonEmptyMCP(argmap.String(args, "provider"), resolveRecordProviderForMCP(record))
	if providerName == "" {
		return issueOpsMCPOutcome(nil, fmt.Errorf("cannot determine provider"), "IssueOps remote create-issue failed")
	}
	prov, err := provider.Resolve(providerName)
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-issue failed")
	}
	body, err := resolveMCPTemplateBody(core.IssueOpsArtifactIssue, providerName, argmap.String(args, "title"), argmap.String(args, "body"), args)
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-issue failed")
	}
	if err := validateMCPConfirmRemoteCreate(argmap.Bool(args, "confirm"), argmap.StringSlice(args, "labels"), argmap.StringSlice(args, "assignees")); err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-issue failed")
	}
	result, err := core.CreateRemoteIssue(core.IssueProviderCreateIssueRequest{
		Repo:      record.Repo,
		Title:     argmap.String(args, "title"),
		Body:      body,
		Labels:    argmap.StringSlice(args, "labels"),
		Assignees: argmap.StringSlice(args, "assignees"),
		Confirm:   argmap.Bool(args, "confirm"),
	}, prov)
	return issueOpsMCPOutcome(result, err, "IssueOps remote create-issue failed")
}

func handleMCPRemoteCreateChild(args map[string]any) MCPToolOutcome {
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), argmap.String(args, "id"))
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-child failed: cannot read cycle")
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		return issueOpsMCPOutcome(nil, fmt.Errorf("cannot create child before linked parent issue"), "IssueOps remote create-child failed")
	}
	labels := argmap.StringSlice(args, "labels")
	assignees := argmap.StringSlice(args, "assignees")
	if err := validateMCPCreateChildInputs(argmap.String(args, "title"), labels, assignees); err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-child failed")
	}
	providerName := firstNonEmptyMCP(argmap.String(args, "provider"), resolveRecordProviderForMCP(record))
	if providerName == "" {
		return issueOpsMCPOutcome(nil, fmt.Errorf("cannot determine provider"), "IssueOps remote create-child failed")
	}
	prov, err := provider.Resolve(providerName)
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-child failed")
	}
	body, err := resolveMCPTemplateBody(core.IssueOpsArtifactChild, providerName, argmap.String(args, "title"), argmap.String(args, "body"), args)
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-child failed")
	}
	result, err := core.CreateRemoteChild(core.IssueProviderCreateChildRequest{
		Repo:           record.Repo,
		ParentIssueURL: record.IssueURL,
		Title:          argmap.String(args, "title"),
		Body:           body,
		Labels:         labels,
		Assignees:      assignees,
		Confirm:        argmap.Bool(args, "confirm"),
	}, prov)
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-child failed")
	}
	if argmap.Bool(args, "confirm") {
		if !result.HierarchyVerified || strings.TrimSpace(result.ChildURL) == "" {
			return issueOpsMCPOutcome(nil, fmt.Errorf("provider did not verify child hierarchy"), "IssueOps remote create-child failed")
		}
		if _, err := core.LinkIssueOpsChild(core.IssueOpsStateRoot(), record.ID, result.ChildURL, argmap.String(args, "title")); err != nil {
			return issueOpsMCPOutcome(nil, err, "IssueOps remote create-child failed")
		}
	}
	return issueOpsMCPOutcome(result, nil, "IssueOps remote create-child failed")
}

func handleMCPRemoteCreatePR(args map[string]any) MCPToolOutcome {
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), argmap.String(args, "id"))
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-pr failed: cannot read cycle")
	}
	providerName := firstNonEmptyMCP(argmap.String(args, "provider"), resolveRecordProviderForMCP(record))
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
	body, err := resolveMCPTemplateBody(core.IssueOpsArtifactPR, providerName, argmap.String(args, "title"), argmap.String(args, "body"), args)
	if err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-pr failed")
	}
	if err := validateMCPConfirmRemoteCreate(argmap.Bool(args, "confirm"), argmap.StringSlice(args, "labels"), argmap.StringSlice(args, "assignees")); err != nil {
		return issueOpsMCPOutcome(nil, err, "IssueOps remote create-pr failed")
	}
	result, err := core.CreateRemotePullRequest(core.IssueProviderCreatePullRequestRequest{
		Repo:       record.Repo,
		Title:      argmap.String(args, "title"),
		Body:       body,
		HeadBranch: head,
		BaseBranch: base,
		Labels:     argmap.StringSlice(args, "labels"),
		Assignees:  argmap.StringSlice(args, "assignees"),
		Confirm:    argmap.Bool(args, "confirm"),
	}, prov)
	return issueOpsMCPOutcome(result, err, "IssueOps remote create-pr failed")
}

func validateMCPCreateChildInputs(title string, labels, assignees []string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("child title is required")
	}
	if len(labels) == 0 {
		return fmt.Errorf("at least one child label is required")
	}
	if len(assignees) == 0 {
		return fmt.Errorf("at least one child assignee is required")
	}
	return nil
}

func resolveMCPTemplateBody(kind core.IssueOpsArtifactKind, providerName, title, body string, args map[string]any) (string, error) {
	template := argmap.String(args, "template")
	if strings.TrimSpace(template) == "" {
		return body, nil
	}
	result := core.RenderIssueOpsTemplate(core.IssueOpsTemplateInput{
		Kind:         kind,
		Template:     core.IssueOpsTemplateKind(template),
		Provider:     providerName,
		Title:        title,
		Body:         body,
		Fields:       templateFieldsFromMCP(args),
		ScoreSummary: scoreSummaryFromMCP(args),
	})
	if len(result.Validation.Critical) > 0 {
		return "", fmt.Errorf("template validation failed: %s", strings.Join(result.Validation.Critical, ","))
	}
	return result.Body, nil
}

func templateFieldsFromMCP(args map[string]any) map[string]string {
	fields := map[string]string{}
	raw, ok := args["fields"]
	if !ok || raw == nil {
		return fields
	}
	switch v := raw.(type) {
	case map[string]string:
		for key, value := range v {
			fields[key] = value
		}
	case map[string]any:
		for key, value := range v {
			if s, ok := value.(string); ok {
				fields[key] = s
			}
		}
	}
	return fields
}

func scoreSummaryFromMCP(args map[string]any) string {
	if summary := argmap.String(args, "score_summary"); summary != "" {
		return summary
	}
	if raw, ok := args["score_result"]; ok && raw != nil {
		if b, err := json.Marshal(raw); err == nil {
			return string(b)
		}
	}
	return ""
}

func validateMCPConfirmRemoteCreate(confirm bool, labels, assignees []string) error {
	if !confirm {
		return nil
	}
	if len(labels) == 0 {
		return fmt.Errorf("at least one label is required with confirm=true")
	}
	if len(assignees) == 0 {
		return fmt.Errorf("at least one assignee is required with confirm=true")
	}
	return nil
}

func firstNonEmptyMCP(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
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
