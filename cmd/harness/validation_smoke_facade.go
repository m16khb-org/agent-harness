package main

import "agent-harness/cmd/harness/validationcli"

func validateInspect(binary, root string) StepResult {
	return validationcli.ValidateInspect(binary, root)
}

func validateDocsIndex(binary, root string) StepResult {
	return validationcli.ValidateDocsIndex(binary, root)
}

func validateRedactionAudit(root string) StepResult {
	return validationcli.ValidateRedactionAudit(root)
}

func validateQAGate(root string) StepResult {
	return validationcli.ValidateQAGate(root)
}

func validateMermaidDocs(root string) []string {
	return validationcli.ValidateMermaidDocs(root)
}

func lintMermaidBlocks(relPath, text string) []string {
	return validationcli.LintMermaidBlocks(relPath, text)
}

func findUnredactedSecretLike(text string) []string {
	return validationcli.FindUnredactedSecretLike(text)
}

func containsForbiddenLegacyOutsideRuntimePaths(text, root string) bool {
	return validationcli.ContainsForbiddenLegacyOutsideRuntimePaths(text, root)
}

func forbiddenNameHits(root string) []string {
	return validationcli.ForbiddenNameHits(root)
}

func validateHarnessInvariants(root string) StepResult {
	return validationcli.ValidateHarnessInvariants(root)
}
