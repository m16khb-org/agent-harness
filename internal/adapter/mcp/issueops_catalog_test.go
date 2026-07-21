package mcp

import (
	"strings"
	"testing"
)

func TestIssueOpsBasicToolsExposeStableDescriptors(t *testing.T) {
	tools := IssueOpsBasicTools()
	wantNames := []string{
		"issueops_start",
		"issueops_status",
		"issueops_record_intent",
		"issueops_plan_prep_record",
		"issueops_link_issue",
		"issueops_prepare_branch",
		"issueops_link_worktree",
		"issueops_review_design",
		"issueops_link_plan",
		"issueops_prepare_worktree_tools",
		"issueops_reconcile_workspace",
		"issueops_record_execution_decision",
		"issueops_record_compatibility_review",
		"issueops_record_devils_advocate_review",
		"issueops_record_domain_review",
		"issueops_record_ai_slop_clean_evidence",
		"issueops_resolve_feedback",
		"issueops_regress_for_replan",
		"issueops_link_child",
		"issueops_link_related",
	}
	if len(tools) != len(wantNames) {
		t.Fatalf("expected %d issueops basic tools, got %d", len(wantNames), len(tools))
	}

	byName := toolsByName(tools)
	for _, name := range wantNames {
		tool, exists := byName[name]
		if !exists {
			t.Fatalf("missing issueops basic tool %q", name)
		}
		if tool.Description == "" || tool.InputSchema == nil {
			t.Fatalf("incomplete issueops basic tool descriptor: %+v", tool)
		}
	}

	if !schemaRequires(byName["issueops_start"].InputSchema, "repo") {
		t.Fatalf("issueops_start must require repo: %#v", byName["issueops_start"].InputSchema)
	}
	if !schemaRequires(byName["issueops_link_issue"].InputSchema, "issue_url") {
		t.Fatalf("issueops_link_issue must require issue_url: %#v", byName["issueops_link_issue"].InputSchema)
	}
	prepareBranch := byName["issueops_prepare_branch"]
	for _, required := range []string{"id", "provider", "issue_url", "branch", "base_branch"} {
		if !schemaRequires(prepareBranch.InputSchema, required) {
			t.Fatalf("issueops_prepare_branch must require %q: %#v", required, prepareBranch.InputSchema)
		}
	}
	if !schemaHasProperty(prepareBranch.InputSchema, "link_verified") {
		t.Fatalf("issueops_prepare_branch schema missing link_verified: %#v", prepareBranch.InputSchema)
	}
	if !schemaRequires(byName["issueops_record_intent"].InputSchema, "success_criteria") {
		t.Fatalf("issueops_record_intent must require success_criteria: %#v", byName["issueops_record_intent"].InputSchema)
	}
	if !schemaRequires(byName["issueops_link_worktree"].InputSchema, "worktree_path") {
		t.Fatalf("issueops_link_worktree must require worktree_path: %#v", byName["issueops_link_worktree"].InputSchema)
	}
	for _, field := range []string{"id", "workspace_epoch", "host", "session_id", "source_cwd"} {
		if !schemaRequires(byName["issueops_reconcile_workspace"].InputSchema, field) {
			t.Fatalf("issueops_reconcile_workspace must require %s", field)
		}
	}
	if !schemaRequires(byName["issueops_review_design"].InputSchema, "verification") {
		t.Fatalf("issueops_review_design must require verification: %#v", byName["issueops_review_design"].InputSchema)
	}
	reviewDesign := byName["issueops_review_design"]
	if !strings.Contains(reviewDesign.Description, "approved=true") {
		t.Fatalf("issueops_review_design description must explain approved=true gates: %s", reviewDesign.Description)
	}
	verificationDescription := schemaPropertyDescription(reviewDesign.InputSchema, "verification")
	if !strings.Contains(verificationDescription, "design review checked alternatives and risks") {
		t.Fatalf("issueops_review_design verification field must include accepted evidence example: %s", verificationDescription)
	}
	if !schemaRequires(byName["issueops_link_child"].InputSchema, "child_url") {
		t.Fatalf("issueops_link_child must require child_url: %#v", byName["issueops_link_child"].InputSchema)
	}
	executionDecision := byName["issueops_record_execution_decision"]
	for _, required := range []string{"id", "auto_proceed", "hook_blocked", "human_gates", "subagent_use"} {
		if !schemaRequires(executionDecision.InputSchema, required) {
			t.Fatalf("issueops_record_execution_decision must require %q: %#v", required, executionDecision.InputSchema)
		}
	}
	if !schemaHasProperty(executionDecision.InputSchema, "subagent_plans") {
		t.Fatalf("issueops_record_execution_decision schema missing subagent_plans: %#v", executionDecision.InputSchema)
	}
	plansDescription := schemaPropertyDescription(executionDecision.InputSchema, "subagent_plans")
	if !strings.Contains(plansDescription, "tradeoffs") || !strings.Contains(plansDescription, "net-positive") {
		t.Fatalf("execution decision subagent_plans must describe tradeoff handling: %s", plansDescription)
	}
	compatibilityReview := byName["issueops_record_compatibility_review"]
	for _, required := range []string{"id", "backward_compatibility", "side_effects", "rollback_plan", "verification"} {
		if !schemaRequires(compatibilityReview.InputSchema, required) {
			t.Fatalf("issueops_record_compatibility_review must require %q: %#v", required, compatibilityReview.InputSchema)
		}
	}
	if !schemaHasProperty(compatibilityReview.InputSchema, "blockers") {
		t.Fatalf("issueops_record_compatibility_review schema missing blockers: %#v", compatibilityReview.InputSchema)
	}
	if !schemaRequires(byName["issueops_link_related"].InputSchema, "type") || !schemaRequires(byName["issueops_link_related"].InputSchema, "related_url") {
		t.Fatalf("issueops_link_related must require type and related_url: %#v", byName["issueops_link_related"].InputSchema)
	}
}

func TestIssueOpsLifecycleToolsExposeStableDescriptors(t *testing.T) {
	tools := IssueOpsLifecycleTools()
	wantNames := []string{
		"issueops_add_decision",
		"issueops_record_routing",
		"issueops_add_feedback",
		"issueops_mark_issue_updated",
		"issueops_set_phase",
		"issueops_verify_remote_artifact",
		"issueops_remote_render_template",
		"issueops_remote_create_issue",
		"issueops_remote_reflect_devils_advocate",
		"issueops_remote_create_child",
		"issueops_remote_create_pr",
		"issueops_remote_reconcile_create",
		"issueops_remote_sync_graph",
		"issueops_force_release",
		"issueops_pr_readiness",
		"issueops_cleanup_status",
		"issueops_cleanup_close_children",
		"issueops_cleanup_stale",
		"issueops_remote_score",
		"issueops_child_start",
		"issueops_child_status",
		"issueops_child_accept",
		"issueops_child_reject",
		"issueops_child_drop",
		"issueops_resume",
		"issueops_heartbeat",
		"issueops_handoff",
	}
	if len(tools) != len(wantNames) {
		t.Fatalf("expected %d issueops lifecycle tools, got %d", len(wantNames), len(tools))
	}

	byName := toolsByName(tools)
	for _, name := range wantNames {
		tool, exists := byName[name]
		if !exists {
			t.Fatalf("missing issueops lifecycle tool %q", name)
		}
		if tool.Description == "" || tool.InputSchema == nil {
			t.Fatalf("incomplete issueops lifecycle tool descriptor: %+v", tool)
		}
	}

	addFeedback := byName["issueops_add_feedback"]
	for _, required := range []string{"id", "source", "body"} {
		if !schemaRequires(addFeedback.InputSchema, required) {
			t.Fatalf("issueops_add_feedback must require %q: %#v", required, addFeedback.InputSchema)
		}
	}
	if !schemaHasProperty(addFeedback.InputSchema, "classification") {
		t.Fatalf("issueops_add_feedback schema missing classification: %#v", addFeedback.InputSchema)
	}

	verifyRemote := byName["issueops_verify_remote_artifact"]
	for _, required := range []string{"id", "provider", "kind", "url", "labels", "assignees"} {
		if !schemaRequires(verifyRemote.InputSchema, required) {
			t.Fatalf("issueops_verify_remote_artifact must require %q: %#v", required, verifyRemote.InputSchema)
		}
	}
	if !schemaHasProperty(verifyRemote.InputSchema, "target_branch") {
		t.Fatalf("issueops_verify_remote_artifact schema missing target_branch: %#v", verifyRemote.InputSchema)
	}

	if !schemaRequires(byName["issueops_pr_readiness"].InputSchema, "id") {
		t.Fatalf("issueops_pr_readiness must require id: %#v", byName["issueops_pr_readiness"].InputSchema)
	}
	if !schemaHasProperty(byName["issueops_pr_readiness"].InputSchema, "strict") {
		t.Fatalf("issueops_pr_readiness schema missing strict: %#v", byName["issueops_pr_readiness"].InputSchema)
	}
	if !schemaRequires(byName["issueops_remote_score"].InputSchema, "issue") {
		t.Fatalf("issueops_remote_score must require issue: %#v", byName["issueops_remote_score"].InputSchema)
	}
	if !schemaHasProperty(byName["issueops_remote_score"].InputSchema, "threshold") {
		t.Fatalf("issueops_remote_score schema missing threshold: %#v", byName["issueops_remote_score"].InputSchema)
	}
	handoff := byName["issueops_handoff"]
	if !schemaHasProperty(handoff.InputSchema, "expected_context_sha256") {
		t.Fatalf("issueops_handoff schema missing expected_context_sha256: %#v", handoff.InputSchema)
	}
	renderTemplate := byName["issueops_remote_render_template"]
	for _, required := range []string{"kind", "template", "title"} {
		if !schemaRequires(renderTemplate.InputSchema, required) {
			t.Fatalf("issueops_remote_render_template must require %q: %#v", required, renderTemplate.InputSchema)
		}
	}
	for _, property := range []string{"provider", "fields", "score_summary"} {
		if !schemaHasProperty(renderTemplate.InputSchema, property) {
			t.Fatalf("issueops_remote_render_template schema missing %q: %#v", property, renderTemplate.InputSchema)
		}
	}
	for _, toolName := range []string{"issueops_remote_create_issue", "issueops_remote_create_child", "issueops_remote_create_pr"} {
		for _, property := range []string{"provider", "template", "fields", "score_summary"} {
			if !schemaHasProperty(byName[toolName].InputSchema, property) {
				t.Fatalf("%s schema missing %q: %#v", toolName, property, byName[toolName].InputSchema)
			}
		}
	}
	createChild := byName["issueops_remote_create_child"]
	for _, required := range []string{"id", "title", "labels", "assignees"} {
		if !schemaRequires(createChild.InputSchema, required) {
			t.Fatalf("issueops_remote_create_child must require %q: %#v", required, createChild.InputSchema)
		}
	}
	if !schemaHasProperty(createChild.InputSchema, "confirm") {
		t.Fatalf("issueops_remote_create_child schema missing confirm: %#v", createChild.InputSchema)
	}
	createPR := byName["issueops_remote_create_pr"]
	for _, required := range []string{"id", "provider", "title", "body", "head", "base", "labels", "assignees", "confirm"} {
		if !schemaRequires(createPR.InputSchema, required) {
			t.Fatalf("issueops_remote_create_pr must require %q: %#v", required, createPR.InputSchema)
		}
	}
	childStart := byName["issueops_child_start"]
	for _, required := range []string{"parent", "branch", "title", "scope", "acceptance"} {
		if !schemaRequires(childStart.InputSchema, required) {
			t.Fatalf("issueops_child_start must require %q: %#v", required, childStart.InputSchema)
		}
	}
	if !schemaHasProperty(childStart.InputSchema, "child_issue_url") {
		t.Fatalf("issueops_child_start schema missing child_issue_url: %#v", childStart.InputSchema)
	}
	childStartDescription := strings.ToLower(childStart.Description)
	if !strings.Contains(childStartDescription, "write") || !strings.Contains(childStartDescription, "result") {
		t.Fatalf("issueops_child_start description must state write/result contract: %s", childStart.Description)
	}
	childStatus := byName["issueops_child_status"]
	if !schemaRequires(childStatus.InputSchema, "parent") || !schemaHasProperty(childStatus.InputSchema, "repair") {
		t.Fatalf("issueops_child_status must require parent and expose repair: %#v", childStatus.InputSchema)
	}
	childAccept := byName["issueops_child_accept"]
	for _, required := range []string{"parent", "child", "evidence"} {
		if !schemaRequires(childAccept.InputSchema, required) {
			t.Fatalf("issueops_child_accept must require %q: %#v", required, childAccept.InputSchema)
		}
	}
	for _, toolName := range []string{"issueops_child_reject", "issueops_child_drop"} {
		for _, required := range []string{"parent", "child", "reason"} {
			if !schemaRequires(byName[toolName].InputSchema, required) {
				t.Fatalf("%s must require %q: %#v", toolName, required, byName[toolName].InputSchema)
			}
		}
	}
	for _, property := range []string{"repo", "id", "bind"} {
		if !schemaHasProperty(byName["issueops_resume"].InputSchema, property) {
			t.Fatalf("issueops_resume schema missing %q: %#v", property, byName["issueops_resume"].InputSchema)
		}
	}
	if !schemaRequires(byName["issueops_heartbeat"].InputSchema, "id") {
		t.Fatalf("issueops_heartbeat must require id: %#v", byName["issueops_heartbeat"].InputSchema)
	}
	if !schemaRequires(byName["issueops_handoff"].InputSchema, "action") || !schemaRequires(byName["issueops_handoff"].InputSchema, "id") {
		t.Fatalf("issueops_handoff must be one action-discriminated tool: %#v", byName["issueops_handoff"].InputSchema)
	}
	if !schemaHasProperty(byName["issueops_handoff"].InputSchema, "allow_codex_hook_trust_bypass") {
		t.Fatalf("issueops_handoff must expose the explicit Codex hook-trust bypass attestation: %#v", byName["issueops_handoff"].InputSchema)
	}
	properties := byName["issueops_handoff"].InputSchema["properties"].(map[string]any)
	recoveryActions := properties["recovery_action"].(map[string]any)["enum"].([]string)
	for _, action := range []string{"abandon", "finalize-cancel"} {
		if !strings.Contains("|"+strings.Join(recoveryActions, "|")+"|", "|"+action+"|") {
			t.Fatalf("issueops_handoff recovery_action must expose %s: %#v", action, recoveryActions)
		}
	}
}

func TestOwnershipTransferCLIAndMCPActionParity(t *testing.T) {
	handoff := toolsByName(IssueOpsLifecycleTools())["issueops_handoff"]
	properties := handoff.InputSchema["properties"].(map[string]any)
	actions := properties["action"].(map[string]any)["enum"].([]string)
	for _, action := range []string{"start", "claim", "acknowledge-context", "publish", "complete", "cleanup-preview", "cleanup-approve", "cleanup-record", "recover"} {
		if !strings.Contains("|"+strings.Join(actions, "|")+"|", "|"+action+"|") {
			t.Fatalf("MCP handoff action enum missing CLI action %q: %#v", action, actions)
		}
	}
	for _, field := range []string{"id", "host", "session_id", "agent_id", "source_cwd", "cwd", "attempt", "ownership_epoch", "context_sha256", "inventory_fingerprint", "disposition", "step", "confirm", "result_format"} {
		if _, ok := properties[field]; !ok {
			t.Fatalf("MCP handoff schema missing action-conditional field %q", field)
		}
	}
	description := strings.ToLower(handoff.Description)
	for _, phrase := range []string{"isolated owner", "fresh source session", "after merge", "no cleanup runs automatically"} {
		if !strings.Contains(description, phrase) {
			t.Fatalf("MCP handoff description must state %q: %s", phrase, handoff.Description)
		}
	}
}
