package mcp

func CommandPolicyInputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"workspace_root", "cwd", "argv"},
		"properties": map[string]any{
			"workspace_root":  map[string]any{"type": "string", "description": "Workspace root boundary."},
			"cwd":             map[string]any{"type": "string", "description": "Command working directory; must be inside workspace_root."},
			"argv":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Command argv array. Shell strings are not accepted."},
			"timeout":         map[string]any{"type": "string", "description": "Duration such as 30s or 2m; max 15m."},
			"env_allowlist":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Allowed environment variable names."},
			"network_allowed": map[string]any{"type": "boolean", "description": "Whether network access is allowed."},
			"write_allowed":   map[string]any{"type": "boolean", "description": "Whether workspace writes are allowed."},
			"shell_allowed":   map[string]any{"type": "boolean", "description": "Whether shell interpreter argv[0] is allowed."},
			"shell_reason":    map[string]any{"type": "string", "description": "Required reason when shell_allowed is true."},
			"audit_log_id":    map[string]any{"type": "string", "description": "Optional caller-provided audit log correlation id."},
		},
	}
}

func CommandPolicyTools() []Tool {
	return []Tool{
		{
			Name:        "command_policy_check",
			Description: "Evaluate whether an argv-based command request is allowed by the harness command policy without executing it.",
			InputSchema: CommandPolicyInputSchema(),
		},
		{
			Name:        "command_fake_run",
			Description: "Run the command policy and return a fake runner result. This never executes the command; it only proves policy acceptance/denial and audit metadata.",
			InputSchema: CommandPolicyInputSchema(),
		},
	}
}

func CommandPolicyAuditTools() []Tool {
	return []Tool{
		{
			Name:        "command_policy_audit",
			Description: "Evaluate command policy and append a redacted JSONL audit record without executing the command. This writes only to the harness audit log.",
			InputSchema: CommandPolicyInputSchema(),
		},
	}
}
