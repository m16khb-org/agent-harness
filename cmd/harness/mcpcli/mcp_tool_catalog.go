package mcpcli

import mcpadapter "agent-harness/internal/adapter/mcp"

func MCPTools() []map[string]any {
	tools := coreMCPToolMaps()
	tools = append(tools, mcpadapter.ToolMaps(mcpadapter.CommandPolicyTools())...)
	tools = append(tools, mcpadapter.ToolMaps(mcpadapter.StateTools())...)
	tools = append(tools, mcpadapter.ToolMaps(mcpadapter.IssueOpsBasicTools())...)
	tools = append(tools, mcpadapter.ToolMaps(mcpadapter.IssueOpsLifecycleTools())...)
	tools = append(tools, []map[string]any{
		{
			"name":        "daemon_status",
			"description": "Report whether the shared agent-harness daemon backing this MCP proxy is reachable, including socket and pid metadata.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "self_augment",
			"description": "Plan the self-augmentation loop: use GENIUS_THINK.md, repo signals, and research-backed strategies to choose concrete feature/performance/quality improvements. The native skill performs implementation; this tool exposes the scoring contract and candidate curriculum, and can persist the chosen plan to harness state for durable Reflexion-style memory.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"cycles":       map[string]any{"type": "integer", "description": "Number of autonomous improvement cycles to plan; defaults to 1."},
				"target_score": map[string]any{"type": "number", "description": "Exclusive per-goal score threshold; defaults to 95."},
				"save_state":   map[string]any{"type": "boolean", "description": "When true, save the selected self-augmentation plan to harness state."},
				"state_key":    map[string]any{"type": "string", "description": "State key for save_state; defaults to self-augment-latest."},
			}},
		},
		{
			"name":        "self_augment_lesson",
			"description": "Store a Reflexion-style self-augmentation lesson in harness state for durable cross-session learning.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"lesson", "next_action"}, "properties": map[string]any{
				"candidate_id": map[string]any{"type": "string", "description": "Candidate id this lesson belongs to; defaults to current selected open candidate."},
				"lesson":       map[string]any{"type": "string", "description": "Lesson learned from failure, QA issue, or design concern."},
				"next_action":  map[string]any{"type": "string", "description": "Specific next action that should use this lesson."},
				"source":       map[string]any{"type": "string", "description": "Source that produced the lesson; defaults to self-augment."},
				"severity":     map[string]any{"type": "string", "description": "info, warning, or error. Defaults to info."},
				"state_key":    map[string]any{"type": "string", "description": "State key; defaults to self-augment-lesson-<candidate>-<timestamp>."},
			}},
		},
		{
			"name":        "self_verify",
			"description": "Run the self-verification loop. Defaults to quick one-iteration verification; set full=true for the full gate with at least 10 seeded iterations. Termination requires every concrete goal score to be greater than target_score.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"full":         map[string]any{"type": "boolean", "description": "Run the full ten-plus-iteration verification gate. Defaults to false quick mode."},
				"iterations":   map[string]any{"type": "integer", "description": "Full-gate iteration count; requires full=true and must be at least 10."},
				"seed":         map[string]any{"type": "integer", "description": "Base seed for deterministic per-iteration fuzz fixtures."},
				"target_score": map[string]any{"type": "number", "description": "Exclusive per-goal score threshold; defaults to 95."},
				"save_state":   map[string]any{"type": "boolean", "description": "When true, save compact summary to harness state after the run."},
				"state_key":    map[string]any{"type": "string", "description": "State key for save_state; defaults to self-verify-latest."},
			}},
		},
		{
			"name":        "self_verify_candidates",
			"description": "Export the self-verification loop improvement candidate curriculum, including open/satisfied IDs and the next selected candidate.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"save_state": map[string]any{"type": "boolean", "description": "When true, save the candidate export snapshot to harness state."},
				"state_key":  map[string]any{"type": "string", "description": "State key for save_state; defaults to self-verify-candidates-latest."},
			}},
		},
		{
			"name":        "self_verify_history",
			"description": "List saved self-verification loop summary checkpoints from harness state, sorted by snapshot generation time for quick baseline/candidate discovery.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"prefix":          map[string]any{"type": "string", "description": "State key prefix to scan; defaults to self-verify. Use empty string to scan all keys."},
				"limit":           map[string]any{"type": "integer", "description": "Maximum entries to return; defaults to 20, 0 returns all."},
				"retention_limit": map[string]any{"type": "integer", "description": "Maximum matching summaries to retain by newest-first ordering. Omit or use 0 to disable retention planning."},
				"prune_retention": map[string]any{"type": "boolean", "description": "When true, plan retention pruning. This is a dry-run unless confirm is also true."},
				"confirm":         map[string]any{"type": "boolean", "description": "When true with prune_retention, delete retention candidates beyond retention_limit."},
			}},
		},
		{
			"name":        "self_verify_compare",
			"description": "Compare two saved self-verification loop summary checkpoints from harness state and report elapsed-time, failed-step, step-label, and goal-score regressions.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"baseline_key", "candidate_key"}, "properties": map[string]any{
				"baseline_key":               map[string]any{"type": "string", "description": "State key containing the baseline self-verification summary snapshot."},
				"candidate_key":              map[string]any{"type": "string", "description": "State key containing the candidate self-verification summary snapshot."},
				"max_elapsed_regression_pct": map[string]any{"type": "number", "description": "Allowed elapsed_ms increase percentage before regression; defaults to 20."},
			}},
		},
		{
			"name":        "self_verify_promote",
			"description": "Promote a saved self-verification loop summary checkpoint to a baseline state key. Defaults to dry-run; pass confirm=true to write the baseline.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"from_key", "baseline_key"}, "properties": map[string]any{
				"from_key":     map[string]any{"type": "string", "description": "State key containing the candidate self-verification summary snapshot to promote."},
				"baseline_key": map[string]any{"type": "string", "description": "State key to write as the promoted baseline."},
				"confirm":      map[string]any{"type": "boolean", "description": "When true, write baseline_key; false or omitted performs a dry-run."},
			}},
		},
	}...)
	tools = append(tools, mcpadapter.ToolMaps(mcpadapter.AdapterOwnedTools())...)
	tools = append(tools, mcpadapter.ToolMaps(mcpadapter.CommandPolicyAuditTools())...)
	tools = append(tools, mcpadapter.ToolMaps(mcpadapter.LocalAssistantTools())...)
	return tools
}
