package validationcli

import "issueops/cmd/issueops/validationcli/smoke"

func validateInspect(binary, root string) StepResult {
	return smoke.ValidateInspect(binary, root)
}

func validateDocsIndex(binary, root string) StepResult {
	return smoke.ValidateDocsIndex(binary, root)
}
