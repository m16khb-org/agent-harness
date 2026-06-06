package main

import (
	"path/filepath"
	"strings"
	"time"
)

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
