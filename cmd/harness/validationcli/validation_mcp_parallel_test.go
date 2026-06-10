package validationcli

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateMCPWithDepsCoversSuccessAndResponseFailures(t *testing.T) {
	root := t.TempDir()
	deps := MCPValidationDeps{
		MkdirTemp: func(_ string, pattern string) (string, error) { return filepath.Join(root, pattern+"dir"), nil },
		RemoveAll: func(string) error { return nil },
		RunCommandStepEnv: func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) StepResult {
			return StepResult{Label: label, Command: strings.Join(args, " "), OK: true}
		},
		RunCommandStepEnvWithBudget: func(_ string, label string, _ time.Duration, input string, env []string, _ int, _ string, args ...string) StepResult {
			if !strings.Contains(input, "tools/list") || !containsString(env, "HARNESS_STATE_DIR="+filepath.Join(root, "agent-harness-mcp-state-*dir")) {
				return StepResult{Label: label, OK: false, Error: "missing input or env"}
			}
			if !containsString(env, "HARNESS_MCP_DIRECT=1") {
				return StepResult{Label: label, OK: false, Error: "MCP smoke must use direct transport for deterministic batch input"}
			}
			return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: validMCPResponses()}
		},
	}

	step := ValidateMCPWithDeps("harness", root, deps)
	if !step.OK || step.Label != "MCP smoke" || !strings.Contains(step.Stdout, "atomic_commit_preflight") {
		t.Fatalf("expected MCP smoke success, got %+v", step)
	}

	deps.RunCommandStepEnvWithBudget = func(string, string, time.Duration, string, []string, int, string, ...string) StepResult {
		return StepResult{Label: "MCP smoke", OK: true, Stdout: `{"jsonrpc":"2.0","id":1,"result":{}}` + "\n"}
	}
	wrongCount := ValidateMCPWithDeps("harness", root, deps)
	if wrongCount.OK || !strings.Contains(wrongCount.Error, "expected 12 MCP responses, got 1") {
		t.Fatalf("expected response count failure, got %+v", wrongCount)
	}

	deps.RunCommandStepEnvWithBudget = func(string, string, time.Duration, string, []string, int, string, ...string) StepResult {
		return StepResult{Label: "MCP smoke", OK: true, Stdout: strings.Repeat("not-json\n", 12)}
	}
	badJSON := ValidateMCPWithDeps("harness", root, deps)
	if badJSON.OK || !strings.Contains(badJSON.Error, "response 1 is invalid JSON") {
		t.Fatalf("expected invalid JSON failure, got %+v", badJSON)
	}

	deps.RunCommandStepEnvWithBudget = func(string, string, time.Duration, string, []string, int, string, ...string) StepResult {
		return StepResult{Label: "MCP smoke", OK: true, Stdout: strings.Repeat(`{"jsonrpc":"2.0","id":1,"error":{"code":-1}}`+"\n", 12)}
	}
	missingResult := ValidateMCPWithDeps("harness", root, deps)
	if missingResult.OK || !strings.Contains(missingResult.Error, "response 1 has no result") {
		t.Fatalf("expected missing result failure, got %+v", missingResult)
	}

	deps.RunCommandStepEnvWithBudget = func(string, string, time.Duration, string, []string, int, string, ...string) StepResult {
		return StepResult{Label: "MCP smoke", OK: true, Stdout: strings.Repeat(`{"jsonrpc":"2.0","id":1,"result":{}}`+"\n", 12)}
	}
	missingTool := ValidateMCPWithDeps("harness", root, deps)
	if missingTool.OK || missingTool.Error != "MCP smoke did not expose expected tool/resource" {
		t.Fatalf("expected tool/resource failure, got %+v", missingTool)
	}
}

func TestValidateMCPWithDepsCoversTempAndCommandFailure(t *testing.T) {
	deps := MCPValidationDeps{
		MkdirTemp: func(string, string) (string, error) { return "", errors.New("temp failed") },
	}
	if step := ValidateMCPWithDeps("harness", t.TempDir(), deps); step.OK || !strings.Contains(step.Error, "temp failed") {
		t.Fatalf("expected temp failure, got %+v", step)
	}

	root := t.TempDir()
	call := 0
	deps = MCPValidationDeps{
		MkdirTemp: func(_ string, pattern string) (string, error) {
			call++
			if call == 2 {
				return "", errors.New("daemon temp failed")
			}
			return filepath.Join(root, pattern), nil
		},
		RemoveAll: func(string) error { return nil },
	}
	if step := ValidateMCPWithDeps("harness", root, deps); step.OK || !strings.Contains(step.Error, "daemon temp failed") {
		t.Fatalf("expected daemon temp failure, got %+v", step)
	}

	deps = MCPValidationDeps{
		MkdirTemp: func(_ string, pattern string) (string, error) { return filepath.Join(root, pattern), nil },
		RemoveAll: func(string) error { return nil },
		RunCommandStepEnvWithBudget: func(string, string, time.Duration, string, []string, int, string, ...string) StepResult {
			return StepResult{Label: "MCP smoke", OK: false, Error: "mcp failed"}
		},
	}
	if step := ValidateMCPWithDeps("harness", root, deps); step.OK || step.Error != "mcp failed" {
		t.Fatalf("expected command failure passthrough, got %+v", step)
	}
}

func validMCPResponses() string {
	payload := strings.Join([]string{
		"atomic_commit_preflight", "docs_index", "project_docs_route", "project_docs_read", "project_docs_update", "project_docs_record",
		"api_doc_static_check", "api_doc_review", "harness://project-docs", "harness://project-doc-upkeep", "command_policy_check", "state_write",
		"state_prune", "state_doctor", "state_migrate", "self_augment", "self_augment_lesson", "self_verify", "self_verify_candidates",
		"self_verify_history", "self_verify_compare", "self_verify_promote", "dry_run", "healthy", "to_schema", "Lore:",
	}, " ")
	lines := make([]string, 0, 12)
	for id := 1; id <= 12; id++ {
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"text": payload}})
		lines = append(lines, string(body))
	}
	return strings.Join(lines, "\n") + "\n"
}
