package mcp

import "testing"

func TestIssueOpsBasicToolsExposeStableDescriptors(t *testing.T) {
	tools := IssueOpsBasicTools()
	wantNames := []string{
		"issueops_start",
		"issueops_status",
		"issueops_link_issue",
		"issueops_link_plan",
		"issueops_link_worktree",
		"issueops_prepare_worktree_tools",
		"issueops_link_child",
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
	if !schemaRequires(byName["issueops_link_worktree"].InputSchema, "worktree_path") {
		t.Fatalf("issueops_link_worktree must require worktree_path: %#v", byName["issueops_link_worktree"].InputSchema)
	}
	if !schemaRequires(byName["issueops_link_child"].InputSchema, "child_url") {
		t.Fatalf("issueops_link_child must require child_url: %#v", byName["issueops_link_child"].InputSchema)
	}
}

func TestIssueOpsLifecycleToolsExposeStableDescriptors(t *testing.T) {
	tools := IssueOpsLifecycleTools()
	wantNames := []string{
		"issueops_prepare_branch",
		"issueops_add_feedback",
		"issueops_mark_issue_updated",
		"issueops_set_phase",
		"issueops_verify_remote_artifact",
		"issueops_pr_readiness",
		"issueops_cleanup_status",
		"issueops_remote_score",
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

	prepareBranch := byName["issueops_prepare_branch"]
	for _, required := range []string{"id", "provider", "issue_url", "branch", "base_branch"} {
		if !schemaRequires(prepareBranch.InputSchema, required) {
			t.Fatalf("issueops_prepare_branch must require %q: %#v", required, prepareBranch.InputSchema)
		}
	}
	if !schemaHasProperty(prepareBranch.InputSchema, "link_verified") {
		t.Fatalf("issueops_prepare_branch schema missing link_verified: %#v", prepareBranch.InputSchema)
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
}
