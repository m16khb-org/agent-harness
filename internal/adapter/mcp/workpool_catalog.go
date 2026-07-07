package mcp

func WorkPoolTools() []Tool {
	stringProp := func(description string) map[string]any {
		return map[string]any{"type": "string", "description": description}
	}
	stringArrayProp := func(description string) map[string]any {
		return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": description}
	}
	return []Tool{
		{
			Name:        "workpool_create",
			Description: "Create a bounded lease-based work pool, optionally linked to a parent IssueOps cycle.",
			InputSchema: map[string]any{"type": "object", "required": []string{"repo", "name"}, "properties": map[string]any{
				"repo":         stringProp("Repository path."),
				"name":         stringProp("Pool name."),
				"parent_cycle": stringProp("Optional parent IssueOps cycle id."),
				"size":         map[string]any{"type": "integer", "description": "Maximum concurrent leases."},
				"lease_ttl":    stringProp("Lease duration such as 15m."),
				"max_attempts": map[string]any{"type": "integer", "description": "Maximum reject/requeue attempts."},
			}},
		},
		{
			Name:        "workpool_add_task",
			Description: "Add a task to an active work pool.",
			InputSchema: map[string]any{"type": "object", "required": []string{"pool", "title"}, "properties": map[string]any{
				"pool":         stringProp("Work pool id."),
				"title":        stringProp("Task title."),
				"instructions": stringProp("Task instructions."),
				"scope":        stringArrayProp("Task scope entries."),
				"acceptance":   stringArrayProp("Acceptance criteria."),
			}},
		},
		{Name: "workpool_claim", Description: "Claim the next pending task from a work pool.", InputSchema: map[string]any{"type": "object", "required": []string{"pool", "worker"}, "properties": map[string]any{"pool": stringProp("Work pool id."), "worker": stringProp("Worker id.")}}},
		{Name: "workpool_heartbeat", Description: "Extend a held work-pool task lease.", InputSchema: map[string]any{"type": "object", "required": []string{"pool", "task", "worker"}, "properties": map[string]any{"pool": stringProp("Work pool id."), "task": stringProp("Task id."), "worker": stringProp("Worker id.")}}},
		{
			Name:        "workpool_submit",
			Description: "Submit evidence for a leased work-pool task.",
			InputSchema: map[string]any{"type": "object", "required": []string{"pool", "task", "worker", "evidence"}, "properties": map[string]any{
				"pool":     stringProp("Work pool id."),
				"task":     stringProp("Task id."),
				"worker":   stringProp("Worker id."),
				"evidence": stringArrayProp("Verification evidence."),
				"branch":   stringProp("Submission branch."),
				"worktree": stringProp("Submission worktree path."),
			}},
		},
		{Name: "workpool_accept", Description: "Accept a submitted work-pool task.", InputSchema: map[string]any{"type": "object", "required": []string{"pool", "task", "evidence"}, "properties": map[string]any{"pool": stringProp("Work pool id."), "task": stringProp("Task id."), "evidence": stringArrayProp("Review evidence.")}}},
		{Name: "workpool_reject", Description: "Reject or requeue a work-pool task.", InputSchema: map[string]any{"type": "object", "required": []string{"pool", "task", "reason"}, "properties": map[string]any{"pool": stringProp("Work pool id."), "task": stringProp("Task id."), "reason": stringProp("Rejection reason."), "requeue": map[string]any{"type": "boolean", "description": "Return task to pending if attempts remain."}}}},
		{Name: "workpool_status", Description: "Read a work pool, reaping expired leases first and returning task counts.", InputSchema: map[string]any{"type": "object", "required": []string{"pool"}, "properties": map[string]any{"pool": stringProp("Work pool id.")}}},
		{Name: "workpool_reap", Description: "Reap expired leases in a work pool.", InputSchema: map[string]any{"type": "object", "required": []string{"pool"}, "properties": map[string]any{"pool": stringProp("Work pool id.")}}},
		{Name: "workpool_close", Description: "Close a terminal work pool, or force-close with a reason.", InputSchema: map[string]any{"type": "object", "required": []string{"pool"}, "properties": map[string]any{"pool": stringProp("Work pool id."), "force": map[string]any{"type": "boolean", "description": "Close a non-terminal pool."}, "reason": stringProp("Force-close reason.")}}},
	}
}
