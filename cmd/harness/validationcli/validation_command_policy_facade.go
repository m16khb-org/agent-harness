package validationcli

import "agent-harness/cmd/harness/validationcli/commandpolicy"

func validateCommandPolicy(binary, root string) StepResult {
	return commandpolicy.Validate(binary, root)
}
