package validationcli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core"
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

func TestValidateParallelTempIsolationWithDepsCoversSuccessErrorsAndCollision(t *testing.T) {
	root := t.TempDir()
	deps := parallelIsolationValidationDeps{
		runProbe: func(_ string, _ string, seed int64, worker int) parallelIsolationProbe {
			base := filepath.Join(root, fmt.Sprintf("worker-%d-%d", seed, worker))
			return parallelIsolationProbe{Worker: worker, TempRoot: base, StateDir: base + "/state", DaemonDir: base + "/daemon", ArtifactPath: base + "/build/harness", Key: fmt.Sprintf("parallel-%d-%d", seed, worker), Commands: []string{fmt.Sprintf("write-%d", worker)}}
		},
	}
	step := validateParallelTempIsolationWithDeps("harness", root, 7, deps)
	if !step.OK || step.Label != "parallel isolation" || !strings.Contains(step.Command, "write-0") || !strings.Contains(step.Stdout, `"workers": 3`) {
		t.Fatalf("expected parallel success, got %+v", step)
	}

	deps.runProbe = func(_ string, _ string, seed int64, worker int) parallelIsolationProbe {
		return parallelIsolationProbe{Worker: worker, TempRoot: filepath.Join(root, "same"), StateDir: filepath.Join(root, "state", fmt.Sprint(worker)), DaemonDir: filepath.Join(root, "daemon", fmt.Sprint(worker)), ArtifactPath: filepath.Join(root, "artifact", fmt.Sprint(worker)), Key: fmt.Sprintf("parallel-%d-%d", seed, worker)}
	}
	collision := validateParallelTempIsolationWithDeps("harness", root, 7, deps)
	if collision.OK || !strings.Contains(collision.Error, "path collision:") {
		t.Fatalf("expected path collision, got %+v", collision)
	}

	deps.runProbe = func(_ string, _ string, seed int64, worker int) parallelIsolationProbe {
		return parallelIsolationProbe{Worker: worker, TempRoot: filepath.Join(root, fmt.Sprint(worker)), StateDir: filepath.Join(root, "state", fmt.Sprint(worker)), DaemonDir: filepath.Join(root, "daemon", fmt.Sprint(worker)), ArtifactPath: filepath.Join(root, "artifact", fmt.Sprint(worker)), Key: fmt.Sprintf("parallel-%d-%d", seed, worker), Error: "boom"}
	}
	failed := validateParallelTempIsolationWithDeps("harness", root, 7, deps)
	if failed.OK || !strings.Contains(failed.Error, "worker 0: boom") {
		t.Fatalf("expected worker error aggregation, got %+v", failed)
	}
}

func TestRunParallelIsolationProbeWithDepsCoversCommandAndContractFailures(t *testing.T) {
	root := t.TempDir()
	deps := parallelIsolationProbeDeps{
		mkdirTemp: func(_ string, pattern string) (string, error) { return filepath.Join(root, pattern+"dir"), nil },
		removeAll: func(string) error { return nil },
		mkdirAll:  func(string, os.FileMode) error { return nil },
		writeFile: func(string, []byte, os.FileMode) error { return nil },
		runCommandStepEnv: func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) StepResult {
			switch {
			case strings.Contains(label, "write"):
				return StepResult{Label: label, Command: strings.Join(args, " "), OK: true}
			case strings.Contains(label, "read"):
				body, _ := json.Marshal(core.StateResult{OK: true, Record: core.StateRecord{Key: "parallel-9-2", Content: "worker=2 seed=9"}})
				return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
			default:
				body, _ := json.Marshal(core.StateListResult{OK: true, Keys: []string{"parallel-9-2"}})
				return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
			}
		},
	}
	probe := runParallelIsolationProbeWithDeps("harness", root, 9, 2, deps)
	if probe.Error != "" || probe.Key != "parallel-9-2" || len(probe.Commands) != 3 {
		t.Fatalf("expected successful probe, got %+v", probe)
	}

	deps.runCommandStepEnv = func(string, string, time.Duration, string, []string, string, ...string) StepResult {
		return StepResult{OK: false, Error: "write denied"}
	}
	writeFail := runParallelIsolationProbeWithDeps("harness", root, 9, 2, deps)
	if !strings.Contains(writeFail.Error, "state write failed: write denied") {
		t.Fatalf("expected write failure, got %+v", writeFail)
	}

	deps.runCommandStepEnv = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) StepResult {
		if strings.Contains(label, "read") {
			return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: `{"ok":`}
		}
		return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: `{}`}
	}
	badReadJSON := runParallelIsolationProbeWithDeps("harness", root, 9, 2, deps)
	if !strings.Contains(badReadJSON.Error, "state read parse failed") {
		t.Fatalf("expected read parse failure, got %+v", badReadJSON)
	}

	deps.runCommandStepEnv = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) StepResult {
		if strings.Contains(label, "read") {
			body, _ := json.Marshal(core.StateResult{OK: true, Record: core.StateRecord{Key: "other", Content: "wrong"}})
			return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
		}
		return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: `{}`}
	}
	wrongContent := runParallelIsolationProbeWithDeps("harness", root, 9, 2, deps)
	if wrongContent.Error != "state read returned another worker's content" {
		t.Fatalf("expected content failure, got %+v", wrongContent)
	}

	deps.runCommandStepEnv = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) StepResult {
		if strings.Contains(label, "read") {
			body, _ := json.Marshal(core.StateResult{OK: true, Record: core.StateRecord{Key: "parallel-9-2", Content: "worker=2 seed=9"}})
			return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
		}
		return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: `{"ok":`}
	}
	badListJSON := runParallelIsolationProbeWithDeps("harness", root, 9, 2, deps)
	if !strings.Contains(badListJSON.Error, "state list parse failed") {
		t.Fatalf("expected list parse failure, got %+v", badListJSON)
	}

	deps.runCommandStepEnv = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) StepResult {
		if strings.Contains(label, "read") {
			body, _ := json.Marshal(core.StateResult{OK: true, Record: core.StateRecord{Key: "parallel-9-2", Content: "worker=2 seed=9"}})
			return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
		}
		body, _ := json.Marshal(core.StateListResult{OK: true, Keys: []string{"parallel-9-2", "leak"}})
		return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
	}
	leaked := runParallelIsolationProbeWithDeps("harness", root, 9, 2, deps)
	if !strings.Contains(leaked.Error, "state list leaked keys across workers") {
		t.Fatalf("expected list leak failure, got %+v", leaked)
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
