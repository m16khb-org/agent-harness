package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core"
)

type contractAuditWorkerValidationDeps struct {
	mkdirTemp         func(string, string) (string, error)
	removeAll         func(string) error
	readFile          func(string) ([]byte, error)
	runCommandStep    func(string, string, time.Duration, string, string, ...string) StepResult
	runCommandStepEnv func(string, string, time.Duration, string, []string, string, ...string) StepResult
}

func (deps contractAuditWorkerValidationDeps) withDefaults() contractAuditWorkerValidationDeps {
	if deps.mkdirTemp == nil {
		deps.mkdirTemp = os.MkdirTemp
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.readFile == nil {
		deps.readFile = os.ReadFile
	}
	if deps.runCommandStep == nil {
		deps.runCommandStep = runCommandStep
	}
	if deps.runCommandStepEnv == nil {
		deps.runCommandStepEnv = runCommandStepEnv
	}
	return deps
}

func validateCommandAudit(binary, root string, seed int64) StepResult {
	return validateCommandAuditWithDeps(binary, root, seed, contractAuditWorkerValidationDeps{})
}

func validateCommandAuditWithDeps(binary, root string, seed int64, deps contractAuditWorkerValidationDeps) StepResult {
	_ = seed
	deps = deps.withDefaults()
	auditDir, err := deps.mkdirTemp("", "agent-harness-audit-*")
	if err != nil {
		return failedStep("command audit smoke", err)
	}
	defer deps.removeAll(auditDir)
	auditLog := filepath.Join(auditDir, "audit.jsonl")
	step := deps.runCommandStepEnv(root, "command audit smoke", 30*time.Second, "", []string{"HARNESS_AUDIT_LOG=" + auditLog}, binary, "policy", "audit", "--workspace-root", root, "--cwd", root, "--json", "--", "git", "status", "--short")
	if !step.OK {
		return step
	}
	b, err := deps.readFile(auditLog)
	if err != nil {
		return failedStep("command audit smoke", err)
	}
	errs := []string{}
	text := string(b)
	if !strings.Contains(text, "command_policy_audit") || !strings.Contains(text, "audit_log_id") {
		errs = append(errs, "audit log missing command_policy_audit fields")
	}
	if strings.Contains(strings.ToLower(text), "secret-value") || strings.Contains(text, "sk-123") {
		errs = append(errs, "audit log contains unredacted secret fixture")
	}
	return assertionStep("command audit smoke", time.Now(), errs)
}

func validateContractCheck(binary, root string) StepResult {
	return validateContractCheckWithDeps(binary, root, contractAuditWorkerValidationDeps{})
}

func validateContractCheckWithDeps(binary, root string, deps contractAuditWorkerValidationDeps) StepResult {
	deps = deps.withDefaults()
	step := deps.runCommandStep(root, "contract check", 30*time.Second, "", binary, "contract", "check", "--json")
	if !step.OK {
		return step
	}
	var result CompatibilityContract
	if err := json.Unmarshal([]byte(step.Stdout), &result); err != nil {
		return failedStep("contract check", err)
	}
	errs := []string{}
	if !result.OK || result.Hash == "" {
		errs = append(errs, "contract did not pass or hash is empty")
	}
	for _, want := range []string{"worker", "contract", "policy"} {
		found := false
		for _, command := range result.CLICommands {
			if command.Name == want {
				found = true
			}
		}
		if !found {
			errs = append(errs, "missing CLI command "+want)
		}
	}
	return assertionStep("contract check", time.Now(), errs)
}

func validateWorkerLifecycle(binary, root string, seed int64) StepResult {
	return validateWorkerLifecycleWithDeps(binary, root, seed, contractAuditWorkerValidationDeps{})
}

func validateWorkerLifecycleWithDeps(binary, root string, seed int64, deps contractAuditWorkerValidationDeps) StepResult {
	deps = deps.withDefaults()
	workerDir, err := deps.mkdirTemp("", "agent-harness-worker-*")
	if err != nil {
		return failedStep("worker lifecycle smoke", err)
	}
	defer deps.removeAll(workerDir)
	env := []string{"HARNESS_WORKER_DIR=" + workerDir}
	enqueue := deps.runCommandStepEnv(root, "worker lifecycle enqueue", 30*time.Second, "", env, binary, "worker", "enqueue", "--kind", "smoke", "--payload", fmt.Sprintf("seed=%d", seed), "--json")
	if !enqueue.OK {
		return enqueue
	}
	var job core.WorkerJob
	if err := json.Unmarshal([]byte(enqueue.Stdout), &job); err != nil {
		return failedStep("worker lifecycle smoke", err)
	}
	status := deps.runCommandStepEnv(root, "worker lifecycle status", 30*time.Second, "", env, binary, "worker", "status", "--id", job.ID, "--json")
	cancel := deps.runCommandStepEnv(root, "worker lifecycle cancel", 30*time.Second, "", env, binary, "worker", "cancel", "--id", job.ID, "--json")
	list := deps.runCommandStepEnv(root, "worker lifecycle list", 30*time.Second, "", env, binary, "worker", "list", "--json")
	errs := []string{}
	for _, step := range []StepResult{status, cancel, list} {
		if !step.OK {
			errs = append(errs, step.Label+" failed")
		}
	}
	if !job.NoShell || job.Status != core.WorkerStatusQueued {
		errs = append(errs, "worker job is not queued no-shell")
	}
	return assertionStep("worker lifecycle smoke", time.Now(), errs)
}
