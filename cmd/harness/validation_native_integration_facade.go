package main

import "agent-harness/cmd/harness/validationcli"

func validateNativeIntegration(root string) StepResult {
	return validationcli.ValidateNativeIntegration(root)
}

type ClaudeMCPDuplicateWarning = validationcli.ClaudeMCPDuplicateWarning

func detectClaudeMCPDuplicateWarnings(output string) []ClaudeMCPDuplicateWarning {
	return validationcli.DetectClaudeMCPDuplicateWarnings(output)
}

func claudeMCPDuplicateWarningFixture() string {
	return validationcli.ClaudeMCPDuplicateWarningFixture()
}
