package validationcli

import "issueops/cmd/issueops/validationcli/nativeintegration"

type ClaudeMCPDuplicateWarning = nativeintegration.ClaudeMCPDuplicateWarning

func validateNativeIntegration(root string) StepResult {
	return nativeintegration.Validate(root)
}
