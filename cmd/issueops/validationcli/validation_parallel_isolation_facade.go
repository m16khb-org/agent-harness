package validationcli

import "issueops/cmd/issueops/validationcli/parallelisolation"

func validateParallelTempIsolation(binary, root string, seed int64) StepResult {
	return parallelisolation.Validate(binary, root, seed)
}
