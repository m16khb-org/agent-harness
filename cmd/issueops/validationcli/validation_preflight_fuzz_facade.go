package validationcli

import "issueops/cmd/issueops/validationcli/preflightfuzz"

func validatePreflightFuzz(binary, root string, seed int64) StepResult {
	return preflightfuzz.Validate(binary, root, seed)
}
