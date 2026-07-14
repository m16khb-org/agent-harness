package validationcli

import "agent-harness/cmd/harness/validationcli/contractauditworker"

type ContractAuditWorkerValidationDeps = contractauditworker.ValidationDeps

func ValidateCommandAudit(binary, root string, seed int64) StepResult {
	return contractauditworker.ValidateCommandAudit(binary, root, seed)
}

func ValidateCommandAuditWithDeps(binary, root string, seed int64, deps ContractAuditWorkerValidationDeps) StepResult {
	return contractauditworker.ValidateCommandAuditWithDeps(binary, root, seed, deps)
}

func ValidateContractCheck(binary, root string) StepResult {
	return contractauditworker.ValidateContractCheck(binary, root)
}

func ValidateContractCheckWithDeps(binary, root string, deps ContractAuditWorkerValidationDeps) StepResult {
	return contractauditworker.ValidateContractCheckWithDeps(binary, root, deps)
}

func ValidateToolConformance(binary, root string) StepResult {
	return contractauditworker.ValidateToolConformance(binary, root)
}

func ValidateToolConformanceWithDeps(binary, root string, deps ContractAuditWorkerValidationDeps) StepResult {
	return contractauditworker.ValidateToolConformanceWithDeps(binary, root, deps)
}

func ValidateWorkerLifecycle(binary, root string, seed int64) StepResult {
	return contractauditworker.ValidateWorkerLifecycle(binary, root, seed)
}

func ValidateWorkerLifecycleWithDeps(binary, root string, seed int64, deps ContractAuditWorkerValidationDeps) StepResult {
	return contractauditworker.ValidateWorkerLifecycleWithDeps(binary, root, seed, deps)
}
