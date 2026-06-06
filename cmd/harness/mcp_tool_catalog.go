package main

import mcpadapter "agent-harness/internal/adapter/mcp"

func mcpTools() []map[string]any {
	tools := []map[string]any{
		{
			"name":        "harness_inspect",
			"description": "Inspect the agent-harness installation, shared skills, docs, and native Codex/Claude integration status.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"repo": map[string]any{"type": "string", "description": "Optional target repository path."}}},
		},
		{
			"name":        "atomic_commit_preflight",
			"description": "Run a read-only git preflight for the atomic-commit-push workflow: branch/upstream, staged/unstaged/untracked files, secret-like paths, and commit style hints.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": "Git repository path. Defaults to the agent project directory."}}},
		},
		{
			"name":        "commit_policy",
			"description": "Return the Conventional Commit + Lore body policy used by this harness.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "skill_manifest",
			"description": "List shared skills exposed by the harness and their metadata.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "docs_index",
			"description": "Return a lightweight index of AGENTS.md, CLAUDE.md, GENIUS_THINK.md, .agent-harness markdown files, and self-* skill docs: relative path, title, headings, and byte size.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "project_docs_route",
			"description": "Given a task, return the project AGENTS.md and .agent-harness documents an agent should read before working.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"repo": map[string]any{"type": "string", "description": "Target repository path. Defaults to current directory."},
				"task": map[string]any{"type": "string", "description": "Task description such as commit, test, architecture, dependency, deploy, or general."},
			}},
		},
		{
			"name":        "project_docs_bootstrap_plan",
			"description": "Dry-run the project docs bootstrap that creates or updates AGENTS.md and .agent-harness/*.md from repository evidence. This tool never writes files.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"repo": map[string]any{"type": "string", "description": "Target repository path. Defaults to current directory."},
			}},
		},
		{
			"name":        "project_docs_read",
			"description": "Read one allowed .agent-harness project document and return its content plus SHA-256. Use before project_docs_update so autonomous doc refreshes preserve user consensus and avoid stale overwrites.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"rel_path"}, "properties": map[string]any{
				"repo":     map[string]any{"type": "string", "description": "Target repository path. Defaults to current directory."},
				"rel_path": map[string]any{"type": "string", "description": "Allowed project doc path, for example .agent-harness/TESTING.md or TESTING.md."},
			}},
		},
		{
			"name":        "project_docs_update",
			"description": "Update one allowed .agent-harness project document after reading repo evidence and preserving user consensus. Dry-run unless confirm=true. Existing files require expected_sha256 from project_docs_read. Do not use for solved false cases or ADR entries; use project_docs_record there.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"rel_path", "content", "summary"}, "properties": map[string]any{
				"repo":            map[string]any{"type": "string", "description": "Target repository path. Defaults to current directory."},
				"rel_path":        map[string]any{"type": "string", "description": "Allowed project doc path under .agent-harness, for example .agent-harness/OPERATIONS.md."},
				"content":         map[string]any{"type": "string", "description": "Full replacement content for the one document. Preserve stronger existing local guidance and user decisions."},
				"expected_sha256": map[string]any{"type": "string", "description": "SHA-256 returned by project_docs_read. Required when the file exists."},
				"summary":         map[string]any{"type": "string", "description": "Short reason for the update and how it maintains current consensus."},
				"evidence":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Files, commands, tests, or user instructions that justify the update."},
				"confirm":         map[string]any{"type": "boolean", "description": "Set true to write. Omit or false for dry-run preview."},
			}},
		},
		{
			"name":        "project_docs_record",
			"description": "Append a durable project note to .agent-harness/CAUTIONS.md after a solved problem/false case, or to .agent-harness/ADR.md after a decision with rationale. Use only when there is a concrete issue resolved or decision made; this tool writes files.",
			"inputSchema": map[string]any{"type": "object", "required": []string{"kind", "title", "summary"}, "properties": map[string]any{
				"repo":         map[string]any{"type": "string", "description": "Target repository path. Defaults to current directory."},
				"kind":         map[string]any{"type": "string", "description": "caution for solved problems/false cases; adr for decisions."},
				"title":        map[string]any{"type": "string", "description": "Short record title."},
				"summary":      map[string]any{"type": "string", "description": "One-sentence summary of the issue or decision."},
				"context":      map[string]any{"type": "string", "description": "Relevant context or trigger."},
				"resolution":   map[string]any{"type": "string", "description": "How the problem was solved; use for caution records."},
				"decision":     map[string]any{"type": "string", "description": "Chosen decision; use for ADR records."},
				"evidence":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Commands, files, tests, or source evidence."},
				"alternatives": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Rejected alternatives or tradeoffs."},
				"consequences": map[string]any{"type": "string", "description": "Expected follow-up consequences for ADR records."},
				"source":       map[string]any{"type": "string", "description": "Calling workflow or agent source."},
			}},
		},
		{
			"name":        "api_doc_review",
			"description": "Run the API documentation review gate on staged or explicit controller/DTO/handler/OpenAPI files. By default it reviews only git staged API candidate files and does not fail unrelated legacy Swagger/OpenAPI debt. Use for endpoint, controller, DTO, schema, or OpenAPI changes.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"repo":        map[string]any{"type": "string", "description": "Target git repository path. Defaults to current directory."},
				"files":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Explicit API candidate files. Omit to use staged controller/DTO/handler/OpenAPI files."},
				"all":         map[string]any{"type": "boolean", "description": "When true, review all tracked API candidate files. Default false keeps scope to staged changes."},
				"diff_file":   map[string]any{"type": "string", "description": "Optional file containing a diff to review instead of git diff --cached."},
				"prompt_file": map[string]any{"type": "string", "description": "Optional project-specific Swagger/OpenAPI rules."},
				"model":       map[string]any{"type": "string", "description": "Codex model. Defaults to gpt-5.5."},
				"reasoning":   map[string]any{"type": "string", "description": "Codex reasoning effort. Defaults to medium."},
				"timeout":     map[string]any{"type": "string", "description": "Timeout such as 3m. Defaults to 3m."},
			}},
		},
		{
			"name":        "api_doc_static_check",
			"description": "Run deterministic API documentation checks for syntax-level Swagger/OpenAPI omissions such as missing operation descriptions, params, body/query/header docs, 400/401 responses, and NestJS DTO decorators. Use before api_doc_review.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"repo":  map[string]any{"type": "string", "description": "Target git repository path. Defaults to current directory."},
				"files": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Explicit API candidate files. Omit to use staged controller/DTO/handler/OpenAPI files."},
				"all":   map[string]any{"type": "boolean", "description": "When true, check all tracked API candidate files. Default false keeps scope to staged changes."},
			}},
		},
	}
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
