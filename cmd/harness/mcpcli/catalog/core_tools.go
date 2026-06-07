package catalog

func coreMCPToolMaps() []map[string]any {
	return []map[string]any{
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
}
