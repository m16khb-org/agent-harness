package validationcli

import "issueops/cmd/issueops/validationcli/commandpolicy"

func validateCommandPolicy(binary, root string) StepResult {
	return commandpolicy.Validate(binary, root)
}
