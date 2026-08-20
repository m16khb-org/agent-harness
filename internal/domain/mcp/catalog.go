package mcp

// Tool describes a stable MCP tool schema fragment owned by the MCP adapter.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// DispatchGroup names the handler group that owns a set of MCP tools.
// The MCP server uses this to route tool calls to the correct handler.
type DispatchGroup string

const (
	DispatchProject         DispatchGroup = "project"
	DispatchPolicyState     DispatchGroup = "policy_state"
	DispatchIssueOps        DispatchGroup = "issueops"
	DispatchLoop            DispatchGroup = "loop"
	DispatchAssistantWorker DispatchGroup = "assistant_worker"
	DispatchSelfLoop        DispatchGroup = "self_loop"
)

// catalogSection binds one catalog function to its dispatch handler group and
// records whether its tools are advertised in the tools/list response.
type catalogSection struct {
	group      DispatchGroup
	advertised bool
	tools      func() []Tool
}

// catalogSections is the single ordered source of truth for the MCP tool
// catalog. Both the advertised tools/list (AdvertisedTools) and the
// name->handler routing table (DispatchMap) derive from this slice, so adding a
// tool means editing exactly one catalog function referenced here. The
// advertised order matches the stable mcp_tools.golden.json snapshot.
func catalogSections() []catalogSection {
	return []catalogSection{
		{DispatchProject, true, coreProjectTools},
		{DispatchPolicyState, true, CommandPolicyTools},
		{DispatchPolicyState, true, StateTools},
		{DispatchIssueOps, true, IssueOpsBasicTools},
		{DispatchLoop, true, LoopTools},
		{DispatchAssistantWorker, true, func() []Tool { return []Tool{DaemonStatusTool()} }},
		{DispatchSelfLoop, true, selfLoopAdvertisedTools},
		{DispatchAssistantWorker, true, AdapterOwnedTools},
		{DispatchPolicyState, true, CommandPolicyAuditTools},
		{DispatchAssistantWorker, true, LocalAssistantTools},
		{DispatchSelfLoop, false, selfLoopAliasTools},
	}
}

// DaemonStatusTool returns the standalone daemon-status assistant-worker tool.
// It lives in no sub-catalog, so it is declared once here and flows into both
// the advertised list and DispatchMap via catalogSections.
func DaemonStatusTool() Tool {
	return Tool{
		Name:        "daemon_status",
		Description: "Report whether the shared agent-harness daemon backing this MCP proxy is reachable, including socket and pid metadata.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}
}

// AdvertisedTools returns every MCP tool advertised in tools/list, in the
// stable order pinned by mcp_tools.golden.json.
func AdvertisedTools() []Tool {
	var out []Tool
	for _, s := range catalogSections() {
		if s.advertised {
			out = append(out, s.tools()...)
		}
	}
	return out
}

// AllTools returns every MCP tool known to the catalog, advertised or not.
func AllTools() []Tool {
	var out []Tool
	for _, s := range catalogSections() {
		out = append(out, s.tools()...)
	}
	return out
}

// DispatchMap returns a map from every MCP tool name to its handler group.
// It derives from catalogSections so routing can never drift from the catalog:
// adding a tool to a section makes it both routable and (if advertised) listed.
func DispatchMap() map[string]DispatchGroup {
	out := make(map[string]DispatchGroup)
	for _, s := range catalogSections() {
		for _, t := range s.tools() {
			out[t.Name] = s.group
		}
	}
	return out
}

// coreProjectTools returns the harness project-management tools. This is their
// single authoritative definition: the CLI catalog package derives its
// tools/list payload from AdvertisedTools rather than re-declaring them.
func coreProjectTools() []Tool {
	return []Tool{
		{
			Name:        "harness_inspect",
			Description: "Inspect the agent-harness installation, shared skills, docs, and native Codex/Claude integration status.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"repo": map[string]any{"type": "string", "description": "Optional target repository path."}}},
		},
		{
			Name:        "atomic_commit_preflight",
			Description: "Run a read-only git preflight for the atomic-commit-push workflow: branch/upstream, staged/unstaged/untracked files, secret-like paths, and commit style hints.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": "Git repository path. Defaults to the agent project directory."}}},
		},
		{
			Name:        "commit_policy",
			Description: "Return the Conventional Commit + Lore body policy used by this harness.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "skill_manifest",
			Description: "List shared skills exposed by the harness and their metadata.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "docs_index",
			Description: "Return a lightweight index of AGENTS.md, CLAUDE.md, GENIUS_THINK.md, .agent-harness markdown files, and self-* skill docs: relative path, title, headings, and byte size.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "project_docs_route",
			Description: "Given a task, return the project AGENTS.md and .agent-harness documents an agent should read before working.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"repo": map[string]any{"type": "string", "description": "Target repository path. Defaults to current directory."},
				"task": map[string]any{"type": "string", "description": "Task description such as commit, test, architecture, dependency, deploy, or general."},
			}},
		},
		{
			Name:        "project_docs_bootstrap_plan",
			Description: "Dry-run the project docs bootstrap that creates or updates AGENTS.md and .agent-harness/*.md from repository evidence. This tool never writes files.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"repo": map[string]any{"type": "string", "description": "Target repository path. Defaults to current directory."}}},
		},
		{
			Name:        "project_docs_read",
			Description: "Read one allowed .agent-harness project document and return its content plus SHA-256. Use before project_docs_revise so autonomous doc refreshes preserve user consensus and avoid stale overwrites.",
			InputSchema: map[string]any{"type": "object", "required": []string{"rel_path"}, "properties": map[string]any{
				"repo":     map[string]any{"type": "string", "description": "Target repository path. Defaults to current directory."},
				"rel_path": map[string]any{"type": "string", "description": "Allowed project doc path, for example .agent-harness/TESTING.md or TESTING.md."},
			}},
		},
		{
			Name:        "project_docs_revise",
			Description: "Revise (fully replace) one allowed .agent-harness project document after reading repo evidence and preserving user consensus. Dry-run unless confirm=true. Existing files require expected_sha256 from project_docs_read. Do not use for solved false cases or ADR entries; use project_docs_append there.",
			InputSchema: map[string]any{"type": "object", "required": []string{"rel_path", "content", "summary"}, "properties": map[string]any{
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
			Name:        "project_docs_append",
			Description: "Append one dated record file under .agent-harness/cautions/ (kind=caution) or .agent-harness/adr/ (kind=adr) without modifying the family root index. Use only when there is a concrete issue resolved or decision made; this tool writes files.",
			InputSchema: map[string]any{"type": "object", "required": []string{"kind", "title", "summary"}, "properties": map[string]any{
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
			Name:        "api_doc_review",
			Description: "Render the API documentation host-agent review prompt/schema, or record a supplied JSON review result for staged or explicit controller/DTO/handler/OpenAPI files. By default it scopes to git staged API candidate files and does not fail unrelated legacy Swagger/OpenAPI debt.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"repo":        map[string]any{"type": "string", "description": "Target git repository path. Defaults to current directory."},
				"files":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Explicit API candidate files. Omit to use staged controller/DTO/handler/OpenAPI files."},
				"all":         map[string]any{"type": "boolean", "description": "When true, review all tracked API candidate files. Default false keeps scope to staged changes."},
				"diff_file":   map[string]any{"type": "string", "description": "Optional file containing a diff to review instead of git diff --cached."},
				"prompt_file": map[string]any{"type": "string", "description": "Optional project-specific Swagger/OpenAPI rules."},
				"result_file": map[string]any{"type": "string", "description": "Optional host-agent JSON review result to record as evidence. Omit to render prompt/schema for the host agent."},
			}},
		},
		{
			Name:        "api_doc_static_check",
			Description: "Run deterministic API documentation checks for syntax-level Swagger/OpenAPI omissions such as missing operation descriptions, params, body/query/header docs, 400/401 responses, and NestJS DTO decorators. Use before api_doc_review.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"repo":  map[string]any{"type": "string", "description": "Target git repository path. Defaults to current directory."},
				"files": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Explicit API candidate files. Omit to use staged controller/DTO/handler/OpenAPI files."},
				"all":   map[string]any{"type": "boolean", "description": "When true, check all tracked API candidate files. Default false keeps scope to staged changes."},
			}},
		},
	}
}

// selfLoopAdvertisedTools returns the self-improvement loop tools advertised in
// tools/list: the self-augment plan/lesson tools and the self-verify family.
func selfLoopAdvertisedTools() []Tool {
	return []Tool{
		{
			Name:        "self_augment",
			Description: "Plan the self-augmentation loop: use GENIUS_THINK.md, repo signals, and research-backed strategies to choose concrete feature/performance/quality improvements. The native skill performs implementation; this tool exposes the scoring contract and candidate curriculum, and can persist the chosen plan to harness state for durable Reflexion-style memory.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"cycles":       map[string]any{"type": "integer", "description": "Number of autonomous improvement cycles to plan; defaults to 1."},
				"target_score": map[string]any{"type": "number", "description": "Exclusive per-goal score threshold; defaults to 95."},
				"save_state":   map[string]any{"type": "boolean", "description": "When true, save the selected self-augmentation plan to harness state."},
				"state_key":    map[string]any{"type": "string", "description": "State key for save_state; defaults to self-augment-latest."},
			}},
		},
		{
			Name:        "self_augment_lesson",
			Description: "Store a Reflexion-style self-augmentation lesson in harness state for durable cross-session learning.",
			InputSchema: map[string]any{"type": "object", "required": []string{"lesson", "next_action"}, "properties": map[string]any{
				"candidate_id": map[string]any{"type": "string", "description": "Candidate id this lesson belongs to; defaults to current selected open candidate."},
				"lesson":       map[string]any{"type": "string", "description": "Lesson learned from failure, QA issue, or design concern."},
				"next_action":  map[string]any{"type": "string", "description": "Specific next action that should use this lesson."},
				"source":       map[string]any{"type": "string", "description": "Source that produced the lesson; defaults to self-augment."},
				"severity":     map[string]any{"type": "string", "description": "info, warning, or error. Defaults to info."},
				"state_key":    map[string]any{"type": "string", "description": "State key; defaults to self-augment-lesson-<candidate>-<timestamp>."},
			}},
		},
		{
			Name:        "self_verify",
			Description: "Run one complete deterministic self-verification evidence pass. Termination requires every concrete goal score to be greater than target_score.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"seed":         map[string]any{"type": "integer", "description": "Seed for deterministic fuzz fixtures."},
				"target_score": map[string]any{"type": "number", "description": "Exclusive per-goal score threshold; defaults to 95."},
				"save_state":   map[string]any{"type": "boolean", "description": "When true, save compact summary to harness state after the run."},
				"state_key":    map[string]any{"type": "string", "description": "State key for save_state; defaults to self-verify-latest."},
			}},
		},
		{
			Name:        "self_verify_candidates",
			Description: "Export the self-verification loop improvement candidate curriculum, including open/satisfied IDs and the next selected candidate.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"save_state": map[string]any{"type": "boolean", "description": "When true, save the candidate export snapshot to harness state."},
				"state_key":  map[string]any{"type": "string", "description": "State key for save_state; defaults to self-verify-candidates-latest."},
			}},
		},
		{
			Name:        "self_verify_history",
			Description: "List saved self-verification loop summary checkpoints from harness state, sorted by snapshot generation time for quick baseline/candidate discovery.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"prefix":          map[string]any{"type": "string", "description": "State key prefix to scan; defaults to self-verify. Use empty string to scan all keys."},
				"limit":           map[string]any{"type": "integer", "description": "Maximum entries to return; defaults to 20, 0 returns all."},
				"retention_limit": map[string]any{"type": "integer", "description": "Maximum matching summaries to retain by newest-first ordering. Omit or use 0 to disable retention planning."},
				"prune_retention": map[string]any{"type": "boolean", "description": "When true, plan retention pruning. This is a dry-run unless confirm is also true."},
				"confirm":         map[string]any{"type": "boolean", "description": "When true with prune_retention, delete retention candidates beyond retention_limit."},
			}},
		},
		{
			Name:        "self_verify_compare",
			Description: "Compare two saved self-verification loop summary checkpoints from harness state and report elapsed-time, failed-step, step-label, and goal-score regressions.",
			InputSchema: map[string]any{"type": "object", "required": []string{"baseline_key", "candidate_key"}, "properties": map[string]any{
				"baseline_key":               map[string]any{"type": "string", "description": "State key containing the baseline self-verification summary snapshot."},
				"candidate_key":              map[string]any{"type": "string", "description": "State key containing the candidate self-verification summary snapshot."},
				"max_elapsed_regression_pct": map[string]any{"type": "number", "description": "Allowed elapsed_ms increase percentage before regression; defaults to 20."},
			}},
		},
		{
			Name:        "self_verify_promote",
			Description: "Promote a saved self-verification loop summary checkpoint to a baseline state key. Defaults to dry-run; pass confirm=true to write the baseline.",
			InputSchema: map[string]any{"type": "object", "required": []string{"from_key", "baseline_key"}, "properties": map[string]any{
				"from_key":            map[string]any{"type": "string", "description": "State key containing the candidate self-verification summary snapshot to promote."},
				"baseline_key":        map[string]any{"type": "string", "description": "State key to write as the promoted baseline."},
				"confirm":             map[string]any{"type": "boolean", "description": "When true, write baseline_key; false or omitted performs a dry-run."},
				"allow_failed_source": map[string]any{"type": "boolean", "description": "Promote even when the source snapshot did not pass the gate (baseline-poisoning override; off by default)."},
			}},
		},
	}
}

// selfLoopAliasTools returns the self-augment-prefixed aliases of the
// self-verify history/compare/promote tools. They route to the self-loop
// handler but are not advertised in tools/list, keeping the catalog free of
// duplicate-looking names.
func selfLoopAliasTools() []Tool {
	return []Tool{
		{
			Name:        "self_augment_history",
			Description: "Alias for self_verify_history that scans self-augment prefixed state checkpoints.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"prefix":          map[string]any{"type": "string", "description": "State key prefix to scan; defaults to self-augment."},
				"limit":           map[string]any{"type": "integer", "description": "Maximum entries to return; defaults to 20, 0 returns all."},
				"retention_limit": map[string]any{"type": "integer", "description": "Maximum matching summaries to retain by newest-first ordering."},
				"prune_retention": map[string]any{"type": "boolean", "description": "When true, plan retention pruning."},
				"confirm":         map[string]any{"type": "boolean", "description": "When true with prune_retention, delete retention candidates beyond retention_limit."},
			}},
		},
		{
			Name:        "self_augment_compare",
			Description: "Alias for self_verify_compare that compares self-augment prefixed state checkpoints.",
			InputSchema: map[string]any{"type": "object", "required": []string{"baseline_key", "candidate_key"}, "properties": map[string]any{
				"baseline_key":               map[string]any{"type": "string", "description": "State key containing the baseline self-augment summary snapshot."},
				"candidate_key":              map[string]any{"type": "string", "description": "State key containing the candidate self-augment summary snapshot."},
				"max_elapsed_regression_pct": map[string]any{"type": "number", "description": "Allowed elapsed_ms increase percentage before regression; defaults to 20."},
			}},
		},
		{
			Name:        "self_augment_promote",
			Description: "Alias for self_verify_promote that promotes self-augment prefixed state checkpoints.",
			InputSchema: map[string]any{"type": "object", "required": []string{"from_key", "baseline_key"}, "properties": map[string]any{
				"from_key":            map[string]any{"type": "string", "description": "State key containing the candidate self-augment summary snapshot to promote."},
				"baseline_key":        map[string]any{"type": "string", "description": "State key to write as the promoted baseline."},
				"confirm":             map[string]any{"type": "boolean", "description": "When true, write baseline_key; false or omitted performs a dry-run."},
				"allow_failed_source": map[string]any{"type": "boolean", "description": "Promote even when the source snapshot did not pass the gate (baseline-poisoning override; off by default)."},
			}},
		},
	}
}

func ToolMaps(tools []Tool) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		out = append(out, map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		})
	}
	return out
}

func ResourceMaps(resources []Resource) []map[string]any {
	out := make([]map[string]any, 0, len(resources))
	for _, resource := range resources {
		out = append(out, map[string]any{
			"uri":         resource.URI,
			"name":        resource.Name,
			"description": resource.Description,
			"mimeType":    resource.MimeType,
		})
	}
	return out
}
