package validationcli

import "issueops/cmd/issueops/validationcli/invariants"

func containsForbiddenLegacyOutsideRuntimePaths(text, root string) bool {
	return invariants.ContainsForbiddenLegacyOutsideRuntimePaths(text, root)
}

func forbiddenNameHits(root string) []string {
	return invariants.ForbiddenNameHits(root)
}

func validateHarnessInvariants(root string) StepResult {
	return invariants.ValidateHarnessInvariants(root)
}
