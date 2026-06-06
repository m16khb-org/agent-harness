package main

import (
	"time"

	"agent-harness/cmd/harness/validationcli"
)

type contractAuditWorkerValidationDeps struct {
	mkdirTemp         func(string, string) (string, error)
	removeAll         func(string) error
	readFile          func(string) ([]byte, error)
	runCommandStep    func(string, string, time.Duration, string, string, ...string) StepResult
	runCommandStepEnv func(string, string, time.Duration, string, []string, string, ...string) StepResult
}

func contractAuditWorkerDepsForValidationCLI(deps contractAuditWorkerValidationDeps) validationcli.ContractAuditWorkerValidationDeps {
	return validationcli.ContractAuditWorkerValidationDeps{
		MkdirTemp:         deps.mkdirTemp,
		RemoveAll:         deps.removeAll,
		ReadFile:          deps.readFile,
		RunCommandStep:    deps.runCommandStep,
		RunCommandStepEnv: deps.runCommandStepEnv,
	}
}

func validateCommandAudit(binary, root string, seed int64) StepResult {
	return validationcli.ValidateCommandAudit(binary, root, seed)
}

func validateCommandAuditWithDeps(binary, root string, seed int64, deps contractAuditWorkerValidationDeps) StepResult {
	return validationcli.ValidateCommandAuditWithDeps(binary, root, seed, contractAuditWorkerDepsForValidationCLI(deps))
}

func validateContractCheck(binary, root string) StepResult {
	return validationcli.ValidateContractCheck(binary, root)
}

func validateContractCheckWithDeps(binary, root string, deps contractAuditWorkerValidationDeps) StepResult {
	return validationcli.ValidateContractCheckWithDeps(binary, root, contractAuditWorkerDepsForValidationCLI(deps))
}

func validateWorkerLifecycle(binary, root string, seed int64) StepResult {
	return validationcli.ValidateWorkerLifecycle(binary, root, seed)
}

func validateWorkerLifecycleWithDeps(binary, root string, seed int64, deps contractAuditWorkerValidationDeps) StepResult {
	return validationcli.ValidateWorkerLifecycleWithDeps(binary, root, seed, contractAuditWorkerDepsForValidationCLI(deps))
}
