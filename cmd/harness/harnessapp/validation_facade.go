package harnessapp

import (
	"agent-harness/cmd/harness/validationcli"
)

type ClaudeMCPDuplicateWarning = validationcli.ClaudeMCPDuplicateWarning

func validateInspect(binary, root string) StepResult {
	return validationcli.ValidateInspect(binary, root)
}

func validateDocsIndex(binary, root string) StepResult {
	return validationcli.ValidateDocsIndex(binary, root)
}

func validateCommandPolicy(binary, root string) StepResult {
	return validationcli.ValidateCommandPolicy(binary, root)
}

func validateMCP(binary, root string) StepResult {
	return validationcli.ValidateMCP(binary, root)
}

func validateStateRoundtrip(binary, root string, seed int64) StepResult {
	return validationcli.ValidateStateRoundtrip(binary, root, seed)
}

func validateInstallDryRunSmoke(binary, root string, seed int64) StepResult {
	return validationcli.ValidateInstallDryRunSmoke(binary, root, seed)
}

func validateParallelTempIsolation(binary, root string, seed int64) StepResult {
	return validationcli.ValidateParallelTempIsolation(binary, root, seed)
}

func validateDaemonRestartResilience(binary, root string, seed int64) StepResult {
	return validationcli.ValidateDaemonRestartResilience(binary, root, seed)
}

func validatePreflightFuzz(binary, root string, seed int64) StepResult {
	return validationcli.ValidatePreflightFuzz(binary, root, seed)
}

func validateWebFetchBattery(binary, root string, seed int64) StepResult {
	return validationcli.ValidateWebFetchBattery(binary, root, seed)
}

func validateCommandAudit(binary, root string, seed int64) StepResult {
	return validationcli.ValidateCommandAudit(binary, root, seed)
}

func validateContractCheck(binary, root string) StepResult {
	return validationcli.ValidateContractCheck(binary, root)
}

func validateToolConformance(binary, root string) StepResult {
	return validationcli.ValidateToolConformance(binary, root)
}

func validateWorkerLifecycle(binary, root string, seed int64) StepResult {
	return validationcli.ValidateWorkerLifecycle(binary, root, seed)
}

func validateSelfVerifyCandidateExport(binary, root string, seed int64) StepResult {
	return validationcli.ValidateSelfVerifyCandidateExport(binary, root, seed)
}

func validateStepBudgetBaseline(binary, root string, seed int64) StepResult {
	return validationcli.ValidateStepBudgetBaseline(binary, root, seed)
}

func validateRedactionAudit(root string) StepResult {
	return validationcli.ValidateRedactionAudit(root)
}

func validateQAGate(root string) StepResult {
	return validationcli.ValidateQAGate(root)
}

func validateHarnessInvariants(root string) StepResult {
	return validationcli.ValidateHarnessInvariants(root)
}

func validateNativeIntegration(root string) StepResult {
	return validationcli.ValidateNativeIntegration(root)
}
