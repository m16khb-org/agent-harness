package mcpsmoke

import (
	"encoding/json"
	"fmt"
	"strings"
)

func MCPSmokeInput() string {
	return strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"self-verify","version":"0"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"harness://commit-policy"}}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"harness://state"}}`,
		`{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"harness://docs"}}`,
		`{"jsonrpc":"2.0","id":6,"method":"resources/read","params":{"uri":"harness://command-policy"}}`,
		`{"jsonrpc":"2.0","id":7,"method":"resources/read","params":{"uri":"harness://project-docs"}}`,
		`{"jsonrpc":"2.0","id":8,"method":"resources/read","params":{"uri":"harness://project-doc-upkeep"}}`,
		`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"project_docs_route","arguments":{"repo":".","task":"commit"}}}`,
		`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"state_prune","arguments":{"max_age":"1h"}}}`,
		`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"state_doctor","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"state_migrate","arguments":{}}}`,
	}, "\n") + "\n"
}

func ValidateMCPSmokeContract(step *StepResult) {
	lines := splitLines(step.Stdout)
	if len(lines) != 12 {
		step.OK = false
		step.Error = fmt.Sprintf("expected 12 MCP responses, got %d", len(lines))
		return
	}
	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			step.OK = false
			step.Error = fmt.Sprintf("response %d is invalid JSON: %v", i+1, err)
			return
		}
		if _, ok := obj["result"]; !ok {
			step.OK = false
			step.Error = fmt.Sprintf("response %d has no result", i+1)
			return
		}
	}
	if !MCPSmokeHasExpectedMarkers(step.Stdout) {
		step.OK = false
		step.Error = "MCP smoke did not expose expected tool/resource"
	}
}

func MCPSmokeHasExpectedMarkers(stdout string) bool {
	for _, marker := range MCPSmokeExpectedMarkers() {
		if !strings.Contains(stdout, marker) {
			return false
		}
	}
	return true
}

func MCPSmokeExpectedMarkers() []string {
	return []string{
		"atomic_commit_preflight",
		"docs_index",
		"project_docs_route",
		"project_docs_read",
		"project_docs_update",
		"project_docs_record",
		"api_doc_static_check",
		"api_doc_review",
		"harness://project-docs",
		"harness://project-doc-upkeep",
		"command_policy_check",
		"state_write",
		"state_prune",
		"state_doctor",
		"state_migrate",
		"self_augment",
		"self_augment_lesson",
		"self_verify",
		"self_verify_candidates",
		"self_verify_history",
		"self_verify_compare",
		"self_verify_promote",
		"dry_run",
		"healthy",
		"to_schema",
		"Lore:",
	}
}
