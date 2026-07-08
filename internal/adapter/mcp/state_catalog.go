package mcp

func StateTools() []Tool {
	return []Tool{
		{
			Name:        "state_write",
			Description: "Write a small agent state checkpoint to HARNESS_STATE_DIR or ~/.local/state/agent-harness. Keys allow [A-Za-z0-9._-] and cannot contain path traversal.",
			InputSchema: map[string]any{"type": "object", "required": []string{"key", "content"}, "properties": map[string]any{
				"key":     map[string]any{"type": "string", "description": "State key, max 128 chars, no path separators."},
				"content": map[string]any{"type": "string", "description": "State checkpoint content."},
			}},
		},
		{
			Name:        "state_read",
			Description: "Read an agent state checkpoint by key.",
			InputSchema: map[string]any{"type": "object", "required": []string{"key"}, "properties": map[string]any{
				"key": map[string]any{"type": "string", "description": "State key."},
			}},
		},
		{
			Name:        "state_list",
			Description: "List agent state checkpoint keys and metadata.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "state_prune",
			Description: "Prune old agent state checkpoints. Defaults to dry-run; pass confirm=true to delete records older than max_age.",
			InputSchema: map[string]any{"type": "object", "required": []string{"max_age"}, "properties": map[string]any{
				"max_age": map[string]any{"type": "string", "description": "Duration such as 720h or 168h. Must be positive."},
				"confirm": map[string]any{"type": "boolean", "description": "When true, delete matching records; false or omitted performs a dry-run."},
			}},
		},
		{
			Name:        "state_doctor",
			Description: "Inspect agent state checkpoint files for invalid JSON, key mismatches, byte-count drift, invalid timestamps, and unexpected files without modifying state.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "state_maintain",
			Description: "Run sqlite store maintenance on every existing harness store root: truncate the WAL via wal_checkpoint(TRUNCATE) and re-assert private 0600 file modes. Roots without a store are skipped, never created.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "state_migrate",
			Description: "Migrate valid legacy state checkpoints to the current schema. Defaults to dry-run; pass confirm=true to rewrite eligible records.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"confirm": map[string]any{"type": "boolean", "description": "When true, rewrite legacy records; false or omitted performs a dry-run."},
			}},
		},
	}
}
