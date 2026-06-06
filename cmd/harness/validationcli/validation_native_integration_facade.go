package validationcli

import "agent-harness/cmd/harness/validationcli/nativeintegration"

type ClaudeMCPDuplicateWarning = nativeintegration.ClaudeMCPDuplicateWarning

func validateNativeIntegration(root string) StepResult {
	return nativeintegration.Validate(root)
}
