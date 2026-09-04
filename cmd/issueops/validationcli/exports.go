package validationcli

import (
	"issueops/cmd/issueops/validationcli/goformat"
	"issueops/cmd/issueops/validationcli/nativeintegration"
	webfetchvalidation "issueops/cmd/issueops/validationcli/webfetch"
)

func ValidateCommandPolicy(binary, root string) StepResult {
	return validateCommandPolicy(binary, root)
}

func ValidateCommandAuditSmoke(binary, root string, seed int64) StepResult {
	return ValidateCommandAudit(binary, root, seed)
}

func ValidateContractCheckSmoke(binary, root string) StepResult {
	return ValidateContractCheck(binary, root)
}

func ValidateWorkerLifecycleSmoke(binary, root string, seed int64) StepResult {
	return ValidateWorkerLifecycle(binary, root, seed)
}

func ValidateStateRoundtrip(binary, root string, seed int64) StepResult {
	return validateStateRoundtrip(binary, root, seed)
}

func ValidateParallelTempIsolation(binary, root string, seed int64) StepResult {
	return validateParallelTempIsolation(binary, root, seed)
}

func ValidateInstallDryRunSmoke(binary, root string, seed int64) StepResult {
	return validateInstallDryRunSmoke(binary, root, seed)
}

func ValidateDaemonRestartResilience(binary, root string, seed int64) StepResult {
	return validateDaemonRestartResilience(binary, root, seed)
}

func ValidatePreflightFuzz(binary, root string, seed int64) StepResult {
	return validatePreflightFuzz(binary, root, seed)
}

func ValidateWebFetchBattery(binary, root string, seed int64) StepResult {
	return webfetchvalidation.Validate(binary, root, seed)
}

func ValidateInspect(binary, root string) StepResult {
	return validateInspect(binary, root)
}

func ValidateDocsIndex(binary, root string) StepResult {
	return validateDocsIndex(binary, root)
}

func ValidateRedactionAudit(root string) StepResult {
	return validateRedactionAudit(root)
}

func ValidateGoFormat(root string) StepResult {
	return goformat.Validate(root)
}

func ValidateQAGate(root string) StepResult {
	return validateQAGate(root)
}

func ValidateNativeIntegration(root string) StepResult {
	return validateNativeIntegration(root)
}

func DetectClaudeMCPDuplicateWarnings(output string) []ClaudeMCPDuplicateWarning {
	return nativeintegration.DetectClaudeMCPDuplicateWarnings(output)
}

func ClaudeMCPDuplicateWarningFixture() string {
	return nativeintegration.ClaudeMCPDuplicateWarningFixture()
}

func ValidateMermaidDocs(root string) []string {
	return validateMermaidDocs(root)
}

func LintMermaidBlocks(relPath, text string) []string {
	return lintMermaidBlocks(relPath, text)
}

func FindUnredactedSecretLike(text string) []string {
	return findUnredactedSecretLike(text)
}

func ContainsForbiddenLegacyOutsideRuntimePaths(text, root string) bool {
	return containsForbiddenLegacyOutsideRuntimePaths(text, root)
}

func ForbiddenNameHits(root string) []string {
	return forbiddenNameHits(root)
}

func ValidateHarnessInvariants(root string) StepResult {
	return validateHarnessInvariants(root)
}
