package mcp

func LocalAssistantTools() []Tool {
	return []Tool{
		{
			Name:        "commit_suggest",
			Description: "Generate a Conventional + Lore Hybrid commit message suggestion based on git diff using Gemini 3.5 Flash.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"repo":        map[string]any{"type": "string", "description": "Target repository path. Defaults to current directory."},
					"staged":      map[string]any{"type": "boolean", "description": "When true, suggest commit based on staged changes (git diff --cached); otherwise unstaged. Defaults to false."},
					"agy_command": map[string]any{"type": "string", "description": "Antigravity CLI executable path. Defaults to 'agy'."},
					"agy_model":   map[string]any{"type": "string", "description": "required agy settings.json model label; defaults to current settings model."},
				},
			},
		},
		{
			Name:        "lint_diagnose",
			Description: "Run a command, capture failure outputs, and provide a diagnosis using Gemini 3.5 Flash.",
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"command_argv"},
				"properties": map[string]any{
					"repo":         map[string]any{"type": "string", "description": "Target repository path. Defaults to current directory."},
					"command_argv": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "The command argv array to run and diagnose."},
					"agy_command":  map[string]any{"type": "string", "description": "Antigravity CLI executable path. Defaults to 'agy'."},
					"agy_model":    map[string]any{"type": "string", "description": "required agy settings.json model label; defaults to current settings model."},
				},
			},
		},
	}
}
