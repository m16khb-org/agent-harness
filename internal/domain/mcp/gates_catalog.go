package mcp

// GatesTools는 태스크 게이트 ledger 도구군이다. CLI gates 명령과 같은
// contract DTO를 공유한다.
func GatesTools() []Tool {
	stringProp := func(description string) map[string]any {
		return map[string]any{"type": "string", "description": description}
	}
	boolProp := func(description string) map[string]any {
		return map[string]any{"type": "boolean", "description": description}
	}
	return []Tool{
		{
			Name:        "gates_check",
			Description: "Evaluate unlazy-compatible task gate ledgers (GATES.md, gates/*.md). Runs CHECK commands for unmet gates through the command policy engine (workspace boundary, env allowlist, timeout, audit), matches EXPECT, flips checkboxes, and records evidence. Complete means zero unmet gates; ABANDON gates count as resolved.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"workspace_root":  stringProp("Workspace root boundary for CHECK execution. Defaults to cwd."),
				"cwd":             stringProp("Directory holding GATES.md/gates/*.md and the CHECK working directory. Defaults to the agent project directory."),
				"files":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Explicit gate ledger files. Omit to discover GATES.md plus gates/*.md under cwd."},
				"timeout_seconds": map[string]any{"type": "integer", "description": "Per-CHECK timeout in seconds. Defaults to 120."},
				"env_allowlist":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Environment variable allowlist for CHECK commands. Defaults to HOME,PATH."},
				"write_allowed":   boolProp("Allow workspace-write commands for CHECK execution. Default true."),
				"network_allowed": boolProp("Allow network access for CHECK execution. Default false."),
			}},
		},
		{
			Name:        "gates_status",
			Description: "Report gate ledger state without executing anything or modifying files. Flags unchecked gates and checked-but-EVIDENCE-pending gates (a checkbox is a claim; evidence is the proof).",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"workspace_root": stringProp("Workspace root. Defaults to cwd."),
				"cwd":            stringProp("Directory holding GATES.md/gates/*.md. Defaults to the agent project directory."),
				"files":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Explicit gate ledger files. Omit to discover GATES.md plus gates/*.md under cwd."},
			}},
		},
		{
			Name:        "gates_report",
			Description: "Render the final-report ledger paste: per-file N-of-N gate counts, per-gate state, evidence, and abandon reasons. Re-measures counts at report time instead of trusting memory.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"workspace_root": stringProp("Workspace root. Defaults to cwd."),
				"cwd":            stringProp("Directory holding GATES.md/gates/*.md. Defaults to the agent project directory."),
				"files":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Explicit gate ledger files. Omit to discover GATES.md plus gates/*.md under cwd."},
			}},
		},
		{
			Name:        "gates_abandon",
			Description: "Record an honest ABANDON line for one gate in a ledger file. Abandoned gates count as resolved but stay visible in reports; use instead of silently dropping a gate.",
			InputSchema: map[string]any{"type": "object", "required": []string{"gate_id", "reason"}, "properties": map[string]any{
				"file":    stringProp("Gate ledger file. Defaults to GATES.md."),
				"gate_id": stringProp("Gate id to abandon, for example G2."),
				"reason":  stringProp("Honest reason the gate cannot be met."),
			}},
		},
		{
			Name:        "gates_init",
			Description: "Scaffold a new gate ledger file. Refuses to overwrite an existing file. Each gate spec looks like \"G1: outcome | CHECK: command | EXPECT: expectation\".",
			InputSchema: map[string]any{"type": "object", "required": []string{"gates"}, "properties": map[string]any{
				"file":  stringProp("Gate ledger file to create. Defaults to GATES.md."),
				"scope": stringProp("Gate scope name for the heading."),
				"gates": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Gate specs: \"ID: outcome | CHECK: command | EXPECT: expectation\"."},
			}},
		},
	}
}
