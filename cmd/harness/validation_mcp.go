package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type mcpValidationDeps struct {
	mkdirTemp                   func(string, string) (string, error)
	removeAll                   func(string) error
	runCommandStepEnv           func(string, string, time.Duration, string, []string, string, ...string) StepResult
	runCommandStepEnvWithBudget func(string, string, time.Duration, string, []string, int, string, ...string) StepResult
}

func (deps mcpValidationDeps) withDefaults() mcpValidationDeps {
	if deps.mkdirTemp == nil {
		deps.mkdirTemp = os.MkdirTemp
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.runCommandStepEnv == nil {
		deps.runCommandStepEnv = runCommandStepEnv
	}
	if deps.runCommandStepEnvWithBudget == nil {
		deps.runCommandStepEnvWithBudget = runCommandStepEnvWithBudget
	}
	return deps
}

func validateMCP(binary, root string) StepResult {
	return validateMCPWithDeps(binary, root, mcpValidationDeps{})
}

func validateMCPWithDeps(binary, root string, deps mcpValidationDeps) StepResult {
	deps = deps.withDefaults()
	tempState, err := deps.mkdirTemp("", "agent-harness-mcp-state-*")
	if err != nil {
		return failedStep("MCP smoke", err)
	}
	defer deps.removeAll(tempState)
	daemonDir, err := deps.mkdirTemp("", "ahd-*")
	if err != nil {
		return failedStep("MCP smoke", err)
	}
	defer deps.removeAll(daemonDir)
	env := []string{
		"HARNESS_STATE_DIR=" + tempState,
		"HARNESS_DAEMON_DIR=" + daemonDir,
	}
	defer deps.runCommandStepEnv(root, "MCP daemon stop", 5*time.Second, "", env, binary, "daemon", "stop", "--json")

	input := strings.Join([]string{
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
	step := deps.runCommandStepEnvWithBudget(root, "MCP smoke", 30*time.Second, input, env, 0, binary, "mcp")
	if !step.OK {
		return step
	}
	lines := splitLines(step.Stdout)
	if len(lines) != 12 {
		step.OK = false
		step.Error = fmt.Sprintf("expected 12 MCP responses, got %d", len(lines))
		return step
	}
	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			step.OK = false
			step.Error = fmt.Sprintf("response %d is invalid JSON: %v", i+1, err)
			return step
		}
		if _, ok := obj["result"]; !ok {
			step.OK = false
			step.Error = fmt.Sprintf("response %d has no result", i+1)
			return step
		}
	}
	if !strings.Contains(step.Stdout, "atomic_commit_preflight") || !strings.Contains(step.Stdout, "docs_index") || !strings.Contains(step.Stdout, "project_docs_route") || !strings.Contains(step.Stdout, "project_docs_read") || !strings.Contains(step.Stdout, "project_docs_update") || !strings.Contains(step.Stdout, "project_docs_record") || !strings.Contains(step.Stdout, "api_doc_static_check") || !strings.Contains(step.Stdout, "api_doc_review") || !strings.Contains(step.Stdout, "harness://project-docs") || !strings.Contains(step.Stdout, "harness://project-doc-upkeep") || !strings.Contains(step.Stdout, "command_policy_check") || !strings.Contains(step.Stdout, "state_write") || !strings.Contains(step.Stdout, "state_prune") || !strings.Contains(step.Stdout, "state_doctor") || !strings.Contains(step.Stdout, "state_migrate") || !strings.Contains(step.Stdout, "self_augment") || !strings.Contains(step.Stdout, "self_augment_lesson") || !strings.Contains(step.Stdout, "self_verify") || !strings.Contains(step.Stdout, "self_verify_candidates") || !strings.Contains(step.Stdout, "self_verify_history") || !strings.Contains(step.Stdout, "self_verify_compare") || !strings.Contains(step.Stdout, "self_verify_promote") || !strings.Contains(step.Stdout, "dry_run") || !strings.Contains(step.Stdout, "healthy") || !strings.Contains(step.Stdout, "to_schema") || !strings.Contains(step.Stdout, "Lore:") {
		step.OK = false
		step.Error = "MCP smoke did not expose expected tool/resource"
	}
	step.Stdout, step.StdoutTruncated, step.StdoutBytes = tailWithBudget(step.Stdout, selfVerifyAggregateOutputBudgetBytes)
	return step
}
