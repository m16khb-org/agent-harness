package mcpsmoke

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ValidateMCPSmokeContract(step *StepResult) {
	lines := splitLines(step.Stdout)
	if len(lines) != 11 {
		step.OK = false
		step.Error = fmt.Sprintf("expected 11 MCP SDK results, got %d", len(lines))
		return
	}
	for i, line := range lines {
		var result any
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			step.OK = false
			step.Error = fmt.Sprintf("SDK result %d is invalid JSON: %v", i+1, err)
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
		"project_docs_revise",
		"project_docs_append",
		"api_doc_static_check",
		"api_doc_review",
		"issueops://project-docs",
		"issueops://project-doc-upkeep",
		"command_policy_check",
		"state_write",
		"state_prune",
		"state_doctor",
		"self_augment",
		"self_augment_lesson",
		"self_verify",
		"self_verify_candidates",
		"self_verify_history",
		"self_verify_compare",
		"self_verify_promote",
		"dry_run",
		"healthy",
		"Lore:",
	}
}
