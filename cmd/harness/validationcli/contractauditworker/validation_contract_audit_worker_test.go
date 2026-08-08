package contractauditworker

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/cmd/harness/contractcli"
	"agent-harness/internal/adapter/cli"
	workercontract "agent-harness/internal/contract/worker"
)

func TestValidateCommandAuditWithDepsCoversSuccessCommandReadAndContractFailures(t *testing.T) {
	root := t.TempDir()
	auditLog := filepath.Join(root, "audit.jsonl")
	deps := ValidationDeps{
		MkdirTemp: func(string, string) (string, error) { return root, nil },
		RemoveAll: func(string) error { return nil },
		ReadFile: func(path string) ([]byte, error) {
			if path != auditLog {
				t.Fatalf("unexpected audit path %s", path)
			}
			return []byte(`{"kind":"command_policy_audit","audit_log_id":"abc","payload":"<redacted>"}`), nil
		},
		RunCommandStepEnv: func(_ string, label string, _ time.Duration, _ string, env []string, _ string, args ...string) StepResult {
			if label != "command audit smoke" || !containsString(env, "HARNESS_AUDIT_LOG="+auditLog) || !containsString(args, "audit") {
				return StepResult{Label: label, OK: false, Error: "bad command envelope"}
			}
			return StepResult{Label: label, Command: strings.Join(args, " "), OK: true}
		},
	}

	step := ValidateCommandAuditWithDeps("harness", root, 11, deps)
	if !step.OK || step.Label != "command audit smoke" {
		t.Fatalf("expected audit success, got %+v", step)
	}

	deps.MkdirTemp = func(string, string) (string, error) { return "", errors.New("temp failed") }
	if step := ValidateCommandAuditWithDeps("harness", root, 11, deps); step.OK || !strings.Contains(step.Error, "temp failed") {
		t.Fatalf("expected temp failure, got %+v", step)
	}

	deps.MkdirTemp = func(string, string) (string, error) { return root, nil }
	deps.RunCommandStepEnv = func(string, string, time.Duration, string, []string, string, ...string) StepResult {
		return StepResult{Label: "command audit smoke", OK: false, Error: "audit command failed"}
	}
	if step := ValidateCommandAuditWithDeps("harness", root, 11, deps); step.OK || step.Error != "audit command failed" {
		t.Fatalf("expected command failure passthrough, got %+v", step)
	}

	deps.RunCommandStepEnv = func(string, string, time.Duration, string, []string, string, ...string) StepResult {
		return StepResult{Label: "command audit smoke", OK: true}
	}
	deps.ReadFile = func(string) ([]byte, error) { return nil, errors.New("read failed") }
	if step := ValidateCommandAuditWithDeps("harness", root, 11, deps); step.OK || !strings.Contains(step.Error, "read failed") {
		t.Fatalf("expected read failure, got %+v", step)
	}

	deps.ReadFile = func(string) ([]byte, error) { return []byte("secret-value sk-123"), nil }
	badLog := ValidateCommandAuditWithDeps("harness", root, 11, deps)
	for _, want := range []string{"audit log missing command_policy_audit fields", "audit log contains unredacted secret fixture"} {
		if !strings.Contains(badLog.Error, want) {
			t.Fatalf("expected %q in %+v", want, badLog)
		}
	}
}

func TestValidateContractAuditWorkerWrappersUseExecutableSurface(t *testing.T) {
	root := t.TempDir()
	binary := writeContractAuditWorkerFakeBinary(t, root)

	for _, tc := range []struct {
		name string
		run  func() StepResult
	}{
		{name: "command audit", run: func() StepResult { return ValidateCommandAudit(binary, root, 101) }},
		{name: "contract check", run: func() StepResult { return ValidateContractCheck(binary, root) }},
		{name: "tool conformance", run: func() StepResult { return ValidateToolConformance(binary, root) }},
		{name: "worker lifecycle", run: func() StepResult { return ValidateWorkerLifecycle(binary, root, 101) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if step := tc.run(); !step.OK {
				t.Fatalf("expected wrapper success, got %+v", step)
			}
		})
	}
}

func writeContractAuditWorkerFakeBinary(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "fake-harness")
	body := `#!/bin/sh
set -eu
case "$*" in
  "policy audit --workspace-root "*)
    printf '{"kind":"command_policy_audit","audit_log_id":"abc"}\n' > "$HARNESS_AUDIT_LOG"
    printf '{"ok":true}\n'
    ;;
  "contract check --json")
    printf '{"ok":true,"hash":"abc","cli_commands":[{"name":"worker"},{"name":"contract"},{"name":"policy"}]}\n'
    ;;
	  "contract conformance baseline --json")
	    printf '{"ok":true,"case_count":10,"gate":{"decision":"baseline_passed"}}\n'
	    ;;
  "worker enqueue "*)
    printf '{"ok":true,"id":"job-1","kind":"smoke","status":"queued","no_shell":true}\n'
    ;;
  "worker status "*|"worker cancel "*|"worker list --json")
    printf '{"ok":true}\n'
    ;;
  *)
    echo "unexpected fake harness args: $*" >&2
    exit 2
    ;;
esac
`
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidateContractCheckWithDepsCoversSuccessParseAndContractFailures(t *testing.T) {
	root := t.TempDir()
	contract := contractcli.CompatibilityContract{OK: true, Hash: "abc", CLICommands: []cli.Command{{Name: "worker"}, {Name: "contract"}, {Name: "policy"}}}
	body, _ := json.Marshal(contract)
	deps := ValidationDeps{
		RunCommandStep: func(_ string, label string, _ time.Duration, _ string, _ string, args ...string) StepResult {
			if label != "contract check" || strings.Join(args, " ") != "contract check --json" {
				return StepResult{Label: label, OK: false, Error: "bad contract command"}
			}
			return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
		},
	}
	if step := ValidateContractCheckWithDeps("harness", root, deps); !step.OK {
		t.Fatalf("expected contract success, got %+v", step)
	}

	deps.RunCommandStep = func(string, string, time.Duration, string, string, ...string) StepResult {
		return StepResult{Label: "contract check", OK: false, Error: "contract command failed"}
	}
	if step := ValidateContractCheckWithDeps("harness", root, deps); step.OK || step.Error != "contract command failed" {
		t.Fatalf("expected command failure passthrough, got %+v", step)
	}

	deps.RunCommandStep = func(string, string, time.Duration, string, string, ...string) StepResult {
		return StepResult{Label: "contract check", OK: true, Stdout: `{"ok":`}
	}
	if step := ValidateContractCheckWithDeps("harness", root, deps); step.OK || !strings.Contains(step.Error, "unexpected end") {
		t.Fatalf("expected parse failure, got %+v", step)
	}

	bad := contractcli.CompatibilityContract{OK: false, Hash: "", CLICommands: []cli.Command{{Name: "worker"}}}
	badBody, _ := json.Marshal(bad)
	deps.RunCommandStep = func(string, string, time.Duration, string, string, ...string) StepResult {
		return StepResult{Label: "contract check", OK: true, Stdout: string(badBody)}
	}
	failed := ValidateContractCheckWithDeps("harness", root, deps)
	for _, want := range []string{"contract did not pass or hash is empty", "missing CLI command contract", "missing CLI command policy"} {
		if !strings.Contains(failed.Error, want) {
			t.Fatalf("expected %q in %+v", want, failed)
		}
	}
}

func TestValidateToolConformanceWithDepsAddsTypedFailureEvidence(t *testing.T) {
	root := t.TempDir()
	deps := ValidationDeps{
		RunCommandStep: func(_ string, label string, _ time.Duration, _ string, _ string, args ...string) StepResult {
			if label != "tool contract conformance" || strings.Join(args, " ") != "contract conformance baseline --json" {
				return StepResult{Label: label, OK: false, Error: "bad conformance command"}
			}
			return StepResult{Label: label, OK: true, Stdout: `{"ok":true,"case_count":10,"gate":{"decision":"baseline_passed"}}`}
		},
	}
	if step := ValidateToolConformanceWithDeps("harness", root, deps); !step.OK {
		t.Fatalf("expected conformance success, got %+v", step)
	}

	deps.RunCommandStep = func(string, string, time.Duration, string, string, ...string) StepResult {
		return StepResult{Label: "tool contract conformance", OK: true, Stdout: `{"ok":false,"case_count":9,"gate":{"decision":"inconclusive"}}`}
	}
	failed := ValidateToolConformanceWithDeps("harness", root, deps)
	if failed.OK || len(failed.FailureEvidence) != 1 || failed.FailureEvidence[0].Code != "baseline_contract_failed" {
		t.Fatalf("expected typed conformance failure, got %+v", failed)
	}
}

func TestValidateWorkerLifecycleWithDepsCoversSuccessParseCommandAndContractFailures(t *testing.T) {
	root := t.TempDir()
	workerDir := filepath.Join(root, "worker")
	queued := workercontract.WorkerJob{OK: true, ID: "job-1", Kind: "smoke", Status: workercontract.WorkerStatusQueued, NoShell: true}
	queuedBody, _ := json.Marshal(queued)
	deps := ValidationDeps{
		MkdirTemp: func(string, string) (string, error) { return workerDir, nil },
		RemoveAll: func(string) error { return nil },
		RunCommandStepEnv: func(_ string, label string, _ time.Duration, _ string, env []string, _ string, args ...string) StepResult {
			if !containsString(env, "HARNESS_WORKER_DIR="+workerDir) {
				return StepResult{Label: label, OK: false, Error: "missing worker env"}
			}
			if strings.Contains(label, "enqueue") {
				return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(queuedBody)}
			}
			return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: `{}`}
		},
	}
	if step := ValidateWorkerLifecycleWithDeps("harness", root, 7, deps); !step.OK {
		t.Fatalf("expected worker success, got %+v", step)
	}

	deps.MkdirTemp = func(string, string) (string, error) { return "", errors.New("worker temp failed") }
	if step := ValidateWorkerLifecycleWithDeps("harness", root, 7, deps); step.OK || !strings.Contains(step.Error, "worker temp failed") {
		t.Fatalf("expected temp failure, got %+v", step)
	}

	deps.MkdirTemp = func(string, string) (string, error) { return workerDir, nil }
	deps.RunCommandStepEnv = func(string, string, time.Duration, string, []string, string, ...string) StepResult {
		return StepResult{Label: "worker lifecycle enqueue", OK: false, Error: "enqueue failed"}
	}
	if step := ValidateWorkerLifecycleWithDeps("harness", root, 7, deps); step.OK || step.Error != "enqueue failed" {
		t.Fatalf("expected enqueue passthrough, got %+v", step)
	}

	deps.RunCommandStepEnv = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) StepResult {
		if strings.Contains(label, "enqueue") {
			return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: `{"ok":`}
		}
		return StepResult{Label: label, OK: true}
	}
	if step := ValidateWorkerLifecycleWithDeps("harness", root, 7, deps); step.OK || !strings.Contains(step.Error, "unexpected end") {
		t.Fatalf("expected enqueue parse failure, got %+v", step)
	}

	badJob := workercontract.WorkerJob{OK: true, ID: "job-2", Status: workercontract.WorkerStatusRunning, NoShell: false}
	badBody, _ := json.Marshal(badJob)
	deps.RunCommandStepEnv = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) StepResult {
		if strings.Contains(label, "enqueue") {
			return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(badBody)}
		}
		if strings.Contains(label, "status") {
			return StepResult{Label: label, OK: false, Error: "status failed"}
		}
		return StepResult{Label: label, OK: true}
	}
	failed := ValidateWorkerLifecycleWithDeps("harness", root, 7, deps)
	for _, want := range []string{"worker lifecycle status failed", "worker job is not queued no-shell"} {
		if !strings.Contains(failed.Error, want) {
			t.Fatalf("expected %q in %+v", want, failed)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
