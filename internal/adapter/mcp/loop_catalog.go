package mcp

func LoopTools() []Tool {
	stringProp := func(description string) map[string]any {
		return map[string]any{"type": "string", "description": description}
	}
	stringArrayProp := func(description string) map[string]any {
		return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": description}
	}
	boolProp := func(description string) map[string]any {
		return map[string]any{"type": "boolean", "description": description}
	}
	return []Tool{
		{
			Name:        "loop_start",
			Description: "Create or resume a durable verify-until-done loop contract. This writes state only; it records verify_argv but never executes shell commands.",
			InputSchema: map[string]any{"type": "object", "required": []string{"repo", "name", "goal"}, "properties": map[string]any{
				"repo":         stringProp("Repository path."),
				"name":         stringProp("Stable loop name. Same repo+name resumes an active loop."),
				"goal":         stringProp("Loop goal text."),
				"verify_argv":  stringArrayProp("Command argv to record as intended verification; harness never executes it."),
				"max_attempts": map[string]any{"type": "integer", "description": "Maximum fail attempts before auto-exhaustion."},
			}},
		},
		{
			Name:        "loop_record_attempt",
			Description: "Append a pass/fail attempt with evidence to an active loop contract. This writes state and auto-exhausts after max failed attempts.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "verdict", "evidence"}, "properties": map[string]any{
				"id":       stringProp("Loop id."),
				"verdict":  stringProp("Attempt verdict: pass or fail."),
				"evidence": stringArrayProp("Observable evidence for this attempt."),
			}},
		},
		{
			Name:        "loop_status",
			Description: "Read a durable loop contract status and summary. This does not write state.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id": stringProp("Loop id."),
			}},
		},
		{
			Name:        "loop_stop",
			Description: "Stop a loop contract. success=true requires the latest attempt verdict to be pass; non-success stops require a reason.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id":      stringProp("Loop id."),
				"success": boolProp("Mark the loop succeeded. Requires latest attempt verdict pass."),
				"reason":  stringProp("Reason for non-success stop."),
			}},
		},
	}
}
