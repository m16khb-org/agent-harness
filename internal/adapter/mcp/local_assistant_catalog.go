package mcp

func LocalAssistantTools() []Tool {
	return []Tool{
		{
			Name:        "commit_suggest",
			Description: "Render a host-agent prompt for a Conventional + Lore Hybrid commit message based on git diff.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"repo":   map[string]any{"type": "string", "description": "Target repository path. Defaults to current directory."},
					"staged": map[string]any{"type": "boolean", "description": "When true, suggest commit based on staged changes (git diff --cached); otherwise unstaged. Defaults to false."},
				},
			},
		},
		{
			Name:        "lint_diagnose",
			Description: "Run a command, capture failure outputs, and render a host-agent diagnosis prompt.",
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"command_argv"},
				"properties": map[string]any{
					"repo":         map[string]any{"type": "string", "description": "Target repository path. Defaults to current directory."},
					"command_argv": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "The command argv array to run and diagnose."},
				},
			},
		},
		{
			Name:        "web_fetch_resilient",
			Description: "Run the clean-room resilient public web fetch engine with safe URL handling, route accounting, response validation, and citation-ready output.",
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"url"},
				"properties": map[string]any{
					"url":       map[string]any{"type": "string", "description": "Public http(s) URL to fetch."},
					"timeout":   map[string]any{"type": "string", "description": "Optional fetch timeout such as 30s."},
					"max_chars": map[string]any{"type": "integer", "description": "Optional maximum content characters returned."},
				},
			},
		},
	}
}
