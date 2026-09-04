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
		RunSDKSmoke: func(_ string, _ string, env []string, _ time.Duration) StepResult {
			if !containsString(env, "ISSUEOPS_STATE_DIR="+filepath.Join(root, "issueops-mcp-state-*dir")) {
				return StepResult{Label: "MCP smoke", OK: false, Error: "missing env"}
			}
			if !containsString(env, "ISSUEOPS_MCP_DIRECT=1") {
				return StepResult{Label: "MCP smoke", OK: false, Error: "MCP smoke must use direct SDK transport"}
			}
			return StepResult{Label: "MCP smoke", Command: "issueops mcp", OK: true, Stdout: validMCPResponses()}
		},
	}

	step := ValidateMCPWithDeps("issueops", root, deps)
	if !step.OK || step.Label != "MCP smoke" || !strings.Contains(step.Stdout, "atomic_commit_preflight") {
		t.Fatalf("expected MCP smoke success, got %+v", step)
	}

	deps.RunSDKSmoke = func(string, string, []string, time.Duration) StepResult {
		return StepResult{Label: "MCP smoke", OK: true, Stdout: `{}` + "\n"}
	}
	wrongCount := ValidateMCPWithDeps("issueops", root, deps)
	if wrongCount.OK || !strings.Contains(wrongCount.Error, "expected 11 MCP SDK results, got 1") {
		t.Fatalf("expected response count failure, got %+v", wrongCount)
	}

	deps.RunSDKSmoke = func(string, string, []string, time.Duration) StepResult {
		return StepResult{Label: "MCP smoke", OK: true, Stdout: strings.Repeat("not-json\n", 11)}
	}
	badJSON := ValidateMCPWithDeps("issueops", root, deps)
	if badJSON.OK || !strings.Contains(badJSON.Error, "SDK result 1 is invalid JSON") {
		t.Fatalf("expected invalid JSON failure, got %+v", badJSON)
	}

	deps.RunSDKSmoke = func(string, string, []string, time.Duration) StepResult {
		return StepResult{Label: "MCP smoke", OK: true, Stdout: strings.Repeat(`{}`+"\n", 11)}
	}
	missingTool := ValidateMCPWithDeps("issueops", root, deps)
	if missingTool.OK || missingTool.Error != "MCP smoke did not expose expected tool/resource" {
		t.Fatalf("expected tool/resource failure, got %+v", missingTool)
	}
}

func TestValidateMCPWithDepsCoversTempAndCommandFailure(t *testing.T) {
	deps := MCPValidationDeps{
		MkdirTemp: func(string, string) (string, error) { return "", errors.New("temp failed") },
	}
	if step := ValidateMCPWithDeps("issueops", t.TempDir(), deps); step.OK || !strings.Contains(step.Error, "temp failed") {
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
	if step := ValidateMCPWithDeps("issueops", root, deps); step.OK || !strings.Contains(step.Error, "daemon temp failed") {
		t.Fatalf("expected daemon temp failure, got %+v", step)
	}

	deps = MCPValidationDeps{
		MkdirTemp: func(_ string, pattern string) (string, error) { return filepath.Join(root, pattern), nil },
		RemoveAll: func(string) error { return nil },
		RunSDKSmoke: func(string, string, []string, time.Duration) StepResult {
			return StepResult{Label: "MCP smoke", OK: false, Error: "mcp failed"}
		},
	}
	if step := ValidateMCPWithDeps("issueops", root, deps); step.OK || step.Error != "mcp failed" {
		t.Fatalf("expected command failure passthrough, got %+v", step)
	}
}

func validMCPResponses() string {
	payload := strings.Join([]string{
		"atomic_commit_preflight", "docs_index", "project_docs_route", "project_docs_read", "project_docs_revise", "project_docs_append",
		"api_doc_static_check", "api_doc_review", "issueops://project-docs", "issueops://project-doc-upkeep", "command_policy_check", "state_write",
		"state_prune", "state_doctor", "self_augment", "self_augment_lesson", "self_verify", "self_verify_candidates",
		"self_verify_history", "self_verify_compare", "self_verify_promote", "dry_run", "healthy", "Lore:",
	}, " ")
	lines := make([]string, 0, 11)
	for id := 1; id <= 11; id++ {
		body, _ := json.Marshal(map[string]any{"id": id, "text": payload})
		lines = append(lines, string(body))
	}
	return strings.Join(lines, "\n") + "\n"
}
