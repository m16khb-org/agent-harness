package mcp

func LocalAssistantTools() []Tool {
	return []Tool{
		{
			Name:        "commit_suggest",
			Description: "Generate a Conventional + Lore Hybrid commit message suggestion based on git diff using Z.AI Coding Plan.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"repo":   map[string]any{"type": "string", "description": "Target repository path. Defaults to current directory."},
					"staged": map[string]any{"type": "boolean", "description": "When true, suggest commit based on staged changes (git diff --cached); otherwise unstaged. Defaults to false."},
					"model":  map[string]any{"type": "string", "description": "Z.AI Coding Plan model. Defaults to glm-5-turbo."},
				},
			},
		},
		{
			Name:        "lint_diagnose",
			Description: "Run a command, capture failure outputs, and provide a diagnosis using Z.AI Coding Plan.",
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"command_argv"},
				"properties": map[string]any{
					"repo":         map[string]any{"type": "string", "description": "Target repository path. Defaults to current directory."},
					"command_argv": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "The command argv array to run and diagnose."},
					"model":        map[string]any{"type": "string", "description": "Z.AI Coding Plan model. Defaults to glm-5-turbo."},
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
