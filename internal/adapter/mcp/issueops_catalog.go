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

func IssueOpsLifecycleTools() []Tool {
	return []Tool{
		{
			Name:        "issueops_prepare_branch",
			Description: "Record provider-linked issue branch preparation and expose the required MCP-first, provider API fallback, fail-closed order. This must be used before creating a local worktree for an IssueOps issue branch.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "provider", "issue_url", "branch", "base_branch"}, "properties": map[string]any{
				"id":                map[string]any{"type": "string", "description": "IssueOps id."},
				"provider":          map[string]any{"type": "string", "description": "Remote provider: github or gitlab.", "enum": []string{"github", "gitlab"}},
				"issue_url":         map[string]any{"type": "string", "description": "GitHub/GitLab issue URL."},
				"branch":            map[string]any{"type": "string", "description": "Provider-linked branch name, such as 2386-title or 2387-title. For GitLab, the full branch name must start with the issue number followed by a hyphen so the issue Development section links it."},
				"base_branch":       map[string]any{"type": "string", "description": "Remote base branch or ref."},
				"base_sha":          map[string]any{"type": "string", "description": "Optional resolved base commit SHA."},
				"remote_branch_url": map[string]any{"type": "string", "description": "Optional provider branch URL after creation."},
				"link_verified":     map[string]any{"type": "boolean", "description": "Whether the provider issue UI/API was verified to show the branch link."},
			}},
		},
		{
			Name:        "issueops_add_feedback",
			Description: "Record user or review feedback for an IssueOps loop and move it to the feedback phase.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "source", "body"}, "properties": map[string]any{
				"id":             map[string]any{"type": "string", "description": "IssueOps id."},
				"source":         map[string]any{"type": "string", "description": "Feedback source, such as user, review, CI, or QA."},
				"body":           map[string]any{"type": "string", "description": "Feedback body."},
				"classification": map[string]any{"type": "string", "description": "Optional feedback classification, such as contract_change, defect, question, or noise."},
			}},
		},
		{
			Name:        "issueops_mark_issue_updated",
			Description: "Record that unresolved contract_change feedback has been reflected in the remote issue body, unblocking PR readiness.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "IssueOps id."},
			}},
		},
		{
			Name:        "issueops_set_phase",
			Description: "Advance an IssueOps loop to a named lifecycle phase (problem, grill, plan, implement, ai-slop-clean, feedback, pr, done). The ai-slop-clean phase requires linked issue, provider-linked branch, plan, linked worktree, and implementation changes; the pr phase requires strict PR readiness; the done phase requires prior pr phase plus verified remote PR/MR artifact state.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id":    map[string]any{"type": "string", "description": "IssueOps id."},
				"phase": map[string]any{"type": "string", "description": "Target phase: problem, grill, plan, implement, ai-slop-clean, feedback, pr, or done.", "enum": []string{"problem", "grill", "plan", "implement", "ai-slop-clean", "feedback", "pr", "done"}},
				"to":    map[string]any{"type": "string", "description": "Compatibility alias for phase, matching the CLI --to flag.", "enum": []string{"problem", "grill", "plan", "implement", "ai-slop-clean", "feedback", "pr", "done"}},
			}},
		},
		{
			Name:        "issueops_verify_remote_artifact",
			Description: "Record that the created PR/MR was verified remotely with URL, labels, and assignees before the IssueOps loop may enter done.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "provider", "kind", "url", "labels", "assignees"}, "properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "IssueOps id."},
				"provider":  map[string]any{"type": "string", "description": "Remote provider.", "enum": []string{"github", "gitlab"}},
				"kind":      map[string]any{"type": "string", "description": "Remote artifact kind.", "enum": []string{"pr", "mr"}},
				"url":       map[string]any{"type": "string", "description": "Verified PR/MR URL."},
				"labels":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Verified remote labels."},
				"assignees": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Verified remote assignees."},
			}},
		},
		{
			Name:        "issueops_pr_readiness",
			Description: "Report whether an IssueOps loop has the issue, plan, exact worktree, clean git state, and upstream sync needed before drafting a PR or MR.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id":     map[string]any{"type": "string", "description": "IssueOps id."},
				"strict": map[string]any{"type": "boolean", "description": "When true, require linked worktree path, clean git status, matching branch, existing plan path, upstream, and 0/0 divergence."},
			}},
		},
		{
			Name:        "issueops_cleanup_status",
			Description: "Report whether a merged IssueOps PR/MR is ready for local cleanup before deleting worktrees or branches.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id":     map[string]any{"type": "string", "description": "IssueOps id."},
				"merged": map[string]any{"type": "boolean", "description": "Whether the remote PR/MR merge was verified before cleanup."},
			}},
		},
		{
			Name:        "issueops_remote_score",
			Description: "Deterministically score related issue and label candidates for a new IssueOps issue and select only those at/above the threshold. Read-only background_join gate; join before any remote artifact write.",
			InputSchema: map[string]any{"type": "object", "required": []string{"issue"}, "properties": map[string]any{
				"provider":  map[string]any{"type": "string", "description": "Remote provider: github or gitlab. Defaults to github.", "enum": []string{"github", "gitlab"}},
				"threshold": map[string]any{"type": "number", "description": "Selection cutoff from 0 to 1. Defaults to 0.70."},
				"issue": map[string]any{"type": "object", "description": "The new issue artifact being scored.", "required": []string{"title", "body"}, "properties": map[string]any{
					"provider": map[string]any{"type": "string"},
					"title":    map[string]any{"type": "string"},
					"body":     map[string]any{"type": "string"},
				}},
				"issue_candidates": map[string]any{"type": "array", "description": "Existing related issue candidates to score.", "items": map[string]any{"type": "object", "properties": map[string]any{
					"id":     map[string]any{"type": "string"},
					"title":  map[string]any{"type": "string"},
					"body":   map[string]any{"type": "string"},
					"url":    map[string]any{"type": "string"},
					"state":  map[string]any{"type": "string"},
					"labels": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"score":  map[string]any{"type": "number"},
				}}},
				"label_candidates": map[string]any{"type": "array", "description": "Label candidates to score.", "items": map[string]any{"type": "object", "properties": map[string]any{
					"name":        map[string]any{"type": "string"},
					"description": map[string]any{"type": "string"},
					"aliases":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"score":       map[string]any{"type": "number"},
				}}},
			}},
		},
	}
}
