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
			Name:        "issueops_record_intent",
			Description: "Record the raw user request, interpreted intent, success criteria, constraints, non-goals, and ambiguity ledger before an IssueOps loop may enter planning.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "raw_request", "interpreted_intent", "success_criteria"}, "properties": map[string]any{
				"id":                 map[string]any{"type": "string", "description": "IssueOps id."},
				"raw_request":        map[string]any{"type": "string", "description": "Original user request text."},
				"interpreted_intent": map[string]any{"type": "string", "description": "Main agent interpretation of the user's intent."},
				"success_criteria":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Concrete success criteria."},
				"constraints":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Known constraints."},
				"ambiguities":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Resolved, deferred, or blocking ambiguity ledger entries."},
				"non_goals":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Explicit non-goals."},
			}},
		},
		{
			Name:        "issueops_link_issue",
			Description: "Attach a GitHub/GitLab issue URL to an IssueOps loop. The loop moves to plan phase only when its intent contract is recorded.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "issue_url"}, "properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "IssueOps id."},
				"issue_url": map[string]any{"type": "string", "description": "GitHub/GitLab issue URL."},
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
			Name:        "issueops_review_design",
			Description: "Record the reviewed IssueOps design, refactor boundary, alternatives, risks, verification matrix, and approval before implementation.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "problem_summary", "proposed_design", "verification"}, "properties": map[string]any{
				"id":              map[string]any{"type": "string", "description": "IssueOps id."},
				"problem_summary": map[string]any{"type": "string", "description": "Reviewed problem summary."},
				"proposed_design": map[string]any{"type": "string", "description": "Reviewed design."},
				"refactor_plan":   map[string]any{"type": "string", "description": "Refactor plan or boundary decision."},
				"alternatives":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Alternatives considered."},
				"risks":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Design risks."},
				"verification":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Verification steps."},
				"open_questions":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Open design questions. Must be empty when approved is true."},
				"approved":        map[string]any{"type": "boolean", "description": "Whether the design is approved for implementation."},
			}},
		},
		{
			Name:        "issueops_link_plan",
			Description: "Attach the issue-driven plan path to an IssueOps loop and move it to the implementation phase. Requires linked issue, verified provider branch evidence, linked worktree, and approved design review.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "plan_path"}, "properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "IssueOps id."},
				"plan_path": map[string]any{"type": "string", "description": "Plan file path."},
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
