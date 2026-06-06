package validationcli

import "agent-harness/cmd/harness/validationcli/parallelisolation"

func validateParallelTempIsolation(binary, root string, seed int64) StepResult {
	return parallelisolation.Validate(binary, root, seed)
}
