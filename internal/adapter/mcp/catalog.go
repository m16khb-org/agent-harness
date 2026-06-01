package mcp

// Tool describes a stable MCP tool schema fragment owned by the MCP adapter.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// AdapterOwnedTools returns tool descriptors for surfaces that are intentionally
// adapter-level contracts rather than core business logic. cmd/harness maps
// these descriptors to concrete handlers.
func AdapterOwnedTools() []Tool {
	return []Tool{
		{
			Name:        "contract_schema",
			Description: "Return the current CLI/MCP compatibility contract, required response fields, and stable schema hash. This tool is read-only and is used before changing response DTOs.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "contract_check",
			Description: "Check whether the current CLI/MCP compatibility contract is internally consistent. This tool is read-only and returns a pass/fail result plus warnings.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "worker_enqueue",
			Description: "Create a no-shell local worker job record. This writes only to the harness worker state directory and never executes shell commands.",
			InputSchema: map[string]any{"type": "object", "required": []string{"kind"}, "properties": map[string]any{"kind": map[string]any{"type": "string"}, "payload": map[string]any{"type": "string"}}},
		},
		{
			Name:        "worker_run_read_only",
			Description: "Run an argv-only read-only command as a local worker evidence job. This writes only the worker job record and command evidence, forces write/network/shell disabled, applies workspace policy, timeout, env allowlist, redaction, and bounded output.",
			InputSchema: map[string]any{"type": "object", "required": []string{"kind", "workspace_root", "cwd", "argv"}, "properties": map[string]any{
				"kind":           map[string]any{"type": "string"},
				"payload":        map[string]any{"type": "string"},
				"workspace_root": map[string]any{"type": "string"},
				"cwd":            map[string]any{"type": "string"},
				"argv":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"timeout":        map[string]any{"type": "string"},
				"env_allowlist":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			}},
		},
		{
			Name:        "worker_status",
			Description: "Read a no-shell local worker job by id. This is read-only and returns the persisted job lifecycle record.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{"id": map[string]any{"type": "string"}}},
		},
		{
			Name:        "worker_list",
			Description: "List no-shell local worker jobs from the harness worker state directory. This is read-only.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "worker_cancel",
			Description: "Mark a queued no-shell local worker job as cancelled. This writes only the job lifecycle record and never kills processes.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{"id": map[string]any{"type": "string"}}},
		},
	}
}
