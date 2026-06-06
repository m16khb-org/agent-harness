package mcp

func IssueOpsBasicTools() []Tool {
	return []Tool{
		{
			Name:        "issueops_start",
			Description: "Start an IssueOps loop and persist its issue-driven workflow state under harness state.",
			InputSchema: map[string]any{"type": "object", "required": []string{"repo"}, "properties": map[string]any{
				"repo":   map[string]any{"type": "string", "description": "Repository path this IssueOps loop belongs to."},
				"branch": map[string]any{"type": "string", "description": "Optional working branch name."},
			}},
		},
		{
			Name:        "issueops_status",
			Description: "Read a persisted IssueOps loop by id.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "IssueOps id."},
			}},
		},
		{
			Name:        "issueops_link_issue",
			Description: "Attach a GitHub/GitLab issue URL to an IssueOps loop and move it to the plan phase.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "issue_url"}, "properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "IssueOps id."},
				"issue_url": map[string]any{"type": "string", "description": "GitHub/GitLab issue URL."},
			}},
		},
		{
			Name:        "issueops_link_plan",
			Description: "Attach the issue-driven plan path to an IssueOps loop and move it to the implementation phase. Requires linked issue and verified provider branch evidence.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "plan_path"}, "properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "IssueOps id."},
				"plan_path": map[string]any{"type": "string", "description": "Plan file path."},
			}},
		},
		{
			Name:        "issueops_link_worktree",
			Description: "Attach the exact existing issue-driven worktree path that mutating tool guards must target. Requires linked issue and verified provider branch evidence.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "worktree_path"}, "properties": map[string]any{
				"id":            map[string]any{"type": "string", "description": "IssueOps id."},
				"worktree_path": map[string]any{"type": "string", "description": "Expected isolated worktree path."},
			}},
		},
		{
			Name:        "issueops_prepare_worktree_tools",
			Description: "Prepare the linked IssueOps worktree before tests by checking dependencies and initializing CodeGraph against the exact worktree path.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "IssueOps id."},
			}},
		},
		{
			Name:        "issueops_link_child",
			Description: "Record an existing provider-native child work item for an IssueOps loop, such as a GitHub sub-issue or GitLab child item.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "child_url"}, "properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "IssueOps id."},
				"child_url": map[string]any{"type": "string", "description": "GitHub sub-issue or GitLab child item URL."},
				"title":     map[string]any{"type": "string", "description": "Optional child issue title."},
			}},
		},
	}
}
